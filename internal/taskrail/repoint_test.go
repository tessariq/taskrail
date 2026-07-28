package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// repointFixture seeds a repo whose active spec (v0.1.0) has two anchors and
// whose older spec (v0.2.0 as a stand-in second document) carries a third, so
// both the --area and the cross-spec --spec-ref paths have real targets.
func repointFixture(t *testing.T) string {
	t.Helper()
	repo := seedFixtureRepo(t)
	writeFile(t, filepath.Join(repo, "specs", "v0.1.0.md"), "# Taskrail v0.1.0\n\n## Summary\n\nx\n\n## Details\n\ny\n")
	writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"), "# Taskrail v0.2.0\n\n## Legacy Area\n\nz\n")
	writeTask(t, repo, "T-001", "Drifted item", "todo", "high", "specs/v0.2.0.md#legacy-area", nil)
	writeTask(t, repo, "T-002", "Sibling item", "todo", "medium", "specs/v0.1.0.md#summary", nil)
	return repo
}

func repointTestService(t *testing.T, repo string) *Service {
	t.Helper()
	return newTestService(t, repo, time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC))
}

func TestRepointTaskAreaResolvesActiveSpecRef(t *testing.T) {
	t.Parallel()

	repo := repointFixture(t)
	svc := repointTestService(t, repo)
	siblingBefore := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-002.md"))

	result, err := svc.RepointTask(RepointTaskInput{TaskID: "T-001", Area: "details"})
	if err != nil {
		t.Fatalf("repoint with area: %v", err)
	}
	if result.OldSpecRef != "specs/v0.2.0.md#legacy-area" {
		t.Fatalf("expected old spec_ref reported, got %q", result.OldSpecRef)
	}
	if result.NewSpecRef != "specs/v0.1.0.md#details" {
		t.Fatalf("expected resolved active-spec ref, got %q", result.NewSpecRef)
	}
	if !result.Applied {
		t.Fatal("expected applied repoint")
	}
	if result.Validation == nil || !result.Validation.Valid {
		t.Fatalf("expected valid post-repoint state, got %+v", result.Validation)
	}

	_, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	target, ok := taskByID(tasks, "T-001")
	if !ok {
		t.Fatal("expected T-001 after repoint")
	}
	if target.Frontmatter.SpecRef != "specs/v0.1.0.md#details" {
		t.Fatalf("expected persisted spec_ref, got %q", target.Frontmatter.SpecRef)
	}
	// Repoint re-encodes one reference field: every other identity and status
	// field, and the filename, must survive untouched.
	if target.Frontmatter.ID != "T-001" || target.Frontmatter.Title != "Drifted item" {
		t.Fatalf("expected id/title untouched, got %q / %q", target.Frontmatter.ID, target.Frontmatter.Title)
	}
	if target.Frontmatter.Status != "todo" || target.Frontmatter.Priority != "high" {
		t.Fatalf("expected status/priority untouched, got %q / %q", target.Frontmatter.Status, target.Frontmatter.Priority)
	}
	if len(target.Frontmatter.Dependencies) != 0 {
		t.Fatalf("expected dependencies untouched, got %v", target.Frontmatter.Dependencies)
	}
	if filepath.Base(target.Filename) != "T-001.md" {
		t.Fatalf("expected filename untouched, got %q", target.Filename)
	}
	// No other task file is rewritten.
	if got := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-002.md")); got != siblingBefore {
		t.Fatalf("expected sibling task untouched, got:\n%s", got)
	}
	// STATE.md is re-projected (counts rendered from the task set).
	state := readBytes(t, filepath.Join(repo, "planning", "STATE.md"))
	if !strings.Contains(state, "## Task Counts") {
		t.Fatalf("expected re-projected STATE.md body, got:\n%s", state)
	}
}

func TestRepointTaskExplicitSpecRefSupportsCrossSpec(t *testing.T) {
	t.Parallel()

	repo := repointFixture(t)
	svc := repointTestService(t, repo)

	// The active spec is v0.1.0; an explicit ref may still target another spec.
	result, err := svc.RepointTask(RepointTaskInput{TaskID: "T-002", SpecRef: "specs/v0.2.0.md#legacy-area"})
	if err != nil {
		t.Fatalf("repoint with explicit spec_ref: %v", err)
	}
	if result.NewSpecRef != "specs/v0.2.0.md#legacy-area" {
		t.Fatalf("expected explicit spec_ref, got %q", result.NewSpecRef)
	}

	_, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	target, _ := taskByID(tasks, "T-002")
	if target.Frontmatter.SpecRef != "specs/v0.2.0.md#legacy-area" {
		t.Fatalf("expected persisted cross-spec ref, got %q", target.Frontmatter.SpecRef)
	}
}

func TestRepointTaskUnknownAreaFailsWithHintAndWritesNothing(t *testing.T) {
	t.Parallel()

	repo := repointFixture(t)
	svc := repointTestService(t, repo)
	before := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-001.md"))
	stateBefore := readBytes(t, filepath.Join(repo, "planning", "STATE.md"))

	_, err := svc.RepointTask(RepointTaskInput{TaskID: "T-001", Area: "does-not-exist"})
	if err == nil {
		t.Fatal("expected error for unknown active-spec area")
	}
	if !strings.Contains(err.Error(), "spec show v0.1.0 --anchors") {
		t.Fatalf("expected hint at spec show --anchors, got %v", err)
	}
	if got := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-001.md")); got != before {
		t.Fatal("expected task file untouched on rejection")
	}
	if got := readBytes(t, filepath.Join(repo, "planning", "STATE.md")); got != stateBefore {
		t.Fatal("expected STATE.md untouched on rejection")
	}
}

func TestRepointTaskRejectsInvalidExplicitSpecRef(t *testing.T) {
	t.Parallel()

	repo := repointFixture(t)
	svc := repointTestService(t, repo)
	before := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-001.md"))

	_, err := svc.RepointTask(RepointTaskInput{TaskID: "T-001", SpecRef: "specs/v0.1.0.md#nope"})
	if err == nil {
		t.Fatal("expected error for unresolvable spec_ref anchor")
	}
	if !strings.Contains(err.Error(), "spec_ref") {
		t.Fatalf("expected spec_ref error, got %v", err)
	}
	if got := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-001.md")); got != before {
		t.Fatal("expected task file untouched on rejection")
	}
}

func TestRepointTaskRejectsTerminalTasks(t *testing.T) {
	t.Parallel()

	// Completed and cancelled tasks are delivered history, excluded from drift by
	// the coverage orphan rule, so they are never re-pointed.
	for _, status := range []string{"completed", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()

			repo := repointFixture(t)
			writeTask(t, repo, "T-003", "Done item", status, "medium", "specs/v0.2.0.md#legacy-area", nil)
			svc := repointTestService(t, repo)
			before := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-003.md"))

			_, err := svc.RepointTask(RepointTaskInput{TaskID: "T-003", Area: "details"})
			if err == nil {
				t.Fatalf("expected rejection for %s task", status)
			}
			if !strings.Contains(err.Error(), status) {
				t.Fatalf("expected error naming the %s status, got %v", status, err)
			}
			if got := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-003.md")); got != before {
				t.Fatal("expected task file untouched on rejection")
			}
		})
	}
}

func TestRepointTaskRequiresExactlyOneSelector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input RepointTaskInput
		want  string
	}{
		{
			name:  "both",
			input: RepointTaskInput{TaskID: "T-001", Area: "details", SpecRef: "specs/v0.1.0.md#summary"},
			want:  "mutually exclusive",
		},
		{
			name:  "neither",
			input: RepointTaskInput{TaskID: "T-001"},
			want:  "required",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := repointFixture(t)
			svc := repointTestService(t, repo)
			before := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-001.md"))

			_, err := svc.RepointTask(tc.input)
			if err == nil {
				t.Fatal("expected selector error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
			if got := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-001.md")); got != before {
				t.Fatal("expected task file untouched on rejection")
			}
		})
	}
}

func TestRepointTaskDryRunWritesNothing(t *testing.T) {
	t.Parallel()

	repo := repointFixture(t)
	svc := repointTestService(t, repo)
	before := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-001.md"))
	stateBefore := readBytes(t, filepath.Join(repo, "planning", "STATE.md"))

	result, err := svc.RepointTask(RepointTaskInput{TaskID: "T-001", Area: "details", DryRun: true})
	if err != nil {
		t.Fatalf("dry-run repoint: %v", err)
	}
	if result.Applied {
		t.Fatal("expected applied=false on dry run")
	}
	if result.OldSpecRef != "specs/v0.2.0.md#legacy-area" || result.NewSpecRef != "specs/v0.1.0.md#details" {
		t.Fatalf("expected planned change reported, got %q -> %q", result.OldSpecRef, result.NewSpecRef)
	}
	if got := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-001.md")); got != before {
		t.Fatal("expected task file untouched on dry run")
	}
	if got := readBytes(t, filepath.Join(repo, "planning", "STATE.md")); got != stateBefore {
		t.Fatal("expected STATE.md untouched on dry run")
	}
}

func TestRepointTaskDryRunValidationPreviewsPostApplyState(t *testing.T) {
	t.Parallel()

	// The operator's reason to run repoint is often that the current spec_ref is
	// broken — here the spec it names no longer exists, so the repo is invalid
	// right now. A dry run must answer "would this fix it?", so its validation
	// reflects the post-apply state, not the state it is about to replace.
	repo := repointFixture(t)
	if err := os.Remove(filepath.Join(repo, "specs", "v0.2.0.md")); err != nil {
		t.Fatalf("remove spec: %v", err)
	}
	svc := repointTestService(t, repo)

	current, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if current.Valid {
		t.Fatal("expected the pre-repoint repo to be invalid")
	}

	before := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-001.md"))
	result, err := svc.RepointTask(RepointTaskInput{TaskID: "T-001", Area: "details", DryRun: true})
	if err != nil {
		t.Fatalf("dry-run repoint: %v", err)
	}
	if result.Validation == nil || !result.Validation.Valid {
		t.Fatalf("expected dry run to preview a valid post-apply state, got %+v", result.Validation)
	}
	// Previewing must stay side-effect-free.
	if got := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-001.md")); got != before {
		t.Fatal("expected task file untouched on dry run")
	}

	// And the preview is honest: applying it really does yield that result.
	applied, err := svc.RepointTask(RepointTaskInput{TaskID: "T-001", Area: "details"})
	if err != nil {
		t.Fatalf("apply repoint: %v", err)
	}
	if applied.Validation == nil || !applied.Validation.Valid {
		t.Fatalf("expected valid state after apply, got %+v", applied.Validation)
	}
}

func TestRepointTaskDryRunPreviewKeepsUnrelatedViolations(t *testing.T) {
	t.Parallel()

	// The preview replaces only the field being re-pointed: a violation the
	// repoint does not touch must still show up, so a dry run never reads as a
	// clean bill of health for the whole repo.
	repo := repointFixture(t)
	writeTask(t, repo, "T-003", "Broken sibling", "todo", "medium", "specs/v0.9.9.md#gone", nil)
	svc := repointTestService(t, repo)

	result, err := svc.RepointTask(RepointTaskInput{TaskID: "T-001", Area: "details", DryRun: true})
	if err != nil {
		t.Fatalf("dry-run repoint: %v", err)
	}
	if result.Validation == nil || result.Validation.Valid {
		t.Fatalf("expected the unrelated violation to survive the preview, got %+v", result.Validation)
	}
	found := false
	for _, v := range result.Validation.Violations {
		if strings.Contains(v, "T-003") {
			found = true
		}
		if strings.Contains(v, "T-001") {
			t.Fatalf("expected no violation for the re-pointed task, got %q", v)
		}
	}
	if !found {
		t.Fatalf("expected a T-003 violation, got %+v", result.Validation.Violations)
	}
}

func TestRepointTaskRejectsUnknownTask(t *testing.T) {
	t.Parallel()

	repo := repointFixture(t)
	svc := repointTestService(t, repo)

	_, err := svc.RepointTask(RepointTaskInput{TaskID: "T-404", Area: "details"})
	if err == nil {
		t.Fatal("expected error for unknown task")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestRepointTaskRejectsUnchangedSpecRef(t *testing.T) {
	t.Parallel()

	// A no-op repoint would rewrite STATE.md's updated_at and dirty the working
	// tree for no change; reject it the way rename rejects a no-op re-slug.
	repo := repointFixture(t)
	svc := repointTestService(t, repo)
	stateBefore := readBytes(t, filepath.Join(repo, "planning", "STATE.md"))

	_, err := svc.RepointTask(RepointTaskInput{TaskID: "T-002", Area: "summary"})
	if err == nil {
		t.Fatal("expected error when the task already carries the target spec_ref")
	}
	if !strings.Contains(err.Error(), "already") {
		t.Fatalf("expected already-points-at error, got %v", err)
	}
	if got := readBytes(t, filepath.Join(repo, "planning", "STATE.md")); got != stateBefore {
		t.Fatal("expected STATE.md untouched on rejection")
	}
}

func TestRepointTaskRequiresActiveSpecForArea(t *testing.T) {
	t.Parallel()

	repo := repointFixture(t)
	writeFile(t, filepath.Join(repo, "planning", "STATE.md"), `---
schema_version: 1
updated_at: "2026-03-31T00:00:00Z"
active_spec_version: ""
active_spec_path: ""
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
	svc := repointTestService(t, repo)

	_, err := svc.RepointTask(RepointTaskInput{TaskID: "T-001", Area: "details"})
	if err == nil {
		t.Fatal("expected error when no active spec is set")
	}
	if !strings.Contains(err.Error(), "active spec") {
		t.Fatalf("expected missing-active-spec error, got %v", err)
	}
}

func TestRepointTaskBumpsOnlyTargetUpdatedAt(t *testing.T) {
	t.Parallel()

	repo := repointFixture(t)
	svc := repointTestService(t, repo)

	if _, err := svc.RepointTask(RepointTaskInput{TaskID: "T-001", Area: "details"}); err != nil {
		t.Fatalf("repoint: %v", err)
	}

	_, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	target, _ := taskByID(tasks, "T-001")
	if target.Frontmatter.UpdatedAt != "2026-07-28T12:00:00Z" {
		t.Fatalf("expected target updated_at bumped, got %q", target.Frontmatter.UpdatedAt)
	}
	sibling, _ := taskByID(tasks, "T-002")
	if sibling.Frontmatter.UpdatedAt != "2026-03-31T00:00:00Z" {
		t.Fatalf("expected sibling updated_at untouched, got %q", sibling.Frontmatter.UpdatedAt)
	}
	if _, err := os.Stat(filepath.Join(repo, "planning", "tasks", "T-001.md")); err != nil {
		t.Fatalf("expected task file at its original path: %v", err)
	}
}
