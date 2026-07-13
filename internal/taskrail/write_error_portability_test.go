package taskrail

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// assertNoRootLeak fails when err is nil or embeds the absolute repository root.
// The write-side error-portability contract (T-088, mirroring T-062's read-side
// sweep) requires every filesystem error to name a repo-relative path and never
// the caller's absolute repository location.
func assertNoRootLeak(t *testing.T, repo string, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a write error, got nil")
	}
	if strings.Contains(err.Error(), repo) {
		t.Fatalf("error leaks absolute repo path %q: %v", repo, err)
	}
}

// requireReadOnlyDirBlocksWrites makes dir read-only and verifies the platform
// actually enforces that against file creation, skipping the test otherwise.
// The permission-based fault injections below depend on a read-only directory
// rejecting a write, but that does not hold everywhere: root bypasses the write
// bits, and native Windows ignores a directory's read-only attribute for file
// creation. Probing the actual behaviour (rather than keying off GOOS/uid)
// keeps the check correct across platforms, privilege levels, and filesystems —
// where the injection cannot fire it is a skip, not a failure. Callers must set
// dir to mode 0o755 first; a t.Cleanup restores it so t.TempDir removal works.
func requireReadOnlyDirBlocksWrites(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod read-only %s: %v", filepath.Base(dir), err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	probe := filepath.Join(dir, ".write-probe")
	if err := os.WriteFile(probe, []byte("x"), 0o644); err == nil {
		_ = os.Remove(probe)
		t.Skip("read-only directory does not block writes here (root or native Windows); permission injection cannot fire")
	}
}

// assertPortablePermissionError requires err to name a repo-relative path (no
// absolute root leak) while still classifying as fs.ErrPermission through fsCause.
func assertPortablePermissionError(t *testing.T, repo string, err error) {
	t.Helper()
	assertNoRootLeak(t, repo, err)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("errors.Is(fs.ErrPermission) classification must survive fsCause: %v", err)
	}
}

// replaceWithDir swaps the file at path for a directory so a subsequent WriteFile
// there fails with EISDIR.
func replaceWithDir(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove %s: %v", filepath.Base(path), err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("occupy %s with dir: %v", filepath.Base(path), err)
	}
}

func TestEnsureDirErrorOmitsAbsolutePath(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	blocker := filepath.Join(repo, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed blocker file: %v", err)
	}
	// MkdirAll below a regular file fails (ENOTDIR).
	assertNoRootLeak(t, repo, ensureDir(repo, filepath.Join(blocker, "sub")))
}

func TestWriteFileIfMissingWriteErrorOmitsAbsolutePath(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	ro := filepath.Join(repo, "ro")
	if err := os.Mkdir(ro, 0o755); err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	requireReadOnlyDirBlocksWrites(t, ro)
	err := writeFileIfMissing(repo, filepath.Join(ro, "f.md"), []byte("x"))
	assertPortablePermissionError(t, repo, err)
}

func TestWriteFileIfMissingStatErrorOmitsAbsolutePath(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	file := filepath.Join(repo, "afile")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	// Stat of a path *below* a regular file returns ENOTDIR (not ErrNotExist),
	// hitting writeFileIfMissing's stat-error branch.
	assertNoRootLeak(t, repo, writeFileIfMissing(repo, filepath.Join(file, "sub.md"), []byte("y")))
}

func TestSaveStateWriteErrorOmitsAbsolutePath(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC))
	state, err := svc.loadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	replaceWithDir(t, svc.paths.StateFile)
	assertNoRootLeak(t, repo, svc.saveState(state))
}

func TestSaveTaskWriteErrorOmitsAbsolutePath(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-010", "Portable task", "todo", "low", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC))
	tasks, err := svc.loadTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	task := tasks[0]
	replaceWithDir(t, task.Filename)
	assertNoRootLeak(t, repo, svc.saveTask(task))
}

func TestInstallSkillFileWriteErrorOmitsAbsolutePath(t *testing.T) {
	t.Parallel()
	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC))
	dir := filepath.Join(repo, ".claude", "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("seed skills dir: %v", err)
	}
	requireReadOnlyDirBlocksWrites(t, dir)
	var res SkillInstallResult
	err := svc.installSkillFile(filepath.Join(dir, "probe.md"), []byte("x"), false, &res)
	assertPortablePermissionError(t, repo, err)
}

func TestWriteMarkerErrorOmitsAbsolutePath(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	// Occupy .taskrail with a regular file so MkdirAll fails (ENOTDIR).
	if err := os.WriteFile(filepath.Join(repo, taskrailConfigDir), []byte("x"), 0o644); err != nil {
		t.Fatalf("occupy config dir path: %v", err)
	}
	assertNoRootLeak(t, repo, writeMarker(repo, LayoutConfig{LayoutVersion: 1}))
}

func TestWriteImportedSpecErrorOmitsAbsolutePath(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC))
	requireReadOnlyDirBlocksWrites(t, svc.paths.SpecsDir)
	_, err := svc.writeImportedSpec(ImportDraft{Source: "notes.md"})
	assertPortablePermissionError(t, repo, err)
}

func TestAddSpecWriteErrorOmitsAbsolutePath(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC))
	requireReadOnlyDirBlocksWrites(t, svc.paths.SpecsDir)
	_, err := svc.AddSpec("v9.9.9")
	assertPortablePermissionError(t, repo, err)
}

func TestVerifyWriteErrorOmitsAbsolutePath(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Verified item", "completed", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC))
	// Pre-create plan.md as a directory inside the artifact dir so the plan write
	// fails (EISDIR) after ensureDir has already created the tree.
	artifactDir := filepath.Join(svc.paths.VerifyDir, "T-002", "20260624T120000Z")
	if err := os.MkdirAll(filepath.Join(artifactDir, "plan.md"), 0o755); err != nil {
		t.Fatalf("occupy plan path with dir: %v", err)
	}
	_, err := svc.Verify(VerifyInput{TaskID: "T-002", Result: "pass", Summary: "x"})
	assertNoRootLeak(t, repo, err)
}
