package durabletx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/tessariq/taskrail/internal/durablefs"
	"github.com/tessariq/taskrail/internal/repolock"
)

type transactionEntry struct {
	manifest  manifestMember
	candidate []byte
	original  []byte
	// fence holds the writer's fence bytes in memory; only their digest and
	// mode are recorded in the manifest, exactly like the final candidate.
	fence               []byte
	preSemantic         bool
	preSemanticPriority int
	observed            observation
}

type observation struct {
	present   bool
	snapshot  durablefs.Snapshot
	ancestors []durablefs.Identity
}

var (
	testHookAfterPhase     func(Phase) error
	testHookAfterMember    func(Phase, string) error
	testHookAfterFenceMove func() error
)

// Run prepares and publishes one durable transaction. Any error after the
// journal becomes visible either restores the complete original set or retains
// a recovery fence; it never reports an ordinary un-fenced partial write.
//
// A transaction with a fence member publishes in two stages: the fence bytes
// land after the originals are recorded durably and before any other semantic
// byte changes, and the fence member's final candidate publishes as the last
// semantic operation, after post-publication validation.
func Run(ctx context.Context, own Ownership, repo repolock.Repository, req Request) (Result, error) {
	if err := req.validate(repo); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := authorize(own, req); err != nil {
		return Result{}, err
	}
	if err := repositoryMatches(own, repo); err != nil {
		return Result{}, refused(err)
	}
	store, err := openStore(own, repo)
	if err != nil {
		return Result{}, err
	}
	defer store.Close()

	id, err := newTransactionID(own)
	if err != nil {
		return Result{}, err
	}
	entries, err := prepareEntries(store, req)
	if err != nil {
		return Result{}, err
	}
	preview := evidence(entries)
	if req.Validate != nil {
		if err := req.Validate(preview); err != nil {
			return Result{}, failure(KindValidation, id, "", preview, err)
		}
	}
	if err := persistPreparation(store, id, req.Command, entries); err != nil {
		return Result{}, failure(KindRecovery, id, PhasePrepared, preview, err)
	}
	doc := journal{TransactionID: id, Command: req.Command, Phase: PhasePrepared}
	if err := phaseHook(doc.Phase); err != nil {
		return Result{}, failure(KindRecovery, id, doc.Phase, preview, err)
	}
	if err := recheckOriginals(store, entries); err != nil {
		return Result{}, recoverRunFailure(store, id, doc, entries, err)
	}
	doc, err = advancePhase(store, id, doc, PhaseFencePublished)
	if err != nil {
		return Result{}, failure(KindRecovery, id, doc.Phase, evidence(entries), err)
	}
	if err := publishFenceMembers(store, entries); err != nil {
		return Result{}, recoverRunFailure(store, id, doc, entries, err)
	}
	if err := recheckAfterFence(store, entries); err != nil {
		return Result{}, recoverRunFailure(store, id, doc, entries, err)
	}
	preSemantic := hasPreSemantic(entries)
	if preSemantic {
		// Pre-semantic members are candidate bytes, so recovery must see the
		// publishing phase before the first one can reach disk.
		doc, err = advancePhase(store, id, doc, PhasePublishing)
		if err != nil {
			return Result{}, failure(KindRecovery, id, doc.Phase, evidence(entries), err)
		}
		if err := publishPreSemantic(ctx, store, entries); err != nil {
			return Result{}, recoverRunFailure(store, id, doc, entries, err)
		}
	}
	if preSemantic && req.ValidateBeforeCandidates != nil {
		current, observeErr := observeEntries(store, entries)
		if observeErr != nil {
			return Result{}, recoverRunFailure(store, id, doc, entries, observeErr)
		}
		if err := req.ValidateBeforeCandidates(evidenceFrom(entries, current)); err != nil {
			return Result{}, recoverRunFailure(store, id, doc, entries, err)
		}
	}
	if !preSemantic {
		doc, err = advancePhase(store, id, doc, PhasePublishing)
		if err != nil {
			return Result{}, failure(KindRecovery, id, doc.Phase, evidence(entries), err)
		}
	}
	if err := publishCandidates(ctx, store, entries); err != nil {
		return Result{}, recoverRunFailure(store, id, doc, entries, err)
	}
	doc, err = advancePhase(store, id, doc, PhaseCandidatePublished)
	if err != nil {
		return Result{}, failure(KindRecovery, id, doc.Phase, evidence(entries), err)
	}
	doc, err = advancePhase(store, id, doc, PhaseValidating)
	if err != nil {
		return Result{}, failure(KindRecovery, id, doc.Phase, evidence(entries), err)
	}
	current, err := observeEntries(store, entries)
	if err != nil || !candidateComplete(entries, current) {
		return Result{}, recoverRunFailure(store, id, doc, entries,
			fmt.Errorf("candidate changed before validation: %w", err))
	}
	if req.Validate != nil {
		if err := ctx.Err(); err != nil {
			return Result{}, recoverRunFailure(store, id, doc, entries, err)
		}
		if err := req.Validate(evidenceFrom(entries, current)); err != nil {
			return Result{}, recoverRunFailure(store, id, doc, entries, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return Result{}, recoverRunFailure(store, id, doc, entries, err)
	}
	if err := publishFenceFinals(ctx, store, entries); err != nil {
		return Result{}, recoverRunFailure(store, id, doc, entries, err)
	}
	current, err = observeEntries(store, entries)
	if err != nil || !allCandidate(entries, current) {
		return Result{}, recoverRunFailure(store, id, doc, entries,
			fmt.Errorf("candidate changed after final publication: %w", err))
	}
	if err := markComplete(store, id, req.Command, AcceptCandidate, entries); err != nil {
		return Result{}, failure(KindRecovery, id, PhaseValidating, evidenceFrom(entries, current), err)
	}
	latest, err := observeEntries(store, entries)
	if err != nil || !sameObservations(current, latest) || !allCandidate(entries, latest) {
		return Result{}, failure(KindConflict, id, PhaseValidating, evidenceFrom(entries, latest),
			fmt.Errorf("transaction set changed after validation: %w", err))
	}
	if err := ctx.Err(); err != nil {
		return Result{}, failure(KindRecovery, id, PhaseValidating, evidenceFrom(entries, latest), err)
	}
	if err := cleanup(store, id, entries, func() error {
		current, err := observeEntries(store, entries)
		if err != nil || !sameObservations(latest, current) || !allCandidate(entries, current) {
			return fmt.Errorf("transaction set changed before fence clear: %w", err)
		}
		return nil
	}); err != nil {
		return Result{}, failure(KindRecovery, id, PhaseValidating, evidenceFrom(entries, current), err)
	}
	return Result{TransactionID: id, Phase: PhaseCandidatePublished, Members: evidenceFrom(entries, current)}, nil
}

func hasPreSemantic(entries []*transactionEntry) bool {
	for _, entry := range entries {
		if entry.preSemantic {
			return true
		}
	}
	return false
}

func repositoryMatches(own Ownership, repo repolock.Repository) error {
	owner := own.Owner()
	if owner.RepositoryRoot != repo.Root || owner.StorageMode != repo.Mode || owner.StorageRoot != repo.StorageRoot() {
		return fmt.Errorf("repository context does not match held lock")
	}
	if own.Repository() != repo {
		return fmt.Errorf("repository roots do not match held lock")
	}
	return nil
}

func prepareEntries(store *store, req Request) ([]*transactionEntry, error) {
	entries := make([]*transactionEntry, 0, len(req.Consumed)+len(req.Members))
	for _, consumed := range req.Consumed {
		entries = append(entries, &transactionEntry{manifest: manifestMember{Kind: consumed.Kind, Reported: consumed.Reported, Path: consumed.Path}})
	}
	for _, member := range req.Members {
		entry := &transactionEntry{manifest: manifestMember{Kind: member.Kind, Reported: member.Reported, Path: member.Path, Published: true},
			candidate: slices.Clone(member.Content), preSemantic: member.PreSemantic, preSemanticPriority: member.PreSemanticPriority}
		if member.Fence != nil {
			entry.fence = slices.Clone(member.Fence)
		}
		entries = append(entries, entry)
	}
	slices.SortFunc(entries, func(a, b *transactionEntry) int {
		if kinds := strings.Compare(string(a.manifest.Kind), string(b.manifest.Kind)); kinds != 0 {
			return kinds
		}
		return strings.Compare(a.manifest.Reported, b.manifest.Reported)
	})
	for _, entry := range entries {
		observed, content, err := observe(store, entry.manifest.Kind, entry.manifest.Path)
		if err != nil {
			return nil, err
		}
		entry.observed, entry.original = observed, content
		entry.manifest.Ancestors = identities(observed.ancestors)
		if observed.present {
			original := stateOf(observed.snapshot)
			entry.manifest.Original = &original
		}
		if entry.manifest.Published {
			mode := defaultMemberMode
			if observed.present {
				mode = observed.snapshot.Mode
			} else if requested := requestedMode(req.Members, entry.manifest.Kind, entry.manifest.Reported); requested != 0 {
				mode = requested
			}
			candidate := fileState{SHA256: digest(entry.candidate), Mode: uint32(durablefs.PortableMode(mode))}
			entry.manifest.Candidate = &candidate
			if entry.fence != nil {
				entry.manifest.Fence = &fileState{SHA256: digest(entry.fence), Mode: candidate.Mode}
			}
		}
	}
	return entries, nil
}

func requestedMode(members []Member, kind PathKind, reported string) fs.FileMode {
	for _, member := range members {
		if member.Kind == kind && member.Reported == reported {
			return member.Mode
		}
	}
	return 0
}

func persistPreparation(store *store, id, command string, entries []*transactionEntry) error {
	tx := store.transactionDir(id)
	if err := store.ensureDirectory(store.base, store.baseAbsolute, store.transactions); err != nil {
		return err
	}
	preparing, err := encodeDocument(journal{TransactionID: id, Command: command, Phase: PhasePrepared})
	if err != nil {
		return err
	}
	if err := publishDocument(store.base, preparingPath(store, id), preparing); err != nil {
		return err
	}
	if err := store.ensureDirectory(store.base, store.baseAbsolute, tx+"/"+originalsDirName); err != nil {
		return err
	}
	for i, entry := range entries {
		if entry.manifest.Original == nil {
			continue
		}
		if err := publishDocument(store.base, fmt.Sprintf("%s/%s/%08d", tx, originalsDirName, i), entry.original); err != nil {
			return err
		}
	}
	if err := store.ensureDirectory(store.base, store.baseAbsolute, tx+"/"+finalsDirName); err != nil {
		return err
	}
	for i, entry := range entries {
		if entry.manifest.Fence == nil {
			continue
		}
		// A fence member's final bytes publish after validation, so an
		// interruption between the two leaves recovery to complete them; that
		// completion is mechanical only when the exact bytes are retained.
		if err := publishDocument(store.base, fmt.Sprintf("%s/%s/%08d", tx, finalsDirName, i), entry.candidate); err != nil {
			return err
		}
	}
	members := make([]manifestMember, len(entries))
	for i, entry := range entries {
		members[i] = entry.manifest
	}
	data, err := encodeDocument(manifest{TransactionID: id, Command: command, Members: members})
	if err != nil {
		return err
	}
	if err := publishDocument(store.base, tx+"/"+manifestName, data); err != nil {
		return err
	}
	if err := store.writeJournal(id, journal{TransactionID: id, Command: command, Phase: PhasePrepared}); err != nil {
		return err
	}
	return removeFile(store.base, preparingPath(store, id))
}

func preparingPath(store *store, id string) string {
	return store.transactions + "/" + id + ".preparing.json"
}

func clearingPath(store *store, id string) string {
	return store.transactions + "/" + id + ".clearing.json"
}

func publishDocument(root *durablefs.Root, name string, data []byte) error {
	entry, err := root.Publish(name, data, documentMode)
	if err != nil {
		return err
	}
	return entry.Close()
}

func advancePhase(store *store, id string, doc journal, next Phase) (journal, error) {
	advanced, err := store.advance(id, doc, next)
	if err != nil {
		return doc, err
	}
	if err := phaseHook(next); err != nil {
		return advanced, err
	}
	return advanced, nil
}

func phaseHook(phase Phase) error {
	if testHookAfterPhase != nil {
		return testHookAfterPhase(phase)
	}
	return nil
}

func publishCandidates(ctx context.Context, store *store, entries []*transactionEntry) error {
	for _, entry := range entries {
		// A fence member's final candidate publishes as the transaction's last
		// semantic operation, after post-publication validation.
		if !entry.manifest.Published || entry.manifest.Fence != nil || entry.preSemantic {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		current, _, err := observe(store, entry.manifest.Kind, entry.manifest.Path)
		if err != nil || !sameOriginal(entry, current) {
			return fmt.Errorf("%s changed before publication: %w", entry.manifest.Reported, err)
		}
		if err := put(store, entry, current, entry.candidate, entry.manifest.Candidate.mode()); err != nil {
			return err
		}
		if testHookAfterMember != nil {
			if err := testHookAfterMember(PhasePublishing, entry.manifest.Reported); err != nil {
				return err
			}
		}
	}
	return nil
}

func publishPreSemantic(ctx context.Context, store *store, entries []*transactionEntry) error {
	ordered := make([]*transactionEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.preSemantic {
			continue
		}
		ordered = append(ordered, entry)
	}
	slices.SortFunc(ordered, func(a, b *transactionEntry) int {
		if a.preSemanticPriority != b.preSemanticPriority {
			return a.preSemanticPriority - b.preSemanticPriority
		}
		return strings.Compare(a.manifest.Reported, b.manifest.Reported)
	})
	for _, entry := range ordered {
		if err := ctx.Err(); err != nil {
			return err
		}
		current, _, err := observe(store, entry.manifest.Kind, entry.manifest.Path)
		if err != nil || !sameOriginal(entry, current) {
			return fmt.Errorf("%s changed before pre-semantic publication: %w", entry.manifest.Reported, err)
		}
		if err := put(store, entry, current, entry.candidate, entry.manifest.Candidate.mode()); err != nil {
			return err
		}
	}
	return nil
}

func put(store *store, entry *transactionEntry, current observation, content []byte, mode fs.FileMode) error {
	root := store.rootFor(entry.manifest.Kind)
	if current.present {
		bound, err := root.Rebind(entry.manifest.Path, current.snapshot)
		if err != nil {
			return err
		}
		replaced, err := bound.Replace(content, mode)
		if err != nil {
			bound.Close()
			return err
		}
		return replaced.Close()
	}
	parent := path.Dir(entry.manifest.Path)
	if parent != "." {
		if err := store.ensureDirectory(root, store.absoluteFor(entry.manifest.Kind), parent); err != nil {
			return err
		}
	}
	created, err := root.Publish(entry.manifest.Path, content, mode)
	if err != nil {
		return err
	}
	return created.Close()
}

func recheckOriginals(store *store, entries []*transactionEntry) error {
	current, err := observeEntries(store, entries)
	if err != nil {
		return err
	}
	for i, entry := range entries {
		if !sameOriginal(entry, current[i]) {
			return fmt.Errorf("%s changed since preparation", entry.manifest.Reported)
		}
	}
	return nil
}

func sameOriginal(entry *transactionEntry, current observation) bool {
	if entry.manifest.Original == nil {
		return !current.present && identityPrefix(entry.manifest.Ancestors, current.ancestors)
	}
	return current.present && entry.manifest.Original.holds(current.snapshot) &&
		entry.manifest.Original.Identity.matches(current.snapshot.Identity) && identityPrefix(entry.manifest.Ancestors, current.ancestors)
}

func identityPrefix(expected []identity, current []durablefs.Identity) bool {
	if len(current) < len(expected) {
		return false
	}
	for i := range expected {
		if !expected[i].matches(current[i]) {
			return false
		}
	}
	return true
}

func identities(values []durablefs.Identity) []identity {
	result := make([]identity, len(values))
	for i, value := range values {
		result[i] = *recordedIdentity(value)
	}
	return result
}

func observeEntries(store *store, entries []*transactionEntry) ([]observation, error) {
	result := make([]observation, len(entries))
	for i, entry := range entries {
		observed, _, err := observe(store, entry.manifest.Kind, entry.manifest.Path)
		if err != nil {
			return nil, err
		}
		result[i] = observed
	}
	return result, nil
}

func observe(store *store, kind PathKind, relative string) (observation, []byte, error) {
	base := store.absoluteFor(kind)
	parent, leaf := path.Dir(relative), path.Base(relative)
	var tree durablefs.TreeSnapshot
	var err error
	if parent == "." {
		tree, err = durablefs.ObserveRoot(base)
	} else {
		tree, err = durablefs.ObserveTree(base, parent)
	}
	if err != nil {
		return observation{}, nil, err
	}
	ancestors := slices.Clone(tree.Ancestors)
	if tree.Present {
		ancestors = append(ancestors, tree.Identity)
	}
	for _, candidate := range tree.Entries {
		if candidate.Path != leaf {
			continue
		}
		if candidate.Directory {
			return observation{}, nil, fmt.Errorf("%s is a directory", relative)
		}
		data, snapshot, err := durablefs.ReadFile(base, relative, maximumJournalBytes)
		if err != nil || snapshot != candidate.Snapshot {
			return observation{}, nil, fmt.Errorf("read %s from stable snapshot: %w", relative, err)
		}
		return observation{present: true, snapshot: snapshot, ancestors: ancestors}, data, nil
	}
	return observation{ancestors: ancestors}, nil, nil
}

func allCandidate(entries []*transactionEntry, current []observation) bool {
	for i, entry := range entries {
		if entry.manifest.Published {
			if !holdsCandidate(entry, current[i]) {
				return false
			}
		} else if !sameOriginal(entry, current[i]) {
			return false
		}
	}
	return true
}

func holdsCandidate(entry *transactionEntry, current observation) bool {
	return entry.manifest.Published && current.present && entry.manifest.Candidate.holds(current.snapshot) &&
		identityPrefix(entry.manifest.Ancestors, current.ancestors)
}

func evidence(entries []*transactionEntry) []Evidence {
	current := make([]observation, len(entries))
	for i, entry := range entries {
		current[i] = entry.observed
	}
	return evidenceFrom(entries, current)
}

func evidenceFrom(entries []*transactionEntry, current []observation) []Evidence {
	result := make([]Evidence, len(entries))
	for i, entry := range entries {
		item := Evidence{Kind: entry.manifest.Kind, Reported: entry.manifest.Reported}
		if entry.manifest.Original != nil {
			item.OriginalSHA256 = entry.manifest.Original.SHA256
		}
		if entry.manifest.Candidate != nil {
			item.CandidateSHA256 = entry.manifest.Candidate.SHA256
		}
		if entry.manifest.Fence != nil {
			item.FenceSHA256 = entry.manifest.Fence.SHA256
		}
		if current[i].present {
			item.CurrentSHA256 = current[i].snapshot.SHA256
			item.IdentityChanged = entry.manifest.Original != nil && entry.manifest.Original.holds(current[i].snapshot) &&
				!entry.manifest.Original.Identity.matches(current[i].snapshot.Identity)
		}
		result[i] = item
	}
	return result
}

func recoverRunFailure(store *store, id string, doc journal, entries []*transactionEntry, cause error) error {
	current, err := observeEntries(store, entries)
	if err != nil {
		return failure(KindRollbackFailed, id, doc.Phase, evidence(entries), cause)
	}
	action := RestoreOriginal
	next := PhaseRollingBack
	// Before publication began, an untouched set needs no restore: clearing the
	// retained fence is the whole undo. A fence member holding its fence bytes
	// is a written byte, so that state restores instead.
	if (doc.Phase == PhasePrepared || doc.Phase == PhaseFencePublished) && allOriginal(entries, current) {
		action, next = ClearFence, PhaseRecoveryClearing
	}
	if canAdvance(doc.Phase, next) {
		advanced, err := advancePhase(store, id, doc, next)
		if err != nil {
			return failure(KindRecovery, id, doc.Phase, evidence(entries), errors.Join(cause, err))
		}
		doc = advanced
	}
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if !entry.manifest.Published || sameOriginal(entry, current[i]) {
			continue
		}
		if !holdsRecordedWrite(entry, current[i]) {
			return failure(KindRollbackFailed, id, doc.Phase, evidenceFrom(entries, current), cause)
		}
		if err := restoreOne(store, entry, current[i]); err != nil {
			return failure(KindRollbackFailed, id, doc.Phase, evidenceFrom(entries, current), errors.Join(cause, err))
		}
		if testHookAfterMember != nil {
			if err := testHookAfterMember(PhaseRollingBack, entry.manifest.Reported); err != nil {
				return failure(KindRollbackFailed, id, doc.Phase, evidenceFrom(entries, current), errors.Join(cause, err))
			}
		}
	}
	current, err = observeEntries(store, entries)
	if err != nil || !allOriginal(entries, current) {
		return failure(KindRollbackFailed, id, doc.Phase, evidenceFrom(entries, current), errors.Join(cause, err))
	}
	if err := markComplete(store, id, doc.Command, action, entries); err != nil {
		return failure(KindRecovery, id, doc.Phase, evidenceFrom(entries, current), errors.Join(cause, err))
	}
	if err := cleanup(store, id, entries, func() error {
		latest, err := observeEntries(store, entries)
		if err != nil || !allOriginal(entries, latest) {
			return fmt.Errorf("restored set changed before fence clear: %w", err)
		}
		return nil
	}); err != nil {
		return failure(KindRecovery, id, doc.Phase, evidenceFrom(entries, current), errors.Join(cause, err))
	}
	return failure(KindRolledBack, id, doc.Phase, evidenceFrom(entries, current), cause)
}

// holdsRecordedWrite reports whether the member still holds exactly the bytes
// this transaction wrote — its final candidate, or the fence bytes when the
// final candidate has not published yet. Anything else is an external edit the
// rollback must not overwrite.
func holdsRecordedWrite(entry *transactionEntry, current observation) bool {
	if !current.present {
		return false
	}
	if entry.manifest.Candidate != nil && entry.manifest.Candidate.holds(current.snapshot) {
		return true
	}
	return holdsFence(entry, current)
}

func allOriginal(entries []*transactionEntry, current []observation) bool {
	for i, entry := range entries {
		if !holdsOriginal(entry, current[i]) {
			return false
		}
	}
	return true
}

func holdsOriginal(entry *transactionEntry, current observation) bool {
	if entry.manifest.Original == nil {
		return !current.present && identityPrefix(entry.manifest.Ancestors, current.ancestors)
	}
	return current.present && entry.manifest.Original.holds(current.snapshot) && identityPrefix(entry.manifest.Ancestors, current.ancestors)
}

func restoreOne(store *store, entry *transactionEntry, current observation) error {
	if entry.manifest.Original == nil {
		bound, err := store.rootFor(entry.manifest.Kind).Rebind(entry.manifest.Path, current.snapshot)
		if err != nil {
			return err
		}
		return bound.Remove()
	}
	return put(store, entry, current, entry.original, entry.manifest.Original.mode())
}

type completion struct {
	Action   Action   `json:"action"`
	Manifest manifest `json:"manifest"`
}

const completionName = "complete.json"

func markComplete(store *store, id, command string, action Action, entries []*transactionEntry) error {
	members := make([]manifestMember, len(entries))
	for i, entry := range entries {
		members[i] = entry.manifest
	}
	data, err := encodeDocument(completion{Action: action, Manifest: manifest{
		TransactionID: id, Command: command, Members: members,
	}})
	if err != nil {
		return err
	}
	return publishDocument(store.base, store.transactionDir(id)+"/"+completionName, data)
}

func cleanup(store *store, id string, entries []*transactionEntry, beforeClear func() error) error {
	tx := store.transactionDir(id)
	archiveRoot := path.Dir(store.transactions) + "/completed-transactions"
	if err := store.ensureDirectory(store.base, store.baseAbsolute, archiveRoot); err != nil {
		return err
	}
	archive := archiveRoot + "/" + id
	fenceMarker := preparingPath(store, id)
	if _, _, err := durablefs.ReadFile(store.baseAbsolute, fenceMarker, maximumJournalBytes); err != nil {
		fenceMarker = clearingPath(store, id)
		if _, _, clearingErr := durablefs.ReadFile(store.baseAbsolute, fenceMarker, maximumJournalBytes); clearingErr != nil {
			completeBytes, _, completeErr := durablefs.ReadFile(store.baseAbsolute, tx+"/"+completionName, maximumJournalBytes)
			if completeErr != nil {
				return completeErr
			}
			if err := publishDocument(store.base, fenceMarker, completeBytes); err != nil {
				return err
			}
		}
	}
	if beforeClear != nil {
		if err := beforeClear(); err != nil {
			return err
		}
	}
	tree, err := durablefs.ObserveTree(store.baseAbsolute, tx)
	if err != nil {
		return err
	}
	if tree.Present {
		if err := store.base.MoveDir(tx, archive); err != nil {
			return err
		}
		if testHookAfterFenceMove != nil {
			if err := testHookAfterFenceMove(); err != nil {
				return err
			}
		}
	}
	if err := store.base.SyncDir(store.transactions); err != nil {
		return err
	}
	archiveTree, err := durablefs.ObserveTree(store.baseAbsolute, archive)
	if err != nil {
		return err
	}
	if archiveTree.Present {
		if err := store.base.SyncDir(archiveRoot); err != nil {
			return err
		}
	}
	if err := removeFile(store.base, fenceMarker); err != nil {
		return err
	}
	// A successful cleanup leaves the transactions root empty; removing it
	// returns the tree to its absent baseline. The removal is best-effort for
	// the same reason the archive cleanup below is: the boundary treats an
	// empty tree as no retained state either way.
	remaining, err := durablefs.ObserveTree(store.baseAbsolute, store.transactions)
	if err != nil {
		return err
	}
	if remaining.Present && len(remaining.Entries) == 0 {
		_ = removeDirectory(store.base, store.transactions)
	}
	// The durable move is the fence-clear commit point. Cleanup after it cannot
	// change semantic state and must not turn a committed transaction into an
	// unfenced failure; interrupted archive cleanup is harmless retained garbage.
	for i, entry := range entries {
		if entry.manifest.Original != nil {
			_ = removeFile(store.base, fmt.Sprintf("%s/%s/%08d", archive, originalsDirName, i))
		}
		if entry.manifest.Fence != nil {
			_ = removeFile(store.base, fmt.Sprintf("%s/%s/%08d", archive, finalsDirName, i))
		}
	}
	_ = removeDirectory(store.base, archive+"/"+originalsDirName)
	_ = removeDirectory(store.base, archive+"/"+finalsDirName)
	for _, name := range []string{manifestName, journalName, completionName} {
		_ = removeFile(store.base, archive+"/"+name)
	}
	_ = removeDirectory(store.base, archive)
	return nil
}

func removeFile(root *durablefs.Root, name string) error {
	entry, err := root.Bind(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return entry.Remove()
}

func removeDirectory(root *durablefs.Root, name string) error {
	err := root.RemoveDir(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
