package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestActivateSpecRepointsAndRevalidates covers the successful write: both
// STATE.md fields move to the target version, the body is re-rendered around
// the new path, and validation is re-run and reported as valid.
func TestActivateSpecRepointsAndRevalidates(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"), "# Taskrail v0.2.0\n\n## Summary\n\nNext spec.\n")

	svc := newTestService(t, repo, time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC))
	result, err := svc.ActivateSpec("v0.2.0")
	if err != nil {
		t.Fatalf("ActivateSpec: %v", err)
	}

	if result.ActiveSpecVersion != "v0.2.0" || result.ActiveSpecPath != "specs/v0.2.0.md" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !result.Validation.Valid {
		t.Fatalf("expected valid state after activation, got violations %v", result.Validation.Violations)
	}

	state, err := svc.loadState()
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if state.Frontmatter.ActiveSpecVersion != "v0.2.0" {
		t.Fatalf("active_spec_version not persisted: %q", state.Frontmatter.ActiveSpecVersion)
	}
	if state.Frontmatter.ActiveSpecPath != "specs/v0.2.0.md" {
		t.Fatalf("active_spec_path not persisted: %q", state.Frontmatter.ActiveSpecPath)
	}
	if !strings.Contains(state.Body, "`specs/v0.2.0.md`") {
		t.Fatalf("re-rendered body missing new active spec path:\n%s", state.Body)
	}
}

// TestActivateSpecReportsCoverageForNewSpec proves activation returns the
// shared coverage report computed for the now-active spec (T-067): the figure
// reflects the target spec's areas and their linked tasks, matching what
// `taskrail coverage` would report against the same repointed state.
func TestActivateSpecReportsCoverageForNewSpec(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"),
		"# Taskrail v0.2.0\n\n## Potential Features\n\n### Alpha\n\n### Beta\n")
	// One task covers Alpha under the newly-activated spec; Beta stays uncovered.
	writeTask(t, repo, "T-001", "Cover Alpha", "todo", "high", "specs/v0.2.0.md#alpha", nil)

	svc := newTestService(t, repo, time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC))
	result, err := svc.ActivateSpec("v0.2.0")
	if err != nil {
		t.Fatalf("ActivateSpec: %v", err)
	}

	if result.Coverage.ActiveSpecPath != "specs/v0.2.0.md" {
		t.Fatalf("coverage computed for wrong spec: %q", result.Coverage.ActiveSpecPath)
	}
	if result.Coverage.CoverableAreas != 2 || result.Coverage.CoveredAreas != 1 {
		t.Fatalf("unexpected coverage counts: %d/%d", result.Coverage.CoveredAreas, result.Coverage.CoverableAreas)
	}
	if result.Coverage.Percent == nil || *result.Coverage.Percent != 50 {
		t.Fatalf("expected 50%% coverage, got %v", result.Coverage.Percent)
	}
}

// TestActivateSpecCallsOutPreviousSpecOrphans proves the migration callout
// (T-075): after the active spec moves, activation lists the non-completed tasks
// whose spec_ref still points at the *previously* active spec — the fresh
// orphans the move just created — as a subset of the shared coverage orphan
// list, without touching task files. A completed task on the previous spec is
// delivered history (not listed); a task pointing at some other non-active spec
// is an orphan but not a previous-spec orphan.
func TestActivateSpecCallsOutPreviousSpecOrphans(t *testing.T) {
	repo := seedFixtureRepo(t) // active spec v0.1.0
	writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"),
		"# Taskrail v0.2.0\n\n## Potential Features\n\n### Alpha\n")
	writeFile(t, filepath.Join(repo, "specs", "v0.0.9.md"),
		"# Taskrail v0.0.9\n\n## Summary\n\nOlder spec.\n")
	// Still on the previous spec after the move — the actionable new orphan.
	writeTask(t, repo, "T-001", "Still on v0.1.0", "todo", "high", "specs/v0.1.0.md#summary", nil)
	// Completed on the previous spec: delivered history, never an orphan.
	writeTask(t, repo, "T-002", "Delivered on v0.1.0", "completed", "high", "specs/v0.1.0.md#summary", nil)
	// Covers the newly-active spec: not an orphan at all.
	writeTask(t, repo, "T-003", "Covers Alpha", "todo", "high", "specs/v0.2.0.md#alpha", nil)
	// Points at a different non-active spec: an orphan, but not a previous-spec orphan.
	writeTask(t, repo, "T-004", "On v0.0.9", "todo", "high", "specs/v0.0.9.md#summary", nil)

	svc := newTestService(t, repo, time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC))
	result, err := svc.ActivateSpec("v0.2.0")
	if err != nil {
		t.Fatalf("ActivateSpec: %v", err)
	}

	if result.PreviousSpecPath != "specs/v0.1.0.md" {
		t.Fatalf("previous spec path = %q, want specs/v0.1.0.md", result.PreviousSpecPath)
	}
	// The callout is a strict subset of the shared coverage orphan list: both
	// T-001 (previous spec) and T-004 (other spec) are orphans, but only T-001
	// is a previous-spec orphan.
	if len(result.Coverage.Orphans) != 2 {
		t.Fatalf("expected 2 coverage orphans (T-001, T-004), got %d: %+v", len(result.Coverage.Orphans), result.Coverage.Orphans)
	}
	if len(result.PreviousSpecOrphans) != 1 {
		t.Fatalf("expected exactly one previous-spec orphan, got %d: %+v", len(result.PreviousSpecOrphans), result.PreviousSpecOrphans)
	}
	o := result.PreviousSpecOrphans[0]
	if o.TaskID != "T-001" || o.SpecRef != "specs/v0.1.0.md#summary" {
		t.Fatalf("unexpected orphan: %+v", o)
	}

	// Informational only: the task files must be untouched by the callout.
	taskBytes, err := os.ReadFile(filepath.Join(repo, "planning", "tasks", "T-001.md"))
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	if !strings.Contains(string(taskBytes), "status: todo") {
		t.Fatalf("previous-spec orphan task status must be untouched:\n%s", taskBytes)
	}
}

// TestActivateSpecRejectsMissingVersion locks the no-write contract: a
// well-formed version whose spec file does not exist is rejected and STATE.md
// is left byte-for-byte unchanged.
func TestActivateSpecRejectsMissingVersion(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC))

	statePath := filepath.Join(repo, "planning", "STATE.md")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read STATE.md: %v", err)
	}

	if _, err := svc.ActivateSpec("v9.9.9"); err == nil {
		t.Fatal("expected error activating a nonexistent spec version")
	}

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("re-read STATE.md: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("rejected activation must not write STATE.md")
	}
}

// TestActivateSpecRejectsNonConformingVersion rejects a version that does not
// follow the versioned-specs naming convention, with no write, even if a file
// with that raw name happens to exist.
func TestActivateSpecRejectsNonConformingVersion(t *testing.T) {
	repo := seedFixtureRepo(t)
	// A file named after the malformed version exists; convention still governs.
	writeFile(t, filepath.Join(repo, "specs", "garbage.md"), "# nope\n")
	svc := newTestService(t, repo, time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC))

	statePath := filepath.Join(repo, "planning", "STATE.md")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read STATE.md: %v", err)
	}

	if _, err := svc.ActivateSpec("garbage"); err == nil {
		t.Fatal("expected error for a non-conforming version name")
	}

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("re-read STATE.md: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("rejected activation must not write STATE.md")
	}
}

// TestActivateSpecLeavesTaskFilesUntouched proves activation repoints the active
// spec only: task files (status and bytes) are never rewritten.
func TestActivateSpecLeavesTaskFilesUntouched(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"), "# Taskrail v0.2.0\n\n## Summary\n\nNext spec.\n")
	writeTask(t, repo, "T-001", "Existing item", "todo", "high", "specs/v0.1.0.md#summary", nil)

	taskPath := filepath.Join(repo, "planning", "tasks", "T-001.md")
	before, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task: %v", err)
	}

	svc := newTestService(t, repo, time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC))
	if _, err := svc.ActivateSpec("v0.2.0"); err != nil {
		t.Fatalf("ActivateSpec: %v", err)
	}

	after, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("re-read task: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("activation must not rewrite task files")
	}
}

// TestActivateSpecPreservesContinuationNotes proves activate is a generic
// repoint-only writer: it does not special-case, inject, or clear continuation
// notes. Removing the one-time bootstrap note is a separate sanctioned hand-edit,
// not command logic, so pre-existing notes must survive a re-render untouched.
func TestActivateSpecPreservesContinuationNotes(t *testing.T) {
	repo := seedFixtureRepo(t) // fixture carries one note: "Fixture repo."
	writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"), "# Taskrail v0.2.0\n\n## Summary\n\nNext spec.\n")

	svc := newTestService(t, repo, time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC))
	if _, err := svc.ActivateSpec("v0.2.0"); err != nil {
		t.Fatalf("ActivateSpec: %v", err)
	}

	state, err := svc.loadState()
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if len(state.Frontmatter.ContinuationNotes) != 1 || state.Frontmatter.ContinuationNotes[0] != "Fixture repo." {
		t.Fatalf("continuation notes not preserved verbatim: %v", state.Frontmatter.ContinuationNotes)
	}
}
