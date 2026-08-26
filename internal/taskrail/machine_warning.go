package taskrail

import (
	"encoding/json"
	"fmt"
	"slices"
)

// The closed warning union. A warning variant is identified by its `code`, and
// each variant declares an exact member set, so a command-local or cross-variant
// field is a decode failure rather than an extra a consumer must interpret.

// The statuses a verify-order warning may report; `completed` is excluded
// because the warning exists only while completion has not happened.
var machineVerifyOrderStatuses = []string{"todo", "in_progress", "blocked", "cancelled"}

// MachineWarning is one member of the closed warning union. Only the members
// its Code declares are populated; nullable members use pointers so an absent
// value stays distinguishable from an empty one.
type MachineWarning struct {
	Code           string
	Message        string
	TaskID         string
	SpecRef        string
	ActiveSpecPath string
	StorageMode    string
	StorageRoot    string
	Status         string
	ExpectedStatus string
	OriginBranch   *string
	OriginHead     *string
	CurrentBranch  *string
	CurrentHead    *string
}

// MachineWarnings projects the advisories a command raised in process onto the
// closed union the envelope publishes. The encoder emits only the members the
// variant's code declares, so a value carried for one variant can never leak
// into another's object. The result is always non-nil: the envelope's warning
// array is required and is `[]` when a command warned about nothing.
func MachineWarnings(warnings []Warning) []MachineWarning {
	projected := make([]MachineWarning, 0, len(warnings))
	for _, warning := range warnings {
		projected = append(projected, MachineWarning{
			Code:           warning.Code,
			Message:        warning.Message,
			TaskID:         warning.TaskID,
			SpecRef:        warning.SpecRef,
			ActiveSpecPath: warning.ActiveSpecPath,
			StorageMode:    warning.StorageMode,
			StorageRoot:    warning.StorageRoot,
			Status:         warning.Status,
			ExpectedStatus: warning.ExpectedStatus,
		})
	}
	return projected
}

func decodeMachineWarnings(raw json.RawMessage) ([]MachineWarning, error) {
	elements, err := arrayMember(raw, "document", "warnings")
	if err != nil {
		return nil, err
	}
	warnings := make([]MachineWarning, 0, len(elements))
	for i, element := range elements {
		warning, err := decodeMachineWarning(element, fmt.Sprintf("warning at index %d", i))
		if err != nil {
			return nil, err
		}
		if i > 0 && slices.Compare(warningOrderKey(warnings[i-1]), warningOrderKey(warning)) > 0 {
			return nil, fmt.Errorf("document warnings are not in contract order at index %d", i)
		}
		warnings = append(warnings, warning)
	}
	return warnings, nil
}

func decodeMachineWarning(raw json.RawMessage, what string) (MachineWarning, error) {
	obj, err := strictObject(raw, what)
	if err != nil {
		return MachineWarning{}, err
	}
	code, err := enumMember(obj, what, "code", machineWarningCodes)
	if err != nil {
		return MachineWarning{}, fmt.Errorf("%s member %q is not a registered warning code", what, "code")
	}
	if err := exactMembers(obj, what, warningMembers(code)); err != nil {
		return MachineWarning{}, err
	}
	warning := MachineWarning{Code: code}
	if warning.Message, err = stringMember(obj, what, "message"); err != nil {
		return MachineWarning{}, err
	}
	switch code {
	case "empty_derived_slug":
		warning.TaskID, err = stringMember(obj, what, "task_id")
	case "selected_off_spec", "selected_non_active_spec", "skipped_non_active_spec":
		err = decodeSelectionWarning(obj, what, &warning)
	case "local_initialized":
		err = decodeLocalInitializedWarning(obj, what, &warning)
	case "local_head_drift":
		err = decodeHeadDriftWarning(obj, what, &warning)
	case "verify_pass_before_complete":
		err = decodeVerifyOrderWarning(obj, what, &warning)
	}
	if err != nil {
		return MachineWarning{}, err
	}
	return warning, nil
}

// warningMembers is the companion's exact member list per variant.
func warningMembers(code string) []string {
	switch code {
	case "empty_derived_slug":
		return []string{"code", "message", "task_id"}
	case "selected_off_spec", "selected_non_active_spec", "skipped_non_active_spec":
		return []string{"code", "message", "task_id", "spec_ref", "active_spec_path"}
	case "local_initialized":
		return []string{"code", "message", "storage_mode", "storage_root"}
	case "local_head_drift":
		return []string{"code", "message", "origin_branch", "origin_head", "current_branch", "current_head"}
	case "verify_pass_before_complete":
		return []string{"code", "message", "task_id", "status", "expected_status"}
	default:
		return []string{"code", "message"}
	}
}

func decodeSelectionWarning(obj map[string]json.RawMessage, what string, warning *MachineWarning) error {
	var err error
	if warning.TaskID, err = stringMember(obj, what, "task_id"); err != nil {
		return err
	}
	if warning.SpecRef, err = stringMember(obj, what, "spec_ref"); err != nil {
		return err
	}
	warning.ActiveSpecPath, err = stringMember(obj, what, "active_spec_path")
	return err
}

func decodeLocalInitializedWarning(obj map[string]json.RawMessage, what string, warning *MachineWarning) error {
	var err error
	if warning.StorageMode, err = fixedMember(obj, what, "storage_mode", "local"); err != nil {
		return err
	}
	warning.StorageRoot, err = fixedMember(obj, what, "storage_root", ".taskrail/local")
	return err
}

func decodeHeadDriftWarning(obj map[string]json.RawMessage, what string, warning *MachineWarning) error {
	var err error
	if warning.OriginBranch, err = nullableStringMember(obj, what, "origin_branch"); err != nil {
		return err
	}
	if warning.OriginHead, err = nullableStringMember(obj, what, "origin_head"); err != nil {
		return err
	}
	if warning.CurrentBranch, err = nullableStringMember(obj, what, "current_branch"); err != nil {
		return err
	}
	warning.CurrentHead, err = nullableStringMember(obj, what, "current_head")
	return err
}

func decodeVerifyOrderWarning(obj map[string]json.RawMessage, what string, warning *MachineWarning) error {
	var err error
	if warning.TaskID, err = stringMember(obj, what, "task_id"); err != nil {
		return err
	}
	if warning.Status, err = enumMember(obj, what, "status", machineVerifyOrderStatuses); err != nil {
		return err
	}
	warning.ExpectedStatus, err = fixedMember(obj, what, "expected_status", "completed")
	return err
}

// warningOrderKey is the companion's warning sort key: code, then the variant's
// identifying members with nulls last. Message is not identifying.
func warningOrderKey(w MachineWarning) []string {
	key := []string{w.Code}
	switch w.Code {
	case "empty_derived_slug":
		key = append(key, w.TaskID)
	case "selected_off_spec", "selected_non_active_spec", "skipped_non_active_spec":
		key = append(key, w.TaskID, w.SpecRef, w.ActiveSpecPath)
	case "local_head_drift":
		key = append(key, nullLast(w.OriginBranch), nullLast(w.OriginHead), nullLast(w.CurrentBranch), nullLast(w.CurrentHead))
	case "verify_pass_before_complete":
		key = append(key, w.TaskID, w.Status, w.ExpectedStatus)
	}
	return key
}

// nullLast encodes a nullable string so a present value sorts before null.
func nullLast(value *string) string {
	if value == nil {
		return "\x01"
	}
	return "\x00" + *value
}
