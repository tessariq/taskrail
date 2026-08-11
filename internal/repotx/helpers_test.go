package repotx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tessariq/taskrail/internal/repolock"
)

// newRepository returns an explicitly supplied non-Git repository context. The
// temporary root is resolved because containment compares physical paths, and a
// symlinked temporary directory would otherwise look like an escape.
func newRepository(t *testing.T) repolock.Repository {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temporary root: %v", err)
	}
	return repolock.Repository{Root: root, Mode: repolock.ModeCommitted}
}

func ownerCapability() repolock.Capability {
	return repolock.Capability{
		Commands:   []string{"complete"},
		TaskFields: []string{"status", "updated_at"},
	}
}

func acquire(t *testing.T, repo repolock.Repository, capability repolock.Capability) *repolock.Lock {
	t.Helper()
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: repo,
		Command:    "complete",
		Capability: capability,
	})
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })
	return lock
}

func managed(repo repolock.Repository, reported string, content string) Candidate {
	return Candidate{
		Path: Path{
			Kind:     Managed,
			Reported: reported,
			Physical: filepath.Join(repo.Root, filepath.FromSlash(reported)),
		},
		Content: []byte(content),
	}
}

func seed(t *testing.T, repo repolock.Repository, reported string, content string) {
	t.Helper()
	physical := filepath.Join(repo.Root, filepath.FromSlash(reported))
	if err := os.MkdirAll(filepath.Dir(physical), 0o755); err != nil {
		t.Fatalf("seed directory for %s: %v", reported, err)
	}
	if err := os.WriteFile(physical, []byte(content), 0o644); err != nil {
		t.Fatalf("seed %s: %v", reported, err)
	}
}

func readManaged(t *testing.T, repo repolock.Repository, reported string) (string, bool) {
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

func snapshotFor(t *testing.T, snapshots []Snapshot, reported string) Snapshot {
	t.Helper()
	for _, snapshot := range snapshots {
		if snapshot.Path == reported {
			return snapshot
		}
	}
	t.Fatalf("no snapshot for %s in %+v", reported, snapshots)
	return Snapshot{}
}

func wantDigest(t *testing.T, got *string, content string, what string) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s digest is absent, want the digest of %q", what, content)
	}
	if *got != digestOf([]byte(content)) {
		t.Fatalf("%s digest = %q, want the digest of %q", what, *got, content)
	}
}

func wantMachineCode(t *testing.T, txErr *Error, want string) {
	t.Helper()
	code, registered := txErr.MachineCode()
	if !registered || code != want {
		t.Fatalf("machine code = (%q, registered=%v), want %q", code, registered, want)
	}
}

func txError(t *testing.T, err error) *Error {
	t.Helper()
	var txErr *Error
	if !errors.As(err, &txErr) {
		t.Fatalf("error %v is not a transaction failure", err)
	}
	return txErr
}
