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
// where `git mv` always fails and only the plain-rename fallback runs — these
// tests exist to exercise the git path itself, so they need a genuine repo and
// skip when git is unavailable.
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

func TestRenameTaskStagesGitRenameInRealRepo(t *testing.T) {
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

	// `git mv` (not the plain-rename fallback) ran, so the move is staged as a
	// rename rather than a delete plus an untracked add. The status code is `R`
	// followed by the worktree column, which the post-move id rewrite makes `M`.
	status := runGit("status", "--porcelain")
	var staged bool
	for _, line := range strings.Split(status, "\n") {
		if strings.HasPrefix(line, "R") && strings.Contains(line, "planning/tasks/T-001.md -> planning/tasks/T-001-base.md") {
			staged = true
		}
	}
	if !staged {
		t.Fatalf("rename not staged as a git rename:\n%s", status)
	}
	if v, err := svc.Validate(); err != nil || !v.Valid {
		t.Fatalf("validate after rename: valid=%v violations=%v err=%v", v.Valid, v.Violations, err)
	}
}

func TestRenameTaskRollbackLeavesGitIndexCleanInRealRepo(t *testing.T) {
	t.Parallel()
	repo, runGit := realGitFixtureRepo(t)
	writeTask(t, repo, "T-001", "Base", "completed", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002", "Dependent", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-001"})
	runGit("add", "-A")
	runGit("commit", "-q", "-m", "seed")
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))

	// Under real version control the move runs through `git mv`, so the rollback
	// must unstage it too: a plain move back would leave the rename staged in the
	// index while the worktree shows the original name.
	requireReadOnlyFileBlocksWrites(t, filepath.Join(repo, "planning", "tasks", "T-002.md"))
	if _, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "base"}); err == nil {
		t.Fatal("expected rename to fail on the unwritable inbound task file")
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
	// A stray file already occupies the target path. `git mv` refuses such a
	// destination, and the plain-rename fallback would silently clobber it.
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

	// The refusal above is Taskrail's guard, not git's: confirm git mv really does
	// fail on an existing destination, so the fallback's masking of that failure
	// stays a covered path. The assertion is on behaviour, not on git's message
	// text, which is localized.
	source := filepath.Join(repo, "planning", "tasks", "T-001.md")
	if err := gitMove(repo, source, stray); err == nil {
		t.Fatal("git mv accepted an existing destination")
	}
	if got := readBytes(t, stray); got != strayTaskContent {
		t.Fatalf("failed git mv still overwrote the destination:\n%s", got)
	}
	if !fileExists(source) {
		t.Fatal("failed git mv removed the source file")
	}
}
