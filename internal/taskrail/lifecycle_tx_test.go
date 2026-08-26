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

// The T-233 transactional lifecycle matrix: next, start, complete, block,
// unblock, and release must hold the repository mutation lock, publish only their declared
// task/state files, validate the complete candidate before publication, and
// roll back handled failures while preserving external edits.

var lifecycleCommands = []string{"next", "start", "complete", "block", "unblock", "release"}

// lifecycleFixture seeds a repo whose T-002 is startable and whose T-009
// sentinel carries a frontmatter field no Taskrail struct models, so any writer
// that rewrites unselected tasks visibly drops it.
func lifecycleFixture(t *testing.T) (*Service, string) {
	t.Helper()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Work item", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeFile(t, filepath.Join(repo, "planning", "tasks", "T-009-sentinel.md"), `---
id: T-009-sentinel
title: Sentinel
status: todo
priority: low
spec_ref: specs/v0.1.0.md#summary
dependencies: []
updated_at: "2026-03-31T00:00:00Z"
sentinel_marker: must-survive-every-lifecycle-write
---

# T-009-sentinel Sentinel
`)
	return newTestService(t, repo, time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)), repo
}

func lifecycleSentinelPath(svc *Service) string {
	return filepath.Join(svc.paths.TasksDir, "T-009-sentinel.md")
}

// prepareLifecycleCommand drives T-002 into the status the named writer acts
// on, so the command under test runs exactly once on a legal transition.
func prepareLifecycleCommand(t *testing.T, svc *Service, command string) {
	t.Helper()
	switch command {
	case "complete":
		if _, err := svc.Start("T-002"); err != nil {
			t.Fatalf("prepare start: %v", err)
		}
	case "unblock":
		if _, err := svc.Block("T-002", "waiting"); err != nil {
			t.Fatalf("prepare block: %v", err)
		}
	case "release":
		if _, err := svc.Start("T-002"); err != nil {
			t.Fatalf("prepare start: %v", err)
		}
	}
}

// runOneLifecycleCommand runs the named writer once against the fixture,
// returning its error. It is the shared body of the command matrix.
func runOneLifecycleCommand(t *testing.T, svc *Service, command string) error {
	t.Helper()
	switch command {
	case "next":
		_, err := svc.Next()
		return err
	case "start":
		_, err := svc.Start("T-002")
		return err
	case "complete":
		_, err := svc.Complete("T-002", "done")
		return err
	case "block":
		_, err := svc.Block("T-002", "waiting")
		return err
	case "unblock":
		_, err := svc.Unblock("T-002", "recovered")
		return err
	case "release":
		_, err := svc.ReleaseTask(ReleaseTaskInput{TaskID: "T-002", Reason: "rework"})
		return err
	default:
		t.Fatalf("unknown lifecycle command %q", command)
		return nil
	}
}

// installLifecycleHook installs the transaction validation-phase hook for one
// test and guarantees its removal, because the hook is package-global.
func installLifecycleHook(t *testing.T, hook func()) {
	t.Helper()
	testHookWriterValidated = hook
	t.Cleanup(func() { testHookWriterValidated = nil })
}

func installWriterCandidateBuiltHook(t *testing.T, hook func()) {
	t.Helper()
	testHookWriterCandidateBuilt = hook
	t.Cleanup(func() { testHookWriterCandidateBuilt = nil })
}

// observeLockDuring runs one lifecycle command with the validation-phase test
// hook inspecting the lock. The hook runs after the snapshot and before
// publication, which is exactly the window in which the writer must hold it.
func observeLockDuring(t *testing.T, svc *Service, command string) repolock.Status {
	t.Helper()
	var observed repolock.Status
	installLifecycleHook(t, func() {
		status, err := repolock.Inspect(svc.paths.LockRepository())
		if err != nil {
			t.Errorf("inspect lock during %s: %v", command, err)
			return
		}
		observed = status
	})
	if err := runOneLifecycleCommand(t, svc, command); err != nil {
		t.Fatalf("%s under observation: %v", command, err)
	}
	return observed
}

// Each writer takes the discovered repository mutation lock and records its
// canonical command in the owner metadata for the whole transaction.
func TestLifecycleWritersHoldTheMutationLock(t *testing.T) {
	for _, command := range lifecycleCommands {
		t.Run(command, func(t *testing.T) {
			svc, _ := lifecycleFixture(t)
			prepareLifecycleCommand(t, svc, command)
			observed := observeLockDuring(t, svc, command)
			if !observed.Held || observed.Owner == nil {
				t.Fatalf("%s completed without holding the repository mutation lock: %+v", command, observed)
			}
			wantCommand := command
			if command == "release" {
				wantCommand = "task release"
			}
			if observed.Owner.Command != wantCommand {
				t.Errorf("lock owner command = %q, want %q", observed.Owner.Command, wantCommand)
			}
			if observed.Owner.PID != os.Getpid() {
				t.Errorf("lock owner pid = %d, want %d", observed.Owner.PID, os.Getpid())
			}
			if observed.Owner.StorageMode != repolock.ModeCommitted {
				t.Errorf("lock owner storage mode = %q, want committed", observed.Owner.StorageMode)
			}
		})
	}
}

// A writer must not publish anything but its declared task and state files: the
// sentinel task's unmodeled frontmatter field survives every command byte for
// byte, proving the save-all rewrite is gone.
func TestLifecycleWritersNeverRewriteUnselectedTasks(t *testing.T) {
	for _, command := range lifecycleCommands {
		t.Run(command, func(t *testing.T) {
			svc, _ := lifecycleFixture(t)
			prepareLifecycleCommand(t, svc, command)
			sentinel := lifecycleSentinelPath(svc)
			before := readBytes(t, sentinel)
			if !strings.Contains(before, "sentinel_marker") {
				t.Fatal("sentinel fixture lost its marker before the run")
			}

			if err := runOneLifecycleCommand(t, svc, command); err != nil {
				t.Fatalf("%s: %v", command, err)
			}
			if got := readBytes(t, sentinel); got != before {
				t.Fatalf("%s rewrote the unselected sentinel task\nbefore:\n%s\nafter:\n%s", command, before, got)
			}
		})
	}
}

func TestLifecycleWritersPreserveSelectedTaskUnknownFrontmatter(t *testing.T) {
	for _, command := range []string{"start", "complete", "block", "unblock", "release"} {
		t.Run(command, func(t *testing.T) {
			svc, _ := lifecycleFixture(t)
			prepareLifecycleCommand(t, svc, command)
			selected := filepath.Join(svc.paths.TasksDir, "T-002.md")
			writeFile(t, selected, strings.Replace(readBytes(t, selected),
				"updated_at:", "selected_sentinel: must-survive\nupdated_at:", 1))

			if err := runOneLifecycleCommand(t, svc, command); err != nil {
				t.Fatalf("%s: %v", command, err)
			}
			if got := readBytes(t, selected); !strings.Contains(got, "selected_sentinel: must-survive") {
				t.Fatalf("%s dropped selected-task metadata:\n%s", command, got)
			}
		})
	}
}

func TestLifecycleStartPreservesSelectedTaskBodyBytes(t *testing.T) {
	svc, _ := lifecycleFixture(t)
	selected := filepath.Join(svc.paths.TasksDir, "T-002.md")
	writeFile(t, selected, strings.Replace(readBytes(t, selected), "---\n\n#", "---\n\n\n#", 1))
	before := readBytes(t, selected)

	if _, err := svc.Start("T-002"); err != nil {
		t.Fatal(err)
	}
	after := readBytes(t, selected)
	beforeBody := before[strings.Index(before, "\n---\n")+len("\n---\n"):]
	afterBody := after[strings.Index(after, "\n---\n")+len("\n---\n"):]
	if afterBody != beforeBody {
		t.Fatalf("start changed body bytes:\nbefore:%q\nafter: %q", beforeBody, afterBody)
	}
}

func TestLifecycleStartAcceptsMixedBodyLineEndings(t *testing.T) {
	svc, _ := lifecycleFixture(t)
	selected := filepath.Join(svc.paths.TasksDir, "T-002.md")
	writeFile(t, selected, strings.ReplaceAll(readBytes(t, selected), "Fixture task.", "Fixture task.\r"))

	if _, err := svc.Start("T-002"); err != nil {
		t.Fatalf("start mixed-line-ending task: %v", err)
	}
	if got := readBytes(t, selected); !strings.Contains(got, "Fixture task.\r\n") {
		t.Fatalf("start normalized mixed body bytes: %q", got)
	}
}

// While another owner holds the lock, every lifecycle writer refuses with
// lock_held and leaves the repository untouched.
func TestLifecycleWritersRefuseWhileLockHeld(t *testing.T) {
	for _, command := range lifecycleCommands {
		t.Run(command, func(t *testing.T) {
			svc, repo := lifecycleFixture(t)
			prepareLifecycleCommand(t, svc, command)
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
			err = runOneLifecycleCommand(t, svc, command)
			if err == nil || MachineFailureFor(err).Code != MachineCodeLockHeld {
				t.Fatalf("%s under a held lock = %v, want lock_held", command, err)
			}
			if got := snapshotTree(t, repo); !mapEqual(got, before) {
				t.Fatalf("%s changed repository bytes despite lock_held", command)
			}
		})
	}
}

// An external edit landing between the snapshot and the write set is a
// write_conflict: the command publishes nothing and the external bytes survive.
func TestLifecycleConflictPreservesExternalEdit(t *testing.T) {
	for _, command := range lifecycleCommands {
		t.Run(command, func(t *testing.T) {
			svc, repo := lifecycleFixture(t)
			prepareLifecycleCommand(t, svc, command)
			statePath := filepath.Join(repo, "planning", "STATE.md")
			taskPath := filepath.Join(repo, "planning", "tasks", "T-002.md")
			stateBefore := readBytes(t, statePath)
			taskBefore := readBytes(t, taskPath)

			installLifecycleHook(t, func() {
				writeFile(t, statePath, stateBefore+"<!-- external edit -->\n")
			})
			err := runOneLifecycleCommand(t, svc, command)
			if err == nil || MachineFailureFor(err).Code != MachineCodeWriteConflict {
				t.Fatalf("%s against an external edit = %v, want write_conflict", command, err)
			}
			if len(MachineFailureFor(err).Snapshots) == 0 {
				t.Fatalf("%s conflict reported no byte-evidence snapshots", command)
			}
			if got := readBytes(t, statePath); !strings.Contains(got, "external edit") {
				t.Fatalf("%s overwrote the external edit:\n%s", command, got)
			}
			if got := readBytes(t, taskPath); got != taskBefore {
				t.Fatalf("%s published the task file despite the conflict", command)
			}
		})
	}
}

func TestLifecycleRefusesTaskAddedDuringValidation(t *testing.T) {
	svc, repo := lifecycleFixture(t)
	before := readBytes(t, filepath.Join(repo, "planning", "STATE.md"))
	installLifecycleHook(t, func() {
		writeTask(t, repo, "T-010-concurrent", "Concurrent", "todo", "low", "specs/v0.1.0.md#summary", nil)
	})

	_, err := svc.Next()
	if err == nil || MachineFailureFor(err).Code != MachineCodeValidationFailed {
		t.Fatalf("next with a concurrent task addition = %v, want validation_failed", err)
	}
	if got := readBytes(t, filepath.Join(repo, "planning", "STATE.md")); got != before {
		t.Fatal("next published a stale state projection")
	}
}

func TestLifecycleRefusesTaskEditedBeforeTransactionSnapshot(t *testing.T) {
	svc, repo := lifecycleFixture(t)
	stateBefore := readBytes(t, filepath.Join(repo, "planning", "STATE.md"))
	selected := filepath.Join(repo, "planning", "tasks", "T-002.md")
	installLifecycleHook(t, func() {
		writeFile(t, selected, strings.Replace(readBytes(t, selected), "priority: high", "priority: low", 1))
	})

	_, err := svc.Next()
	if err == nil || MachineFailureFor(err).Code != MachineCodeValidationFailed {
		t.Fatalf("next with a concurrent task edit = %v, want validation_failed", err)
	}
	if got := readBytes(t, filepath.Join(repo, "planning", "STATE.md")); got != stateBefore {
		t.Fatal("next published a state projection from stale task content")
	}
}

func TestLifecycleRefusesUnmodeledTaskByteChangeBeforeTransactionSnapshot(t *testing.T) {
	svc, repo := lifecycleFixture(t)
	stateBefore := readBytes(t, filepath.Join(repo, "planning", "STATE.md"))
	sentinel := lifecycleSentinelPath(svc)
	installWriterCandidateBuiltHook(t, func() {
		writeFile(t, sentinel, strings.Replace(readBytes(t, sentinel),
			"sentinel_marker: must-survive-every-lifecycle-write",
			"sentinel_marker: externally-updated", 1))
	})

	_, err := svc.Next()
	if err == nil || MachineFailureFor(err).Code != MachineCodeValidationFailed {
		t.Fatalf("next with an unmodeled concurrent task edit = %v, want validation_failed", err)
	}
	if got := readBytes(t, filepath.Join(repo, "planning", "STATE.md")); got != stateBefore {
		t.Fatal("next published a state projection from stale task bytes")
	}
	if got := readBytes(t, sentinel); !strings.Contains(got, "sentinel_marker: externally-updated") {
		t.Fatalf("next overwrote the external task bytes:\n%s", got)
	}
}

// A candidate that would introduce a validation violation the repository did
// not already have is never published: the transition refuses with
// validation_failed and the repository keeps its original bytes, while a
// pre-existing violation is preserved and reported rather than healed.
func TestLifecycleRefusesWhenTransitionAddsViolations(t *testing.T) {
	svc, repo := lifecycleFixture(t)
	// A drifted second task that is already in_progress while STATE.md names no
	// current task: starting T-002 would create "multiple in_progress", a
	// violation the baseline does not carry.
	writeTask(t, repo, "T-003-drift", "Drifted", "in_progress", "low", "specs/v0.1.0.md#summary", nil)
	before := snapshotTree(t, repo)

	err := runOneLifecycleCommand(t, svc, "start")
	if err == nil || MachineFailureFor(err).Code != MachineCodeValidationFailed {
		t.Fatalf("start into multiple in_progress = %v, want validation_failed", err)
	}
	if got := snapshotTree(t, repo); !mapEqual(got, before) {
		t.Fatal("refused transition changed repository bytes")
	}

	// A pre-existing violation the transition merely preserves does not refuse
	// the write: blocking a task whose null loop policy is already invalid
	// succeeds and reports the violation instead of healing it.
	writeTask(t, repo, "T-004-nullpolicy", "Null policy", "todo", "low", "specs/v0.1.0.md#summary", nil)
	nullPath := filepath.Join(repo, "planning", "tasks", "T-004-nullpolicy.md")
	writeFile(t, nullPath, strings.Replace(readBytes(t, nullPath),
		"dependencies: []", "dependencies: []\nloop_policy: null\nloop_reason: null", 1))

	result, err := svc.Block("T-004-nullpolicy", "explicitly held")
	if err != nil {
		t.Fatalf("block with a pre-existing violation: %v", err)
	}
	if result.Validation.Valid || !containsString(result.Validation.Violations, "loop_policy must be a string") {
		t.Fatalf("block healed or hid the pre-existing violation: %+v", result.Validation)
	}
	if got := readBytes(t, nullPath); !strings.Contains(got, "loop_policy: null") {
		t.Fatalf("block rewrote the null policy bytes:\n%s", got)
	}
}

// A publication failure after the first file lands rolls the transaction back
// to the original bytes and reports partial-write evidence.
func TestLifecyclePublicationFailureRollsBack(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission-based fault injection is ineffective as root")
	}
	for _, command := range []string{"start", "complete", "block", "unblock", "release"} {
		t.Run(command, func(t *testing.T) {
			svc, repo := lifecycleFixture(t)
			prepareLifecycleCommand(t, svc, command)
			statePath := filepath.Join(repo, "planning", "STATE.md")
			tasksDir := filepath.Join(repo, "planning", "tasks")
			stateBefore := readBytes(t, statePath)
			taskPath := filepath.Join(tasksDir, "T-002.md")
			taskBefore := readBytes(t, taskPath)
			// STATE.md sorts before the task file, so it publishes first; a
			// read-only tasks directory then fails the task publication.
			if err := os.Chmod(tasksDir, 0o500); err != nil {
				t.Fatalf("lock tasks dir: %v", err)
			}
			t.Cleanup(func() { _ = os.Chmod(tasksDir, 0o755) })

			err := runOneLifecycleCommand(t, svc, command)
			if err == nil || MachineFailureFor(err).Code != MachineCodePartialWrite {
				t.Fatalf("%s with a failing task publication = %v, want partial_write", command, err)
			}
			if got := readBytes(t, statePath); got != stateBefore {
				t.Fatalf("%s left STATE.md rolled forward:\n%s", command, got)
			}
			if got := readBytes(t, taskPath); got != taskBefore {
				t.Fatalf("%s left selected-task metadata partially published:\n%s", command, got)
			}
		})
	}
}

// A delegated loop child joins its parent's lock narrowed to the selected task
// and the exact write set, and every widening attempt refuses without writes.
func TestDelegatedLifecycleWrites(t *testing.T) {
	const selected = "T-002"

	// delegationFixture acquires a loop-style delegating lock over the fixture
	// repository and returns the service, delegation secrets, and granted writes.
	delegationFixture := func(t *testing.T, status string) (*Service, string, repolock.Delegation) {
		t.Helper()
		repo := seedFixtureRepo(t)
		writeTask(t, repo, "T-002", "Work item", status, "high", "specs/v0.1.0.md#summary", nil)
		if status == "in_progress" {
			writeFixtureState(t, repo, "v0.1.0", "T-002", "Work item", "in_progress")
		}
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
		executable, err := os.Executable()
		if err != nil {
			t.Fatalf("resolve test executable: %v", err)
		}
		lock, err := repolock.Acquire(context.Background(), repolock.Request{
			Repository:    svc.paths.LockRepository(),
			Command:       "loop",
			TransactionID: "0123456789abcdef0123456789abcdef",
			Capability: repolock.Capability{
				Commands:     []string{"loop"},
				SelectedTask: selected,
				Writes:       svc.loopDelegationGrant(selected).Writes,
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

	asDelegate := func(t *testing.T, delegation repolock.Delegation) {
		t.Helper()
		executable, err := os.Executable()
		if err != nil {
			t.Fatalf("resolve test executable: %v", err)
		}
		t.Setenv("TASKRAIL", executable)
		t.Setenv("TASKRAIL_DELEGATION_ID", "0123456789abcdef0123456789abcdef")
		t.Setenv("TASKRAIL_DELEGATION_TOKEN", delegation.Token)
		t.Setenv("TASKRAIL_EXECUTABLE_SHA256", delegation.ExecutableSHA256)
	}

	t.Run("permitted lifecycle work publishes normally", func(t *testing.T) {
		svc, repo, delegation := delegationFixture(t, "in_progress")
		asDelegate(t, delegation)

		result, err := svc.Complete(selected, "done under delegation")
		if err != nil {
			t.Fatalf("delegated complete: %v", err)
		}
		if result.Status != "completed" || !result.Validation.Valid {
			t.Fatalf("delegated complete result = %+v", result)
		}
		task := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-002.md"))
		if !strings.Contains(task, "status: completed") {
			t.Fatalf("delegated complete did not publish the task:\n%s", task)
		}
	})

	t.Run("wrong token refuses without writes", func(t *testing.T) {
		svc, repo, delegation := delegationFixture(t, "in_progress")
		asDelegate(t, delegation)
		t.Setenv("TASKRAIL_DELEGATION_TOKEN", "not-the-token")

		before := snapshotTree(t, repo)
		if _, err := svc.Complete(selected, "nope"); err == nil || MachineFailureFor(err).Code != MachineCodeDelegatedRefused {
			t.Fatalf("delegated complete with a wrong token = %v, want delegated_write_refused", err)
		}
		if got := snapshotTree(t, repo); !mapEqual(got, before) {
			t.Fatal("refused delegation changed repository bytes")
		}
	})

	t.Run("wrong invocation refuses without writes", func(t *testing.T) {
		svc, repo, delegation := delegationFixture(t, "in_progress")
		asDelegate(t, delegation)
		t.Setenv("TASKRAIL_DELEGATION_ID", "fedcba9876543210fedcba9876543210")

		before := snapshotTree(t, repo)
		if _, err := svc.Complete(selected, "nope"); err == nil || MachineFailureFor(err).Code != MachineCodeDelegatedRefused {
			t.Fatalf("delegated complete with a wrong invocation = %v, want delegated_write_refused", err)
		}
		if got := snapshotTree(t, repo); !mapEqual(got, before) {
			t.Fatal("wrong invocation changed repository bytes")
		}
	})

	t.Run("wrong executable refuses without writes", func(t *testing.T) {
		svc, repo, delegation := delegationFixture(t, "in_progress")
		asDelegate(t, delegation)
		t.Setenv("TASKRAIL", filepath.Join(t.TempDir(), "other-taskrail"))

		before := snapshotTree(t, repo)
		if _, err := svc.Complete(selected, "nope"); err == nil || MachineFailureFor(err).Code != MachineCodeDelegatedRefused {
			t.Fatalf("delegated complete with a wrong executable = %v, want delegated_write_refused", err)
		}
		if got := snapshotTree(t, repo); !mapEqual(got, before) {
			t.Fatal("wrong executable changed repository bytes")
		}
	})

	t.Run("another task refuses without writes", func(t *testing.T) {
		svc, repo, delegation := delegationFixture(t, "todo")
		asDelegate(t, delegation)
		writeTask(t, repo, "T-003-other", "Other", "todo", "low", "specs/v0.1.0.md#summary", nil)

		before := snapshotTree(t, repo)
		if _, err := svc.Start("T-003-other"); err == nil || MachineFailureFor(err).Code != MachineCodeDelegatedRefused {
			t.Fatalf("delegated start of an unselected task = %v, want delegated_write_refused", err)
		}
		if got := snapshotTree(t, repo); !mapEqual(got, before) {
			t.Fatal("cross-task delegation changed repository bytes")
		}
	})

	t.Run("next is not a delegated command", func(t *testing.T) {
		svc, repo, delegation := delegationFixture(t, "todo")
		asDelegate(t, delegation)

		before := snapshotTree(t, repo)
		if _, err := svc.Next(); err == nil || MachineFailureFor(err).Code != MachineCodeDelegatedRefused {
			t.Fatalf("delegated next = %v, want delegated_write_refused", err)
		}
		if got := snapshotTree(t, repo); !mapEqual(got, before) {
			t.Fatal("refused delegated next changed repository bytes")
		}
	})
}

// A loop issues one broad task-scoped grant before its child knows the concrete
// verification destination. Each lifecycle writer must authenticate that grant,
// then narrow to its own command and transaction paths.
func TestDelegatedLifecycleWritersShareLoopGrant(t *testing.T) {
	const selected = "T-002"
	repo := seedFixtureRepo(t)
	writeTask(t, repo, selected, "Work item", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC))
	setTestVerificationIDs(svc)
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository:    svc.paths.LockRepository(),
		Command:       "loop",
		TransactionID: "0123456789abcdef0123456789abcdef",
		Capability: repolock.Capability{Commands: []string{"loop"}, SelectedTask: selected,
			Writes: svc.loopDelegationGrant(selected).Writes},
		ExecutablePath: executable,
	})
	if err != nil {
		t.Fatalf("acquire loop lock: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })
	delegation, err := lock.Delegation()
	if err != nil {
		t.Fatalf("read loop delegation: %v", err)
	}
	t.Setenv("TASKRAIL", executable)
	t.Setenv("TASKRAIL_DELEGATION_ID", "0123456789abcdef0123456789abcdef")
	t.Setenv("TASKRAIL_DELEGATION_TOKEN", delegation.Token)
	t.Setenv("TASKRAIL_EXECUTABLE_SHA256", delegation.ExecutableSHA256)

	if _, err := svc.Start(selected); err != nil {
		t.Fatalf("delegated start: %v", err)
	}
	if _, err := svc.Block(selected, "blocked under the loop grant"); err != nil {
		t.Fatalf("delegated block: %v", err)
	}
	if _, err := svc.Unblock(selected, "resume under the loop grant"); err != nil {
		t.Fatalf("delegated unblock: %v", err)
	}
	if _, err := svc.Start(selected); err != nil {
		t.Fatalf("delegated restart: %v", err)
	}
	if _, err := svc.Complete(selected, "completed under the loop grant"); err != nil {
		t.Fatalf("delegated complete: %v", err)
	}
	if _, err := runVerify(t, svc, baseVerifyInput()); err != nil {
		t.Fatalf("delegated verify: %v", err)
	}
}

// Outside Git, the discovered non-Git repository places its lock beneath its
// own .taskrail/runtime/, and the lifecycle writers hold exactly that lock.
func TestLifecycleWritersLockNonGitRepositoriesUnderRuntime(t *testing.T) {
	repo := t.TempDir()
	seedFixtureTree(t, repo)
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: 1\n")
	writeTask(t, repo, "T-002", "Work item", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC))

	var lockPath string
	installLifecycleHook(t, func() {
		lockPath = repolock.LockPath(svc.paths.LockRepository())
		if _, err := os.Stat(lockPath); err != nil {
			t.Errorf("lock file missing during transaction: %v", err)
		}
	})
	if _, err := svc.Start("T-002"); err != nil {
		t.Fatalf("start in a non-Git repository: %v", err)
	}
	if want := filepath.Join(repo, ".taskrail", "runtime", "mutation.lock"); lockPath != want {
		t.Fatalf("lock path = %q, want %q", lockPath, want)
	}
}

func mapEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if other, ok := b[key]; !ok || other != value {
			return false
		}
	}
	return true
}
