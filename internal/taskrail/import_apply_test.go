package taskrail

import (
	"encoding/json"
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
	if alphaID == "" || betaID == "" || alphaID == betaID {
		t.Fatalf("expected two distinct real task ids, got %q and %q", alphaID, betaID)
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

	parent, err := svc.CreateTask(CreateTaskInput{Title: "Parent", SpecRef: "specs/v0.1.0.md#summary"})
	if err != nil {
		t.Fatalf("seed parent task: %v", err)
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
