package taskrail

import (
	"path/filepath"
	"testing"
)

func TestDiscoverPathsFallsBackWhenMarkerAbsent(t *testing.T) {
	repo := initGitRepo(t)

	paths, err := DiscoverPaths(repo)
	if err != nil {
		t.Fatalf("discover paths: %v", err)
	}

	assertDefaultLayout(t, repo, paths)
}

func TestDiscoverPathsReadsMarkerWithDefaultLayout(t *testing.T) {
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"),
		"layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n")

	paths, err := DiscoverPaths(repo)
	if err != nil {
		t.Fatalf("discover paths: %v", err)
	}

	// Marker that pins the current layout must resolve identically to the fallback.
	assertDefaultLayout(t, repo, paths)
}

func TestDiscoverPathsResolvesFromMarkerLocations(t *testing.T) {
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"),
		"layout_version: 1\nspecs_dir: product/specs\nplanning_dir: work/planning\n")

	paths, err := DiscoverPaths(repo)
	if err != nil {
		t.Fatalf("discover paths: %v", err)
	}

	wantPlanning := filepath.Join(repo, "work", "planning")
	if paths.SpecsDir != filepath.Join(repo, "product", "specs") {
		t.Fatalf("specs dir: got %q", paths.SpecsDir)
	}
	if paths.PlanningDir != wantPlanning {
		t.Fatalf("planning dir: got %q", paths.PlanningDir)
	}
	if paths.TasksDir != filepath.Join(wantPlanning, "tasks") {
		t.Fatalf("tasks dir: got %q", paths.TasksDir)
	}
	if paths.StateFile != filepath.Join(wantPlanning, "STATE.md") {
		t.Fatalf("state file: got %q", paths.StateFile)
	}
}

func TestDiscoverPathsDefaultsMissingMarkerFields(t *testing.T) {
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: 1\n")

	paths, err := DiscoverPaths(repo)
	if err != nil {
		t.Fatalf("discover paths: %v", err)
	}

	assertDefaultLayout(t, repo, paths)
}

func TestDiscoverPathsRejectsEscapingMarkerLocation(t *testing.T) {
	cases := map[string]string{
		"planning_dir": "layout_version: 1\nplanning_dir: ../../outside\n",
		"specs_dir":    "layout_version: 1\nspecs_dir: ../../outside\n",
	}
	for name, config := range cases {
		t.Run(name, func(t *testing.T) {
			repo := initGitRepo(t)
			writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), config)

			if _, err := DiscoverPaths(repo); err == nil {
				t.Fatalf("expected error for %s escaping repo root", name)
			}
		})
	}
}

func TestDiscoverPathsRejectsMalformedMarker(t *testing.T) {
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: [not-an-int\n")

	if _, err := DiscoverPaths(repo); err == nil {
		t.Fatal("expected error for malformed layout config")
	}
}

func assertDefaultLayout(t *testing.T, repo string, paths Paths) {
	t.Helper()
	planning := filepath.Join(repo, "planning")
	artifacts := filepath.Join(planning, "artifacts")
	want := Paths{
		RepoRoot:     repo,
		SpecsDir:     filepath.Join(repo, "specs"),
		PlanningDir:  planning,
		TasksDir:     filepath.Join(planning, "tasks"),
		ArtifactsDir: artifacts,
		VerifyDir:    filepath.Join(artifacts, "verify"),
		StateFile:    filepath.Join(planning, "STATE.md"),
	}
	if paths != want {
		t.Fatalf("layout mismatch:\n got  %+v\n want %+v", paths, want)
	}
}
