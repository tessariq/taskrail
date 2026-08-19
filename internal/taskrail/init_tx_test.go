package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tessariq/taskrail/internal/repolock"
)

func TestInitApplyHoldsLockAndRefusesConcurrentDestinationEdit(t *testing.T) {
	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC))
	external := []byte("external marker\n")
	var lockObserved bool
	testHookInitValidated = func() {
		status, err := repolock.Inspect(svc.paths.LockRepository())
		if err != nil {
			t.Fatalf("inspect init lock: %v", err)
		}
		lockObserved = status.Held && status.Owner != nil && status.Owner.Command == "init"
		if err := os.MkdirAll(filepath.Dir(markerFile(repo)), 0o755); err != nil {
			t.Fatalf("mkdir marker parent: %v", err)
		}
		if err := os.WriteFile(markerFile(repo), external, 0o644); err != nil {
			t.Fatalf("write concurrent marker: %v", err)
		}
	}
	t.Cleanup(func() { testHookInitValidated = nil })

	_, err := svc.Init(InitInput{})
	if err == nil || !strings.Contains(err.Error(), "changed since the transaction snapshot") {
		t.Fatalf("Init error = %v, want write conflict", err)
	}
	if !lockObserved {
		t.Fatal("init candidate validation did not hold the repository lock")
	}
	got, readErr := os.ReadFile(markerFile(repo))
	if readErr != nil || string(got) != string(external) {
		t.Fatalf("concurrent marker = %q, err=%v; want preserved", got, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "planning", "STATE.md")); !os.IsNotExist(statErr) {
		t.Fatalf("failed init published state: %v", statErr)
	}
}

func TestRetrofitApplySnapshotsNotesSource(t *testing.T) {
	repo, notes := seedRetrofitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC))
	notesPath := filepath.Join(repo, notes)
	testHookInitValidated = func() {
		if err := os.WriteFile(notesPath, []byte("# changed concurrently\n"), 0o644); err != nil {
			t.Fatalf("write concurrent notes: %v", err)
		}
	}
	t.Cleanup(func() { testHookInitValidated = nil })

	_, err := svc.Retrofit(RetrofitInput{NotesPath: notes, Apply: true})
	if err == nil || !strings.Contains(err.Error(), "changed since the transaction snapshot") {
		t.Fatalf("Retrofit error = %v, want source conflict", err)
	}
	if _, statErr := os.Stat(markerFile(repo)); !os.IsNotExist(statErr) {
		t.Fatalf("failed retrofit published marker: %v", statErr)
	}
}

func TestInitAndRetrofitPreviewsCreateNoLockArtifacts(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*Service) error
	}{
		{name: "init", run: func(s *Service) error { _, err := s.Init(InitInput{}); return err }},
		{name: "retrofit", run: func(s *Service) error { _, err := s.Retrofit(RetrofitInput{}); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := seedNonStandardRepo(t)
			svc := newTestService(t, repo, time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC))
			before := snapshotTree(t, repo)
			if err := tc.run(svc); err != nil {
				t.Fatalf("preview: %v", err)
			}
			after := snapshotTree(t, repo)
			if len(before) != len(after) {
				t.Fatalf("preview changed tree: before=%v after=%v", before, after)
			}
			for path, content := range before {
				if after[path] != content {
					t.Fatalf("preview changed %s", path)
				}
			}
			if status, err := repolock.Inspect(svc.paths.LockRepository()); err != nil || status.Held {
				t.Fatalf("preview lock status = %+v, err=%v", status, err)
			}
		})
	}
}

func TestRetrofitPreviewRefusesUnstableSource(t *testing.T) {
	repo, notes := seedRetrofitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC))
	testHookInitPreviewBuilt = func() {
		if err := os.WriteFile(filepath.Join(repo, notes), []byte("# changed concurrently\n"), 0o644); err != nil {
			t.Fatalf("write concurrent notes: %v", err)
		}
	}
	t.Cleanup(func() { testHookInitPreviewBuilt = nil })

	_, err := svc.Retrofit(RetrofitInput{NotesPath: notes})
	if err == nil || MachineFailureFor(err).Code != MachineCodeSourceChanged {
		t.Fatalf("Retrofit error = %v, want source_changed", err)
	}
	if status, inspectErr := repolock.Inspect(svc.paths.LockRepository()); inspectErr != nil || status.Held {
		t.Fatalf("unstable preview lock status = %+v, err=%v", status, inspectErr)
	}
}

func TestInitCandidateObservationRefusesLaterDestination(t *testing.T) {
	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC))
	before := snapshotTree(t, repo)
	externalState := []byte("external state\n")
	testHookInitPathObserved = func(logical string) {
		if logical != "planning/STATE.md" {
			return
		}
		if err := os.MkdirAll(filepath.Join(repo, "planning"), 0o755); err != nil {
			t.Fatalf("mkdir planning: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repo, logical), externalState, 0o644); err != nil {
			t.Fatalf("write concurrent state: %v", err)
		}
	}
	t.Cleanup(func() { testHookInitPathObserved = nil })

	_, err := svc.Init(InitInput{})
	if err == nil || MachineFailureFor(err).Code != MachineCodeValidationFailed {
		t.Fatalf("Init error = %v, want validation_failed conflict", err)
	}
	after := snapshotTree(t, repo)
	if got := after[filepath.Join("planning", "STATE.md")]; got != string(externalState) {
		t.Fatalf("external state = %q, want preserved", got)
	}
	if added := addedPaths(before, after); len(added) != 1 || added[0] != filepath.Join("planning", "STATE.md") {
		t.Fatalf("failed candidate published files: %v", added)
	}
}

func TestRetrofitApplyRefusesNotesOutsideRepository(t *testing.T) {
	repo, _ := seedRetrofitRepo(t)
	external := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(external, []byte("# External notes\n"), 0o644); err != nil {
		t.Fatalf("write external notes: %v", err)
	}
	svc := newTestService(t, repo, time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC))

	if _, err := svc.Retrofit(RetrofitInput{NotesPath: external}); err == nil || MachineFailureFor(err).Code != MachineCodePathBlocked {
		t.Fatalf("Retrofit preview error = %v, want path_blocked", err)
	}
	_, err := svc.Retrofit(RetrofitInput{NotesPath: external, Apply: true})
	if err == nil || MachineFailureFor(err).Code != MachineCodePathBlocked {
		t.Fatalf("Retrofit error = %v, want path_blocked", err)
	}
	if _, statErr := os.Stat(markerFile(repo)); !os.IsNotExist(statErr) {
		t.Fatalf("refused retrofit published marker: %v", statErr)
	}
}

func TestLayout2PreviewRefusesUnstableSource(t *testing.T) {
	repo := seedLayout1Repo(t)
	testHookInitPreviewBuilt = func() {
		if err := os.WriteFile(filepath.Join(repo, "specs", "v0.1.0.md"), []byte("# changed concurrently\n"), 0o644); err != nil {
			t.Fatalf("write concurrent spec: %v", err)
		}
	}
	t.Cleanup(func() { testHookInitPreviewBuilt = nil })

	_, err := layout1Service(t, repo).Init(InitInput{})
	if err == nil || MachineFailureFor(err).Code != MachineCodeSourceChanged {
		t.Fatalf("Init error = %v, want source_changed", err)
	}
}
