package durablefs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/repolock"
)

func TestObserveTreeReturnsStableMembershipAndBytes(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "runtime", "transactions", "tx"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(base, "runtime", "transactions", "tx", "journal.json"), []byte("one"), 0o600)

	first, err := ObserveTree(base, "runtime/transactions")
	if err != nil {
		t.Fatal(err)
	}
	if !first.Present || len(first.Entries) != 2 || first.Entries[1].Snapshot.SHA256 == "" {
		t.Fatalf("snapshot = %+v", first)
	}
	content, snapshot, err := ReadFile(base, "runtime/transactions/tx/journal.json", 16)
	if err != nil || string(content) != "one" || snapshot != first.Entries[1].Snapshot {
		t.Fatalf("stable file read: content=%q snapshot=%+v err=%v", content, snapshot, err)
	}
	if _, _, err := ReadFile(base, "runtime/transactions/tx/journal.json", 2); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("bounded file read = %v, want ErrUnsupported", err)
	}
	second, err := ObserveTree(base, "runtime/transactions")
	if err != nil || !first.Same(second) {
		t.Fatalf("unchanged snapshot differs: same=%t err=%v", first.Same(second), err)
	}
	mustWrite(t, filepath.Join(base, "runtime", "transactions", "tx", "journal.json"), []byte("two"), 0o600)
	changed, err := ObserveTree(base, "runtime/transactions")
	if err != nil || first.Same(changed) {
		t.Fatalf("changed snapshot not detected: same=%t err=%v", first.Same(changed), err)
	}
}

func TestObserveTreeRefusesAliasesLinksAndSpecialEntries(t *testing.T) {
	for _, test := range []struct {
		name  string
		build func(*testing.T, string)
	}{
		{name: "linked ancestor", build: func(t *testing.T, base string) {
			outside := t.TempDir()
			if err := os.Symlink(outside, filepath.Join(base, "runtime")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "linked member", build: func(t *testing.T, base string) {
			if err := os.MkdirAll(filepath.Join(base, "runtime", "transactions"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(t.TempDir(), filepath.Join(base, "runtime", "transactions", "tx")); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := t.TempDir()
			test.build(t, base)
			if _, err := ObserveTree(base, "runtime/transactions"); err == nil {
				t.Fatal("ObserveTree succeeded, want refusal")
			}
		})
	}

	if runtime.GOOS != "windows" {
		base := t.TempDir()
		if err := os.MkdirAll(filepath.Join(base, "runtime", "transactions", "Case"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(base, "runtime", "transactions", "case"), 0o755); errors.Is(err, os.ErrExist) {
			t.Skip("case-insensitive filesystem prevents creating a distinct alias")
		} else if err != nil {
			t.Fatal(err)
		}
		_, err := ObserveTree(base, "runtime/transactions")
		if !errors.Is(err, ErrAlias) || !strings.Contains(err.Error(), "Case") {
			t.Fatalf("alias error = %v", err)
		}
	}
}

func TestObserveTreeDetectsReplacementDuringInspection(t *testing.T) {
	base := t.TempDir()
	transactions := filepath.Join(base, "runtime", "transactions")
	if err := os.MkdirAll(filepath.Join(transactions, "tx"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(transactions, "tx", "journal.json"), []byte("one"), 0o600)
	testHookObserveTree = func() {
		testHookObserveTree = nil
		if err := os.Rename(transactions, transactions+"-old"); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(transactions, "tx"), 0o755); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(transactions, "tx", "journal.json"), []byte("two"), 0o600)
	}
	t.Cleanup(func() { testHookObserveTree = nil })
	if _, err := ObserveTree(base, "runtime/transactions"); !errors.Is(err, ErrConflict) {
		t.Fatalf("replacement error = %v, want ErrConflict", err)
	}
}

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
			if runtime.GOOS == "windows" && test.name == "mode" {
				t.Skip("Windows does not expose Unix permission changes")
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
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports directory durability as unsupported")
	}
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
	if entry.Snapshot().Mode != PortableMode(0o600) {
		t.Fatalf("mode = %o, want %o", entry.Snapshot().Mode, PortableMode(0o600))
	}
	if err := entry.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "nested", "file")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed file stat = %v", err)
	}
}

func TestRemoveDirExpectedRefusesReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports directory durability as unsupported")
	}
	repo := t.TempDir()
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	directory, err := root.Mkdir("session", 0o755)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, "session")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := root.Mkdir("allocation", 0o755); err != nil {
		t.Fatal(err)
	}
	replacement, err := root.Mkdir("session", 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if replacement.Identity == directory.Identity {
		t.Skip("filesystem reused the original directory identity")
	}
	if err := root.RemoveDirExpected("session", directory.Identity); !errors.Is(err, ErrConflict) {
		t.Fatalf("RemoveDirExpected = %v, want identity conflict", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("replacement directory was removed: %v", err)
	}
}

func TestPublishDirectoryCommitsCompleteBytesAtOneAbsentName(t *testing.T) {
	requireDirectoryPublication(t)
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, "reviews"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)

	directory, err := root.PublishDirectory(context.Background(), "reviews/session", []DirectoryFile{
		{Name: "a.json", Content: []byte("a"), Mode: 0o640},
		{Name: "b.json", Content: []byte("b"), Mode: 0o600},
	})
	if err != nil {
		t.Fatal(err)
	}
	if directory.Path != "reviews/session" {
		t.Fatalf("directory = %+v", directory)
	}
	for name, want := range map[string]string{"a.json": "a", "b.json": "b"} {
		got, readErr := os.ReadFile(filepath.Join(repo, "reviews", "session", name))
		if readErr != nil || string(got) != want {
			t.Fatalf("%s = %q, err=%v", name, got, readErr)
		}
	}
}

func TestPublishDirectoryLosesDestinationRaceWithoutClobbering(t *testing.T) {
	requireDirectoryPublication(t)
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, "reviews"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	testHookBeforeDirectoryCommit = func(_, path string) {
		testHookBeforeDirectoryCommit = nil
		if err := os.Mkdir(filepath.Join(repo, filepath.FromSlash(path)), 0o755); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(repo, filepath.FromSlash(path), "winner"), []byte("external"), 0o644)
	}
	t.Cleanup(func() { testHookBeforeDirectoryCommit = nil })

	_, err := root.PublishDirectory(context.Background(), "reviews/session", []DirectoryFile{{Name: "review.json", Content: []byte("candidate"), Mode: 0o644}})
	if err == nil {
		t.Fatal("PublishDirectory succeeded after destination race")
	}
	got, readErr := os.ReadFile(filepath.Join(repo, "reviews", "session", "winner"))
	if readErr != nil || string(got) != "external" {
		t.Fatalf("winner bytes = %q, err=%v", got, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "reviews", "session", "review.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("candidate merged into winner: %v", statErr)
	}
}

func TestDirectoryNoReplaceCommitHasOneConcurrentWinner(t *testing.T) {
	repo := t.TempDir()
	for _, candidate := range []string{"one", "two"} {
		if err := os.Mkdir(filepath.Join(repo, candidate), 0o755); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(repo, candidate, "review.json"), []byte(candidate), 0o644)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, candidate := range []string{"one", "two"} {
		go func() {
			parent, err := os.OpenRoot(repo)
			if err != nil {
				results <- err
				return
			}
			defer parent.Close()
			<-start
			results <- moveDirectoryNoReplace(parent, candidate, parent, "winner")
		}()
	}
	close(start)
	var succeeded int
	for range 2 {
		if err := <-results; err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("successful commits = %d, want 1", succeeded)
	}
	got, err := os.ReadFile(filepath.Join(repo, "winner", "review.json"))
	if err != nil || (string(got) != "one" && string(got) != "two") {
		t.Fatalf("winner bytes = %q, err=%v", got, err)
	}
}

func TestPublishDirectoryStagingFailureLeavesNoFinalDirectory(t *testing.T) {
	requireDirectoryPublication(t)
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, "reviews"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	testHookBeforeDirectoryFile = func(name string) error {
		if name == "b.json" {
			return errors.New("injected staging failure")
		}
		return nil
	}
	t.Cleanup(func() { testHookBeforeDirectoryFile = nil })

	_, err := root.PublishDirectory(context.Background(), "reviews/session", []DirectoryFile{
		{Name: "a.json", Content: []byte("a"), Mode: 0o644},
		{Name: "b.json", Content: []byte("b"), Mode: 0o644},
	})
	if err == nil {
		t.Fatal("PublishDirectory succeeded")
	}
	if _, statErr := os.Stat(filepath.Join(repo, "reviews", "session")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("final directory exists: %v", statErr)
	}
	entries, readErr := os.ReadDir(filepath.Join(repo, "reviews"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".taskrail-durable-") {
			t.Fatalf("staging directory remains: %s", entry.Name())
		}
	}
}

func TestPublishDirectoryRefusesStagedByteAndMembershipChanges(t *testing.T) {
	requireDirectoryPublication(t)
	for _, test := range []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{name: "bytes", mutate: func(t *testing.T, staged string) {
			mustWrite(t, filepath.Join(staged, "review.json"), []byte("changed"), 0o644)
		}},
		{name: "member", mutate: func(t *testing.T, staged string) {
			mustWrite(t, filepath.Join(staged, "extra.json"), []byte("extra"), 0o644)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := t.TempDir()
			if err := os.Mkdir(filepath.Join(repo, "reviews"), 0o755); err != nil {
				t.Fatal(err)
			}
			root, lock := openTestRoot(t, repo)
			defer releaseTestRoot(t, root, lock)
			testHookBeforeDirectoryCommit = func(staged, _ string) {
				testHookBeforeDirectoryCommit = nil
				test.mutate(t, filepath.Join(repo, filepath.FromSlash(staged)))
			}
			t.Cleanup(func() { testHookBeforeDirectoryCommit = nil })
			_, err := root.PublishDirectory(context.Background(), "reviews/session", []DirectoryFile{{Name: "review.json", Content: []byte("candidate"), Mode: 0o644}})
			if !errors.Is(err, ErrConflict) {
				t.Fatalf("PublishDirectory = %v, want ErrConflict", err)
			}
			if _, statErr := os.Stat(filepath.Join(repo, "reviews", "session")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("final directory exists: %v", statErr)
			}
		})
	}
}

func TestPublishDirectoryRefusesStageSubstitutionAtCommit(t *testing.T) {
	requireDirectoryPublication(t)
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, "reviews"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	testHookBeforeDirectoryMove = func(staged, _ string) {
		testHookBeforeDirectoryMove = nil
		physical := filepath.Join(repo, filepath.FromSlash(staged))
		if err := os.Rename(physical, physical+"-original"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(physical, 0o755); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(physical, "review.json"), []byte("external"), 0o644)
	}
	t.Cleanup(func() { testHookBeforeDirectoryMove = nil })
	_, err := root.PublishDirectory(context.Background(), "reviews/session", []DirectoryFile{{Name: "review.json", Content: []byte("candidate"), Mode: 0o644}})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("PublishDirectory = %v, want ErrConflict", err)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "reviews", "session")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("final directory exists: %v", statErr)
	}
	var substitute string
	entries, readErr := os.ReadDir(filepath.Join(repo, "reviews"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".taskrail-durable-") && !strings.HasSuffix(entry.Name(), "-original") {
			substitute = filepath.Join(repo, "reviews", entry.Name(), "review.json")
		}
	}
	got, readErr := os.ReadFile(substitute)
	if readErr != nil || string(got) != "external" {
		t.Fatalf("substituted stage bytes = %q, err=%v", got, readErr)
	}
}

func TestPublishDirectoryRollsBackStageSubstitutionAfterFinalCheck(t *testing.T) {
	requireDirectoryPublication(t)
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, "reviews"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	var substitute string
	testHookAfterDirectoryCheck = func(staged, _ string) {
		testHookAfterDirectoryCheck = nil
		physical := filepath.Join(repo, filepath.FromSlash(staged))
		if err := os.Rename(physical, physical+"-candidate"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(physical, 0o755); err != nil {
			t.Fatal(err)
		}
		substitute = filepath.Join(physical, "review.json")
		mustWrite(t, substitute, []byte("external"), 0o644)
	}
	t.Cleanup(func() { testHookAfterDirectoryCheck = nil })

	_, err := root.PublishDirectory(context.Background(), "reviews/session", []DirectoryFile{{Name: "review.json", Content: []byte("candidate"), Mode: 0o644}})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("PublishDirectory = %v, want ErrConflict", err)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "reviews", "session")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("final directory exists: %v", statErr)
	}
	got, readErr := os.ReadFile(substitute)
	if readErr != nil || string(got) != "external" {
		t.Fatalf("substituted bytes = %q, err=%v", got, readErr)
	}
}

func TestPublishDirectoryRollsBackAliasCreatedAtCommit(t *testing.T) {
	requireDirectoryPublication(t)
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, "reviews"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	testHookAfterDirectoryCheck = func(_, _ string) {
		testHookAfterDirectoryCheck = nil
		if err := os.Mkdir(filepath.Join(repo, "reviews", "Session"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { testHookAfterDirectoryCheck = nil })

	_, err := root.PublishDirectory(context.Background(), "reviews/session", []DirectoryFile{{Name: "review.json", Content: []byte("candidate"), Mode: 0o644}})
	if !errors.Is(err, ErrAlias) && !errors.Is(err, os.ErrExist) {
		t.Fatalf("PublishDirectory = %v, want alias or native no-replace refusal", err)
	}
	entries, readErr := os.ReadDir(filepath.Join(repo, "reviews"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	aliasPreserved := false
	for _, entry := range entries {
		if entry.Name() == "session" {
			t.Fatal("exact final directory exists")
		}
		if entry.Name() == "Session" {
			aliasPreserved = true
		}
	}
	if !aliasPreserved {
		t.Fatal("external alias was not preserved")
	}
}

func TestPublishDirectoryCleanupPreservesSubstitutedStage(t *testing.T) {
	requireDirectoryPublication(t)
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, "reviews"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	var substitute string
	testHookBeforeDirectoryCommit = func(staged, _ string) {
		testHookBeforeDirectoryCommit = nil
		physical := filepath.Join(repo, filepath.FromSlash(staged))
		if err := os.Rename(physical, physical+"-original"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(physical, 0o755); err != nil {
			t.Fatal(err)
		}
		substitute = filepath.Join(physical, "review.json")
		mustWrite(t, substitute, []byte("external"), 0o644)
	}
	t.Cleanup(func() { testHookBeforeDirectoryCommit = nil })
	_, err := root.PublishDirectory(context.Background(), "reviews/session", []DirectoryFile{{Name: "review.json", Content: []byte("candidate"), Mode: 0o644}})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("PublishDirectory = %v, want ErrConflict", err)
	}
	got, readErr := os.ReadFile(substitute)
	if readErr != nil || string(got) != "external" {
		t.Fatalf("substituted stage bytes = %q, err=%v", got, readErr)
	}
}

func TestPublishDirectoryPostcommitSubstitutionDoesNotReportSuccess(t *testing.T) {
	requireDirectoryPublication(t)
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, "reviews"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	directoryBarriers := 0
	testHookBarrier = func(step Barrier) error {
		if step != BarrierDirectory {
			return nil
		}
		directoryBarriers++
		if directoryBarriers == 3 {
			final := filepath.Join(repo, "reviews", "session")
			if err := os.Rename(final, final+"-candidate"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(final, 0o755); err != nil {
				t.Fatal(err)
			}
			mustWrite(t, filepath.Join(final, "review.json"), []byte("external"), 0o644)
		}
		return nil
	}
	t.Cleanup(func() { testHookBarrier = nil })
	_, err := root.PublishDirectory(context.Background(), "reviews/session", []DirectoryFile{{Name: "review.json", Content: []byte("candidate"), Mode: 0o644}})
	if err == nil {
		t.Fatal("PublishDirectory reported success after final substitution")
	}
	if _, statErr := os.Stat(filepath.Join(repo, "reviews", "session")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("final directory exists: %v", statErr)
	}
	entries, readErr := os.ReadDir(filepath.Join(repo, "reviews"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	preserved := false
	for _, entry := range entries {
		got, err := os.ReadFile(filepath.Join(repo, "reviews", entry.Name(), "review.json"))
		if err == nil && string(got) == "external" {
			preserved = true
		}
	}
	if !preserved {
		t.Fatal("external final bytes were not preserved after rollback")
	}
}

func requireDirectoryPublication(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports directory durability as unsupported")
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
	entries, err := os.ReadDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == "case" {
			t.Fatal("lower-case alias was created")
		}
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

func TestMoveDirAtomicallyClearsSourceNamespace(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "transactions", "tx"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, "completed"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(repo, "transactions", "tx", "journal.json"), []byte("journal"), 0o644)
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	if err := root.MoveDir("transactions/tx", "completed/tx"); err != nil {
		var mutation *MutationError
		if !errors.As(err, &mutation) || !mutation.Committed || !errors.Is(err, ErrUnsupported) {
			t.Fatalf("MoveDir: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, "transactions", "tx")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source remains: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(repo, "completed", "tx", "journal.json"))
	if err != nil || string(data) != "journal" {
		t.Fatalf("destination bytes = %q, err = %v", data, err)
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
	t.Cleanup(resetTestHooks)
	lock := acquireTestLock(t, repo)
	root, err := Open(repo, lock)
	if err != nil {
		lock.Release()
		t.Fatal(err)
	}
	return root, lock
}

func resetTestHooks() {
	testHookBeforeMutation = nil
	testHookBarrier = nil
	testHookBeforeRootOpen = nil
	testHookAfterDirOpen = nil
	testHookBeforeStageCAS = nil
	testHookRemoveStage = nil
	testHookBeforeLink = nil
	testHookAfterCommit = nil
	testHookEntryClose = nil
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
