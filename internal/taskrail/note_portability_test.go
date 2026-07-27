package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const gitignoredNotePath = "planning/artifacts/manual-test/T-002/20260101T000000Z/report.md"

// startWorkTask seeds a todo task and starts it so a transition can run against
// an in_progress task.
func startWorkTask(t *testing.T, repo string, svc *Service, id string) {
	t.Helper()
	writeTask(t, repo, id, "Work item", "todo", "high", "specs/v0.1.0.md#summary", nil)
	if _, err := svc.Start(id); err != nil {
		t.Fatalf("start %s: %v", id, err)
	}
}

// assertCleanValidState fails if the working tree carries an invalid state after a
// rejected transition — the transition must not have written a partial mutation.
func assertCleanValidState(t *testing.T, svc *Service) {
	t.Helper()
	validation, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !validation.Valid {
		t.Fatalf("state invalid after a rejected transition: %+v", validation.Violations)
	}
}

// TestCompleteRejectsGitignoredArtifactNote: a complete note embedding a concrete
// gitignored artifact path is rejected before any write, and state stays valid.
func TestCompleteRejectsGitignoredArtifactNote(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	startWorkTask(t, repo, svc, "T-002")

	_, err := svc.Complete("T-002", "see "+gitignoredNotePath)
	if err == nil {
		t.Fatal("expected complete to reject a note embedding a gitignored artifact path")
	}
	if !strings.Contains(err.Error(), gitignoredNotePath) {
		t.Fatalf("error should name the offending path, got %v", err)
	}
	// The task must remain in_progress (no partial transition).
	_, tasks, _ := svc.loadStateAndTasks()
	task, _ := taskByID(tasks, "T-002")
	if task.Frontmatter.Status != "in_progress" {
		t.Fatalf("rejected complete must not change status, got %s", task.Frontmatter.Status)
	}
	assertCleanValidState(t, svc)
}

// TestBlockRejectsGitignoredArtifactNote: block records its reason in the
// validated blockers ledger, so a gitignored path there must be rejected too.
func TestBlockRejectsGitignoredArtifactNote(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	writeTask(t, repo, "T-002", "Work item", "todo", "high", "specs/v0.1.0.md#summary", nil)

	if _, err := svc.Block("T-002", "blocked, evidence at "+gitignoredNotePath); err == nil {
		t.Fatal("expected block to reject a reason embedding a gitignored artifact path")
	}
	// No partial transition: status stays todo and no blocker was recorded.
	state, tasks, _ := svc.loadStateAndTasks()
	task, _ := taskByID(tasks, "T-002")
	if task.Frontmatter.Status != "todo" {
		t.Fatalf("rejected block must not change status, got %s", task.Frontmatter.Status)
	}
	if len(state.Frontmatter.Blockers) != 0 {
		t.Fatalf("rejected block must not record a blocker, got %v", state.Frontmatter.Blockers)
	}
	assertCleanValidState(t, svc)
}

// TestUnblockRejectsGitignoredArtifactNote: unblock appends its reason to the
// committed body, so it must reject a gitignored path.
func TestUnblockRejectsGitignoredArtifactNote(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	writeTask(t, repo, "T-002", "Work item", "todo", "high", "specs/v0.1.0.md#summary", nil)
	if _, err := svc.Block("T-002", "waiting on vendor"); err != nil {
		t.Fatalf("block: %v", err)
	}

	if _, err := svc.Unblock("T-002", "resolved, log at "+gitignoredNotePath); err == nil {
		t.Fatal("expected unblock to reject a reason embedding a gitignored artifact path")
	}
	// No partial transition: the task stays blocked.
	_, tasks, _ := svc.loadStateAndTasks()
	task, _ := taskByID(tasks, "T-002")
	if task.Frontmatter.Status != "blocked" {
		t.Fatalf("rejected unblock must not change status, got %s", task.Frontmatter.Status)
	}
	assertCleanValidState(t, svc)
}

// TestVerifyFollowupRejectsGitignoredDetails: a `verify --create-followup` bakes
// the verification details into the committed follow-up task body, so a gitignored
// path in --details must be rejected before the follow-up (or any artifact) is
// written, leaving state valid.
func TestVerifyFollowupRejectsGitignoredDetails(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Verified item", "completed", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	_, err := svc.Verify(VerifyInput{
		TaskID:         "T-002",
		Result:         "fail",
		Summary:        "needs follow-up",
		Details:        "evidence at " + gitignoredNotePath,
		CreateFollowup: true,
	})
	if err == nil {
		t.Fatal("expected verify --create-followup to reject gitignored details")
	}
	if _, statErr := os.Stat(filepath.Join(repo, "planning", "tasks", "T-003.md")); statErr == nil {
		t.Fatal("no follow-up task file should be written on rejection")
	}
	assertCleanValidState(t, svc)
}

// TestVerifyFollowupRejectsGitignoredSummaryTitle: without an explicit
// --followup-title the follow-up title falls back to the verification summary, so
// a gitignored path in --summary must also be rejected.
func TestVerifyFollowupRejectsGitignoredSummaryTitle(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Verified item", "completed", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	_, err := svc.Verify(VerifyInput{
		TaskID:         "T-002",
		Result:         "fail",
		Summary:        "see " + gitignoredNotePath,
		CreateFollowup: true,
	})
	if err == nil {
		t.Fatal("expected verify --create-followup to reject a gitignored summary-derived title")
	}
	assertCleanValidState(t, svc)
}

// TestCompleteAcceptsPortableNote: the guard rejects only concrete gitignored
// files. A path-free summary and a bare directory-prefix reference (the forms
// validate deliberately allows) still complete cleanly.
func TestCompleteAcceptsPortableNote(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	startWorkTask(t, repo, svc, "T-002")

	// Directory prefix only (no concrete file) is contract prose, not a dangling path.
	if _, err := svc.Complete("T-002", "evidence recorded under planning/artifacts/manual-test/ (gitignored)"); err != nil {
		t.Fatalf("portable note must be accepted, got %v", err)
	}
	_, tasks, _ := svc.loadStateAndTasks()
	task, _ := taskByID(tasks, "T-002")
	if task.Frontmatter.Status != "completed" {
		t.Fatalf("expected completed, got %s", task.Frontmatter.Status)
	}
	assertCleanValidState(t, svc)
}
