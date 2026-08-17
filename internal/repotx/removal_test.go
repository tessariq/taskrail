package repotx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tessariq/taskrail/internal/repolock"
)

// A removal candidate publishes a path's absence. Rename-shaped transactions
// pair one with the creation of the successor path, so removal has to satisfy
// the same snapshot, recheck, and rollback contracts a byte candidate does.

// renameCapability is the direct-owner bound a `task rename` writer acquires.
func renameCapability() repolock.Capability {
	return repolock.Capability{
		Commands:   []string{"task rename"},
		TaskFields: []string{"id", "updated_at", "dependencies"},
	}
}

// acquireRename takes the lock a rename writer holds for these tests.
func acquireRename(t *testing.T, repo repolock.Repository) *repolock.Lock {
	t.Helper()
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: repo,
		Command:    "task rename",
		Capability: renameCapability(),
	})
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })
	return lock
}

// removal is one published absence: the reported path inside repo.
func removal(repo repolock.Repository, reported string) Candidate {
	return Candidate{
		Path: Path{
			Kind:     Managed,
			Reported: reported,
			Physical: filepath.Join(repo.Root, filepath.FromSlash(reported)),
		},
		Remove: true,
	}
}

// A rename-shaped transaction — remove the old path, publish the successor —
// lands both, and the committed evidence reports the removal as a nil
// candidate digest beside the original it replaced.
func TestCommitPublishesRemovalWithSuccessor(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/tasks/T-1.md", "original task")
	lock := acquireRename(t, repo)

	result, err := Commit(context.Background(), lock, Request{
		Command:      "task rename",
		SelectedTask: "T-1",
		Published: []Candidate{
			removal(repo, "planning/tasks/T-1.md"),
			managed(repo, "planning/tasks/T-1-base.md", "renamed task"),
		},
	})
	if err != nil {
		t.Fatalf("commit rename-shaped transaction: %v", err)
	}
	if _, exists := readManaged(t, repo, "planning/tasks/T-1.md"); exists {
		t.Fatal("removed path still present after commit")
	}
	if got, _ := readManaged(t, repo, "planning/tasks/T-1-base.md"); got != "renamed task" {
		t.Fatalf("successor bytes = %q, want %q", got, "renamed task")
	}

	snapshot := snapshotFor(t, result.Snapshots, "planning/tasks/T-1.md")
	wantDigest(t, snapshot.OriginalSHA256, "original task", "removal original")
	if snapshot.CandidateSHA256 != nil {
		t.Fatalf("removal candidate digest = %q, want absent: the candidate is the path's absence", *snapshot.CandidateSHA256)
	}
	if snapshot.CurrentSHA256 != nil {
		t.Fatalf("removal current digest = %q, want absent after removal", *snapshot.CurrentSHA256)
	}
}

// Removing a path that never existed is a no-op publication, mirroring
// rollback's leniency for an absent path: the transaction still commits and
// the evidence reports absence on every side.
func TestCommitRemovalOfAbsentPathIsANoOp(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/zz-state.md", "projected state")
	lock := acquireRename(t, repo)

	result, err := Commit(context.Background(), lock, Request{
		Command: "task rename",
		Published: []Candidate{
			removal(repo, "planning/tasks/T-1.md"),
			managed(repo, "planning/zz-state.md", "projected state"),
		},
	})
	if err != nil {
		t.Fatalf("commit absent-path removal: %v", err)
	}
	snapshot := snapshotFor(t, result.Snapshots, "planning/tasks/T-1.md")
	if snapshot.OriginalSHA256 != nil || snapshot.CandidateSHA256 != nil || snapshot.CurrentSHA256 != nil {
		t.Fatalf("absent removal evidence = %+v, want absence on every side", snapshot)
	}
}

// A handled failure after a removal landed rolls the removed file back to its
// snapshot bytes, so a rename cannot lose the task file it was moving.
func TestRemovalRollsBackToOriginalBytes(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/tasks/T-1.md", "original task")
	lock := acquireRename(t, repo)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelAfterFirstPublish(t, cancel, nil)

	_, err := Commit(ctx, lock, Request{
		Command: "task rename",
		Published: []Candidate{
			removal(repo, "planning/tasks/T-1.md"),
			managed(repo, "planning/zz-state.md", "projected state"),
		},
	})

	txErr := txError(t, err)
	if txErr.Kind != KindRolledBack {
		t.Fatalf("kind = %q, want %q (preserved %v)", txErr.Kind, KindRolledBack, txErr.Preserved)
	}
	if got, _ := readManaged(t, repo, "planning/tasks/T-1.md"); got != "original task" {
		t.Fatalf("removed path not restored on rollback: %q", got)
	}
	if _, exists := readManaged(t, repo, "planning/zz-state.md"); exists {
		t.Fatal("an unpublished candidate reached the repository")
	}
	snapshot := snapshotFor(t, txErr.Snapshots(), "planning/tasks/T-1.md")
	wantDigest(t, snapshot.CurrentSHA256, "original task", "restored current")
}

// A removed path that someone recreates before rollback belongs to whoever
// wrote it: the foreign bytes are preserved and the removal is reported as a
// rollback failure rather than clobbered.
func TestRollbackPreservesExternallyRecreatedRemoval(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("recreation check needs an unwritable directory to be meaningful as root")
	}
	repo := newRepository(t)
	seed(t, repo, "planning/tasks/T-1.md", "original task")
	// A read-only parent keeps the transaction's own restore from succeeding
	// for the wrong reason silently; the recreated foreign bytes below are the
	// bytes rollback must refuse to touch.
	lock := acquireRename(t, repo)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelAfterFirstPublish(t, cancel, func() {
		seed(t, repo, "planning/tasks/T-1.md", "someone else's task")
	})

	_, err := Commit(ctx, lock, Request{
		Command: "task rename",
		Published: []Candidate{
			removal(repo, "planning/tasks/T-1.md"),
			managed(repo, "planning/zz-state.md", "projected state"),
		},
	})

	txErr := txError(t, err)
	if txErr.Kind != KindRollbackFailed {
		t.Fatalf("kind = %q, want %q", txErr.Kind, KindRollbackFailed)
	}
	wantMachineCode(t, txErr, "rollback_failed")
	if len(txErr.Preserved) != 1 || txErr.Preserved[0] != "planning/tasks/T-1.md" {
		t.Fatalf("preserved = %v, want the recreated removal path", txErr.Preserved)
	}
	if got, _ := readManaged(t, repo, "planning/tasks/T-1.md"); got != "someone else's task" {
		t.Fatalf("recreated path not preserved: %q", got)
	}
}

// An edit to a to-be-removed path between the snapshot and publication is a
// conflict, never an overwriting removal.
func TestRemovalRefusesAnEditThatLandsMidTransaction(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/tasks/T-1.md", "original task")
	seed(t, repo, "planning/zz-state.md", "projected state")
	lock := acquireRename(t, repo)
	testHookAfterPublish = func(Path) {
		seed(t, repo, "planning/tasks/T-1.md", "someone else's edit")
	}
	t.Cleanup(func() { testHookAfterPublish = nil })

	_, err := Commit(context.Background(), lock, Request{
		Command: "task rename",
		Published: []Candidate{
			managed(repo, "planning/aa-first.md", "first"),
			removal(repo, "planning/tasks/T-1.md"),
		},
	})

	txErr := txError(t, err)
	if txErr.Kind != KindConflict {
		t.Fatalf("kind = %q, want %q", txErr.Kind, KindConflict)
	}
	wantMachineCode(t, txErr, "write_conflict")
	if got, _ := readManaged(t, repo, "planning/tasks/T-1.md"); got != "someone else's edit" {
		t.Fatalf("external edit not preserved: %q", got)
	}
	if _, exists := readManaged(t, repo, "planning/aa-first.md"); exists {
		t.Fatal("a path published before the conflict was not rolled back")
	}
}

// A removal candidate that also carries bytes is a defect no writer could
// have built: it is an ordinary error, not a transaction outcome.
func TestCommitRejectsRemovalWithContent(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/tasks/T-1.md", "original task")
	lock := acquireRename(t, repo)

	both := removal(repo, "planning/tasks/T-1.md")
	both.Content = []byte("conflicting bytes")
	_, err := Commit(context.Background(), lock, Request{
		Command: "task rename",
		Published: []Candidate{
			both,
			managed(repo, "planning/zz-state.md", "projected state"),
		},
	})
	if err == nil {
		t.Fatal("expected rejection")
	}
	var txErr *Error
	if errors.As(err, &txErr) {
		t.Fatalf("a malformed request must not surface as a transaction failure: %v", err)
	}
	if got, _ := readManaged(t, repo, "planning/tasks/T-1.md"); got != "original task" {
		t.Fatalf("rejected request changed repository bytes: %q", got)
	}
}
