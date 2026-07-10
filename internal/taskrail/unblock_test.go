package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestUnblockReturnsBlockedTaskToTodo locks in the core inverse of block: a
// blocked task returns to todo (status + updated_at rewritten) and becomes
// eligible for next selection again.
func TestUnblockReturnsBlockedTaskToTodo(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Work item", "todo", "high", "specs/v0.1.0.md#summary", nil)
	now := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	svc := newTestService(t, repo, now)

	if _, err := svc.Block("T-002", "waiting on vendor"); err != nil {
		t.Fatalf("block: %v", err)
	}
	result, err := svc.Unblock("T-002", "")
	if err != nil {
		t.Fatalf("unblock: %v", err)
	}
	if result.Status != "todo" {
		t.Fatalf("result status = %q, want todo", result.Status)
	}
	if result.UpdatedAt != timestamp(now) {
		t.Fatalf("result updated_at = %q, want %q", result.UpdatedAt, timestamp(now))
	}

	_, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	task, _ := taskByID(tasks, "T-002")
	if task.Frontmatter.Status != "todo" {
		t.Fatalf("task status = %q, want todo", task.Frontmatter.Status)
	}
	if task.Frontmatter.UpdatedAt != timestamp(now) {
		t.Fatalf("task updated_at = %q, want %q", task.Frontmatter.UpdatedAt, timestamp(now))
	}

	next, err := svc.Next()
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if next.TaskID != "T-002" {
		t.Fatalf("next selected %q, want T-002 back in selection", next.TaskID)
	}
}

// TestUnblockRejectsNonBlockedWithoutWriting locks in the guard mirroring start's
// todo-only guard: any non-blocked status errors and writes nothing — neither the
// task file nor STATE.md changes.
func TestUnblockRejectsNonBlockedWithoutWriting(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"todo", "in_progress", "completed"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			repo := seedFixtureRepo(t)
			writeTask(t, repo, "T-002", "Work item", status, "high", "specs/v0.1.0.md#summary", nil)
			svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

			statePath := filepath.Join(repo, "planning", "STATE.md")
			taskPath := filepath.Join(repo, "planning", "tasks", "T-002.md")
			stateBefore := readFileBytes(t, statePath)
			taskBefore := readFileBytes(t, taskPath)

			if _, err := svc.Unblock("T-002", "recovered"); err == nil {
				t.Fatalf("unblock of %s status: expected error, got nil", status)
			}
			if got := readFileBytes(t, statePath); got != stateBefore {
				t.Errorf("STATE.md changed on rejected unblock of %s", status)
			}
			if got := readFileBytes(t, taskPath); got != taskBefore {
				t.Errorf("task file changed on rejected unblock of %s", status)
			}
		})
	}
}

// TestUnblockDropsOwnBlockerAndRetainsOthers locks in that unblocking removes only
// the target's blockers entry while another still-blocked task keeps its reason.
func TestUnblockDropsOwnBlockerAndRetainsOthers(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "First", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-003", "Second", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.Block("T-002", "waiting on A"); err != nil {
		t.Fatalf("block T-002: %v", err)
	}
	if _, err := svc.Block("T-003", "waiting on B"); err != nil {
		t.Fatalf("block T-003: %v", err)
	}
	if _, err := svc.Unblock("T-002", ""); err != nil {
		t.Fatalf("unblock T-002: %v", err)
	}

	blockers := blockerEntries(t, svc)
	if len(blockers) != 1 {
		t.Fatalf("blockers = %v, want exactly 1 entry", blockers)
	}
	if !hasBlocker(blockers, "T-003: waiting on B") {
		t.Errorf("still-blocked task's reason lost: %v", blockers)
	}
	if hasBlocker(blockers, "T-002: waiting on A") {
		t.Errorf("unblocked task's entry retained: %v", blockers)
	}
}

// TestUnblockReasonAppendsNoteAndNeverReblocks covers the optional --reason: a
// non-empty reason appends a timestamped Implementation Notes entry and is never
// re-added to the STATE.md blockers list; an empty reason appends no note.
func TestUnblockReasonAppendsNoteAndNeverReblocks(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)

	t.Run("non-empty reason appends note", func(t *testing.T) {
		t.Parallel()
		repo := seedFixtureRepo(t)
		writeTask(t, repo, "T-002", "Work item", "todo", "high", "specs/v0.1.0.md#summary", nil)
		svc := newTestService(t, repo, now)
		if _, err := svc.Block("T-002", "waiting on vendor"); err != nil {
			t.Fatalf("block: %v", err)
		}
		if _, err := svc.Unblock("T-002", "vendor responded"); err != nil {
			t.Fatalf("unblock: %v", err)
		}
		_, tasks, err := svc.loadStateAndTasks()
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		task, _ := taskByID(tasks, "T-002")
		wantNote := "- " + timestamp(now) + ": vendor responded"
		if !strings.Contains(task.Body, wantNote) {
			t.Fatalf("task body missing note %q:\n%s", wantNote, task.Body)
		}
		if hasBlocker(blockerEntries(t, svc), "T-002: vendor responded") {
			t.Errorf("reason re-added to blockers list")
		}
	})

	t.Run("empty reason appends no note", func(t *testing.T) {
		t.Parallel()
		repo := seedFixtureRepo(t)
		writeTask(t, repo, "T-002", "Work item", "todo", "high", "specs/v0.1.0.md#summary", nil)
		svc := newTestService(t, repo, now)
		if _, err := svc.Block("T-002", "waiting on vendor"); err != nil {
			t.Fatalf("block: %v", err)
		}
		_, tasksBefore, err := svc.loadStateAndTasks()
		if err != nil {
			t.Fatalf("load before: %v", err)
		}
		before, _ := taskByID(tasksBefore, "T-002")
		bodyBefore := before.Body

		if _, err := svc.Unblock("T-002", "   "); err != nil {
			t.Fatalf("unblock: %v", err)
		}
		_, tasksAfter, err := svc.loadStateAndTasks()
		if err != nil {
			t.Fatalf("load after: %v", err)
		}
		after, _ := taskByID(tasksAfter, "T-002")
		if after.Body != bodyBefore {
			t.Fatalf("empty reason changed task body:\nbefore:\n%s\nafter:\n%s", bodyBefore, after.Body)
		}
	})
}

// TestUnblockRerunsValidationAndReportsResult covers acceptance criterion 5 /
// spec v0.3.0.md#task-unblocking: the transition re-runs validation and returns
// its result, paralleling spec activate.
func TestUnblockRerunsValidationAndReportsResult(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Work item", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Block("T-002", "waiting"); err != nil {
		t.Fatalf("block: %v", err)
	}
	result, err := svc.Unblock("T-002", "")
	if err != nil {
		t.Fatalf("unblock: %v", err)
	}
	if !result.Validation.Valid {
		t.Fatalf("expected valid state after unblock, got violations: %v", result.Validation.Violations)
	}
}

// TestUnblockKeepsBlockedSummaryWhenOthersRemain locks in that unblocking one of
// several blocked tasks (with no active task) does not falsely mark the state
// idle while another blocker is still listed: status_summary stays "blocked" and
// next_action points at a still-blocked task, not the just-unblocked one.
func TestUnblockKeepsBlockedSummaryWhenOthersRemain(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "First", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-003", "Second", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Block("T-002", "waiting A"); err != nil {
		t.Fatalf("block T-002: %v", err)
	}
	if _, err := svc.Block("T-003", "waiting B"); err != nil {
		t.Fatalf("block T-003: %v", err)
	}

	// Unblock the most-recently blocked task so a naive "leave next_action alone"
	// would leave next_action pointing at the just-unblocked T-003.
	if _, err := svc.Unblock("T-003", ""); err != nil {
		t.Fatalf("unblock T-003: %v", err)
	}

	state, err := svc.loadState()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if state.Frontmatter.StatusSummary != "blocked" {
		t.Errorf("status_summary = %q, want blocked (T-002 still blocked)", state.Frontmatter.StatusSummary)
	}
	if !strings.Contains(state.Frontmatter.NextAction, "T-002") {
		t.Errorf("next_action = %q, want reference to still-blocked T-002", state.Frontmatter.NextAction)
	}
	if strings.Contains(state.Frontmatter.NextAction, "T-003") {
		t.Errorf("next_action = %q still references just-unblocked T-003", state.Frontmatter.NextAction)
	}
}

// TestUnblockGoesIdleWhenNoBlockersRemain locks in the neutral reset once the
// last blocker is cleared and no task is active.
func TestUnblockGoesIdleWhenNoBlockersRemain(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Only", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Block("T-002", "waiting"); err != nil {
		t.Fatalf("block: %v", err)
	}
	if _, err := svc.Unblock("T-002", ""); err != nil {
		t.Fatalf("unblock: %v", err)
	}
	state, err := svc.loadState()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if state.Frontmatter.StatusSummary != "idle" {
		t.Errorf("status_summary = %q, want idle", state.Frontmatter.StatusSummary)
	}
}

// TestUnblockLeavesActiveTaskPointersUntouched locks in that unblocking a task
// while a different task is in progress does not clobber the active task's
// status_summary/next_action — those pointers belong to the active task.
func TestUnblockLeavesActiveTaskPointersUntouched(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Active", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-003", "Blocked", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Start("T-002"); err != nil {
		t.Fatalf("start T-002: %v", err)
	}
	if _, err := svc.Block("T-003", "waiting"); err != nil {
		t.Fatalf("block T-003: %v", err)
	}
	before, err := svc.loadState()
	if err != nil {
		t.Fatalf("load before: %v", err)
	}
	wantSummary := before.Frontmatter.StatusSummary
	wantNext := before.Frontmatter.NextAction

	if _, err := svc.Unblock("T-003", ""); err != nil {
		t.Fatalf("unblock T-003: %v", err)
	}
	after, err := svc.loadState()
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if after.Frontmatter.CurrentTask != "T-002" {
		t.Fatalf("active task pointer changed: %q", after.Frontmatter.CurrentTask)
	}
	if after.Frontmatter.StatusSummary != wantSummary {
		t.Errorf("status_summary changed: %q -> %q", wantSummary, after.Frontmatter.StatusSummary)
	}
	if after.Frontmatter.NextAction != wantNext {
		t.Errorf("next_action changed: %q -> %q", wantNext, after.Frontmatter.NextAction)
	}
}

func readFileBytes(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
