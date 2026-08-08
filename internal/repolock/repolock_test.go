package repolock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// committedRepo builds an explicitly supplied committed-mode Git context. The
// primitives never discover a repository, so a plain temporary directory pair
// stands in for a worktree and its Git common directory (T-155 acceptance:
// storage discovery and command routing stay with T-222/T-223).
func committedRepo(t *testing.T) Repository {
	t.Helper()
	root := t.TempDir()
	common := filepath.Join(root, ".git")
	if err := os.MkdirAll(common, 0o755); err != nil {
		t.Fatalf("create git common dir: %v", err)
	}
	return Repository{Root: root, GitCommonDir: common, Mode: ModeCommitted}
}

func localRepo(t *testing.T) Repository {
	t.Helper()
	repo := committedRepo(t)
	repo.Mode = ModeLocal
	return repo
}

func nonGitRepo(t *testing.T) Repository {
	t.Helper()
	return Repository{Root: t.TempDir(), Mode: ModeCommitted}
}

func writerCapability() Capability {
	return Capability{Commands: []string{"complete"}, TaskFields: []string{"status", "updated_at"}}
}

// writerRequest is the ordinary committed-writer claim the acquisition tests
// vary one condition of at a time.
func writerRequest(repo Repository) Request {
	return Request{Repository: repo, Command: "complete", Capability: writerCapability()}
}

func acquire(t *testing.T, repo Repository) *Lock {
	t.Helper()
	lock, err := Acquire(context.Background(), writerRequest(repo))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })
	return lock
}

func readLockBytes(t *testing.T, repo Repository) []byte {
	t.Helper()
	data, err := os.ReadFile(LockPath(repo))
	if err != nil {
		t.Fatalf("read lock file: %v", err)
	}
	return data
}

func TestLockPathUsesGitCommonDirectorySoLinkedWorktreesCoordinate(t *testing.T) {
	repo := committedRepo(t)
	want := filepath.Join(repo.GitCommonDir, "taskrail", "mutation.lock")
	if got := LockPath(repo); got != want {
		t.Fatalf("lock path = %q, want %q", got, want)
	}
}

func TestLockPathUsesRootLocalRuntimeDirectoryOutsideGit(t *testing.T) {
	repo := nonGitRepo(t)
	want := filepath.Join(repo.Root, ".taskrail", "runtime", "mutation.lock")
	if got := LockPath(repo); got != want {
		t.Fatalf("lock path = %q, want %q", got, want)
	}
}

// Lock identity is a function of the supplied roots alone, so two callers in
// different working directories coordinate instead of taking two locks.
func TestLockIdentityDoesNotDependOnInvocationDirectory(t *testing.T) {
	repo := committedRepo(t)
	before := LockPath(repo)

	other := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(other); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	if after := LockPath(repo); after != before {
		t.Fatalf("lock path changed with the invocation directory: %q then %q", before, after)
	}
}

func TestAcquireRecordsTheNormativeOwnerMetadata(t *testing.T) {
	repo := committedRepo(t)
	lock, err := Acquire(context.Background(), Request{
		Repository:    repo,
		Command:       "complete",
		TransactionID: "tx-1",
		Capability:    writerCapability(),
	})
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })

	owner := lock.Owner()
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(owner.LockID) {
		t.Fatalf("lock_id %q is not a lower-case 32-hex id", owner.LockID)
	}
	if owner.Command != "complete" || owner.PID != os.Getpid() || owner.Host == "" {
		t.Fatalf("unexpected owner identity: %+v", owner)
	}
	if _, err := time.Parse(time.RFC3339, owner.StartedAt); err != nil {
		t.Fatalf("started_at %q is not RFC3339: %v", owner.StartedAt, err)
	}
	if owner.RepositoryRoot != repo.Root || owner.StorageMode != ModeCommitted || owner.StorageRoot != repo.Root {
		t.Fatalf("unexpected repository identity: %+v", owner)
	}
	if owner.TransactionID == nil || *owner.TransactionID != "tx-1" {
		t.Fatalf("transaction_id = %v, want tx-1", owner.TransactionID)
	}
	// Not delegated: the executable digest and the delegation digest stay null.
	if owner.ExecutableSHA256 != nil || owner.DelegationDigest != nil {
		t.Fatalf("undelegated lock recorded delegation metadata: %+v", owner)
	}

	var raw map[string]any
	if err := json.Unmarshal(readLockBytes(t, repo), &raw); err != nil {
		t.Fatalf("unmarshal lock file: %v", err)
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	want := []string{
		"command", "delegation_digest", "executable_sha256", "host", "lock_id",
		"pid", "repository_root", "started_at", "storage_mode", "storage_root", "transaction_id",
	}
	if !slices.Equal(keys, want) {
		t.Fatalf("lock metadata keys = %v, want %v", keys, want)
	}
}

// Local mode keeps the same logical repository identity as committed mode while
// naming its own storage root, so a later join can tell the two apart.
func TestLocalModeKeepsLogicalIdentityAndExplicitStorageRoot(t *testing.T) {
	repo := localRepo(t)
	lock := acquire(t, repo)

	owner := lock.Owner()
	if owner.RepositoryRoot != repo.Root {
		t.Fatalf("repository_root = %q, want the logical root %q", owner.RepositoryRoot, repo.Root)
	}
	if want := filepath.Join(repo.Root, ".taskrail", "local"); owner.StorageRoot != want {
		t.Fatalf("storage_root = %q, want %q", owner.StorageRoot, want)
	}
	if owner.StorageMode != ModeLocal {
		t.Fatalf("storage_mode = %q, want local", owner.StorageMode)
	}
}

func TestAcquireRejectsInvalidRepositoryContexts(t *testing.T) {
	tests := []struct {
		name string
		repo Repository
	}{
		{"empty root", Repository{Mode: ModeCommitted}},
		{"relative root", Repository{Root: "relative", Mode: ModeCommitted}},
		{"relative git common dir", Repository{Root: t.TempDir(), GitCommonDir: "relative", Mode: ModeCommitted}},
		{"unknown mode", Repository{Root: t.TempDir(), Mode: StorageMode("stealth")}},
		// Local mode is unsupported outside a non-bare worktree, so a local
		// context without a Git common directory is not a repository at all.
		{"local mode outside git", Repository{Root: t.TempDir(), Mode: ModeLocal}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Acquire(context.Background(), Request{
				Repository: test.repo,
				Command:    "complete",
				Capability: writerCapability(),
			}); err == nil {
				t.Fatal("expected a refusal")
			}
		})
	}
}

func TestAcquireRejectsACommandOutsideItsOwnCapability(t *testing.T) {
	_, err := Acquire(context.Background(), Request{
		Repository: committedRepo(t),
		Command:    "task new",
		Capability: writerCapability(),
	})
	if err == nil {
		t.Fatal("expected a refusal for a command the capability does not allow")
	}
}

func TestSecondAcquireInTheSameProcessRefusesWithoutMutation(t *testing.T) {
	repo := committedRepo(t)
	acquire(t, repo)
	before := readLockBytes(t, repo)

	_, err := Acquire(context.Background(), Request{
		Repository: repo,
		Command:    "start",
		Capability: Capability{Commands: []string{"start"}, TaskFields: []string{"status"}},
	})
	if !errors.Is(err, ErrSameProcess) {
		t.Fatalf("second same-process acquire error = %v, want ErrSameProcess", err)
	}
	if got := readLockBytes(t, repo); !slices.Equal(got, before) {
		t.Fatal("a refused acquire rewrote the lock file")
	}
}

func TestAcquireRefusesAnAbandonedLockAndNeverClearsIt(t *testing.T) {
	repo := committedRepo(t)
	// A metadata record left behind by a dead owner: the PID cannot be alive and
	// the start time is long past. Neither signal is a distributed lease, so the
	// lock must survive untouched for `lock clear` (T-231) to inspect.
	abandoned := writeRawLock(t, repo, Owner{
		LockID:         strings.Repeat("a", 32),
		Command:        "complete",
		PID:            1 << 30,
		Host:           "long-gone-host",
		StartedAt:      "2000-01-01T00:00:00Z",
		RepositoryRoot: repo.Root,
		StorageMode:    ModeCommitted,
		StorageRoot:    repo.Root,
	})

	if _, err := Acquire(context.Background(), writerRequest(repo)); !errors.Is(err, ErrHeld) {
		t.Fatalf("acquire error = %v, want ErrHeld", err)
	}
	if got := readLockBytes(t, repo); !slices.Equal(got, abandoned) {
		t.Fatal("acquire auto-cleared an abandoned lock")
	}
}

func TestAcquireAndInspectRefuseMalformedMetadataWithoutClearingIt(t *testing.T) {
	repo := committedRepo(t)
	if err := os.MkdirAll(filepath.Dir(LockPath(repo)), 0o755); err != nil {
		t.Fatalf("create lock root: %v", err)
	}
	malformed := []byte("{not json")
	if err := os.WriteFile(LockPath(repo), malformed, 0o644); err != nil {
		t.Fatalf("write malformed lock: %v", err)
	}

	_, err := Acquire(context.Background(), writerRequest(repo))
	if !errors.Is(err, ErrHeld) || !errors.Is(err, ErrMalformed) {
		t.Fatalf("acquire error = %v, want both ErrHeld and ErrMalformed", err)
	}
	if _, err := Inspect(repo); !errors.Is(err, ErrMalformed) {
		t.Fatalf("inspect error = %v, want ErrMalformed", err)
	}
	if got := readLockBytes(t, repo); !slices.Equal(got, malformed) {
		t.Fatal("malformed metadata was rewritten or cleared")
	}
}

func TestAcquireAndInspectRejectSemanticallyInvalidMetadata(t *testing.T) {
	repo := committedRepo(t)
	// Well-formed JSON carrying an impossible lock id: still malformed, and still
	// never cleared, so a corrupt record cannot be laundered into an acquisition.
	raw := writeRawLock(t, repo, Owner{
		LockID:         "NOT-HEX",
		Command:        "complete",
		PID:            os.Getpid(),
		Host:           "host",
		StartedAt:      time.Now().UTC().Format(time.RFC3339),
		RepositoryRoot: repo.Root,
		StorageMode:    ModeCommitted,
		StorageRoot:    repo.Root,
	})
	if _, err := Inspect(repo); !errors.Is(err, ErrMalformed) {
		t.Fatalf("inspect error = %v, want ErrMalformed", err)
	}
	_, err := Acquire(context.Background(), writerRequest(repo))
	if !errors.Is(err, ErrHeld) || !errors.Is(err, ErrMalformed) {
		t.Fatalf("acquire error = %v, want both ErrHeld and ErrMalformed", err)
	}
	if got := readLockBytes(t, repo); !slices.Equal(got, raw) {
		t.Fatal("invalid metadata was rewritten")
	}
}

func TestAcquireHonoursACancelledContextAndLeavesNoLock(t *testing.T) {
	repo := committedRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Acquire(ctx, writerRequest(repo))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("acquire error = %v, want context.Canceled", err)
	}
	if _, err := os.Stat(LockPath(repo)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("an interrupted acquisition left a lock file: %v", err)
	}
	// The interruption must not poison the in-process registry either.
	acquire(t, repo)
}

// lateCancel reports no cancellation the first time it is asked and cancellation
// afterwards, which lands an interruption exactly between the lock's publication
// and the post-publication check that withdraws it.
type lateCancel struct {
	context.Context
	asked bool
}

func (c *lateCancel) Err() error {
	if !c.asked {
		c.asked = true
		return nil
	}
	return context.Canceled
}

func TestAcquireWithdrawsALockWhenCancellationRacesPublication(t *testing.T) {
	repo := committedRepo(t)

	_, err := Acquire(&lateCancel{Context: context.Background()}, writerRequest(repo))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("acquire error = %v, want context.Canceled", err)
	}
	if _, err := os.Stat(LockPath(repo)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a cancellation racing publication left a lock file: %v", err)
	}
	// The withdrawn claim must also leave the in-process registry clean.
	acquire(t, repo)
}

func TestReleaseKeepsTheHandleLiveWhenTheDeleteFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions do not gate deletion on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores the directory permissions this test relies on")
	}
	repo := committedRepo(t)
	lock, err := Acquire(context.Background(), writerRequest(repo))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	dir := filepath.Dir(LockPath(repo))
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("seal lock directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	if err := lock.Release(); err == nil {
		t.Fatal("expected release to report the failed delete")
	}
	// The lock is still ours and still on disk, so the handle stays usable and
	// this process must still be told it is the holder.
	if err := lock.Authorize("complete", "status"); err != nil {
		t.Fatalf("handle was spent by a failed delete: %v", err)
	}
	if _, err := Acquire(context.Background(), writerRequest(repo)); !errors.Is(err, ErrSameProcess) {
		t.Fatalf("re-acquire error = %v, want ErrSameProcess", err)
	}

	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("unseal lock directory: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("retry release: %v", err)
	}
}

func TestReleaseIsCompareAndDeleteAndIsNotRepeatable(t *testing.T) {
	repo := committedRepo(t)
	lock, err := Acquire(context.Background(), writerRequest(repo))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := os.Stat(LockPath(repo)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("release left the lock file: %v", err)
	}
	if err := lock.Release(); !errors.Is(err, ErrReleased) {
		t.Fatalf("second release error = %v, want ErrReleased", err)
	}
	if _, err := lock.Delegation(); !errors.Is(err, ErrReleased) {
		t.Fatalf("delegation after release error = %v, want ErrReleased", err)
	}
	if err := lock.Authorize("complete", "status"); !errors.Is(err, ErrReleased) {
		t.Fatalf("authorize after release error = %v, want ErrReleased", err)
	}
}

func TestReleaseRefusesToDeleteAReplacedLock(t *testing.T) {
	repo := committedRepo(t)
	lock, err := Acquire(context.Background(), writerRequest(repo))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	replacement := writeRawLock(t, repo, Owner{
		LockID:         strings.Repeat("b", 32),
		Command:        "start",
		PID:            os.Getpid(),
		Host:           "other",
		StartedAt:      time.Now().UTC().Format(time.RFC3339),
		RepositoryRoot: repo.Root,
		StorageMode:    ModeCommitted,
		StorageRoot:    repo.Root,
	})

	if err := lock.Release(); err == nil {
		t.Fatal("expected release to refuse a lock it no longer owns")
	}
	if got := readLockBytes(t, repo); !slices.Equal(got, replacement) {
		t.Fatal("release deleted a lock owned by someone else")
	}
}

// Publication links a private staging file into place, and the two names share
// one permission set — so the staging file's mode is the published lock's mode,
// and an owner-only lock would be unreadable to another user inspecting it.
func TestPublishedLockIsReadableBeyondItsOwner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not model the Unix permission bits this pins")
	}
	repo := committedRepo(t)
	acquire(t, repo)

	info, err := os.Stat(LockPath(repo))
	if err != nil {
		t.Fatalf("stat lock file: %v", err)
	}
	if got := info.Mode().Perm(); got != lockFileMode {
		t.Fatalf("lock file mode = %04o, want %04o", got, lockFileMode)
	}
}

func TestInspectIsReadOnly(t *testing.T) {
	repo := committedRepo(t)
	status, err := Inspect(repo)
	if err != nil {
		t.Fatalf("inspect absent lock: %v", err)
	}
	if status.Held || status.SHA256 != "" || status.Owner != nil {
		t.Fatalf("absent lock reported as %+v", status)
	}
	// A read-only caller needs no lock, so inspection creates nothing at all.
	if _, err := os.Stat(filepath.Dir(LockPath(repo))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inspect created the lock root: %v", err)
	}

	lock := acquire(t, repo)
	status, err = Inspect(repo)
	if err != nil {
		t.Fatalf("inspect held lock: %v", err)
	}
	digest := sha256.Sum256(readLockBytes(t, repo))
	if !status.Held || status.SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("held lock reported as %+v", status)
	}
	if status.Owner == nil || status.Owner.LockID != lock.Owner().LockID {
		t.Fatalf("inspect reported owner %+v", status.Owner)
	}
}

func TestCapabilityNarrowsButNeverWidens(t *testing.T) {
	base := Capability{Commands: []string{"complete", "verify"}, TaskFields: []string{"status", "updated_at"}}

	narrower, err := base.Narrow(Capability{Commands: []string{"verify"}, TaskFields: []string{"status"}})
	if err != nil {
		t.Fatalf("narrow: %v", err)
	}
	if err := narrower.Allows("complete", "status"); err == nil {
		t.Fatal("narrowed capability still allows a dropped command")
	}
	if err := narrower.Allows("verify", "status"); err != nil {
		t.Fatalf("narrowed capability rejects its own command: %v", err)
	}

	for _, wider := range []Capability{
		{Commands: []string{"complete", "task new"}, TaskFields: []string{"status"}},
		{Commands: []string{"complete"}, TaskFields: []string{"status", "spec_ref"}},
	} {
		if _, err := base.Narrow(wider); err == nil {
			t.Fatalf("narrow accepted a widening capability: %+v", wider)
		}
	}
}

func TestAuthorizeRefusesUnsupportedCommandsAndFields(t *testing.T) {
	lock := acquire(t, committedRepo(t))
	if err := lock.Authorize("complete", "status", "updated_at"); err != nil {
		t.Fatalf("authorize in-capability write: %v", err)
	}
	if err := lock.Authorize("task new", "status"); err == nil {
		t.Fatal("authorize accepted an unsupported command")
	}
	if err := lock.Authorize("complete", "spec_ref"); err == nil {
		t.Fatal("authorize accepted an unrelated task field")
	}
}

// writeRawLock installs owner as the lock file's bytes and returns them, so a
// test can assert afterwards that a refusal left the record untouched.
func writeRawLock(t *testing.T, repo Repository, owner Owner) []byte {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(LockPath(repo)), 0o755); err != nil {
		t.Fatalf("create lock root: %v", err)
	}
	data, err := json.Marshal(owner)
	if err != nil {
		t.Fatalf("marshal owner: %v", err)
	}
	if err := os.WriteFile(LockPath(repo), data, 0o644); err != nil {
		t.Fatalf("write lock file: %v", err)
	}
	return data
}
