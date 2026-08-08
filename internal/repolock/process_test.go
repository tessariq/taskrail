package repolock

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The mutation lock coordinates separate processes, not goroutines, so the
// contention tests re-exec this test binary as a real second process
// (specs/v0.5.0.md#repository-discovery-locking-and-recovery). That observes the
// native filesystem lock rather than asserting on scheduler timing.
const (
	helperModeEnv        = "REPOLOCK_HELPER_MODE"
	helperRootEnv        = "REPOLOCK_HELPER_ROOT"
	helperGitCommonEnv   = "REPOLOCK_HELPER_GIT_COMMON_DIR"
	helperStorageModeEnv = "REPOLOCK_HELPER_STORAGE_MODE"
	helperTokenEnv       = "REPOLOCK_HELPER_TOKEN"
	helperExecDigestEnv  = "REPOLOCK_HELPER_EXECUTABLE_SHA256"
)

// TestRepolockHelperProcess is not a test: it is the entry point the contention
// tests re-exec. It runs only when the mode variable is set, prints one line of
// evidence to stdout, and always exits zero so the parent asserts on that line.
func TestRepolockHelperProcess(t *testing.T) {
	mode := os.Getenv(helperModeEnv)
	if mode == "" {
		t.Skip("helper process entry point; runs only when re-executed")
	}
	repo := Repository{
		Root:         os.Getenv(helperRootEnv),
		GitCommonDir: os.Getenv(helperGitCommonEnv),
		Mode:         StorageMode(os.Getenv(helperStorageModeEnv)),
	}

	switch mode {
	case "acquire":
		lock, err := Acquire(context.Background(), Request{
			Repository: repo,
			Command:    "start",
			Capability: Capability{Commands: []string{"start"}, TaskFields: []string{"status"}},
		})
		if err != nil {
			os.Stdout.WriteString("refused: " + err.Error() + "\n")
			return
		}
		_ = lock.Release()
		os.Stdout.WriteString("acquired\n")
	case "join":
		joined, err := Join(JoinRequest{
			Repository:       repo,
			Command:          "complete",
			Token:            os.Getenv(helperTokenEnv),
			ExecutableSHA256: os.Getenv(helperExecDigestEnv),
			Capability:       childCapability(),
		})
		if err != nil {
			os.Stdout.WriteString("refused: " + err.Error() + "\n")
			return
		}
		os.Stdout.WriteString("joined: " + joined.Owner().LockID + "\n")
	default:
		os.Stdout.WriteString("unknown mode\n")
	}
}

func runHelper(t *testing.T, repo Repository, mode string, extra ...string) string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestRepolockHelperProcess$")
	cmd.Env = append(os.Environ(),
		helperModeEnv+"="+mode,
		helperRootEnv+"="+repo.Root,
		helperGitCommonEnv+"="+repo.GitCommonDir,
		helperStorageModeEnv+"="+string(repo.Mode),
	)
	cmd.Env = append(cmd.Env, extra...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helper process failed: %v\n%s", err, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "acquired") || strings.HasPrefix(line, "refused: ") || strings.HasPrefix(line, "joined: ") {
			return line
		}
	}
	t.Fatalf("helper process produced no verdict:\n%s", out)
	return ""
}

func TestSecondProcessRefusesAHeldLockWithoutMutatingIt(t *testing.T) {
	repo := committedRepo(t)
	lock := acquire(t, repo)
	before := readLockBytes(t, repo)

	verdict := runHelper(t, repo, "acquire")
	if !strings.HasPrefix(verdict, "refused: ") {
		t.Fatalf("second process verdict = %q, want a refusal", verdict)
	}
	if got := readLockBytes(t, repo); !slices.Equal(got, before) {
		t.Fatal("the refused second process rewrote the lock file")
	}
	if lock.Owner().PID != os.Getpid() {
		t.Fatalf("owner pid = %d, want this process", lock.Owner().PID)
	}
}

// A linked worktree resolves the same Git common directory, so a writer running
// there must contend with this one rather than take a second lock.
func TestLinkedWorktreeSharesTheSameLock(t *testing.T) {
	repo := committedRepo(t)
	acquire(t, repo)

	linked := Repository{
		Root:         t.TempDir(),
		GitCommonDir: repo.GitCommonDir,
		Mode:         ModeCommitted,
	}
	if LockPath(linked) != LockPath(repo) {
		t.Fatalf("linked worktree resolved %q, want the shared %q", LockPath(linked), LockPath(repo))
	}
	if verdict := runHelper(t, linked, "acquire"); !strings.HasPrefix(verdict, "refused: ") {
		t.Fatalf("linked-worktree verdict = %q, want a refusal", verdict)
	}
}

func TestAnotherProcessAcquiresAfterRelease(t *testing.T) {
	repo := committedRepo(t)
	lock, err := Acquire(context.Background(), writerRequest(repo))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	if verdict := runHelper(t, repo, "acquire"); verdict != "acquired" {
		t.Fatalf("verdict after release = %q, want acquired", verdict)
	}
}

func TestDelegatedChildProcessJoinsWithoutReleasingTheLock(t *testing.T) {
	repo := committedRepo(t)
	lock, delegation := acquireDelegating(t, repo, stageExecutable(t, "taskrail-bytes"))
	before := readLockBytes(t, repo)

	verdict := runHelper(t, repo, "join",
		helperTokenEnv+"="+delegation.Token,
		helperExecDigestEnv+"="+delegation.ExecutableSHA256,
	)
	if verdict != "joined: "+lock.Owner().LockID {
		t.Fatalf("delegated child verdict = %q, want a join on %s", verdict, lock.Owner().LockID)
	}
	if got := readLockBytes(t, repo); !slices.Equal(got, before) {
		t.Fatal("the delegated child mutated the lock file")
	}

	// An unrelated writer in that same child position still refuses: joining is
	// gated on the token, not on being a descendant.
	unrelated := runHelper(t, repo, "join",
		helperTokenEnv+"="+strings.Repeat("c", 64),
		helperExecDigestEnv+"="+delegation.ExecutableSHA256,
	)
	if !strings.HasPrefix(unrelated, "refused: ") {
		t.Fatalf("unrelated child verdict = %q, want a refusal", unrelated)
	}
	if verdict := runHelper(t, repo, "acquire"); !strings.HasPrefix(verdict, "refused: ") {
		t.Fatalf("unrelated loop verdict = %q, want a refusal", verdict)
	}
}

func TestReleaseAfterAnInterruptedProcessIsNotAutomatic(t *testing.T) {
	repo := committedRepo(t)
	// Simulate the abrupt-death case the protocol refuses to paper over: a lock
	// file with no live owner. The next process must still refuse.
	writeRawLock(t, repo, Owner{
		LockID:         strings.Repeat("d", 32),
		Command:        "verify",
		PID:            1 << 30,
		Host:           "gone",
		StartedAt:      "2001-02-03T04:05:06Z",
		RepositoryRoot: repo.Root,
		StorageMode:    ModeCommitted,
		StorageRoot:    repo.Root,
	})
	if verdict := runHelper(t, repo, "acquire"); !strings.HasPrefix(verdict, "refused: ") {
		t.Fatalf("verdict against an abandoned lock = %q, want a refusal", verdict)
	}
	if _, err := os.Stat(LockPath(repo)); err != nil {
		t.Fatalf("the abandoned lock was auto-cleared: %v", err)
	}
}

func TestLockRootIsCreatedOnlyByAcquisition(t *testing.T) {
	repo := nonGitRepo(t)
	if _, err := Inspect(repo); err != nil {
		t.Fatalf("inspect: %v", err)
	}
	runtime := filepath.Join(repo.Root, ".taskrail", "runtime")
	if _, err := os.Stat(runtime); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only inspection created %s: %v", runtime, err)
	}
	acquire(t, repo)
	if _, err := os.Stat(runtime); err != nil {
		t.Fatalf("acquisition did not create the runtime lock root: %v", err)
	}
}
