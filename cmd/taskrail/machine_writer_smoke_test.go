package main

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/taskrail"
)

// The v0.5 machine API for the semantic writers: every writer that accepted
// `--json` publishes the common envelope, the three lifecycle transitions gain
// one, refusals publish registered error envelopes, and neither mode persists
// anything the other does not.

// seedTodo initializes a managed repository carrying one todo task.
func seedTodo(t *testing.T) string {
	t.Helper()
	root := setupRepo(t)
	writeTask(t, root, "T-100", "todo", "")
	return root
}

// seedActive initializes a managed repository whose single task is in progress,
// which is the state complete and verify require.
func seedActive(t *testing.T) string {
	t.Helper()
	root := seedTodo(t)
	if out, err := runRoot(t, "start", "T-100"); err != nil {
		t.Fatalf("start: %v (output %q)", err, out)
	}
	return root
}

// seedUnmanagedNotes is an unmanaged repository carrying the notes source the
// guided retrofit reads.
func seedUnmanagedNotes(t *testing.T) string {
	t.Helper()
	return setupUnmarkedRepoWithNote(t, "# Roadmap\n\n## Ship it\n")
}

// seedSecondSpec adds a second versioned spec, so a task can be re-pointed or
// the active spec moved without inventing an anchor.
func seedSecondSpec(t *testing.T) string {
	t.Helper()
	root := seedTodo(t)
	if out, err := runRoot(t, "spec", "add", "v0.2.0"); err != nil {
		t.Fatalf("spec add: %v (output %q)", err, out)
	}
	return root
}

func seedDependencyEdge(t *testing.T) string {
	t.Helper()
	root := seedDependencyTasks(t)
	if out, err := runRoot(t, "task", "dependency", "add", "T-100-target", "T-101-dependency"); err != nil {
		t.Fatalf("dependency add: %v (output %q)", err, out)
	}
	return root
}

// seedDraft writes a minimal agent draft `import --apply` can land.
func seedDraft(t *testing.T) string {
	t.Helper()
	root := setupRepo(t)
	draft := `{
  "schema_version": 1,
  "target": "tasks",
  "source": "notes.md",
  "tasks": [{"key": "alpha", "title": "Alpha task", "spec_ref": "specs/v0.1.0.md#summary", "priority": "medium"}]
}`
	if err := os.WriteFile(filepath.Join(root, "draft.json"), []byte(draft), 0o644); err != nil {
		t.Fatalf("write draft: %v", err)
	}
	return root
}

// seedNotes writes an importable markdown source into a managed repository.
func seedNotes(t *testing.T) string {
	t.Helper()
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("# Roadmap\n\n## Ship it\n\n- Add login\n"), 0o644); err != nil {
		t.Fatalf("write notes: %v", err)
	}
	return root
}

// writerMachineInvocations covers every semantic writer's machine surface,
// including the preview and apply outcomes of the writers that have both.
var writerMachineInvocations = []struct {
	name    string
	setup   func(t *testing.T) string
	args    []string
	command string
	shape   string
}{
	{"init", setupUnmarkedRepo, []string{"init", "--json"}, "init", "InitResult"},
	{"retrofit preview", seedUnmanagedNotes,
		[]string{"retrofit", "notes/ideas.md", "--json"}, "retrofit", "RetrofitResult"},
	{"retrofit apply", seedUnmanagedNotes,
		[]string{"retrofit", "notes/ideas.md", "--apply", "--json"}, "retrofit", "RetrofitResult"},
	{"retrofit emit-prompt", seedUnmanagedNotes,
		[]string{"retrofit", "notes/ideas.md", "--emit-prompt", "--json"}, "retrofit", "EmitPromptResult"},
	{"repair dry run", seedTodo, []string{"repair", "--json"}, "repair", "RepairResult"},
	{"repair apply", seedTodo, []string{"repair", "--apply", "--json"}, "repair", "RepairResult"},
	{"next", seedTodo, []string{"next", "--json"}, "next", "NextResult"},
	{"start", seedTodo, []string{"start", "T-100", "--json"}, "start", "StartResult"},
	{"complete", seedActive, []string{"complete", "T-100", "--json"}, "complete", "CompleteResult"},
	{"block", seedActive, []string{"block", "T-100", "--reason", "waiting", "--json"}, "block", "BlockResult"},
	{"unblock", func(t *testing.T) string {
		root := seedActive(t)
		if out, err := runRoot(t, "block", "T-100", "--reason", "waiting"); err != nil {
			t.Fatalf("block: %v (output %q)", err, out)
		}
		return root
	}, []string{"unblock", "T-100", "--json"}, "unblock", "UnblockResult"},
	{"verify", seedActive,
		[]string{"verify", "T-100", "--result", "pass", "--summary", "checked", "--json"}, "verify", "VerifyResult"},
	{"task new", seedTodo,
		[]string{"task", "new", "--title", "Scaffolded", "--spec-ref", "specs/v0.1.0.md#summary", "--json"}, "task new", "TaskNewResult"},
	{"task rename dry run", seedTodo,
		[]string{"task", "rename", "T-100", "--slug", "renamed", "--dry-run", "--json"}, "task rename", "TaskRenameResult"},
	{"task rename apply", seedTodo,
		[]string{"task", "rename", "T-100", "--slug", "renamed", "--json"}, "task rename", "TaskRenameResult"},
	{"task repoint dry run", seedSecondSpec,
		[]string{"task", "repoint", "T-100", "--spec-ref", "specs/v0.2.0.md#summary", "--dry-run", "--json"}, "task repoint", "TaskRepointResult"},
	{"task repoint apply", seedSecondSpec,
		[]string{"task", "repoint", "T-100", "--spec-ref", "specs/v0.2.0.md#summary", "--json"}, "task repoint", "TaskRepointResult"},
	{"task release dry run", seedActive,
		[]string{"task", "release", "T-100", "--reason", "rework", "--dry-run", "--json"}, "task release", "TaskReleaseResult"},
	{"task release apply", seedActive,
		[]string{"task", "release", "T-100", "--reason", "rework", "--json"}, "task release", "TaskReleaseResult"},
	{"task dependency add dry run", seedDependencyTasks,
		[]string{"task", "dependency", "add", "T-100-target", "T-101-dependency", "--dry-run", "--json"}, "task dependency add", "DependencyResult"},
	{"task dependency add apply", seedDependencyTasks,
		[]string{"task", "dependency", "add", "T-100-target", "T-101-dependency", "--json"}, "task dependency add", "DependencyResult"},
	{"task dependency remove dry run", seedDependencyEdge,
		[]string{"task", "dependency", "remove", "T-100-target", "T-101-dependency", "--dry-run", "--json"}, "task dependency remove", "DependencyResult"},
	{"task dependency remove apply", seedDependencyEdge,
		[]string{"task", "dependency", "remove", "T-100-target", "T-101-dependency", "--json"}, "task dependency remove", "DependencyResult"},
	{"spec add", seedTodo, []string{"spec", "add", "v0.2.0", "--json"}, "spec add", "SpecAddResult"},
	{"spec activate", seedSecondSpec,
		[]string{"spec", "activate", "v0.2.0", "--json"}, "spec activate", "SpecActivateResult"},
	{"import preview", seedNotes,
		[]string{"import", "notes.md", "--to", "tasks", "--json"}, "import", "ImportPreviewResult"},
	{"import emit-prompt", seedNotes,
		[]string{"import", "notes.md", "--to", "tasks", "--emit-prompt", "--json"}, "import", "EmitPromptResult"},
	{"import apply", seedDraft,
		[]string{"import", "--apply", "draft.json", "--json"}, "import", "ImportV1ApplyResult"},
}

// A1: every semantic writer publishes its registered result shape inside the
// common envelope, on clean stdout, naming its own canonical command path.
func TestWriterCommandsPublishTheCommonEnvelope(t *testing.T) {
	for _, invocation := range writerMachineInvocations {
		t.Run(invocation.name, func(t *testing.T) {
			invocation.setup(t)
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
			if invocation.command == "verify" {
				if len(envelope.Warnings) != 1 || envelope.Warnings[0].Code != "verify_pass_before_complete" {
					t.Errorf("warnings = %v, want verify order warning", envelope.Warnings)
				}
			} else if len(envelope.Warnings) != 0 {
				t.Errorf("warnings = %v, want none", envelope.Warnings)
			}

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
		})
	}
}

// A2: each lifecycle result carries exactly the fields its contract names, with
// the transition's own status and the common validation object.
func TestLifecycleResultsCarryTheirExactFields(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(t *testing.T) string
		args   []string
		fields []string
		status string
	}{
		{"start", seedTodo, []string{"start", "T-100", "--json"},
			[]string{"status", "task_id", "updated_at", "validation"}, "in_progress"},
		{"complete", seedActive, []string{"complete", "T-100", "--json"},
			[]string{"completion_id", "status", "task_id", "updated_at", "validation"}, "completed"},
		{"block", seedActive, []string{"block", "T-100", "--reason", "waiting", "--json"},
			[]string{"reason", "status", "task_id", "updated_at", "validation"}, "blocked"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			stdout, _, err := runRootSplit(t, tc.args...)
			if err != nil {
				t.Fatalf("%v: %v", tc.args, err)
			}
			var result map[string]any
			decodeMachineResult(t, stdout, &result)

			if got := slices.Sorted(maps.Keys(result)); !slices.Equal(got, tc.fields) {
				t.Fatalf("result fields = %v, want %v", got, tc.fields)
			}
			if result["status"] != tc.status {
				t.Errorf("status = %v, want %q", result["status"], tc.status)
			}
			if result["task_id"] != "T-100" {
				t.Errorf("task_id = %v, want T-100", result["task_id"])
			}
			if tc.name == "complete" {
				completionID, ok := result["completion_id"].(string)
				if !ok || !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(completionID) {
					t.Errorf("completion_id = %v, want lower-case 32-hex", result["completion_id"])
				}
			}
			validation, ok := result["validation"].(map[string]any)
			if !ok {
				t.Fatalf("validation is not an object: %v", result["validation"])
			}
			if got := slices.Sorted(maps.Keys(validation)); !slices.Equal(got, []string{"valid", "violations"}) {
				t.Fatalf("validation fields = %v, want [valid violations]", got)
			}
			if validation["valid"] != true {
				t.Errorf("a clean repository reported valid = %v", validation["valid"])
			}
		})
	}
}

// A2/A4: a transition run in machine mode persists exactly what the equivalent
// human-mode transition persists, and reports the same validation meaning.
func TestLifecycleModesPersistTheSameBytes(t *testing.T) {
	lifecycle := [][]string{
		{"start", "T-100"},
		{"verify", "T-100", "--result", "fail", "--summary", "checked"},
		{"block", "T-100", "--reason", "waiting"},
		{"unblock", "T-100"},
	}
	runLifecycle := func(t *testing.T, machine bool) map[string]string {
		t.Helper()
		root := seedTodo(t)
		for _, step := range lifecycle {
			args := step
			if machine {
				args = append(slices.Clone(step), "--json")
			}
			if _, _, err := runRootSplit(t, args...); err != nil {
				t.Fatalf("%v: %v", args, err)
			}
		}
		// Verification artifacts are producer-local and timestamp-named, so the
		// committed tracked-work files are what the two modes must agree on.
		committed := map[string]string{}
		for path, content := range readAllFiles(t, root) {
			if !strings.HasPrefix(filepath.ToSlash(path), "planning/artifacts/") {
				committed[path] = normalizeTimestamps(content)
			}
		}
		return committed
	}

	text := runLifecycle(t, false)
	machine := runLifecycle(t, true)
	if len(text) != len(machine) {
		t.Fatalf("modes wrote different file sets: %d text files, %d machine files", len(text), len(machine))
	}
	for path, want := range text {
		if got := machine[path]; got != want {
			t.Errorf("%s differs between modes:\ntext=%q\nmachine=%q", path, want, got)
		}
	}
}

// A3: every writer refusal publishes a registered error envelope its command's
// contract admits, reports the operation as uncommitted, and classifies the exit
// exactly as the equivalent human invocation does.
func TestWriterRefusalsPublishRegisteredErrorEnvelopes(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T) string
		args    []string
		command string
		code    string
	}{
		{"unknown task", seedTodo, []string{"start", "T-404", "--json"}, "start", taskrail.MachineCodeTaskNotFound},
		{"task is not todo", seedActive, []string{"start", "T-100", "--json"}, "start", taskrail.MachineCodeInvalidStatus},
		{"task is not transitionable", seedTodo, []string{"complete", "T-100", "--json"}, "complete", taskrail.MachineCodeInvalidStatus},
		{"empty block reason", seedActive, []string{"block", "T-100", "--reason", "  ", "--json"}, "block", taskrail.MachineCodeInvalidReason},
		{"missing block reason", seedActive, []string{"block", "T-100", "--json"}, "block", taskrail.MachineCodeInvalidArguments},
		{"task is not blocked", seedActive, []string{"unblock", "T-100", "--json"}, "unblock", taskrail.MachineCodeInvalidStatus},
		{"invalid verify result", seedActive,
			[]string{"verify", "T-100", "--result", "maybe", "--summary", "checked", "--json"}, "verify", taskrail.MachineCodeInvalidArguments},
		{"missing verify summary", seedActive,
			[]string{"verify", "T-100", "--result", "pass", "--json"}, "verify", taskrail.MachineCodeInvalidArguments},
		{"task new without a reference", seedTodo,
			[]string{"task", "new", "--title", "Orphan", "--json"}, "task new", taskrail.MachineCodeInvalidArguments},
		{"task new following a missing parent", seedTodo,
			[]string{"task", "new", "--title", "Child", "--follow-up", "T-404", "--json"}, "task new", taskrail.MachineCodeInvalidArguments},
		{"rename onto an existing id", func(t *testing.T) string {
			root := seedTodo(t)
			writeTask(t, root, "T-100-taken", "todo", "")
			return root
		}, []string{"task", "rename", "T-100", "--slug", "taken", "--json"}, "task rename", taskrail.MachineCodeDestinationExists},
		{"repoint delivered history", func(t *testing.T) string {
			root := setupRepo(t)
			writeTask(t, root, "T-100", "completed", "")
			return root
		}, []string{"task", "repoint", "T-100", "--spec-ref", "specs/v0.1.0.md#summary", "--json"}, "task repoint", taskrail.MachineCodeInvalidStatus},
		{"spec already exists", seedTodo, []string{"spec", "add", "v0.1.0", "--json"}, "spec add", taskrail.MachineCodeDestinationExists},
		{"unknown spec version", seedTodo, []string{"spec", "activate", "v9.9.9", "--json"}, "spec activate", taskrail.MachineCodeInvalidArguments},
		{"invalid import draft", func(t *testing.T) string {
			root := setupRepo(t)
			if err := os.WriteFile(filepath.Join(root, "draft.json"), []byte(`{"schema_version": 1}`), 0o644); err != nil {
				t.Fatalf("write draft: %v", err)
			}
			return root
		}, []string{"import", "--apply", "draft.json", "--json"}, "import", taskrail.MachineCodeInvalidProposal},
		{"retrofit on a managed repository", seedTodo,
			[]string{"retrofit", "--json"}, "retrofit", taskrail.MachineCodeDestinationExists},
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
			// A refusal never commits, so it can neither report the operation as
			// applied nor name a path it wrote.
			if failure.Details.Applied {
				t.Error("a refusal reported applied = true")
			}
			if len(failure.Details.Paths) != 0 {
				t.Errorf("a refusal named written paths %v", failure.Details.Paths)
			}
			entry, ok := taskrail.MachineCommandEntryFor(tc.command, taskrail.MachineSurfaceStdout)
			if !ok {
				t.Fatalf("no inventory entry for %q", tc.command)
			}
			if !slices.Contains(entry.Errors, failure.Code) {
				t.Errorf("%s does not allow error code %q", entry.CompanionRow, failure.Code)
			}

			// The equivalent human invocation fails the same way and never puts a
			// document on stdout.
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

// A3: a refused writer leaves the repository exactly as it found it, so the
// error envelope's `applied:false` is a fact about the tree and not just a
// constant.
func TestRefusedWritersChangeNothing(t *testing.T) {
	root := seedActive(t)
	before := readAllFiles(t, root)

	for _, args := range [][]string{
		{"start", "T-100", "--json"},
		{"unblock", "T-100", "--json"},
		{"task", "new", "--title", "Orphan", "--json"},
		{"spec", "add", "v0.1.0", "--json"},
	} {
		if _, _, err := runRootSplit(t, args...); err == nil {
			t.Fatalf("%v unexpectedly succeeded", args)
		}
	}

	after := readAllFiles(t, root)
	if len(after) != len(before) {
		t.Fatalf("refused writers changed the file set: %d before, %d after", len(before), len(after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Errorf("refused writers changed %s", path)
		}
	}
}

// A3: a publication failure inside one transaction never tears the write.
// `task new` publishes its task and the re-projected state as one unit, so a
// failing task publication rolls the state back and the failure envelope
// reports an uncommitted operation with nothing left on disk to reconcile.
func TestPartialWriteRollsBackTheWholeTransaction(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission-based fault injection is ineffective on this host")
	}
	root := setupRepo(t)
	statePath := filepath.Join(root, "planning", "STATE.md")
	stateBefore, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state before: %v", err)
	}
	// STATE.md sorts before the task path, so it publishes first; a read-only
	// tasks directory then fails the task publication mid-transaction.
	tasksDir := filepath.Join(root, "planning", "tasks")
	if err := os.Chmod(tasksDir, 0o500); err != nil {
		t.Fatalf("lock tasks dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(tasksDir, 0o755) })

	stdout, _, err := runRootSplit(t,
		"task", "new", "--title", "Halfway", "--spec-ref", "specs/v0.1.0.md#summary", "--json")
	if err == nil {
		t.Fatalf("expected the blocked task publication to fail, got %q", stdout)
	}
	failure := decodeMachineError(t, stdout)
	if failure.Code != taskrail.MachineCodePartialWrite {
		t.Fatalf("code = %q, want %q (message %q)", failure.Code, taskrail.MachineCodePartialWrite, failure.Message)
	}
	if failure.Details.Applied {
		t.Error("a rolled-back write reported applied = true")
	}
	// The transaction undid its own state publication: no half of the write is
	// on disk, which is what makes this a refusal rather than a torn result.
	if _, statErr := os.Stat(filepath.Join(root, "planning", "tasks", "T-001-halfway.md")); statErr == nil {
		t.Error("rolled-back transaction left its task file on disk")
	}
	if got, readErr := os.ReadFile(statePath); readErr != nil || string(got) != string(stateBefore) {
		t.Errorf("STATE.md not restored to its original bytes (err %v):\n%s", readErr, got)
	}
}

// timestampPattern and identityPattern match the generated bytes two equivalent
// lifecycle runs legitimately disagree on.
var timestampPattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`)
var identityPattern = regexp.MustCompile(`\b[0-9a-f]{32}\b`)

func normalizeTimestamps(content string) string {
	content = timestampPattern.ReplaceAllString(content, "<time>")
	return identityPattern.ReplaceAllString(content, "<identity>")
}
