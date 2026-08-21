package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tessariq/taskrail/internal/repolock"
)

// Structural writers repair state or publish specs without changing task files.
// They still need the common transaction boundary because their candidates are
// derived from the full task and spec corpus.
func TestStructuralWritersHoldLockAndLeaveTasksUntouched(t *testing.T) {
	for _, command := range []string{"repair", "spec add", "spec activate"} {
		t.Run(command, func(t *testing.T) {
			repo := seedFixtureRepo(t)
			writeTask(t, repo, "T-009-sentinel", "Sentinel", "todo", "low", "specs/v0.1.0.md#summary", nil)
			if command == "repair" {
				state := filepath.Join(repo, "planning", "STATE.md")
				writeFile(t, state, strings.Replace(readBytes(t, state), "todo: 1", "todo: 99", 1))
			} else {
				writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"), "# Taskrail v0.2.0\n\n## Summary\n")
			}
			svc := newTestService(t, repo, time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC))
			sentinel := filepath.Join(repo, "planning", "tasks", "T-009-sentinel.md")
			before := readBytes(t, sentinel)
			var observed repolock.Status
			installLifecycleHook(t, func() {
				var err error
				observed, err = repolock.Inspect(svc.paths.LockRepository())
				if err != nil {
					t.Errorf("inspect lock: %v", err)
				}
			})

			var err error
			switch command {
			case "repair":
				_, err = svc.Repair(RepairInput{Apply: true})
			case "spec add":
				_, err = svc.AddSpec("v0.3.0")
			case "spec activate":
				_, err = svc.ActivateSpec("v0.2.0")
			}
			if err != nil {
				t.Fatalf("%s: %v", command, err)
			}
			if !observed.Held || observed.Owner == nil || observed.Owner.Command != command {
				t.Fatalf("%s did not hold its mutation lock: %+v", command, observed)
			}
			if got := readBytes(t, sentinel); got != before {
				t.Fatalf("%s rewrote a task file", command)
			}
		})
	}
}

func TestStructuralWritersRefuseSnapshotConflicts(t *testing.T) {
	for _, command := range []string{"repair", "spec add", "spec activate"} {
		t.Run(command, func(t *testing.T) {
			repo := seedFixtureRepo(t)
			if command == "repair" {
				state := filepath.Join(repo, "planning", "STATE.md")
				writeFile(t, state, strings.Replace(readBytes(t, state), "todo: 1", "todo: 99", 1))
			} else {
				writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"), "# Taskrail v0.2.0\n\n## Summary\n")
			}
			svc := newTestService(t, repo, time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC))
			state := filepath.Join(repo, "planning", "STATE.md")
			before := readBytes(t, state)
			installLifecycleHook(t, func() {
				writeFile(t, state, before+"<!-- external edit -->\n")
			})

			var err error
			switch command {
			case "repair":
				_, err = svc.Repair(RepairInput{Apply: true})
			case "spec add":
				_, err = svc.AddSpec("v0.3.0")
			case "spec activate":
				_, err = svc.ActivateSpec("v0.2.0")
			}
			if err == nil || MachineFailureFor(err).Code != MachineCodeWriteConflict {
				t.Fatalf("%s with external state edit = %v, want write_conflict", command, err)
			}
			if got := readBytes(t, state); !strings.Contains(got, "external edit") {
				t.Fatalf("%s overwrote external state bytes", command)
			}
		})
	}
}

func TestAddSpecPreservesDestinationCreatedDuringValidation(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC))
	destination := filepath.Join(repo, "specs", "v0.3.0.md")
	readme := filepath.Join(repo, "specs", "README.md")
	readmeBefore := readBytes(t, readme)
	installLifecycleHook(t, func() {
		writeFile(t, destination, "# external spec\n")
	})

	_, err := svc.AddSpec("v0.3.0")
	if err == nil || MachineFailureFor(err).Code != MachineCodeWriteConflict {
		t.Fatalf("AddSpec with destination race = %v, want write_conflict", err)
	}
	if got := readBytes(t, destination); got != "# external spec\n" {
		t.Fatalf("AddSpec overwrote raced destination: %q", got)
	}
	if got := readBytes(t, readme); got != readmeBefore {
		t.Fatal("AddSpec updated README despite destination collision")
	}
	if _, err := os.Stat(destination); err != nil {
		t.Fatalf("destination disappeared: %v", err)
	}
}
