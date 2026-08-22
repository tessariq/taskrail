package taskrail

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// realGitFixtureRepo seeds the standard fixture tree inside a real `git init`
// root. The other rename tests use the stub .git directory from initGitRepo,
// so these tests exist to exercise rename's behavior inside a genuine Git
// worktree, and skip when git is unavailable.
func realGitFixtureRepo(t *testing.T) (string, func(args ...string) string) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(git, args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
			"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
			"LC_ALL=C",
		)
		out, gitErr := cmd.CombinedOutput()
		if gitErr != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), gitErr, out)
		}
		return string(out)
	}
	runGit("init", "-q")
	seedFixtureTree(t, repo)
	return repo, runGit
}

// Rename publishes by filesystem operations rather than Git staging: the move
// lands in the worktree with the index untouched, so the operator (or the
// delivery runner that owns Git) stages it as one reviewed change.
func TestRenameTaskPublishesByFilesystemOperationsInRealRepo(t *testing.T) {
	t.Parallel()
	repo, runGit := realGitFixtureRepo(t)
	writeTask(t, repo, "T-001", "Base", "completed", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002", "Dependent", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-001"})
	runGit("add", "-A")
	runGit("commit", "-q", "-m", "seed")
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "base"}); err != nil {
		t.Fatalf("rename: %v", err)
	}

	// Nothing is staged: the index holds exactly the committed tree.
	if staged := runGit("diff", "--cached", "--name-only"); strings.TrimSpace(staged) != "" {
		t.Fatalf("rename staged changes through the Git index:\n%s", staged)
	}
	// The worktree carries the whole coupled change: the old path deleted, the
	// renamed task untracked, and the dependent's edge rewritten.
	status := runGit("status", "--porcelain")
	if !strings.Contains(status, " D planning/tasks/T-001.md") {
		t.Fatalf("old task file not deleted in the worktree:\n%s", status)
	}
	if !strings.Contains(status, "?? planning/tasks/T-001-base.md") {
		t.Fatalf("renamed task file absent from the worktree:\n%s", status)
	}
	dependent := readBytes(t, filepath.Join(repo, "planning", "tasks", "T-002.md"))
	if !strings.Contains(dependent, "- T-001-base") {
		t.Fatalf("dependent's edge not rewritten:\n%s", dependent)
	}
	if v, err := svc.Validate(); err != nil || !v.Valid {
		t.Fatalf("validate after rename: valid=%v violations=%v err=%v", v.Valid, v.Violations, err)
	}
}

// A handled publication failure rolls the worktree back to its exact original
// bytes, so neither the worktree nor the Git index shows any residue.
func TestRenameTaskRollbackLeavesGitIndexCleanInRealRepo(t *testing.T) {
	requirePermissionFaultInjection(t)
	repo, runGit := realGitFixtureRepo(t)
	writeTask(t, repo, "T-001", "Base", "completed", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002", "Dependent", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-001"})
	runGit("add", "-A")
	runGit("commit", "-q", "-m", "seed")
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	// Fail the first task publication after STATE.md has landed: the tasks
	// directory is locked for the transaction's writes, and the rollback has to
	// return the state file to its committed bytes.
	tasksDir := filepath.Join(repo, "planning", "tasks")
	installLifecycleHook(t, func() {
		if err := os.Chmod(tasksDir, 0o500); err != nil {
			t.Fatalf("lock tasks dir: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(tasksDir, 0o755) })
	})
	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "base"}); err == nil {
		t.Fatal("expected rename to fail on the locked tasks directory")
	}
	if status := runGit("status", "--porcelain"); strings.TrimSpace(status) != "" {
		t.Fatalf("rollback left the git index or worktree dirty:\n%s", status)
	}
	if v, err := svc.Validate(); err != nil || !v.Valid {
		t.Fatalf("tree not valid after rollback: valid=%v violations=%v err=%v", v.Valid, v.Violations, err)
	}
}

func TestRenameTaskRefusesExistingDestinationInRealRepo(t *testing.T) {
	t.Parallel()
	repo, runGit := realGitFixtureRepo(t)
	writeTask(t, repo, "T-001", "Base", "todo", "high", "specs/v0.1.0.md#summary", nil)
	// A stray file already occupies the target path; the rename must refuse it
	// before any write rather than clobber the bytes.
	stray := filepath.Join(repo, "planning", "tasks", "T-001-base.md")
	writeFile(t, stray, strayTaskContent)
	runGit("add", "-A")
	runGit("commit", "-q", "-m", "seed")
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "base"}); err == nil {
		t.Fatal("expected refusal when the target file already exists")
	}
	if got := readBytes(t, stray); got != strayTaskContent {
		t.Fatalf("stray file overwritten:\n%s", got)
	}
	if !fileExists(filepath.Join(repo, "planning", "tasks", "T-001.md")) {
		t.Fatal("source file lost on destination collision")
	}
	if status := runGit("status", "--porcelain"); strings.TrimSpace(status) != "" {
		t.Fatalf("refused rename still touched the tree:\n%s", status)
	}
}
