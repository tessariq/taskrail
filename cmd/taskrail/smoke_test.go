package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupRepo creates an isolated temporary repository, changes into it, and
// initializes the Taskrail structure via the real init command. It returns the
// repository root. The .git marker lets findRepoRoot anchor service discovery.
func setupRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("create .git marker: %v", err)
	}
	t.Chdir(root)

	out, err := runRoot(t, "init")
	if err != nil {
		t.Fatalf("init command: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "initialized taskrail structure") {
		t.Fatalf("unexpected init output: %q", out)
	}
	return root
}

// writeTask drops a minimal valid task file into planning/tasks. The spec_ref
// anchor matches a heading in the starter spec written by init.
func writeTask(t *testing.T, root, id, status string, deps string) {
	t.Helper()
	depLine := "dependencies: []"
	if deps != "" {
		depLine = "dependencies: [" + deps + "]"
	}
	content := strings.Join([]string{
		"---",
		"id: " + id,
		"title: Task " + id,
		"status: " + status,
		"priority: high",
		"spec_ref: specs/v0.1.0.md#summary",
		depLine,
		`updated_at: "2026-06-19T00:00:00Z"`,
		"---",
		"",
		"# " + id,
		"",
		"Body.",
		"",
	}, "\n")
	path := filepath.Join(root, "planning", "tasks", id+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write task %s: %v", id, err)
	}
}

func TestInitCreatesStructure(t *testing.T) {
	root := setupRepo(t)
	for _, rel := range []string{
		"planning/STATE.md",
		"planning/tasks",
		"planning/artifacts/verify",
		"specs/README.md",
		"specs/v0.1.0.md",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("expected %s to exist: %v", rel, err)
		}
	}
}

func TestValidateValidRepo(t *testing.T) {
	setupRepo(t)

	out, err := runRoot(t, "validate")
	if err != nil {
		t.Fatalf("validate: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "state valid") {
		t.Fatalf("expected valid state, got %q", out)
	}

	jsonOut, err := runRoot(t, "validate", "--json")
	if err != nil {
		t.Fatalf("validate --json: %v", err)
	}
	if !strings.Contains(jsonOut, `"valid": true`) {
		t.Fatalf("expected valid:true json, got %q", jsonOut)
	}
}

func TestValidateInvalidRepoExitsNonZero(t *testing.T) {
	root := setupRepo(t)
	// Invalid status triggers a validation violation.
	writeTask(t, root, "T-100", "bogus", "")

	out, err := runRoot(t, "validate")
	if err == nil {
		t.Fatalf("expected error for invalid state, output %q", out)
	}
	if !strings.Contains(out, "state invalid") {
		t.Fatalf("expected invalid marker in output, got %q", out)
	}
}

func TestNextNoEligibleTask(t *testing.T) {
	setupRepo(t)
	out, err := runRoot(t, "next")
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if !strings.Contains(out, "no eligible task") {
		t.Fatalf("expected no eligible task, got %q", out)
	}
}

func TestNextSelectsTask(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-100", "todo", "")

	out, err := runRoot(t, "next", "--json")
	if err != nil {
		t.Fatalf("next --json: %v", err)
	}
	if !strings.Contains(out, "T-100") {
		t.Fatalf("expected T-100 selected, got %q", out)
	}
}

func TestStartCompleteFlow(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-100", "todo", "")

	if out, err := runRoot(t, "start", "T-100"); err != nil {
		t.Fatalf("start: %v (output %q)", err, out)
	}
	// Starting a second task while one is active must fail.
	writeTask(t, root, "T-101", "todo", "")
	if _, err := runRoot(t, "start", "T-101"); err == nil {
		t.Fatal("expected error starting second active task")
	}

	if out, err := runRoot(t, "complete", "T-100", "--note", "done"); err != nil {
		t.Fatalf("complete: %v (output %q)", err, out)
	}

	// Repo should be valid again with no active task.
	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after complete: %v (output %q)", err, out)
	}
}

func TestBlockCommand(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-100", "todo", "")
	if out, err := runRoot(t, "start", "T-100"); err != nil {
		t.Fatalf("start: %v (output %q)", err, out)
	}

	if out, err := runRoot(t, "block", "T-100", "--reason", "waiting on upstream"); err != nil {
		t.Fatalf("block: %v (output %q)", err, out)
	}

	// Missing required reason flag must fail wiring.
	writeTask(t, root, "T-101", "todo", "")
	if _, err := runRoot(t, "block", "T-101"); err == nil {
		t.Fatal("expected error when --reason is omitted")
	}
}

func TestVerifyCommand(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-100", "todo", "")
	if out, err := runRoot(t, "start", "T-100"); err != nil {
		t.Fatalf("start: %v (output %q)", err, out)
	}
	if out, err := runRoot(t, "complete", "T-100"); err != nil {
		t.Fatalf("complete: %v (output %q)", err, out)
	}

	out, err := runRoot(t, "verify", "T-100", "--result", "pass", "--summary", "looks good", "--json")
	if err != nil {
		t.Fatalf("verify: %v (output %q)", err, out)
	}
	if !strings.Contains(out, `"result": "pass"`) {
		t.Fatalf("expected pass result in json, got %q", out)
	}

	verifyDir := filepath.Join(root, "planning", "artifacts", "verify", "T-100")
	entries, err := os.ReadDir(verifyDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected verify artifacts under %s: %v", verifyDir, err)
	}
	stamp := entries[0].Name()
	for _, name := range []string{"plan.md", "report.json", "report.md"} {
		if _, err := os.Stat(filepath.Join(verifyDir, stamp, name)); err != nil {
			t.Errorf("expected artifact %s: %v", name, err)
		}
	}
}

func TestVerifyInvalidResult(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-100", "todo", "")
	if _, err := runRoot(t, "verify", "T-100", "--result", "maybe", "--summary", "x"); err == nil {
		t.Fatal("expected error for invalid verify result")
	}
}

func TestTaskNewScaffoldsAndValidates(t *testing.T) {
	root := setupRepo(t)

	out, err := runRoot(t, "task", "new", "--title", "Scaffolded via CLI", "--spec-ref", "specs/v0.1.0.md#summary", "--priority", "high", "--json")
	if err != nil {
		t.Fatalf("task new: %v (output %q)", err, out)
	}
	if !strings.Contains(out, `"task_id": "T-001"`) {
		t.Fatalf("expected T-001 scaffolded, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-001.md")); err != nil {
		t.Fatalf("expected scaffolded task file: %v", err)
	}

	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after scaffold: %v (output %q)", err, out)
	}
}

func TestTaskNewRejectsBadSpecRef(t *testing.T) {
	root := setupRepo(t)
	// Seed a task so the dir is non-empty: a regression writing before validating
	// would add T-101, making the absence check below load-bearing.
	writeTask(t, root, "T-100", "todo", "")

	if _, err := runRoot(t, "task", "new", "--title", "x", "--spec-ref", "specs/v0.1.0.md#nope"); err == nil {
		t.Fatal("expected error for unknown spec anchor")
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-101.md")); !os.IsNotExist(err) {
		t.Fatalf("expected no T-101.md written on rejection, stat err=%v", err)
	}
}

func TestImportPreviewAndApply(t *testing.T) {
	root := setupRepo(t)
	notes := strings.Join([]string{
		"# Feature Notes",
		"",
		"## Add search endpoint",
		"",
		"Return ranked results.",
		"",
		"- index the corpus",
		"- expose the query API",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte(notes), 0o644); err != nil {
		t.Fatalf("write notes: %v", err)
	}

	// Preview prints the draft and writes nothing.
	out, err := runRoot(t, "import", "notes.md", "--to", "tasks", "--json")
	if err != nil {
		t.Fatalf("import preview: %v (output %q)", err, out)
	}
	if !strings.Contains(out, `"applied": false`) || !strings.Contains(out, "Add search endpoint") {
		t.Fatalf("unexpected preview output: %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "imports")); !os.IsNotExist(err) {
		t.Fatalf("preview must not write imports dir, stat err=%v", err)
	}

	// Apply writes a reviewable draft file under planning/imports.
	applyOut, err := runRoot(t, "import", "notes.md", "--to", "tasks", "--apply")
	if err != nil {
		t.Fatalf("import apply: %v (output %q)", err, applyOut)
	}
	if !strings.Contains(applyOut, "planning/imports/") {
		t.Fatalf("expected written path in output, got %q", applyOut)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "imports", "notes.tasks.import.json")); err != nil {
		t.Fatalf("expected written draft file: %v", err)
	}
}

func TestImportRequiresTarget(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("# T\n\n- a\n"), 0o644); err != nil {
		t.Fatalf("write notes: %v", err)
	}
	if _, err := runRoot(t, "import", "notes.md"); err == nil {
		t.Fatal("expected error when --to is omitted")
	}
}

func TestCommandsFailOutsideRepo(t *testing.T) {
	// A bare temp dir with no .git ancestor: service discovery must fail.
	t.Chdir(t.TempDir())
	for _, args := range [][]string{{"validate"}, {"next"}, {"start", "T-1"}} {
		if _, err := runRoot(t, args...); err == nil {
			t.Errorf("expected discovery error for %v", args)
		}
	}
}
