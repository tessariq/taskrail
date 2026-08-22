package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func releaseFixture(t *testing.T) (*Service, string) {
	t.Helper()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002-active", "Active work", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Start("T-002-active"); err != nil {
		t.Fatalf("start fixture task: %v", err)
	}
	return svc, repo
}

func TestReleaseTaskDryRunAndApplyHaveTheSameCandidate(t *testing.T) {
	t.Parallel()
	svc, repo := releaseFixture(t)
	taskPath := filepath.Join(repo, "planning", "tasks", "T-002-active.md")
	statePath := filepath.Join(repo, "planning", "STATE.md")
	taskBefore := readFileBytes(t, taskPath)
	stateBefore := readFileBytes(t, statePath)

	preview, err := svc.ReleaseTask(ReleaseTaskInput{
		TaskID: "T-002-active", Reason: "resume after reviewing the approach", DryRun: true,
	})
	if err != nil {
		t.Fatalf("release dry run: %v", err)
	}
	if preview.Applied || preview.PriorStatus != "in_progress" || preview.Status != "todo" {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if preview.CurrentTaskBefore == nil || *preview.CurrentTaskBefore != "T-002-active" || preview.CurrentTaskAfter != nil || !preview.CurrentTaskCleared {
		t.Fatalf("unexpected pointer preview: %+v", preview)
	}
	if preview.TaskSHA256Before == preview.TaskSHA256After {
		t.Fatal("release preview must report the appended-note candidate digest")
	}
	if got := readFileBytes(t, taskPath); got != taskBefore {
		t.Fatal("dry run changed task bytes")
	}
	if got := readFileBytes(t, statePath); got != stateBefore {
		t.Fatal("dry run changed state bytes")
	}

	applied, err := svc.ReleaseTask(ReleaseTaskInput{TaskID: "T-002-active", Reason: "resume after reviewing the approach"})
	if err != nil {
		t.Fatalf("release apply: %v", err)
	}
	if !applied.Applied || applied.TaskSHA256Before != preview.TaskSHA256Before || applied.TaskSHA256After != preview.TaskSHA256After {
		t.Fatalf("apply does not match preview: preview=%+v apply=%+v", preview, applied)
	}
	state, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load released repository: %v", err)
	}
	task, _ := taskByID(tasks, "T-002-active")
	if task.Frontmatter.Status != "todo" {
		t.Fatalf("status = %q, want todo", task.Frontmatter.Status)
	}
	if !strings.Contains(task.Body, "- 2026-08-22T12:00:00Z: resume after reviewing the approach") {
		t.Fatalf("release reason was not recorded in Implementation Notes:\n%s", task.Body)
	}
	if state.Frontmatter.CurrentTask != "" || state.Frontmatter.CurrentTaskTitle != "" || state.Frontmatter.StatusSummary != statusSummaryIdle {
		t.Fatalf("state did not return to idle: %+v", state.Frontmatter)
	}
}

func TestReleaseTaskRefusesInvalidPointersAndReasonsWithoutWriting(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*State)
		reason string
		code   string
	}{
		{"missing pointer", func(state *State) { state.Frontmatter.CurrentTask = "" }, "rework", MachineCodeRepositoryInvalid},
		{"contradictory title", func(state *State) { state.Frontmatter.CurrentTaskTitle = "another task" }, "rework", MachineCodeRepositoryInvalid},
		{"untrimmed reason", func(_ *State) {}, " rework", MachineCodeInvalidReason},
		{"control reason", func(_ *State) {}, "rework\nnow", MachineCodeInvalidReason},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo := releaseFixture(t)
			if tc.name == "missing pointer" || tc.name == "contradictory title" {
				state, err := svc.loadState()
				if err != nil {
					t.Fatalf("load state: %v", err)
				}
				tc.mutate(state)
				state.Body = renderStateBody(state.Frontmatter, nil)
				data, err := marshalFrontmatter(state.Frontmatter, state.Body)
				if err != nil {
					t.Fatalf("marshal state: %v", err)
				}
				if err := os.WriteFile(filepath.Join(repo, "planning", "STATE.md"), data, 0o644); err != nil {
					t.Fatalf("write contradictory state: %v", err)
				}
			}
			taskPath := filepath.Join(repo, "planning", "tasks", "T-002-active.md")
			statePath := filepath.Join(repo, "planning", "STATE.md")
			taskBefore, stateBefore := readFileBytes(t, taskPath), readFileBytes(t, statePath)
			_, err := svc.ReleaseTask(ReleaseTaskInput{TaskID: "T-002-active", Reason: tc.reason})
			if err == nil || MachineFailureFor(err).Code != tc.code {
				t.Fatalf("release error = %v (%s), want %s", err, MachineFailureFor(err).Code, tc.code)
			}
			if got := readFileBytes(t, taskPath); got != taskBefore {
				t.Fatal("rejected release changed task bytes")
			}
			if got := readFileBytes(t, statePath); got != stateBefore {
				t.Fatal("rejected release changed state bytes")
			}
		})
	}
}

func TestReleaseTaskRefusesContradictoryActiveLedgerWithoutWriting(t *testing.T) {
	t.Parallel()
	svc, repo := releaseFixture(t)
	writeTask(t, repo, "T-003-second-active", "Second active", "in_progress", "medium", "specs/v0.1.0.md#summary", nil)
	taskPath := filepath.Join(repo, "planning", "tasks", "T-002-active.md")
	statePath := filepath.Join(repo, "planning", "STATE.md")
	taskBefore, stateBefore := readFileBytes(t, taskPath), readFileBytes(t, statePath)
	_, err := svc.ReleaseTask(ReleaseTaskInput{TaskID: "T-002-active", Reason: "rework"})
	if err == nil || MachineFailureFor(err).Code != MachineCodeRepositoryInvalid {
		t.Fatalf("release with two active tasks = %v (%s), want repository_invalid", err, MachineFailureFor(err).Code)
	}
	if readFileBytes(t, taskPath) != taskBefore || readFileBytes(t, statePath) != stateBefore {
		t.Fatal("contradictory release changed repository bytes")
	}
}

func TestReleaseTaskRefusesRecoveryAppearingAfterServiceConstruction(t *testing.T) {
	svc, repo := releaseFixture(t)
	writeRecoveryJournal(t, filepath.Join(repo, ".git", "taskrail"), "status")
	taskPath := filepath.Join(repo, "planning", "tasks", "T-002-active.md")
	statePath := filepath.Join(repo, "planning", "STATE.md")
	taskBefore, stateBefore := readFileBytes(t, taskPath), readFileBytes(t, statePath)
	_, err := svc.ReleaseTask(ReleaseTaskInput{TaskID: "T-002-active", Reason: "rework"})
	if err == nil || MachineFailureFor(err).Code != MachineCodeRecoveryPending {
		t.Fatalf("release with a retained recovery transaction = %v (%s), want recovery_pending", err, MachineFailureFor(err).Code)
	}
	if readFileBytes(t, taskPath) != taskBefore || readFileBytes(t, statePath) != stateBefore {
		t.Fatal("recovery-pending release changed repository bytes")
	}
}

func TestReleaseTaskRefusesTaskPreimageChangedBeforeTransactionSnapshot(t *testing.T) {
	svc, repo := releaseFixture(t)
	taskPath := filepath.Join(repo, "planning", "tasks", "T-002-active.md")
	statePath := filepath.Join(repo, "planning", "STATE.md")
	stateBefore := readFileBytes(t, statePath)
	testHookReleaseCandidatePrepared = func() {
		writeFile(t, taskPath, strings.Replace(readFileBytes(t, taskPath), "updated_at:", "unknown_marker: external\nupdated_at:", 1))
	}
	t.Cleanup(func() { testHookReleaseCandidatePrepared = nil })

	_, err := svc.ReleaseTask(ReleaseTaskInput{TaskID: "T-002-active", Reason: "rework"})
	if err == nil || MachineFailureFor(err).Code != MachineCodeWriteConflict {
		t.Fatalf("release with changed task preimage = %v (%s), want write_conflict", err, MachineFailureFor(err).Code)
	}
	if got := readFileBytes(t, statePath); got != stateBefore {
		t.Fatal("preimage conflict changed STATE.md")
	}
	if got := readFileBytes(t, taskPath); !strings.Contains(got, "unknown_marker: external") {
		t.Fatalf("preimage conflict overwrote external task bytes:\n%s", got)
	}
}

func TestReleaseTaskPreservesMetadataAndRefusesDelegation(t *testing.T) {
	t.Run("preserves loop and verification metadata", func(t *testing.T) {
		svc, repo := releaseFixture(t)
		path := filepath.Join(repo, "planning", "tasks", "T-002-active.md")
		before := readFileBytes(t, path)
		before = strings.Replace(before, "updated_at:", "loop_policy: hold\nloop_reason: operator approval required\nlast_verification_id: 0123456789abcdef0123456789abcdef\nlast_verification_result: fail\nlast_verified_at: \"2026-08-21T12:00:00Z\"\nupdated_at:", 1)
		if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
			t.Fatalf("seed metadata: %v", err)
		}

		if _, err := svc.ReleaseTask(ReleaseTaskInput{TaskID: "T-002-active", Reason: "resume later"}); err != nil {
			t.Fatalf("release: %v", err)
		}
		after := readFileBytes(t, path)
		for _, line := range []string{
			"loop_policy: hold", "loop_reason: operator approval required",
			"last_verification_id: 0123456789abcdef0123456789abcdef",
			"last_verification_result: fail", "last_verified_at: \"2026-08-21T12:00:00Z\"",
		} {
			if !strings.Contains(after, line) {
				t.Errorf("release dropped preserved metadata %q:\n%s", line, after)
			}
		}
	})

	t.Run("delegated child is refused without writes", func(t *testing.T) {
		svc, repo := releaseFixture(t)
		t.Setenv("TASKRAIL_DELEGATION_ID", "0123456789abcdef0123456789abcdef")
		taskPath := filepath.Join(repo, "planning", "tasks", "T-002-active.md")
		statePath := filepath.Join(repo, "planning", "STATE.md")
		taskBefore, stateBefore := readFileBytes(t, taskPath), readFileBytes(t, statePath)
		_, err := svc.ReleaseTask(ReleaseTaskInput{TaskID: "T-002-active", Reason: "rework"})
		if err == nil || MachineFailureFor(err).Code != MachineCodeDelegatedRefused {
			t.Fatalf("delegated release = %v (%s), want delegated_write_refused", err, MachineFailureFor(err).Code)
		}
		if readFileBytes(t, taskPath) != taskBefore || readFileBytes(t, statePath) != stateBefore {
			t.Fatal("delegated release changed repository bytes")
		}
	})
}
