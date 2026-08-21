package taskrail

import (
	"errors"
	"fmt"
	"io/fs"
)

// A command failure arrives at the machine boundary as an ordinary Go error, but
// the v0.5 envelope needs one registered error code for it. The site that knows
// why the failure happened tags the code here and the producer reads it back, so
// no layer has to re-derive a contract code by matching failure prose.

// The registered common error codes this package tags failures with. They are
// held to the closed registry by TestMachineCodesAreRegistered.
const (
	MachineCodeInvalidArguments    = "invalid_arguments"
	MachineCodeNotInitialized      = "not_initialized"
	MachineCodeIncompatibleLayout  = "incompatible_layout"
	MachineCodeMigrationInProgress = "migration_in_progress"
	MachineCodeRepositoryInvalid   = "repository_invalid"
	MachineCodeValidationFailed    = "validation_failed"
	MachineCodeTaskNotFound        = "task_not_found"
	MachineCodeReviewNotFound      = "review_not_found"
	MachineCodePromptNotFound      = "prompt_not_found"
	MachineCodePromptInvalid       = "prompt_invalid"
	MachineCodeInvalidStatus       = "invalid_status"
	MachineCodeInvalidReason       = "invalid_reason"
	MachineCodeInvalidDigest       = "invalid_digest"
	MachineCodeSourceChanged       = "source_changed"
	MachineCodeInvalidProposal     = "invalid_proposal"
	MachineCodeDestinationExists   = "destination_exists"
	MachineCodePathBlocked         = "path_blocked"
	MachineCodeLockHeld            = "lock_held"
	MachineCodeDelegatedRefused    = "delegated_write_refused"
	MachineCodeDependencyExists    = "dependency_exists"
	MachineCodeDependencyAbsent    = "dependency_absent"
	MachineCodeDependencyCycle     = "dependency_cycle"
	MachineCodeCancelledDependency = "cancelled_dependency"
	MachineCodeWriteConflict       = "write_conflict"
	MachineCodeRecoveryPending     = "recovery_pending"
	MachineCodePartialWrite        = "partial_write"
	MachineCodeRollbackFailed      = "rollback_failed"
	MachineCodeUnsupported         = "unsupported"
)

// MachineFailure is what a failing writer knows about its own outcome beyond the
// message: which registered code names it, whether the complete semantic
// operation still committed, and which managed paths the agent has to look at.
type MachineFailure struct {
	Code string
	// Applied reports that the complete semantic operation committed. A refusal
	// and a write that landed only partly are both false; only a failure after
	// the operation itself committed — re-running validation, for instance — is
	// true.
	Applied bool
	// Paths are the managed paths the failure implicates, such as the artifacts a
	// partial write left behind.
	Paths []string
	// Violations carry subject-specific facts about the failure, which the
	// companion places in the error details rather than in ad hoc members.
	Violations []MachineViolation
	// Snapshots carry normal-transaction byte evidence in deterministic order.
	Snapshots []MachineSnapshot
	// Recovery is present only when retained state strictly identifies one
	// canonical transaction. Malformed retained state still fails closed without
	// inventing journal facts.
	Recovery *MachineRecoveryRef
}

// machineCodedError carries a failure's machine facts without changing its
// message, so human mode keeps its exact wording and JSON mode gains the
// registered code, `applied`, and paths beside it.
type machineCodedError struct {
	failure MachineFailure
	err     error
}

func (e machineCodedError) Error() string { return e.err.Error() }

func (e machineCodedError) Unwrap() error { return e.err }

// WithMachineErrorCode tags err with the registered common error code its
// command would publish for it. A nil error stays nil so call sites can tag
// inline.
func WithMachineErrorCode(code string, err error) error {
	return WithMachineFailure(MachineFailure{Code: code}, err)
}

// WithMachineFailure tags err with everything its command's error envelope
// reports about the outcome. A nil error stays nil so call sites can tag inline.
func WithMachineFailure(failure MachineFailure, err error) error {
	if err == nil {
		return nil
	}
	return machineCodedError{failure: failure, err: err}
}

// MachineFailureFor returns the machine facts a failure was tagged with. An
// untagged failure is repository_invalid: the command discovered a repository and
// then could not read it as Taskrail state, which is the one conclusion that
// holds without knowing more, and which every command's error subset admits.
func MachineFailureFor(err error) MachineFailure {
	var coded machineCodedError
	if errors.As(err, &coded) {
		return coded.failure
	}
	return MachineFailure{Code: MachineCodeRepositoryInvalid}
}

// invalidArgumentsf builds a rejection of an operand the caller chose, already
// carrying the code its command's envelope publishes for one.
func invalidArgumentsf(format string, a ...any) error {
	return WithMachineErrorCode(MachineCodeInvalidArguments, fmt.Errorf(format, a...))
}

// missingOrInvalidCode classifies a managed-file read failure: absence means the
// repository is not initialized as far as this command needs, and any other
// failure means the file exists but the repository cannot be read.
func missingOrInvalidCode(err error, missing string) string {
	if errors.Is(err, fs.ErrNotExist) {
		return missing
	}
	return MachineCodeRepositoryInvalid
}
