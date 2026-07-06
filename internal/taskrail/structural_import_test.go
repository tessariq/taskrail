package taskrail

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const sampleNotes = `# Payments Revamp

Some preamble that is not a heading or a bullet.

## Add checkout endpoint

Accept a cart and return a payment intent.

- validate the cart total
- create the intent
  - call the gateway
  - persist the intent id

## Reconcile webhooks

Handle asynchronous settlement callbacks.
`

func writeSource(t *testing.T, repo, rel, content string) string {
	t.Helper()
	path := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir source parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	return path
}

func importFixture(t *testing.T) *Service {
	t.Helper()
	repo := seedFixtureRepo(t)
	writeSource(t, repo, "notes.md", sampleNotes)
	return newTestService(t, repo, time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC))
}

func TestImportToTasksEmitsUnitPerHeadingAndBullet(t *testing.T) {
	t.Parallel()
	svc := importFixture(t)

	result, err := svc.Import(ImportInput{SourcePath: "notes.md", Target: "tasks"})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Target != "tasks" {
		t.Fatalf("expected target tasks, got %q", result.Target)
	}
	if len(result.Draft.SpecSections) != 0 {
		t.Fatalf("expected no spec sections for --to tasks, got %d", len(result.Draft.SpecSections))
	}

	// Two subheadings (H2) plus two top-level bullets become task drafts; the H1
	// document title and the nested sub-bullets do not.
	titles := draftTitles(result.Draft.Tasks)
	want := []string{"Add checkout endpoint", "validate the cart total", "create the intent", "Reconcile webhooks"}
	if !reflect.DeepEqual(titles, want) {
		t.Fatalf("unexpected task titles\n got=%v\nwant=%v", titles, want)
	}

	// The nested sub-bullets ride along in the parent bullet's body.
	intent := taskDraftByTitle(t, result.Draft.Tasks, "create the intent")
	if !strings.Contains(intent.Body, "call the gateway") {
		t.Fatalf("expected nested bullet in body, got %q", intent.Body)
	}

	if violations := ValidateImportDraft(result.Draft); len(violations) != 0 {
		t.Fatalf("structural draft must satisfy the T-032 schema, got %v", violations)
	}
	assertUniqueKeys(t, result.Draft.Tasks)
}

func TestImportToSpecEmitsSectionPerHeading(t *testing.T) {
	t.Parallel()
	svc := importFixture(t)

	result, err := svc.Import(ImportInput{SourcePath: "notes.md", Target: "spec"})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(result.Draft.Tasks) != 0 {
		t.Fatalf("expected no task drafts for --to spec, got %d", len(result.Draft.Tasks))
	}

	headings := make([]string, 0, len(result.Draft.SpecSections))
	for _, s := range result.Draft.SpecSections {
		headings = append(headings, s.Heading)
	}
	want := []string{"Payments Revamp", "Add checkout endpoint", "Reconcile webhooks"}
	if !reflect.DeepEqual(headings, want) {
		t.Fatalf("unexpected spec headings\n got=%v\nwant=%v", headings, want)
	}

	first := result.Draft.SpecSections[1]
	if !strings.Contains(first.Body, "Accept a cart") {
		t.Fatalf("expected section body captured, got %q", first.Body)
	}
	if violations := ValidateImportDraft(result.Draft); len(violations) != 0 {
		t.Fatalf("spec draft must satisfy the schema, got %v", violations)
	}
}

func TestImportToPlanningCombinesAndSeedsState(t *testing.T) {
	t.Parallel()
	svc := importFixture(t)

	result, err := svc.Import(ImportInput{SourcePath: "notes.md", Target: "planning"})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(result.Draft.Tasks) == 0 || len(result.Draft.SpecSections) == 0 {
		t.Fatalf("planning bootstrap must carry both tasks and spec sections, got %d tasks / %d sections",
			len(result.Draft.Tasks), len(result.Draft.SpecSections))
	}
	if strings.TrimSpace(result.StateSeed) == "" {
		t.Fatal("planning bootstrap must include a STATE seed")
	}
	if !strings.Contains(result.StateSeed, "todo:") {
		t.Fatalf("STATE seed should summarize task counts, got %q", result.StateSeed)
	}
	if violations := ValidateImportDraft(result.Draft); len(violations) != 0 {
		t.Fatalf("planning draft must satisfy the schema, got %v", violations)
	}
}

func TestImportPreviewDoesNotWriteAndLeavesSourceIntact(t *testing.T) {
	t.Parallel()
	svc := importFixture(t)
	srcPath := filepath.Join(svc.paths.RepoRoot, "notes.md")
	before, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}

	if _, err := svc.Import(ImportInput{SourcePath: "notes.md", Target: "planning"}); err != nil {
		t.Fatalf("import: %v", err)
	}
	if _, err := os.Stat(filepath.Join(svc.paths.PlanningDir, "imports")); !os.IsNotExist(err) {
		t.Fatalf("preview must not create the imports dir, stat err=%v", err)
	}
	after, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("re-read source: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("preview must not modify the source file")
	}
}

func TestImportIsDeterministic(t *testing.T) {
	t.Parallel()
	svc := importFixture(t)

	first, err := svc.Import(ImportInput{SourcePath: "notes.md", Target: "planning"})
	if err != nil {
		t.Fatalf("import first: %v", err)
	}
	second, err := svc.Import(ImportInput{SourcePath: "notes.md", Target: "planning"})
	if err != nil {
		t.Fatalf("import second: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("structural import must be deterministic for identical input")
	}
}

func TestImportRejectsUnknownTarget(t *testing.T) {
	t.Parallel()
	svc := importFixture(t)
	if _, err := svc.Import(ImportInput{SourcePath: "notes.md", Target: "everything"}); err == nil {
		t.Fatal("expected error for unknown target")
	}
}

func TestImportRejectsMissingSource(t *testing.T) {
	t.Parallel()
	svc := importFixture(t)
	if _, err := svc.Import(ImportInput{SourcePath: "does-not-exist.md", Target: "tasks"}); err == nil {
		t.Fatal("expected error for missing source file")
	}
}

func TestImportRejectsStructurelessSource(t *testing.T) {
	t.Parallel()
	svc := importFixture(t)
	writeSource(t, svc.paths.RepoRoot, "flat.md", "just a paragraph with no headings or bullets\n")
	if _, err := svc.Import(ImportInput{SourcePath: "flat.md", Target: "tasks"}); err == nil {
		t.Fatal("expected error when the source has no importable structure")
	}
}

func draftTitles(tasks []TaskDraft) []string {
	titles := make([]string, 0, len(tasks))
	for _, task := range tasks {
		titles = append(titles, task.Title)
	}
	return titles
}

func taskDraftByTitle(t *testing.T, tasks []TaskDraft, title string) TaskDraft {
	t.Helper()
	for _, task := range tasks {
		if task.Title == title {
			return task
		}
	}
	t.Fatalf("task draft %q not found", title)
	return TaskDraft{}
}

func assertUniqueKeys(t *testing.T, tasks []TaskDraft) {
	t.Helper()
	seen := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		if task.Key == "" {
			t.Fatalf("task draft %q has no key", task.Title)
		}
		if _, dup := seen[task.Key]; dup {
			t.Fatalf("duplicate task key %q", task.Key)
		}
		seen[task.Key] = struct{}{}
	}
}
