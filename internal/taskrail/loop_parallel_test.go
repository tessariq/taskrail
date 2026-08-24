package taskrail

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParallelCloneUsesShallowTransportClone(t *testing.T) {
	repo, _ := loopFixture(t)
	clone := filepath.Join(t.TempDir(), "worker")
	ref, err := gitCommand(repo, "symbolic-ref", "HEAD")
	if err != nil {
		t.Fatalf("read fixture ref: %v", err)
	}

	head, err := gitCommand(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("read fixture HEAD: %v", err)
	}
	if err := parallelClone(repo, clone, strings.TrimSpace(ref), strings.TrimSpace(head), 1); err != nil {
		t.Fatalf("parallelClone: %v", err)
	}
	shallow, err := gitCommand(clone, "rev-parse", "--is-shallow-repository")
	if err != nil {
		t.Fatalf("inspect shallow clone: %v", err)
	}
	if strings.TrimSpace(shallow) != "true" {
		t.Fatalf("is-shallow-repository = %q, want true", shallow)
	}
	if head, err := gitCommand(clone, "rev-parse", "HEAD"); err != nil || strings.TrimSpace(head) == "" {
		t.Fatalf("clone has no checked-out HEAD: %q, %v", head, err)
	}
}

func TestLoopExecuteDeliversParallelCloneBatch(t *testing.T) {
	clearLoopChildEnvironment(t)
	repo, svc := loopFixture(t)
	writeTask(t, repo, "T-001-ready", "First", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002-ready", "Second", "todo", "medium", "specs/v0.1.0.md#summary", nil)
	setLoopPolicy(t, repo, "T-001-ready", "allow", "independent work")
	setLoopPolicy(t, repo, "T-002-ready", "allow", "independent work")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "allow parallel tasks")

	binary := buildParallelTaskrail(t)
	previous := loopExecutablePath
	loopExecutablePath = func() (string, error) { return binary, nil }
	t.Cleanup(func() { loopExecutablePath = previous })
	script := filepath.Join(t.TempDir(), "worker.sh")
	writeFile(t, script, `#!/bin/sh
set -eu
read first
cat >/dev/null
case "$first" in
  "Run the repository's full aggregate validation"*) exit 0 ;;
esac
set -- $first
task_id=$5
task_id=${task_id%.}
"$TASKRAIL" start "$task_id"
"$TASKRAIL" complete "$task_id" --note "parallel test"
"$TASKRAIL" verify "$task_id" --result pass --summary "parallel test" --details "parallel test"
git config user.email test@example.com
git config user.name Test
git add planning
git commit -m "complete $task_id"
`)
	if err := os.Chmod(script, 0o700); err != nil {
		t.Fatalf("make worker executable: %v", err)
	}

	report, err := svc.LoopExecute(context.Background(), LoopInvocation{MaxIterations: 2, Parallel: 2, Child: []string{script}})
	if err != nil {
		t.Fatalf("LoopExecute: %v", err)
	}
	if report.Outcome != "batch_pass" || report.Parallel == nil || len(report.Parallel.Delivery.PublishedTasks) != 2 {
		encoded, _ := json.MarshalIndent(report, "", "  ")
		t.Fatalf("parallel report = %s", encoded)
	}
	if report.LastIteration != nil || report.Parallel.Integration.Head == nil {
		t.Fatalf("parallel diagnostic shape = %+v", report)
	}
	for _, taskID := range []string{"T-001-ready", "T-002-ready"} {
		tasks, err := svc.loadTasks()
		task, found := taskByIDFromSlice(tasks, taskID)
		if err != nil || !found || task.Frontmatter.Status != "completed" {
			t.Fatalf("task %s after parallel delivery = %+v, %v", taskID, task, err)
		}
	}
}

func buildParallelTaskrail(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}
	binary := filepath.Join(t.TempDir(), "taskrail")
	command := exec.Command("go", "build", "-o", binary, "./cmd/taskrail")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build taskrail: %v\n%s", err, output)
	}
	return binary
}
