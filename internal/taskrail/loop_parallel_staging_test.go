package taskrail

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParallelIntegrationStagesOnlyLiteralStatePath(t *testing.T) {
	for _, planning := range []string{"planning", "plan[1]"} {
		t.Run(planning, func(t *testing.T) {
			repo, _ := loopFixture(t)
			writeTask(t, repo, "T-001-ready", "First", "todo", "high", "specs/v0.1.0.md#summary", nil)
			if planning != "planning" {
				if err := os.Rename(filepath.Join(repo, "planning"), filepath.Join(repo, planning)); err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: 2\nspecs_dir: specs\nplanning_dir: "+planning+"\nstorage_mode: committed\nimplementation_review_max_rounds: 1\n")
			}
			svc, err := NewService(repo)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.Repair(RepairInput{Apply: true}); err != nil {
				t.Fatal(err)
			}
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-m", "configure planning")
			workerRoot := filepath.Join(t.TempDir(), "worker")
			runGit(t, repo, "clone", "--no-local", repo, workerRoot)
			workerService, err := NewService(workerRoot)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := workerService.Start("T-001-ready"); err != nil {
				t.Fatal(err)
			}
			if _, err := workerService.Complete("T-001-ready", "done"); err != nil {
				t.Fatal(err)
			}
			runGit(t, workerRoot, "add", ".")
			runGit(t, workerRoot, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "complete task")
			candidate, err := gitCommand(workerRoot, "rev-parse", "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			// Conflict children may leave output unstaged; state staging must not adopt it.
			collateral := "plan1/STATE.md"
			writeFile(t, filepath.Join(repo, collateral), "unrelated child output\n")
			worker := ParallelWorker{Workspace: &workerRoot, CandidateCommit: stringPtr(strings.TrimSpace(candidate))}
			if _, err := svc.integrateParallelCandidate(context.Background(), LoopInvocation{}, LoopPreflightSnapshot{}, repo, &worker, nil, nil); err != nil {
				t.Fatal(err)
			}
			if paths, err := gitCommand(repo, "ls-files", "--", collateral); err != nil || paths != "" {
				t.Fatalf("unrelated output entered index: %q, %v", paths, err)
			}
			if paths, err := gitCommand(repo, "ls-tree", "--name-only", "HEAD", "--", collateral); err != nil || paths != "" {
				t.Fatalf("unrelated output entered commit: %q, %v", paths, err)
			}
			if data, err := os.ReadFile(filepath.Join(repo, collateral)); err != nil || string(data) != "unrelated child output\n" {
				t.Fatalf("unrelated output changed: %q, %v", data, err)
			}
			if repair, err := svc.Repair(RepairInput{}); err != nil || len(repair.BodyDiff) != 0 || len(repair.Changes) != 0 {
				t.Fatalf("integrated state needs repair: %+v, %v", repair, err)
			}
			if paths, err := gitCommand(repo, "diff", "--name-only", "HEAD"); err != nil || paths != "" {
				t.Fatalf("tracked integration output remains uncommitted: %q, %v", paths, err)
			}
		})
	}
}
