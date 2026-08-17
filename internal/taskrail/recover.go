package taskrail

import (
	"context"
	"errors"
	"fmt"
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
	TransactionID string            `json:"transaction_id"`
	Command       string            `json:"command"`
	Action        string            `json:"action"`
	Applied       bool              `json:"applied"`
	Snapshots     []MachineSnapshot `json:"snapshots"`
	Validation    ValidationResult  `json:"validation"`
}

// NewRecoveryService discovers the repository for the recovery boundary itself.
// Unlike NewService it admits retained transaction state, because `recover` is
// the one command that state exists to be handed to.
func NewRecoveryService(start string) (*Service, error) {
	paths, err := DiscoverPaths(start)
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
func (s *Service) RecoverTransaction(ctx context.Context, transactionID string, apply bool) (result RecoverResult, err error) {
	if !transactionIDPattern.MatchString(transactionID) {
		return RecoverResult{}, invalidArgumentsf("transaction id %q is not a lower-case 32-hex id", transactionID)
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

	// No durable writer has shipped its recovery validator yet, so none is
	// supplied here: the engine refuses accept_candidate for a command whose
	// owning writer has not declared one, which is the exact contract — this
	// boundary never chooses semantic content on a writer's behalf. Each durable
	// writer task registers its validator when it wires publication.
	recovered, err = durabletx.Recover(ctx, lock, s.paths.LockRepository(), durabletx.RecoveryRequest{
		TransactionID: transactionID,
		Apply:         apply,
	})
	if err != nil {
		return RecoverResult{}, s.mapRecoveryError(transactionID, err)
	}

	validation, err := s.Validate()
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
		Snapshots:     recoverySnapshots(recovered.Snapshots),
		Validation:    validation,
	}, nil
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
	case errors.Is(err, repolock.ErrHeld), errors.Is(err, repolock.ErrSameProcess):
		return WithMachineErrorCode(MachineCodeLockHeld, fmt.Errorf(
			"the repository mutation lock is held: inspect it with taskrail lock status and clear an abandoned owner with taskrail lock clear before recovering: %w", err))
	case errors.Is(err, repolock.ErrMalformed):
		return WithMachineErrorCode(MachineCodeRepositoryInvalid,
			fmt.Errorf("repository mutation lock metadata is unreadable: %w", err))
	default:
		return err
	}
}
