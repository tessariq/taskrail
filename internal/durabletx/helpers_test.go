package durabletx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tessariq/taskrail/internal/repolock"
)

// newRepository returns an explicitly supplied non-Git repository context. The
// temporary root is resolved because durable binding compares physical paths,
// and a symlinked temporary directory would otherwise refuse as an alias.
func newRepository(t *testing.T) repolock.Repository {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temporary root: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".taskrail", "runtime"), 0o755); err != nil {
		t.Fatalf("seed runtime directory: %v", err)
	}
	requireDirectoryDurability(t, root)
	return repolock.Repository{Root: root, Mode: repolock.ModeCommitted}
}

func newLocalRepository(t *testing.T) repolock.Repository {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	common := filepath.Join(root, ".git")
	if err := os.MkdirAll(filepath.Join(common, "taskrail"), 0o755); err != nil {
		t.Fatal(err)
	}
	repo := repolock.Repository{Root: root, GitCommonDir: common, Mode: repolock.ModeLocal}
	if err := os.MkdirAll(repo.StorageRoot(), 0o755); err != nil {
		t.Fatal(err)
	}
	requireDirectoryDurability(t, repo.StorageRoot())
	return repo
}

func requireDirectoryDurability(t *testing.T, root string) {
	t.Helper()
	directory, err := os.Open(root)
	if err != nil {
		t.Fatalf("open directory durability probe: %v", err)
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		t.Skipf("filesystem does not support durable directory sync: %v", syncErr)
	}
	if closeErr != nil {
		t.Fatalf("close directory durability probe: %v", closeErr)
	}
}

func ownerCapability() repolock.Capability {
	return repolock.Capability{Commands: []string{"init"}, TaskFields: []string{"status"}}
}

func acquire(t *testing.T, repo repolock.Repository, capability repolock.Capability) *repolock.Lock {
	t.Helper()
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository:    repo,
		Command:       "init",
		TransactionID: "0123456789abcdef0123456789abcdef",
		Capability:    capability,
	})
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })
	return lock
}

func member(reported, content string) Member {
	return Member{Kind: Managed, Reported: reported, Path: reported, Content: []byte(content)}
}

func seed(t *testing.T, repo repolock.Repository, reported, content string) {
	t.Helper()
	physical := filepath.Join(repo.Root, filepath.FromSlash(reported))
	if err := os.MkdirAll(filepath.Dir(physical), 0o755); err != nil {
		t.Fatalf("seed directory for %s: %v", reported, err)
	}
	if err := os.WriteFile(physical, []byte(content), 0o644); err != nil {
		t.Fatalf("seed %s: %v", reported, err)
	}
}

func read(t *testing.T, repo repolock.Repository, reported string) (string, bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo.Root, filepath.FromSlash(reported)))
	if errors.Is(err, os.ErrNotExist) {
		return "", false
	}
	if err != nil {
		t.Fatalf("read %s: %v", reported, err)
	}
	return string(data), true
}

// retained is the transaction ID of the single retained fence, or "" when the
// transactions root holds nothing.
func retained(t *testing.T, repo repolock.Repository) string {
	t.Helper()
	entries, err := os.ReadDir(TransactionsDir(repo))
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatalf("read transactions root: %v", err)
	}
	switch len(entries) {
	case 0:
		return ""
	case 1:
		return entries[0].Name()
	default:
		t.Fatalf("transactions root holds %d entries, want at most one", len(entries))
		return ""
	}
}

func request(command string, members ...Member) Request {
	return Request{Command: command, Members: members}
}

func txError(t *testing.T, err error) *Error {
	t.Helper()
	var durableErr *Error
	if !errors.As(err, &durableErr) {
		t.Fatalf("error %v is not a durable transaction failure", err)
	}
	return durableErr
}

type delegatedOwnership struct {
	*repolock.Lock
	capability repolock.Capability
}

func (d delegatedOwnership) Capability() repolock.Capability { return d.capability }
func (d delegatedOwnership) IsDelegate() bool                { return true }
