package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/taskrail"
)

// decodeEnvelope decodes stdout as one strict schema-1 machine document, so a
// test observes exactly what an agent would accept.
func decodeEnvelope(t *testing.T, stdout string) taskrail.MachineEnvelope {
	t.Helper()
	envelope, err := taskrail.DecodeMachineEnvelope([]byte(stdout))
	if err != nil {
		t.Fatalf("decode machine document: %v (document %q)", err, stdout)
	}
	if envelope.SchemaVersion != taskrail.MachineSchemaVersion {
		t.Fatalf("schema_version = %d, want %d", envelope.SchemaVersion, taskrail.MachineSchemaVersion)
	}
	return envelope
}

// decodeMachineResult decodes a success envelope's command-owned result payload
// into target.
func decodeMachineResult(t *testing.T, stdout string, target any) {
	t.Helper()
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error != nil {
		t.Fatalf("expected a result envelope, got error %q: %s", envelope.Error.Code, envelope.Error.Message)
	}
	if err := json.Unmarshal(envelope.Result, target); err != nil {
		t.Fatalf("decode result payload: %v (payload %s)", err, envelope.Result)
	}
}

// decodeMachineError decodes an error envelope and returns its common error.
func decodeMachineError(t *testing.T, stdout string) taskrail.MachineError {
	t.Helper()
	envelope := decodeEnvelope(t, stdout)
	if envelope.Error == nil {
		t.Fatalf("expected an error envelope, got a result: %s", envelope.Result)
	}
	return *envelope.Error
}

// setupSpecRepo initializes a repository whose active spec has coverable areas
// and adds a second spec, so the whole read-only family — including `spec diff`
// and the two coverage gates — has a subject.
func setupSpecRepo(t *testing.T) string {
	t.Helper()
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.1.0.md"), []byte(coverageSmokeSpec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	if out, err := runRoot(t, "spec", "add", "v0.2.0"); err != nil {
		t.Fatalf("spec add: %v (output %q)", err, out)
	}
	return root
}

// readOnlyMachineInvocations covers every inherited read-only `--json` command,
// including both coverage reports.
var readOnlyMachineInvocations = []struct {
	command string
	shape   string
	args    []string
}{
	{"validate", "ValidateResult", []string{"validate", "--json"}},
	{"coverage", "CoverageReport", []string{"coverage", "--json"}},
	{"coverage", "GapReport", []string{"coverage", "--gaps", "--json"}},
	{"status", "StatusResult", []string{"status", "--json"}},
	{"stats", "StatsResult", []string{"stats", "--json"}},
	{"spec list", "SpecListResult", []string{"spec", "list", "--json"}},
	{"spec show", "SpecShowResult", []string{"spec", "show", "v0.1.0", "--json"}},
	{"spec diff", "SpecDiffResult", []string{"spec", "diff", "v0.1.0", "v0.2.0", "--json"}},
}

// A1: every inherited read-only report is now one schema-1 document naming its
// canonical command path, with the payload on stdout and nothing else.
func TestReadOnlyCommandsPublishTheCommonEnvelope(t *testing.T) {
	for _, invocation := range readOnlyMachineInvocations {
		t.Run(strings.Join(invocation.args, " "), func(t *testing.T) {
			setupSpecRepo(t)
			stdout, _, err := runRootSplit(t, invocation.args...)
			if err != nil {
				t.Fatalf("%v: %v", invocation.args, err)
			}
			envelope := decodeEnvelope(t, stdout)
			if envelope.Command != invocation.command {
				t.Errorf("command = %q, want %q", envelope.Command, invocation.command)
			}
			if envelope.Error != nil {
				t.Fatalf("expected a result envelope, got error %q", envelope.Error.Code)
			}
			if len(envelope.Warnings) != 0 {
				t.Errorf("warnings = %v, want none", envelope.Warnings)
			}
			var payload map[string]any
			if err := json.Unmarshal(envelope.Result, &payload); err != nil {
				t.Fatalf("result is not an object: %v", err)
			}
			if len(payload) == 0 {
				t.Errorf("result payload is empty for %v", invocation.args)
			}
		})
	}
}

// A1: the inventory names the shape each command may publish, and the producer
// boundary refuses anything else, so the two must agree here too.
func TestReadOnlyResultShapesAreInventoried(t *testing.T) {
	for _, invocation := range readOnlyMachineInvocations {
		entry, ok := taskrail.MachineCommandEntryFor(invocation.command, taskrail.MachineSurfaceStdout)
		if !ok {
			t.Fatalf("no inventory entry for %q", invocation.command)
		}
		if entry.JSONState != taskrail.MachineJSONEnvelope {
			t.Errorf("%s is inventoried as %q, want the common envelope", entry.CompanionRow, entry.JSONState)
		}
		if !slices.Contains(entry.Results, invocation.shape) {
			t.Errorf("%s does not name result shape %q", entry.CompanionRow, invocation.shape)
		}
	}
}

// A2: a completed report whose findings its contract makes gating stays a result
// envelope and exits non-zero.
func TestGatingReportsStayResultEnvelopes(t *testing.T) {
	t.Run("validate", func(t *testing.T) {
		root := setupRepo(t)
		writeTask(t, root, "T-500", "todo", `"T-404-missing"`)

		stdout, _, err := runRootSplit(t, "validate", "--json")
		if err == nil {
			t.Fatal("expected an invalid repository to exit non-zero")
		}
		var result struct {
			Valid      bool     `json:"valid"`
			Violations []string `json:"violations"`
		}
		decodeMachineResult(t, stdout, &result)
		if result.Valid {
			t.Error("valid = true, want false")
		}
		if len(result.Violations) == 0 {
			t.Error("gating validate reported no violations")
		}
	})

	t.Run("coverage --min", func(t *testing.T) {
		setupSpecRepo(t)
		stdout, _, err := runRootSplit(t, "coverage", "--min", "100", "--json")
		if err == nil {
			t.Fatal("expected --min gating to exit non-zero")
		}
		var result map[string]any
		decodeMachineResult(t, stdout, &result)
	})

	t.Run("coverage --gaps --fail-on", func(t *testing.T) {
		root := setupSpecRepo(t)
		writeCoverageTaskFile(t, root, "T-500", "completed", "specs/v0.1.0.md#alpha")

		stdout, _, err := runRootSplit(t, "coverage", "--gaps", "--fail-on", "missing-verification", "--json")
		if err == nil {
			t.Fatal("expected --fail-on gating to exit non-zero")
		}
		var result struct {
			Signals []map[string]any `json:"signals"`
		}
		decodeMachineResult(t, stdout, &result)
		if len(result.Signals) == 0 {
			t.Error("gating gap report carries no signals")
		}
	})
}

// A3: an inability to produce the promised report is an error envelope carrying
// the narrow registered code, and text mode exits the same way.
func TestReadOnlyFailuresPublishRegisteredErrorEnvelopes(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T)
		args  []string
		code  string
		// jsonOnly marks a failure --json itself causes, so there is no
		// equivalent human invocation to compare exits with.
		jsonOnly bool
	}{
		{
			name:  "no repository",
			setup: func(t *testing.T) { t.Chdir(t.TempDir()) },
			args:  []string{"status", "--json"},
			code:  taskrail.MachineCodeNotInitialized,
		},
		{
			name:  "uninitialized repository",
			setup: func(t *testing.T) { setupUnmarkedRepo(t) },
			args:  []string{"stats", "--json"},
			code:  taskrail.MachineCodeNotInitialized,
		},
		{
			name: "unreadable state",
			setup: func(t *testing.T) {
				root := setupRepo(t)
				if err := os.WriteFile(filepath.Join(root, "planning", "STATE.md"), []byte("not frontmatter"), 0o644); err != nil {
					t.Fatalf("corrupt state: %v", err)
				}
			},
			args: []string{"status", "--json"},
			code: taskrail.MachineCodeRepositoryInvalid,
		},
		{
			name:  "unknown spec version",
			setup: func(t *testing.T) { setupRepo(t) },
			args:  []string{"spec", "show", "v9.9.9", "--json"},
			code:  taskrail.MachineCodeInvalidArguments,
		},
		{
			name:  "malformed spec version",
			setup: func(t *testing.T) { setupRepo(t) },
			args:  []string{"spec", "diff", "v0.1.0", "not-a-version", "--json"},
			code:  taskrail.MachineCodeInvalidArguments,
		},
		{
			name:  "unknown coverage area",
			setup: func(t *testing.T) { setupRepo(t) },
			args:  []string{"coverage", "--area", "no-such-area", "--json"},
			code:  taskrail.MachineCodeInvalidArguments,
		},
		{
			name:  "conflicting coverage flags",
			setup: func(t *testing.T) { setupRepo(t) },
			args:  []string{"coverage", "--gaps", "--min", "50", "--json"},
			code:  taskrail.MachineCodeInvalidArguments,
		},
		{
			name:  "unknown fail-on category",
			setup: func(t *testing.T) { setupRepo(t) },
			args:  []string{"coverage", "--gaps", "--fail-on", "nonsense", "--json"},
			code:  taskrail.MachineCodeInvalidArguments,
		},
		{
			name:     "graph export with json",
			setup:    func(t *testing.T) { setupRepo(t) },
			args:     []string{"stats", "--format", "dot", "--json"},
			code:     taskrail.MachineCodeInvalidArguments,
			jsonOnly: true,
		},
		{
			name:  "missing operand",
			setup: func(t *testing.T) { setupRepo(t) },
			args:  []string{"spec", "show", "--json"},
			code:  taskrail.MachineCodeInvalidArguments,
		},
		{
			name:  "unknown flag",
			setup: func(t *testing.T) { setupRepo(t) },
			args:  []string{"status", "--json", "--nonsense"},
			code:  taskrail.MachineCodeInvalidArguments,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			stdout, _, err := runRootSplit(t, tc.args...)
			if err == nil {
				t.Fatalf("%v unexpectedly succeeded", tc.args)
			}
			failure := decodeMachineError(t, stdout)
			if failure.Code != tc.code {
				t.Errorf("code = %q, want %q (message %q)", failure.Code, tc.code, failure.Message)
			}
			if failure.Message == "" {
				t.Error("error envelope carries no message")
			}
			if failure.Details.Applied {
				t.Error("a read-only failure reported applied = true")
			}
			entry, ok := taskrail.MachineCommandEntryFor(machineCommandPathFor(tc.args), taskrail.MachineSurfaceStdout)
			if ok && !slices.Contains(entry.Errors, failure.Code) {
				t.Errorf("%s does not allow error code %q", entry.CompanionRow, failure.Code)
			}

			if tc.jsonOnly {
				return
			}
			// A3: the equivalent human invocation classifies the outcome the
			// same way and never puts a document on stdout.
			textArgs := withoutJSON(tc.args)
			textOut, _, textErr := runRootSplit(t, textArgs...)
			if textErr == nil {
				t.Fatalf("%v succeeded in text mode but failed in JSON mode", textArgs)
			}
			if strings.Contains(textOut, `"schema_version"`) {
				t.Errorf("text mode wrote a machine document: %q", textOut)
			}
		})
	}
}

// A4: read-only machine invocations change nothing, including the ones whose
// completed report gates.
func TestReadOnlyMachineInvocationsAreSideEffectFree(t *testing.T) {
	invocations := [][]string{
		{"validate", "--json"},
		{"coverage", "--json"},
		{"coverage", "--gaps", "--json"},
		{"coverage", "--min", "100", "--json"},
		{"coverage", "--gaps", "--fail-on", "missing-verification", "--json"},
		{"status", "--json"},
		{"stats", "--json"},
		{"spec", "list", "--json"},
		{"spec", "show", "v0.1.0", "--json"},
		{"spec", "diff", "v0.1.0", "v0.2.0", "--json"},
	}
	for _, args := range invocations {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			root := setupSpecRepo(t)
			writeCoverageTaskFile(t, root, "T-500", "completed", "specs/v0.1.0.md#alpha")
			before := readAllFiles(t, root)

			// The gating invocations exit non-zero on purpose; only the
			// repository bytes are under test here.
			_, _, _ = runRootSplit(t, args...)

			for path, content := range readAllFiles(t, root) {
				if before[path] != content {
					t.Errorf("%v changed %s", args, path)
				}
			}
			if len(readAllFiles(t, root)) != len(before) {
				t.Errorf("%v changed the repository file set", args)
			}
		})
	}
}

// machineCommandPathFor derives the canonical command path from an argument
// vector, so a failure case can be checked against its own inventory entry.
func machineCommandPathFor(args []string) string {
	var tokens []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			break
		}
		tokens = append(tokens, arg)
	}
	return strings.Join(tokens, " ")
}

func withoutJSON(args []string) []string {
	text := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != "--json" {
			text = append(text, arg)
		}
	}
	return text
}
