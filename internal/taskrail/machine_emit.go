package taskrail

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"unicode/utf8"
)

// This file is the producer half of the v0.5 machine API: one boundary every
// JSON-capable command publishes through. A command describes the outcome it
// reached — warnings plus exactly one registered result or common error — and the
// boundary owns the wire document: member order, contract collection order,
// empty-array normalization, and the exit classification.
//
// Nothing reaches stdout until the assembled bytes have passed the strict
// decoder and the inventory entry the command claims, so an unregistered
// command/result/error combination is refused instead of published. That makes a
// producer regression a refusal at the boundary rather than a document an agent
// has to discover is wrong.

// machineExitFailure is the one non-zero status the CLI reports; `main` exits 1
// for every command failure, and a gating report joins that classification
// rather than inventing a second failure status.
const machineExitFailure = 1

// MachineResult is one command's success payload.
type MachineResult struct {
	// Shape is the companion result shape the producer built, such as
	// "StatusResult". The boundary holds it to the command's inventory entry; the
	// payload itself stays command-owned.
	Shape string
	// Value marshals to the result object.
	Value any
	// Gated marks a completed read-only report whose findings its command
	// contract makes gating. It stays a result envelope and exits non-zero; a
	// command with no report-result exception is refused for claiming it.
	Gated bool
}

// MachineOutcome is one finished command as its producer knows it, before any
// bytes exist. Exactly one of Result and Error is set.
type MachineOutcome struct {
	Command  string
	Surface  MachineSurface
	Warnings []MachineWarning
	Result   *MachineResult
	Error    *MachineError
}

// ExitCode classifies the outcome's process exit status. Human and JSON modes
// call it with the same outcome, so equivalent outcomes cannot classify
// differently by mode. Warnings never change it.
func (o MachineOutcome) ExitCode() int {
	switch {
	case o.Error != nil:
		return machineExitFailure
	case o.Result != nil && o.Result.Gated:
		return machineExitFailure
	default:
		return 0
	}
}

// EmitMachineDocument writes exactly one schema-version-1 document to out,
// newline-terminated. It writes nothing when the outcome is refused, so a
// refusal never leaves a partial document on stdout. Diagnostics belong on
// another stream: out carries the document and nothing else.
func EmitMachineDocument(out io.Writer, o MachineOutcome) error {
	document, err := EncodeMachineDocument(o)
	if err != nil {
		return err
	}
	_, err = out.Write(append(document, '\n'))
	return err
}

// EncodeMachineDocument returns the exact document bytes for one outcome, or an
// error naming the single contract the outcome breaks.
func EncodeMachineDocument(o MachineOutcome) ([]byte, error) {
	document, err := buildMachineDocument(o)
	if err != nil {
		return nil, err
	}
	publication := MachinePublication{
		Command:  o.Command,
		Surface:  o.Surface,
		ExitCode: o.ExitCode(),
		Document: document,
	}
	if o.Result != nil {
		publication.Result = o.Result.Shape
	}
	if err := CheckMachinePublication(publication); err != nil {
		return nil, err
	}
	return document, nil
}

// machineDocumentWire fixes the companion's top-level member order. Exactly one
// of Result and Error is populated, so the other is omitted rather than emitted
// as null.
type machineDocumentWire struct {
	SchemaVersion int               `json:"schema_version"`
	Command       string            `json:"command"`
	Warnings      []json.RawMessage `json:"warnings"`
	Result        json.RawMessage   `json:"result,omitempty"`
	Error         *machineErrorWire `json:"error,omitempty"`
}

type machineErrorWire struct {
	Code    string                  `json:"code"`
	Message string                  `json:"message"`
	Details machineErrorDetailsWire `json:"details"`
}

type machineErrorDetailsWire struct {
	Applied    bool                    `json:"applied"`
	Violations []machineViolationWire  `json:"violations"`
	Paths      []string                `json:"paths"`
	Snapshots  []machineSnapshotWire   `json:"snapshots"`
	Recovery   *machineRecoveryRefWire `json:"recovery"`
}

type machineViolationWire struct {
	Code    string  `json:"code"`
	Message string  `json:"message"`
	Path    *string `json:"path"`
}

type machineSnapshotWire struct {
	PathKind        string  `json:"path_kind"`
	Path            string  `json:"path"`
	OriginalSHA256  *string `json:"original_sha256"`
	CandidateSHA256 *string `json:"candidate_sha256"`
	CurrentSHA256   *string `json:"current_sha256"`
}

type machineRecoveryRefWire struct {
	TransactionID string `json:"transaction_id"`
	Command       string `json:"command"`
	Phase         string `json:"phase"`
}

func buildMachineDocument(o MachineOutcome) ([]byte, error) {
	if o.Result != nil && o.Error != nil {
		return nil, errors.New("outcome carries both a result and an error")
	}
	if o.Result == nil && o.Error == nil {
		return nil, errors.New("outcome carries neither a result nor an error")
	}
	if err := checkMachineText(o); err != nil {
		return nil, err
	}
	warnings, err := encodeMachineWarnings(o.Warnings)
	if err != nil {
		return nil, err
	}
	document := machineDocumentWire{
		SchemaVersion: MachineSchemaVersion,
		Command:       o.Command,
		Warnings:      warnings,
	}
	if o.Result != nil {
		if document.Result, err = json.Marshal(o.Result.Value); err != nil {
			return nil, fmt.Errorf("encode result payload for shape %q: %w", o.Result.Shape, err)
		}
	}
	if o.Error != nil {
		document.Error = encodeMachineError(*o.Error)
	}
	// Indentation keeps `--json` output as readable on a terminal as the shipped
	// pre-v0.5 shapes were; the contract fixes members, not whitespace.
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode machine document: %w", err)
	}
	return encoded, nil
}

// checkMachineText refuses producer text that is not valid UTF-8. Marshalling
// substitutes U+FFFD for an invalid byte instead of failing, and the substituted
// document is itself valid UTF-8, so the self-verifying decode cannot catch it:
// a path built from a filesystem name would publish as a location no agent can
// resolve. The command-owned result payload stays the command's own contract,
// because only its shape knows which of its members are text.
func checkMachineText(o MachineOutcome) error {
	for _, text := range machineOutcomeText(o) {
		if !utf8.ValidString(text) {
			return fmt.Errorf("outcome carries text that is not valid UTF-8: %q", text)
		}
	}
	return nil
}

// machineOutcomeText collects every string the boundary itself puts on the wire.
func machineOutcomeText(o MachineOutcome) []string {
	text := []string{o.Command}
	for _, warning := range o.Warnings {
		for _, name := range warningMembers(warning.Code) {
			switch value := machineWarningMember(warning, name).(type) {
			case string:
				text = append(text, value)
			case *string:
				text = appendNonNil(text, value)
			}
		}
	}
	if o.Error == nil {
		return text
	}
	details := o.Error.Details
	text = append(text, o.Error.Code, o.Error.Message)
	text = append(text, details.Paths...)
	for _, violation := range details.Violations {
		text = appendNonNil(append(text, violation.Code, violation.Message), violation.Path)
	}
	for _, snapshot := range details.Snapshots {
		text = append(text, snapshot.PathKind, snapshot.Path)
		text = appendNonNil(text, snapshot.OriginalSHA256)
		text = appendNonNil(text, snapshot.CandidateSHA256)
		text = appendNonNil(text, snapshot.CurrentSHA256)
	}
	if details.Recovery != nil {
		text = append(text, details.Recovery.TransactionID, details.Recovery.Command, details.Recovery.Phase)
	}
	return text
}

func appendNonNil(text []string, value *string) []string {
	if value == nil {
		return text
	}
	return append(text, *value)
}

// sortedByContractKey orders a collection by the companion's sort key for it
// rather than trusting the caller: a producer assembles a collection as it
// discovers members, and ordering is the boundary's contract, not theirs.
func sortedByContractKey[T any](values []T, key func(T) []string) []T {
	sorted := slices.Clone(values)
	slices.SortStableFunc(sorted, func(a, b T) int {
		return slices.Compare(key(a), key(b))
	})
	return sorted
}

func encodeMachineWarnings(warnings []MachineWarning) ([]json.RawMessage, error) {
	sorted := sortedByContractKey(warnings, warningOrderKey)
	encoded := make([]json.RawMessage, 0, len(sorted))
	for _, warning := range sorted {
		member, err := encodeMachineWarning(warning)
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, member)
	}
	return encoded, nil
}

// encodeMachineWarning emits exactly the members the variant declares, reusing
// the decoder's member list so producer and decoder cannot disagree about a
// variant's shape.
func encodeMachineWarning(warning MachineWarning) (json.RawMessage, error) {
	if !slices.Contains(machineWarningCodes, warning.Code) {
		return nil, fmt.Errorf("warning %q is not a registered warning code", warning.Code)
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, name := range warningMembers(warning.Code) {
		if i > 0 {
			buf.WriteByte(',')
		}
		value, err := json.Marshal(machineWarningMember(warning, name))
		if err != nil {
			return nil, fmt.Errorf("encode warning %q member %q: %w", warning.Code, name, err)
		}
		fmt.Fprintf(&buf, "%q:%s", name, value)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// machineWarningMember projects one variant member. Nullable members stay
// pointers so an absent value encodes as null rather than as an empty string.
func machineWarningMember(w MachineWarning, name string) any {
	switch name {
	case "code":
		return w.Code
	case "message":
		return w.Message
	case "task_id":
		return w.TaskID
	case "spec_ref":
		return w.SpecRef
	case "active_spec_path":
		return w.ActiveSpecPath
	case "storage_mode":
		return w.StorageMode
	case "storage_root":
		return w.StorageRoot
	case "status":
		return w.Status
	case "expected_status":
		return w.ExpectedStatus
	case "origin_branch":
		return w.OriginBranch
	case "origin_head":
		return w.OriginHead
	case "current_branch":
		return w.CurrentBranch
	case "current_head":
		return w.CurrentHead
	default:
		return nil
	}
}

func encodeMachineError(source MachineError) *machineErrorWire {
	details := source.Details
	// Cloning nil yields nil, so this required array needs the empty-array
	// normalization the two make()-built collections below get for free.
	paths := slices.Clone(details.Paths)
	if paths == nil {
		paths = []string{}
	}
	slices.Sort(paths)

	wire := &machineErrorWire{
		Code:    source.Code,
		Message: source.Message,
		Details: machineErrorDetailsWire{
			Applied:    details.Applied,
			Violations: encodeMachineViolations(details.Violations),
			Paths:      paths,
			Snapshots:  encodeMachineSnapshots(details.Snapshots),
		},
	}
	if details.Recovery != nil {
		wire.Details.Recovery = &machineRecoveryRefWire{
			TransactionID: details.Recovery.TransactionID,
			Command:       details.Recovery.Command,
			Phase:         details.Recovery.Phase,
		}
	}
	return wire
}

func encodeMachineViolations(violations []MachineViolation) []machineViolationWire {
	sorted := sortedByContractKey(violations, violationOrderKey)
	wire := make([]machineViolationWire, 0, len(sorted))
	for _, violation := range sorted {
		wire = append(wire, machineViolationWire{
			Code: violation.Code, Message: violation.Message, Path: violation.Path,
		})
	}
	return wire
}

func encodeMachineSnapshots(snapshots []MachineSnapshot) []machineSnapshotWire {
	sorted := sortedByContractKey(snapshots, snapshotOrderKey)
	wire := make([]machineSnapshotWire, 0, len(sorted))
	for _, snapshot := range sorted {
		wire = append(wire, machineSnapshotWire{
			PathKind:        snapshot.PathKind,
			Path:            snapshot.Path,
			OriginalSHA256:  snapshot.OriginalSHA256,
			CandidateSHA256: snapshot.CandidateSHA256,
			CurrentSHA256:   snapshot.CurrentSHA256,
		})
	}
	return wire
}
