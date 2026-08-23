package taskrail

import (
	"encoding/json"
	"fmt"
	"slices"
)

// EncodeLoopResultFileDocument builds the result-file-only surface. Postflight
// failures carry their complete diagnostic instead of common error details.
func EncodeLoopResultFileDocument(report *LoopDiagnostic, cause error, warnings []MachineWarning) ([]byte, error) {
	if cause == nil {
		return EncodeMachineDocument(MachineOutcome{Command: "loop", Surface: MachineSurfaceResultFile, Warnings: warnings, Result: &MachineResult{Shape: "LoopDiagnostic", Value: report}})
	}
	failure := MachineFailureFor(cause)
	if !slices.Contains(loopPostflightErrors, failure.Code) {
		return EncodeMachineDocument(MachineOutcome{Command: "loop", Surface: MachineSurfaceResultFile, Warnings: warnings, Error: &MachineError{Code: failure.Code, Message: cause.Error(), Details: MachineErrorDetails{Applied: failure.Applied, Violations: failure.Violations, Paths: failure.Paths, Snapshots: failure.Snapshots, Recovery: failure.Recovery}}})
	}
	if report == nil {
		return nil, fmt.Errorf("loop postflight error %q has no diagnostic", failure.Code)
	}
	encodedWarnings, err := encodeMachineWarnings(warnings)
	if err != nil {
		return nil, err
	}
	document, err := json.MarshalIndent(struct {
		SchemaVersion int               `json:"schema_version"`
		Command       string            `json:"command"`
		Warnings      []json.RawMessage `json:"warnings"`
		Error         struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details LoopDiagnostic `json:"details"`
		} `json:"error"`
	}{SchemaVersion: MachineSchemaVersion, Command: "loop", Warnings: encodedWarnings, Error: struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details LoopDiagnostic `json:"details"`
	}{Code: failure.Code, Message: cause.Error(), Details: *report}}, "", "  ")
	if err != nil {
		return nil, err
	}
	return document, nil
}
