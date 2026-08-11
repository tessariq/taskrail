package repotx

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tessariq/taskrail/internal/repolock"
)

// delegatedWrites is the exact write set a loop child is granted for the one
// task its parent selected.
func delegatedWrites() []string {
	return []string{"planning/STATE.md", "planning/tasks/T-1.md"}
}

func childCapability() repolock.Capability {
	return repolock.Capability{
		Commands:     []string{"complete"},
		TaskFields:   []string{"status", "updated_at"},
		SelectedTask: "T-1",
		Writes:       delegatedWrites(),
	}
}

func delegatingLock(t *testing.T, repo repolock.Repository) repolock.Delegation {
	t.Helper()
	executable := filepath.Join(t.TempDir(), "taskrail")
	if err := os.WriteFile(executable, []byte("taskrail-bytes"), 0o755); err != nil {
		t.Fatalf("stage executable: %v", err)
	}
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository:     repo,
		Command:        "loop",
		Capability:     repolock.Capability{Commands: []string{"loop"}},
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
	return delegation
}

func join(t *testing.T, repo repolock.Repository, capability repolock.Capability) *repolock.Joined {
	t.Helper()
	delegation := delegatingLock(t, repo)
	joined, err := repolock.Join(repolock.JoinRequest{
		Repository:       repo,
		Command:          "complete",
		Token:            delegation.Token,
		ExecutableSHA256: delegation.ExecutableSHA256,
		Capability:       capability,
	})
	if err != nil {
		t.Fatalf("join as delegate: %v", err)
	}
	return joined
}

func delegatedRequest(repo repolock.Repository) Request {
	return Request{
		Command:      "complete",
		SelectedTask: "T-1",
		TaskFields:   []string{"status"},
		Published: []Candidate{
			managed(repo, "planning/tasks/T-1.md", "completed task"),
			managed(repo, "planning/STATE.md", "projected state"),
		},
	}
}

// A4: permitted narrowed delegated work publishes normally.
func TestDelegatedWriteWithinItsBoundSucceeds(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/tasks/T-1.md", "original task")
	joined := join(t, repo, childCapability())

	if _, err := Commit(context.Background(), joined, delegatedRequest(repo)); err != nil {
		t.Fatalf("delegated commit: %v", err)
	}
	if got, _ := readManaged(t, repo, "planning/tasks/T-1.md"); got != "completed task" {
		t.Fatalf("task bytes = %q, want the candidate", got)
	}
}

// A4: every bound dimension refuses a widening attempt before any mutation.
func TestDelegatedWideningRefusesBeforeMutation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		capability repolock.Capability
		mutate     func(*Request, repolock.Repository)
	}{
		{
			name:       "another command",
			capability: childCapability(),
			mutate:     func(r *Request, _ repolock.Repository) { r.Command = "task release" },
		},
		{
			name:       "another task field",
			capability: childCapability(),
			mutate:     func(r *Request, _ repolock.Repository) { r.TaskFields = []string{"status", "loop_policy"} },
		},
		{
			name:       "another selected task",
			capability: childCapability(),
			mutate:     func(r *Request, _ repolock.Repository) { r.SelectedTask = "T-2" },
		},
		{
			name:       "a path outside the write set",
			capability: childCapability(),
			mutate: func(r *Request, repo repolock.Repository) {
				r.Published = append(r.Published, managed(repo, "planning/tasks/T-2.md", "other task"))
			},
		},
		{
			name: "no selected task at all",
			capability: repolock.Capability{
				Commands:   []string{"complete"},
				TaskFields: []string{"status"},
				Writes:     delegatedWrites(),
			},
		},
		{
			name: "no write set at all",
			capability: repolock.Capability{
				Commands:     []string{"complete"},
				TaskFields:   []string{"status"},
				SelectedTask: "T-1",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepository(t)
			seed(t, repo, "planning/tasks/T-1.md", "original task")
			joined := join(t, repo, tc.capability)
			req := delegatedRequest(repo)
			if tc.mutate != nil {
				tc.mutate(&req, repo)
			}

			_, err := Commit(context.Background(), joined, req)

			txErr := txError(t, err)
			if txErr.Kind != KindRefused {
				t.Fatalf("kind = %q, want %q", txErr.Kind, KindRefused)
			}
			wantMachineCode(t, txErr, "delegated_write_refused")
			if len(txErr.Snapshots()) != 0 {
				t.Fatalf("a pre-mutation refusal reported %d snapshots", len(txErr.Snapshots()))
			}
			if got, _ := readManaged(t, repo, "planning/tasks/T-1.md"); got != "original task" {
				t.Fatalf("task bytes = %q, want the original after refusal", got)
			}
			if _, exists := readManaged(t, repo, "planning/STATE.md"); exists {
				t.Fatal("a refused delegated write published state")
			}
		})
	}
}

// A4: nesting delegated work only ever narrows it further.
func TestNestedDelegationCannotWiden(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/tasks/T-1.md", "original task")
	joined := join(t, repo, childCapability())

	for _, tc := range []struct {
		name      string
		requested repolock.Capability
	}{
		{
			name: "another selected task",
			requested: repolock.Capability{
				Commands: []string{"complete"}, TaskFields: []string{"status"},
				SelectedTask: "T-2", Writes: delegatedWrites(),
			},
		},
		{
			name: "an added write path",
			requested: repolock.Capability{
				Commands: []string{"complete"}, TaskFields: []string{"status"},
				SelectedTask: "T-1", Writes: append(delegatedWrites(), "planning/tasks/T-2.md"),
			},
		},
		{
			name: "an unbounded write set",
			requested: repolock.Capability{
				Commands: []string{"complete"}, TaskFields: []string{"status"},
				SelectedTask: "T-1",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := joined.Narrow(tc.requested); err == nil {
				t.Fatal("nesting widened a delegated capability")
			}
		})
	}

	nested, err := joined.Narrow(repolock.Capability{
		Commands: []string{"complete"}, TaskFields: []string{"status"},
		SelectedTask: "T-1", Writes: []string{"planning/tasks/T-1.md"},
	})
	if err != nil {
		t.Fatalf("narrow to a subset: %v", err)
	}
	_, err = Commit(context.Background(), nested, delegatedRequest(repo))
	txErr := txError(t, err)
	if txErr.Kind != KindRefused {
		t.Fatalf("kind = %q, want %q for a path the nested bound dropped", txErr.Kind, KindRefused)
	}
	if got, _ := readManaged(t, repo, "planning/tasks/T-1.md"); got != "original task" {
		t.Fatalf("task bytes = %q, want the original after refusal", got)
	}
}

// A4: repository, storage, and executable identity keep binding a delegate, so a
// transaction can never start from a join across any of those boundaries.
func TestDelegatedJoinStaysBoundToItsRepositoryAndExecutable(t *testing.T) {
	repo := newRepository(t)
	delegation := delegatingLock(t, repo)
	other := newRepository(t)

	for _, tc := range []struct {
		name string
		req  repolock.JoinRequest
	}{
		{
			name: "another repository",
			req: repolock.JoinRequest{
				Repository: other, Command: "complete", Token: delegation.Token,
				ExecutableSHA256: delegation.ExecutableSHA256, Capability: childCapability(),
			},
		},
		{
			name: "another storage mode",
			req: repolock.JoinRequest{
				Repository: repolock.Repository{
					Root: repo.Root, GitCommonDir: filepath.Join(repo.Root, ".git"), Mode: repolock.ModeLocal,
				},
				Command: "complete", Token: delegation.Token,
				ExecutableSHA256: delegation.ExecutableSHA256, Capability: childCapability(),
			},
		},
		{
			name: "another executable",
			req: repolock.JoinRequest{
				Repository: repo, Command: "complete", Token: delegation.Token,
				ExecutableSHA256: digestOf([]byte("other-binary")), Capability: childCapability(),
			},
		},
		{
			name: "another token",
			req: repolock.JoinRequest{
				Repository: repo, Command: "complete", Token: "not-the-token",
				ExecutableSHA256: delegation.ExecutableSHA256, Capability: childCapability(),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := repolock.Join(tc.req); err == nil {
				t.Fatal("a delegate joined across a bound it does not match")
			}
		})
	}
}
