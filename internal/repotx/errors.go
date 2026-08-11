package repotx

import (
	"fmt"
	"strings"
)

// Kind names one handled transaction outcome. Each kind answers the question an
// agent actually has after a failure — is anything of mine on disk, and whose
// bytes are there now — which is why rollback success and rollback failure are
// different kinds rather than one "write failed".
type Kind string

const (
	// KindRefused is a write outside this ownership's bound. Nothing was read or
	// written; the refusal happens before the snapshot.
	KindRefused Kind = "refused"
	// KindOutsideRepository is a path that does not resolve inside the locked
	// repository, whether lexically before the snapshot or through a symlinked
	// directory component at publication.
	KindOutsideRepository Kind = "outside_repository"
	// KindNotRegularFile is a path the transaction found occupied by something it
	// cannot snapshot or replace — a directory, a symlink, a device. It is
	// separate from KindOutsideRepository because it is discovered mid-scan and
	// therefore carries the evidence collected so far.
	KindNotRegularFile Kind = "not_regular_file"
	// KindUnreadable is a path the transaction could not observe at all:
	// permission denied, an I/O error, a symlink loop. An absent path is not one
	// of these — absence is a legitimate state the snapshot records.
	KindUnreadable Kind = "unreadable"
	// KindValidation is a candidate the writer's own validation rejected. It is
	// raised before the first write, so the repository is untouched.
	KindValidation Kind = "validation"
	// KindConflict is an external edit found between the snapshot and the first
	// write. The transaction publishes nothing rather than overwrite it.
	KindConflict Kind = "conflict"
	// KindRolledBack is a publication failure this transaction fully undid. The
	// repository holds exactly its original bytes.
	KindRolledBack Kind = "rolled_back"
	// KindRollbackFailed is a publication failure this transaction could not
	// fully undo, because a published path was externally changed or a restore
	// itself failed. Preserved names what was left alone.
	KindRollbackFailed Kind = "rollback_failed"
)

// Error is one handled transaction failure plus the exact byte evidence needed
// to tell what is on disk. A transaction that committed returns a Result
// instead, so an Error always means the command's semantic operation did not
// apply — even when Preserved shows bytes that are still there.
type Error struct {
	Kind Kind
	// Preserved names the published reported paths a rollback deliberately left
	// alone because their current bytes were no longer this transaction's
	// candidate. It is empty for every kind but KindRollbackFailed.
	Preserved []string

	snapshots []Snapshot
	err       error
}

// Snapshots is the deterministic per-path evidence, ordered by path kind and
// then path. Each entry carries exact original, candidate, and current digests.
// A refusal that happened before any path was read reports no evidence rather
// than a null one, matching the machine contract's empty-array rule.
func (e *Error) Snapshots() []Snapshot {
	if e.snapshots == nil {
		return []Snapshot{}
	}
	return e.snapshots
}

// MachineCode is the registered v0.5 error code this failure publishes as, and
// whether the transaction has one at all.
//
// KindRolledBack has none, and the second return value says so rather than
// leaving an empty string to be published as a code: the repository is back to
// its original bytes, so what failed is the caller's own operation — a
// cancelled run, an unwritable disk — and only the caller knows which registered
// code names it. Every other kind is a fact about the transaction itself and
// carries its own code.
func (e *Error) MachineCode() (string, bool) {
	switch e.Kind {
	case KindRefused:
		return "delegated_write_refused", true
	case KindOutsideRepository, KindNotRegularFile, KindUnreadable:
		return "repository_invalid", true
	case KindValidation:
		return "validation_failed", true
	case KindConflict:
		return "write_conflict", true
	case KindRollbackFailed:
		return "rollback_failed", true
	default:
		return "", false
	}
}

func (e *Error) Error() string {
	if len(e.Preserved) == 0 {
		return fmt.Sprintf("transaction %s: %v", e.Kind, e.err)
	}
	return fmt.Sprintf("transaction %s (preserved %s): %v",
		e.Kind, strings.Join(e.Preserved, ", "), e.err)
}

func (e *Error) Unwrap() error { return e.err }

func failure(kind Kind, snapshots []Snapshot, err error) *Error {
	return &Error{Kind: kind, snapshots: snapshots, err: err}
}
