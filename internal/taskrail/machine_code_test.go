package taskrail

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"testing"
)

func TestMachineCodesAreRegistered(t *testing.T) {
	for _, code := range []string{
		MachineCodeInvalidArguments,
		MachineCodeNotInitialized,
		MachineCodeIncompatibleLayout,
		MachineCodeRepositoryInvalid,
	} {
		if !slices.Contains(machineErrorCodes, code) {
			t.Errorf("tagged code %q is outside the closed error registry", code)
		}
	}
}

func TestWithMachineErrorCodePreservesTheFailure(t *testing.T) {
	cause := errors.New("read state file planning/STATE.md: no such file or directory")
	tagged := WithMachineErrorCode(MachineCodeNotInitialized, cause)

	if tagged.Error() != cause.Error() {
		t.Errorf("tagged message is %q, want the untagged %q", tagged, cause)
	}
	if !errors.Is(tagged, cause) {
		t.Error("tagged error no longer unwraps to its cause")
	}
	if got := MachineErrorCodeFor(tagged); got != MachineCodeNotInitialized {
		t.Errorf("code is %q, want %q", got, MachineCodeNotInitialized)
	}
	if got := MachineErrorCodeFor(fmt.Errorf("coverage: %w", tagged)); got != MachineCodeNotInitialized {
		t.Errorf("code through a wrapping layer is %q, want %q", got, MachineCodeNotInitialized)
	}
}

func TestWithMachineErrorCodeKeepsNilNil(t *testing.T) {
	if err := WithMachineErrorCode(MachineCodeInvalidArguments, nil); err != nil {
		t.Errorf("tagging nil produced %v", err)
	}
}

// An untagged failure must still name a code every command's error subset
// admits, so a producer can never publish an unregistered one.
func TestMachineErrorCodeForUntaggedFailure(t *testing.T) {
	if got := MachineErrorCodeFor(errors.New("boom")); got != MachineCodeRepositoryInvalid {
		t.Errorf("untagged code is %q, want %q", got, MachineCodeRepositoryInvalid)
	}
}

func TestMissingOrInvalidCodeSeparatesAbsenceFromUnreadability(t *testing.T) {
	absent := &fs.PathError{Op: "open", Path: "planning/STATE.md", Err: fs.ErrNotExist}
	if got := missingOrInvalidCode(absent, MachineCodeNotInitialized); got != MachineCodeNotInitialized {
		t.Errorf("absent file code is %q, want %q", got, MachineCodeNotInitialized)
	}
	denied := &fs.PathError{Op: "open", Path: "planning/STATE.md", Err: fs.ErrPermission}
	if got := missingOrInvalidCode(denied, MachineCodeNotInitialized); got != MachineCodeRepositoryInvalid {
		t.Errorf("unreadable file code is %q, want %q", got, MachineCodeRepositoryInvalid)
	}
}
