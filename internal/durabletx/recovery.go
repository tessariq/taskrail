package durabletx

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"slices"

	"github.com/tessariq/taskrail/internal/durablefs"
	"github.com/tessariq/taskrail/internal/repolock"
)

// Action is the single mechanical recovery operation selected from retained
// journal evidence and the complete current set.
type Action string

const (
	RestoreOriginal Action = "restore_original"
	AcceptCandidate Action = "accept_candidate"
	ClearFence      Action = "clear_fence"
)

// RecoveryRequest identifies retained state and supplies the owning command's
// pure validator. Apply false is a read-only preview.
type RecoveryRequest struct {
	TransactionID string
	Apply         bool
	Validate      func(command string, snapshots []Evidence) error
}

// RecoveryResult is one previewed or applied mechanical action.
type RecoveryResult struct {
	TransactionID string
	Command       string
	Phase         Phase
	Action        Action
	Applied       bool
	Snapshots     []Evidence
}

// Recover previews or applies the only action compatible with the immutable
// manifest, retained phase, and complete current set.
func Recover(ctx context.Context, own Ownership, repo repolock.Repository, req RecoveryRequest) (RecoveryResult, error) {
	if !transactionIDPattern.MatchString(req.TransactionID) {
		return RecoveryResult{}, fmt.Errorf("transaction id %q is not lower-case 32-hex", req.TransactionID)
	}
	if declared := own.Owner().TransactionID; declared == nil || *declared != req.TransactionID {
		return RecoveryResult{}, refused(fmt.Errorf("held lock does not name transaction %s", req.TransactionID))
	}
	if err := repositoryMatches(own, repo); err != nil {
		return RecoveryResult{}, refused(err)
	}
	if err := ctx.Err(); err != nil {
		return RecoveryResult{}, err
	}
	store, err := openStore(own, repo)
	if err != nil {
		return RecoveryResult{}, err
	}
	defer store.Close()
	doc, entries, completed, err := loadRetained(store, req.TransactionID)
	if err != nil {
		return RecoveryResult{}, err
	}
	current, err := observeEntries(store, entries)
	if err != nil {
		return RecoveryResult{}, failure(KindConflict, req.TransactionID, doc.Phase, evidence(entries), err)
	}
	action, err := selectAction(doc.Phase, entries, current)
	if completed != nil {
		action, err = completed.Action, nil
		if !actionComplete(action, entries, current) {
			err = fmt.Errorf("completed recovery marker disagrees with the current set")
		}
	}
	if err != nil {
		return RecoveryResult{}, failure(KindConflict, req.TransactionID, doc.Phase, evidenceFrom(entries, current), err)
	}
	result := RecoveryResult{TransactionID: req.TransactionID, Command: doc.Command, Phase: doc.Phase, Action: action, Snapshots: evidenceFrom(entries, current)}
	if action == AcceptCandidate && completed == nil {
		if req.Validate == nil {
			return RecoveryResult{}, failure(KindValidation, req.TransactionID, doc.Phase, result.Snapshots,
				fmt.Errorf("accept_candidate requires the %q validator", doc.Command))
		}
		if err := req.Validate(doc.Command, result.Snapshots); err != nil {
			return RecoveryResult{}, failure(KindValidation, req.TransactionID, doc.Phase, result.Snapshots, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return RecoveryResult{}, failure(KindRecovery, req.TransactionID, doc.Phase, result.Snapshots, err)
	}
	if !req.Apply {
		return result, nil
	}
	if completed != nil {
		if err := cleanup(store, req.TransactionID, entries, func() error {
			latest, err := observeEntries(store, entries)
			if err != nil || !sameObservations(current, latest) || !actionComplete(action, entries, latest) {
				return fmt.Errorf("completed set changed before fence clear: %w", err)
			}
			return nil
		}); err != nil {
			return RecoveryResult{}, failure(KindRecovery, req.TransactionID, doc.Phase, result.Snapshots, err)
		}
		result.Applied = true
		return result, nil
	}
	recoveryPhase := phaseForAction(action)
	if doc.Phase != recoveryPhase {
		doc, err = advancePhase(store, req.TransactionID, doc, recoveryPhase)
		if err != nil {
			return RecoveryResult{}, failure(KindRecovery, req.TransactionID, doc.Phase, result.Snapshots, err)
		}
	}
	latest, err := observeEntries(store, entries)
	if err != nil || !sameObservations(current, latest) {
		return RecoveryResult{}, failure(KindConflict, req.TransactionID, doc.Phase, evidenceFrom(entries, latest),
			fmt.Errorf("transaction set changed before recovery apply: %w", err))
	}
	if err := applyAction(ctx, store, action, entries, latest); err != nil {
		return RecoveryResult{}, failure(KindRecovery, req.TransactionID, doc.Phase, evidenceFrom(entries, latest), err)
	}
	final, err := observeEntries(store, entries)
	if err != nil || !actionComplete(action, entries, final) {
		return RecoveryResult{}, failure(KindRecovery, req.TransactionID, doc.Phase, evidenceFrom(entries, final),
			fmt.Errorf("recovery action did not reach its complete state: %w", err))
	}
	if action == AcceptCandidate {
		if err := req.Validate(doc.Command, evidenceFrom(entries, final)); err != nil {
			return RecoveryResult{}, failure(KindValidation, req.TransactionID, doc.Phase, evidenceFrom(entries, final), err)
		}
	}
	if err := ctx.Err(); err != nil {
		return RecoveryResult{}, failure(KindRecovery, req.TransactionID, doc.Phase, evidenceFrom(entries, final), err)
	}
	if err := markComplete(store, req.TransactionID, doc.Command, action, entries); err != nil {
		return RecoveryResult{}, failure(KindRecovery, req.TransactionID, doc.Phase, evidenceFrom(entries, final), err)
	}
	latest, err = observeEntries(store, entries)
	if err != nil || !sameObservations(final, latest) || !actionComplete(action, entries, latest) {
		return RecoveryResult{}, failure(KindConflict, req.TransactionID, doc.Phase, evidenceFrom(entries, latest),
			fmt.Errorf("transaction set changed after recovery validation: %w", err))
	}
	if err := ctx.Err(); err != nil {
		return RecoveryResult{}, failure(KindRecovery, req.TransactionID, doc.Phase, evidenceFrom(entries, latest), err)
	}
	if err := cleanup(store, req.TransactionID, entries, func() error {
		current, err := observeEntries(store, entries)
		if err != nil || !sameObservations(latest, current) || !actionComplete(action, entries, current) {
			return fmt.Errorf("recovered set changed before fence clear: %w", err)
		}
		return nil
	}); err != nil {
		return RecoveryResult{}, failure(KindRecovery, req.TransactionID, doc.Phase, evidenceFrom(entries, final), err)
	}
	result.Phase, result.Applied, result.Snapshots = doc.Phase, true, evidenceFrom(entries, final)
	return result, nil
}

func loadRetained(store *store, id string) (journal, []*transactionEntry, *completion, error) {
	var done completion
	completionErr := store.readDocument(store.transactionDir(id)+"/"+completionName, &done)
	if completionErr != nil && !errors.Is(completionErr, fs.ErrNotExist) {
		return journal{}, nil, nil, completionErr
	}
	if errors.Is(completionErr, fs.ErrNotExist) {
		completionErr = store.readDocument(clearingPath(store, id), &done)
		if completionErr != nil && !errors.Is(completionErr, fs.ErrNotExist) {
			return journal{}, nil, nil, completionErr
		}
	}
	var doc journal
	journalErr := store.readDocument(store.transactionDir(id)+"/"+journalName, &doc)
	if journalErr != nil && completionErr != nil {
		if !errors.Is(journalErr, fs.ErrNotExist) || !errors.Is(completionErr, fs.ErrNotExist) {
			return journal{}, nil, nil, errors.Join(journalErr, completionErr)
		}
		var preparing journal
		if err := store.readDocument(preparingPath(store, id), &preparing); err != nil {
			return journal{}, nil, nil, journalErr
		}
		if err := preparing.validate(id); err != nil || preparing.Phase != PhasePrepared {
			return journal{}, nil, nil, fmt.Errorf("invalid preparing marker: %w", err)
		}
		return preparing, []*transactionEntry{}, &completion{Action: ClearFence}, nil
	}
	if journalErr != nil {
		if !errors.Is(journalErr, fs.ErrNotExist) {
			return journal{}, nil, nil, journalErr
		}
		doc = journal{TransactionID: id, Command: done.Manifest.Command, Phase: phaseForAction(done.Action)}
	}
	if err := doc.validate(id); err != nil {
		return journal{}, nil, nil, err
	}
	if completionErr == nil {
		if err := done.Manifest.validate(id, store.repo); err != nil {
			return journal{}, nil, nil, fmt.Errorf("invalid completion manifest: %w", err)
		}
		if done.Manifest.Command != doc.Command || !completionMatchesPhase(done.Action, doc.Phase) {
			return journal{}, nil, nil, fmt.Errorf("completion action disagrees with retained journal")
		}
	}
	var saved manifest
	manifestErr := store.readDocument(store.transactionDir(id)+"/"+manifestName, &saved)
	if manifestErr != nil {
		if completionErr != nil {
			return journal{}, nil, nil, manifestErr
		}
		if !errors.Is(manifestErr, fs.ErrNotExist) {
			return journal{}, nil, nil, manifestErr
		}
		saved = done.Manifest
	} else if completionErr == nil && !reflect.DeepEqual(saved, done.Manifest) {
		return journal{}, nil, nil, fmt.Errorf("completion manifest disagrees with retained manifest")
	}
	if err := saved.validate(id, store.repo); err != nil {
		return journal{}, nil, nil, err
	}
	if saved.Command != doc.Command {
		return journal{}, nil, nil, fmt.Errorf("journal and manifest command disagree")
	}
	entries := make([]*transactionEntry, len(saved.Members))
	for i, member := range saved.Members {
		entry := &transactionEntry{manifest: member}
		if member.Original != nil && completionErr != nil {
			data, err := durableReadOriginal(store, id, i)
			if err != nil {
				return journal{}, nil, nil, err
			}
			if digest(data) != member.Original.SHA256 {
				return journal{}, nil, nil, fmt.Errorf("original %d digest disagrees with manifest", i)
			}
			entry.original = data
		}
		if member.Fence != nil && completionErr != nil {
			data, err := durableReadFinal(store, id, i)
			if err != nil {
				return journal{}, nil, nil, err
			}
			if digest(data) != member.Candidate.SHA256 {
				return journal{}, nil, nil, fmt.Errorf("retained final %d digest disagrees with manifest", i)
			}
			entry.candidate = data
		}
		entries[i] = entry
	}
	if completionErr == nil {
		if done.Action != RestoreOriginal && done.Action != AcceptCandidate && done.Action != ClearFence {
			return journal{}, nil, nil, fmt.Errorf("completion records unknown action %q", done.Action)
		}
		return doc, entries, &done, nil
	}
	return doc, entries, nil, nil
}

func completionMatchesPhase(action Action, phase Phase) bool {
	switch phase {
	case PhaseValidating, PhaseRecoveryAccepting:
		return action == AcceptCandidate
	case PhaseRollingBack, PhaseRecoveryRestoring:
		return action == RestoreOriginal
	case PhaseRecoveryClearing:
		return action == ClearFence
	default:
		return false
	}
}

func durableReadOriginal(store *store, id string, index int) ([]byte, error) {
	relative := fmt.Sprintf("%s/%s/%08d", store.transactionDir(id), originalsDirName, index)
	data, _, err := durablefs.ReadFile(store.baseAbsolute, relative, maximumJournalBytes)
	return data, err
}

func durableReadFinal(store *store, id string, index int) ([]byte, error) {
	relative := fmt.Sprintf("%s/%s/%08d", store.transactionDir(id), finalsDirName, index)
	data, _, err := durablefs.ReadFile(store.baseAbsolute, relative, maximumJournalBytes)
	return data, err
}

func selectAction(phase Phase, entries []*transactionEntry, current []observation) (Action, error) {
	switch phase {
	case PhaseRollingBack, PhaseRecoveryRestoring:
		for i, entry := range entries {
			if holdsOriginal(entry, current[i]) {
				continue
			}
			// A fence member mid-rollback still holds its fence bytes: the
			// rollback restores it last, so an interrupted restore retries the
			// same action rather than stranding the repository.
			if holdsCandidate(entry, current[i]) || holdsFence(entry, current[i]) {
				continue
			}
			return "", fmt.Errorf("%s changed during restore recovery", entry.manifest.Reported)
		}
		return RestoreOriginal, nil
	case PhaseRecoveryAccepting:
		if candidateComplete(entries, current) {
			return AcceptCandidate, nil
		}
		return "", fmt.Errorf("candidate changed during accept recovery")
	case PhaseRecoveryClearing:
		if allExactOriginal(entries, current) {
			return ClearFence, nil
		}
		return "", fmt.Errorf("original set changed during clear recovery")
	}
	original := 0
	for i, entry := range entries {
		if sameOriginal(entry, current[i]) {
			original++
			continue
		}
		if holdsCandidate(entry, current[i]) || holdsFence(entry, current[i]) {
			continue
		}
		return "", fmt.Errorf("%s holds neither recorded original nor candidate state", entry.manifest.Reported)
	}
	switch phase {
	case PhasePrepared, PhaseFencePublished:
		if original == len(entries) {
			return ClearFence, nil
		}
		if phase == PhaseFencePublished && onlyFenceWritten(entries, current) {
			return RestoreOriginal, nil
		}
		return "", fmt.Errorf("candidate bytes exist before publication phase")
	case PhaseCandidatePublished, PhaseValidating:
		if candidateComplete(entries, current) {
			return AcceptCandidate, nil
		}
		return "", fmt.Errorf("candidate-published phase does not hold the complete candidate")
	case PhasePublishing:
		if original == len(entries) {
			return ClearFence, nil
		}
		if allCandidate(entries, current) {
			return AcceptCandidate, nil
		}
		return RestoreOriginal, nil
	}
	return "", fmt.Errorf("phase %q has no recovery action", phase)
}

func phaseForAction(action Action) Phase {
	switch action {
	case RestoreOriginal:
		return PhaseRecoveryRestoring
	case AcceptCandidate:
		return PhaseRecoveryAccepting
	default:
		return PhaseRecoveryClearing
	}
}

func sameObservations(a, b []observation) bool {
	return slices.EqualFunc(a, b, sameObservation)
}

func sameObservation(a, b observation) bool {
	return a.present == b.present && a.snapshot == b.snapshot && slices.Equal(a.ancestors, b.ancestors)
}

func applyAction(ctx context.Context, store *store, action Action, entries []*transactionEntry, current []observation) error {
	if action == AcceptCandidate {
		return completeFenceFinals(ctx, store, entries)
	}
	if action != RestoreOriginal {
		return nil
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry := entries[i]
		if !entry.manifest.Published || holdsOriginal(entry, current[i]) {
			continue
		}
		latest, _, err := observe(store, entry.manifest.Kind, entry.manifest.Path)
		if err != nil || !sameObservation(current[i], latest) || !holdsRecordedWrite(entry, latest) {
			return fmt.Errorf("%s changed before restore: %w", entry.manifest.Reported, err)
		}
		if err := restoreOne(store, entry, latest); err != nil {
			return err
		}
		if testHookAfterMember != nil {
			if err := testHookAfterMember(PhaseRecoveryRestoring, entry.manifest.Reported); err != nil {
				return err
			}
		}
	}
	return nil
}

func actionComplete(action Action, entries []*transactionEntry, current []observation) bool {
	if action == RestoreOriginal {
		return allOriginal(entries, current)
	}
	if action == ClearFence {
		return allExactOriginal(entries, current)
	}
	return allCandidate(entries, current)
}

func allExactOriginal(entries []*transactionEntry, current []observation) bool {
	for i, entry := range entries {
		if !sameOriginal(entry, current[i]) {
			return false
		}
	}
	return true
}
