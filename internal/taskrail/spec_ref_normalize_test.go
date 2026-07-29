package taskrail

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// canonicalRef is the one spelling every writer must persist for the fixture
// spec's only anchor.
const canonicalRef = "specs/v0.1.0.md#summary"

// specRefSpellings are equivalent spellings of canonicalRef that validate accepts
// today and that used to land verbatim in task frontmatter.
var specRefSpellings = []string{
	"specs//v0.1.0.md#summary",
	"./specs/v0.1.0.md#summary",
	"specs/./v0.1.0.md#summary",
	"specs/sub/../v0.1.0.md#summary",
}

func TestNormalizeSpecRefCanonicalizesPathAndKeepsAnchor(t *testing.T) {
	t.Parallel()

	for _, spelling := range specRefSpellings {
		got, err := normalizeSpecRef(spelling)
		if err != nil {
			t.Fatalf("normalize %q: %v", spelling, err)
		}
		if got != canonicalRef {
			t.Fatalf("normalize %q = %q, want %q", spelling, got, canonicalRef)
		}
	}

	// The anchor half is opaque: only the path half is canonicalized.
	got, err := normalizeSpecRef("./specs/v0.1.0.md#Odd Anchor/With#Hash")
	if err != nil {
		t.Fatalf("normalize anchor case: %v", err)
	}
	if want := "specs/v0.1.0.md#Odd Anchor/With#Hash"; got != want {
		t.Fatalf("normalize anchor case = %q, want %q", got, want)
	}
}

// TestNormalizeSpecRefRejectsWhatParseRejects locks the "never widens what is
// accepted" acceptance: normalization runs through parseSpecRef, so the traversal
// guard and the shape guards still reject.
func TestNormalizeSpecRefRejectsWhatParseRejects(t *testing.T) {
	t.Parallel()

	for _, bad := range []string{
		"../outside.md#summary",
		"specs/../../outside.md#summary",
		"specs/v0.1.0.md",
		"#summary",
		"specs/v0.1.0.md#",
	} {
		if _, err := normalizeSpecRef(bad); err == nil {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
}

func TestCreateTaskNormalizesSpecRefOnWrite(t *testing.T) {
	t.Parallel()

	for _, spelling := range specRefSpellings {
		repo := seedFixtureRepo(t)
		svc := newTestService(t, repo, time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC))

		result, err := svc.CreateTask(CreateTaskInput{Title: "Normalized", SpecRef: spelling})
		if err != nil {
			t.Fatalf("create task with %q: %v", spelling, err)
		}
		if result.SpecRef != canonicalRef {
			t.Fatalf("reported spec_ref for %q = %q, want %q", spelling, result.SpecRef, canonicalRef)
		}
		assertPersistedSpecRef(t, svc, result.TaskID, canonicalRef)
	}
}

// TestCreateTaskNormalizesAreaResolvedSpecRef covers the --area writer: the
// resolved reference is built from STATE.md's active_spec_path, so an
// un-normalized active path must not leak into task frontmatter either.
func TestCreateTaskNormalizesAreaResolvedSpecRef(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeFixtureStateWithActivePath(t, repo, "./specs/v0.1.0.md")
	svc := newTestService(t, repo, time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC))

	result, err := svc.CreateTask(CreateTaskInput{Title: "Area scoped", Area: "summary"})
	if err != nil {
		t.Fatalf("create task with area: %v", err)
	}
	if result.SpecRef != canonicalRef {
		t.Fatalf("reported spec_ref = %q, want %q", result.SpecRef, canonicalRef)
	}
	assertPersistedSpecRef(t, svc, result.TaskID, canonicalRef)
}

// TestCreateTaskStillRejectsTraversalSpecRef locks that normalization did not
// widen the write path: a traversal-shaped reference is rejected before any write.
func TestCreateTaskStillRejectsTraversalSpecRef(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC))

	if _, err := svc.CreateTask(CreateTaskInput{Title: "Escape", SpecRef: "../outside.md#summary"}); err == nil {
		t.Fatal("expected traversal spec_ref to be rejected")
	} else if !strings.Contains(err.Error(), "within the repository") {
		t.Fatalf("expected traversal guard error, got %v", err)
	}
}

func TestRepointTaskNormalizesSpecRefOnWrite(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"), "# Taskrail v0.2.0\n\n## Legacy Area\n\nz\n")
	writeTask(t, repo, "T-001", "Drifted item", "todo", "high", "specs/v0.2.0.md#legacy-area", nil)
	svc := newTestService(t, repo, time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC))

	result, err := svc.RepointTask(RepointTaskInput{TaskID: "T-001", SpecRef: "./specs//v0.1.0.md#summary"})
	if err != nil {
		t.Fatalf("repoint: %v", err)
	}
	if result.NewSpecRef != canonicalRef {
		t.Fatalf("reported new spec_ref = %q, want %q", result.NewSpecRef, canonicalRef)
	}
	assertPersistedSpecRef(t, svc, "T-001", canonicalRef)
}

// TestRepointTaskRejectsNoOpAcrossSpellings is the concrete behavioral
// consequence T-130 names: with a canonical stored form, re-pointing to another
// spelling of the same reference is recognized as the no-op it is, instead of
// rewriting the task file, STATE.md, and updated_at for no change.
func TestRepointTaskRejectsNoOpAcrossSpellings(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "Settled item", "todo", "high", canonicalRef, nil)
	svc := newTestService(t, repo, time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC))

	taskPath := filepath.Join(repo, "planning", "tasks", "T-001.md")
	statePath := filepath.Join(repo, "planning", "STATE.md")
	taskBefore := readBytes(t, taskPath)
	stateBefore := readBytes(t, statePath)

	_, err := svc.RepointTask(RepointTaskInput{TaskID: "T-001", SpecRef: "./specs//v0.1.0.md#summary"})
	if err == nil {
		t.Fatal("expected a no-op repoint across spellings to be rejected")
	}
	if !strings.Contains(err.Error(), "already points at") {
		t.Fatalf("expected no-op guard error, got %v", err)
	}
	if got := readBytes(t, taskPath); got != taskBefore {
		t.Fatalf("expected task file untouched, got:\n%s", got)
	}
	if got := readBytes(t, statePath); got != stateBefore {
		t.Fatalf("expected STATE.md untouched, got:\n%s", got)
	}
}

func TestApplyImportDraftNormalizesSpecRefOnWrite(t *testing.T) {
	t.Parallel()

	svc := applyFixture(t)
	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "tasks",
		Source:        "notes.md",
		Tasks: []TaskDraft{
			{Key: "alpha", Title: "Alpha task", SpecRef: "./specs/v0.1.0.md#summary"},
			{Key: "beta", Title: "Beta task", SpecRef: "specs//v0.1.0.md#summary"},
		},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/draft.json", draft)

	result, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel})
	if err != nil {
		t.Fatalf("apply draft: %v", err)
	}
	if len(result.Tasks) != 2 {
		t.Fatalf("expected 2 created tasks, got %d", len(result.Tasks))
	}
	for _, created := range result.Tasks {
		assertPersistedSpecRef(t, svc, created.TaskID, canonicalRef)
	}
}

// TestValidateAcceptsStoredUnnormalizedSpecRef locks that this is a write-path
// fix, not a migration: an existing task file carrying an un-normalized reference
// keeps validating and is never rewritten in bulk.
func TestValidateAcceptsStoredUnnormalizedSpecRef(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "Legacy item", "todo", "high", "./specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC))

	result, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected stored un-normalized spec_ref to stay valid, got %+v", result.Violations)
	}
	assertPersistedSpecRef(t, svc, "T-001", "./specs/v0.1.0.md#summary")
}

func assertPersistedSpecRef(t *testing.T, svc *Service, taskID, want string) {
	t.Helper()
	_, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	task, ok := taskByID(tasks, taskID)
	if !ok {
		t.Fatalf("task %s not found", taskID)
	}
	if task.Frontmatter.SpecRef != want {
		t.Fatalf("persisted spec_ref for %s = %q, want %q", taskID, task.Frontmatter.SpecRef, want)
	}
}

// writeFixtureStateWithActivePath rewrites the fixture STATE.md with a chosen
// active_spec_path spelling, so the --area writer can be exercised against an
// un-normalized active path.
func writeFixtureStateWithActivePath(t *testing.T, repo, activePath string) {
	t.Helper()
	writeFile(t, filepath.Join(repo, "planning", "STATE.md"), `---
schema_version: 1
updated_at: "2026-03-31T00:00:00Z"
active_spec_version: v0.1.0
active_spec_path: `+activePath+`
current_task: ""
current_task_title: ""
status_summary: idle
blockers: []
next_action: Start the next task
last_verification_result: Not yet run
relevant_artifacts: []
continuation_notes:
  - Fixture repo.
---

# STATE
`)
}
