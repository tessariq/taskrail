package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func renameFixture(t *testing.T) (*Service, string) {
	t.Helper()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "Base", "completed", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002", "Dependent", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-001"})
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))
	return svc, repo
}

func taskDeps(t *testing.T, svc *Service, id string) []string {
	t.Helper()
	tasks, err := svc.loadTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	task, ok := taskByID(tasks, id)
	if !ok {
		t.Fatalf("task %s not found", id)
	}
	return task.Frontmatter.Dependencies
}

func TestRenameTaskReslugsIDFilenameAndInboundDeps(t *testing.T) {
	t.Parallel()
	svc, repo := renameFixture(t)

	res, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "Base Widget"})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if !res.Applied {
		t.Fatalf("expected applied rename, got %+v", res)
	}
	if res.NewID != "T-001-base-widget" {
		t.Fatalf("new id = %q, want T-001-base-widget", res.NewID)
	}

	if fileExists(filepath.Join(repo, "planning", "tasks", "T-001.md")) {
		t.Fatal("old task file still present")
	}
	if !fileExists(filepath.Join(repo, "planning", "tasks", "T-001-base-widget.md")) {
		t.Fatal("renamed task file missing")
	}

	if deps := taskDeps(t, svc, "T-002"); len(deps) != 1 || deps[0] != "T-001-base-widget" {
		t.Fatalf("inbound dependency not rewritten: %v", deps)
	}

	// A dependency_ref change is reported for the inbound task.
	var sawDepChange bool
	for _, ch := range res.Changes {
		if ch.Kind == "dependency_ref" && ch.TaskID == "T-002" && ch.From == "T-001" && ch.To == "T-001-base-widget" {
			sawDepChange = true
		}
	}
	if !sawDepChange {
		t.Fatalf("missing dependency_ref change: %+v", res.Changes)
	}

	if v, err := svc.Validate(); err != nil || !v.Valid {
		t.Fatalf("validate after rename: valid=%v violations=%v err=%v", v.Valid, v.Violations, err)
	}
}

func TestRenameTaskTitleDerivesSlug(t *testing.T) {
	t.Parallel()
	svc, repo := renameFixture(t)

	res, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Title: "Base Widget"})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if res.NewID != "T-001-base-widget" {
		t.Fatalf("new id = %q, want T-001-base-widget", res.NewID)
	}
	if !fileExists(filepath.Join(repo, "planning", "tasks", "T-001-base-widget.md")) {
		t.Fatal("renamed task file missing")
	}
	// --title is only a slug source; it must not rewrite the frontmatter title.
	tasks, _ := svc.loadTasks()
	task, _ := taskByID(tasks, "T-001-base-widget")
	if task.Frontmatter.Title != "Base" {
		t.Fatalf("frontmatter title changed to %q, want unchanged 'Base'", task.Frontmatter.Title)
	}
}

func TestRenameTaskPreservesNumericPrefix(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-042-old-slug", "Numbered", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	res, err := svc.RenameTask(RenameTaskInput{OldID: "T-042-old-slug", Slug: "new-slug"})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if res.NewID != "T-042-new-slug" {
		t.Fatalf("new id = %q, want T-042-new-slug", res.NewID)
	}
}

func TestRenameTaskCollisionMakesNoPartialChange(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001-alpha", "Alpha", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-001-beta", "Beta", "todo", "high", "specs/v0.1.0.md#summary", nil)
	// Impossible numeric-prefix collision guard: two T-001 ids can only exist in a
	// crafted fixture, but the collision path must still refuse cleanly.
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-001-alpha", Slug: "beta"}); err == nil {
		t.Fatal("expected collision error")
	}
	// No partial change: the source file is untouched and no target overwrite happened.
	if !fileExists(filepath.Join(repo, "planning", "tasks", "T-001-alpha.md")) {
		t.Fatal("source file lost on collision")
	}
}

func TestRenameTaskDryRunWritesNothing(t *testing.T) {
	t.Parallel()
	svc, repo := renameFixture(t)

	res, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "base-widget", DryRun: true})
	if err != nil {
		t.Fatalf("rename dry run: %v", err)
	}
	if res.Applied {
		t.Fatal("dry run must not apply")
	}
	if len(res.Changes) == 0 {
		t.Fatal("dry run should still report the planned change set")
	}
	// Nothing on disk moved and no inbound edit landed.
	if !fileExists(filepath.Join(repo, "planning", "tasks", "T-001.md")) {
		t.Fatal("dry run renamed the file")
	}
	if fileExists(filepath.Join(repo, "planning", "tasks", "T-001-base-widget.md")) {
		t.Fatal("dry run wrote the target file")
	}
	if deps := taskDeps(t, svc, "T-002"); len(deps) != 1 || deps[0] != "T-001" {
		t.Fatalf("dry run edited inbound deps: %v", deps)
	}
}

func TestRenameTaskRequiresExactlyOneSelector(t *testing.T) {
	t.Parallel()
	svc, _ := renameFixture(t)

	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-001"}); err == nil {
		t.Fatal("expected error when neither --slug nor --title is given")
	}
	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "a", Title: "b"}); err == nil {
		t.Fatal("expected error when both --slug and --title are given")
	}
}

func TestRenameTaskMissingTaskErrors(t *testing.T) {
	t.Parallel()
	svc, _ := renameFixture(t)
	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-999", Slug: "x"}); err == nil {
		t.Fatal("expected error for unknown task id")
	}
}

func TestRenameTaskRefusesWhenDestinationFileExists(t *testing.T) {
	t.Parallel()
	svc, repo := renameFixture(t)

	// A stray file already occupies the rename's target path (a filename!=id drift
	// the repair tooling exists to heal). Its content must survive: a plain
	// os.Rename would silently clobber it.
	stray := filepath.Join(repo, "planning", "tasks", "T-001-widget.md")
	strayContent := "---\nid: T-900\ntitle: Precious\nstatus: todo\npriority: low\nspec_ref: specs/v0.1.0.md#summary\ndependencies: []\nupdated_at: \"2026-01-01T00:00:00Z\"\n---\n\n# PRECIOUS DATA MUST SURVIVE\n"
	writeFile(t, stray, strayContent)

	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "Widget"}); err == nil {
		t.Fatal("expected error when the target file already exists")
	}
	got, err := os.ReadFile(stray)
	if err != nil {
		t.Fatalf("stray file lost: %v", err)
	}
	if string(got) != strayContent {
		t.Fatalf("stray file overwritten:\n%s", got)
	}
	if !fileExists(filepath.Join(repo, "planning", "tasks", "T-001.md")) {
		t.Fatal("source file lost on destination collision")
	}
}

func TestRenameTaskReprojectsStateBody(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-003-active", "Active", "in_progress", "high", "specs/v0.1.0.md#summary", nil)
	writeFixtureState(t, repo, "v0.1.0", "T-003-active", "Active", "in_progress")
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-003-active", Slug: "running"}); err != nil {
		t.Fatalf("rename: %v", err)
	}
	state, err := svc.loadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	// The rendered STATE.md body — not just the frontmatter — must reflect the new
	// id, so the Current Focus section stays consistent with the projection.
	if !strings.Contains(state.Body, "T-003-running") {
		t.Fatalf("STATE.md body not re-projected to new id:\n%s", state.Body)
	}
	if strings.Contains(state.Body, "`T-003-active`") {
		t.Fatalf("STATE.md body still shows the old id:\n%s", state.Body)
	}
}

func TestRenameTaskRejectsSlugNormalizingToEmpty(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-042-old-slug", "Slugged", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	// A garbage slug that normalizes to "" must error, not silently strip the slug
	// and convert the task to a bare id.
	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-042-old-slug", Slug: "!!!"}); err == nil {
		t.Fatal("expected error when the slug normalizes to empty")
	}
	if !fileExists(filepath.Join(repo, "planning", "tasks", "T-042-old-slug.md")) {
		t.Fatal("source file changed despite empty-slug rejection")
	}
	if fileExists(filepath.Join(repo, "planning", "tasks", "T-042.md")) {
		t.Fatal("slug silently stripped to a bare id")
	}
}

func TestRenameTaskRewritesAllInboundDependents(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "Base", "completed", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002", "Dep A", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-001"})
	writeTask(t, repo, "T-003", "Dep B", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-001"})
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "base"}); err != nil {
		t.Fatalf("rename: %v", err)
	}
	for _, id := range []string{"T-002", "T-003"} {
		if deps := taskDeps(t, svc, id); len(deps) != 1 || deps[0] != "T-001-base" {
			t.Fatalf("%s inbound dep not rewritten: %v", id, deps)
		}
	}
	if v, err := svc.Validate(); err != nil || !v.Valid {
		t.Fatalf("validate: valid=%v violations=%v err=%v", v.Valid, v.Violations, err)
	}
}

func TestRenameTaskUpdatesCurrentTaskPointer(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-003-active", "Active", "in_progress", "high", "specs/v0.1.0.md#summary", nil)
	writeFixtureState(t, repo, "v0.1.0", "T-003-active", "Active", "in_progress")
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	res, err := svc.RenameTask(RenameTaskInput{OldID: "T-003-active", Slug: "running"})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if res.NewID != "T-003-running" {
		t.Fatalf("new id = %q", res.NewID)
	}
	state, err := svc.loadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.Frontmatter.CurrentTask != "T-003-running" {
		t.Fatalf("current_task = %q, want T-003-running", state.Frontmatter.CurrentTask)
	}
	if v, err := svc.Validate(); err != nil || !v.Valid {
		t.Fatalf("validate after rename of active task: valid=%v violations=%v err=%v", v.Valid, v.Violations, err)
	}
}
