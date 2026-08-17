package taskrail

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tessariq/taskrail/internal/repolock"
)

// The lock command surface (specs/v0.5.0.md#repository-discovery-locking-and-recovery):
// `lock status` inspects read-only, `lock clear` is a guarded compare-and-delete,
// and both map their refusals onto the registered machine error codes their
// companion row allows.

func lockOwnerFixture(repo string, pid int, host string) repolock.Owner {
	return repolock.Owner{
		LockID:         strings.Repeat("e", 32),
		Command:        "verify",
		PID:            pid,
		Host:           host,
		StartedAt:      "2001-02-03T04:05:06Z",
		RepositoryRoot: repo,
		StorageMode:    repolock.ModeCommitted,
		StorageRoot:    repo,
	}
}

// seedLockFile installs a raw lock record and returns its bytes.
func seedLockFile(t *testing.T, repo string, owner repolock.Owner) []byte {
	t.Helper()
	svc := newTestService(t, repo, time.Now())
	path := repolock.LockPath(svc.lockRepository())
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create lock root: %v", err)
	}
	data, err := json.Marshal(owner)
	if err != nil {
		t.Fatalf("marshal owner: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write lock file: %v", err)
	}
	return data
}

func sha256Of(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func TestLockStatusReportsAbsenceWithoutWriting(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Now())
	before := readTree(t, repo)

	result, err := svc.LockStatus()
	if err != nil {
		t.Fatalf("lock status: %v", err)
	}
	if result.Held || result.SHA256 != nil || result.Owner != nil {
		t.Fatalf("absent lock reported %+v", result)
	}
	assertTreeUnchanged(t, repo, before)
}

func TestLockStatusReportsExactOwnerMetadataAndDigest(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	owner := lockOwnerFixture(repo, 1, "another-host")
	transaction := "tx-abc"
	owner.TransactionID = &transaction
	raw := seedLockFile(t, repo, owner)

	svc := newTestService(t, repo, time.Now())
	result, err := svc.LockStatus()
	if err != nil {
		t.Fatalf("lock status: %v", err)
	}
	if !result.Held || result.Owner == nil || result.SHA256 == nil {
		t.Fatalf("held lock reported %+v", result)
	}
	if *result.SHA256 != sha256Of(raw) {
		t.Fatalf("sha256 = %q, want the raw-file digest", *result.SHA256)
	}
	if result.Owner.LockID != owner.LockID || result.Owner.PID != 1 || result.Owner.Host != "another-host" {
		t.Fatalf("owner = %+v", result.Owner)
	}
	if result.Owner.TransactionID == nil || *result.Owner.TransactionID != transaction {
		t.Fatalf("transaction_id = %v", result.Owner.TransactionID)
	}
	// The persisted metadata carries only a delegation-token digest; a
	// delegated lock must expose the same digest-only surface.
	if result.Owner.DelegationDigest != nil {
		t.Fatalf("undelegated lock reported a delegation digest: %+v", result.Owner)
	}

	// The schema-1 document never gains a token member: the owner marshals to
	// exactly the companion's LockOwner shape.
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal status result: %v", err)
	}
	for _, forbidden := range []string{"token", "grant"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("status result leaks %q: %s", forbidden, encoded)
		}
	}
}

func TestLockStatusRefusesMalformedMetadataAsRepositoryInvalid(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Now())
	path := repolock.LockPath(svc.lockRepository())
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create lock root: %v", err)
	}
	before := []byte("{not json")
	if err := os.WriteFile(path, before, 0o644); err != nil {
		t.Fatalf("write malformed lock: %v", err)
	}

	if _, err := svc.LockStatus(); err == nil {
		t.Fatal("expected a refusal for malformed metadata")
	} else if MachineFailureFor(err).Code != MachineCodeRepositoryInvalid {
		t.Fatalf("code = %q, want repository_invalid", MachineFailureFor(err).Code)
	}
	if got, readErr := os.ReadFile(path); readErr != nil || string(got) != string(before) {
		t.Fatalf("status rewrote malformed bytes: %q %v", got, readErr)
	}
}

func TestLockClearRemovesOnlyTheUnchangedStaleLock(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	raw := seedLockFile(t, repo, lockOwnerFixture(repo, 1, "another-host"))
	transactions := filepath.Join(filepath.Dir(repolock.LockPath(newTestService(t, repo, time.Now()).lockRepository())), "transactions", strings.Repeat("1", 32))
	if err := os.MkdirAll(transactions, 0o755); err != nil {
		t.Fatalf("create transactions tree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(transactions, "journal.json"), []byte(`{"retained":true}`), 0o644); err != nil {
		t.Fatalf("write journal: %v", err)
	}

	svc := newTestService(t, repo, time.Now())
	result, err := svc.LockClear(strings.Repeat("e", 32), sha256Of(raw))
	if err != nil {
		t.Fatalf("lock clear: %v", err)
	}
	if result.LockID != strings.Repeat("e", 32) || !result.Cleared || result.PriorSHA256 != sha256Of(raw) {
		t.Fatalf("clear result = %+v", result)
	}
	journal, err := os.ReadFile(filepath.Join(transactions, "journal.json"))
	if err != nil || string(journal) != `{"retained":true}` {
		t.Fatalf("clear removed retained transaction data: %q %v", journal, err)
	}
}

func TestLockClearRefusalCodes(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	raw := seedLockFile(t, repo, lockOwnerFixture(repo, 1, "another-host"))
	svc := newTestService(t, repo, time.Now())
	lockID := strings.Repeat("e", 32)

	tests := []struct {
		name string
		id   string
		dig  string
		want string
	}{
		{"malformed lock id", "NOT-HEX", sha256Of(raw), MachineCodeInvalidArguments},
		{"malformed expected digest", lockID, "nope", MachineCodeInvalidDigest},
		{"digest race", lockID, sha256Of([]byte("other bytes")), MachineCodeSourceChanged},
		{"replacement lock id", strings.Repeat("f", 32), sha256Of(raw), MachineCodeSourceChanged},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := svc.LockClear(test.id, test.dig)
			if err == nil {
				t.Fatal("expected a refusal")
			}
			if MachineFailureFor(err).Code != test.want {
				t.Fatalf("code = %q, want %q (%v)", MachineFailureFor(err).Code, test.want, err)
			}
		})
	}
	// The refused clears left the record untouched.
	if _, err := os.Stat(repolock.LockPath(svc.lockRepository())); err != nil {
		t.Fatalf("a refused clear removed the lock: %v", err)
	}
}

func TestLockClearRefusalCodesForAbsentAndLiveOwners(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	absent := newTestService(t, repo, time.Now())
	if _, err := absent.LockClear(strings.Repeat("e", 32), strings.Repeat("a", 64)); MachineFailureFor(err).Code != MachineCodeInvalidDigest {
		t.Fatalf("absent lock code = %q, want invalid_digest (%v)", MachineFailureFor(err).Code, err)
	}

	live := seedFixtureRepo(t)
	host, err := os.Hostname()
	if err != nil {
		t.Fatalf("hostname: %v", err)
	}
	owner := lockOwnerFixture(live, os.Getpid(), host)
	raw := seedLockFile(t, live, owner)
	if _, err := newTestService(t, live, time.Now()).LockClear(owner.LockID, sha256Of(raw)); MachineFailureFor(err).Code != MachineCodeLockHeld {
		t.Fatalf("live owner code = %q, want lock_held (%v)", MachineFailureFor(err).Code, err)
	}
}

func TestLockStatusAndClearWorkOutsideGit(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	seedFixtureTree(t, repo)
	// No .git marker: discovery walks ancestors to the layout marker and the
	// lock resolves beneath the root-local runtime directory.
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: 1\n")
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("discover non-Git repo: %v", err)
	}
	if got := repolock.LockPath(svc.lockRepository()); got != filepath.Join(repo, ".taskrail", "runtime", "mutation.lock") {
		t.Fatalf("non-Git lock path = %q", got)
	}

	result, err := svc.LockStatus()
	if err != nil || result.Held {
		t.Fatalf("non-Git status = %+v, %v", result, err)
	}

	owner := lockOwnerFixture(repo, 1, "another-host")
	raw := seedLockFile(t, repo, owner)
	svc2, err := NewService(repo)
	if err != nil {
		t.Fatalf("rediscover: %v", err)
	}
	if result, err := svc2.LockStatus(); err != nil || !result.Held || result.Owner.LockID != owner.LockID {
		t.Fatalf("non-Git held status = %+v, %v", result, err)
	}
	if _, err := svc2.LockClear(owner.LockID, sha256Of(raw)); err != nil {
		t.Fatalf("non-Git clear: %v", err)
	}
	if _, err := os.Stat(repolock.LockPath(svc2.lockRepository())); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("non-Git clear left the lock behind: %v", err)
	}
}

// readTree snapshots every file under root for a zero-write assertion.
func readTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshot[path] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return snapshot
}

func assertTreeUnchanged(t *testing.T, root string, before map[string]string) {
	t.Helper()
	after := readTree(t, root)
	if len(before) != len(after) {
		t.Fatalf("file set changed: %d files before, %d after", len(before), len(after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Fatalf("%s changed", path)
		}
	}
}
