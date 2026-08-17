package repolock

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// deadPID returns a process id that has provably exited, so a stale-owner
// fixture cannot accidentally name a live process.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestNothingMatchesThis$")
	if err := cmd.Run(); err != nil {
		t.Fatalf("run short-lived process: %v", err)
	}
	return cmd.Process.Pid
}

func staleOwner(repo Repository, pid int) Owner {
	return Owner{
		LockID:         strings.Repeat("b", 32),
		Command:        "verify",
		PID:            pid,
		Host:           "a-host-that-is-not-this-one",
		StartedAt:      "2001-02-03T04:05:06Z",
		RepositoryRoot: repo.Root,
		StorageMode:    ModeCommitted,
		StorageRoot:    repo.Root,
	}
}

func clearRequest(repo Repository, lockID string, digest string) ClearRequest {
	return ClearRequest{Repository: repo, LockID: lockID, ExpectSHA256: digest}
}

func TestClearRemovesTheExactObservedStaleLock(t *testing.T) {
	repo := committedRepo(t)
	raw := writeRawLock(t, repo, staleOwner(repo, deadPID(t)))

	prior, err := Clear(clearRequest(repo, strings.Repeat("b", 32), sha256Hex(raw)))
	if err != nil {
		t.Fatalf("clear a stale lock: %v", err)
	}
	if prior != sha256Hex(raw) {
		t.Fatalf("prior digest = %q, want the observed raw digest", prior)
	}
	if _, err := os.Stat(LockPath(repo)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("clear left the lock file behind: %v", err)
	}
}

// Clearing ownership must never reach retained transaction data that shares
// the lock root: only the named lock record goes away.
func TestClearNeverRemovesRetainedTransactionData(t *testing.T) {
	repo := committedRepo(t)
	raw := writeRawLock(t, repo, staleOwner(repo, deadPID(t)))
	transactions := filepath.Join(filepath.Dir(LockPath(repo)), "transactions", strings.Repeat("1", 32))
	journal := filepath.Join(transactions, "journal.json")
	if err := os.MkdirAll(transactions, 0o755); err != nil {
		t.Fatalf("create retained transactions tree: %v", err)
	}
	if err := os.WriteFile(journal, []byte(`{"retained":true}`), 0o644); err != nil {
		t.Fatalf("write retained journal: %v", err)
	}

	if _, err := Clear(clearRequest(repo, strings.Repeat("b", 32), sha256Hex(raw))); err != nil {
		t.Fatalf("clear a stale lock beside retained data: %v", err)
	}
	data, err := os.ReadFile(journal)
	if err != nil || string(data) != `{"retained":true}` {
		t.Fatalf("clear disturbed retained transaction data: %q %v", data, err)
	}
	if _, err := os.Stat(transactions); err != nil {
		t.Fatalf("clear removed the transactions directory: %v", err)
	}
}

func TestClearRefusesAProvablyLiveSameHostOwner(t *testing.T) {
	repo := committedRepo(t)
	// A live same-host owner: this process holds the PID and the host matches.
	live := staleOwner(repo, os.Getpid())
	host, err := os.Hostname()
	if err != nil {
		t.Fatalf("hostname: %v", err)
	}
	live.Host = host
	raw := writeRawLock(t, repo, live)

	if _, err := Clear(clearRequest(repo, live.LockID, sha256Hex(raw))); !errors.Is(err, ErrLiveOwner) {
		t.Fatalf("clear a live owner error = %v, want ErrLiveOwner", err)
	}
	if got := readLockBytes(t, repo); !strings.Contains(string(got), live.LockID) {
		t.Fatal("the refused clear removed or rewrote the live lock")
	}
}

func TestClearAllowsADeadSameHostOwner(t *testing.T) {
	repo := committedRepo(t)
	host, err := os.Hostname()
	if err != nil {
		t.Fatalf("hostname: %v", err)
	}
	dead := staleOwner(repo, deadPID(t))
	dead.Host = host
	raw := writeRawLock(t, repo, dead)

	if _, err := Clear(clearRequest(repo, dead.LockID, sha256Hex(raw))); err != nil {
		t.Fatalf("clear a dead same-host owner: %v", err)
	}
}

func TestClearRefusals(t *testing.T) {
	valid := strings.Repeat("b", 32)
	tests := []struct {
		name string
		req  func(t *testing.T) ClearRequest
		want error
	}{
		{"malformed lock id", func(t *testing.T) ClearRequest {
			repo := committedRepo(t)
			observed := writeRawLock(t, repo, staleOwner(repo, deadPID(t)))
			return ClearRequest{Repository: repo, LockID: "NOT-HEX", ExpectSHA256: sha256Hex(observed)}
		}, nil},
		{"malformed expected digest", func(t *testing.T) ClearRequest {
			repo := committedRepo(t)
			writeRawLock(t, repo, staleOwner(repo, deadPID(t)))
			return ClearRequest{Repository: repo, LockID: valid, ExpectSHA256: "nope"}
		}, nil},
		{"absent lock", func(t *testing.T) ClearRequest {
			return clearRequest(committedRepo(t), valid, strings.Repeat("a", 64))
		}, ErrNotHeld},
		{"digest race", func(t *testing.T) ClearRequest {
			repo := committedRepo(t)
			writeRawLock(t, repo, staleOwner(repo, deadPID(t)))
			return clearRequest(repo, valid, sha256Hex([]byte("changed bytes")))
		}, ErrChanged},
		{"replacement lock id", func(t *testing.T) ClearRequest {
			repo := committedRepo(t)
			observed := writeRawLock(t, repo, staleOwner(repo, deadPID(t)))
			return clearRequest(repo, strings.Repeat("c", 32), sha256Hex(observed))
		}, ErrChanged},
		{"malformed metadata", func(t *testing.T) ClearRequest {
			repo := committedRepo(t)
			if err := os.MkdirAll(filepath.Dir(LockPath(repo)), 0o755); err != nil {
				t.Fatalf("create lock root: %v", err)
			}
			malformed := []byte("{not json")
			if err := os.WriteFile(LockPath(repo), malformed, 0o644); err != nil {
				t.Fatalf("write malformed lock: %v", err)
			}
			return clearRequest(repo, valid, sha256Hex(malformed))
		}, ErrMalformed},
		{"invalid repository", func(t *testing.T) ClearRequest {
			return clearRequest(Repository{Root: "relative", Mode: ModeCommitted}, valid, strings.Repeat("a", 64))
		}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Clear(test.req(t))
			if err == nil {
				t.Fatal("expected a refusal")
			}
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

// A refusal must leave the record untouched, exactly as acquisition refusals do.
func TestClearRefusalLeavesTheRecordUntouched(t *testing.T) {
	repo := committedRepo(t)
	observed := writeRawLock(t, repo, staleOwner(repo, deadPID(t)))

	if _, err := Clear(clearRequest(repo, strings.Repeat("c", 32), sha256Hex(observed))); !errors.Is(err, ErrChanged) {
		t.Fatalf("replacement refusal error = %v, want ErrChanged", err)
	}
	if got := readLockBytes(t, repo); string(got) != string(observed) {
		t.Fatal("a refused clear rewrote the lock record")
	}
}

// A stale lock in a non-Git root-local repository clears through the same
// guarded path, so operator recovery does not depend on Git.
func TestClearWorksInANonGitRootLocalRepository(t *testing.T) {
	repo := nonGitRepo(t)
	stale := staleOwner(repo, deadPID(t))
	stale.StorageMode = ModeCommitted
	raw := writeRawLock(t, repo, stale)

	if _, err := Clear(clearRequest(repo, stale.LockID, sha256Hex(raw))); err != nil {
		t.Fatalf("clear a non-Git stale lock: %v", err)
	}
	if _, err := os.Stat(LockPath(repo)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("clear left the non-Git lock behind: %v", err)
	}
}
