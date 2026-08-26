package taskrail

import (
	"fmt"
	"strings"
)

// WorkflowSnapshotContext identifies the immutable subjects a workflow report
// must bind. It intentionally excludes index and destination checks, which are
// publication concerns that need a writer lock.
type WorkflowSnapshotContext struct {
	RepoRoot, SpecPath, ReviewsRoot, ArtifactsDir string
}

// CaptureWorkflowSubjects accepts only a clean attached worktree and reads the
// selected spec from its recorded HEAD tree. This makes the returned values safe
// to pass directly to DecodeWorkflowReport without trusting mutable bytes.
func CaptureWorkflowSubjects(context WorkflowSnapshotContext) (WorkflowSubjects, error) {
	if context.RepoRoot == "" || !workflowSnapshotPath(context.SpecPath) || !workflowSnapshotPath(context.ReviewsRoot) || !workflowSnapshotPath(context.ArtifactsDir) {
		return WorkflowSubjects{}, fmt.Errorf("workflow snapshot context is incomplete or has a non-canonical path")
	}
	status, err := gitCommand(context.RepoRoot, "status", "--porcelain=v2", "--branch", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return WorkflowSubjects{}, fmt.Errorf("inspect workflow worktree status: %w", err)
	}
	head, err := parseWorkflowSnapshotStatus(status)
	if err != nil {
		return WorkflowSubjects{}, err
	}
	if branch, err := gitCommand(context.RepoRoot, "symbolic-ref", "--quiet", "HEAD"); err != nil || strings.TrimSpace(branch) == "" {
		return WorkflowSubjects{}, fmt.Errorf("workflow snapshot requires an attached HEAD")
	}
	currentHead, err := gitCommand(context.RepoRoot, "rev-parse", "--verify", "HEAD")
	if err != nil || strings.TrimSpace(currentHead) != head {
		return WorkflowSubjects{}, fmt.Errorf("workflow snapshot HEAD changed during capture")
	}
	spec, err := readGitTreeBlob(context.RepoRoot, head, context.SpecPath)
	if err != nil {
		return WorkflowSubjects{}, fmt.Errorf("read selected workflow spec: %w", err)
	}
	product, err := WorkflowProductSHA256(context.RepoRoot, head, context.ReviewsRoot)
	if err != nil {
		return WorkflowSubjects{}, err
	}
	return WorkflowSubjects{
		SpecPath:      context.SpecPath,
		Spec:          spec,
		TestedHead:    head,
		ProductSHA256: product,
		ArtifactsDir:  context.ArtifactsDir,
	}, nil
}

func workflowSnapshotPath(value string) bool {
	return !absolutePathStart.MatchString(value) && !strings.ContainsRune(value, 0) && canonicalPathSegments(value)
}

// parseWorkflowSnapshotStatus extracts the exact clean HEAD Git observed. The
// caller confirms attachment separately because porcelain's detached marker is
// also a valid branch name.
func parseWorkflowSnapshotStatus(status string) (string, error) {
	var head string
	for _, line := range strings.Split(strings.TrimSuffix(status, "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.oid "):
			head = strings.TrimPrefix(line, "# branch.oid ")
		case strings.HasPrefix(line, "# branch.head "):
			continue
		case strings.HasPrefix(line, "# "):
			continue
		case line != "":
			return "", fmt.Errorf("workflow snapshot worktree is not clean")
		}
	}
	if !workflowObjectID.MatchString(head) {
		return "", fmt.Errorf("workflow snapshot requires a full HEAD object ID")
	}
	return head, nil
}
