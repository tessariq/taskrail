package durabletx

import "fmt"

// Kind classifies a handled durable transaction outcome.
type Kind string

const (
	KindRefused        Kind = "refused"
	KindConflict       Kind = "conflict"
	KindValidation     Kind = "validation"
	KindRolledBack     Kind = "rolled_back"
	KindRollbackFailed Kind = "rollback_failed"
	KindRecovery       Kind = "recovery_pending"
)

// Error carries deterministic whole-set evidence for a handled failure.
type Error struct {
	Kind          Kind
	TransactionID string
	Phase         Phase
	Action        Action
	Preserved     []string
	snapshots     []Evidence
	err           error
}

func (e *Error) Error() string {
	return fmt.Sprintf("durable transaction %s at %s: %v", e.Kind, e.Phase, e.err)
}

func (e *Error) Unwrap() error { return e.err }

func (e *Error) Snapshots() []Evidence {
	return append([]Evidence(nil), e.snapshots...)
}

// MachineCode returns the registered v0.5 error code for handled outcomes that
// have one. A fully rolled-back caller error remains the caller's code.
func (e *Error) MachineCode() (string, bool) {
	switch e.Kind {
	case KindRefused:
		return "delegated_write_refused", true
	case KindConflict:
		return "write_conflict", true
	case KindValidation:
		return "validation_failed", true
	case KindRollbackFailed:
		return "rollback_failed", true
	case KindRecovery:
		return "recovery_pending", true
	default:
		return "", false
	}
}

func failure(kind Kind, id string, phase Phase, snapshots []Evidence, err error) *Error {
	return &Error{Kind: kind, TransactionID: id, Phase: phase, snapshots: snapshots, err: err}
}
