package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// repairService seeds a fixture repo with the given tasks, overwrites STATE.md
// with the supplied current_task pointer (body regenerated to stay internally
// consistent with that frontmatter), and returns a service pinned to now.
func repairService(t *testing.T, currentTask, currentTaskTitle string, tasks func(repo string)) (*Service, string) {
	t.Helper()
	repo := seedFixtureRepo(t)
	if tasks != nil {
		tasks(repo)
	}
	svc := newTestService(t, repo, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC))

	state, loaded, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load seeded state: %v", err)
	}
	state.Frontmatter.CurrentTask = currentTask
	state.Frontmatter.CurrentTaskTitle = currentTaskTitle
	state.Body = renderStateBody(state.Frontmatter, loaded)
	if err := svc.saveState(state); err != nil {
		t.Fatalf("save seeded state: %v", err)
	}
	return svc, repo
}

func snapshotTasksDir(t *testing.T, repo string) map[string]string {
	t.Helper()
	dir := filepath.Join(repo, "planning", "tasks")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read tasks dir: %v", err)
	}
	snap := make(map[string]string, len(entries))
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read task %s: %v", e.Name(), err)
		}
		snap[e.Name()] = string(data)
	}
	return snap
}

// A current_task pointing at no in_progress task is a mechanical drift: repair
// clears it, leaves the pointed-at task's status untouched, and validation passes.
func TestRepairClearsOrphanCurrentTask(t *testing.T) {
	svc, _ := repairService(t, "T-001", "Task One", func(repo string) {
		writeTask(t, repo, "T-001", "Task One", "todo", "high", "specs/v0.1.0.md#summary", nil)
	})

	before, err := svc.Validate()
	if err != nil {
		t.Fatalf("pre-validate: %v", err)
	}
	if before.Valid {
		t.Fatal("expected seeded state to be invalid (orphan current_task)")
	}

	result, err := svc.Repair(RepairInput{Apply: true})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if !result.Applied {
		t.Fatal("expected repair to apply a change")
	}
	if result.Validation == nil || !result.Validation.Valid {
		t.Fatalf("expected valid state after repair, got %+v", result.Validation)
	}

	state, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if state.Frontmatter.CurrentTask != "" {
		t.Errorf("current_task = %q, want cleared", state.Frontmatter.CurrentTask)
	}
	if tasks[0].Frontmatter.Status != "todo" {
		t.Errorf("task status advanced to %q; repair must never advance status", tasks[0].Frontmatter.Status)
	}
}

// A current_task disagreeing with the single in_progress task is corrected to
// match the task file (the source of truth), never the reverse.
func TestRepairFixesCurrentTaskMismatch(t *testing.T) {
	svc, _ := repairService(t, "T-001", "Task One", func(repo string) {
		writeTask(t, repo, "T-001", "Task One", "todo", "high", "specs/v0.1.0.md#summary", nil)
		writeTask(t, repo, "T-002", "Task Two", "in_progress", "high", "specs/v0.1.0.md#summary", nil)
	})

	result, err := svc.Repair(RepairInput{Apply: true})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if result.Validation == nil || !result.Validation.Valid {
		t.Fatalf("expected valid state after repair, got %+v", result.Validation)
	}

	state, _, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if state.Frontmatter.CurrentTask != "T-002" {
		t.Errorf("current_task = %q, want T-002", state.Frontmatter.CurrentTask)
	}
	if state.Frontmatter.CurrentTaskTitle != "Task Two" {
		t.Errorf("current_task_title = %q, want Task Two", state.Frontmatter.CurrentTaskTitle)
	}
}

// Stale STATE.md body counts (frontmatter otherwise consistent) are a mechanical
// drift repair regenerates; the fix surfaces in the dry-run body diff first.
func TestRepairRegeneratesStaleCounts(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "Task One", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC))

	state, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	good := renderStateBody(state.Frontmatter, tasks)
	stale := strings.Replace(good, "todo: 1", "todo: 42", 1)
	if stale == good {
		t.Fatal("failed to corrupt seeded counts")
	}
	state.Body = stale
	if err := svc.saveState(state); err != nil {
		t.Fatalf("save stale state: %v", err)
	}

	dry, err := svc.Repair(RepairInput{Apply: false})
	if err != nil {
		t.Fatalf("dry-run repair: %v", err)
	}
	if len(dry.BodyDiff) == 0 {
		t.Fatal("expected dry-run to report a body diff for stale counts")
	}

	if _, err := svc.Repair(RepairInput{Apply: true}); err != nil {
		t.Fatalf("apply repair: %v", err)
	}
	reloaded, err := svc.loadState()
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if strings.Contains(reloaded.Body, "todo: 42") {
		t.Error("stale count survived repair")
	}
	if !strings.Contains(reloaded.Body, "todo: 1") {
		t.Error("repair did not restore the correct todo count")
	}
}

// The Explicitly-Excluded guard: repair only rewrites STATE.md. It never edits a
// task file (no status advance) and never creates one (no fabricated work).
func TestRepairNeverTouchesTaskFiles(t *testing.T) {
	svc, repo := repairService(t, "T-001", "Task One", func(repo string) {
		writeTask(t, repo, "T-001", "Task One", "todo", "high", "specs/v0.1.0.md#summary", nil)
	})

	before := snapshotTasksDir(t, repo)
	if _, err := svc.Repair(RepairInput{Apply: true}); err != nil {
		t.Fatalf("repair: %v", err)
	}
	after := snapshotTasksDir(t, repo)

	if len(before) != len(after) {
		t.Fatalf("task file count changed: %d -> %d (repair must not fabricate work)", len(before), len(after))
	}
	for name, content := range before {
		if after[name] != content {
			t.Errorf("task file %s changed; repair must never mutate task files", name)
		}
	}
}

// A consistent, valid state is a no-op: repair proposes nothing and does not
// rewrite STATE.md (so updated_at is untouched).
func TestRepairNoOpWhenConsistent(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "Task One", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC))
	state, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	state.Body = renderStateBody(state.Frontmatter, tasks)
	if err := svc.saveState(state); err != nil {
		t.Fatalf("save: %v", err)
	}
	original, err := os.ReadFile(svc.paths.StateFile)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}

	result, err := svc.Repair(RepairInput{Apply: true})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if result.Applied {
		t.Error("expected no-op repair to report Applied=false")
	}
	if len(result.Changes) != 0 || len(result.BodyDiff) != 0 {
		t.Errorf("expected no proposed changes, got changes=%v body=%v", result.Changes, result.BodyDiff)
	}

	after, err := os.ReadFile(svc.paths.StateFile)
	if err != nil {
		t.Fatalf("reread state: %v", err)
	}
	if string(after) != string(original) {
		t.Error("no-op repair rewrote STATE.md")
	}
}

// A correct current_task pointer with a stale current_task_title is a mechanical
// drift repair heals on its own, keeping STATE.md's rendered focus coherent.
func TestRepairFixesStaleTitleOnly(t *testing.T) {
	svc, _ := repairService(t, "T-002", "Old Title", func(repo string) {
		writeTask(t, repo, "T-002", "Task Two", "in_progress", "high", "specs/v0.1.0.md#summary", nil)
	})
	// Make status_summary already consistent so only the title drifts; QC-1
	// (status_summary reconciliation) is exercised separately.
	state, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	state.Frontmatter.StatusSummary = "in_progress"
	state.Body = renderStateBody(state.Frontmatter, tasks)
	if err := svc.saveState(state); err != nil {
		t.Fatalf("save: %v", err)
	}

	result, err := svc.Repair(RepairInput{Apply: true})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if len(result.Changes) != 1 || result.Changes[0].Field != "current_task_title" || result.Changes[0].To != "Task Two" {
		t.Fatalf("expected a single current_task_title correction to Task Two, got %+v", result.Changes)
	}

	state, _, err = svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if state.Frontmatter.CurrentTask != "T-002" {
		t.Errorf("current_task = %q, want T-002 (must be left correct)", state.Frontmatter.CurrentTask)
	}
	if state.Frontmatter.CurrentTaskTitle != "Task Two" {
		t.Errorf("current_task_title = %q, want Task Two", state.Frontmatter.CurrentTaskTitle)
	}
}

// With no task files at all, a lingering current_task pointer is still stale and
// repair clears it.
func TestRepairClearsCurrentTaskWithNoTasks(t *testing.T) {
	svc, _ := repairService(t, "T-001", "Ghost", nil)

	result, err := svc.Repair(RepairInput{Apply: true})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if result.Validation == nil || !result.Validation.Valid {
		t.Fatalf("expected valid state after repair, got %+v", result.Validation)
	}
	state, err := svc.loadState()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if state.Frontmatter.CurrentTask != "" {
		t.Errorf("current_task = %q, want cleared", state.Frontmatter.CurrentTask)
	}
}

// Multiple in_progress tasks are not mechanically resolvable (choosing one would
// regress the other's status), so repair leaves current_task alone, writes
// nothing, and validation still flags the condition.
func TestRepairLeavesMultipleInProgressToValidation(t *testing.T) {
	svc, _ := repairService(t, "T-001", "Task One", func(repo string) {
		writeTask(t, repo, "T-001", "Task One", "in_progress", "high", "specs/v0.1.0.md#summary", nil)
		writeTask(t, repo, "T-002", "Task Two", "in_progress", "high", "specs/v0.1.0.md#summary", nil)
	})
	original, err := os.ReadFile(svc.paths.StateFile)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}

	result, err := svc.Repair(RepairInput{Apply: true})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if result.Applied {
		t.Error("repair must not apply when it cannot mechanically resolve the drift")
	}
	for _, ch := range result.Changes {
		if ch.Field == "current_task" {
			t.Errorf("repair changed current_task under multiple in_progress: %+v", ch)
		}
	}
	if after, _ := os.ReadFile(svc.paths.StateFile); string(after) != string(original) {
		t.Error("repair rewrote STATE.md despite an unresolvable drift")
	}
	if result.Validation == nil {
		t.Fatal("expected validation result")
	}
	found := false
	for _, v := range result.Validation.Violations {
		if strings.Contains(v, "multiple in_progress") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected validation to still flag multiple in_progress, got %v", result.Validation.Violations)
	}
}

// QC-1: exactly one in_progress task with a stale status_summary is a mechanical
// drift repair heals in the single deterministic direction — set it to
// "in_progress" — touching only STATE.md frontmatter, never the task file.
func TestRepairFixesStaleStatusSummaryInProgress(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Task Two", "in_progress", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC))

	state, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// Pointer correct so only status_summary is stale, isolating the QC-1 case.
	state.Frontmatter.CurrentTask = "T-002"
	state.Frontmatter.CurrentTaskTitle = "Task Two"
	state.Frontmatter.StatusSummary = "idle"
	state.Body = renderStateBody(state.Frontmatter, tasks)
	if err := svc.saveState(state); err != nil {
		t.Fatalf("save: %v", err)
	}
	before := snapshotTasksDir(t, repo)

	result, err := svc.Repair(RepairInput{Apply: true})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if len(result.Changes) != 1 || result.Changes[0].Field != "status_summary" ||
		result.Changes[0].From != "idle" || result.Changes[0].To != "in_progress" {
		t.Fatalf("expected a single status_summary idle->in_progress correction, got %+v", result.Changes)
	}
	if result.Validation == nil || !result.Validation.Valid {
		t.Fatalf("expected valid state after repair, got %+v", result.Validation)
	}

	reloaded, err := svc.loadState()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Frontmatter.StatusSummary != "in_progress" {
		t.Errorf("status_summary = %q, want in_progress", reloaded.Frontmatter.StatusSummary)
	}
	if !strings.Contains(reloaded.Body, "- in_progress") {
		t.Error("body Status line did not track the corrected status_summary")
	}
	for name, content := range before {
		if snapshotTasksDir(t, repo)[name] != content {
			t.Errorf("task file %s changed; repair must never touch task files", name)
		}
	}
}

// Excluded direction (audit case D): with no task in_progress, status_summary's
// correct value (idle vs blocked) reflects the last transition, not task state.
// Two safe values exist, so repair must leave a stale "in_progress" alone.
func TestRepairLeavesStatusSummaryWhenNoInProgress(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "Task One", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC))

	state, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	state.Frontmatter.CurrentTask = ""
	state.Frontmatter.CurrentTaskTitle = ""
	state.Frontmatter.StatusSummary = "in_progress" // stale, but idle/blocked is ambiguous
	state.Body = renderStateBody(state.Frontmatter, tasks)
	if err := svc.saveState(state); err != nil {
		t.Fatalf("save: %v", err)
	}

	result, err := svc.Repair(RepairInput{Apply: true})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	for _, ch := range result.Changes {
		if ch.Field == "status_summary" {
			t.Errorf("repair touched status_summary in the excluded idle/blocked direction: %+v", ch)
		}
	}
	reloaded, err := svc.loadState()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Frontmatter.StatusSummary != "in_progress" {
		t.Errorf("status_summary = %q, want left as authored (in_progress)", reloaded.Frontmatter.StatusSummary)
	}
}

// Boundary: more than one in_progress task is unresolvable, so QC-1 refuses to
// touch status_summary just as it refuses to touch the current_task pointer.
func TestRepairLeavesStatusSummaryUnderMultipleInProgress(t *testing.T) {
	svc, _ := repairService(t, "T-001", "Task One", func(repo string) {
		writeTask(t, repo, "T-001", "Task One", "in_progress", "high", "specs/v0.1.0.md#summary", nil)
		writeTask(t, repo, "T-002", "Task Two", "in_progress", "high", "specs/v0.1.0.md#summary", nil)
	})

	result, err := svc.Repair(RepairInput{Apply: true})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	for _, ch := range result.Changes {
		if ch.Field == "status_summary" {
			t.Errorf("repair changed status_summary under multiple in_progress: %+v", ch)
		}
	}
}

// A dry run reports the proposed repair but writes nothing to disk.
func TestRepairDryRunWritesNothing(t *testing.T) {
	svc, _ := repairService(t, "T-001", "Task One", func(repo string) {
		writeTask(t, repo, "T-001", "Task One", "todo", "high", "specs/v0.1.0.md#summary", nil)
	})
	original, err := os.ReadFile(svc.paths.StateFile)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}

	result, err := svc.Repair(RepairInput{Apply: false})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if result.Applied {
		t.Error("dry run reported Applied=true")
	}
	if len(result.Changes) == 0 {
		t.Error("expected dry run to propose a change")
	}

	after, err := os.ReadFile(svc.paths.StateFile)
	if err != nil {
		t.Fatalf("reread state: %v", err)
	}
	if string(after) != string(original) {
		t.Error("dry run wrote to STATE.md")
	}
}
