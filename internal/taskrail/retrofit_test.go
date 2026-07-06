package taskrail

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// seedRetrofitRepo builds an unmarked, non-standard repository that carries a
// notes/ directory with structured human notes (a heading plus bullets), so the
// guided retrofit can both propose a layout mapping and import the notes into a
// planning bootstrap.
func seedRetrofitRepo(t *testing.T) (repo, notesRel string) {
	t.Helper()
	repo = initGitRepo(t)
	notes := "# Roadmap\n\n## Ship the importer\n\nWire structural import into retrofit.\n\n- Add a login flow\n- Add a logout flow\n"
	writeFile(t, filepath.Join(repo, "notes", "ideas.md"), notes)
	return repo, filepath.Join("notes", "ideas.md")
}

func TestRetrofitDryRunImportsNotesAndProposesLayout(t *testing.T) {
	t.Parallel()

	repo, notes := seedRetrofitRepo(t)
	before := snapshotTree(t, repo)

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	result, err := svc.Retrofit(RetrofitInput{NotesPath: notes})
	if err != nil {
		t.Fatalf("retrofit dry run: %v", err)
	}
	if result.Applied {
		t.Fatal("retrofit dry run must not apply changes")
	}

	wantMapping := []RetrofitMapping{{Source: "notes", Target: "planning", Role: "planning"}}
	if !reflect.DeepEqual(result.Mapping, wantMapping) {
		t.Fatalf("mapping = %+v, want %+v", result.Mapping, wantMapping)
	}
	if result.Bootstrap == nil {
		t.Fatal("retrofit with notes must produce a planning bootstrap")
	}
	if got := len(result.Bootstrap.Draft.Tasks); got == 0 {
		t.Fatal("bootstrap must carry task drafts parsed from the notes")
	}
	if result.Bootstrap.StateSeed == "" {
		t.Fatal("planning bootstrap must carry a state seed")
	}
	if result.Validation != nil {
		t.Fatal("dry run must not report a validation result")
	}
	// The proposal must name the tracked structure it would create.
	if !changesMention(result.Changes, "STATE.md") {
		t.Fatalf("changes must propose STATE.md, got %v", result.Changes)
	}

	after := snapshotTree(t, repo)
	if added := addedPaths(before, after); len(added) != 0 {
		t.Fatalf("dry run added files: %v", added)
	}
	if _, present, err := readMarker(repo); err != nil || present {
		t.Fatalf("dry run must not write marker: present=%v err=%v", present, err)
	}
}

func TestRetrofitApplyScaffoldsValidatesAndPreservesNotes(t *testing.T) {
	t.Parallel()

	repo, notes := seedRetrofitRepo(t)
	notesAbs := filepath.Join(repo, notes)
	notesBefore, err := os.ReadFile(notesAbs)
	if err != nil {
		t.Fatalf("read notes: %v", err)
	}

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	result, err := svc.Retrofit(RetrofitInput{NotesPath: notes, Apply: true})
	if err != nil {
		t.Fatalf("retrofit apply: %v", err)
	}
	if !result.Applied {
		t.Fatal("apply must report applied")
	}
	if result.Validation == nil || !result.Validation.Valid {
		t.Fatalf("expected valid post-retrofit validation, got %+v", result.Validation)
	}
	if result.Bootstrap == nil {
		t.Fatal("apply must still surface the planning bootstrap")
	}

	for _, rel := range []string{
		filepath.Join("planning", "STATE.md"),
		filepath.Join("specs", "README.md"),
		filepath.Join("planning", "tasks"),
	} {
		if _, err := os.Stat(filepath.Join(repo, rel)); err != nil {
			t.Fatalf("apply did not scaffold %s: %v", rel, err)
		}
	}
	if _, present, err := readMarker(repo); err != nil || !present {
		t.Fatalf("apply must write marker: present=%v err=%v", present, err)
	}
	// No-clobber: the human notes file is only read, never moved or rewritten.
	notesAfter, err := os.ReadFile(notesAbs)
	if err != nil || string(notesAfter) != string(notesBefore) {
		t.Fatalf("retrofit moved or rewrote notes content: got %q err=%v", string(notesAfter), err)
	}
}

func TestRetrofitApplyNeverClobbersExistingState(t *testing.T) {
	t.Parallel()

	repo, notes := seedRetrofitRepo(t)
	// A human already hand-authored a STATE.md; retrofit must not overwrite it.
	existing := "---\nschema_version: 1\n---\n\n# HUMAN STATE\n"
	writeFile(t, filepath.Join(repo, "planning", "STATE.md"), existing)

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	result, err := svc.Retrofit(RetrofitInput{NotesPath: notes, Apply: true})
	if err != nil {
		t.Fatalf("retrofit apply: %v", err)
	}
	if !result.Applied {
		t.Fatal("apply must report applied")
	}
	// The seeded STATE.md is intentionally incomplete, so post-apply validation
	// must report invalid: that proves retrofit left it untouched rather than
	// regenerating a fresh valid one (which would silently mask a clobber).
	if result.Validation == nil || result.Validation.Valid {
		t.Fatalf("expected validation to stay invalid over the human STATE.md, got %+v", result.Validation)
	}
	got, err := os.ReadFile(filepath.Join(repo, "planning", "STATE.md"))
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if string(got) != existing {
		t.Fatal("retrofit overwrote a human-authored STATE.md")
	}
}

func TestRetrofitWithoutNotesScaffoldsWithoutBootstrap(t *testing.T) {
	t.Parallel()

	repo, _ := seedRetrofitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	result, err := svc.Retrofit(RetrofitInput{Apply: true})
	if err != nil {
		t.Fatalf("retrofit apply: %v", err)
	}
	if result.Bootstrap != nil {
		t.Fatal("retrofit without notes must not produce a bootstrap")
	}
	if result.Validation == nil || !result.Validation.Valid {
		t.Fatalf("expected valid validation, got %+v", result.Validation)
	}
}

func TestRetrofitRefusesManagedRepo(t *testing.T) {
	t.Parallel()

	repo, notes := seedRetrofitRepo(t)
	if err := writeMarker(repo, defaultLayoutConfig()); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Retrofit(RetrofitInput{NotesPath: notes}); err == nil {
		t.Fatal("retrofit must refuse an already-managed repository")
	}
}

func TestRetrofitRejectsMissingNotes(t *testing.T) {
	t.Parallel()

	repo, _ := seedRetrofitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Retrofit(RetrofitInput{NotesPath: "notes/missing.md"}); err == nil {
		t.Fatal("retrofit must surface an unreadable notes path")
	}
}

func changesMention(items []string, sub string) bool {
	for _, item := range items {
		if strings.Contains(item, sub) {
			return true
		}
	}
	return false
}
