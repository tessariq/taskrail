package taskrail

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tessariq/taskrail/internal/repolock"
)

// The T-282 transactional task-mutation matrix: task new, task rename, task
// repoint, and task dependency add/remove must hold the repository mutation
// lock, snapshot their complete consumed and collision set, validate the full
// candidate ledger before publication, publish only their declared task and
// state paths, and roll back handled failures while preserving external edits.

var taskMutationCommands = []string{
	"task new",
	"task rename",
	"task repoint",
	"task dependency add",
	"task dependency remove",
}

// taskMutationFixture seeds a repo whose T-001 is renameable, whose T-002
// carries one dependency edge and is repointable, and whose T-009 sentinel
// carries a frontmatter field no Taskrail struct models, so any writer that
// rewrites unselected tasks visibly drops it.
func taskMutationFixture(t *testing.T) (*Service, string) {
	t.Helper()
	repo := seedFixtureRepo(t)
	writeFile(t, filepath.Join(repo, "specs", "v0.1.0.md"),
		"# Taskrail v0.1.0\n\n## Summary\n\nFixture spec.\n\n## Details\n\nSecond anchor.\n")
	writeTask(t, repo, "T-001", "Base", "completed", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002", "Work item", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-001"})
	writeTask(t, repo, "T-004", "Extra", "todo", "low", "specs/v0.1.0.md#summary", nil)
	writeFile(t, filepath.Join(repo, "planning", "tasks", "T-009-sentinel.md"), `---
id: T-009-sentinel
title: Sentinel
status: todo
priority: low
spec_ref: specs/v0.1.0.md#summary
dependencies: []
updated_at: "2026-03-31T00:00:00Z"
sentinel_marker: must-survive-every-task-mutation-write
---

# T-009-sentinel Sentinel
`)
	return newTestService(t, repo, time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)), repo
}

// runOneTaskMutation runs the named writer once against the fixture, returning
// its error. It is the shared body of the command matrix.
func runOneTaskMutation(t *testing.T, svc *Service, command string) error {
	t.Helper()
	switch command {
	case "task new":
		_, err := svc.CreateTask(CreateTaskInput{
			Title:   "Fresh item",
			Slug:    "Fresh item",
			SpecRef: "specs/v0.1.0.md#summary",
		})
		return err
	case "task rename":
		_, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "base"})
		return err
	case "task repoint":
		_, err := svc.RepointTask(RepointTaskInput{TaskID: "T-002", Area: "details"})
		return err
	case "task dependency add":
		_, err := svc.EditDependency(EditDependencyInput{
			TaskID: "T-002", DependencyID: "T-004", Operation: DependencyAdd,
		})
		return err
	case "task dependency remove":
		_, err := svc.EditDependency(EditDependencyInput{
			TaskID: "T-002", DependencyID: "T-001", Operation: DependencyRemove,
		})
		return err
	default:
		t.Fatalf("unknown task mutation command %q", command)
		return nil
	}
}

// Each writer takes the discovered repository mutation lock and records its
// canonical command in the owner metadata for the whole transaction.
func TestTaskMutationWritersHoldTheMutationLock(t *testing.T) {
	for _, command := range taskMutationCommands {
		t.Run(command, func(t *testing.T) {
			svc, _ := taskMutationFixture(t)
			var observed repolock.Status
			installLifecycleHook(t, func() {
				status, err := repolock.Inspect(svc.paths.LockRepository())
				if err != nil {
					t.Errorf("inspect lock during %s: %v", command, err)
					return
				}
				observed = status
			})
			if err := runOneTaskMutation(t, svc, command); err != nil {
				t.Fatalf("%s under observation: %v", command, err)
			}
			if !observed.Held || observed.Owner == nil {
				t.Fatalf("%s completed without holding the repository mutation lock: %+v", command, observed)
			}
			if observed.Owner.Command != command {
				t.Errorf("lock owner command = %q, want %q", observed.Owner.Command, command)
			}
		})
	}
}

// Each writer publishes only its declared task and state files: the sentinel
// task's unmodeled frontmatter field survives every command byte for byte,
// and the changed-path set is exactly the command's declared write set.
func TestTaskMutationWritersPublishExactWriteSets(t *testing.T) {
	for _, command := range taskMutationCommands {
		t.Run(command, func(t *testing.T) {
			svc, repo := taskMutationFixture(t)
			sentinel := filepath.Join(repo, "planning", "tasks", "T-009-sentinel.md")
			sentinelBefore := readBytes(t, sentinel)
			before := snapshotTree(t, repo)

			if err := runOneTaskMutation(t, svc, command); err != nil {
				t.Fatalf("%s: %v", command, err)
			}
			if got := readBytes(t, sentinel); got != sentinelBefore {
				t.Fatalf("%s rewrote the unselected sentinel task\nbefore:\n%s\nafter:\n%s", command, sentinelBefore, got)
			}

			want := map[string][]string{
				"task new":               {"planning/STATE.md", "planning/tasks/T-010-fresh-item.md"},
				"task rename":            {"planning/STATE.md", "planning/tasks/T-001.md", "planning/tasks/T-001-base.md", "planning/tasks/T-002.md"},
				"task repoint":           {"planning/STATE.md", "planning/tasks/T-002.md"},
				"task dependency add":    {"planning/STATE.md", "planning/tasks/T-002.md"},
				"task dependency remove": {"planning/STATE.md", "planning/tasks/T-002.md"},
			}[command]
			sortStrings(want)
			got := changedPaths(t, before, snapshotTree(t, repo))
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Fatalf("%s write set = %v, want %v", command, got, want)
			}
		})
	}
}

// While another owner holds the lock, every task mutation writer refuses with
// lock_held and leaves the repository untouched.
func TestTaskMutationWritersRefuseWhileLockHeld(t *testing.T) {
	for _, command := range taskMutationCommands {
		t.Run(command, func(t *testing.T) {
			svc, repo := taskMutationFixture(t)
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
			err = runOneTaskMutation(t, svc, command)
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
func TestTaskMutationConflictPreservesExternalEdit(t *testing.T) {
	for _, command := range taskMutationCommands {
		t.Run(command, func(t *testing.T) {
			svc, repo := taskMutationFixture(t)
			statePath := filepath.Join(repo, "planning", "STATE.md")
			stateBefore := readBytes(t, statePath)

			installLifecycleHook(t, func() {
				writeFile(t, statePath, stateBefore+"<!-- external edit -->\n")
			})
			err := runOneTaskMutation(t, svc, command)
			if err == nil || MachineFailureFor(err).Code != MachineCodeWriteConflict {
				t.Fatalf("%s against an external edit = %v, want write_conflict", command, err)
			}
			if len(MachineFailureFor(err).Snapshots) == 0 {
				t.Fatalf("%s conflict reported no byte-evidence snapshots", command)
			}
			if got := readBytes(t, statePath); !strings.Contains(got, "external edit") {
				t.Fatalf("%s overwrote the external edit:\n%s", command, got)
			}
		})
	}
}

// None of the task mutation writers is a delegated command: a loop child
// invoking any of them is refused before anything is read or written, so a
// delegate can never widen its parent's selected work through them.
func TestTaskMutationWritersRefuseDelegatedInvocation(t *testing.T) {
	for _, command := range taskMutationCommands {
		t.Run(command, func(t *testing.T) {
			svc, repo := taskMutationFixture(t)
			before := snapshotTree(t, repo)
			t.Setenv("TASKRAIL_DELEGATION_TOKEN", "child-token")

			err := runOneTaskMutation(t, svc, command)
			if err == nil || MachineFailureFor(err).Code != MachineCodeDelegatedRefused {
				t.Fatalf("delegated %s = %v, want delegated_write_refused", command, err)
			}
			if got := snapshotTree(t, repo); !mapEqual(got, before) {
				t.Fatalf("delegated %s changed repository bytes", command)
			}
		})
	}
}

// A publication failure after the first file lands rolls the transaction back
// to the original bytes: task creation removes its transaction-created file,
// and a rename restores the task file its removal had already taken.
func TestTaskMutationPublicationFailureRollsBack(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission-based fault injection is ineffective as root")
	}
	for _, command := range []string{"task new", "task rename", "task repoint", "task dependency add"} {
		t.Run(command, func(t *testing.T) {
			svc, repo := taskMutationFixture(t)
			statePath := filepath.Join(repo, "planning", "STATE.md")
			stateBefore := readBytes(t, statePath)
			tasksDir := filepath.Join(repo, "planning", "tasks")
			// STATE.md sorts before every tasks/ path, so it publishes first; a
			// read-only tasks directory then fails the first task publication
			// (for rename, the removal of the old path).
			installLifecycleHook(t, func() {
				if err := os.Chmod(tasksDir, 0o500); err != nil {
					t.Fatalf("lock tasks dir: %v", err)
				}
				t.Cleanup(func() { _ = os.Chmod(tasksDir, 0o755) })
			})

			err := runOneTaskMutation(t, svc, command)
			if err == nil || MachineFailureFor(err).Code != MachineCodePartialWrite {
				t.Fatalf("%s with a failing task publication = %v, want partial_write", command, err)
			}
			if got := readBytes(t, statePath); got != stateBefore {
				t.Fatalf("%s left STATE.md rolled forward:\n%s", command, got)
			}
			if _, statErr := os.Stat(filepath.Join(tasksDir, "T-010-fresh-item.md")); command == "task new" && statErr == nil {
				t.Fatal("task new left its transaction-created task file on disk")
			}
			if _, statErr := os.Stat(filepath.Join(tasksDir, "T-001.md")); command == "task rename" && statErr != nil {
				t.Fatalf("rename lost the source task file: %v", statErr)
			}
			if _, statErr := os.Stat(filepath.Join(tasksDir, "T-001-base.md")); command == "task rename" && statErr == nil {
				t.Fatal("rename left the target task file on disk")
			}
		})
	}
}

// A task corpus that moves while the candidate is being validated is a
// stale-ledger refusal: nothing is published.
func TestTaskMutationRefusesCorpusChangeDuringValidation(t *testing.T) {
	svc, repo := taskMutationFixture(t)
	stateBefore := readBytes(t, filepath.Join(repo, "planning", "STATE.md"))
	installLifecycleHook(t, func() {
		writeTask(t, repo, "T-011-concurrent", "Concurrent", "todo", "low", "specs/v0.1.0.md#summary", nil)
	})

	_, err := svc.CreateTask(CreateTaskInput{Title: "Fresh item", Slug: "Fresh item", SpecRef: "specs/v0.1.0.md#summary"})
	if err == nil || MachineFailureFor(err).Code != MachineCodeValidationFailed {
		t.Fatalf("task new with a concurrent corpus change = %v, want validation_failed", err)
	}
	if got := readBytes(t, filepath.Join(repo, "planning", "STATE.md")); got != stateBefore {
		t.Fatal("task new published a state projection from a stale corpus")
	}
}

// Rename and repoint patch only their declared frontmatter lines: a field no
// Taskrail struct models survives the write byte for byte on the selected and
// inbound tasks alike.
func TestTaskMutationWritersPreserveUnmodeledFrontmatter(t *testing.T) {
	markTask := func(t *testing.T, repo, name string) {
		t.Helper()
		path := filepath.Join(repo, "planning", "tasks", name+".md")
		writeFile(t, path, strings.Replace(readBytes(t, path),
			"updated_at:", "unmodeled_marker: must-survive\nupdated_at:", 1))
	}

	t.Run("task rename preserves the selected task's unmodeled fields", func(t *testing.T) {
		svc, repo := taskMutationFixture(t)
		markTask(t, repo, "T-001")
		markTask(t, repo, "T-002")

		if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "base"}); err != nil {
			t.Fatalf("rename: %v", err)
		}
		renamed := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-001-base.md"))
		if !strings.Contains(renamed, "unmodeled_marker: must-survive") {
			t.Fatalf("rename dropped the selected task's unmodeled frontmatter:\n%s", renamed)
		}
		if !strings.Contains(renamed, "id: T-001-base") {
			t.Fatalf("rename did not publish the new id:\n%s", renamed)
		}
		inbound := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-002.md"))
		if !strings.Contains(inbound, "unmodeled_marker: must-survive") {
			t.Fatalf("rename dropped the inbound task's unmodeled frontmatter:\n%s", inbound)
		}
		if !strings.Contains(inbound, "- T-001-base") {
			t.Fatalf("rename did not repoint the inbound dependency:\n%s", inbound)
		}
	})

	t.Run("task repoint preserves the selected task's unmodeled fields", func(t *testing.T) {
		svc, repo := taskMutationFixture(t)
		markTask(t, repo, "T-002")

		if _, err := svc.RepointTask(RepointTaskInput{TaskID: "T-002", Area: "details"}); err != nil {
			t.Fatalf("repoint: %v", err)
		}
		got := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-002.md"))
		if !strings.Contains(got, "unmodeled_marker: must-survive") {
			t.Fatalf("repoint dropped the selected task's unmodeled frontmatter:\n%s", got)
		}
		if !strings.Contains(got, "spec_ref: specs/v0.1.0.md#details") {
			t.Fatalf("repoint did not publish the new spec_ref:\n%s", got)
		}
	})
}
