package taskrail

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartErrorPaths(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Ready", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-003", "Done", "completed", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-004", "Needs dep", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-002"})

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.Start("T-404"); err == nil {
		t.Fatal("expected error for unknown task")
	}
	if _, err := svc.Start("T-003"); err == nil {
		t.Fatal("expected error starting non-todo task")
	}
	if _, err := svc.Start("T-004"); err == nil {
		t.Fatal("expected error for unresolved dependency")
	}

	if _, err := svc.Start("T-002"); err != nil {
		t.Fatalf("start T-002: %v", err)
	}
	if _, err := svc.Start("T-002"); err == nil {
		t.Fatal("expected error when a task is already active")
	}
}

func TestNextContinuesActiveTask(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Active", "todo", "high", "specs/v0.1.0.md#summary", nil)

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Start("T-002"); err != nil {
		t.Fatalf("start: %v", err)
	}

	result, err := svc.Next()
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if result.TaskID != "T-002" || result.Reason != "active task already in progress" {
		t.Fatalf("expected active-task continuation, got %+v", result)
	}
}

func TestNextNoEligibleCandidates(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Blocked dep", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-003"})
	writeTask(t, repo, "T-003", "Pending dep", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-002"})

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	result, err := svc.Next()
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if result.TaskID != "" || result.Reason != "no eligible task" {
		t.Fatalf("expected no eligible task, got %+v", result)
	}
}

func TestBlockAndCompleteErrorPaths(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Done", "completed", "high", "specs/v0.1.0.md#summary", nil)

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.Block("T-002", "   "); err == nil {
		t.Fatal("expected error for empty block reason")
	}
	if _, err := svc.Block("T-404", "reason"); err == nil {
		t.Fatal("expected error blocking unknown task")
	}
	if _, err := svc.Complete("T-002", "note"); err == nil {
		t.Fatal("expected error completing non-transitionable task")
	}
}

func TestVerifyValidation(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Item", "completed", "high", "specs/v0.1.0.md#summary", nil)

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.Verify(VerifyInput{TaskID: "T-002", Result: "maybe", Summary: "x"}); err == nil {
		t.Fatal("expected error for invalid result")
	}
	if _, err := svc.Verify(VerifyInput{TaskID: "T-002", Result: "pass", Summary: "  "}); err == nil {
		t.Fatal("expected error for empty summary")
	}
	if _, err := svc.Verify(VerifyInput{TaskID: "T-404", Result: "pass", Summary: "ok"}); err == nil {
		t.Fatal("expected error for unknown task")
	}
	if _, err := svc.Verify(VerifyInput{
		TaskID:           "T-002",
		Result:           "fail",
		Summary:          "ok",
		CreateFollowup:   true,
		FollowupPriority: "bogus",
	}); err == nil {
		t.Fatal("expected error for invalid follow-up priority")
	}
}

func TestValidateStateViolations(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, "specs", "README.md"), "# Specs\n")
	// State with wrong schema version, missing spec path, empty status summary.
	writeFile(t, filepath.Join(repo, "planning", "STATE.md"), `---
schema_version: 99
updated_at: "2026-03-31T00:00:00Z"
active_spec_version: v0.1.0
active_spec_path: specs/missing.md
current_task: "T-002"
current_task_title: ""
status_summary: ""
blockers: []
next_action: x
last_verification_result: x
relevant_artifacts: []
continuation_notes: []
---

# STATE
`)
	if err := ensureDir(repo, filepath.Join(repo, "planning", "tasks")); err != nil {
		t.Fatalf("mkdir tasks: %v", err)
	}

	svc := newTestService(t, repo, time.Now().UTC())
	result, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid state")
	}
	joined := strings.Join(result.Violations, "\n")
	for _, want := range []string{"schema_version", "active_spec_path does not exist", "status_summary", "current_task"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected violation mentioning %q in:\n%s", want, joined)
		}
	}
}

func TestValidateTaskViolations(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	// Two in_progress tasks plus a self-dependency and filename/id mismatch.
	writeTask(t, repo, "T-002", "First active", "in_progress", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-003", "Second active", "in_progress", "high", "specs/v0.1.0.md#summary", []string{"T-003"})

	svc := newTestService(t, repo, time.Now().UTC())
	result, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid state")
	}
	joined := strings.Join(result.Violations, "\n")
	for _, want := range []string{"multiple in_progress", "cannot depend on itself"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected violation mentioning %q in:\n%s", want, joined)
		}
	}
}
