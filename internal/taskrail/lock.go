package taskrail

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/tessariq/taskrail/internal/repolock"
)

// The operator lock surface over the repository mutation lock protocol
// (specs/v0.5.0.md#repository-discovery-locking-and-recovery): read-only
// inspection and a guarded compare-and-delete for abandoned locks. PID, host,
// and age are evidence, never a lease, so nothing here clears a lock by
// itself; `lock clear` acts only on the operator's exact observation and still
// refuses an owner provably live on this host.

// LockStatusResult is the companion's LockStatusResult: absence, or the exact
// owner metadata and the raw lock-file digest the operator compares against
// when clearing.
type LockStatusResult struct {
	Held   bool            `json:"held"`
	SHA256 *string         `json:"sha256"`
	Owner  *repolock.Owner `json:"owner"`
}

// LockClearResult is the companion's LockClearResult. Cleared is a boolean
// that is only ever published as true: a clear that does not happen is an
// error envelope, not a false result.
type LockClearResult struct {
	LockID      string `json:"lock_id"`
	Cleared     bool   `json:"cleared"`
	PriorSHA256 string `json:"prior_sha256"`
}

// lockDigestPattern is the lower-case 64-hex SHA-256 grammar lock digests use.
var lockDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// lockRepository maps the discovered storage context onto the lock protocol's
// explicit repository: the same roots, mode, and Git-common identity every
// writer's lock resolves through, so inspection and clearing always address
// the lock a writer would contend with.
func (s *Service) lockRepository() repolock.Repository {
	mode := repolock.ModeCommitted
	if s.paths.Storage.Mode == StorageLocal {
		mode = repolock.ModeLocal
	}
	return repolock.Repository{Root: s.paths.RepoRoot, GitCommonDir: s.paths.GitCommonDir, Mode: mode}
}

// LockStatus reports the repository mutation lock without taking it and
// without writing anything. Unreadable lock metadata is a refusal — the bytes
// stay exactly where they are for the operator to inspect.
func (s *Service) LockStatus() (LockStatusResult, error) {
	if err := s.checkLockStatusRecovery(); err != nil {
		return LockStatusResult{}, err
	}
	status, err := repolock.Inspect(s.lockRepository())
	if err != nil {
		if errors.Is(err, repolock.ErrMalformed) {
			return LockStatusResult{}, WithMachineErrorCode(MachineCodeRepositoryInvalid,
				fmt.Errorf("repository mutation lock metadata is unreadable: %w", err))
		}
		return LockStatusResult{}, err
	}
	result := LockStatusResult{Held: status.Held}
	if status.Held {
		result.SHA256 = &status.SHA256
		result.Owner = status.Owner
	}
	if err := s.checkLockStatusRecovery(); err != nil {
		return LockStatusResult{}, err
	}
	return result, nil
}

// checkLockStatusRecovery admits only a stable, canonical retained fence. It
// lets status expose the lock blocking recovery while refusing malformed or
// changing recovery state before it can publish a stale observation.
func (s *Service) checkLockStatusRecovery() error {
	current, err := observeRecovery(s.paths)
	if err != nil {
		return recoveryPending(s.paths, current)
	}
	if recoveryRetained(current) && canonicalRecovery(s.paths, current) == nil {
		return recoveryPending(s.paths, current)
	}
	return nil
}

// LockClear removes exactly the named, unchanged lock the operator observed.
// Refusals map onto the command's registered codes: malformed operands are
// argument failures (the digest operand's own grammar is invalid_digest), an
// absent lock means the expected digest names nothing present, bytes that
// moved on since the observation are source_changed, and a provably live
// same-host owner is lock_held.
func (s *Service) LockClear(lockID, expectSHA256 string) (LockClearResult, error) {
	if !transactionIDPattern.MatchString(lockID) {
		return LockClearResult{}, invalidArgumentsf("lock id %q is not a lower-case 32-hex id", lockID)
	}
	if !lockDigestPattern.MatchString(expectSHA256) {
		return LockClearResult{}, WithMachineErrorCode(MachineCodeInvalidDigest,
			fmt.Errorf("expected digest %q is not a lower-case 64-hex digest", expectSHA256))
	}
	prior, err := repolock.Clear(repolock.ClearRequest{
		Repository:   s.lockRepository(),
		LockID:       lockID,
		ExpectSHA256: expectSHA256,
	})
	if err != nil {
		return LockClearResult{}, mapLockClearError(err)
	}
	return LockClearResult{LockID: lockID, Cleared: true, PriorSHA256: prior}, nil
}

func mapLockClearError(err error) error {
	switch {
	case errors.Is(err, repolock.ErrNotHeld):
		return WithMachineErrorCode(MachineCodeInvalidDigest,
			fmt.Errorf("no repository mutation lock is present, so the expected digest names nothing: %w", err))
	case errors.Is(err, repolock.ErrChanged):
		return WithMachineErrorCode(MachineCodeSourceChanged, err)
	case errors.Is(err, repolock.ErrLiveOwner):
		return WithMachineErrorCode(MachineCodeLockHeld, err)
	case errors.Is(err, repolock.ErrMalformed):
		return WithMachineErrorCode(MachineCodeRepositoryInvalid,
			fmt.Errorf("repository mutation lock metadata is unreadable: %w", err))
	default:
		return err
	}
}
