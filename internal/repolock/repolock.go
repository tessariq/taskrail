// Package repolock implements the repository mutation lock protocol: one
// exclusive lock every Taskrail semantic writer holds for its whole transaction
// (specs/v0.5.0.md#repository-discovery-locking-and-recovery). A Git worktree
// places it beneath the Git common directory so linked worktrees coordinate; a
// discovered non-Git root places it beneath its own `.taskrail/runtime/`.
//
// Read-only callers never take the lock. The protocol coordinates processes, not
// clones: the common directory is not a distributed lease, so an abandoned lock
// is never auto-cleared by PID, host, or age — inspection and guarded clearing
// are operator surfaces (`lock status`, `lock clear`).
//
// The primitives take an explicitly supplied Repository. Discovering one, and
// routing commands through it, belong to the storage-discovery outcomes.
package repolock

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const (
	// gitLockDirName is the Git-common-directory subdirectory Taskrail owns. Git
	// itself never writes here, so a stray entry is always ours.
	gitLockDirName = "taskrail"
	lockFileName   = "mutation.lock"
	// lockFileMode keeps the record world-readable: inspecting a lock is a
	// read-only operation any user sharing the repository may need to perform.
	lockFileMode = 0o644
)

// localStorageDir is the fixed physical root local mode maps the committed
// logical namespaces onto. Deriving it from the repository root keeps a caller
// from supplying a storage root that disagrees with its own mode.
var localStorageDir = filepath.Join(".taskrail", "local")

var (
	// ErrHeld reports that another owner holds the lock. The caller refuses; it
	// never removes or rewrites the record it found.
	ErrHeld = errors.New("repository mutation lock is held")
	// ErrSameProcess reports a second acquisition from the process that already
	// holds this lock — misuse of the protocol rather than contention.
	ErrSameProcess = errors.New("repository mutation lock is already held by this process")
	// ErrNotHeld reports that no lock exists where one was required.
	ErrNotHeld = errors.New("repository mutation lock is not held")
	// ErrMalformed reports unreadable or semantically invalid lock metadata. It
	// is always a refusal: corrupt bytes are evidence an operator inspects, not
	// permission to take the lock.
	ErrMalformed = errors.New("repository mutation lock metadata is malformed")
	// ErrRefused reports a delegated join that failed one of its identity,
	// secrecy, or capability checks.
	ErrRefused = errors.New("delegated join refused")
	// ErrReleased reports use of a handle whose lock has already been released.
	ErrReleased = errors.New("repository mutation lock has been released")
	// ErrNotDelegated reports a delegation request against a lock that was not
	// acquired for delegation.
	ErrNotDelegated = errors.New("repository mutation lock was not acquired for delegation")
)

// StorageMode is where a repository's managed bytes physically live. It is
// recorded in lock metadata so a delegate cannot join across a mode boundary.
type StorageMode string

const (
	ModeCommitted StorageMode = "committed"
	ModeLocal     StorageMode = "local"
)

// Repository is the explicitly supplied repository context a lock coordinates.
// Root is the logical repository identity and is the same value in both storage
// modes; the storage root is derived from Root and Mode so the two can never
// disagree. GitCommonDir is empty for a discovered non-Git root.
type Repository struct {
	Root         string
	GitCommonDir string
	Mode         StorageMode
}

// StorageRoot is the physical root of managed Taskrail storage. Committed mode
// stores managed bytes at the repository root; local mode maps the same logical
// namespaces beneath `.taskrail/local`.
func (r Repository) StorageRoot() string {
	if r.Mode == ModeLocal {
		return filepath.Join(r.Root, localStorageDir)
	}
	return r.Root
}

// validate refuses a context that could resolve two different locks for one
// repository, or one lock for two repositories. Absolute paths are required so
// lock identity never depends on the invocation directory.
func (r Repository) validate() error {
	if r.Root == "" || !filepath.IsAbs(r.Root) {
		return fmt.Errorf("repository root %q must be an absolute path", r.Root)
	}
	if r.GitCommonDir != "" && !filepath.IsAbs(r.GitCommonDir) {
		return fmt.Errorf("git common directory %q must be an absolute path", r.GitCommonDir)
	}
	if r.Mode != ModeCommitted && r.Mode != ModeLocal {
		return fmt.Errorf("unknown storage mode %q", r.Mode)
	}
	// Local mode's contract requires a non-bare worktree, so a local context
	// without a Git common directory is not a repository Taskrail supports.
	if r.Mode == ModeLocal && r.GitCommonDir == "" {
		return errors.New("local storage mode requires a Git worktree")
	}
	return nil
}

// validateClaim runs the checks a lock claim and a delegated join share: a
// repository that resolves exactly one lock, a capability that bounds something,
// and a command inside the capability the caller declared for itself.
func validateClaim(repo Repository, command string, capability Capability) error {
	if err := repo.validate(); err != nil {
		return err
	}
	if command == "" {
		return errors.New("claim names no command")
	}
	if err := capability.validate(); err != nil {
		return err
	}
	return capability.Allows(command)
}

// LockPath is where repo's mutation lock lives. It is a pure function of the
// supplied roots, so every caller in a repository — including one running from a
// linked worktree or an arbitrary subdirectory — resolves the same file.
func LockPath(repo Repository) string {
	if repo.GitCommonDir != "" {
		return filepath.Join(repo.GitCommonDir, gitLockDirName, lockFileName)
	}
	return filepath.Join(repo.Root, ".taskrail", "runtime", lockFileName)
}

// Owner is the exact lock metadata the protocol records. Nullable fields stay
// null rather than empty so a reader can tell "not delegated" from "delegated
// with an empty digest". The delegation token itself is never persisted — only
// its digest — so reading the lock file grants no authority.
type Owner struct {
	LockID           string      `json:"lock_id"`
	Command          string      `json:"command"`
	PID              int         `json:"pid"`
	Host             string      `json:"host"`
	StartedAt        string      `json:"started_at"`
	RepositoryRoot   string      `json:"repository_root"`
	StorageMode      StorageMode `json:"storage_mode"`
	StorageRoot      string      `json:"storage_root"`
	TransactionID    *string     `json:"transaction_id"`
	ExecutableSHA256 *string     `json:"executable_sha256"`
	DelegationDigest *string     `json:"delegation_digest"`
}

var (
	lockIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
	digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// validate rejects a record no Taskrail acquisition could have written. A
// half-written or hand-edited lock must fail closed: the caller refuses to
// acquire and leaves the bytes for an operator, rather than treating "I cannot
// read this" as "nobody holds it".
func (o Owner) validate() error {
	switch {
	case !lockIDPattern.MatchString(o.LockID):
		return fmt.Errorf("lock_id %q is not a lower-case 32-hex id", o.LockID)
	case o.Command == "":
		return errors.New("command is empty")
	case o.PID <= 0:
		return fmt.Errorf("pid %d is not a process id", o.PID)
	case o.Host == "":
		return errors.New("host is empty")
	case !filepath.IsAbs(o.RepositoryRoot):
		return fmt.Errorf("repository_root %q is not absolute", o.RepositoryRoot)
	case !filepath.IsAbs(o.StorageRoot):
		return fmt.Errorf("storage_root %q is not absolute", o.StorageRoot)
	case o.StorageMode != ModeCommitted && o.StorageMode != ModeLocal:
		return fmt.Errorf("unknown storage_mode %q", o.StorageMode)
	}
	if _, err := time.Parse(time.RFC3339, o.StartedAt); err != nil {
		return fmt.Errorf("started_at %q is not RFC3339", o.StartedAt)
	}
	if o.ExecutableSHA256 != nil && !digestPattern.MatchString(*o.ExecutableSHA256) {
		return fmt.Errorf("executable_sha256 %q is not a lower-case 64-hex digest", *o.ExecutableSHA256)
	}
	if o.DelegationDigest != nil && !digestPattern.MatchString(*o.DelegationDigest) {
		return fmt.Errorf("delegation_digest %q is not a lower-case 64-hex digest", *o.DelegationDigest)
	}
	return nil
}

// ExecutableDigest is the SHA-256 of an executable's bytes, the identity a
// delegating owner binds its children to. It streams the file so a large binary
// costs no more memory than a small one.
func ExecutableDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read executable %s: %w", path, err)
	}
	defer file.Close()

	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", fmt.Errorf("hash executable %s: %w", path, err)
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// randomHex returns n cryptographically random bytes as lower-case hex. Lock ids
// only need to be unique, but delegation tokens must also be unguessable, so
// both come from the same CSPRNG rather than from a clock or a counter.
func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
