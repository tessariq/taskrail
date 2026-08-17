package taskrail

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tessariq/taskrail/internal/repolock"
)

// The T-281 transactional verification matrix: verify, with and without
// --create-followup, must hold the repository mutation lock, snapshot its
// complete consumed and published set, validate the full candidate ledger
// before publication, publish exactly its declared artifact/task/state/follow-
// up files, and roll back handled failures while preserving external edits.

const verifyTimestamp = "20260817T090000Z"

// verifyFixture seeds a repo whose T-002 is verifiable and whose T-009
// sentinel carries a frontmatter field no Taskrail struct models, so any writer
// that rewrites unselected tasks visibly drops it.
func verifyFixture(t *testing.T) (*Service, string) {
	t.Helper()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Verified item", "completed", "high", "specs/v0.1.0.md#summary", nil)
	writeFile(t, filepath.Join(repo, "planning", "tasks", "T-009-sentinel.md"), `---
id: T-009-sentinel
title: Sentinel
status: todo
priority: low
spec_ref: specs/v0.1.0.md#summary
dependencies: []
updated_at: "2026-03-31T00:00:00Z"
sentinel_marker: must-survive-every-verify-write
---

# T-009-sentinel Sentinel
`)
	return newTestService(t, repo, time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)), repo
}

func verifyArtifactPaths(taskID string) []string {
	return []string{
		"planning/artifacts/verify/" + taskID + "/" + verifyTimestamp + "/plan.md",
		"planning/artifacts/verify/" + taskID + "/" + verifyTimestamp + "/report.json",
		"planning/artifacts/verify/" + taskID + "/" + verifyTimestamp + "/report.md",
	}
}

// runVerify drives one verification against the fixture, so every test shares
// the exact input surface a CLI invocation has.
func runVerify(t *testing.T, svc *Service, input VerifyInput) (VerifyResult, error) {
	t.Helper()
	result, err := svc.Verify(input)
	if err == nil && result.TaskID != input.TaskID {
		t.Fatalf("verify reported task %q, want %q", result.TaskID, input.TaskID)
	}
	return result, err
}

func baseVerifyInput() VerifyInput {
	return VerifyInput{TaskID: "T-002", Result: "pass", Summary: "Checks done"}
}

// changedPaths reports which repo-relative files differ between two tree
// snapshots, sorted, so a test can assert the exact write set a writer
// published.
func changedPaths(t *testing.T, before, after map[string]string) []string {
	t.Helper()
	changed := make([]string, 0, len(after))
	for path, content := range after {
		if other, ok := before[path]; !ok || other != content {
			changed = append(changed, filepath.ToSlash(path))
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			changed = append(changed, filepath.ToSlash(path))
		}
	}
	sortStrings(changed)
	return changed
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

// Verify holds the discovered repository mutation lock and records its
// canonical command in the owner metadata for the whole transaction.
func TestVerifyHoldsTheMutationLock(t *testing.T) {
	svc, _ := verifyFixture(t)
	var observed repolock.Status
	installLifecycleHook(t, func() {
		status, err := repolock.Inspect(svc.paths.LockRepository())
		if err != nil {
			t.Errorf("inspect lock during verify: %v", err)
			return
		}
		observed = status
	})
	if _, err := runVerify(t, svc, baseVerifyInput()); err != nil {
		t.Fatalf("verify under observation: %v", err)
	}
	if !observed.Held || observed.Owner == nil {
		t.Fatalf("verify completed without holding the repository mutation lock: %+v", observed)
	}
	if observed.Owner.Command != "verify" {
		t.Errorf("lock owner command = %q, want verify", observed.Owner.Command)
	}
}

// Without follow-ups a verification publishes exactly its report/artifact
// files, the selected task, and the re-projected state — the sentinel task
// survives byte for byte, proving the save-all rewrite is gone, and the
// selected task keeps frontmatter fields no Taskrail struct models.
func TestVerifyPublishesExactWriteSetWithoutFollowup(t *testing.T) {
	for _, result := range []string{"pass", "fail"} {
		t.Run(result, func(t *testing.T) {
			svc, repo := verifyFixture(t)
			selected := filepath.Join(repo, "planning", "tasks", "T-002.md")
			writeFile(t, selected, strings.Replace(readBytes(t, selected),
				"updated_at:", "selected_sentinel: must-survive\nupdated_at:", 1))
			sentinel := filepath.Join(repo, "planning", "tasks", "T-009-sentinel.md")
			sentinelBefore := readBytes(t, sentinel)
			before := snapshotTree(t, repo)

			input := baseVerifyInput()
			input.Result = result
			if _, err := runVerify(t, svc, input); err != nil {
				t.Fatalf("verify: %v", err)
			}

			want := append([]string{
				"planning/STATE.md",
				"planning/tasks/T-002.md",
			}, verifyArtifactPaths("T-002")...)
			sortStrings(want)
			got := changedPaths(t, before, snapshotTree(t, repo))
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Fatalf("verify write set = %v, want %v", got, want)
			}
			if readBytes(t, sentinel) != sentinelBefore {
				t.Fatal("verify rewrote the unselected sentinel task")
			}
			if task := readBytes(t, selected); !strings.Contains(task, "selected_sentinel: must-survive") {
				t.Fatalf("verify dropped selected-task metadata:\n%s", task)
			}
			if task := readBytes(t, selected); !strings.Contains(task, "verification "+result) {
				t.Fatalf("verify did not record its note:\n%s", task)
			}
		})
	}
}

// --create-followup additionally publishes exactly the requested fresh task
// and re-projects state from the complete candidate ledger.
func TestVerifyPublishesExactWriteSetWithFollowup(t *testing.T) {
	svc, repo := verifyFixture(t)
	before := snapshotTree(t, repo)

	input := baseVerifyInput()
	input.Result = "fail"
	input.Summary = "Need one follow-up"
	input.CreateFollowup = true
	input.FollowupTitle = "Handle missing edge case"
	result, err := runVerify(t, svc, input)
	if err != nil {
		t.Fatalf("verify with follow-up: %v", err)
	}
	if result.FollowupTaskID != "T-010-handle-missing-edge-case" {
		t.Fatalf("follow-up id = %q", result.FollowupTaskID)
	}

	want := append([]string{
		"planning/STATE.md",
		"planning/tasks/T-002.md",
		"planning/tasks/T-010-handle-missing-edge-case.md",
	}, verifyArtifactPaths("T-002")...)
	sortStrings(want)
	got := changedPaths(t, before, snapshotTree(t, repo))
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("verify+follow-up write set = %v, want %v", got, want)
	}

	state, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("candidate ledger published %d tasks, want 3", len(tasks))
	}
	if state.Frontmatter.NextAction != "Review follow-up task T-010-handle-missing-edge-case" {
		t.Fatalf("next_action = %q", state.Frontmatter.NextAction)
	}
}

// Successive verifications accumulate follow-ups in the ledger while each run
// still publishes only its own fresh task: the multiple-follow-up projection.
func TestVerifyAccumulatesMultipleFollowups(t *testing.T) {
	svc, repo := verifyFixture(t)

	first := baseVerifyInput()
	first.Result = "fail"
	first.CreateFollowup = true
	first.FollowupTitle = "First gap"
	if _, err := runVerify(t, svc, first); err != nil {
		t.Fatalf("first verify: %v", err)
	}

	before := snapshotTree(t, repo)
	second := VerifyInput{
		TaskID:         "T-010-first-gap",
		Result:         "fail",
		Summary:        "Second gap",
		CreateFollowup: true,
		FollowupTitle:  "Second gap",
	}
	result, err := runVerify(t, svc, second)
	if err != nil {
		t.Fatalf("second verify: %v", err)
	}
	if result.FollowupTaskID != "T-011-second-gap" {
		t.Fatalf("second follow-up id = %q", result.FollowupTaskID)
	}

	want := append([]string{
		"planning/STATE.md",
		"planning/tasks/T-010-first-gap.md",
		"planning/tasks/T-011-second-gap.md",
	}, verifyArtifactPaths("T-010-first-gap")...)
	sortStrings(want)
	got := changedPaths(t, before, snapshotTree(t, repo))
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("second verify write set = %v, want %v", got, want)
	}
	_, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(tasks) != 4 {
		t.Fatalf("ledger holds %d tasks, want 4", len(tasks))
	}
}

// While another owner holds the lock, verify refuses with lock_held and
// leaves the repository untouched.
func TestVerifyRefusesWhileLockHeld(t *testing.T) {
	svc, repo := verifyFixture(t)
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: svc.paths.LockRepository(),
		Command:    "something else",
		Capability: repolock.Capability{Commands: []string{"something else"}},
	})
	if err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })

	before := snapshotTree(t, repo)
	_, err = runVerify(t, svc, baseVerifyInput())
	if err == nil || MachineFailureFor(err).Code != MachineCodeLockHeld {
		t.Fatalf("verify under a held lock = %v, want lock_held", err)
	}
	if got := snapshotTree(t, repo); !mapEqual(got, before) {
		t.Fatal("verify changed repository bytes despite lock_held")
	}
}

// An external edit landing between the snapshot and the write set is a
// write_conflict: verify publishes nothing and the external bytes survive.
func TestVerifyConflictPreservesExternalEdit(t *testing.T) {
	svc, repo := verifyFixture(t)
	statePath := filepath.Join(repo, "planning", "STATE.md")
	taskPath := filepath.Join(repo, "planning", "tasks", "T-002.md")
	stateBefore := readBytes(t, statePath)
	taskBefore := readBytes(t, taskPath)

	installLifecycleHook(t, func() {
		writeFile(t, statePath, stateBefore+"<!-- external edit -->\n")
	})
	_, err := runVerify(t, svc, baseVerifyInput())
	if err == nil || MachineFailureFor(err).Code != MachineCodeWriteConflict {
		t.Fatalf("verify against an external edit = %v, want write_conflict", err)
	}
	if len(MachineFailureFor(err).Snapshots) == 0 {
		t.Fatal("verify conflict reported no byte-evidence snapshots")
	}
	if got := readBytes(t, statePath); !strings.Contains(got, "external edit") {
		t.Fatalf("verify overwrote the external edit:\n%s", got)
	}
	if got := readBytes(t, taskPath); got != taskBefore {
		t.Fatal("verify published the task file despite the conflict")
	}
	if _, statErr := os.Stat(filepath.Join(repo, "planning", "artifacts", "verify")); statErr == nil {
		t.Fatal("verify published artifacts despite the conflict")
	}
}

// A task added while the candidate is being validated is a stale-ledger
// refusal: nothing is published.
func TestVerifyRefusesTaskAddedDuringValidation(t *testing.T) {
	svc, repo := verifyFixture(t)
	before := snapshotTree(t, repo)
	installLifecycleHook(t, func() {
		writeTask(t, repo, "T-010-concurrent", "Concurrent", "todo", "low", "specs/v0.1.0.md#summary", nil)
	})

	_, err := runVerify(t, svc, baseVerifyInput())
	if err == nil || MachineFailureFor(err).Code != MachineCodeValidationFailed {
		t.Fatalf("verify with a concurrent task addition = %v, want validation_failed", err)
	}
	got := changedPaths(t, before, snapshotTree(t, repo))
	if strings.Join(got, "\n") != "planning/tasks/T-010-concurrent.md" {
		t.Fatalf("verify changed repository bytes beyond the concurrent addition: %v", got)
	}
}

// A publication failure after the first file lands rolls the transaction back
// to the original bytes and removes transaction-created files.
func TestVerifyPublicationFailureRollsBack(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission-based fault injection is ineffective as root")
	}
	for _, tc := range []struct {
		name string
		// block makes the named boundary fail once the transaction snapshot
		// has been taken.
		block func(t *testing.T, repo string)
	}{
		{
			name: "artifact publication",
			block: func(t *testing.T, repo string) {
				artifacts := filepath.Join(repo, "planning", "artifacts")
				if err := os.MkdirAll(artifacts, 0o755); err != nil {
					t.Fatalf("seed artifacts dir: %v", err)
				}
				if err := os.Chmod(artifacts, 0o500); err != nil {
					t.Fatalf("lock artifacts dir: %v", err)
				}
				t.Cleanup(func() { _ = os.Chmod(artifacts, 0o755) })
			},
		},
		{
			name: "state publication",
			block: func(t *testing.T, repo string) {
				planning := filepath.Join(repo, "planning")
				if err := os.Chmod(planning, 0o500); err != nil {
					t.Fatalf("lock planning dir: %v", err)
				}
				t.Cleanup(func() { _ = os.Chmod(planning, 0o755) })
			},
		},
		{
			name: "task publication",
			block: func(t *testing.T, repo string) {
				tasks := filepath.Join(repo, "planning", "tasks")
				if err := os.Chmod(tasks, 0o500); err != nil {
					t.Fatalf("lock tasks dir: %v", err)
				}
				t.Cleanup(func() { _ = os.Chmod(tasks, 0o755) })
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo := verifyFixture(t)
			installLifecycleHook(t, func() { tc.block(t, repo) })
			statePath := filepath.Join(repo, "planning", "STATE.md")
			taskPath := filepath.Join(repo, "planning", "tasks", "T-002.md")
			stateBefore := readBytes(t, statePath)
			taskBefore := readBytes(t, taskPath)

			input := baseVerifyInput()
			input.Result = "fail"
			input.CreateFollowup = true
			input.FollowupTitle = "Rollback probe"
			_, err := runVerify(t, svc, input)
			if err == nil || MachineFailureFor(err).Code != MachineCodePartialWrite {
				t.Fatalf("verify with a failing %s = %v, want partial_write", tc.name, err)
			}
			assertNoRootLeak(t, repo, err)
			if got := readBytes(t, statePath); got != stateBefore {
				t.Fatalf("%s left STATE.md rolled forward:\n%s", tc.name, got)
			}
			if got := readBytes(t, taskPath); got != taskBefore {
				t.Fatalf("%s left the task rolled forward:\n%s", tc.name, got)
			}
			if _, statErr := os.Stat(filepath.Join(repo, "planning", "tasks", "T-003-rollback-probe.md")); statErr == nil {
				t.Fatalf("%s left the transaction-created follow-up on disk", tc.name)
			}
			artifactDir := filepath.Join(repo, "planning", "artifacts", "verify", "T-002", verifyTimestamp)
			for _, name := range []string{"plan.md", "report.json", "report.md"} {
				if _, statErr := os.Stat(filepath.Join(artifactDir, name)); statErr == nil {
					t.Fatalf("%s left the transaction-created artifact %s on disk", tc.name, name)
				}
			}
		})
	}
}

// A follow-up destination that cannot hold the candidate file — here one
// occupied by a directory — refuses before anything is written, with the
// reported (never absolute) path in the evidence.
func TestVerifyRefusesUnusableFollowupDestination(t *testing.T) {
	svc, repo := verifyFixture(t)
	occupied := filepath.Join(repo, "planning", "tasks", "T-010-occupied-gap.md")
	if err := os.MkdirAll(occupied, 0o755); err != nil {
		t.Fatalf("occupy follow-up path: %v", err)
	}
	before := snapshotTree(t, repo)

	input := baseVerifyInput()
	input.Result = "fail"
	input.CreateFollowup = true
	input.FollowupTitle = "Occupied gap"
	_, err := runVerify(t, svc, input)
	if err == nil || MachineFailureFor(err).Code != MachineCodeRepositoryInvalid {
		t.Fatalf("verify onto an unusable follow-up destination = %v, want repository_invalid", err)
	}
	assertNoRootLeak(t, repo, err)
	after := snapshotTree(t, repo)
	if len(after) != len(before) {
		t.Fatalf("refusal changed the file set: %d -> %d files", len(before), len(after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Fatalf("refusal changed %s", path)
		}
	}
}

// Verify never touches the status bytes of the task it records: even a
// non-canonical spelling survives byte for byte, because verify's declared
// field reach is updated_at and Implementation Notes only.
func TestVerifyPreservesNonCanonicalStatusBytes(t *testing.T) {
	svc, repo := verifyFixture(t)
	selected := filepath.Join(repo, "planning", "tasks", "T-002.md")
	writeFile(t, selected, strings.Replace(readBytes(t, selected),
		"status: completed", "status:   'completed'", 1))
	before := readBytes(t, selected)

	if _, err := runVerify(t, svc, baseVerifyInput()); err != nil {
		t.Fatalf("verify: %v", err)
	}
	after := readBytes(t, selected)
	if !strings.Contains(after, "status:   'completed'") {
		t.Fatalf("verify rewrote the status line it does not own:\n%s", after)
	}
	if after == before {
		t.Fatal("verify changed nothing")
	}
}

// verifyDelegationFixture acquires a loop-style delegating lock over the
// fixture repository, granting exactly the write set a delegated verify must
// claim for the selected task.
func verifyDelegationFixture(t *testing.T, selected string, writes []string) (*Service, string, repolock.Delegation) {
	t.Helper()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, selected, "Verified item", "completed", "high", "specs/v0.1.0.md#summary", nil)
	writeFile(t, filepath.Join(repo, "planning", "tasks", "T-009-sentinel.md"), `---
id: T-009-sentinel
title: Sentinel
status: todo
priority: low
spec_ref: specs/v0.1.0.md#summary
dependencies: []
updated_at: "2026-03-31T00:00:00Z"
---

# T-009-sentinel Sentinel
`)
	svc := newTestService(t, repo, time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC))
	executable := filepath.Join(t.TempDir(), "taskrail")
	writeFile(t, executable, "taskrail-bytes")
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: svc.paths.LockRepository(),
		Command:    "loop",
		Capability: repolock.Capability{
			Commands:     []string{"loop"},
			SelectedTask: selected,
			Writes:       writes,
		},
		ExecutablePath: executable,
	})
	if err != nil {
		t.Fatalf("acquire delegating lock: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })
	delegation, err := lock.Delegation()
	if err != nil {
		t.Fatalf("delegation: %v", err)
	}
	return svc, repo, delegation
}

func asVerifyDelegate(t *testing.T, delegation repolock.Delegation) {
	t.Helper()
	t.Setenv("TASKRAIL_DELEGATION_ID", "0123456789abcdef0123456789abcdef")
	t.Setenv("TASKRAIL_DELEGATION_TOKEN", delegation.Token)
	t.Setenv("TASKRAIL_EXECUTABLE_SHA256", delegation.ExecutableSHA256)
}

// delegatedVerifyWrites is the exact write set a delegated verify claims for
// T-002: state, the selected task, its three artifact files, and — when the
// grant covers creation — the follow-up task file.
func delegatedVerifyWrites(followup string) []string {
	writes := append([]string{
		"planning/STATE.md",
		"planning/tasks/T-002.md",
	}, verifyArtifactPaths("T-002")...)
	if followup != "" {
		writes = append(writes, "planning/tasks/"+followup+".md")
	}
	return writes
}

// A delegated verify joins its parent's narrowed grant and publishes normally;
// a wrong task, an unapproved follow-up, or an unapproved artifact set refuses
// without writes; and the sentinel task a delegate never selected stays
// byte-identical.
func TestDelegatedVerifyWrites(t *testing.T) {
	t.Run("permitted verification publishes normally", func(t *testing.T) {
		svc, repo, delegation := verifyDelegationFixture(t, "T-002", delegatedVerifyWrites(""))
		asVerifyDelegate(t, delegation)
		sentinel := filepath.Join(repo, "planning", "tasks", "T-009-sentinel.md")
		sentinelBefore := readBytes(t, sentinel)

		if _, err := runVerify(t, svc, baseVerifyInput()); err != nil {
			t.Fatalf("delegated verify: %v", err)
		}
		if got := readBytes(t, sentinel); got != sentinelBefore {
			t.Fatal("delegated verify rewrote the unselected sentinel task")
		}
		task := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-002.md"))
		if !strings.Contains(task, "verification pass") {
			t.Fatalf("delegated verify did not publish the task note:\n%s", task)
		}
	})

	t.Run("permitted follow-up creation publishes normally", func(t *testing.T) {
		svc, _, delegation := verifyDelegationFixture(t, "T-002",
			delegatedVerifyWrites("T-010-delegated-gap"))
		asVerifyDelegate(t, delegation)

		input := baseVerifyInput()
		input.Result = "fail"
		input.CreateFollowup = true
		input.FollowupTitle = "Delegated gap"
		result, err := runVerify(t, svc, input)
		if err != nil {
			t.Fatalf("delegated verify with follow-up: %v", err)
		}
		if result.FollowupTaskID != "T-010-delegated-gap" {
			t.Fatalf("follow-up id = %q", result.FollowupTaskID)
		}
	})

	t.Run("another task refuses without writes", func(t *testing.T) {
		svc, repo, delegation := verifyDelegationFixture(t, "T-002", delegatedVerifyWrites(""))
		asVerifyDelegate(t, delegation)
		writeTask(t, repo, "T-005-other", "Other", "completed", "low", "specs/v0.1.0.md#summary", nil)

		before := snapshotTree(t, repo)
		input := baseVerifyInput()
		input.TaskID = "T-005-other"
		if _, err := runVerify(t, svc, input); err == nil || MachineFailureFor(err).Code != MachineCodeDelegatedRefused {
			t.Fatalf("delegated verify of an unselected task = %v, want delegated_write_refused", err)
		}
		if got := snapshotTree(t, repo); !mapEqual(got, before) {
			t.Fatal("cross-task delegation changed repository bytes")
		}
	})

	t.Run("unapproved follow-up creation refuses without writes", func(t *testing.T) {
		svc, repo, delegation := verifyDelegationFixture(t, "T-002", delegatedVerifyWrites(""))
		asVerifyDelegate(t, delegation)

		before := snapshotTree(t, repo)
		input := baseVerifyInput()
		input.Result = "fail"
		input.CreateFollowup = true
		input.FollowupTitle = "Unapproved gap"
		if _, err := runVerify(t, svc, input); err == nil || MachineFailureFor(err).Code != MachineCodeDelegatedRefused {
			t.Fatalf("delegated verify creating an unapproved follow-up = %v, want delegated_write_refused", err)
		}
		if got := snapshotTree(t, repo); !mapEqual(got, before) {
			t.Fatal("unapproved follow-up creation changed repository bytes")
		}
	})

	t.Run("unapproved artifact set refuses without writes", func(t *testing.T) {
		svc, repo, delegation := verifyDelegationFixture(t, "T-002",
			[]string{"planning/STATE.md", "planning/tasks/T-002.md"})
		asVerifyDelegate(t, delegation)

		before := snapshotTree(t, repo)
		if _, err := runVerify(t, svc, baseVerifyInput()); err == nil || MachineFailureFor(err).Code != MachineCodeDelegatedRefused {
			t.Fatalf("delegated verify with an unapproved artifact set = %v, want delegated_write_refused", err)
		}
		if got := snapshotTree(t, repo); !mapEqual(got, before) {
			t.Fatal("unapproved artifact set changed repository bytes")
		}
	})

	t.Run("only canonical verify fields change", func(t *testing.T) {
		svc, repo, delegation := verifyDelegationFixture(t, "T-002", delegatedVerifyWrites(""))
		asVerifyDelegate(t, delegation)
		selected := filepath.Join(repo, "planning", "tasks", "T-002.md")
		writeFile(t, selected, strings.Replace(readBytes(t, selected),
			"updated_at:", "selected_sentinel: must-survive\nloop_policy: null\nupdated_at:", 1))
		before := readBytes(t, selected)

		if _, err := runVerify(t, svc, baseVerifyInput()); err != nil {
			t.Fatalf("delegated verify: %v", err)
		}
		after := readBytes(t, selected)
		if !strings.Contains(after, "selected_sentinel: must-survive") || !strings.Contains(after, "loop_policy: null") {
			t.Fatalf("delegated verify widened its field reach:\n%s", after)
		}
		// The only changes are the stamped timestamp and the appended note:
		// identity, ranking, anchoring, dependencies, and policy fields all
		// survive untouched.
		const stamp = "updated_at: \"2026-08-17T09:00:00Z\""
		lines := strings.Split(before, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "updated_at:") {
				lines[i] = stamp
			}
		}
		want := strings.TrimRight(strings.Join(lines, "\n"), "\n") +
			"\n\n## Implementation Notes\n\n- 2026-08-17T09:00:00Z: verification pass\n"
		if after != want {
			t.Fatalf("delegated verify changed an owned line:\nwant:\n%s\ngot:\n%s", want, after)
		}
	})
}
