package taskrail

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func markerFile(repo string) string {
	return filepath.Join(repo, taskrailConfigDir, taskrailConfigFile)
}

// snapshotTree returns every regular-file path (repo-relative) so tests can
// assert that a run added or changed exactly the files they expect.
func snapshotTree(t *testing.T, repo string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.Walk(repo, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(repo, path)
		if relErr != nil {
			return relErr
		}
		files[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
	return files
}

func TestInitEmptyRepoWritesMarker(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	result, err := svc.Init(false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if result.Outcome != InitCreated {
		t.Fatalf("outcome = %q, want %q", result.Outcome, InitCreated)
	}

	cfg, present, err := readMarker(repo)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if !present {
		t.Fatal("expected marker to exist after fresh init")
	}
	if cfg.LayoutVersion != currentLayoutVersion {
		t.Fatalf("layout_version = %d, want %d", cfg.LayoutVersion, currentLayoutVersion)
	}

	validation, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !validation.Valid {
		t.Fatalf("expected valid repo, got %v", validation.Violations)
	}
}

func TestInitAdoptsLegacyLayoutNonDestructively(t *testing.T) {
	t.Parallel()

	// seedFixtureRepo builds a complete v0.1.0 layout with no marker.
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "Human task", "todo", "high", "specs/v0.1.0.md#summary", nil)
	before := snapshotTree(t, repo)

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	result, err := svc.Init(false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if result.Outcome != InitAdopted {
		t.Fatalf("outcome = %q, want %q", result.Outcome, InitAdopted)
	}

	after := snapshotTree(t, repo)
	added := addedPaths(before, after)
	if len(added) != 1 || added[0] != filepath.Join(taskrailConfigDir, taskrailConfigFile) {
		t.Fatalf("adoption changed files other than the marker: added=%v", added)
	}
	for path, content := range before {
		if after[path] != content {
			t.Fatalf("adoption rewrote human-authored file %s", path)
		}
	}

	cfg, present, err := readMarker(repo)
	if err != nil || !present {
		t.Fatalf("read marker: present=%v err=%v", present, err)
	}
	if cfg.LayoutVersion != currentLayoutVersion {
		t.Fatalf("layout_version = %d, want %d", cfg.LayoutVersion, currentLayoutVersion)
	}
}

func TestInitMigrationDryRunChangesNothing(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeFile(t, markerFile(repo), "layout_version: 0\nspecs_dir: specs\nplanning_dir: planning\n")
	before := snapshotTree(t, repo)

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	result, err := svc.Init(false)
	if err != nil {
		t.Fatalf("init dry run: %v", err)
	}
	if result.Outcome != InitMigrationPreview {
		t.Fatalf("outcome = %q, want %q", result.Outcome, InitMigrationPreview)
	}
	if result.Applied {
		t.Fatal("dry run must not apply changes")
	}
	if result.FromVersion != 0 || result.ToVersion != currentLayoutVersion {
		t.Fatalf("versions = %d -> %d, want 0 -> %d", result.FromVersion, result.ToVersion, currentLayoutVersion)
	}
	if len(result.Changes) == 0 {
		t.Fatal("dry run must report the planned changes")
	}

	after := snapshotTree(t, repo)
	if len(addedPaths(before, after)) != 0 {
		t.Fatalf("dry run added files: %v", addedPaths(before, after))
	}
	cfg, _, err := readMarker(repo)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if cfg.LayoutVersion != 0 {
		t.Fatalf("dry run mutated marker to version %d", cfg.LayoutVersion)
	}
}

func TestInitMigrationApplyBumpsMarkerAndValidates(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeFile(t, markerFile(repo), "layout_version: 0\nspecs_dir: specs\nplanning_dir: planning\n")
	// A human-authored spec must survive migration untouched.
	humanSpec := filepath.Join(repo, "specs", "v0.1.0.md")
	humanContent := "# Taskrail v0.1.0\n\n## Summary\n\nHand-authored content.\n"
	writeFile(t, humanSpec, humanContent)

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	result, err := svc.Init(true)
	if err != nil {
		t.Fatalf("init apply: %v", err)
	}
	if result.Outcome != InitMigrated || !result.Applied {
		t.Fatalf("outcome = %q applied=%v, want %q applied", result.Outcome, result.Applied, InitMigrated)
	}
	if result.Validation == nil || !result.Validation.Valid {
		t.Fatalf("expected valid post-migration validation, got %+v", result.Validation)
	}

	cfg, _, err := readMarker(repo)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if cfg.LayoutVersion != currentLayoutVersion {
		t.Fatalf("layout_version = %d, want %d", cfg.LayoutVersion, currentLayoutVersion)
	}

	got, err := os.ReadFile(humanSpec)
	if err != nil {
		t.Fatalf("read human spec: %v", err)
	}
	if string(got) != humanContent {
		t.Fatal("migration rewrote human-authored spec content")
	}
}

func TestInitRejectsNewerLayoutVersion(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeFile(t, markerFile(repo), "layout_version: 999\nspecs_dir: specs\nplanning_dir: planning\n")

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(false); err == nil {
		t.Fatal("expected error for newer-than-supported layout_version")
	}
}

// seedNonStandardRepo builds an unmarked repository that has candidate
// directories (a populated specs/ and a notes/ folder) but no Taskrail STATE.md
// or tasks/ layout, so init must propose a retrofit rather than fresh-create.
func seedNonStandardRepo(t *testing.T) string {
	t.Helper()
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, "specs", "overview.md"), "# Hand-written specs\n")
	if err := os.MkdirAll(filepath.Join(repo, "notes"), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	writeFile(t, filepath.Join(repo, "notes", "ideas.md"), "loose planning notes\n")
	return repo
}

func TestInitDetectsNonStandardLayoutAndProposesMapping(t *testing.T) {
	t.Parallel()

	repo := seedNonStandardRepo(t)
	before := snapshotTree(t, repo)

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	result, err := svc.Init(false)
	if err != nil {
		t.Fatalf("init dry run: %v", err)
	}
	if result.Outcome != InitRetrofitPreview {
		t.Fatalf("outcome = %q, want %q", result.Outcome, InitRetrofitPreview)
	}
	if result.Applied {
		t.Fatal("retrofit dry run must not apply changes")
	}
	// The seed repo has specs/ and notes/ but no planning/, so detection must
	// propose exactly the specs->specs and notes->planning mappings, in that
	// order, and must not emit a redundant second planning role.
	want := []RetrofitMapping{
		{Source: "specs", Target: "specs", Role: "specs"},
		{Source: "notes", Target: "planning", Role: "planning"},
	}
	if !reflect.DeepEqual(result.Mapping, want) {
		t.Fatalf("mapping = %+v, want %+v", result.Mapping, want)
	}
	if len(result.Changes) == 0 {
		t.Fatal("retrofit preview must report planned changes")
	}

	after := snapshotTree(t, repo)
	if added := addedPaths(before, after); len(added) != 0 {
		t.Fatalf("dry run added files: %v", added)
	}
	if _, present, err := readMarker(repo); err != nil || present {
		t.Fatalf("dry run must not write marker: present=%v err=%v", present, err)
	}
}

func TestInitRetrofitApplyIsNonDestructive(t *testing.T) {
	t.Parallel()

	repo := seedNonStandardRepo(t)
	humanSpec := filepath.Join(repo, "specs", "overview.md")
	humanContent := "# Hand-written specs\n"

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	result, err := svc.Init(true)
	if err != nil {
		t.Fatalf("init apply: %v", err)
	}
	if result.Outcome != InitRetrofitApplied || !result.Applied {
		t.Fatalf("outcome = %q applied=%v, want %q applied", result.Outcome, result.Applied, InitRetrofitApplied)
	}
	if result.Validation == nil || !result.Validation.Valid {
		t.Fatalf("expected valid post-retrofit validation, got %+v", result.Validation)
	}

	got, err := os.ReadFile(humanSpec)
	if err != nil {
		t.Fatalf("read human spec: %v", err)
	}
	if string(got) != humanContent {
		t.Fatal("retrofit rewrote human-authored spec content")
	}

	cfg, present, err := readMarker(repo)
	if err != nil || !present {
		t.Fatalf("read marker: present=%v err=%v", present, err)
	}
	if cfg.LayoutVersion != currentLayoutVersion {
		t.Fatalf("layout_version = %d, want %d", cfg.LayoutVersion, currentLayoutVersion)
	}
}

func addedPaths(before, after map[string]string) []string {
	var added []string
	for path := range after {
		if _, ok := before[path]; !ok {
			added = append(added, path)
		}
	}
	sort.Strings(added)
	// filepath.Join normalizes separators for comparison callers.
	for i := range added {
		added[i] = strings.TrimPrefix(added[i], "./")
	}
	return added
}
