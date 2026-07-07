package taskrail

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
