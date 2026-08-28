package taskrail

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/tessariq/taskrail/internal/durabletx"
	"github.com/tessariq/taskrail/internal/repolock"
)

// The public recovery boundary over the durable transaction engine
// (specs/v0.5.0.md#repository-discovery-locking-and-recovery): one command that
// previews or performs the single mechanically safe action retained journal
// evidence and the complete current set derive. The engine owns action
// selection, whole-set compare-and-swap, and interruption safety; this surface
// owns lock identity, machine classification, and the typed snapshot evidence
// an agent reads.

// RecoverResult is the companion's RecoverResult: the derived action, whether
// this invocation performed it, the typed whole-set evidence, and the coherence
// of the repository state the boundary observed — the current state for a
// preview, the resulting state for an apply.
type RecoverResult struct {
	TransactionID  string            `json:"transaction_id"`
	Command        string            `json:"command"`
	Action         string            `json:"action"`
	Applied        bool              `json:"applied"`
	Takeover       string            `json:"takeover"`
	TakeOverLockID *string           `json:"take_over_lock_id"`
	TakeOverSHA256 *string           `json:"take_over_sha256"`
	Snapshots      []MachineSnapshot `json:"snapshots"`
	Validation     ValidationResult  `json:"validation"`
}

// RecoverRequest supplies the optional, exact operator authorization to take
// over the abandoned lock an interrupted transaction left behind.
type RecoverRequest struct {
	TakeOverLockID string
	ExpectSHA256   string
}

// NewRecoveryService discovers the repository for the recovery boundary itself.
// Unlike NewService it admits retained transaction state and the fenced
// migration marker, because `recover` is the one command that state exists to
// be handed to.
func NewRecoveryService(start string) (*Service, error) {
	paths, err := DiscoverRecoveryPaths(start)
	if err != nil {
		return nil, err
	}
	snapshot, err := observeRecovery(paths)
	if err != nil {
		return nil, err
	}
	return &Service{paths: paths, now: time.Now, recovery: recoveryAdmission{snapshot: snapshot}}, nil
}

// RecoverTransaction previews (apply false) or performs (apply true) the only
// action compatible with the retained journal for transactionID. The mutation
// lock is acquired naming exactly that transaction, so a live or abandoned lock
// holder refuses before any evidence is read, and the engine's whole-set checks
// refuse every unexpected byte without overwriting it.
func (s *Service) RecoverTransaction(ctx context.Context, transactionID string, apply bool, requests ...RecoverRequest) (result RecoverResult, err error) {
	if !transactionIDPattern.MatchString(transactionID) {
		return RecoverResult{}, invalidArgumentsf("transaction id %q is not a lower-case 32-hex id", transactionID)
	}
	if len(requests) > 1 {
		return RecoverResult{}, invalidArgumentsf("recover accepts at most one takeover request")
	}
	if len(requests) == 1 && (requests[0].TakeOverLockID != "" || requests[0].ExpectSHA256 != "") {
		return s.recoverWithTakeover(ctx, transactionID, apply, requests[0])
	}
	if delegatedInvocation() {
		return RecoverResult{}, WithMachineErrorCode(MachineCodeDelegatedRefused,
			fmt.Errorf("delegated loop children cannot invoke recover"))
	}
	command := "recover"
	lock, err := repolock.Acquire(ctx, repolock.Request{
		Repository:    s.paths.LockRepository(),
		Command:       command,
		TransactionID: transactionID,
		Capability:    repolock.Capability{Commands: []string{command}},
	})
	if err != nil {
		return RecoverResult{}, recoveryLockError(err)
	}
	var recovered durabletx.RecoveryResult
	defer func() {
		if releaseErr := lock.Release(); releaseErr != nil && err == nil {
			// A failed release after a committed apply strands the lock, but the
			// recovery itself is on disk — the same applied-tagging rule the
			// read-back failure below follows.
			err = WithMachineFailure(
				MachineFailure{Code: MachineCodeRepositoryInvalid, Applied: apply && recovered.Applied}, releaseErr)
		}
	}()

	recovered, err = durabletx.Recover(ctx, lock, s.paths.LockRepository(), durabletx.RecoveryRequest{
		TransactionID: transactionID,
		Apply:         apply,
		Validate:      s.recoveryValidator(transactionID),
	})
	if err != nil {
		return RecoverResult{}, s.mapRecoveryError(transactionID, err)
	}

	validation, err := s.recoveredValidation(recovered)
	if err != nil {
		// A read-back failure after a committed apply is the outcome of an
		// operation already on disk, so it is tagged applied rather than
		// published as a refusal that changed nothing.
		if apply && recovered.Applied {
			return RecoverResult{}, WithMachineFailure(
				MachineFailure{Code: MachineCodeRepositoryInvalid, Applied: true}, err)
		}
		return RecoverResult{}, err
	}
	return RecoverResult{
		TransactionID: recovered.TransactionID,
		Command:       recovered.Command,
		Action:        string(recovered.Action),
		Applied:       recovered.Applied,
		Takeover:      "none",
		Snapshots:     recoverySnapshots(recovered.Snapshots),
		Validation:    validation,
	}, nil
}

// recoveryPreviewOwnership lets an explicitly authorized takeover preview the
// retained transaction without first clearing or replacing the observed lock.
type recoveryPreviewOwnership struct {
	repository    repolock.Repository
	transactionID string
}

func (o recoveryPreviewOwnership) Owner() repolock.Owner {
	id := o.transactionID
	return repolock.Owner{RepositoryRoot: o.repository.Root, StorageMode: o.repository.Mode,
		StorageRoot: o.repository.StorageRoot(), TransactionID: &id}
}
func (o recoveryPreviewOwnership) Repository() repolock.Repository { return o.repository }
func (o recoveryPreviewOwnership) Capability() repolock.Capability {
	return repolock.Capability{Commands: []string{"recover"}}
}
func (o recoveryPreviewOwnership) Authorize(string, ...string) error { return nil }
func (o recoveryPreviewOwnership) IsDelegate() bool                  { return false }

func (s *Service) recoverWithTakeover(ctx context.Context, transactionID string, apply bool, request RecoverRequest) (result RecoverResult, err error) {
	if request.TakeOverLockID == "" || request.ExpectSHA256 == "" {
		return RecoverResult{}, invalidArgumentsf("--take-over-lock and --expect-sha256 must be supplied together")
	}
	if !transactionIDPattern.MatchString(request.TakeOverLockID) {
		return RecoverResult{}, invalidArgumentsf("lock id %q is not a lower-case 32-hex id", request.TakeOverLockID)
	}
	if !lockDigestPattern.MatchString(request.ExpectSHA256) {
		return RecoverResult{}, WithMachineErrorCode(MachineCodeInvalidDigest,
			fmt.Errorf("expected digest %q is not a lower-case 64-hex digest", request.ExpectSHA256))
	}
	if delegatedInvocation() {
		return RecoverResult{}, WithMachineErrorCode(MachineCodeDelegatedRefused,
			fmt.Errorf("delegated loop children cannot invoke recover"))
	}
	clear := repolock.ClearRequest{Repository: s.paths.LockRepository(), LockID: request.TakeOverLockID, ExpectSHA256: request.ExpectSHA256}
	owner, err := repolock.ObserveClear(clear)
	if err != nil {
		return RecoverResult{}, recoveryTakeoverError(err)
	}
	if owner.TransactionID == nil || *owner.TransactionID != transactionID {
		return RecoverResult{}, invalidArgumentsf("lock %s does not name transaction %s", request.TakeOverLockID, transactionID)
	}
	preview, err := s.runRecovery(ctx, recoveryPreviewOwnership{repository: s.paths.LockRepository(), transactionID: transactionID}, transactionID, false)
	if err != nil {
		return RecoverResult{}, s.mapRecoveryError(transactionID, err)
	}
	if !apply {
		return s.recoveryResult(preview, "preview", &request)
	}
	lock, err := repolock.TakeOver(ctx, clear, repolock.Request{
		Repository: s.paths.LockRepository(), Command: "recover", TransactionID: transactionID,
		Capability: repolock.Capability{Commands: []string{"recover"}},
	})
	if err != nil {
		return RecoverResult{}, recoveryTakeoverError(err)
	}
	defer func() {
		if releaseErr := lock.Release(); releaseErr != nil && err == nil {
			err = WithMachineFailure(MachineFailure{Code: MachineCodeRepositoryInvalid, Applied: result.Applied}, releaseErr)
		}
	}()
	confirmed, err := s.runRecovery(ctx, lock, transactionID, false)
	if err != nil {
		return RecoverResult{}, s.mapRecoveryError(transactionID, err)
	}
	if confirmed.Action != preview.Action || confirmed.Command != preview.Command || !slices.Equal(confirmed.Snapshots, preview.Snapshots) {
		return RecoverResult{}, WithMachineFailure(MachineFailure{Code: MachineCodeWriteConflict, Recovery: s.recoveryRef(transactionID)},
			fmt.Errorf("retained transaction changed between takeover preview and recovery ownership"))
	}
	recovered, err := s.runRecoveryExpected(ctx, lock, transactionID, true, &preview.Action)
	if err != nil {
		return RecoverResult{}, s.mapRecoveryError(transactionID, err)
	}
	return s.recoveryResult(recovered, "applied", &request)
}

func (s *Service) runRecovery(ctx context.Context, ownership durabletx.Ownership, transactionID string, apply bool) (durabletx.RecoveryResult, error) {
	return s.runRecoveryExpected(ctx, ownership, transactionID, apply, nil)
}

func (s *Service) runRecoveryExpected(ctx context.Context, ownership durabletx.Ownership, transactionID string, apply bool, expectedAction *durabletx.Action) (durabletx.RecoveryResult, error) {
	return durabletx.Recover(ctx, ownership, s.paths.LockRepository(), durabletx.RecoveryRequest{
		TransactionID:  transactionID,
		Apply:          apply,
		ExpectedAction: expectedAction,
		Validate:       s.recoveryValidator(transactionID),
	})
}

func (s *Service) recoveryValidator(transactionID string) func(string, []durabletx.Evidence) error {
	return func(command string, snapshots []durabletx.Evidence) error {
		switch command {
		case initMigrationCommand:
			return s.validateInitRecovery(transactionID, snapshots)
		case "review publish":
			return s.validateWorkflowPublicationRecovery(transactionID, snapshots)
		case "import":
			if !isRecoveredImport(snapshots, s.reportedStatePath(), s.paths.logicalManagedPath(s.paths.TasksDir)) {
				return errors.New("retained import transaction does not contain a state and task publication")
			}
			validation, err := s.Validate()
			if err != nil {
				return err
			}
			if !validation.Valid {
				return fmt.Errorf("recovered import candidate failed validation: %s", strings.Join(validation.Violations, "; "))
			}
			return nil
		case localPromoteCommand:
			return s.validateLocalPromotionRecovery(transactionID, snapshots)
		default:
			return fmt.Errorf("no recovery validator is registered for %q", command)
		}
	}
}

func (s *Service) validateLocalPromotionRecovery(transactionID string, snapshots []durabletx.Evidence) error {
	var files []localPromotionFile
	for _, snapshot := range snapshots {
		if snapshot.Kind != durabletx.Worktree {
			continue
		}
		if snapshot.Reported == markerRelPath() {
			if snapshot.FenceSHA256 == "" || snapshot.CandidateSHA256 == "" {
				return fmt.Errorf("local promotion transaction does not fence %s", markerRelPath())
			}
			marker, err := os.ReadFile(filepath.Join(s.paths.RepoRoot, filepath.FromSlash(markerRelPath())))
			if err != nil {
				return err
			}
			if digestBytes(marker) == snapshot.FenceSHA256 {
				marker, err = durabletx.RetainedCandidate(s.paths.LockRepository(), transactionID, durabletx.Worktree, markerRelPath())
				if err != nil {
					return err
				}
			}
			decoded, err := decodeLayoutMarkerStrict(marker)
			if err != nil || decoded.StorageMode != StorageCommitted || decoded.MigrationFence != nil || digestBytes(marker) != snapshot.CandidateSHA256 {
				return fmt.Errorf("retained local promotion marker is invalid")
			}
			continue
		}
		if strings.HasPrefix(snapshot.Reported, localStorageRoot+"/") && snapshot.CandidateSHA256 == "" {
			files = append(files, localPromotionFile{Source: snapshot.Reported})
		}
	}
	if len(files) == 0 {
		return fmt.Errorf("local promotion transaction records no local semantic removals")
	}
	return s.validateLocalPromotionCandidate(localPromotionCandidate{files: files})
}

func isRecoveredImport(snapshots []durabletx.Evidence, statePath, tasksDir string) bool {
	hasState, hasTask := false, false
	prefix := strings.TrimSuffix(tasksDir, "/") + "/"
	for _, snapshot := range snapshots {
		if snapshot.Reported == statePath && snapshot.CandidateSHA256 != "" {
			hasState = true
		}
		if strings.HasPrefix(snapshot.Reported, prefix) && snapshot.CandidateSHA256 != "" {
			hasTask = true
		}
	}
	return hasState && hasTask
}

func (s *Service) recoveryResult(recovered durabletx.RecoveryResult, takeover string, request *RecoverRequest) (RecoverResult, error) {
	validation, err := s.recoveredValidation(recovered)
	if err != nil {
		return RecoverResult{}, err
	}
	result := RecoverResult{TransactionID: recovered.TransactionID, Command: recovered.Command,
		Action: string(recovered.Action), Applied: recovered.Applied, Takeover: takeover,
		Snapshots: recoverySnapshots(recovered.Snapshots), Validation: validation}
	if request != nil {
		result.TakeOverLockID = &request.TakeOverLockID
		result.TakeOverSHA256 = &request.ExpectSHA256
	}
	return result, nil
}

func recoveryTakeoverError(err error) error {
	switch {
	case errors.Is(err, repolock.ErrLiveOwner), errors.Is(err, repolock.ErrHeld), errors.Is(err, repolock.ErrSameProcess):
		return recoveryLockError(err)
	case errors.Is(err, repolock.ErrChanged):
		return WithMachineErrorCode(MachineCodeSourceChanged, err)
	case errors.Is(err, repolock.ErrNotHeld):
		return WithMachineErrorCode(MachineCodeInvalidDigest, err)
	case errors.Is(err, repolock.ErrMalformed):
		return WithMachineErrorCode(MachineCodeRepositoryInvalid, err)
	default:
		return err
	}
}

// recoveredValidation reports the coherence of the state a recovery leaves
// behind. A completed init migration publishes state schema 2, which this
// binary's schema-1 Validate cannot yet read as valid, so that one outcome
// validates through the same strict layout-2 readers the migration itself
// publishes under.
func (s *Service) recoveredValidation(recovered durabletx.RecoveryResult) (ValidationResult, error) {
	if recovered.Applied && recovered.Command == localPromoteCommand && recovered.Action == durabletx.AcceptCandidate && s.paths.Storage.Mode == StorageLocal {
		return s.localPromotionCommittedService().Validate()
	}
	if !recovered.Applied || recovered.Command != initMigrationCommand || recovered.Action != durabletx.AcceptCandidate {
		return s.Validate()
	}
	if s.paths.Storage.Mode == StorageLocal {
		return s.Validate()
	}
	violations := strictLayout2Violations(s.paths.RepoRoot, s.paths.LogicalPlanningDir)
	return ValidationResult{Valid: len(violations) == 0, Violations: violations}, nil
}

// recoverySnapshots projects the engine's whole-set evidence onto the machine
// contract's typed snapshots, in the contract's deterministic order. Managed
// members keep their logical spelling in every storage mode, so a physical
// overlay location is never published as durable semantic data.
func recoverySnapshots(evidence []durabletx.Evidence) []MachineSnapshot {
	out := make([]MachineSnapshot, 0, len(evidence))
	for _, item := range evidence {
		out = append(out, MachineSnapshot{
			PathKind:        string(item.Kind),
			Path:            item.Reported,
			OriginalSHA256:  optionalDigest(item.OriginalSHA256),
			CandidateSHA256: optionalDigest(item.CandidateSHA256),
			CurrentSHA256:   optionalDigest(item.CurrentSHA256),
		})
	}
	slices.SortStableFunc(out, func(a, b MachineSnapshot) int {
		return slices.Compare(snapshotOrderKey(a), snapshotOrderKey(b))
	})
	return out
}

func optionalDigest(value string) *string {
	if value == "" {
		return nil
	}
	digest := value
	return &digest
}

// mapRecoveryError classifies an engine refusal onto the command's registered
// error subset, carrying the exact typed snapshots and the recovery reference
// so the evidence survives the refusal. A leaf rewritten with its recorded
// bytes through a new identity is reported as a violation rather than folded
// into the action; the whole-set ancestor checks already refuse real
// substitutions.
func (s *Service) mapRecoveryError(transactionID string, err error) error {
	var txErr *durabletx.Error
	if errors.As(err, &txErr) {
		code, ok := txErr.MachineCode()
		if !ok {
			code = MachineCodeRepositoryInvalid
		}
		failure := MachineFailure{
			Code:      code,
			Snapshots: recoverySnapshots(txErr.Snapshots()),
			Recovery:  s.recoveryRef(transactionID),
		}
		for _, snapshot := range txErr.Snapshots() {
			if !snapshot.IdentityChanged {
				continue
			}
			path := snapshot.Reported
			failure.Violations = append(failure.Violations, MachineViolation{
				Code:    "identity_substituted",
				Message: fmt.Sprintf("%s path was rewritten through a new file identity", snapshot.Kind),
				Path:    &path,
			})
		}
		return WithMachineFailure(failure, err)
	}
	retained, retainedErr := transactionRetained(s.paths, transactionID)
	if retainedErr == nil && !retained {
		return invalidArgumentsf("no retained transaction %s is present", transactionID)
	}
	return WithMachineErrorCode(MachineCodeRepositoryInvalid,
		fmt.Errorf("retained transaction %s evidence is unreadable: %w", transactionID, err))
}

// recoveryRef names the retained transaction a refusal leaves behind, using the
// admission boundary's own strict derivation so the reference never invents
// journal facts malformed state cannot support.
func (s *Service) recoveryRef(transactionID string) *MachineRecoveryRef {
	snapshot, err := observeRecovery(s.paths)
	if err != nil {
		return nil
	}
	ref := canonicalRecovery(s.paths, snapshot)
	if ref == nil || ref.TransactionID != transactionID {
		return nil
	}
	return ref
}

// transactionRetained reports whether any retained state names transactionID:
// its journal directory or one of its fence markers beneath the shared
// transactions root. It distinguishes an id that names nothing from evidence
// that exists but cannot be read, which the error classification above needs.
func transactionRetained(paths Paths, transactionID string) (bool, error) {
	snapshot, err := observeRecovery(paths)
	if err != nil {
		return false, err
	}
	for _, entry := range snapshot.Entries {
		if strings.HasPrefix(entry.Path, transactionID+"/") || strings.HasPrefix(entry.Path, transactionID+".") {
			return true, nil
		}
	}
	return false, nil
}

// recoveryLockError maps lock acquisition refusals onto the command's
// registered codes. Any holder — live or abandoned — refuses, because the lock
// a transaction named is part of its identity; the operator path for an
// abandoned holder is the guarded lock surface, never this command.
func recoveryLockError(err error) error {
	switch {
	case errors.Is(err, repolock.ErrHeld), errors.Is(err, repolock.ErrSameProcess), errors.Is(err, repolock.ErrLiveOwner):
		return WithMachineErrorCode(MachineCodeLockHeld, fmt.Errorf(
			"the repository mutation lock is held: inspect it with taskrail lock status and use its exact --take-over-lock and --expect-sha256 operands before recovering: %w", err))
	case errors.Is(err, repolock.ErrMalformed):
		return WithMachineErrorCode(MachineCodeRepositoryInvalid,
			fmt.Errorf("repository mutation lock metadata is unreadable: %w", err))
	default:
		return err
	}
}
