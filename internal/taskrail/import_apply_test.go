package taskrail

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestReadImportDraftErrorOmitsAbsolutePath locks the portable-error contract for
// an absolute --draft argument: the missing-file error names a repo-relative path,
// never the caller's absolute repository location.
func TestReadImportDraftErrorOmitsAbsolutePath(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC))

	abs := filepath.Join(repo, "planning", "imports", "missing.json")
	if _, err := svc.readImportDraft(abs); err == nil {
		t.Fatal("expected error for a missing import draft")
	} else if strings.Contains(err.Error(), repo) {
		t.Fatalf("error leaks absolute repo path %q: %v", repo, err)
	}
}

// applyFixture seeds a repo with an existing spec and no tasks, ready to ingest
// an agent-produced draft through ApplyImportDraft.
func applyFixture(t *testing.T) *Service {
	t.Helper()
	repo := seedFixtureRepo(t)
	return newTestService(t, repo, time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC))
}

// writeDraftFile marshals a draft to JSON under the repo and returns the
// repo-relative path an --apply run would receive.
func writeDraftFile(t *testing.T, repo, rel string, draft ImportDraft) string {
	t.Helper()
	data, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		t.Fatalf("marshal draft: %v", err)
	}
	path := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir draft parent: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write draft: %v", err)
	}
	return rel
}

func TestApplyImportDraftCreatesTasksInDependencyOrder(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)

	// beta is listed first but depends on alpha; apply must create alpha first
	// and translate beta's in-draft key dependency to alpha's real task id.
	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "tasks",
		Source:        "notes.md",
		Tasks: []TaskDraft{
			{Key: "beta", Title: "Beta task", SpecRef: "specs/v0.1.0.md#summary", Priority: "high", Dependencies: []string{"alpha"}},
			{Key: "alpha", Title: "Alpha task", SpecRef: "specs/v0.1.0.md#summary"},
		},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/draft.json", draft)

	result, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(result.Tasks) != 2 {
		t.Fatalf("expected 2 created tasks, got %d", len(result.Tasks))
	}
	if result.Tasks[0].Key != "alpha" || result.Tasks[1].Key != "beta" {
		t.Fatalf("tasks must be created in dependency order (alpha before beta), got %+v", result.Tasks)
	}

	alphaID := result.Tasks[0].TaskID
	betaID := result.Tasks[1].TaskID
	if alphaID != "T-001-alpha-task" || betaID != "T-002-beta-task" {
		t.Fatalf("expected title-derived task ids, got %q and %q", alphaID, betaID)
	}
	for _, created := range result.Tasks {
		wantPath := "planning/tasks/" + created.TaskID + ".md"
		if created.Path != wantPath {
			t.Fatalf("task %s path = %q, want %q", created.TaskID, created.Path, wantPath)
		}
	}

	_, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	beta, ok := taskByID(tasks, betaID)
	if !ok {
		t.Fatalf("beta task %s not persisted", betaID)
	}
	if len(beta.Frontmatter.Dependencies) != 1 || beta.Frontmatter.Dependencies[0] != alphaID {
		t.Fatalf("beta dependency must be translated to alpha's real id %s, got %v", alphaID, beta.Frontmatter.Dependencies)
	}
	if beta.Frontmatter.Priority != "high" {
		t.Fatalf("beta priority must be preserved, got %q", beta.Frontmatter.Priority)
	}
}

func TestApplyImportDraftWarnsWhenTitleProducesEmptySlug(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)
	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "tasks",
		Source:        "notes.md",
		Tasks: []TaskDraft{{
			Key:      "empty",
			Title:    "!!!",
			SpecRef:  "specs/v0.1.0.md#summary",
			Priority: "medium",
		}},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/empty-slug.json", draft)

	result, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].TaskID != "T-001" || result.Tasks[0].Path != "planning/tasks/T-001.md" {
		t.Fatalf("expected legitimate bare-id fallback, got %+v", result.Tasks)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0].Message, `"!!!" produced no slug segment`) {
		t.Fatalf("expected visible empty-slug warning, got %+v", result.Warnings)
	}
	validation, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !validation.Valid {
		t.Fatalf("expected valid repo after bare-id fallback, got %v", validation.Violations)
	}
}

func TestApplyImportDraftRejectsMultilineTaskTitle(t *testing.T) {
	t.Parallel()

	svc := applyFixture(t)
	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "tasks",
		Source:        "notes.md",
		Tasks: []TaskDraft{{
			Key:      "injected",
			Title:    "Injected title\n## Acceptance",
			SpecRef:  "specs/v0.1.0.md#summary",
			Priority: "medium",
		}},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/injected-title.json", draft)
	before := snapshotTree(t, svc.paths.RepoRoot)
	if _, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel}); err == nil {
		t.Fatal("expected multiline import title rejection")
	}
	if got := snapshotTree(t, svc.paths.RepoRoot); !maps.Equal(before, got) {
		t.Fatal("rejected import title changed the repository")
	}
}

func TestApplyImportDraftWritesSpecSections(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)

	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "spec",
		Source:        "feature.md",
		SpecSections: []SpecSectionDraft{
			{Heading: "Overview", Body: "Some overview."},
			{Heading: "Details", Body: "More detail."},
		},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/spec.json", draft)

	result, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if result.SpecPath == "" {
		t.Fatal("expected a written spec path")
	}
	data, err := os.ReadFile(filepath.Join(svc.paths.RepoRoot, result.SpecPath))
	if err != nil {
		t.Fatalf("read written spec: %v", err)
	}
	spec := string(data)
	if !strings.Contains(spec, "## Overview") || !strings.Contains(spec, "Some overview.") {
		t.Fatalf("spec must carry the imported sections, got:\n%s", spec)
	}
}

// A destination that cannot be snapshotted rejects the complete candidate before
// publication. Import never reports an attempted spec as a partial success.
func TestApplyImportDraftRefusesFailedSpecDestinationBeforePublication(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)

	// A directory at the target path reliably makes os.WriteFile fail after the
	// collision guard and parent-directory setup have passed.
	target := filepath.Join(svc.paths.SpecsDir, "failed.md")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("block imported spec write: %v", err)
	}
	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "spec",
		Source:        "failed.md",
		SpecSections:  []SpecSectionDraft{{Heading: "Overview", Body: "x"}},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/failed-write.json", draft)

	result, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel})
	if err == nil {
		t.Fatal("expected imported spec write to fail")
	}
	if result.SpecPath != "" || len(result.Tasks) != 0 {
		t.Fatalf("failed publication must not report partial output, got %+v", result)
	}
	if _, statErr := os.Stat(filepath.Join(svc.paths.RepoRoot, "specs", "failed.md")); statErr != nil {
		t.Fatalf("blocked destination must remain untouched: %v", statErr)
	}
}

// An empty file can survive a failed os.WriteFile after truncation. A retry may
// refuse to overwrite it, but the error must name the residual path so the
// operator knows exactly what to review or remove.
func TestApplyImportDraftRetryAfterEmptySpecNamesResidualFile(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)

	residual := filepath.Join(svc.paths.SpecsDir, "failed.md")
	if err := os.WriteFile(residual, nil, 0o644); err != nil {
		t.Fatalf("seed empty residual spec: %v", err)
	}
	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "spec",
		Source:        "failed.md",
		SpecSections:  []SpecSectionDraft{{Heading: "Overview", Body: "retry"}},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/retry-empty.json", draft)

	_, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel})
	if err == nil {
		t.Fatal("retry must not silently overwrite an unmarked residual spec")
	}
	if !strings.Contains(err.Error(), "specs/failed.md") {
		t.Fatalf("retry error must name the residual spec path, got %v", err)
	}
}

func TestApplyImportDraftDoesNotClobberExistingSpec(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)

	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "spec",
		Source:        "v0.1.0.md", // collides with the fixture's existing spec
		SpecSections:  []SpecSectionDraft{{Heading: "Overview", Body: "x"}},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/spec.json", draft)

	if _, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel}); err == nil {
		t.Fatal("apply must refuse to overwrite an existing spec file")
	}
}

// A draft whose task fails a live-repo check must leave the repository
// unchanged: two-phase validation (T-041) pre-flights every task's live checks
// before any spec or task is written, so a failure writes nothing — no orphan
// spec, no partial tasks.
func TestApplyImportDraftLeavesRepoUnchangedOnLiveCheckFailure(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)

	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "planning",
		Source:        "feature.md",
		SpecSections:  []SpecSectionDraft{{Heading: "Overview", Body: "x"}},
		// spec_ref anchor does not exist on the referenced on-disk spec: the
		// live check fails, and pre-flight must reject before any write.
		Tasks: []TaskDraft{{Key: "t", Title: "T", SpecRef: "specs/v0.1.0.md#does-not-exist"}},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/partial.json", draft)

	result, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel})
	if err == nil {
		t.Fatal("expected error when a task fails a live-repo check")
	}
	if result.SpecPath != "" || len(result.Tasks) != 0 {
		t.Fatalf("failed apply must report no written artifacts, got %+v", result)
	}
	if _, statErr := os.Stat(filepath.Join(svc.paths.RepoRoot, "specs", "feature.md")); !os.IsNotExist(statErr) {
		t.Fatalf("no orphan spec must be written on a failed apply, stat err: %v", statErr)
	}
	_, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("no tasks must be created on a failed apply, got %d", len(tasks))
	}
}

// A draft title embedding a concrete gitignored artifact path fails CreateTask's
// portability guard, so pre-flight must mirror that check: otherwise an earlier,
// clean task in the same draft is already written when the later one fails,
// breaking the no-partial-tasks invariant.
func TestApplyImportDraftLeavesRepoUnchangedOnUnportableTitle(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)

	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "tasks",
		Source:        "notes.md",
		Tasks: []TaskDraft{
			{Key: "clean", Title: "Clean task", SpecRef: "specs/v0.1.0.md#summary"},
			{Key: "unportable", Title: "See " + gitignoredNotePath, SpecRef: "specs/v0.1.0.md#summary"},
		},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/unportable.json", draft)

	result, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel})
	if err == nil {
		t.Fatal("expected error when a draft title embeds a gitignored artifact path")
	}
	if !strings.Contains(err.Error(), gitignoredNotePath) {
		t.Fatalf("error should name the offending path, got %v", err)
	}
	if len(result.Tasks) != 0 {
		t.Fatalf("failed apply must report no written artifacts, got %+v", result)
	}
	_, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("no tasks must be created on a failed apply, got %d", len(tasks))
	}
}

// A collision anywhere in the planned import is found during the transaction
// snapshot, before the state, spec, or preceding task can be published.
func TestApplyImportDraftRefusesTaskCollisionBeforePublication(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)

	// Block the second task's file path with a directory: its write fails while
	// the first task is already on disk. loadTasks skips directories, so id
	// allocation still hands the second draft task the T-002 numeric prefix.
	if err := os.MkdirAll(filepath.Join(svc.paths.TasksDir, "T-002-beta-task.md"), 0o755); err != nil {
		t.Fatalf("seed blocking directory: %v", err)
	}

	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "planning",
		Source:        "notes.md",
		SpecSections:  []SpecSectionDraft{{Heading: "Overview", Body: "x"}},
		Tasks: []TaskDraft{
			{Key: "alpha", Title: "Alpha task", SpecRef: "specs/v0.1.0.md#summary"},
			{Key: "beta", Title: "Beta task", SpecRef: "specs/v0.1.0.md#summary"},
		},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/midwrite.json", draft)

	result, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel})
	if err == nil {
		t.Fatal("expected error when a task file write fails mid-loop")
	}
	if result.SpecPath != "" || len(result.Tasks) != 0 {
		t.Fatalf("collision must not report partial output, got %+v", result)
	}
	if _, statErr := os.Stat(filepath.Join(svc.paths.RepoRoot, "specs", "notes.md")); !os.IsNotExist(statErr) {
		t.Fatalf("no spec may be published before a task collision: %v", statErr)
	}
	_, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("no task may be published before a task collision, got %d", len(tasks))
	}
}

func TestApplyImportDraftRefusesSourceChangeBeforePublication(t *testing.T) {
	svc := applyFixture(t)
	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "planning",
		Source:        "feature.md",
		SpecSections:  []SpecSectionDraft{{Heading: "Overview", Body: "x"}},
		Tasks:         []TaskDraft{{Key: "task", Title: "Task", SpecRef: "specs/feature.md#overview"}},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/race.json", draft)
	called := false
	installLifecycleHook(t, func() {
		called = true
		if err := os.WriteFile(filepath.Join(svc.paths.RepoRoot, rel), []byte(`{"schema_version":1,"target":"tasks","source":"changed.md","tasks":[{"key":"changed","title":"Changed","spec_ref":"specs/v0.1.0.md#summary"}]}`), 0o644); err != nil {
			t.Fatalf("change source draft: %v", err)
		}
	})

	result, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel})
	if err == nil {
		t.Fatal("expected source race to refuse publication")
	}
	if !called {
		t.Fatal("import did not validate its candidate under the transaction")
	}
	if failure := MachineFailureFor(err); failure.Code != MachineCodeWriteConflict {
		t.Fatalf("source race code = %q, want %q", failure.Code, MachineCodeWriteConflict)
	}
	if result.SpecPath != "" || len(result.Tasks) != 0 {
		t.Fatalf("source race must not report publication, got %+v", result)
	}
	if _, statErr := os.Stat(filepath.Join(svc.paths.RepoRoot, "specs", "feature.md")); !os.IsNotExist(statErr) {
		t.Fatalf("source race published a spec: %v", statErr)
	}
	if _, tasks, loadErr := svc.loadStateAndTasks(); loadErr != nil || len(tasks) != 0 {
		t.Fatalf("source race published tasks: tasks=%d err=%v", len(tasks), loadErr)
	}
}

func TestApplyImportDraftRefusesDestinationChangeBeforeTransactionSnapshot(t *testing.T) {
	svc := applyFixture(t)
	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "spec",
		Source:        "feature.md",
		SpecSections:  []SpecSectionDraft{{Heading: "Overview", Body: "candidate"}},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/destination-race.json", draft)
	called := false
	testHookImportCandidateBuilt = func() {
		called = true
		if err := os.WriteFile(filepath.Join(svc.paths.SpecsDir, "feature.md"), []byte("# authored\n"), 0o644); err != nil {
			t.Fatalf("create raced destination: %v", err)
		}
	}
	t.Cleanup(func() { testHookImportCandidateBuilt = nil })

	result, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel})
	if err == nil {
		t.Fatal("expected destination race to refuse publication")
	}
	if !called {
		t.Fatal("import did not compare candidate inputs before transaction snapshot")
	}
	if failure := MachineFailureFor(err); failure.Code != MachineCodeWriteConflict {
		t.Fatalf("destination race code = %q, want %q", failure.Code, MachineCodeWriteConflict)
	} else if len(failure.Snapshots) == 0 {
		t.Fatal("destination race must retain transaction snapshots")
	}
	if result.SpecPath != "" || len(result.Tasks) != 0 {
		t.Fatalf("destination race must not report publication, got %+v", result)
	}
	data, readErr := os.ReadFile(filepath.Join(svc.paths.SpecsDir, "feature.md"))
	if readErr != nil || string(data) != "# authored\n" {
		t.Fatalf("destination race overwrote authored bytes: %q, err=%v", data, readErr)
	}
}

// The legitimate planning case: a task's spec_ref points at a heading in the
// spec the same apply is about to write. Pre-flight must resolve that anchor
// against the draft's pending spec sections (not only the on-disk file, which
// does not exist yet) so the apply still succeeds.
func TestApplyImportDraftResolvesSpecRefAgainstPendingImportedSpec(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)

	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "planning",
		Source:        "feature.md",
		SpecSections:  []SpecSectionDraft{{Heading: "Overview", Body: "x"}},
		// Anchor resolves only against the pending imported spec (specs/feature.md).
		Tasks: []TaskDraft{{Key: "t", Title: "T", SpecRef: "specs/feature.md#overview"}},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/pending.json", draft)

	result, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel})
	if err != nil {
		t.Fatalf("apply must succeed for a task referencing the pending spec heading: %v", err)
	}
	if result.SpecPath == "" || len(result.Tasks) != 1 {
		t.Fatalf("expected a written spec and one task, got %+v", result)
	}
	if _, statErr := os.Stat(filepath.Join(svc.paths.RepoRoot, result.Tasks[0].Path)); statErr != nil {
		t.Fatalf("task file must exist: %v", statErr)
	}
}

// Retry after a prior partial apply: an orphan spec written by a previous import
// (carrying the import marker) must be overwritten so a corrected re-apply
// succeeds. Authored specs (no marker) stay protected — see
// TestApplyImportDraftDoesNotClobberExistingSpec.
func TestApplyImportDraftOverwritesOrphanImportedSpec(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)

	// Simulate an orphan left by an earlier import: a spec at the target path
	// carrying the import marker.
	orphan := filepath.Join(svc.paths.RepoRoot, "specs", "feature.md")
	if err := os.WriteFile(orphan, []byte("# feature.md\n\n"+importedSpecMarker+"\n\n## Stale\n\nold.\n"), 0o644); err != nil {
		t.Fatalf("seed orphan spec: %v", err)
	}

	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "spec",
		Source:        "feature.md",
		SpecSections:  []SpecSectionDraft{{Heading: "Overview", Body: "fresh."}},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/retry.json", draft)

	result, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel})
	if err != nil {
		t.Fatalf("apply must overwrite an orphan imported spec on retry: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(svc.paths.RepoRoot, result.SpecPath))
	if err != nil {
		t.Fatalf("read rewritten spec: %v", err)
	}
	spec := string(data)
	if !strings.Contains(spec, "## Overview") || strings.Contains(spec, "## Stale") {
		t.Fatalf("orphan spec must be replaced with the fresh sections, got:\n%s", spec)
	}
}

func TestApplyImportDraftRejectsInvalidDraft(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)

	draft := ImportDraft{SchemaVersion: 999, Target: "bogus"}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/bad.json", draft)

	if _, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel}); err == nil {
		t.Fatal("expected validation error for an invalid draft")
	}
}

// A dependency that is an existing task id (not an in-draft key) must pass
// through translateDeps unchanged onto the created task.
func TestApplyImportDraftPassesThroughExternalTaskDependency(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)

	parent, err := svc.CreateTask(CreateTaskInput{Title: "Parent", Slug: "Parent", SpecRef: "specs/v0.1.0.md#summary"})
	if err != nil {
		t.Fatalf("seed parent task: %v", err)
	}
	if parent.TaskID != "T-001-parent" {
		t.Fatalf("expected slugged parent id, got %q", parent.TaskID)
	}

	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "tasks",
		Tasks:         []TaskDraft{{Key: "child", Title: "Child", SpecRef: "specs/v0.1.0.md#summary", Dependencies: []string{parent.TaskID}}},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/ext.json", draft)

	result, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	_, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	child, ok := taskByID(tasks, result.Tasks[0].TaskID)
	if !ok {
		t.Fatalf("child task %s not persisted", result.Tasks[0].TaskID)
	}
	if len(child.Frontmatter.Dependencies) != 1 || child.Frontmatter.Dependencies[0] != parent.TaskID {
		t.Fatalf("external dependency must pass through unchanged, got %v", child.Frontmatter.Dependencies)
	}
}

func TestApplyImportDraftRejectsUnknownField(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)

	path := filepath.Join(svc.paths.RepoRoot, "planning", "imports", "malformed.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"target":"tasks","unknown_field":true}`), 0o644); err != nil {
		t.Fatalf("write malformed: %v", err)
	}
	if _, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: "planning/imports/malformed.json"}); err == nil {
		t.Fatal("expected parse error for unknown field")
	}
}

func TestApplyImportDraftRejectsMissingFile(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)
	if _, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: "planning/imports/nope.json"}); err == nil {
		t.Fatal("expected error for a missing draft file")
	}
}

func TestApplyImportDraftRejectsDependencyCycle(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)

	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "tasks",
		Tasks: []TaskDraft{
			{Key: "a", Title: "A", SpecRef: "specs/v0.1.0.md#summary", Dependencies: []string{"b"}},
			{Key: "b", Title: "B", SpecRef: "specs/v0.1.0.md#summary", Dependencies: []string{"a"}},
		},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/cycle.json", draft)

	if _, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel}); err == nil {
		t.Fatal("expected error for a dependency cycle among draft keys")
	}
}

// A dependency cycle is detected during task ordering, which the old apply ran
// only after writing the spec — leaving an orphan. Pre-flight must catch the
// cycle before any write so a cyclic draft with spec sections changes nothing.
func TestApplyImportDraftLeavesRepoUnchangedOnDependencyCycle(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)

	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "planning",
		Source:        "feature.md",
		SpecSections:  []SpecSectionDraft{{Heading: "Overview", Body: "x"}},
		Tasks: []TaskDraft{
			{Key: "a", Title: "A", SpecRef: "specs/feature.md#overview", Dependencies: []string{"b"}},
			{Key: "b", Title: "B", SpecRef: "specs/feature.md#overview", Dependencies: []string{"a"}},
		},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/cycle-spec.json", draft)

	if _, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel}); err == nil {
		t.Fatal("expected error for a dependency cycle")
	}
	if _, statErr := os.Stat(filepath.Join(svc.paths.RepoRoot, "specs", "feature.md")); !os.IsNotExist(statErr) {
		t.Fatalf("no orphan spec must be written when the draft has a cycle, stat err: %v", statErr)
	}
}

// TestApplyImportDraftRoundTripsThroughFile proves the emit/apply contract: a
// draft marshaled to disk parses back and applies, so an agent emission and the
// binary agree on exactly one schema.
func TestApplyImportDraftRoundTripsThroughFile(t *testing.T) {
	t.Parallel()
	svc := applyFixture(t)

	draft := ImportDraft{
		SchemaVersion: importDraftSchemaVersion,
		Target:        "tasks",
		Source:        "notes.md",
		Tasks:         []TaskDraft{{Key: "solo", Title: "Solo task", SpecRef: "specs/v0.1.0.md#summary"}},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/round.json", draft)

	result, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].Path == "" {
		t.Fatalf("expected one created task with a path, got %+v", result.Tasks)
	}
	if _, err := os.Stat(filepath.Join(svc.paths.RepoRoot, result.Tasks[0].Path)); err != nil {
		t.Fatalf("created task file must exist: %v", err)
	}
}

func TestApplyImportDraftV1IgnoresLegacyBodyAndUsesStandardScaffold(t *testing.T) {
	svc := applyFixture(t)
	draft := ImportDraft{
		SchemaVersion: 1,
		Target:        "tasks",
		Tasks: []TaskDraft{{
			Title: "Scaffolded outcome", SpecRef: "specs/v0.1.0.md#summary",
			Body: "LEGACY BODY MUST NOT BE PUBLISHED",
		}},
	}
	rel := writeDraftFile(t, svc.paths.RepoRoot, "planning/imports/scaffold.json", draft)
	result, err := svc.ApplyImportDraft(ApplyDraftInput{DraftPath: rel})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	data := readBytes(t, filepath.Join(svc.paths.RepoRoot, result.Tasks[0].Path))
	if strings.Contains(data, draft.Tasks[0].Body) {
		t.Fatalf("v1 apply published legacy body: %q", data)
	}
	for _, heading := range []string{"## Description", "## Acceptance", "## Verification Notes", "## Implementation Notes"} {
		if strings.Count(data, heading) != 1 {
			t.Errorf("task has %d %s headings: %q", strings.Count(data, heading), heading, data)
		}
	}
	if strings.Contains(data, "loop_policy:") || strings.Contains(data, "loop_reason:") {
		t.Fatalf("imported task must remain implicitly held: %q", data)
	}
}

func TestApplyImportDraftV2PublishesExactReviewedBodies(t *testing.T) {
	svc := applyFixture(t)
	writeFile(t, filepath.Join(svc.paths.RepoRoot, ".taskrail", "config.yml"), layout2Marker("committed", "specs", "planning"))
	svc = newTestService(t, svc.paths.RepoRoot, time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC))
	files, subjects := decompositionGolden()
	exactBody := "  ## Description  \r\n\r\nDeliver exact bytes.  \r\n\r\n### Boundary\r\n\r\nKeep CRLF.\r\n\r\n## Acceptance\r\n\r\n- Works.\r\n\r\n## Verification Notes\r\n\r\n- Test it.\r\n\r\n## Implementation Notes\r\n"
	rawBody, err := json.Marshal(exactBody)
	if err != nil {
		t.Fatal(err)
	}
	files["draft.json"] = bytes.Replace(files["draft.json"], []byte(`"body":"## Description\n\nDeliver one outcome.\n\n## Acceptance\n\n- Works.\n\n## Verification Notes\n\n- Test it."`), []byte(`"body":`+string(rawBody)), 1)
	refreshDecompositionFinalDigests(files)
	writeTask(t, svc.paths.RepoRoot, "T-240-implement-the-normative-review-schema-decoders", "Existing dependency", "completed", "medium", "specs/v0.1.0.md#summary", nil)
	writeFile(t, filepath.Join(svc.paths.RepoRoot, "specs", "v0.5.0.md"), string(subjects.Spec))
	statePath := filepath.Join(svc.paths.RepoRoot, "planning", "STATE.md")
	state := strings.Replace(readBytes(t, statePath), "active_spec_version: v0.1.0\nactive_spec_path: specs/v0.1.0.md", "active_spec_version: v0.5.0\nactive_spec_path: specs/v0.5.0.md", 1)
	writeFile(t, statePath, state)

	specReviewDir := filepath.Join(svc.paths.RepoRoot, filepath.FromSlash("planning/reviews/spec/v0.5.0/spec-review-1"))
	for name, data := range subjects.SpecReviewFiles {
		writeFile(t, filepath.Join(specReviewDir, name), string(data))
	}
	bundleDir := filepath.Join(svc.paths.RepoRoot, filepath.FromSlash("planning/reviews/decomposition/v0.5.0/decomposition-1"))
	for name, data := range files {
		writeFile(t, filepath.Join(bundleDir, name), string(data))
	}

	requireRecoveryDirectoryDurability(t, svc.paths.RepoRoot)
	result, err := svc.ApplyImportDraft(ApplyDraftInput{
		DraftPath:          "planning/reviews/decomposition/v0.5.0/decomposition-1/draft.json",
		ExpectSHA256:       digestRaw(files["draft.json"]),
		ReviewManifestPath: "planning/reviews/decomposition/v0.5.0/decomposition-1/manifest.json",
		ExpectReviewSHA256: digestRaw(files["manifest.json"]),
	})
	if err != nil {
		t.Fatalf("apply reviewed draft: %v", err)
	}
	if len(result.Tasks) != 2 {
		t.Fatalf("created tasks = %+v", result.Tasks)
	}
	for i, created := range result.Tasks {
		data := readBytes(t, filepath.Join(svc.paths.RepoRoot, created.Path))
		if !strings.Contains(data, "\n---\n\n# "+created.TaskID+" ") {
			t.Fatalf("%s is missing generated heading: %q", created.TaskID, data)
		}
		if !strings.HasSuffix(data, filesBody(t, files["draft.json"], i)) {
			t.Fatalf("%s did not preserve reviewed body bytes", created.TaskID)
		}
		if strings.Contains(data, "loop_policy:") || strings.Contains(data, "loop_reason:") {
			t.Fatalf("%s must remain implicitly held: %q", created.TaskID, data)
		}
	}
	if body := filesBody(t, files["draft.json"], 0); body != exactBody {
		t.Fatalf("decoded body = %q, want exact CRLF body %q", body, exactBody)
	}
}

func filesBody(t *testing.T, draft []byte, index int) string {
	t.Helper()
	decoded, err := decodeReviewedDraft(draft)
	if err != nil {
		t.Fatalf("decode reviewed draft: %v", err)
	}
	return decoded.Tasks[index].Body
}
