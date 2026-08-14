package durablefs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tessariq/taskrail/internal/repolock"
)

func TestRepositoryLockSerializesRoots(t *testing.T) {
	repo := t.TempDir()
	lock := acquireTestLock(t, repo)
	root, err := Open(repo, lock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })

	_, err = repolock.Acquire(context.Background(), lockRequest(repo))
	if !errors.Is(err, repolock.ErrSameProcess) {
		t.Fatalf("second lock acquisition = %v, want ErrSameProcess", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := root.Publish("after-release", []byte("x"), 0o644); !errors.Is(err, repolock.ErrReleased) {
		t.Fatalf("publish after lock release = %v, want ErrReleased", err)
	}
}

func TestBindRefusesLinksAliasesAndSpecialEntries(t *testing.T) {
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "target"), []byte("target"), 0o644)
	if err := os.Symlink("target", filepath.Join(repo, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(repo, "target"), filepath.Join(repo, "hard")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, "special"), 0o755); err != nil {
		t.Fatal(err)
	}

	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	for _, path := range []string{"link", "target", "hard", "special"} {
		if _, err := root.Bind(path); err == nil {
			t.Errorf("Bind(%q) succeeded, want refusal", path)
		}
	}
	if runtime.GOOS != "windows" {
		mustWrite(t, filepath.Join(repo, "Case"), []byte("case"), 0o644)
		if _, err := root.Bind("case"); !errors.Is(err, ErrAlias) {
			t.Errorf("case alias = %v, want ErrAlias", err)
		}
	}
}

func TestBindRefusesSymlinkedAncestorAndEscapingPaths(t *testing.T) {
	repo := t.TempDir()
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "file"), []byte("outside"), 0o644)
	if err := os.Symlink(outside, filepath.Join(repo, "linked")); err != nil {
		t.Fatal(err)
	}
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)

	for _, path := range []string{"linked/file", "../file", "/file", `linked\file`} {
		if _, err := root.Bind(path); err == nil {
			t.Errorf("Bind(%q) succeeded, want bounded-path refusal", path)
		}
	}
	got, err := os.ReadFile(filepath.Join(outside, "file"))
	if err != nil || string(got) != "outside" {
		t.Fatalf("outside file changed: bytes=%q err=%v", got, err)
	}
}

func TestRootAndRetainedAncestorIdentityMustMatchObservedName(t *testing.T) {
	t.Run("root", func(t *testing.T) {
		parent := t.TempDir()
		repo := filepath.Join(parent, "repo")
		moved := filepath.Join(parent, "moved")
		outside := filepath.Join(parent, "outside")
		for _, path := range []string{repo, outside} {
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		lock := acquireTestLock(t, repo)
		testHookBeforeRootOpen = func() {
			testHookBeforeRootOpen = nil
			if err := os.Rename(repo, moved); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, repo); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := Open(repo, lock); !errors.Is(err, ErrConflict) {
			t.Fatalf("Open after root substitution = %v, want ErrConflict", err)
		}
		if err := os.Remove(repo); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(moved, repo); err != nil {
			t.Fatal(err)
		}
		if err := lock.Release(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("ancestor", func(t *testing.T) {
		repo := t.TempDir()
		if err := os.Mkdir(filepath.Join(repo, "dir"), 0o755); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(repo, "dir", "file"), []byte("original"), 0o644)
		root, lock := openTestRoot(t, repo)
		defer releaseTestRoot(t, root, lock)
		testHookAfterDirOpen = func(part string) {
			testHookAfterDirOpen = nil
			if err := os.Rename(filepath.Join(repo, "dir"), filepath.Join(repo, "moved")); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(repo, "dir"), 0o755); err != nil {
				t.Fatal(err)
			}
			mustWrite(t, filepath.Join(repo, "dir", "file"), []byte("decoy"), 0o644)
		}
		if _, err := root.Bind("dir/file"); !errors.Is(err, ErrConflict) {
			t.Fatalf("Bind after ancestor substitution = %v, want ErrConflict", err)
		}
	})
}

func TestMutationRefusesModeAndLinkCountChanges(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, path string)
	}{
		{name: "mode", mutate: func(t *testing.T, path string) {
			if err := os.Chmod(path, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "link-count", mutate: func(t *testing.T, path string) {
			if err := os.Link(path, path+"-alias"); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := t.TempDir()
			path := filepath.Join(repo, "file")
			mustWrite(t, path, []byte("old"), 0o640)
			root, lock := openTestRoot(t, repo)
			defer releaseTestRoot(t, root, lock)
			entry, err := root.Bind("file")
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(t, path)
			if err := entry.Remove(); !errors.Is(err, ErrConflict) {
				t.Fatalf("Remove() = %v, want ErrConflict", err)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("conflicting file was removed: %v", err)
			}
		})
	}
}

func TestOperationsAreBoundedAndCompareImmediatelyBeforeMutation(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(repo, "dir", "file"), []byte("old"), 0o640)

	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	entry, err := root.Bind("dir/file")
	if err != nil {
		t.Fatal(err)
	}

	testHookBeforeMutation = func(operation, path string) {
		testHookBeforeMutation = nil
		mustWrite(t, filepath.Join(repo, "dir", "file"), []byte("external"), 0o640)
	}
	if _, err := entry.Replace([]byte("candidate"), 0o600); !errors.Is(err, ErrConflict) {
		t.Fatalf("replace after leaf substitution = %v, want ErrConflict", err)
	}
	got, err := os.ReadFile(filepath.Join(repo, "dir", "file"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "external" {
		t.Fatalf("leaf bytes = %q, want external bytes preserved", got)
	}

	entry, err = root.Bind("dir/file")
	if err != nil {
		t.Fatal(err)
	}
	testHookBeforeMutation = func(operation, path string) {
		testHookBeforeMutation = nil
		if err := os.Rename(filepath.Join(repo, "dir"), filepath.Join(repo, "moved")); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(repo, "dir"), 0o755); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(repo, "dir", "file"), []byte("decoy"), 0o640)
	}
	if err := entry.Remove(); !errors.Is(err, ErrConflict) {
		t.Fatalf("remove after ancestor substitution = %v, want ErrConflict", err)
	}
	for _, path := range []string{filepath.Join(repo, "moved", "file"), filepath.Join(repo, "dir", "file")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s was mutated: %v", path, err)
		}
	}
}

func TestCreatePublishReplaceRemoveAndMkdir(t *testing.T) {
	repo := t.TempDir()
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)

	if _, err := root.Mkdir("nested", 0o750); err != nil {
		t.Fatal(err)
	}
	entry, err := root.Publish("nested/file", []byte("one"), 0o640)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := root.Publish("nested/file", []byte("other"), 0o640); !errors.Is(err, os.ErrExist) {
		t.Fatalf("second no-replace publication = %v, want exists", err)
	}
	entry, err = entry.Replace([]byte("two"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Snapshot().Mode != 0o600 {
		t.Fatalf("mode = %o, want 600", entry.Snapshot().Mode)
	}
	if err := entry.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "nested", "file")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed file stat = %v", err)
	}
}

func TestClosedEntryReplaceRefuses(t *testing.T) {
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "file"), []byte("old"), 0o640)
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	entry, err := root.Bind("file")
	if err != nil {
		t.Fatal(err)
	}
	if err := entry.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Replace([]byte("new"), 0o640); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("Replace after Close = %v, want closed", err)
	}
}

func TestMkdirRepeatsAliasCheckAtMutationBoundary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("case-insensitive filesystems reject the alias in Mkdir itself")
	}
	repo := t.TempDir()
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	testHookBeforeMutation = func(operation, path string) {
		testHookBeforeMutation = nil
		if err := os.Mkdir(filepath.Join(repo, "Case"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := root.Mkdir("case", 0o755); !errors.Is(err, ErrAlias) {
		t.Fatalf("Mkdir after alias insertion = %v, want ErrAlias", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "case")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lower-case alias was created: %v", err)
	}
}

func TestSpecialModeChangeConflicts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix special mode bits")
	}
	repo := t.TempDir()
	path := filepath.Join(repo, "file")
	mustWrite(t, path, []byte("old"), 0o640)
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	entry, err := root.Bind("file")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640|os.ModeSetgid); err != nil {
		t.Fatal(err)
	}
	if err := entry.Remove(); !errors.Is(err, ErrConflict) {
		t.Fatalf("Remove after setgid change = %v, want ErrConflict", err)
	}
}

func TestRestartRebindUsesSemanticSnapshotAndIdentityEvidence(t *testing.T) {
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "file"), []byte("stable"), 0o640)
	root, lock := openTestRoot(t, repo)
	entry, err := root.Bind("file")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := entry.Snapshot()
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}

	root, lock = openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	if _, err := root.Rebind("file", snapshot); err != nil {
		t.Fatalf("unchanged restart rebind: %v", err)
	}
	if err := os.Rename(filepath.Join(repo, "file"), filepath.Join(repo, "old-file")); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(repo, "file"), []byte("stable"), 0o640)
	if _, err := root.Rebind("file", snapshot); !errors.Is(err, ErrConflict) {
		t.Fatalf("same semantic bytes with replaced identity = %v, want ErrConflict", err)
	}
}

func acquireTestLock(t *testing.T, repo string) *repolock.Lock {
	t.Helper()
	lock, err := repolock.Acquire(context.Background(), lockRequest(repo))
	if err != nil {
		t.Fatal(err)
	}
	return lock
}

func lockRequest(repo string) repolock.Request {
	return repolock.Request{
		Repository: repolock.Repository{Root: repo, Mode: repolock.ModeCommitted},
		Command:    "durablefs test",
		Capability: repolock.Capability{Commands: []string{"durablefs test"}},
	}
}

func openTestRoot(t *testing.T, repo string) (*Root, *repolock.Lock) {
	t.Helper()
	lock := acquireTestLock(t, repo)
	root, err := Open(repo, lock)
	if err != nil {
		lock.Release()
		t.Fatal(err)
	}
	return root, lock
}

func releaseTestRoot(t *testing.T, root *Root, lock *repolock.Lock) {
	t.Helper()
	if err := root.Close(); err != nil {
		t.Error(err)
	}
	if err := lock.Release(); err != nil {
		t.Error(err)
	}
}

func mustWrite(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatal(err)
	}
}
