package taskrail

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/repolock"
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
		ManagedRoot:  repo,
		WorktreeRoot: repo,
		GitDir:       filepath.Join(repo, ".git"),
		GitCommonDir: filepath.Join(repo, ".git"),
		ConfigFile:   filepath.Join(repo, ".taskrail", "config.yml"),
		StorageRoot:  repo,
		LockRoot:     filepath.Join(repo, ".git", "taskrail"),
		// Discovery resolves the committed context until a local marker exists.
		Storage:            committedStorage(),
		LogicalSpecsDir:    "specs",
		LogicalPlanningDir: "planning",
		LogicalPromptsDir:  ".taskrail/prompts",

		SpecsDir:     filepath.Join(repo, "specs"),
		PlanningDir:  planning,
		PromptsDir:   filepath.Join(repo, ".taskrail", "prompts"),
		TasksDir:     filepath.Join(planning, "tasks"),
		ArtifactsDir: artifacts,
		VerifyDir:    filepath.Join(artifacts, "verify"),
		RuntimeDir:   filepath.Join(repo, ".taskrail", "runtime"),
		StateFile:    filepath.Join(planning, "STATE.md"),
	}
	if paths != want {
		t.Fatalf("layout mismatch:\n got  %+v\n want %+v", paths, want)
	}
}

func TestDiscoverPathsLayout2LocalContextFromDescendant(t *testing.T) {
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), layout2Marker("local", "product/specs", "work/planning"))
	descendant := filepath.Join(repo, "src", "nested")
	if err := os.MkdirAll(descendant, 0o755); err != nil {
		t.Fatal(err)
	}

	paths, err := DiscoverPaths(descendant)
	if err != nil {
		t.Fatalf("discover paths: %v", err)
	}
	overlay := filepath.Join(repo, ".taskrail", "local")
	if paths.ManagedRoot != repo || paths.WorktreeRoot != repo || paths.StorageRoot != overlay {
		t.Fatalf("root identities = managed %q worktree %q storage %q", paths.ManagedRoot, paths.WorktreeRoot, paths.StorageRoot)
	}
	if paths.SpecsDir != filepath.Join(overlay, "product", "specs") || paths.PlanningDir != filepath.Join(overlay, "work", "planning") {
		t.Fatalf("mapped semantic paths = specs %q planning %q", paths.SpecsDir, paths.PlanningDir)
	}
	if paths.PromptsDir != filepath.Join(overlay, "prompts") || paths.RuntimeDir != filepath.Join(overlay, "runtime") {
		t.Fatalf("local operational paths = prompts %q runtime %q", paths.PromptsDir, paths.RuntimeDir)
	}
	if paths.LogicalSpecsDir != "product/specs" || paths.LogicalPlanningDir != "work/planning" || paths.LogicalPromptsDir != ".taskrail/prompts" {
		t.Fatalf("logical identities changed: %+v", paths)
	}
	wantRepo := repolock.Repository{Root: repo, GitCommonDir: filepath.Join(repo, ".git"), Mode: repolock.ModeLocal}
	if got := paths.LockRepository(); got != wantRepo {
		t.Fatalf("lock repository = %+v, want %+v", got, wantRepo)
	}
}

func TestDiscoverPathsLinkedWorktreePreservesGitIdentities(t *testing.T) {
	repo := realGitRepo(t)
	linked := filepath.Join(t.TempDir(), "linked")
	runGit(t, repo, "worktree", "add", "-b", "linked-test", linked)
	writeFile(t, filepath.Join(linked, ".taskrail", "config.yml"), layout2Marker("committed", "specs", "planning"))
	if err := os.MkdirAll(filepath.Join(linked, "planning", "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	worktreeBefore := snapshotTree(t, linked)
	commonBefore := snapshotTree(t, repo)

	paths, err := DiscoverPaths(filepath.Join(linked, "planning", "tasks"))
	if err != nil {
		t.Fatalf("discover linked worktree: %v", err)
	}
	if paths.WorktreeRoot != linked || paths.ManagedRoot != linked {
		t.Fatalf("worktree roots = %q / %q", paths.WorktreeRoot, paths.ManagedRoot)
	}
	if paths.GitDir == paths.GitCommonDir || paths.GitCommonDir != filepath.Join(repo, ".git") {
		t.Fatalf("git identities = dir %q common %q", paths.GitDir, paths.GitCommonDir)
	}
	if paths.LockRoot != filepath.Join(repo, ".git", "taskrail") {
		t.Fatalf("lock root = %q", paths.LockRoot)
	}
	if !reflect.DeepEqual(snapshotTree(t, linked), worktreeBefore) || !reflect.DeepEqual(snapshotTree(t, repo), commonBefore) {
		t.Fatal("successful discovery changed worktree, index, or Git-common metadata")
	}
}

func TestDiscoverPathsSupportsNonGitCommittedAncestor(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), layout2Marker("committed", "specs", "planning"))
	descendant := filepath.Join(repo, "a", "b")
	if err := os.MkdirAll(descendant, 0o755); err != nil {
		t.Fatal(err)
	}

	paths, err := DiscoverPaths(descendant)
	if err != nil {
		t.Fatalf("discover non-git repository: %v", err)
	}
	if paths.WorktreeRoot != "" || paths.GitDir != "" || paths.GitCommonDir != "" {
		t.Fatalf("non-git context has git identities: %+v", paths)
	}
	if paths.LockRoot != filepath.Join(repo, ".taskrail", "runtime") || paths.LockRepository().Mode != repolock.ModeCommitted {
		t.Fatalf("non-git lock context = root %q repo %+v", paths.LockRoot, paths.LockRepository())
	}
}

func TestDiscoverPathsRejectsLocalWithoutGit(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), layout2Marker("local", "specs", "planning"))

	_, err := DiscoverPaths(repo)
	if err == nil || MachineFailureFor(err).Code != MachineCodeRepositoryInvalid || !strings.Contains(err.Error(), "requires a Git worktree") {
		t.Fatalf("local non-git error = %v", err)
	}
}

func TestDiscoverPathsRefusesMismatchMixedAndUnsafeTraversal(t *testing.T) {
	t.Run("managed root differs from worktree", func(t *testing.T) {
		repo := initGitRepo(t)
		nested := filepath.Join(repo, "nested")
		writeFile(t, filepath.Join(nested, ".taskrail", "config.yml"), layout2Marker("committed", "specs", "planning"))
		if _, err := DiscoverPaths(nested); err == nil || !strings.Contains(err.Error(), "does not match Git worktree root") {
			t.Fatalf("mismatch error = %v", err)
		}
	})

	for _, tc := range []struct {
		name string
		mode string
		path string
	}{
		{name: "committed bytes in local mode", mode: "local", path: "planning/STATE.md"},
		{name: "local bytes in committed mode", mode: "committed", path: ".taskrail/local/planning/STATE.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := initGitRepo(t)
			writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), layout2Marker(tc.mode, "specs", "planning"))
			writeFile(t, filepath.Join(repo, filepath.FromSlash(tc.path)), "decoy")
			before := snapshotTree(t, repo)
			paths, err := DiscoverPaths(repo)
			if err == nil || !strings.Contains(err.Error(), "mixed committed/local") {
				t.Fatalf("mixed-state error = %v", err)
			}
			if paths != (Paths{}) {
				t.Fatalf("refusal returned fallback paths: %+v", paths)
			}
			if after := snapshotTree(t, repo); !reflect.DeepEqual(after, before) {
				t.Fatalf("discovery refusal changed repository: before=%v after=%v", before, after)
			}
		})
	}

	t.Run("symlink traversal", func(t *testing.T) {
		repo := initGitRepo(t)
		writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), layout2Marker("committed", "linked/specs", "planning"))
		if err := os.Symlink(t.TempDir(), filepath.Join(repo, "linked")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		if _, err := DiscoverPaths(repo); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("symlink error = %v", err)
		}
	})

	t.Run("special traversal entry", func(t *testing.T) {
		repo := initGitRepo(t)
		writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), layout2Marker("committed", "blocked/specs", "planning"))
		writeFile(t, filepath.Join(repo, "blocked"), "not a directory")
		if _, err := DiscoverPaths(repo); err == nil || !strings.Contains(err.Error(), "non-directory") {
			t.Fatalf("special traversal error = %v", err)
		}
	})

	t.Run("special semantic root", func(t *testing.T) {
		repo := initGitRepo(t)
		writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), layout2Marker("committed", "specs", "planning"))
		writeFile(t, filepath.Join(repo, "specs"), "not a directory")
		if _, err := DiscoverPaths(repo); err == nil || !strings.Contains(err.Error(), "non-directory") {
			t.Fatalf("special root error = %v", err)
		}
	})

	t.Run("case alias", func(t *testing.T) {
		repo := initGitRepo(t)
		writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), layout2Marker("committed", "specs", "planning"))
		if err := os.Mkdir(filepath.Join(repo, ".TASKRAIL"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := DiscoverPaths(repo); err == nil || !strings.Contains(err.Error(), "alias") {
			t.Fatalf("alias error = %v", err)
		}
	})

	t.Run("inactive committed path alias", func(t *testing.T) {
		repo := initGitRepo(t)
		writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), layout2Marker("local", "specs", "planning"))
		if err := os.Mkdir(filepath.Join(repo, "Planning"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := DiscoverPaths(repo); err == nil || !strings.Contains(err.Error(), "alias") {
			t.Fatalf("inactive alias error = %v", err)
		}
	})

	t.Run("unicode normalization alias", func(t *testing.T) {
		repo := initGitRepo(t)
		writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), layout2Marker("committed", "caf\u00e9/specs", "planning"))
		if err := os.Mkdir(filepath.Join(repo, "cafe\u0301"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := DiscoverPaths(repo); err == nil || !strings.Contains(err.Error(), "alias") {
			t.Fatalf("unicode alias error = %v", err)
		}
	})
}

func TestDiscoverPathsLayout2StrictRefusalsAreReadOnly(t *testing.T) {
	cases := map[string]string{
		"unknown field":   layout2Marker("committed", "specs", "planning") + "surprise: true\n",
		"migration fence": "layout_version: 2\nspecs_dir: specs\nplanning_dir: planning\nstorage_mode: committed\nimplementation_review_max_rounds: 2\nmigration_fence:\n  from_layout_version: 1\n  transaction_id: 0123456789abcdef0123456789abcdef\n",
	}
	for name, marker := range cases {
		t.Run(name, func(t *testing.T) {
			repo := initGitRepo(t)
			writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), marker)
			before := snapshotTree(t, repo)
			if _, err := DiscoverPaths(repo); err == nil {
				t.Fatal("expected strict refusal")
			}
			if after := snapshotTree(t, repo); !reflect.DeepEqual(after, before) {
				t.Fatalf("strict refusal changed repository: before=%v after=%v", before, after)
			}
		})
	}
}

func TestDiscoverPathsMissingRootRefuses(t *testing.T) {
	_, err := DiscoverPaths(t.TempDir())
	if err == nil || MachineFailureFor(err).Code != MachineCodeNotInitialized {
		t.Fatalf("missing-root error = %v", err)
	}
}

func layout2Marker(mode, specs, planning string) string {
	return "layout_version: 2\nspecs_dir: " + specs + "\nplanning_dir: " + planning + "\nstorage_mode: " + mode + "\nimplementation_review_max_rounds: 2\n"
}

func realGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test")
	writeFile(t, filepath.Join(repo, "README.md"), "fixture\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "fixture")
	return repo
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
