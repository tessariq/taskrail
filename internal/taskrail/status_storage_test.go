package taskrail

import (
	"path/filepath"
	"testing"
	"time"
)

// Status must report the active storage snapshot it actually read through, so an
// agent selects delivery behavior and transient staging without opening the
// layout marker or reconstructing a configured path
// (specs/v0.5.0.md#uniform-agent-machine-results).

func TestStatusReportsCommittedStorage(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	report, err := svc.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	want := StatusStorage{Mode: "committed", Root: ".", ArtifactsDir: "planning/artifacts"}
	if report.Storage != want {
		t.Fatalf("storage = %+v, want %+v", report.Storage, want)
	}
}

func TestStatusReportsExplicitLocalStorage(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	seedFixtureTree(t, filepath.Join(repo, localStorageRoot))
	svc := newLocalTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	report, err := svc.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	want := StatusStorage{
		Mode:         "local",
		Root:         ".taskrail/local",
		ArtifactsDir: ".taskrail/local/planning/artifacts",
	}
	if report.Storage != want {
		t.Fatalf("storage = %+v, want %+v", report.Storage, want)
	}
	// The overlay path is transient staging only: it must never leak into a
	// durable citation such as the reported active spec path.
	if report.ActiveSpecPath != "specs/v0.1.0.md" {
		t.Fatalf("active_spec_path = %q, want the logical committed namespace", report.ActiveSpecPath)
	}
}

// newLocalTestService builds a service over an explicitly supplied local storage
// context. Discovering that context from a marker belongs to local mode's own
// tasks; the machine contract here only needs a context to report through.
func newLocalTestService(t *testing.T, repo string, now time.Time) *Service {
	t.Helper()
	git, err := discoverGitWorktree(repo)
	if err != nil {
		t.Fatalf("discover lock context: %v", err)
	}
	paths := pathsFromLayout(repo, defaultLayoutConfig(), localStorage())
	paths.ManagedRoot = repo
	paths.WorktreeRoot = git.WorktreeRoot
	paths.GitDir = git.GitDir
	paths.GitCommonDir = git.GitCommonDir
	paths.LockRoot = filepath.Join(git.GitCommonDir, "taskrail")
	return &Service{
		paths: paths,
		now:   func() time.Time { return now },
	}
}
