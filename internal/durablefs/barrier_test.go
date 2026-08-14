package durablefs

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPublicationRunsOrderedDurabilityBarriers(t *testing.T) {
	repo := t.TempDir()
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)

	var events []string
	testHookBarrier = func(step Barrier) error {
		events = append(events, string(step))
		return nil
	}
	defer func() { testHookBarrier = nil }()
	if _, err := root.Publish("file", []byte("data"), 0o640); err != nil {
		t.Fatal(err)
	}
	want := []string{string(BarrierContent), string(BarrierMetadata), string(BarrierDirectory)}
	if !slices.Equal(events, want) {
		t.Fatalf("barriers = %v, want %v", events, want)
	}
}

func TestBarrierFailureNeverReportsSuccess(t *testing.T) {
	for _, failed := range []Barrier{BarrierContent, BarrierMetadata, BarrierDirectory} {
		t.Run(string(failed), func(t *testing.T) {
			repo := t.TempDir()
			root, lock := openTestRoot(t, repo)
			defer releaseTestRoot(t, root, lock)
			testHookBarrier = func(step Barrier) error {
				if step == failed {
					return errors.New("injected sync failure")
				}
				return nil
			}
			defer func() { testHookBarrier = nil }()
			_, err := root.Publish("file", []byte("data"), 0o640)
			if err == nil {
				t.Fatal("Publish succeeded after injected barrier failure")
			}
			if failed != BarrierDirectory {
				if _, statErr := os.Stat(filepath.Join(repo, "file")); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("pre-publication failure left destination: %v", statErr)
				}
			}
		})
	}
}

func TestPrecommitConflictRemovesPrivateStage(t *testing.T) {
	repo := t.TempDir()
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	testHookBeforeMutation = func(operation, path string) {
		testHookBeforeMutation = nil
		mustWrite(t, filepath.Join(repo, path), []byte("external"), 0o644)
	}
	if _, err := root.Publish("file", []byte("candidate"), 0o644); err == nil {
		t.Fatal("Publish succeeded after destination conflict")
	}
	entries, err := os.ReadDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".taskrail-durable-") {
			t.Fatalf("private staging entry remains after conflict: %s", entry.Name())
		}
	}
}

func TestStagedSourceSubstitutionRefuses(t *testing.T) {
	repo := t.TempDir()
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	testHookBeforeStageCAS = func(parent *os.Root, staged string) {
		testHookBeforeStageCAS = nil
		if err := parent.Remove(staged); err != nil {
			t.Fatal(err)
		}
		file, err := parent.OpenFile(staged, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte("substitute")); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := root.Publish("file", []byte("candidate"), 0o644); !errors.Is(err, ErrConflict) {
		t.Fatalf("Publish after staged substitution = %v, want ErrConflict", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "file")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("substituted stage was published: %v", err)
	}
}

func TestPostcommitCleanupFailureReportsStaging(t *testing.T) {
	repo := t.TempDir()
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	testHookRemoveStage = func(parent *os.Root, staged string) error {
		testHookRemoveStage = nil
		return errors.New("injected cleanup failure")
	}
	_, err := root.Publish("file", []byte("candidate"), 0o644)
	var mutation *MutationError
	if !errors.As(err, &mutation) || !mutation.Committed || mutation.Staging == "" {
		t.Fatalf("cleanup error = %#v, want committed MutationError with staging", err)
	}
}

func TestLinkFailureAfterClosedStageDoesNotInventCleanupAmbiguity(t *testing.T) {
	repo := t.TempDir()
	root, lock := openTestRoot(t, repo)
	defer releaseTestRoot(t, root, lock)
	testHookBeforeLink = func(parent *os.Root, leaf string) {
		testHookBeforeLink = nil
		file, err := parent.OpenFile(leaf, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	_, err := root.Publish("file", []byte("candidate"), 0o644)
	var mutation *MutationError
	if err == nil || errors.As(err, &mutation) {
		t.Fatalf("link race error = %#v, want ordinary link refusal with clean stage removal", err)
	}
	entries, readErr := os.ReadDir(repo)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".taskrail-durable-") {
			t.Fatalf("staging entry remains: %s", entry.Name())
		}
	}
}

func TestPostcommitInspectionAndCloseFailuresAreClassified(t *testing.T) {
	t.Run("mkdir-inspection", func(t *testing.T) {
		repo := t.TempDir()
		root, lock := openTestRoot(t, repo)
		defer releaseTestRoot(t, root, lock)
		testHookAfterCommit = func(operation, path string) {
			testHookAfterCommit = nil
			if err := os.Remove(filepath.Join(repo, path)); err != nil {
				t.Fatal(err)
			}
		}
		_, err := root.Mkdir("dir", 0o755)
		var mutation *MutationError
		if !errors.As(err, &mutation) || !mutation.Committed {
			t.Fatalf("mkdir inspection error = %#v, want committed MutationError", err)
		}
	})

	t.Run("remove-close", func(t *testing.T) {
		repo := t.TempDir()
		mustWrite(t, filepath.Join(repo, "file"), []byte("data"), 0o644)
		root, lock := openTestRoot(t, repo)
		defer releaseTestRoot(t, root, lock)
		entry, err := root.Bind("file")
		if err != nil {
			t.Fatal(err)
		}
		testHookEntryClose = func(*os.Root) error {
			testHookEntryClose = nil
			return errors.New("injected close failure")
		}
		err = entry.Remove()
		var mutation *MutationError
		if !errors.As(err, &mutation) || !mutation.Committed {
			t.Fatalf("remove close error = %#v, want committed MutationError", err)
		}
	})
}
