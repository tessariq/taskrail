package taskrail

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// This file is the strict schema-version-1 decoder for the part of a machine
// document every command shares: the envelope, warnings, and the common error
// details. Command-owned `result` payloads stay exact bytes here, because the
// companion registry owns one result shape per command; a consumer decodes the
// common contract once and then decodes its own result.
//
// Strictness is the product: a document is accepted only when it is exactly what
// `specs/contracts/v0.5.0-machine-api.md` describes, so a producer regression
// surfaces at the decoder boundary instead of reaching an agent as a plausible
// partial reading. Rejection is therefore all-or-nothing and never yields a
// partially populated envelope.

// MachineEnvelope is one decoded common document. Exactly one of Result and
// Error is non-nil.
type MachineEnvelope struct {
	SchemaVersion int
	Command       string
	Warnings      []MachineWarning
	// Result is the command-owned success payload's exact object bytes.
	Result json.RawMessage
	Error  *MachineError
}

// MachineError is a common error. Loop postflight errors replace Details with
// the loop diagnostic and are decoded by the loop result-file contract instead.
type MachineError struct {
	Code    string
	Message string
	Details MachineErrorDetails
}

type MachineErrorDetails struct {
	Applied    bool
	Violations []MachineViolation
	Paths      []string
	Snapshots  []MachineSnapshot
	Recovery   *MachineRecoveryRef
}

type MachineViolation struct {
	Code    string
	Message string
	Path    *string
}

// MachineSnapshot is the common snapshot shape. Its JSON tags are the wire
// member names, because a command-owned result payload that carries snapshots
// (RecoverResult) marshals this type directly; the error details path keeps its
// separate wire projection in machine_emit.go.
type MachineSnapshot struct {
	PathKind        string  `json:"path_kind"`
	Path            string  `json:"path"`
	OriginalSHA256  *string `json:"original_sha256"`
	CandidateSHA256 *string `json:"candidate_sha256"`
	CurrentSHA256   *string `json:"current_sha256"`
}

type MachineRecoveryRef struct {
	TransactionID string
	Command       string
	Phase         string
}

var (
	lowerHexDigest    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	absolutePathStart = regexp.MustCompile(`^(/|[A-Za-z]:/)`)

	machineSnapshotPathKinds = []string{"managed", "worktree", "git"}
	machineRecoveryPhases    = []string{
		"prepared", "fence_published", "publishing", "candidate_published",
		"validating", "rolling_back", "recovery_restoring", "recovery_accepting",
		"recovery_clearing",
	}
)

// DecodeMachineEnvelope decodes one schema-version-1 common document. It returns
// the zero envelope with an error whenever the document deviates from the
// companion contract in any way.
func DecodeMachineEnvelope(data []byte) (MachineEnvelope, error) {
	if err := checkDocumentFraming(data); err != nil {
		return MachineEnvelope{}, err
	}
	obj, err := strictObject(data, "document")
	if err != nil {
		return MachineEnvelope{}, err
	}
	// The version gates every other rule, so an unsupported generation is
	// reported as such instead of as a pile of unknown-member failures.
	if err := checkSchemaVersion(obj); err != nil {
		return MachineEnvelope{}, err
	}
	payload, err := payloadMember(obj)
	if err != nil {
		return MachineEnvelope{}, err
	}
	if err := exactMembers(obj, "document", []string{"schema_version", "command", "warnings", payload}); err != nil {
		return MachineEnvelope{}, err
	}

	env := MachineEnvelope{SchemaVersion: MachineSchemaVersion}
	if env.Command, err = commandMember(obj, "document", "command"); err != nil {
		return MachineEnvelope{}, err
	}
	if env.Warnings, err = decodeMachineWarnings(obj["warnings"]); err != nil {
		return MachineEnvelope{}, err
	}
	if payload == "result" {
		if _, err := strictObject(obj["result"], `document member "result"`); err != nil {
			return MachineEnvelope{}, err
		}
		env.Result = obj["result"]
		return env, nil
	}
	decoded, err := decodeMachineError(obj["error"])
	if err != nil {
		return MachineEnvelope{}, err
	}
	env.Error = &decoded
	return env, nil
}

func checkSchemaVersion(obj map[string]json.RawMessage) error {
	raw, ok := obj["schema_version"]
	if !ok {
		return fmt.Errorf("document is missing member %q", "schema_version")
	}
	version, ok := decodeJSONInteger(raw)
	if !ok {
		return fmt.Errorf("document member %q is not an integer", "schema_version")
	}
	if version != MachineSchemaVersion {
		return fmt.Errorf("document declares unsupported schema version %d", version)
	}
	return nil
}

// payloadMember decides the exclusive success/error branch before any member is
// read, so a both/neither document is reported as such instead of as an unknown
// or missing member.
func payloadMember(obj map[string]json.RawMessage) (string, error) {
	_, hasResult := obj["result"]
	_, hasError := obj["error"]
	switch {
	case hasResult && hasError:
		return "", errors.New(`document contains both "result" and "error"`)
	case hasResult:
		return "result", nil
	case hasError:
		return "error", nil
	default:
		return "", errors.New(`document contains neither "result" nor "error"`)
	}
}

func decodeMachineError(raw json.RawMessage) (MachineError, error) {
	obj, err := strictObject(raw, `document member "error"`)
	if err != nil {
		return MachineError{}, err
	}
	if err := exactMembers(obj, "error", []string{"code", "message", "details"}); err != nil {
		return MachineError{}, err
	}
	decoded := MachineError{}
	if decoded.Code, err = enumMember(obj, "error", "code", machineErrorCodes); err != nil {
		return MachineError{}, fmt.Errorf("error member %q is not a registered error code", "code")
	}
	if slices.Contains(loopPostflightErrors, decoded.Code) {
		return MachineError{}, fmt.Errorf("error code %q carries a loop diagnostic, which the common decoder does not accept", decoded.Code)
	}
	if decoded.Message, err = stringMember(obj, "error", "message"); err != nil {
		return MachineError{}, err
	}
	if decoded.Details, err = decodeMachineErrorDetails(obj["details"]); err != nil {
		return MachineError{}, err
	}
	return decoded, nil
}

func decodeMachineErrorDetails(raw json.RawMessage) (MachineErrorDetails, error) {
	obj, err := strictObject(raw, "error details")
	if err != nil {
		return MachineErrorDetails{}, err
	}
	if err := exactMembers(obj, "error details", []string{"applied", "violations", "paths", "snapshots", "recovery"}); err != nil {
		return MachineErrorDetails{}, err
	}
	details := MachineErrorDetails{}
	if details.Applied, err = boolMember(obj, "error details", "applied"); err != nil {
		return MachineErrorDetails{}, err
	}
	if details.Violations, err = decodeMachineViolations(obj["violations"]); err != nil {
		return MachineErrorDetails{}, err
	}
	if details.Paths, err = decodeMachinePaths(obj["paths"]); err != nil {
		return MachineErrorDetails{}, err
	}
	if details.Snapshots, err = decodeMachineSnapshots(obj["snapshots"]); err != nil {
		return MachineErrorDetails{}, err
	}
	if details.Recovery, err = decodeMachineRecovery(obj["recovery"]); err != nil {
		return MachineErrorDetails{}, err
	}
	return details, nil
}

func decodeMachineViolations(raw json.RawMessage) ([]MachineViolation, error) {
	elements, err := arrayMember(raw, "error details", "violations")
	if err != nil {
		return nil, err
	}
	violations := make([]MachineViolation, 0, len(elements))
	for i, element := range elements {
		what := fmt.Sprintf("error violation at index %d", i)
		obj, err := strictObject(element, what)
		if err != nil {
			return nil, err
		}
		if err := exactMembers(obj, what, []string{"code", "message", "path"}); err != nil {
			return nil, err
		}
		violation := MachineViolation{}
		if violation.Code, err = stringMember(obj, what, "code"); err != nil {
			return nil, err
		}
		if violation.Message, err = stringMember(obj, what, "message"); err != nil {
			return nil, err
		}
		if violation.Path, err = nullableStringMember(obj, what, "path"); err != nil {
			return nil, err
		}
		if i > 0 && slices.Compare(violationOrderKey(violations[i-1]), violationOrderKey(violation)) > 0 {
			return nil, fmt.Errorf("error violations are not in contract order at index %d", i)
		}
		violations = append(violations, violation)
	}
	return violations, nil
}

func violationOrderKey(v MachineViolation) []string {
	return []string{v.Code, nullLast(v.Path), v.Message}
}

func decodeMachinePaths(raw json.RawMessage) ([]string, error) {
	elements, err := arrayMember(raw, "error details", "paths")
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(elements))
	for i, element := range elements {
		var path string
		if err := json.Unmarshal(element, &path); err != nil {
			return nil, fmt.Errorf("error path at index %d is not a string", i)
		}
		if path == "" {
			return nil, fmt.Errorf("error path at index %d is empty", i)
		}
		if i > 0 && paths[i-1] > path {
			return nil, fmt.Errorf("error paths are not in contract order at index %d", i)
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func decodeMachineSnapshots(raw json.RawMessage) ([]MachineSnapshot, error) {
	elements, err := arrayMember(raw, "error details", "snapshots")
	if err != nil {
		return nil, err
	}
	snapshots := make([]MachineSnapshot, 0, len(elements))
	for i, element := range elements {
		what := fmt.Sprintf("error snapshot at index %d", i)
		snapshot, err := decodeMachineSnapshot(element, what)
		if err != nil {
			return nil, err
		}
		if i > 0 && slices.Compare(snapshotOrderKey(snapshots[i-1]), snapshotOrderKey(snapshot)) > 0 {
			return nil, fmt.Errorf("error snapshots are not in contract order at index %d", i)
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

func decodeMachineSnapshot(raw json.RawMessage, what string) (MachineSnapshot, error) {
	obj, err := strictObject(raw, what)
	if err != nil {
		return MachineSnapshot{}, err
	}
	members := []string{"path_kind", "path", "original_sha256", "candidate_sha256", "current_sha256"}
	if err := exactMembers(obj, what, members); err != nil {
		return MachineSnapshot{}, err
	}
	snapshot := MachineSnapshot{}
	if snapshot.PathKind, err = enumMember(obj, what, "path_kind", machineSnapshotPathKinds); err != nil {
		return MachineSnapshot{}, err
	}
	if snapshot.Path, err = stringMember(obj, what, "path"); err != nil {
		return MachineSnapshot{}, err
	}
	if err := checkSnapshotPath(what, snapshot.PathKind, snapshot.Path); err != nil {
		return MachineSnapshot{}, err
	}
	if snapshot.OriginalSHA256, err = digestMember(obj, what, "original_sha256"); err != nil {
		return MachineSnapshot{}, err
	}
	if snapshot.CandidateSHA256, err = digestMember(obj, what, "candidate_sha256"); err != nil {
		return MachineSnapshot{}, err
	}
	if snapshot.CurrentSHA256, err = digestMember(obj, what, "current_sha256"); err != nil {
		return MachineSnapshot{}, err
	}
	return snapshot, nil
}

func digestMember(obj map[string]json.RawMessage, what, name string) (*string, error) {
	value, err := nullableStringMember(obj, what, name)
	if err != nil {
		return nil, err
	}
	if value != nil && !lowerHexDigest.MatchString(*value) {
		return nil, fmt.Errorf("%s member %q is not a lower-case 64-hex digest", what, name)
	}
	return value, nil
}

// checkSnapshotPath enforces the path class each `path_kind` selects: managed
// logical and worktree physical paths are canonical repository-relative, while
// Git metadata paths are canonical absolute.
func checkSnapshotPath(what, pathKind, path string) error {
	absolute := absolutePathStart.MatchString(path)
	if pathKind == "git" {
		if !absolute || !canonicalPathSegments(strings.TrimPrefix(path, "/")) {
			return fmt.Errorf("%s member %q is not a canonical absolute path for path_kind %q", what, "path", pathKind)
		}
		return nil
	}
	if absolute || !canonicalPathSegments(path) {
		return fmt.Errorf("%s member %q is not a canonical relative path for path_kind %q", what, "path", pathKind)
	}
	return nil
}

// canonicalPathSegments rejects backslashes, empty, dot, and dot-dot segments,
// and trailing separators, so one path denotes exactly one location.
func canonicalPathSegments(path string) bool {
	if path == "" || strings.Contains(path, `\`) {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func snapshotOrderKey(s MachineSnapshot) []string {
	return []string{s.PathKind, s.Path}
}

func decodeMachineRecovery(raw json.RawMessage) (*MachineRecoveryRef, error) {
	if isJSONNull(raw) {
		return nil, nil
	}
	what := "error recovery"
	obj, err := strictObject(raw, what)
	if err != nil {
		return nil, err
	}
	if err := exactMembers(obj, what, []string{"transaction_id", "command", "phase"}); err != nil {
		return nil, err
	}
	recovery := MachineRecoveryRef{}
	if recovery.TransactionID, err = stringMember(obj, what, "transaction_id"); err != nil {
		return nil, err
	}
	if recovery.Command, err = commandMember(obj, what, "command"); err != nil {
		return nil, err
	}
	if recovery.Phase, err = enumMember(obj, what, "phase", machineRecoveryPhases); err != nil {
		return nil, err
	}
	return &recovery, nil
}
