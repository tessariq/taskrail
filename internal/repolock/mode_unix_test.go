//go:build unix

package repolock

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// The lock record must be inspectable by anyone sharing the repository, so
// neither publication path may let the process umask narrow it. The linking path
// inherits the staged file's explicit mode; the link-free fallback creates the
// path itself, where O_CREATE's mode argument *is* masked — so this pins both.
func TestBothPublicationPathsIgnoreTheProcessUmask(t *testing.T) {
	previous := syscall.Umask(0o077)
	t.Cleanup(func() { syscall.Umask(previous) })

	linked := committedRepo(t)
	acquire(t, linked)
	assertLockMode(t, LockPath(linked))

	// A filesystem without hard links reaches claimAndFill instead of os.Link;
	// call it directly, since no temporary directory here lacks link support.
	fallback := LockPath(committedRepo(t))
	if err := os.MkdirAll(filepath.Dir(fallback), 0o755); err != nil {
		t.Fatalf("create lock root: %v", err)
	}
	if err := claimAndFill(fallback, []byte("{}")); err != nil {
		t.Fatalf("claim without linking: %v", err)
	}
	assertLockMode(t, fallback)
}

func assertLockMode(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat lock file: %v", err)
	}
	if got := info.Mode().Perm(); got != lockFileMode {
		t.Fatalf("lock file %s mode = %04o, want %04o", path, got, lockFileMode)
	}
}
