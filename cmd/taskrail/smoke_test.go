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

// setupUnmarkedRepo creates a bare temp repo carrying only a .git marker and
// changes into it, so retrofit tests start from a non-standard, unmanaged tree.
func setupUnmarkedRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("create .git marker: %v", err)
	}
	t.Chdir(root)
	return root
}

// setupUnmarkedRepoWithNote seeds a bare unmarked repo with body at
// notes/ideas.md, the human-notes source the guided retrofit imports.
func setupUnmarkedRepoWithNote(t *testing.T, body string) string {
	t.Helper()
	root := setupUnmarkedRepo(t)
	notePath := filepath.Join(root, "notes", "ideas.md")
	if err := os.MkdirAll(filepath.Dir(notePath), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(notePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
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
		"specs/README.md",
		"specs/v0.1.0.md",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("expected %s to exist: %v", rel, err)
		}
	}
	// init does not pre-create gitignored artifact output dirs (T-024/T-025).
	for _, rel := range []string{
		"planning/artifacts/verify",
		"planning/artifacts/runs",
		"planning/artifacts/manual-test",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Errorf("expected init not to create %s, stat err=%v", rel, err)
		}
	}
}

// TestInitWithSkillsInstallsOptIn verifies the opt-in flag installs the embedded
// skills into the agent-tool directories, while a default init leaves them out.
func TestInitWithSkillsInstallsOptIn(t *testing.T) {
	root := setupRepo(t)

	// Default init (run by setupRepo) must not have written skill directories.
	for _, dir := range []string{".agents/skills", ".claude/skills"} {
		if _, err := os.Stat(filepath.Join(root, dir)); !os.IsNotExist(err) {
			t.Errorf("default init created %s, stat err=%v", dir, err)
		}
	}

	out, err := runRoot(t, "init", "--with-skills")
	if err != nil {
		t.Fatalf("init --with-skills: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "skills: installed") {
		t.Fatalf("unexpected --with-skills output: %q", out)
	}
	for _, dir := range []string{".agents/skills", ".claude/skills"} {
		if _, err := os.Stat(filepath.Join(root, dir, "autonomous-backlog", "SKILL.md")); err != nil {
			t.Errorf("expected installed skill under %s: %v", dir, err)
		}
	}

	// Re-running is non-destructive and reports nothing newly written.
	out, err = runRoot(t, "init", "--with-skills")
	if err != nil {
		t.Fatalf("re-run init --with-skills: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "already installed") {
		t.Fatalf("expected idempotent re-run notice, got: %q", out)
	}
}

// TestInitRetrofitDryRunThenApply exercises the guided retrofit path end to end
// through the CLI: a repo with a non-standard notes/ directory gets a dry-run
// proposal that writes nothing, then an --apply that scaffolds the layout while
// preserving the pre-existing note, and reports that content was not moved.
func TestInitRetrofitDryRunThenApply(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("create .git marker: %v", err)
	}
	notePath := filepath.Join(root, "notes", "ideas.md")
	if err := os.MkdirAll(filepath.Dir(notePath), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(notePath, []byte("loose notes\n"), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}
	t.Chdir(root)

	out, err := runRoot(t, "init")
	if err != nil {
		t.Fatalf("init dry run: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "non-standard layout detected") ||
		!strings.Contains(out, "notes/ -> planning/ (planning)") ||
		!strings.Contains(out, "existing content is not moved") {
		t.Fatalf("unexpected retrofit dry-run output: %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "STATE.md")); !os.IsNotExist(err) {
		t.Fatalf("dry run scaffolded layout: STATE.md stat err=%v", err)
	}

	out, err = runRoot(t, "init", "--apply")
	if err != nil {
		t.Fatalf("init --apply: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "retrofit applied (existing content was not moved)") ||
		!strings.Contains(out, "validation: valid") {
		t.Fatalf("unexpected retrofit apply output: %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "STATE.md")); err != nil {
		t.Fatalf("apply did not scaffold STATE.md: %v", err)
	}
	got, err := os.ReadFile(notePath)
	if err != nil || string(got) != "loose notes\n" {
		t.Fatalf("retrofit moved or rewrote notes content: got %q err=%v", string(got), err)
	}
}

// TestRetrofitCommandDryRunThenApply exercises the guided retrofit command end to
// end: a non-standard repo with human notes gets a dry-run proposal that imports
// the notes into a planning bootstrap and writes nothing, then an --apply that
// scaffolds the layout, re-runs validation, and preserves the notes.
func TestRetrofitCommandDryRunThenApply(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("create .git marker: %v", err)
	}
	notePath := filepath.Join(root, "notes", "ideas.md")
	if err := os.MkdirAll(filepath.Dir(notePath), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	noteBody := "# Roadmap\n\n## Ship it\n\n- Add login\n- Add logout\n"
	if err := os.WriteFile(notePath, []byte(noteBody), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}
	t.Chdir(root)

	out, err := runRoot(t, "retrofit", "notes/ideas.md")
	if err != nil {
		t.Fatalf("retrofit dry run: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "guided retrofit (dry run)") ||
		!strings.Contains(out, "notes/ -> planning/ (planning)") ||
		!strings.Contains(out, "planning bootstrap") ||
		!strings.Contains(out, "re-run with --apply") {
		t.Fatalf("unexpected retrofit dry-run output: %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "STATE.md")); !os.IsNotExist(err) {
		t.Fatalf("dry run scaffolded layout: STATE.md stat err=%v", err)
	}

	out, err = runRoot(t, "retrofit", "notes/ideas.md", "--apply")
	if err != nil {
		t.Fatalf("retrofit --apply: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "retrofit applied (existing content was not moved)") ||
		!strings.Contains(out, "validation: valid") {
		t.Fatalf("unexpected retrofit apply output: %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "STATE.md")); err != nil {
		t.Fatalf("apply did not scaffold STATE.md: %v", err)
	}
	got, err := os.ReadFile(notePath)
	if err != nil || string(got) != noteBody {
		t.Fatalf("retrofit moved or rewrote notes content: got %q err=%v", string(got), err)
	}
}

// TestRetrofitEmitPromptMatchesImport proves `retrofit <notes> --emit-prompt`
// prints the exact prompt of the planning-target `import --emit-prompt` and
// scaffolds nothing, so retrofit is the single guided entry point without
// forking a second prompt path.
func TestRetrofitEmitPromptMatchesImport(t *testing.T) {
	root := setupUnmarkedRepoWithNote(t, "# Roadmap\n\n## Ship it\n\n- Add login\n- Add logout\n")

	retro, err := runRoot(t, "retrofit", "notes/ideas.md", "--emit-prompt")
	if err != nil {
		t.Fatalf("retrofit emit-prompt: %v (output %q)", err, retro)
	}
	imp, err := runRoot(t, "import", "notes/ideas.md", "--to", "planning", "--emit-prompt")
	if err != nil {
		t.Fatalf("import emit-prompt: %v (output %q)", err, imp)
	}
	if retro != imp {
		t.Fatalf("retrofit emit-prompt output diverged from import:\nretrofit=%q\nimport=%q", retro, imp)
	}
	if !strings.Contains(retro, "Taskrail import prompt") {
		t.Fatalf("emit-prompt output missing prompt header: %q", retro)
	}

	// The machine result must match too, guarding the machine-readable path. The
	// envelopes themselves differ by exactly one field — each names its own
	// command — so the comparison is of the shared result payload.
	retroJSON, _, err := runRootSplit(t, "retrofit", "notes/ideas.md", "--emit-prompt", "--json")
	if err != nil {
		t.Fatalf("retrofit emit-prompt --json: %v (output %q)", err, retroJSON)
	}
	impJSON, _, err := runRootSplit(t, "import", "notes/ideas.md", "--to", "planning", "--emit-prompt", "--json")
	if err != nil {
		t.Fatalf("import emit-prompt --json: %v (output %q)", err, impJSON)
	}
	retroResult, impResult := decodeEnvelope(t, retroJSON), decodeEnvelope(t, impJSON)
	if string(retroResult.Result) != string(impResult.Result) {
		t.Fatalf("retrofit emit-prompt --json diverged from import:\nretrofit=%s\nimport=%s", retroResult.Result, impResult.Result)
	}
	if retroResult.Command != "retrofit" || impResult.Command != "import" {
		t.Fatalf("each envelope must name its own command, got %q and %q", retroResult.Command, impResult.Command)
	}

	// Emit-prompt never scaffolds.
	if _, err := os.Stat(filepath.Join(root, "planning", "STATE.md")); !os.IsNotExist(err) {
		t.Fatalf("emit-prompt scaffolded layout: STATE.md stat err=%v", err)
	}
}

// TestRetrofitEmitPromptAllowedOnManagedRepo confirms the read-only prompt path
// works on any repo, unlike the scaffolding retrofit path which refuses an
// already-managed repository. This locks the intentional marker-check bypass
// that keeps `retrofit --emit-prompt` equivalent to `import --emit-prompt`.
func TestRetrofitEmitPromptAllowedOnManagedRepo(t *testing.T) {
	root := setupRepo(t) // init writes the marker, so the repo is managed
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("# Roadmap\n\n## Ship it\n\n- Add login\n"), 0o644); err != nil {
		t.Fatalf("write notes: %v", err)
	}

	// Plain retrofit refuses a managed repo.
	if _, err := runRoot(t, "retrofit", "notes.md"); err == nil {
		t.Fatal("plain retrofit must refuse an already-managed repository")
	}

	// --emit-prompt is read-only and works regardless.
	out, err := runRoot(t, "retrofit", "notes.md", "--emit-prompt")
	if err != nil {
		t.Fatalf("retrofit --emit-prompt on managed repo: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "Taskrail import prompt") {
		t.Fatalf("unexpected emit-prompt output: %q", out)
	}
}

// TestRetrofitEmitPromptRequiresNotes rejects an emit-prompt run with no notes
// source; there is nothing to build a prompt from.
func TestRetrofitEmitPromptRequiresNotes(t *testing.T) {
	setupUnmarkedRepo(t)
	if _, err := runRoot(t, "retrofit", "--emit-prompt"); err == nil {
		t.Fatal("expected error when --emit-prompt has no notes source")
	}
}

// TestRetrofitEmitPromptRejectsApply keeps the read-only prompt path and the
// scaffold path from being requested at once.
func TestRetrofitEmitPromptRejectsApply(t *testing.T) {
	// The conflict guard fires before any notes I/O, so a bare repo suffices.
	setupUnmarkedRepo(t)
	if _, err := runRoot(t, "retrofit", "notes/ideas.md", "--emit-prompt", "--apply"); err == nil {
		t.Fatal("expected error when --emit-prompt is combined with --apply")
	}
}

// TestRetrofitEmitPromptThenApply exercises the documented adoption path end to
// end: retrofit scaffolds the layout, emit-prompt hands the semantic lift to an
// agent, and the agent-refined draft flows through `import --apply` so real
// spec/task files land with a valid spec_ref (reusing CreateTask validation).
func TestRetrofitEmitPromptThenApply(t *testing.T) {
	root := setupUnmarkedRepoWithNote(t, "# Roadmap\n\n## Ship it\n\n- Add login\n")

	// Emit the prompt (no scaffold yet), then scaffold the tracked layout.
	if out, err := runRoot(t, "retrofit", "notes/ideas.md", "--emit-prompt"); err != nil {
		t.Fatalf("retrofit emit-prompt: %v (output %q)", err, out)
	}
	if out, err := runRoot(t, "retrofit", "notes/ideas.md", "--apply"); err != nil {
		t.Fatalf("retrofit apply: %v (output %q)", err, out)
	}

	// An agent-refined draft: a spec section supplies the heading its task's
	// spec_ref points at, so apply reuses CreateTask validation with no invented
	// anchor.
	draft := `{
  "schema_version": 1,
  "target": "planning",
  "source": "notes/ideas.md",
  "spec_sections": [{"heading": "Roadmap", "body": "Ship the importer."}],
  "tasks": [
    {"key": "importer", "title": "Wire structural import into retrofit", "spec_ref": "specs/ideas.md#roadmap", "priority": "medium"}
  ]
}`
	if err := os.WriteFile(filepath.Join(root, "draft.json"), []byte(draft), 0o644); err != nil {
		t.Fatalf("write draft: %v", err)
	}
	applyOut, err := runRoot(t, "import", "--apply", "draft.json")
	if err != nil {
		t.Fatalf("import apply: %v (output %q)", err, applyOut)
	}
	if !strings.Contains(applyOut, "created T-") || !strings.Contains(applyOut, "wrote spec specs/ideas.md") {
		t.Fatalf("unexpected apply output: %q", applyOut)
	}
	if _, err := os.Stat(filepath.Join(root, "specs", "ideas.md")); err != nil {
		t.Fatalf("apply did not write spec: %v", err)
	}
	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("post-apply validate failed: %v (output %q)", err, out)
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

// TestNextSkipsIdleTaskOutsideActiveSpec is the CLI counterpart to the service
// selector test: an idle todo linked to an older spec is filtered out of `next`
// selection, so both human and --json report no eligible task and expose the
// skipped runnable work as a structured warning (T-108 supersedes the T-103
// advisory-warning behavior for idle selection).
func TestNextSkipsIdleTaskOutsideActiveSpec(t *testing.T) {
	root := setupRepo(t)
	activateSecondSpec(t, root)
	writeTask(t, root, "T-100", "todo", "")

	out, err := runRoot(t, "next")
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	for _, want := range []string{"no eligible task", "warning:", "T-100", "specs/v0.2.0.md"} {
		if !strings.Contains(out, want) {
			t.Fatalf("next output missing %q: %q", want, out)
		}
	}

	root = setupRepo(t)
	activateSecondSpec(t, root)
	writeTask(t, root, "T-100", "todo", "")
	out, err = runRoot(t, "next", "--json")
	if err != nil {
		t.Fatalf("next --json: %v", err)
	}
	for _, want := range []string{`"reason": "no active-spec eligible task"`, `"warnings": [`, `"code": "skipped_non_active_spec"`, `"task_id": "T-100"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("next json missing %q: %q", want, out)
		}
	}
}

// TestNextContinuesActiveOldSpecTaskWithWarning keeps the surviving warning case:
// an already-active in_progress task pointing outside the active spec is still
// returned (the active task owns the workflow slot) with the drift warning.
func TestNextContinuesActiveOldSpecTaskWithWarning(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-100", "todo", "")
	if _, err := runRoot(t, "start", "T-100"); err != nil {
		t.Fatalf("start: %v", err)
	}
	activateSecondSpec(t, root)

	out, err := runRoot(t, "next", "--json")
	if err != nil {
		t.Fatalf("next --json: %v", err)
	}
	for _, want := range []string{`"task_id": "T-100"`, `"reason": "active task already in progress"`, `"code": "selected_non_active_spec"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("next json missing %q: %q", want, out)
		}
	}
}

// TestNextIncludeOffSpecRecoversOlderSpecTask is the CLI counterpart to the
// service recovery test (T-110): with only older-spec work eligible, plain
// `next` reports none, while `next --include-off-spec` returns the off-spec task
// flagged in both human and --json output.
func TestNextIncludeOffSpecRecoversOlderSpecTask(t *testing.T) {
	root := setupRepo(t)
	activateSecondSpec(t, root)
	writeTask(t, root, "T-100", "todo", "")

	out, err := runRoot(t, "next", "--include-off-spec")
	if err != nil {
		t.Fatalf("next --include-off-spec: %v", err)
	}
	for _, want := range []string{"T-100", "off-spec", "specs/v0.1.0.md"} {
		if !strings.Contains(out, want) {
			t.Fatalf("human output missing %q: %q", want, out)
		}
	}

	root = setupRepo(t)
	activateSecondSpec(t, root)
	writeTask(t, root, "T-100", "todo", "")
	out, err = runRoot(t, "next", "--include-off-spec", "--json")
	if err != nil {
		t.Fatalf("next --include-off-spec --json: %v", err)
	}
	for _, want := range []string{`"task_id": "T-100"`, `"off_spec": true`, `"code": "selected_off_spec"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("json output missing %q: %q", want, out)
		}
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

// TestCompleteRejectsGitignoredArtifactNote is the end-to-end guard: a
// `complete --note` embedding a gitignored artifact path fails, and the repo is
// left valid, clean, and with the task still in_progress — no partial transition.
func TestCompleteRejectsGitignoredArtifactNote(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-100", "todo", "")
	if out, err := runRoot(t, "start", "T-100"); err != nil {
		t.Fatalf("start: %v (output %q)", err, out)
	}

	taskPath := filepath.Join(root, "planning", "tasks", "T-100.md")
	statePath := filepath.Join(root, "planning", "STATE.md")
	taskBefore, _ := os.ReadFile(taskPath)
	stateBefore, _ := os.ReadFile(statePath)

	badNote := "evidence at planning/artifacts/manual-test/T-100/20260101T000000Z/report.md"
	if _, err := runRoot(t, "complete", "T-100", "--note", badNote); err == nil {
		t.Fatal("expected complete to reject a note embedding a gitignored artifact path")
	}

	taskAfter, _ := os.ReadFile(taskPath)
	stateAfter, _ := os.ReadFile(statePath)
	if string(taskBefore) != string(taskAfter) {
		t.Fatal("rejected complete must not rewrite the task file")
	}
	if string(stateBefore) != string(stateAfter) {
		t.Fatal("rejected complete must not rewrite STATE.md")
	}
	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after rejected complete: %v (output %q)", err, out)
	}
}

func activateSecondSpec(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.2.0.md"), []byte("# Taskrail v0.2.0\n\n## Summary\n\nFixture spec.\n"), 0o644); err != nil {
		t.Fatalf("write second spec: %v", err)
	}
	statePath := filepath.Join(root, "planning", "STATE.md")
	state, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	updated := strings.ReplaceAll(string(state), "active_spec_version: v0.1.0", "active_spec_version: v0.2.0")
	updated = strings.ReplaceAll(updated, "active_spec_path: specs/v0.1.0.md", "active_spec_path: specs/v0.2.0.md")
	if err := os.WriteFile(statePath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write state: %v", err)
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

func TestUnblockCommand(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-100", "todo", "")
	if out, err := runRoot(t, "start", "T-100"); err != nil {
		t.Fatalf("start: %v (output %q)", err, out)
	}
	if out, err := runRoot(t, "block", "T-100", "--reason", "waiting on upstream"); err != nil {
		t.Fatalf("block: %v (output %q)", err, out)
	}

	// --json reports the todo transition; --reason is optional here (unlike block).
	out, err := runRoot(t, "unblock", "T-100", "--json")
	if err != nil {
		t.Fatalf("unblock --json: %v (output %q)", err, out)
	}
	if !strings.Contains(out, `"status": "todo"`) {
		t.Fatalf("expected todo status in json, got %q", out)
	}

	// Unblocking a non-blocked task must error (mirrors start's guard).
	if _, err := runRoot(t, "unblock", "T-100"); err == nil {
		t.Fatal("expected error when unblocking a non-blocked task")
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

	// A --title derives a slug baked into the id and matching filename.
	out, err := runRoot(t, "task", "new", "--title", "Scaffolded via CLI", "--spec-ref", "specs/v0.1.0.md#summary", "--priority", "high", "--json")
	if err != nil {
		t.Fatalf("task new: %v (output %q)", err, out)
	}
	if !strings.Contains(out, `"task_id": "T-001-scaffolded-via-cli"`) {
		t.Fatalf("expected slugged T-001 scaffolded, got %q", out)
	}
	if !strings.Contains(out, `"path": "planning/tasks/T-001-scaffolded-via-cli.md"`) {
		t.Fatalf("expected slugged path in output, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-001-scaffolded-via-cli.md")); err != nil {
		t.Fatalf("expected scaffolded task file: %v", err)
	}

	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after scaffold: %v (output %q)", err, out)
	}
}

func TestTaskNewCuratedSlugOverridesTitle(t *testing.T) {
	root := setupRepo(t)

	// An explicit --slug wins over the title-derived slug and is itself normalized.
	out, err := runRoot(t, "task", "new",
		"--title", "Curated league-strength coefficients for cross-league OVR comparability",
		"--slug", "League-Strength Coefficients",
		"--spec-ref", "specs/v0.1.0.md#summary", "--json")
	if err != nil {
		t.Fatalf("task new: %v (output %q)", err, out)
	}
	if !strings.Contains(out, `"task_id": "T-001-league-strength-coefficients"`) {
		t.Fatalf("expected curated-slug id, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-001-league-strength-coefficients.md")); err != nil {
		t.Fatalf("expected curated-slug task file: %v", err)
	}
	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after scaffold: %v (output %q)", err, out)
	}
}

// TestTaskNewCapsTitleDerivedSlugButNotExplicit exercises the length cap through
// the real flag-parsing path: it pins the `slugExplicit := --slug != ""` wiring in
// task.go, so a regression there — capping an explicit slug, or failing to cap the
// title fallback — is caught at the CLI boundary, not just the service layer.
func TestTaskNewCapsTitleDerivedSlugButNotExplicit(t *testing.T) {
	setupRepo(t)

	longTitle := "Cap the title derived slug length at roughly fifty characters boundary aware"
	out, err := runRoot(t, "task", "new", "--title", longTitle, "--spec-ref", "specs/v0.1.0.md#summary", "--json")
	if err != nil {
		t.Fatalf("task new: %v (output %q)", err, out)
	}
	if !strings.Contains(out, `"task_id": "T-001-cap-the-title-derived-slug-length-at-roughly-fifty"`) {
		t.Fatalf("expected capped title-derived id, got %q", out)
	}

	// An explicit --slug the operator curated is written verbatim, however long.
	longSlug := "an-intentionally-long-but-curated-slug-the-operator-owns-verbatim"
	out, err = runRoot(t, "task", "new", "--title", longTitle, "--slug", longSlug, "--spec-ref", "specs/v0.1.0.md#summary", "--json")
	if err != nil {
		t.Fatalf("task new: %v (output %q)", err, out)
	}
	if !strings.Contains(out, `"task_id": "T-002-`+longSlug+`"`) {
		t.Fatalf("expected verbatim explicit-slug id, got %q", out)
	}
	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after scaffold: %v (output %q)", err, out)
	}
}

func TestTaskNewSlugWithoutTitleSlugsId(t *testing.T) {
	root := setupRepo(t)

	// --slug with no --title: the slug source is the curated slug, so the id is
	// slugged even though the frontmatter title stays empty.
	out, err := runRoot(t, "task", "new", "--slug", "Curated Slug", "--spec-ref", "specs/v0.1.0.md#summary", "--json")
	if err != nil {
		t.Fatalf("task new: %v (output %q)", err, out)
	}
	if !strings.Contains(out, `"task_id": "T-001-curated-slug"`) {
		t.Fatalf("expected slug-only id, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-001-curated-slug.md")); err != nil {
		t.Fatalf("expected slug-only task file: %v", err)
	}
	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after scaffold: %v (output %q)", err, out)
	}
}

func TestTaskNewWithoutTitleOrSlugStaysBare(t *testing.T) {
	root := setupRepo(t)

	// Neither --title nor --slug: the id stays the bare T-<n> form.
	out, err := runRoot(t, "task", "new", "--spec-ref", "specs/v0.1.0.md#summary", "--json")
	if err != nil {
		t.Fatalf("task new: %v (output %q)", err, out)
	}
	if !strings.Contains(out, `"task_id": "T-001"`) {
		t.Fatalf("expected bare T-001, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-001.md")); err != nil {
		t.Fatalf("expected bare task file: %v", err)
	}
	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after scaffold: %v (output %q)", err, out)
	}
}

func TestTaskNewTransliteratesAccentedTitle(t *testing.T) {
	root := setupRepo(t)

	out, err := runRoot(t, "task", "new", "--title", "Über Fußball", "--spec-ref", "specs/v0.1.0.md#summary", "--json")
	if err != nil {
		t.Fatalf("task new: %v (output %q)", err, out)
	}
	if !strings.Contains(out, `"task_id": "T-001-ueber-fussball"`) {
		t.Fatalf("expected transliterated id, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-001-ueber-fussball.md")); err != nil {
		t.Fatalf("expected transliterated task file: %v", err)
	}
	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after scaffold: %v (output %q)", err, out)
	}
}

// TestTaskNewWarnsOnEmptyDerivedSlugToStderr pins the stream split: the bare-id
// fallback is announced on stderr while stdout stays parseable JSON, so an agent
// piping `--json` is unaffected by the warning it still sees on the terminal.
func TestTaskNewWarnsOnEmptyDerivedSlugToStderr(t *testing.T) {
	root := setupRepo(t)

	stdout, stderr, err := runRootSplit(t, "task", "new", "--slug", "!!!", "--spec-ref", "specs/v0.1.0.md#summary", "--json")
	if err != nil {
		t.Fatalf("task new: %v (stdout %q stderr %q)", err, stdout, stderr)
	}
	if !strings.Contains(stderr, "!!!") || !strings.Contains(stderr, "T-001") {
		t.Fatalf("expected empty-slug warning on stderr, got %q", stderr)
	}
	var decoded struct {
		TaskID string `json:"task_id"`
	}
	decodeMachineResult(t, stdout, &decoded)
	if decoded.TaskID != "T-001" {
		t.Fatalf("expected bare id T-001, got %q", decoded.TaskID)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-001.md")); err != nil {
		t.Fatalf("expected bare task file: %v", err)
	}
	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after bare-id fallback: %v (output %q)", err, out)
	}
}

func TestTaskNewExplicitEmptySlugOverridesTitle(t *testing.T) {
	root := setupRepo(t)

	stdout, stderr, err := runRootSplit(t, "task", "new", "--title", "Visible Title", "--slug", "", "--spec-ref", "specs/v0.1.0.md#summary", "--json")
	if err != nil {
		t.Fatalf("task new: %v (stdout %q stderr %q)", err, stdout, stderr)
	}
	if !strings.Contains(stderr, `"" produced no slug segment`) || !strings.Contains(stderr, "T-001") {
		t.Fatalf("expected explicit-empty-slug warning on stderr, got %q", stderr)
	}
	var decoded struct {
		TaskID string `json:"task_id"`
	}
	decodeMachineResult(t, stdout, &decoded)
	if decoded.TaskID != "T-001" {
		t.Fatalf("expected explicit empty slug to override title with bare id T-001, got %q", decoded.TaskID)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-001.md")); err != nil {
		t.Fatalf("expected bare task file: %v", err)
	}
}

func TestTaskNewExplicitEmptyTitleWarns(t *testing.T) {
	setupRepo(t)

	stdout, stderr, err := runRootSplit(t, "task", "new", "--title", "", "--spec-ref", "specs/v0.1.0.md#summary", "--json")
	if err != nil {
		t.Fatalf("task new: %v (stdout %q stderr %q)", err, stdout, stderr)
	}
	if !strings.Contains(stderr, `"" produced no slug segment`) {
		t.Fatalf("expected explicit-empty-title warning on stderr, got %q", stderr)
	}
	var decoded struct {
		TaskID string `json:"task_id"`
	}
	decodeMachineResult(t, stdout, &decoded)
	if decoded.TaskID != "T-001" {
		t.Fatalf("expected bare id T-001, got %q", decoded.TaskID)
	}
}

// TestTaskNewWithoutSlugSourceIsSilent guards the other side of the signal: the
// intentional bare-id case must not warn, or the warning stops meaning anything.
func TestTaskNewWithoutSlugSourceIsSilent(t *testing.T) {
	setupRepo(t)

	_, stderr, err := runRootSplit(t, "task", "new", "--spec-ref", "specs/v0.1.0.md#summary", "--json")
	if err != nil {
		t.Fatalf("task new: %v (stderr %q)", err, stderr)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected no warning for the intentional bare-id case, got %q", stderr)
	}
}

func TestVerifyFollowupEmptyTitleSlugWarnsWithoutCorruptingJSON(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-001", "completed", "")

	stdout, stderr, err := runRootSplit(t, "verify", "T-001", "--result", "fail", "--summary", "needs follow-up", "--create-followup", "--followup-title", "!!!", "--json")
	if err != nil {
		t.Fatalf("verify: %v (stdout %q stderr %q)", err, stdout, stderr)
	}
	if !strings.Contains(stderr, `"!!!" produced no slug segment`) || !strings.Contains(stderr, "T-002") {
		t.Fatalf("expected empty-slug warning on stderr, got %q", stderr)
	}
	var payload struct {
		FollowupTaskID string `json:"followup_task_id"`
	}
	decodeMachineResult(t, stdout, &payload)
	if payload.FollowupTaskID != "T-002" {
		t.Fatalf("expected bare follow-up id T-002, got %q", payload.FollowupTaskID)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-002.md")); err != nil {
		t.Fatalf("expected bare follow-up task file: %v", err)
	}
}

// TestTaskNewRejectsGitignoredArtifactTitle exercises the portability guard at the
// CLI boundary: a --title naming a concrete gitignored artifact file fails before
// any write, so the repo is never left in a state the next validate rejects.
func TestTaskNewRejectsGitignoredArtifactTitle(t *testing.T) {
	root := setupRepo(t)

	artifactPath := "planning/artifacts/verify/T-001/20260101T000000Z/report.md"
	out, err := runRoot(t, "task", "new", "--title", "Fix regression from "+artifactPath, "--spec-ref", "specs/v0.1.0.md#summary")
	if err == nil {
		t.Fatalf("expected task new to reject a gitignored artifact title, got %q", out)
	}
	if !strings.Contains(err.Error(), artifactPath) {
		t.Fatalf("expected the offending path named, got err %v output %q", err, out)
	}
	entries, err := os.ReadDir(filepath.Join(root, "planning", "tasks"))
	if err != nil {
		t.Fatalf("read tasks dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("rejected task new must not write a task file, got %d entries", len(entries))
	}
	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after rejected scaffold: %v (output %q)", err, out)
	}
}

// TestTaskRenameDeSlugsWithWarningToStderr covers the rename counterpart: an
// empty-normalizing selector strips the slug back to the bare id and says so on
// stderr, leaving stdout parseable.
func TestTaskRenameDeSlugsWithWarningToStderr(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-001-base-widget", "todo", "")

	stdout, stderr, err := runRootSplit(t, "task", "rename", "T-001-base-widget", "--slug", "!!!", "--json")
	if err != nil {
		t.Fatalf("task rename: %v (stdout %q stderr %q)", err, stdout, stderr)
	}
	if !strings.Contains(stderr, "!!!") || !strings.Contains(stderr, "T-001") {
		t.Fatalf("expected empty-slug warning on stderr, got %q", stderr)
	}
	var decoded struct {
		NewID string `json:"new_id"`
	}
	decodeMachineResult(t, stdout, &decoded)
	if decoded.NewID != "T-001" {
		t.Fatalf("expected de-slug to T-001, got %q", decoded.NewID)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-001.md")); err != nil {
		t.Fatalf("expected bare task file after de-slug: %v", err)
	}
	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after de-slug: %v (output %q)", err, out)
	}
}

func TestTaskRenameExplicitEmptySlugDeSlugs(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-001-base-widget", "todo", "")

	stdout, stderr, err := runRootSplit(t, "task", "rename", "T-001-base-widget", "--slug", "", "--json")
	if err != nil {
		t.Fatalf("task rename: %v (stdout %q stderr %q)", err, stdout, stderr)
	}
	if !strings.Contains(stderr, `"" produced no slug segment`) || !strings.Contains(stderr, "T-001") {
		t.Fatalf("expected explicit-empty-slug warning on stderr, got %q", stderr)
	}
	var decoded struct {
		NewID string `json:"new_id"`
	}
	decodeMachineResult(t, stdout, &decoded)
	if decoded.NewID != "T-001" {
		t.Fatalf("expected de-slug to T-001, got %q", decoded.NewID)
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
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-101-x.md")); !os.IsNotExist(err) {
		t.Fatalf("expected no T-101-x.md written on rejection, stat err=%v", err)
	}
}

func TestTaskNewFollowUpInheritsAndValidates(t *testing.T) {
	root := setupRepo(t)

	if _, err := runRoot(t, "task", "new", "--title", "Parent", "--spec-ref", "specs/v0.1.0.md#summary"); err != nil {
		t.Fatalf("scaffold parent: %v", err)
	}
	// Follow-up without --spec-ref inherits the parent's and wires the dependency.
	// The parent id carries its title-derived slug, so --follow-up names the full id.
	out, err := runRoot(t, "task", "new", "--title", "Child", "--follow-up", "T-001-parent", "--json")
	if err != nil {
		t.Fatalf("task new follow-up: %v (output %q)", err, out)
	}
	if !strings.Contains(out, `"task_id": "T-002-child"`) {
		t.Fatalf("expected T-002-child follow-up, got %q", out)
	}
	if !strings.Contains(out, `"spec_ref": "specs/v0.1.0.md#summary"`) {
		t.Fatalf("expected inherited spec_ref in output, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-002-child.md")); err != nil {
		t.Fatalf("expected follow-up task file: %v", err)
	}
	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after follow-up: %v (output %q)", err, out)
	}
}

func TestTaskNewRequiresSpecRefOrFollowUp(t *testing.T) {
	setupRepo(t)
	// Neither --spec-ref, --area, nor --follow-up: the RunE guard rejects before any write.
	if _, err := runRoot(t, "task", "new", "--title", "x"); err == nil {
		t.Fatal("expected error when neither --spec-ref, --area, nor --follow-up provided")
	}
}

func TestTaskNewAreaResolvesActiveSpecRef(t *testing.T) {
	root := setupRepo(t)

	// --area alone satisfies the required-one guard and resolves against the active spec.
	out, err := runRoot(t, "task", "new", "--title", "Area authored", "--area", "summary", "--json")
	if err != nil {
		t.Fatalf("task new --area: %v (output %q)", err, out)
	}
	if !strings.Contains(out, `"spec_ref": "specs/v0.1.0.md#summary"`) {
		t.Fatalf("expected resolved spec_ref in output, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-001-area-authored.md")); err != nil {
		t.Fatalf("expected area-authored task file: %v", err)
	}
	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after area scaffold: %v (output %q)", err, out)
	}
}

func TestTaskNewAreaConflictsWithSpecRef(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-100", "todo", "")

	// --area and --spec-ref are mutually exclusive; the request writes nothing.
	if _, err := runRoot(t, "task", "new", "--title", "x", "--area", "summary", "--spec-ref", "specs/v0.1.0.md#summary"); err == nil {
		t.Fatal("expected error when --area and --spec-ref are both set")
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-101-x.md")); !os.IsNotExist(err) {
		t.Fatalf("expected no T-101-x.md written on rejection, stat err=%v", err)
	}
}

// addSpecArea appends a second heading to the starter spec so a repoint has a
// real target anchor distinct from the writeTask default (#summary).
func addSpecArea(t *testing.T, root, heading string) {
	t.Helper()
	path := filepath.Join(root, "specs", "v0.1.0.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read starter spec: %v", err)
	}
	if err := os.WriteFile(path, append(body, []byte("\n## "+heading+"\n\nArea body.\n")...), 0o644); err != nil {
		t.Fatalf("append spec heading: %v", err)
	}
}

func readTaskFile(t *testing.T, root, id string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, "planning", "tasks", id+".md"))
	if err != nil {
		t.Fatalf("read task %s: %v", id, err)
	}
	return string(body)
}

func TestTaskRepointRewritesSpecRefAndValidates(t *testing.T) {
	root := setupRepo(t)
	addSpecArea(t, root, "Goals")
	writeTask(t, root, "T-001", "todo", "")

	out, err := runRoot(t, "task", "repoint", "T-001", "--area", "goals")
	if err != nil {
		t.Fatalf("task repoint: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "repointed T-001: \"specs/v0.1.0.md#summary\" -> \"specs/v0.1.0.md#goals\"") {
		t.Fatalf("expected human repoint summary, got %q", out)
	}
	if !strings.Contains(out, "validation: valid") {
		t.Fatalf("expected validation line, got %q", out)
	}
	if body := readTaskFile(t, root, "T-001"); !strings.Contains(body, "spec_ref: specs/v0.1.0.md#goals") {
		t.Fatalf("expected rewritten spec_ref, got:\n%s", body)
	}
	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after repoint: %v (output %q)", err, out)
	}
}

func TestTaskRepointJSONReportsChange(t *testing.T) {
	root := setupRepo(t)
	addSpecArea(t, root, "Goals")
	writeTask(t, root, "T-001", "todo", "")

	out, err := runRoot(t, "task", "repoint", "T-001", "--spec-ref", "specs/v0.1.0.md#goals", "--json")
	if err != nil {
		t.Fatalf("task repoint --json: %v (output %q)", err, out)
	}
	if !strings.Contains(out, `"old_spec_ref": "specs/v0.1.0.md#summary"`) {
		t.Fatalf("expected old_spec_ref in output, got %q", out)
	}
	if !strings.Contains(out, `"new_spec_ref": "specs/v0.1.0.md#goals"`) {
		t.Fatalf("expected new_spec_ref in output, got %q", out)
	}
	if !strings.Contains(out, `"applied": true`) {
		t.Fatalf("expected applied repoint, got %q", out)
	}
}

func TestTaskRepointDryRunWritesNothing(t *testing.T) {
	root := setupRepo(t)
	addSpecArea(t, root, "Goals")
	writeTask(t, root, "T-001", "todo", "")
	before := readTaskFile(t, root, "T-001")

	out, err := runRoot(t, "task", "repoint", "T-001", "--area", "goals", "--dry-run")
	if err != nil {
		t.Fatalf("task repoint dry run: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "repoint dry run") {
		t.Fatalf("expected dry-run summary, got %q", out)
	}
	if after := readTaskFile(t, root, "T-001"); after != before {
		t.Fatalf("dry run must not rewrite the task file, got:\n%s", after)
	}
}

func TestTaskRepointConflictingSelectorsRejected(t *testing.T) {
	root := setupRepo(t)
	addSpecArea(t, root, "Goals")
	writeTask(t, root, "T-001", "todo", "")

	if _, err := runRoot(t, "task", "repoint", "T-001", "--area", "goals", "--spec-ref", "specs/v0.1.0.md#goals"); err == nil {
		t.Fatal("expected error when --area and --spec-ref are both set")
	}
	if body := readTaskFile(t, root, "T-001"); !strings.Contains(body, "spec_ref: specs/v0.1.0.md#summary") {
		t.Fatalf("expected untouched spec_ref on rejection, got:\n%s", body)
	}
}

func TestTaskRepointRejectsCompletedTask(t *testing.T) {
	root := setupRepo(t)
	addSpecArea(t, root, "Goals")
	writeTask(t, root, "T-001", "completed", "")

	out, err := runRoot(t, "task", "repoint", "T-001", "--area", "goals")
	if err == nil {
		t.Fatalf("expected rejection for a completed task, output %q", out)
	}
	if body := readTaskFile(t, root, "T-001"); !strings.Contains(body, "spec_ref: specs/v0.1.0.md#summary") {
		t.Fatalf("expected untouched spec_ref on rejection, got:\n%s", body)
	}
}

func TestTaskRenameReslugsAndRewritesInboundDeps(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-001", "completed", "")
	writeTask(t, root, "T-002", "todo", "T-001")

	out, err := runRoot(t, "task", "rename", "T-001", "--slug", "Base Widget", "--json")
	if err != nil {
		t.Fatalf("task rename: %v (output %q)", err, out)
	}
	if !strings.Contains(out, `"new_id": "T-001-base-widget"`) {
		t.Fatalf("expected re-slugged new_id, got %q", out)
	}
	if !strings.Contains(out, `"applied": true`) {
		t.Fatalf("expected applied rename, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-001.md")); !os.IsNotExist(err) {
		t.Fatalf("expected old task file removed, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-001-base-widget.md")); err != nil {
		t.Fatalf("expected renamed task file: %v", err)
	}
	// The inbound dependency in T-002 must now name the new id, so validate passes.
	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after rename: %v (output %q)", err, out)
	}
}

func TestTaskRenameDryRunWritesNothing(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-001", "todo", "")

	out, err := runRoot(t, "task", "rename", "T-001", "--slug", "renamed", "--dry-run")
	if err != nil {
		t.Fatalf("task rename dry run: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "rename dry run") {
		t.Fatalf("expected dry-run summary, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-001.md")); err != nil {
		t.Fatalf("dry run must not move the file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-001-renamed.md")); !os.IsNotExist(err) {
		t.Fatalf("dry run must not write the target, stat err=%v", err)
	}
}

func TestTaskRenameRejectsInvalidPreviewWithoutWrites(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-001", "completed", "")
	writeTask(t, root, "T-002", "todo", "T-001")
	writeTask(t, root, "T-003", "todo", "")

	brokenPath := filepath.Join(root, "planning", "tasks", "T-003.md")
	broken := strings.Replace(readTaskFile(t, root, "T-003"), "specs/v0.1.0.md#summary", "specs/v9.9.9.md#missing", 1)
	if err := os.WriteFile(brokenPath, []byte(broken), 0o644); err != nil {
		t.Fatalf("break sibling spec_ref: %v", err)
	}
	targetBefore := readTaskFile(t, root, "T-001")
	inboundBefore := readTaskFile(t, root, "T-002")
	statePath := filepath.Join(root, "planning", "STATE.md")
	stateBefore, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read STATE.md: %v", err)
	}

	out, err := runRoot(t, "task", "rename", "T-001", "--slug", "renamed")
	if err == nil {
		t.Fatalf("expected invalid preview rejection, output %q", out)
	}
	if !strings.Contains(err.Error(), "T-003") {
		t.Fatalf("expected sibling violation in error, got %v", err)
	}
	if got := readTaskFile(t, root, "T-001"); got != targetBefore {
		t.Fatalf("target changed after rejection:\n%s", got)
	}
	if got := readTaskFile(t, root, "T-002"); got != inboundBefore {
		t.Fatalf("inbound dependent changed after rejection:\n%s", got)
	}
	stateAfter, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read STATE.md after rejection: %v", err)
	}
	if string(stateAfter) != string(stateBefore) {
		t.Fatalf("STATE.md changed after rejection:\n%s", stateAfter)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-001-renamed.md")); !os.IsNotExist(err) {
		t.Fatalf("rejected rename created destination, stat err=%v", err)
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
	if !strings.Contains(out, "Add search endpoint") {
		t.Fatalf("unexpected preview output: %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "imports")); !os.IsNotExist(err) {
		t.Fatalf("preview must not write imports dir, stat err=%v", err)
	}

	// Emit-prompt hands the semantic lift to an agent; still no LLM call, no writes.
	promptOut, err := runRoot(t, "import", "notes.md", "--to", "tasks", "--emit-prompt")
	if err != nil {
		t.Fatalf("import emit-prompt: %v (output %q)", err, promptOut)
	}
	if !strings.Contains(promptOut, "Taskrail import prompt") || !strings.Contains(promptOut, "Add search endpoint") {
		t.Fatalf("unexpected emit-prompt output: %q", promptOut)
	}

	// Apply ingests an agent-produced draft and scaffolds real task files.
	draft := `{
  "schema_version": 1,
  "target": "tasks",
  "source": "notes.md",
  "tasks": [
    {"key": "search", "title": "Add search endpoint", "spec_ref": "specs/v0.1.0.md#summary", "priority": "high"}
  ]
}`
	if err := os.WriteFile(filepath.Join(root, "draft.json"), []byte(draft), 0o644); err != nil {
		t.Fatalf("write draft: %v", err)
	}
	applyOut, err := runRoot(t, "import", "--apply", "draft.json")
	if err != nil {
		t.Fatalf("import apply: %v (output %q)", err, applyOut)
	}
	if !strings.Contains(applyOut, "created T-") {
		t.Fatalf("expected created task in output, got %q", applyOut)
	}
	if _, err := os.Stat(filepath.Join(root, "planning", "tasks", "T-001-add-search-endpoint.md")); err != nil {
		t.Fatalf("expected scaffolded task file: %v", err)
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

// TestRepairCommandDryRunThenApply drives the repair surface end to end: an
// in_progress task with an empty current_task pointer is a mechanical drift the
// dry run reports without writing, then --apply reconciles STATE.md and validates.
func TestRepairCommandDryRunThenApply(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-001", "in_progress", "")

	// The seeded drift makes validation fail first.
	if _, err := runRoot(t, "validate"); err == nil {
		t.Fatal("expected validate to fail on the seeded drift")
	}

	statePath := filepath.Join(root, "planning", "STATE.md")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}

	out, err := runRoot(t, "repair")
	if err != nil {
		t.Fatalf("repair dry run: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "repair dry run") ||
		!strings.Contains(out, "current_task") ||
		!strings.Contains(out, "re-run with --apply") {
		t.Fatalf("unexpected repair dry-run output: %q", out)
	}
	if after, _ := os.ReadFile(statePath); string(after) != string(before) {
		t.Fatal("dry run wrote to STATE.md")
	}

	out, err = runRoot(t, "repair", "--apply")
	if err != nil {
		t.Fatalf("repair apply: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "repair applied") || !strings.Contains(out, "validation: valid") {
		t.Fatalf("unexpected repair apply output: %q", out)
	}
	if _, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after repair: %v", err)
	}
}
