package taskrail

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// statusResult is a stand-in for a command-owned result payload. The producer
// boundary never inspects a result shape, so any JSON object serves here.
type statusResult struct {
	StatusSummary string `json:"status_summary"`
}

func ptrString(value string) *string { return &value }

func TestEmitMachineDocumentPublishesOneStrictDocument(t *testing.T) {
	var out bytes.Buffer
	outcome := MachineOutcome{
		Command: "status",
		Surface: MachineSurfaceStdout,
		Warnings: []MachineWarning{
			{Code: "skill_version_skew", Message: "skills were written by another binary"},
			{
				Code: "selected_off_spec", Message: "task is off the active spec",
				TaskID: "T-001", SpecRef: "specs/v0.4.0.md#area", ActiveSpecPath: "specs/v0.5.0.md",
			},
		},
		Result: &MachineResult{Shape: "StatusResult", Value: statusResult{StatusSummary: "idle"}},
	}

	if err := EmitMachineDocument(&out, outcome); err != nil {
		t.Fatalf("contractual outcome was refused: %v", err)
	}
	document, ok := strings.CutSuffix(out.String(), "\n")
	if !ok {
		t.Fatalf("document is not newline-terminated: %q", out.String())
	}
	envelope, err := DecodeMachineEnvelope([]byte(document))
	if err != nil {
		t.Fatalf("emitted document does not decode: %v\n%s", err, document)
	}
	if envelope.Command != "status" || envelope.Error != nil {
		t.Fatalf("emitted %+v, want a result envelope for status", envelope)
	}
	var payload statusResult
	if err := json.Unmarshal(envelope.Result, &payload); err != nil {
		t.Fatalf("result payload does not decode: %v\n%s", err, envelope.Result)
	}
	if payload.StatusSummary != "idle" {
		t.Fatalf("result payload is %+v, want the payload the producer supplied", payload)
	}
	// The producer owns contract order, so the warnings the caller supplied
	// unsorted come back sorted rather than as a decode failure.
	if len(envelope.Warnings) != 2 || envelope.Warnings[0].Code != "selected_off_spec" {
		t.Fatalf("warnings are %+v, want them in contract order", envelope.Warnings)
	}
	if outcome.ExitCode() != 0 {
		t.Fatalf("result envelope exits %d, want 0", outcome.ExitCode())
	}
}

func TestEmitMachineDocumentNormalizesEmptyCollections(t *testing.T) {
	var out bytes.Buffer
	outcome := MachineOutcome{
		Command: "status",
		Surface: MachineSurfaceStdout,
		Result:  &MachineResult{Shape: "StatusResult", Value: statusResult{StatusSummary: "idle"}},
	}
	if err := EmitMachineDocument(&out, outcome); err != nil {
		t.Fatalf("contractual outcome was refused: %v", err)
	}
	// The strict decoder rejects `"warnings": null`, so a successful decode is
	// the contract assertion; the literal additionally pins the rendered form a
	// human reads on a terminal.
	envelope, err := DecodeMachineEnvelope(out.Bytes())
	if err != nil {
		t.Fatalf("emitted document does not decode: %v\n%s", err, out.String())
	}
	if len(envelope.Warnings) != 0 {
		t.Fatalf("warnings are %+v, want none", envelope.Warnings)
	}
	if !strings.Contains(out.String(), `"warnings": []`) {
		t.Fatalf("absent warnings are not an empty array:\n%s", out.String())
	}
}

// TestEmitMachineDocumentNormalizesEmptyErrorDetails covers the same rule inside
// an error: a refusal that knows no violation, path, or snapshot still emits
// every required array, and an absent recovery reference stays null.
func TestEmitMachineDocumentNormalizesEmptyErrorDetails(t *testing.T) {
	var out bytes.Buffer
	outcome := MachineOutcome{
		Command: "next",
		Surface: MachineSurfaceStdout,
		Error:   &MachineError{Code: "not_initialized", Message: "repository is not initialized"},
	}
	if err := EmitMachineDocument(&out, outcome); err != nil {
		t.Fatalf("contractual error outcome was refused: %v", err)
	}
	envelope, err := DecodeMachineEnvelope(out.Bytes())
	if err != nil {
		t.Fatalf("emitted error document does not decode: %v\n%s", err, out.String())
	}
	details := envelope.Error.Details
	if len(details.Violations) != 0 || len(details.Paths) != 0 || len(details.Snapshots) != 0 {
		t.Fatalf("error details are %+v, want empty collections", details)
	}
	if details.Recovery != nil {
		t.Fatalf("error details report recovery %+v, want none", details.Recovery)
	}
}

func TestEmitMachineDocumentOrdersErrorDetails(t *testing.T) {
	var out bytes.Buffer
	outcome := MachineOutcome{
		Command: "next",
		Surface: MachineSurfaceStdout,
		Error: &MachineError{
			Code:    "lock_held",
			Message: "another writer holds the repository lock",
			Details: MachineErrorDetails{
				Violations: []MachineViolation{
					{Code: "state_stale", Message: "state is stale", Path: ptrString("planning/STATE.md")},
					{Code: "lock_owner", Message: "owned by another process"},
				},
				Paths: []string{"planning/tasks/T-002.md", "planning/STATE.md"},
				Snapshots: []MachineSnapshot{
					{PathKind: "worktree", Path: "bin/taskrail"},
					{PathKind: "managed", Path: "planning/STATE.md", CurrentSHA256: ptrString(strings.Repeat("a", 64))},
				},
				Recovery: &MachineRecoveryRef{TransactionID: "tx-1", Command: "next", Phase: "rolling_back"},
			},
		},
	}
	if err := EmitMachineDocument(&out, outcome); err != nil {
		t.Fatalf("contractual error outcome was refused: %v", err)
	}
	envelope, err := DecodeMachineEnvelope(out.Bytes())
	if err != nil {
		t.Fatalf("emitted error document does not decode: %v\n%s", err, out.String())
	}
	details := envelope.Error.Details
	if details.Applied {
		t.Fatal("error details report applied:true")
	}
	if details.Violations[0].Code != "lock_owner" {
		t.Fatalf("violations are %+v, want them in contract order", details.Violations)
	}
	if details.Paths[0] != "planning/STATE.md" {
		t.Fatalf("paths are %v, want them in contract order", details.Paths)
	}
	if details.Snapshots[0].PathKind != "managed" {
		t.Fatalf("snapshots are %+v, want them in contract order", details.Snapshots)
	}
	if details.Recovery == nil || details.Recovery.Phase != "rolling_back" {
		t.Fatalf("recovery is %+v", details.Recovery)
	}
	if outcome.ExitCode() != 1 {
		t.Fatalf("error envelope exits %d, want 1", outcome.ExitCode())
	}
}

// TestEmitMachineDocumentGatesCompletedReports covers the report-result
// exception: a completed report whose findings gate exits non-zero and stays a
// result envelope.
func TestEmitMachineDocumentGatesCompletedReports(t *testing.T) {
	var out bytes.Buffer
	outcome := MachineOutcome{
		Command: "validate",
		Surface: MachineSurfaceStdout,
		Result:  &MachineResult{Shape: "ValidateResult", Value: map[string]any{"valid": false}, Gated: true},
	}
	if err := EmitMachineDocument(&out, outcome); err != nil {
		t.Fatalf("gating report was refused: %v", err)
	}
	envelope, err := DecodeMachineEnvelope(out.Bytes())
	if err != nil {
		t.Fatalf("emitted document does not decode: %v", err)
	}
	if envelope.Error != nil {
		t.Fatal("a gating report published an error envelope")
	}
	if outcome.ExitCode() != 1 {
		t.Fatalf("gating report exits %d, want 1", outcome.ExitCode())
	}
}

func TestEmitMachineDocumentRefusesBeforeEmission(t *testing.T) {
	cases := []struct {
		name    string
		outcome MachineOutcome
		wantErr string
	}{
		{
			name: "command outside the inventory",
			outcome: MachineOutcome{
				Command: "version", Surface: MachineSurfaceStdout,
				Result: &MachineResult{Shape: "StatusResult", Value: statusResult{StatusSummary: "idle"}},
			},
			wantErr: `no schema-1 machine contract for "version stdout"`,
		},
		{
			name: "result shape the command never names",
			outcome: MachineOutcome{
				Command: "status", Surface: MachineSurfaceStdout,
				Result: &MachineResult{Shape: "NextResult", Value: statusResult{StatusSummary: "idle"}},
			},
			wantErr: `publishes result shape "NextResult"`,
		},
		{
			name: "error code outside the command's subset",
			outcome: MachineOutcome{
				Command: "status", Surface: MachineSurfaceStdout,
				Error: &MachineError{Code: "lock_held", Message: "locked"},
			},
			wantErr: `publishes error "lock_held"`,
		},
		{
			name: "warning outside the command's subset",
			outcome: MachineOutcome{
				Command: "stats", Surface: MachineSurfaceStdout,
				Warnings: []MachineWarning{{Code: "empty_derived_slug", Message: "empty slug", TaskID: "T-001"}},
				Result:   &MachineResult{Shape: "StatsResult", Value: map[string]any{"total": 1}},
			},
			wantErr: `publishes warning "empty_derived_slug"`,
		},
		{
			name: "gated result from a command that never gates",
			outcome: MachineOutcome{
				Command: "status", Surface: MachineSurfaceStdout,
				Result: &MachineResult{Shape: "StatusResult", Value: statusResult{StatusSummary: "idle"}, Gated: true},
			},
			wantErr: `which its contract never gates`,
		},
		{
			name: "both a result and an error",
			outcome: MachineOutcome{
				Command: "status", Surface: MachineSurfaceStdout,
				Result: &MachineResult{Shape: "StatusResult", Value: statusResult{StatusSummary: "idle"}},
				Error:  &MachineError{Code: "unsupported", Message: "unsupported"},
			},
			wantErr: "both a result and an error",
		},
		{
			name:    "neither a result nor an error",
			outcome: MachineOutcome{Command: "status", Surface: MachineSurfaceStdout},
			wantErr: "neither a result nor an error",
		},
		{
			name: "unregistered warning code",
			outcome: MachineOutcome{
				Command: "status", Surface: MachineSurfaceStdout,
				Warnings: []MachineWarning{{Code: "disk_full", Message: "disk is full"}},
				Result:   &MachineResult{Shape: "StatusResult", Value: statusResult{StatusSummary: "idle"}},
			},
			wantErr: "registered warning code",
		},
		{
			name: "result payload that is not a JSON object",
			outcome: MachineOutcome{
				Command: "status", Surface: MachineSurfaceStdout,
				Result: &MachineResult{Shape: "StatusResult", Value: []string{"idle"}},
			},
			wantErr: `"result" is not a JSON object`,
		},
		{
			name: "result payload that cannot be marshalled",
			outcome: MachineOutcome{
				Command: "status", Surface: MachineSurfaceStdout,
				Result: &MachineResult{Shape: "StatusResult", Value: make(chan int)},
			},
			wantErr: "result payload",
		},
		{
			name: "warning text that is not valid UTF-8",
			outcome: MachineOutcome{
				Command: "status", Surface: MachineSurfaceStdout,
				Warnings: []MachineWarning{{Code: "skill_version_skew", Message: "skew in \xff"}},
				Result:   &MachineResult{Shape: "StatusResult", Value: statusResult{StatusSummary: "idle"}},
			},
			wantErr: "not valid UTF-8",
		},
		{
			name: "error path that is not valid UTF-8",
			outcome: MachineOutcome{
				Command: "next", Surface: MachineSurfaceStdout,
				Error: &MachineError{
					Code:    "path_blocked",
					Message: "destination is blocked",
					Details: MachineErrorDetails{Paths: []string{"planning/tasks/T-\xff.md"}},
				},
			},
			wantErr: "not valid UTF-8",
		},
		{
			name: "loop postflight code on the common surface",
			outcome: MachineOutcome{
				Command: "next", Surface: MachineSurfaceStdout,
				Error: &MachineError{Code: "child_failed", Message: "child failed"},
			},
			wantErr: "carries a loop diagnostic",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := EmitMachineDocument(&out, tc.outcome)
			if err == nil {
				t.Fatalf("outcome was published, want refusal %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("refusal is %q, want it to contain %q", err, tc.wantErr)
			}
			if out.Len() != 0 {
				t.Fatalf("a refused outcome wrote %d bytes to stdout:\n%s", out.Len(), out.String())
			}
		})
	}
}

// TestMachineOutcomeExitClassificationMatchesHumanMode pins the one classifier
// both modes call: an outcome's exit status is a property of the outcome, so a
// command cannot classify the same outcome differently by mode.
func TestMachineOutcomeExitClassificationMatchesHumanMode(t *testing.T) {
	cases := []struct {
		name    string
		outcome MachineOutcome
		want    int
	}{
		{
			name:    "completed operation",
			outcome: MachineOutcome{Result: &MachineResult{Shape: "StatusResult"}},
			want:    0,
		},
		{
			name:    "completed report whose findings gate",
			outcome: MachineOutcome{Result: &MachineResult{Shape: "ValidateResult", Gated: true}},
			want:    1,
		},
		{
			name:    "writer refusal",
			outcome: MachineOutcome{Error: &MachineError{Code: "lock_held"}},
			want:    1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.outcome.ExitCode(); got != tc.want {
				t.Fatalf("outcome exits %d, want %d", got, tc.want)
			}
		})
	}
}
