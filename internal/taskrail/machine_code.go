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
	MachineCodeInvalidArguments   = "invalid_arguments"
	MachineCodeNotInitialized     = "not_initialized"
	MachineCodeIncompatibleLayout = "incompatible_layout"
	MachineCodeRepositoryInvalid  = "repository_invalid"
)

// machineCodedError carries a registered error code without changing the
// failure's message, so human mode keeps its exact wording and JSON mode gains
// the code beside it.
type machineCodedError struct {
	code string
	err  error
}

func (e machineCodedError) Error() string { return e.err.Error() }

func (e machineCodedError) Unwrap() error { return e.err }

// WithMachineErrorCode tags err with the registered common error code its
// command would publish for it. A nil error stays nil so call sites can tag
// inline.
func WithMachineErrorCode(code string, err error) error {
	if err == nil {
		return nil
	}
	return machineCodedError{code: code, err: err}
}

// invalidArgumentsf builds a rejection of an operand the caller chose, already
// carrying the code its command's envelope publishes for one.
func invalidArgumentsf(format string, a ...any) error {
	return WithMachineErrorCode(MachineCodeInvalidArguments, fmt.Errorf(format, a...))
}

// MachineErrorCodeFor returns the code a failure was tagged with. An untagged
// failure is repository_invalid: the command discovered a repository and then
// could not read it as Taskrail state, which is the one conclusion that holds
// without knowing more, and which every command's error subset admits.
func MachineErrorCodeFor(err error) string {
	var coded machineCodedError
	if errors.As(err, &coded) {
		return coded.code
	}
	return MachineCodeRepositoryInvalid
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
