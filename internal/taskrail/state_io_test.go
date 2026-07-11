package taskrail

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFsCauseStripsAbsolutePath proves the read-error helper drops the
// *fs.PathError's absolute path while preserving errors.Is classification, so
// callers can still detect not-exist and emit portable, path-free error tails.
func TestFsCauseStripsAbsolutePath(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "does-not-exist.md")
	_, raw := os.ReadFile(abs)
	if raw == nil {
		t.Fatal("expected a read error for a missing file")
	}
	if !strings.Contains(raw.Error(), abs) {
		t.Fatalf("precondition: raw error should embed the absolute path: %v", raw)
	}

	stripped := fsCause(raw)
	if strings.Contains(stripped.Error(), abs) {
		t.Fatalf("fsCause must not leak the absolute path: %v", stripped)
	}
	if !errors.Is(stripped, os.ErrNotExist) {
		t.Fatalf("fsCause must preserve not-exist classification: %v", stripped)
	}

	// A non-PathError passes through unchanged.
	plain := errors.New("boom")
	if fsCause(plain) != plain {
		t.Fatalf("fsCause must return non-PathError errors unchanged")
	}
}

// A directory path makes os.WriteFile fail, letting us assert the write-error
// wrapping without depending on platform-specific errno text.
func TestSaveStateWrapsWriteError(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Now().UTC())

	dir := filepath.Join(repo, "planning", "state-as-dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	svc.paths.StateFile = dir

	err := svc.saveState(&State{Frontmatter: StateFrontmatter{SchemaVersion: 1}, Body: "x"})
	if err == nil {
		t.Fatal("expected error writing state to a directory path")
	}
	if !strings.Contains(err.Error(), "write state file") {
		t.Fatalf("error missing operation context: %q", err.Error())
	}
	if errors.Unwrap(err) == nil {
		t.Fatalf("expected wrapped underlying error, got flat: %q", err.Error())
	}
}

func TestSaveTaskWrapsWriteError(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Now().UTC())

	dir := filepath.Join(repo, "planning", "task-as-dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	err := svc.saveTask(&Task{Frontmatter: TaskFrontmatter{ID: "T-001"}, Filename: dir})
	if err == nil {
		t.Fatal("expected error writing task to a directory path")
	}
	if !strings.Contains(err.Error(), "write task file") {
		t.Fatalf("error missing operation context: %q", err.Error())
	}
	wantPrefix := "write task file " + filepath.Base(dir) + ":"
	if !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Fatalf("error missing failing task filename context %q: %q", wantPrefix, err.Error())
	}
	if errors.Unwrap(err) == nil {
		t.Fatalf("expected wrapped underlying error, got flat: %q", err.Error())
	}
}
