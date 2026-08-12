package taskrail

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// WorkflowEvidenceContext binds file evidence to the repository and active
// storage snapshot a workflow report was produced against.
type WorkflowEvidenceContext struct {
	RepoRoot     string
	Storage      StorageContext
	PlanningDir  string
	ArtifactsDir string
}

// ValidateWorkflowFileEvidence proves every strictly decoded file citation
// against immutable review bytes or the report's exact Git tree. It never reads
// product bytes from the mutable worktree.
func ValidateWorkflowFileEvidence(report WorkflowReport, context WorkflowEvidenceContext) error {
	if context.RepoRoot == "" || context.PlanningDir == "" || context.ArtifactsDir == "" || !workflowObjectID.MatchString(report.TestedHead) {
		return fmt.Errorf("workflow file evidence context is incomplete")
	}
	for _, observation := range report.Observations {
		for _, evidence := range observation.Evidence {
			if evidence.Kind != "file" {
				continue
			}
			if evidence.Path == nil || evidence.SHA256 == nil {
				return fmt.Errorf("workflow observation %q has incomplete file evidence", observation.ObservationID)
			}
			if err := validateWorkflowEvidencePath(*evidence.Path, "workflow observation "+observation.ObservationID, WorkflowSubjects{ArtifactsDir: context.ArtifactsDir}); err != nil {
				return err
			}
			data, err := resolveWorkflowEvidenceBytes(*evidence.Path, report.TestedHead, context)
			if err != nil {
				return fmt.Errorf("workflow observation %q file evidence path %q: %w", observation.ObservationID, *evidence.Path, err)
			}
			if digestRaw(data) != *evidence.SHA256 {
				return fmt.Errorf("workflow observation %q file evidence path %q digest does not match resolved bytes", observation.ObservationID, *evidence.Path)
			}
		}
	}
	return nil
}

func resolveWorkflowEvidenceBytes(logicalPath, head string, context WorkflowEvidenceContext) ([]byte, error) {
	if workflowReviewPath(logicalPath, context.PlanningDir) {
		return readPublishedReview(context, logicalPath)
	}
	if workflowManagedPath(logicalPath, context.PlanningDir) {
		return nil, fmt.Errorf("path is managed or transient, not durable product evidence")
	}
	return readGitTreeBlob(context.RepoRoot, head, logicalPath)
}

func workflowReviewPath(logicalPath, planningDir string) bool {
	rel := strings.TrimPrefix(logicalPath, planningDir+"/reviews/")
	if rel == logicalPath {
		return false
	}
	for _, root := range []string{"spec/", "task/", "decomposition/", "workflow-adversarial/"} {
		if strings.HasPrefix(rel, root) {
			return true
		}
	}
	return false
}

func workflowManagedPath(logicalPath, planningDir string) bool {
	for _, root := range []string{
		path.Join(planningDir, "STATE.md"),
		path.Join(planningDir, "tasks"),
		path.Join(planningDir, "prompts"),
		path.Join(planningDir, "provenance"),
	} {
		if logicalPath == root || strings.HasPrefix(logicalPath, root+"/") {
			return true
		}
	}
	return strings.HasPrefix(logicalPath, planningDir+"/reviews/")
}

func readPublishedReview(context WorkflowEvidenceContext, logicalPath string) ([]byte, error) {
	physical := filepath.Join(context.RepoRoot, filepath.FromSlash(context.Storage.physical(logicalPath)))
	rel, err := filepath.Rel(context.RepoRoot, physical)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("path escapes the repository")
	}
	current := context.RepoRoot
	for _, segment := range strings.Split(filepath.ToSlash(rel), "/") {
		current = filepath.Join(current, filepath.FromSlash(segment))
		info, err := os.Lstat(current)
		if err != nil {
			return nil, fmt.Errorf("resolve regular published review: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("path does not resolve to a regular published review")
		}
	}
	info, err := os.Stat(physical)
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("path does not resolve to a regular published review")
	}
	data, err := os.ReadFile(physical)
	if err != nil {
		return nil, fmt.Errorf("read published review: %w", err)
	}
	return data, nil
}

func readGitTreeBlob(repoRoot, head, logicalPath string) ([]byte, error) {
	cmd := exec.Command("git", "-C", repoRoot, "ls-tree", "-z", head, "--", ":(literal)"+logicalPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("inspect bound Git tree: %w", err)
	}
	entries := bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0})
	if len(entries) != 1 || len(entries[0]) == 0 {
		return nil, fmt.Errorf("path is absent from the bound Git tree")
	}
	metadata, _, found := bytes.Cut(entries[0], []byte{'\t'})
	if !found {
		return nil, fmt.Errorf("inspect bound Git tree: malformed ls-tree result")
	}
	fields := strings.Fields(string(metadata))
	if len(fields) != 3 || (fields[0] != "100644" && fields[0] != "100755") || fields[1] != "blob" {
		return nil, fmt.Errorf("path is not a regular blob in the bound Git tree")
	}
	blob := exec.Command("git", "-C", repoRoot, "cat-file", "blob", fields[2])
	data, err := blob.Output()
	if err != nil {
		return nil, fmt.Errorf("read bound Git blob: %w", err)
	}
	return data, nil
}
