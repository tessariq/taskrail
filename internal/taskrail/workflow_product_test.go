package taskrail

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaptureWorkflowSubjectsBindsCleanAttachedHead(t *testing.T) {
	repo, runGit := workflowEvidenceGitRepo(t)
	writeFile(t, filepath.Join(repo, "specs", "v0.5.0.md"), "# Selected spec\n")
	writeFile(t, filepath.Join(repo, "product.txt"), "product\n")
	runGit("add", "specs/v0.5.0.md", "product.txt")
	runGit("commit", "-q", "-m", "snapshot")
	head := strings.TrimSpace(runGit("rev-parse", "HEAD"))

	context := WorkflowSnapshotContext{
		RepoRoot: repo, SpecPath: "specs/v0.5.0.md", ReviewsRoot: "planning/reviews", ArtifactsDir: "planning/artifacts",
	}
	subjects, err := CaptureWorkflowSubjects(context)
	if err != nil {
		t.Fatalf("CaptureWorkflowSubjects: %v", err)
	}
	if subjects.TestedHead != head || string(subjects.Spec) != "# Selected spec\n" || subjects.SpecPath != context.SpecPath {
		t.Fatalf("subjects = %#v, want selected spec and head %q", subjects, head)
	}
	wantProduct, err := WorkflowProductSHA256(repo, head, context.ReviewsRoot)
	if err != nil || subjects.ProductSHA256 != wantProduct {
		t.Fatalf("product = %q, %v; want %q", subjects.ProductSHA256, err, wantProduct)
	}

	writeFile(t, filepath.Join(repo, "product.txt"), "dirty\n")
	if _, err := CaptureWorkflowSubjects(context); err == nil || !strings.Contains(err.Error(), "not clean") {
		t.Fatalf("dirty capture error = %v, want clean-worktree refusal", err)
	}
	runGit("checkout", "--", "product.txt")
	runGit("checkout", "-q", "--detach")
	if _, err := CaptureWorkflowSubjects(context); err == nil || !strings.Contains(err.Error(), "attached HEAD") {
		t.Fatalf("detached capture error = %v, want attached-head refusal", err)
	}
	runGit("checkout", "-q", "-b", "(detached)")
	if _, err := CaptureWorkflowSubjects(context); err != nil {
		t.Fatalf("CaptureWorkflowSubjects on branch named (detached): %v", err)
	}
}

func TestWorkflowProductSHA256UsesBoundTreeAndExcludesReviewSubtree(t *testing.T) {
	repo, runGit := workflowEvidenceGitRepo(t)
	writeFile(t, filepath.Join(repo, "a.txt"), "alpha\n")
	writeFile(t, filepath.Join(repo, "bin", "run"), "run\n")
	writeFile(t, filepath.Join(repo, "planning", "reviews", "workflow-adversarial", "INDEX.json"), "ignored\n")
	runGit("add", "a.txt", "bin/run", "planning/reviews/workflow-adversarial/INDEX.json")
	runGit("update-index", "--chmod=+x", "bin/run")
	runGit("commit", "-q", "-m", "product")
	head := strings.TrimSpace(runGit("rev-parse", "HEAD"))

	got, err := WorkflowProductSHA256(repo, head, "planning/reviews")
	if err != nil {
		t.Fatalf("WorkflowProductSHA256: %v", err)
	}
	want := workflowProductDigest(t, []workflowProductFixture{
		{path: "a.txt", mode: "100644", content: []byte("alpha\n")},
		{path: "bin/run", mode: "100755", content: []byte("run\n")},
	})
	if got != want {
		t.Fatalf("digest = %q, want %q", got, want)
	}

	writeFile(t, filepath.Join(repo, "a.txt"), "worktree only\n")
	if again, err := WorkflowProductSHA256(repo, head, "planning/reviews"); err != nil || again != want {
		t.Fatalf("bound-tree digest = %q, %v; want %q", again, err, want)
	}
}

func TestWorkflowProductSHA256FramesGitlinks(t *testing.T) {
	repo, runGit := workflowEvidenceGitRepo(t)
	runGit("commit", "-q", "--allow-empty", "-m", "base")
	gitlink := strings.TrimSpace(runGit("rev-parse", "HEAD"))
	runGit("update-index", "--add", "--cacheinfo", "160000,"+gitlink+",module")
	runGit("commit", "-q", "-m", "gitlink")
	head := strings.TrimSpace(runGit("rev-parse", "HEAD"))

	got, err := WorkflowProductSHA256(repo, head, "planning/reviews")
	if err != nil {
		t.Fatalf("WorkflowProductSHA256: %v", err)
	}
	want := workflowProductDigest(t, []workflowProductFixture{{path: "module", mode: "160000", content: []byte(gitlink)}})
	if got != want {
		t.Fatalf("digest = %q, want %q", got, want)
	}
}

func TestWorkflowProductSHA256RejectsNULReviewRoot(t *testing.T) {
	repo, runGit := workflowEvidenceGitRepo(t)
	runGit("commit", "-q", "--allow-empty", "-m", "base")
	head := strings.TrimSpace(runGit("rev-parse", "HEAD"))
	if _, err := WorkflowProductSHA256(repo, head, "planning/reviews\x00"); err == nil || !strings.Contains(err.Error(), "canonical") {
		t.Fatalf("error = %v, want malformed review-root refusal", err)
	}
}

func TestParseWorkflowSnapshotStatusBindsAttachedCleanHead(t *testing.T) {
	head := strings.Repeat("a", 40)
	if got, err := parseWorkflowSnapshotStatus("# branch.oid " + head + "\n# branch.head main\n# branch.upstream origin/main\n"); err != nil || got != head {
		t.Fatalf("parseWorkflowSnapshotStatus = %q, %v; want %q", got, err, head)
	}
	if got, err := parseWorkflowSnapshotStatus("# branch.oid " + head + "\n# branch.head (detached)\n"); err != nil || got != head {
		t.Fatalf("detached-label parse = %q, %v; want %q", got, err, head)
	}
	if _, err := parseWorkflowSnapshotStatus("# branch.oid " + head + "\n# branch.head main\n? untracked.txt\n"); err == nil || !strings.Contains(err.Error(), "not clean") {
		t.Fatalf("dirty error = %v, want clean-worktree refusal", err)
	}
}

type workflowProductFixture struct {
	path, mode string
	content    []byte
}

func workflowProductDigest(t *testing.T, entries []workflowProductFixture) string {
	t.Helper()
	var frame bytes.Buffer
	frame.WriteString("taskrail-workflow-product-v1")
	frame.WriteByte(0)
	for _, entry := range entries {
		frame.WriteString(entry.path)
		frame.WriteByte(0)
		frame.WriteString(entry.mode)
		frame.WriteByte(0)
		frame.WriteString(fmt.Sprintf("%d", len(entry.content)))
		frame.WriteByte(0)
		frame.Write(entry.content)
		frame.WriteByte(0)
	}
	sum := sha256.Sum256(frame.Bytes())
	return fmt.Sprintf("%x", sum)
}
