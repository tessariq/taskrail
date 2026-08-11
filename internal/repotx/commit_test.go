package repotx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tessariq/taskrail/internal/repolock"
)

// A1: a successful multi-file transaction publishes every candidate and reports
// exactly the bytes it wrote.
func TestCommitPublishesTheCompleteWriteSet(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/tasks/T-1.md", "original task")
	seed(t, repo, "specs/v0.5.0.md", "spec")
	lock := acquire(t, repo, ownerCapability())

	result, err := Commit(context.Background(), lock, Request{
		Command:      "complete",
		SelectedTask: "T-1",
		TaskFields:   []string{"status"},
		Consumed: []Path{{
			Kind:     Managed,
			Reported: "specs/v0.5.0.md",
			Physical: filepath.Join(repo.Root, "specs", "v0.5.0.md"),
		}},
		Published: []Candidate{
			managed(repo, "planning/tasks/T-1.md", "completed task"),
			managed(repo, "planning/STATE.md", "projected state"),
		},
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	if got, _ := readManaged(t, repo, "planning/tasks/T-1.md"); got != "completed task" {
		t.Fatalf("task bytes = %q, want the candidate", got)
	}
	if got, _ := readManaged(t, repo, "planning/STATE.md"); got != "projected state" {
		t.Fatalf("state bytes = %q, want the candidate", got)
	}
	if got, _ := readManaged(t, repo, "specs/v0.5.0.md"); got != "spec" {
		t.Fatalf("consumed spec bytes = %q, want them unchanged", got)
	}

	if len(result.Snapshots) != 3 {
		t.Fatalf("snapshots = %d, want one per consumed and published path", len(result.Snapshots))
	}
	task := snapshotFor(t, result.Snapshots, "planning/tasks/T-1.md")
	wantDigest(t, task.OriginalSHA256, "original task", "task original")
	wantDigest(t, task.CandidateSHA256, "completed task", "task candidate")
	wantDigest(t, task.CurrentSHA256, "completed task", "task current")

	state := snapshotFor(t, result.Snapshots, "planning/STATE.md")
	if state.OriginalSHA256 != nil {
		t.Fatalf("state original digest = %q, want absent", *state.OriginalSHA256)
	}
	wantDigest(t, state.CurrentSHA256, "projected state", "state current")

	spec := snapshotFor(t, result.Snapshots, "specs/v0.5.0.md")
	if spec.CandidateSHA256 != nil {
		t.Fatal("a consumed path reported a candidate digest")
	}
	wantDigest(t, spec.CurrentSHA256, "spec", "consumed current")
}

// A1: the complete candidate is validated before the first write, and the
// repository mutation lock is held while it happens.
func TestCommitValidatesTheCandidateBeforeTheFirstWrite(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/tasks/T-1.md", "original task")
	lock := acquire(t, repo, ownerCapability())

	var preview []Snapshot
	_, err := Commit(context.Background(), lock, Request{
		Command:      "complete",
		SelectedTask: "T-1",
		Published: []Candidate{
			managed(repo, "planning/tasks/T-1.md", "completed task"),
			managed(repo, "planning/STATE.md", "projected state"),
		},
		Validate: func(snapshots []Snapshot) error {
			preview = snapshots
			status, statusErr := repolock.Inspect(repo)
			if statusErr != nil || !status.Held {
				t.Fatalf("lock during validation: held=%v err=%v", status.Held, statusErr)
			}
			if got, _ := readManaged(t, repo, "planning/tasks/T-1.md"); got != "original task" {
				t.Fatalf("task bytes during validation = %q, want the original", got)
			}
			return errors.New("candidate ledger is invalid")
		},
	})

	txErr := txError(t, err)
	if txErr.Kind != KindValidation {
		t.Fatalf("kind = %q, want %q", txErr.Kind, KindValidation)
	}
	wantMachineCode(t, txErr, "validation_failed")
	if got, _ := readManaged(t, repo, "planning/tasks/T-1.md"); got != "original task" {
		t.Fatalf("task bytes = %q, want the original after validation refusal", got)
	}
	if _, exists := readManaged(t, repo, "planning/STATE.md"); exists {
		t.Fatal("a rejected candidate published state anyway")
	}
	wantDigest(t, snapshotFor(t, preview, "planning/STATE.md").CandidateSHA256, "projected state", "preview candidate")
}

// A1/A2: an external edit between the snapshot and the first write is a conflict
// that publishes nothing.
func TestCommitRefusesWhenTheSnapshotWentStale(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/tasks/T-1.md", "original task")
	lock := acquire(t, repo, ownerCapability())

	_, err := Commit(context.Background(), lock, Request{
		Command:      "complete",
		SelectedTask: "T-1",
		Published:    []Candidate{managed(repo, "planning/tasks/T-1.md", "completed task")},
		Validate: func([]Snapshot) error {
			seed(t, repo, "planning/tasks/T-1.md", "someone else's edit")
			return nil
		},
	})

	txErr := txError(t, err)
	if txErr.Kind != KindConflict {
		t.Fatalf("kind = %q, want %q", txErr.Kind, KindConflict)
	}
	wantMachineCode(t, txErr, "write_conflict")
	if got, _ := readManaged(t, repo, "planning/tasks/T-1.md"); got != "someone else's edit" {
		t.Fatalf("task bytes = %q, want the external edit preserved", got)
	}
	snapshot := snapshotFor(t, txErr.Snapshots(), "planning/tasks/T-1.md")
	wantDigest(t, snapshot.OriginalSHA256, "original task", "conflict original")
	wantDigest(t, snapshot.CandidateSHA256, "completed task", "conflict candidate")
	wantDigest(t, snapshot.CurrentSHA256, "someone else's edit", "conflict current")
}

// cancelAfterFirstPublish interrupts publication once one path has landed, which
// is the handled multi-file failure the rollback contract is written for.
func cancelAfterFirstPublish(t *testing.T, cancel context.CancelFunc, extra func()) {
	t.Helper()
	published := 0
	testHookAfterPublish = func(Path) {
		published++
		if published == 1 {
			if extra != nil {
				extra()
			}
			cancel()
		}
	}
	t.Cleanup(func() { testHookAfterPublish = nil })
}

// A2: a handled failure after the first write restores every published path.
func TestHandledFailureRollsBackPublishedPaths(t *testing.T) {
	for _, tc := range []struct {
		name    string
		seeded  bool
		content string
	}{
		{name: "existing path returns to its original bytes", seeded: true, content: "original task"},
		{name: "created path is removed again", seeded: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepository(t)
			if tc.seeded {
				seed(t, repo, "planning/tasks/T-1.md", tc.content)
			}
			lock := acquire(t, repo, ownerCapability())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cancelAfterFirstPublish(t, cancel, nil)

			_, err := Commit(ctx, lock, Request{
				Command:      "complete",
				SelectedTask: "T-1",
				Published: []Candidate{
					managed(repo, "planning/tasks/T-1.md", "completed task"),
					managed(repo, "planning/zz-state.md", "projected state"),
				},
			})

			txErr := txError(t, err)
			if txErr.Kind != KindRolledBack {
				t.Fatalf("kind = %q, want %q (preserved %v)", txErr.Kind, KindRolledBack, txErr.Preserved)
			}
			if code, registered := txErr.MachineCode(); registered {
				t.Fatalf("machine code = %q, want the caller to supply its cause's own code", code)
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error %v does not carry the cause", err)
			}
			got, exists := readManaged(t, repo, "planning/tasks/T-1.md")
			if exists != tc.seeded || got != tc.content {
				t.Fatalf("task after rollback = (%q, exists=%v), want (%q, exists=%v)",
					got, exists, tc.content, tc.seeded)
			}
			if _, exists := readManaged(t, repo, "planning/zz-state.md"); exists {
				t.Fatal("an unpublished candidate reached the repository")
			}
			snapshot := snapshotFor(t, txErr.Snapshots(), "planning/tasks/T-1.md")
			if tc.seeded {
				wantDigest(t, snapshot.CurrentSHA256, tc.content, "rolled-back current")
			} else if snapshot.CurrentSHA256 != nil {
				t.Fatalf("current digest = %q, want absent after removal", *snapshot.CurrentSHA256)
			}
		})
	}
}

// A2: rollback never overwrites bytes that are no longer this transaction's
// candidate; it preserves them and reports the rollback failure.
func TestRollbackPreservesExternallyChangedBytes(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/tasks/T-1.md", "original task")
	lock := acquire(t, repo, ownerCapability())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelAfterFirstPublish(t, cancel, func() {
		seed(t, repo, "planning/tasks/T-1.md", "someone else's edit")
	})

	_, err := Commit(ctx, lock, Request{
		Command:      "complete",
		SelectedTask: "T-1",
		Published: []Candidate{
			managed(repo, "planning/tasks/T-1.md", "completed task"),
			managed(repo, "planning/zz-state.md", "projected state"),
		},
	})

	txErr := txError(t, err)
	if txErr.Kind != KindRollbackFailed {
		t.Fatalf("kind = %q, want %q", txErr.Kind, KindRollbackFailed)
	}
	wantMachineCode(t, txErr, "rollback_failed")
	if len(txErr.Preserved) != 1 || txErr.Preserved[0] != "planning/tasks/T-1.md" {
		t.Fatalf("preserved = %v, want the externally changed path", txErr.Preserved)
	}
	if got, _ := readManaged(t, repo, "planning/tasks/T-1.md"); got != "someone else's edit" {
		t.Fatalf("task bytes = %q, want the external edit preserved", got)
	}
	snapshot := snapshotFor(t, txErr.Snapshots(), "planning/tasks/T-1.md")
	wantDigest(t, snapshot.OriginalSHA256, "original task", "preserved original")
	wantDigest(t, snapshot.CandidateSHA256, "completed task", "preserved candidate")
	wantDigest(t, snapshot.CurrentSHA256, "someone else's edit", "preserved current")
}

// A2: a published path the transaction can no longer read is reported as
// preserved, because an unread path is one whose original bytes nobody proved.
func TestRollbackReportsUnreadablePathsAsPreserved(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/tasks/T-1.md", "original task")
	lock := acquire(t, repo, ownerCapability())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	physical := filepath.Join(repo.Root, "planning", "tasks", "T-1.md")
	cancelAfterFirstPublish(t, cancel, func() {
		if err := os.Remove(physical); err != nil {
			t.Fatalf("replace published path: %v", err)
		}
		if err := os.Mkdir(physical, 0o755); err != nil {
			t.Fatalf("replace published path with a directory: %v", err)
		}
	})

	_, err := Commit(ctx, lock, Request{
		Command:      "complete",
		SelectedTask: "T-1",
		Published: []Candidate{
			managed(repo, "planning/tasks/T-1.md", "completed task"),
			managed(repo, "planning/zz-state.md", "projected state"),
		},
	})

	txErr := txError(t, err)
	if txErr.Kind != KindRollbackFailed {
		t.Fatalf("kind = %q, want %q", txErr.Kind, KindRollbackFailed)
	}
	if len(txErr.Preserved) != 1 || txErr.Preserved[0] != "planning/tasks/T-1.md" {
		t.Fatalf("preserved = %v, want the unreadable path", txErr.Preserved)
	}
	if info, statErr := os.Lstat(physical); statErr != nil || !info.IsDir() {
		t.Fatalf("rollback disturbed the replaced path: info=%v err=%v", info, statErr)
	}
	if current := snapshotFor(t, txErr.Snapshots(), "planning/tasks/T-1.md").CurrentSHA256; current != nil {
		t.Fatalf("current digest = %q, want none for a path the transaction could not read", *current)
	}
}

// A3: a path that becomes unreadable mid-transaction reports no current digest
// rather than the digest of whatever the previous phase saw there.
func TestUnreadablePathReportsNoCurrentDigest(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/tasks/T-1.md", "original task")
	lock := acquire(t, repo, ownerCapability())
	physical := filepath.Join(repo.Root, "planning", "tasks", "T-1.md")

	_, err := Commit(context.Background(), lock, Request{
		Command:      "complete",
		SelectedTask: "T-1",
		Published:    []Candidate{managed(repo, "planning/tasks/T-1.md", "completed task")},
		Validate: func([]Snapshot) error {
			if err := os.Remove(physical); err != nil {
				return err
			}
			return os.Mkdir(physical, 0o755)
		},
	})

	txErr := txError(t, err)
	if txErr.Kind != KindNotRegularFile {
		t.Fatalf("kind = %q, want %q", txErr.Kind, KindNotRegularFile)
	}
	snapshot := snapshotFor(t, txErr.Snapshots(), "planning/tasks/T-1.md")
	wantDigest(t, snapshot.OriginalSHA256, "original task", "unreadable original")
	if snapshot.CurrentSHA256 != nil {
		t.Fatalf("current digest = %q, want none once the path stopped being readable", *snapshot.CurrentSHA256)
	}
}

// A1/A2: a path edited after the whole-set recheck but before its own write is
// never overwritten; publication stops and what already landed rolls back.
func TestPublicationRefusesAnEditThatLandsMidTransaction(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/a.md", "original a")
	seed(t, repo, "planning/b.md", "original b")
	lock := acquire(t, repo, ownerCapability())
	testHookAfterPublish = func(Path) {
		seed(t, repo, "planning/b.md", "someone else's edit")
	}
	t.Cleanup(func() { testHookAfterPublish = nil })

	_, err := Commit(context.Background(), lock, Request{
		Command:      "complete",
		SelectedTask: "T-1",
		Published: []Candidate{
			managed(repo, "planning/a.md", "candidate a"),
			managed(repo, "planning/b.md", "candidate b"),
		},
	})

	txErr := txError(t, err)
	if txErr.Kind != KindConflict {
		t.Fatalf("kind = %q, want %q (preserved %v)", txErr.Kind, KindConflict, txErr.Preserved)
	}
	wantMachineCode(t, txErr, "write_conflict")
	if got, _ := readManaged(t, repo, "planning/b.md"); got != "someone else's edit" {
		t.Fatalf("b bytes = %q, want the external edit preserved", got)
	}
	if got, _ := readManaged(t, repo, "planning/a.md"); got != "original a" {
		t.Fatalf("a bytes = %q, want the original after rollback", got)
	}
	snapshot := snapshotFor(t, txErr.Snapshots(), "planning/b.md")
	wantDigest(t, snapshot.OriginalSHA256, "original b", "conflict original")
	wantDigest(t, snapshot.CandidateSHA256, "candidate b", "conflict candidate")
	wantDigest(t, snapshot.CurrentSHA256, "someone else's edit", "conflict current")
}

// A3: a path occupied by something the transaction cannot replace fails with the
// evidence it had already gathered, not with an empty report.
func TestUnusablePathFailsWithTheEvidenceCollectedSoFar(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/a.md", "original a")
	if err := os.MkdirAll(filepath.Join(repo.Root, "planning", "zz.md"), 0o755); err != nil {
		t.Fatalf("occupy a published path with a directory: %v", err)
	}
	lock := acquire(t, repo, ownerCapability())

	_, err := Commit(context.Background(), lock, Request{
		Command:      "complete",
		SelectedTask: "T-1",
		Published: []Candidate{
			managed(repo, "planning/a.md", "candidate a"),
			managed(repo, "planning/zz.md", "candidate zz"),
		},
	})

	txErr := txError(t, err)
	if txErr.Kind != KindNotRegularFile {
		t.Fatalf("kind = %q, want %q", txErr.Kind, KindNotRegularFile)
	}
	wantMachineCode(t, txErr, "repository_invalid")
	wantDigest(t, snapshotFor(t, txErr.Snapshots(), "planning/a.md").OriginalSHA256, "original a", "sibling original")
	if got, _ := readManaged(t, repo, "planning/a.md"); got != "original a" {
		t.Fatalf("a bytes = %q, want the original", got)
	}
}

// A4: a symlinked directory component cannot carry a publication outside the
// locked repository, even though the declared physical path looks contained, and
// the refusal lands before any directory is created out there.
func TestPublicationRefusesASymlinkedDirectoryEscape(t *testing.T) {
	for _, tc := range []struct {
		name     string
		link     string
		reported string
		escaped  string
	}{
		{
			name:     "the parent directory itself is a link",
			link:     "planning",
			reported: "planning/STATE.md",
			escaped:  "STATE.md",
		},
		{
			name:     "a component partway down is a link",
			link:     "planning",
			reported: "planning/reviews/STATE.md",
			escaped:  "reviews",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepository(t)
			outside, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatalf("resolve escape target: %v", err)
			}
			if err := os.Symlink(outside, filepath.Join(repo.Root, tc.link)); err != nil {
				t.Skipf("this platform does not allow creating symlinks: %v", err)
			}
			lock := acquire(t, repo, ownerCapability())

			_, err = Commit(context.Background(), lock, Request{
				Command:      "complete",
				SelectedTask: "T-1",
				Published:    []Candidate{managed(repo, tc.reported, "projected state")},
			})

			txErr := txError(t, err)
			if txErr.Kind != KindOutsideRepository {
				t.Fatalf("kind = %q, want %q", txErr.Kind, KindOutsideRepository)
			}
			wantMachineCode(t, txErr, "repository_invalid")
			if _, statErr := os.Stat(filepath.Join(outside, tc.escaped)); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("%s appeared outside the repository through a symlinked directory", tc.escaped)
			}
		})
	}
}

// A3: a path the transaction cannot observe at all is still a typed failure with
// evidence, not a bare filesystem error that carries no snapshot.
func TestUnobservablePathIsATypedFailure(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/a.md", "original a")
	loop := filepath.Join(repo.Root, "planning", "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Skipf("this platform does not allow creating symlinks: %v", err)
	}
	lock := acquire(t, repo, ownerCapability())

	_, err := Commit(context.Background(), lock, Request{
		Command:      "complete",
		SelectedTask: "T-1",
		Published: []Candidate{
			managed(repo, "planning/a.md", "candidate a"),
			managed(repo, "planning/loop/state.md", "projected state"),
		},
	})

	txErr := txError(t, err)
	if txErr.Kind != KindUnreadable {
		t.Fatalf("kind = %q, want %q", txErr.Kind, KindUnreadable)
	}
	wantMachineCode(t, txErr, "repository_invalid")
	wantDigest(t, snapshotFor(t, txErr.Snapshots(), "planning/a.md").OriginalSHA256, "original a", "sibling original")
	if got, _ := readManaged(t, repo, "planning/a.md"); got != "original a" {
		t.Fatalf("a bytes = %q, want the original", got)
	}
}

// A3: a file whose bytes cannot be read, even though it looks like an ordinary
// regular file, is a typed failure rather than a bare filesystem error.
func TestUnreadableFileIsATypedFailure(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/tasks/T-1.md", "original task")
	physical := filepath.Join(repo.Root, "planning", "tasks", "T-1.md")
	if err := os.Chmod(physical, 0o000); err != nil {
		t.Skipf("this platform does not support removing read permission: %v", err)
	}
	if _, err := os.ReadFile(physical); err == nil {
		t.Skip("this process can read a mode-0 file, so the failure is unreachable here")
	}
	lock := acquire(t, repo, ownerCapability())

	_, err := Commit(context.Background(), lock, Request{
		Command:      "complete",
		SelectedTask: "T-1",
		Published:    []Candidate{managed(repo, "planning/tasks/T-1.md", "completed task")},
	})

	txErr := txError(t, err)
	if txErr.Kind != KindUnreadable {
		t.Fatalf("kind = %q, want %q", txErr.Kind, KindUnreadable)
	}
	wantMachineCode(t, txErr, "repository_invalid")
	if snapshot := snapshotFor(t, txErr.Snapshots(), "planning/tasks/T-1.md"); snapshot.CurrentSHA256 != nil {
		t.Fatalf("current digest = %q, want none for an unreadable path", *snapshot.CurrentSHA256)
	}
}

// A3/A4: the containment proof classifies a directory it cannot resolve, so a
// link planted mid-transaction cannot turn into an unclassified error either.
func TestExistingAncestorClassifiesAnUnresolvableDirectory(t *testing.T) {
	root := t.TempDir()
	loop := filepath.Join(root, "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Skipf("this platform does not allow creating symlinks: %v", err)
	}

	_, err := existingAncestor(filepath.Join(loop, "reviews"))

	txErr := txError(t, err)
	if txErr.Kind != KindUnreadable {
		t.Fatalf("kind = %q, want %q", txErr.Kind, KindUnreadable)
	}
	wantMachineCode(t, txErr, "repository_invalid")
}

// A4: a link planted while the transaction is creating directories is caught by
// the second containment proof, after the first one saw only a safe ancestor.
func TestPublicationRefusesALinkPlantedDuringDirectoryCreation(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, "planning/keep.md", "keep")
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve escape target: %v", err)
	}
	planted := filepath.Join(repo.Root, "planning", "reviews")
	if err := os.Symlink(outside, planted); err != nil {
		t.Skipf("this platform does not allow creating symlinks: %v", err)
	}
	if err := os.Remove(planted); err != nil {
		t.Fatalf("clear the probe symlink: %v", err)
	}
	lock := acquire(t, repo, ownerCapability())
	testHookBeforeMkdir = func(Path) {
		if err := os.Symlink(outside, planted); err != nil && !errors.Is(err, os.ErrExist) {
			t.Errorf("plant the symlink: %v", err)
		}
	}
	t.Cleanup(func() { testHookBeforeMkdir = nil })

	_, err = Commit(context.Background(), lock, Request{
		Command:      "complete",
		SelectedTask: "T-1",
		Published:    []Candidate{managed(repo, "planning/reviews/state.md", "projected state")},
	})

	txErr := txError(t, err)
	if txErr.Kind != KindOutsideRepository {
		t.Fatalf("kind = %q, want %q", txErr.Kind, KindOutsideRepository)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "state.md")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("a publication escaped through a link planted during directory creation")
	}
}

// A3: mixed managed, worktree, and Git evidence is typed and deterministically
// ordered by path kind and then path.
func TestSnapshotsAreTypedAndDeterministic(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	gitPhysical := filepath.Join(repo.Root, ".git", "info", "exclude")

	result, err := Commit(context.Background(), lock, Request{
		Command:      "complete",
		SelectedTask: "T-1",
		Published: []Candidate{
			managed(repo, "planning/STATE.md", "state"),
			{
				Path: Path{
					Kind:     Worktree,
					Reported: ".agents/skills/taskrail/SKILL.md",
					Physical: filepath.Join(repo.Root, ".agents", "skills", "taskrail", "SKILL.md"),
				},
				Content: []byte("skill"),
			},
			{
				Path: Path{
					Kind:     Git,
					Reported: filepath.ToSlash(gitPhysical),
					Physical: gitPhysical,
				},
				Content: []byte("exclusion"),
			},
			managed(repo, "planning/tasks/T-1.md", "task"),
		},
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	want := []struct {
		kind PathKind
		path string
	}{
		{Git, filepath.ToSlash(gitPhysical)},
		{Managed, "planning/STATE.md"},
		{Managed, "planning/tasks/T-1.md"},
		{Worktree, ".agents/skills/taskrail/SKILL.md"},
	}
	if len(result.Snapshots) != len(want) {
		t.Fatalf("snapshots = %d, want %d", len(result.Snapshots), len(want))
	}
	for i, expected := range want {
		got := result.Snapshots[i]
		if got.Kind != expected.kind || got.Path != expected.path {
			t.Fatalf("snapshot %d = (%s, %s), want (%s, %s)", i, got.Kind, got.Path, expected.kind, expected.path)
		}
	}
}

// A4: a physical location outside the locked repository is refused before any
// path is read, whoever owns the lock.
func TestCommitRefusesPathsOutsideTheRepository(t *testing.T) {
	repo := newRepository(t)
	outside := filepath.Join(t.TempDir(), "escape.md")
	lock := acquire(t, repo, ownerCapability())

	_, err := Commit(context.Background(), lock, Request{
		Command:      "complete",
		SelectedTask: "T-1",
		Published: []Candidate{{
			Path:    Path{Kind: Managed, Reported: "planning/STATE.md", Physical: outside},
			Content: []byte("state"),
		}},
	})

	txErr := txError(t, err)
	if txErr.Kind != KindOutsideRepository {
		t.Fatalf("kind = %q, want %q", txErr.Kind, KindOutsideRepository)
	}
	wantMachineCode(t, txErr, "repository_invalid")
	if _, err := os.Stat(outside); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a refused transaction wrote outside the repository")
	}
}

func TestCommitRejectsRequestsNoWriterCouldHaveBuilt(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	state := managed(repo, "planning/STATE.md", "state")

	for _, tc := range []struct {
		name string
		req  Request
	}{
		{name: "no command", req: Request{Published: []Candidate{state}}},
		{name: "empty write set", req: Request{Command: "complete"}},
		{
			name: "duplicate reported path",
			req:  Request{Command: "complete", Published: []Candidate{state, managed(repo, "planning/STATE.md", "other")}},
		},
		{
			name: "two paths on one location",
			req: Request{Command: "complete", Published: []Candidate{state, {
				Path:    Path{Kind: Managed, Reported: "planning/other.md", Physical: state.Physical},
				Content: []byte("other"),
			}}},
		},
		{
			name: "non-canonical reported path",
			req: Request{Command: "complete", Published: []Candidate{{
				Path:    Path{Kind: Managed, Reported: "planning/../escape.md", Physical: state.Physical},
				Content: []byte("state"),
			}}},
		},
		{
			name: "relative physical location",
			req: Request{Command: "complete", Published: []Candidate{{
				Path:    Path{Kind: Managed, Reported: "planning/STATE.md", Physical: "planning/STATE.md"},
				Content: []byte("state"),
			}}},
		},
		{
			name: "unknown path kind",
			req: Request{Command: "complete", Published: []Candidate{{
				Path:    Path{Kind: "durable", Reported: "planning/STATE.md", Physical: state.Physical},
				Content: []byte("state"),
			}}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Commit(context.Background(), lock, tc.req); err == nil {
				t.Fatal("a malformed transaction request was accepted")
			}
			if _, exists := readManaged(t, repo, "planning/STATE.md"); exists {
				t.Fatal("a malformed transaction request published bytes")
			}
		})
	}
}
