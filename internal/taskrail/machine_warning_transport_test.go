package taskrail

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// The envelope's warning array is the one place a warning is published, so every
// closed variant must survive the producer boundary as its exact object, and no
// variant may change the process exit status
// (specs/v0.5.0.md#uniform-agent-machine-results).

func encodedWarnings(t *testing.T, o MachineOutcome) []json.RawMessage {
	t.Helper()
	document, err := EncodeMachineDocument(o)
	if err != nil {
		t.Fatalf("encode document: %v", err)
	}
	envelope := struct {
		Warnings []json.RawMessage `json:"warnings"`
	}{}
	if err := json.Unmarshal(document, &envelope); err != nil {
		t.Fatalf("decode document: %v", err)
	}
	if _, err := DecodeMachineEnvelope(document); err != nil {
		t.Fatalf("strict decode: %v", err)
	}
	return envelope.Warnings
}

func TestEveryWarningVariantPublishesItsExactObject(t *testing.T) {
	t.Parallel()

	origin := "main"
	head := "0f1e2d3"
	cases := []struct {
		name    string
		command string
		warning MachineWarning
		want    string
	}{
		{
			name:    "unknown skill version",
			command: "status",
			warning: MachineWarning{Code: "unknown_skill_version", Message: "no marker"},
			want:    `{"code":"unknown_skill_version","message":"no marker"}`,
		},
		{
			name:    "skill version skew",
			command: "status",
			warning: MachineWarning{Code: "skill_version_skew", Message: "older"},
			want:    `{"code":"skill_version_skew","message":"older"}`,
		},
		{
			name:    "empty derived slug",
			command: "task new",
			warning: MachineWarning{Code: "empty_derived_slug", Message: "bare id", TaskID: "T-001"},
			want:    `{"code":"empty_derived_slug","message":"bare id","task_id":"T-001"}`,
		},
		{
			name:    "selected off spec",
			command: "next",
			warning: MachineWarning{
				Code: "selected_off_spec", Message: "off-spec", TaskID: "T-001",
				SpecRef: "specs/v0.1.0.md#summary", ActiveSpecPath: "specs/v0.5.0.md",
			},
			want: `{"code":"selected_off_spec","message":"off-spec","task_id":"T-001",` +
				`"spec_ref":"specs/v0.1.0.md#summary","active_spec_path":"specs/v0.5.0.md"}`,
		},
		{
			name:    "selected non active spec",
			command: "next",
			warning: MachineWarning{
				Code: "selected_non_active_spec", Message: "away", TaskID: "T-001",
				SpecRef: "specs/v0.1.0.md#summary", ActiveSpecPath: "specs/v0.5.0.md",
			},
			want: `{"code":"selected_non_active_spec","message":"away","task_id":"T-001",` +
				`"spec_ref":"specs/v0.1.0.md#summary","active_spec_path":"specs/v0.5.0.md"}`,
		},
		{
			name:    "skipped non active spec",
			command: "status",
			warning: MachineWarning{
				Code: "skipped_non_active_spec", Message: "skipped", TaskID: "T-001",
				SpecRef: "specs/v0.1.0.md#summary", ActiveSpecPath: "specs/v0.5.0.md",
			},
			want: `{"code":"skipped_non_active_spec","message":"skipped","task_id":"T-001",` +
				`"spec_ref":"specs/v0.1.0.md#summary","active_spec_path":"specs/v0.5.0.md"}`,
		},
		{
			name:    "local initialized",
			command: "start",
			warning: MachineWarning{
				Code: "local_initialized", Message: "bootstrapped",
				StorageMode: "local", StorageRoot: ".taskrail/local",
			},
			want: `{"code":"local_initialized","message":"bootstrapped",` +
				`"storage_mode":"local","storage_root":".taskrail/local"}`,
		},
		{
			name:    "local head drift",
			command: "status",
			warning: MachineWarning{
				Code: "local_head_drift", Message: "drifted",
				OriginBranch: &origin, CurrentHead: &head,
			},
			want: `{"code":"local_head_drift","message":"drifted","origin_branch":"main",` +
				`"origin_head":null,"current_branch":null,"current_head":"0f1e2d3"}`,
		},
		{
			name:    "verify pass before complete",
			command: "verify",
			warning: MachineWarning{
				Code: "verify_pass_before_complete", Message: "not completed",
				TaskID: "T-001", Status: "in_progress", ExpectedStatus: "completed",
			},
			want: `{"code":"verify_pass_before_complete","message":"not completed",` +
				`"task_id":"T-001","status":"in_progress","expected_status":"completed"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			outcome := MachineOutcome{
				Command:  tc.command,
				Surface:  MachineSurfaceStdout,
				Warnings: []MachineWarning{tc.warning},
				Result:   &MachineResult{Shape: machineResultShapeFor(t, tc.command), Value: map[string]string{}},
			}
			warnings := encodedWarnings(t, outcome)
			if len(warnings) != 1 {
				t.Fatalf("warnings = %v, want exactly one", warnings)
			}
			// Compacted, because the document is indented for terminal
			// readability while the contract fixes members and their order.
			var compact bytes.Buffer
			if err := json.Compact(&compact, warnings[0]); err != nil {
				t.Fatalf("compact warning: %v", err)
			}
			if got := compact.String(); got != tc.want {
				t.Fatalf("warning object =\n%s\nwant\n%s", got, tc.want)
			}
			if code := outcome.ExitCode(); code != 0 {
				t.Fatalf("exit code = %d, want 0: warnings never change exit status", code)
			}
		})
	}
}

// machineResultShapeFor picks the result shape the command's contract names, so
// the variant test publishes through the same registration check every command
// passes rather than around it.
func machineResultShapeFor(t *testing.T, command string) string {
	t.Helper()
	entry, ok := MachineCommandEntryFor(command, MachineSurfaceStdout)
	if !ok {
		t.Fatalf("no inventory entry for command %q", command)
	}
	return entry.Results[0]
}

func TestMachineWarningsProjectInheritedAdvisories(t *testing.T) {
	t.Parallel()

	got := MachineWarnings([]Warning{
		{Code: "skill_version_skew", Message: "older"},
		{
			Code: "selected_off_spec", Message: "off-spec", TaskID: "T-001",
			SpecRef: "specs/v0.1.0.md#summary", ActiveSpecPath: "specs/v0.5.0.md",
		},
	})
	want := []MachineWarning{
		{Code: "skill_version_skew", Message: "older"},
		{
			Code: "selected_off_spec", Message: "off-spec", TaskID: "T-001",
			SpecRef: "specs/v0.1.0.md#summary", ActiveSpecPath: "specs/v0.5.0.md",
		},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("warnings = %+v, want %+v", got, want)
	}
	if MachineWarnings(nil) == nil {
		t.Fatal("MachineWarnings(nil) must be an empty slice, since the envelope array is never null")
	}
}

// A command-local warning field would publish the same advisory twice in two
// shapes, so the inherited results that still carry advisories in process must
// keep them off the wire entirely.
func TestInheritedResultsPublishNoCommandLocalWarnings(t *testing.T) {
	t.Parallel()

	warned := []Warning{{Code: "empty_derived_slug", Message: "bare id", TaskID: "T-001"}}
	for _, result := range []any{
		NextResult{Reason: "selected", Warnings: warned},
		StatusNext{Reason: "selected", Warnings: warned},
		CreateTaskResult{TaskID: "T-001", Warnings: warned},
		VerifyResult{TaskID: "T-001", Warnings: warned},
		RenameTaskResult{Warnings: warned},
		ApplyDraftResult{Warnings: warned},
	} {
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("marshal %T: %v", result, err)
		}
		if strings.Contains(string(encoded), "warning") || strings.Contains(string(encoded), "empty_derived_slug") {
			t.Fatalf("%T publishes a command-local warning field: %s", result, encoded)
		}
	}
}
