package taskrail

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateWorkflowFileEvidenceResolvesBoundGitBlob(t *testing.T) {
	repo, runGit := workflowEvidenceGitRepo(t)
	writeFile(t, filepath.Join(repo, "product.txt"), "committed\n")
	runGit("add", "product.txt")
	runGit("commit", "-q", "-m", "product")
	head := strings.TrimSpace(runGit("rev-parse", "HEAD"))

	context := WorkflowEvidenceContext{
		RepoRoot: repo, Storage: committedStorage(), PlanningDir: "planning", ArtifactsDir: "planning/artifacts",
	}
	before := runGit("status", "--porcelain=v1", "--untracked-files=all")
	if err := ValidateWorkflowFileEvidence(workflowFileReport(head, "product.txt", digestRaw([]byte("committed\n"))), context); err != nil {
		t.Fatalf("ValidateWorkflowFileEvidence: %v", err)
	}
	if after := runGit("status", "--porcelain=v1", "--untracked-files=all"); after != before {
		t.Fatalf("validation changed repository status: before %q, after %q", before, after)
	}

	writeFile(t, filepath.Join(repo, "product.txt"), "worktree only\n")
	if err := ValidateWorkflowFileEvidence(workflowFileReport(head, "product.txt", digestRaw([]byte("worktree only\n"))), context); err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("worktree substitution error = %v, want digest mismatch against bound tree", err)
	}
	writeFile(t, filepath.Join(repo, "untracked.txt"), "untracked\n")
	if err := ValidateWorkflowFileEvidence(workflowFileReport(head, "untracked.txt", digestRaw([]byte("untracked\n"))), context); err == nil || !strings.Contains(err.Error(), "bound Git tree") {
		t.Fatalf("untracked error = %v, want missing bound Git blob", err)
	}
}

func TestValidateWorkflowFileEvidenceRequiresRegularGitBlob(t *testing.T) {
	repo, runGit := workflowEvidenceGitRepo(t)
	writeFile(t, filepath.Join(repo, "target.txt"), "target\n")
	if err := os.Symlink("target.txt", filepath.Join(repo, "alias.txt")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	runGit("add", "target.txt", "alias.txt")
	runGit("commit", "-q", "-m", "symlink")
	head := strings.TrimSpace(runGit("rev-parse", "HEAD"))
	context := WorkflowEvidenceContext{RepoRoot: repo, Storage: committedStorage(), PlanningDir: "planning", ArtifactsDir: "planning/artifacts"}

	err := ValidateWorkflowFileEvidence(workflowFileReport(head, "alias.txt", digestRaw([]byte("target.txt"))), context)
	if err == nil || !strings.Contains(err.Error(), "regular blob") {
		t.Fatalf("symlink error = %v, want regular blob refusal", err)
	}
}

func TestValidateWorkflowFileEvidenceResolvesPublishedReviewWithoutAliases(t *testing.T) {
	repo, runGit := workflowEvidenceGitRepo(t)
	runGit("commit", "-q", "--allow-empty", "-m", "empty")
	head := strings.TrimSpace(runGit("rev-parse", "HEAD"))
	reviewPath := "planning/reviews/task/T-001/session-1/review.json"
	reviewBytes := []byte("published review\n")
	writeFile(t, filepath.Join(repo, filepath.FromSlash(reviewPath)), string(reviewBytes))
	context := WorkflowEvidenceContext{RepoRoot: repo, Storage: committedStorage(), PlanningDir: "planning", ArtifactsDir: "planning/artifacts"}

	if err := ValidateWorkflowFileEvidence(workflowFileReport(head, reviewPath, digestRaw(reviewBytes)), context); err != nil {
		t.Fatalf("published review: %v", err)
	}
	if err := ValidateWorkflowFileEvidence(workflowFileReport(head, reviewPath, reviewDigestA), context); err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("review digest error = %v, want mismatch", err)
	}

	aliasRoot := filepath.Join(repo, "planning", "reviews", "spec")
	if err := os.MkdirAll(filepath.Dir(aliasRoot), 0o755); err != nil {
		t.Fatalf("mkdir reviews: %v", err)
	}
	if err := os.Symlink(filepath.Join(repo, "planning", "artifacts"), aliasRoot); err != nil {
		t.Fatalf("symlink review root: %v", err)
	}
	err := ValidateWorkflowFileEvidence(workflowFileReport(head, "planning/reviews/spec/proposal.json", reviewDigestA), context)
	if err == nil || !strings.Contains(err.Error(), "regular published review") {
		t.Fatalf("review alias error = %v, want no-follow refusal", err)
	}
}

func TestValidateWorkflowFileEvidenceRejectsManagedAndTransientRoots(t *testing.T) {
	context := WorkflowEvidenceContext{RepoRoot: t.TempDir(), Storage: localStorage(), PlanningDir: "planning", ArtifactsDir: "planning/artifacts"}
	for _, path := range []string{
		"planning/STATE.md",
		"planning/tasks/T-001.md",
		"planning/artifacts/review-proposals/workflow/wf-1/report.json",
		".taskrail/local/planning/reviews/task/T-001/session/review.json",
	} {
		t.Run(path, func(t *testing.T) {
			err := ValidateWorkflowFileEvidence(workflowFileReport(workflowHead, path, reviewDigestA), context)
			if err == nil || !strings.Contains(err.Error(), "not durable") {
				t.Fatalf("error = %v, want durable-path refusal", err)
			}
		})
	}
}

func workflowFileReport(head, path, digest string) WorkflowReport {
	return WorkflowReport{TestedHead: head, Observations: []WorkflowObservation{{
		ObservationID: "obs-1",
		Evidence:      []WorkflowEvidence{{Kind: "file", Path: &path, SHA256: &digest}},
	}}}
}

func workflowEvidenceGitRepo(t *testing.T) (string, func(...string) string) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(git, append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Taskrail", "GIT_AUTHOR_EMAIL=taskrail@example.com", "GIT_COMMITTER_NAME=Taskrail", "GIT_COMMITTER_EMAIL=taskrail@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return string(out)
	}
	run("init", "-q")
	return repo, run
}
