package taskrail

import (
	"strings"
	"testing"
	"time"
)

// blockerEntries returns the STATE.md blockers slice after loading, for concise
// assertions about which tasks' reasons the committed state retains.
func blockerEntries(t *testing.T, svc *Service) []string {
	t.Helper()
	state, err := svc.loadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	return state.Frontmatter.Blockers
}

func hasBlocker(blockers []string, want string) bool {
	for _, b := range blockers {
		if b == want {
			return true
		}
	}
	return false
}

// TestBlockRetainsEveryBlockedTaskReason locks in that blocking a second task
// while a first is still blocked keeps BOTH reasons in STATE.md, one entry per
// task — not just the most recently blocked one.
func TestBlockRetainsEveryBlockedTaskReason(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "First", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-003", "Second", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.Block("T-002", "waiting on vendor A"); err != nil {
		t.Fatalf("block T-002: %v", err)
	}
	if _, err := svc.Block("T-003", "waiting on vendor B"); err != nil {
		t.Fatalf("block T-003: %v", err)
	}

	blockers := blockerEntries(t, svc)
	if len(blockers) != 2 {
		t.Fatalf("blockers = %v, want 2 entries", blockers)
	}
	if !hasBlocker(blockers, "T-002: waiting on vendor A") {
		t.Errorf("first blocked task's reason lost: %v", blockers)
	}
	if !hasBlocker(blockers, "T-003: waiting on vendor B") {
		t.Errorf("second blocked task's reason missing: %v", blockers)
	}
}

// TestBlockPreservesActiveTaskSummary locks in that blocking a todo while a
// different task is in_progress leaves the active task's status_summary and
// next_action pointers intact (never clobbered to "blocked"), while still
// recording the newly blocked task's reason. Blocking while idle keeps today's
// behavior: status_summary flips to "blocked".
func TestBlockPreservesActiveTaskSummary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		startActive bool
		wantSummary string
		wantNextHas string
	}{
		{
			name:        "block while another task in_progress",
			startActive: true,
			wantSummary: "in_progress",
			wantNextHas: "T-004",
		},
		{
			name:        "block while idle",
			startActive: false,
			wantSummary: "blocked",
			wantNextHas: "T-003",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := seedFixtureRepo(t)
			writeTask(t, repo, "T-003", "Blocked", "todo", "high", "specs/v0.1.0.md#summary", nil)
			writeTask(t, repo, "T-004", "Active", "todo", "high", "specs/v0.1.0.md#summary", nil)
			svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

			if tt.startActive {
				if _, err := svc.Start("T-004"); err != nil {
					t.Fatalf("start T-004: %v", err)
				}
			}
			if _, err := svc.Block("T-003", "waiting on vendor"); err != nil {
				t.Fatalf("block T-003: %v", err)
			}

			state, err := svc.loadState()
			if err != nil {
				t.Fatalf("load state: %v", err)
			}
			if state.Frontmatter.StatusSummary != tt.wantSummary {
				t.Errorf("status_summary = %q, want %q", state.Frontmatter.StatusSummary, tt.wantSummary)
			}
			if !strings.Contains(state.Frontmatter.NextAction, tt.wantNextHas) {
				t.Errorf("next_action = %q, want reference to %q", state.Frontmatter.NextAction, tt.wantNextHas)
			}
			if tt.startActive && state.Frontmatter.CurrentTask != "T-004" {
				t.Errorf("current_task = %q, want T-004 still active", state.Frontmatter.CurrentTask)
			}
			if !hasBlocker(blockerEntries(t, svc), "T-003: waiting on vendor") {
				t.Errorf("blocked task's reason not recorded: %v", state.Frontmatter.Blockers)
			}
		})
	}
}

// TestUnrelatedTransitionsPreserveBlockers locks in that starting and completing
// an unrelated task does not wipe a still-blocked task's recorded reason.
func TestUnrelatedTransitionsPreserveBlockers(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-003", "Blocked", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-004", "Unrelated", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.Block("T-003", "waiting on decision"); err != nil {
		t.Fatalf("block T-003: %v", err)
	}
	if _, err := svc.Start("T-004"); err != nil {
		t.Fatalf("start T-004: %v", err)
	}
	if got := blockerEntries(t, svc); !hasBlocker(got, "T-003: waiting on decision") {
		t.Fatalf("starting T-004 wiped T-003's blocker: %v", got)
	}
	if _, err := svc.Complete("T-004", "done"); err != nil {
		t.Fatalf("complete T-004: %v", err)
	}
	if got := blockerEntries(t, svc); !hasBlocker(got, "T-003: waiting on decision") {
		t.Fatalf("completing T-004 wiped T-003's blocker: %v", got)
	}
}

// TestUpsertBlockerReplacesWithoutDuplicating locks in the property upsertBlocker
// exists to guarantee: setting a task's reason replaces its single entry and
// never appends a duplicate, while leaving other tasks' entries untouched.
func TestUpsertBlockerReplacesWithoutDuplicating(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		start  []string
		id     string
		reason string
		want   []string
	}{
		{"append to empty", nil, "T-001", "a", []string{"T-001: a"}},
		{"append alongside others", []string{"T-001: a"}, "T-002", "b", []string{"T-001: a", "T-002: b"}},
		{"replace existing among others", []string{"T-001: a", "T-002: b"}, "T-001", "a2", []string{"T-002: b", "T-001: a2"}},
		{"replace only entry", []string{"T-001: a"}, "T-001", "a2", []string{"T-001: a2"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := upsertBlocker(tc.start, tc.id, tc.reason)
			if len(got) != len(tc.want) {
				t.Fatalf("upsertBlocker(%v, %q, %q) = %v, want %v", tc.start, tc.id, tc.reason, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("entry %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestRemoveBlockerDropsOnlyTargetAndToleratesMalformed covers removeBlocker's
// order-preserving drop and blockerID's parse of an entry without a colon.
func TestRemoveBlockerDropsOnlyTargetAndToleratesMalformed(t *testing.T) {
	t.Parallel()
	got := removeBlocker([]string{"T-001: a", "malformed-no-colon", "T-002: b"}, "T-001")
	want := []string{"malformed-no-colon", "T-002: b"}
	if len(got) != len(want) {
		t.Fatalf("removeBlocker = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %q, want %q", i, got[i], want[i])
		}
	}
	if blockerID("malformed-no-colon") != "malformed-no-colon" {
		t.Errorf("blockerID of colon-less entry = %q, want whole string", blockerID("malformed-no-colon"))
	}
}

// TestBlockedTasksMarksMissingReason covers the defensive fallback: a blocked
// task with no entry in the blockers list (e.g. legacy or hand-repaired state)
// reports the explicit "not recorded" marker rather than a silent empty reason.
func TestBlockedTasksMarksMissingReason(t *testing.T) {
	t.Parallel()
	tasks := []*Task{
		{Frontmatter: TaskFrontmatter{ID: "T-001", Title: "Recorded", Status: "blocked"}},
		{Frontmatter: TaskFrontmatter{ID: "T-002", Title: "Missing", Status: "blocked"}},
	}
	got := blockedTasks(tasks, []string{"T-001: has a reason"})
	if len(got) != 2 {
		t.Fatalf("blocked = %+v, want 2", got)
	}
	if got[0].Reason != "has a reason" {
		t.Errorf("T-001 reason = %q, want 'has a reason'", got[0].Reason)
	}
	if got[1].Reason != UnrecordedBlockerReason {
		t.Errorf("T-002 reason = %q, want marker %q", got[1].Reason, UnrecordedBlockerReason)
	}
}
