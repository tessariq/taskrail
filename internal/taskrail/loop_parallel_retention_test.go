package taskrail

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoopParallelRetainsUnpublishedIntegration(t *testing.T) {
	clearLoopChildEnvironment(t)
	binary := buildTaskrailExecutable(t)
	previous := loopExecutablePath
	loopExecutablePath = func() (string, error) { return binary, nil }
	t.Cleanup(func() { loopExecutablePath = previous })
	t.Setenv("GO_WANT_LOOP_CHILD", "1")
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, retention := range []string{"failure", "always", "never"} {
		t.Run(retention, func(t *testing.T) {
			repo, svc := loopFixture(t)
			for _, id := range []string{"T-001-ready", "T-002-ready"} {
				writeTask(t, repo, id, id, "todo", "high", "specs/v0.1.0.md#summary", nil)
				setLoopPolicy(t, repo, id, "allow", "independent work")
			}
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-m", "allow parallel tasks")
			base, _ := gitCommand(repo, "rev-parse", "HEAD")
			workspaceRoot := t.TempDir()
			for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
				t.Setenv(key, workspaceRoot)
			}
			var integrationPath string
			testHookParallelPublishAfterFetch = func() {
				matches, err := filepath.Glob(filepath.Join(workspaceRoot, "taskrail-parallel-*", "integration"))
				if err != nil || len(matches) != 1 {
					t.Fatalf("integration paths = %v, %v", matches, err)
				}
				integrationPath = matches[0]
				writeFile(t, filepath.Join(repo, "source-drift.txt"), "caller-owned drift\n")
			}
			t.Cleanup(func() { testHookParallelPublishAfterFetch = nil })
			child := []string{helper, "-test.run=^TestLoopLaunchChildHelper$", "--", "parallel-worker", filepath.Join(t.TempDir(), "unused")}
			report, err := svc.LoopExecute(context.Background(), LoopInvocation{MaxIterations: 2, Parallel: 2,
				KeepWorkspaces: retention, Child: child})
			if err != nil {
				t.Fatal(err)
			}
			batch := report.Parallel
			if report.Outcome != "batch_failed" || batch == nil || !batch.Integration.AggregatePass || integrationPath == "" ||
				!hasLoopIntegrityCode(report.MutationViolations, "source_drift") || len(batch.Delivery.PublishedTasks) != 0 {
				t.Fatalf("publication refusal = %+v", report)
			}
			if got := strings.Join(batch.Delivery.PendingTasks, ","); got != "T-001-ready,T-002-ready" {
				t.Errorf("pending tasks = %q", got)
			}
			if head, err := gitCommand(repo, "rev-parse", "HEAD"); err != nil || head != base {
				t.Fatalf("source head = %q, %v; want %q", head, err, base)
			}
			if retention == "never" {
				if batch.Integration.Workspace != nil || !strings.Contains(report.NextAction, "lost") {
					t.Errorf("removed integration = %+v; next action = %q", batch.Integration, report.NextAction)
				}
				if _, err := os.Stat(filepath.Dir(integrationPath)); !os.IsNotExist(err) {
					t.Errorf("never retained invocation root: %v", err)
				}
				return
			}
			if batch.Integration.Workspace == nil || *batch.Integration.Workspace != integrationPath {
				t.Fatalf("retained integration path = %v; want %s", batch.Integration.Workspace, integrationPath)
			}
			if head, err := gitCommand(integrationPath, "rev-parse", "HEAD"); err != nil || strings.TrimSpace(head) != *batch.Integration.Head {
				t.Fatalf("retained integration HEAD = %q, %v", head, err)
			}
			integrated, err := NewService(integrationPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, service := range []*Service{svc, integrated} {
				tasks, err := service.loadTasks()
				if err != nil {
					t.Fatal(err)
				}
				for _, worker := range batch.Workers {
					task, found := taskByIDFromSlice(tasks, worker.Task.TaskID)
					want := "todo"
					if service == integrated {
						want = "completed"
					}
					if !worker.Integrated || !found || task.Frontmatter.Status != want {
						t.Errorf("task %s status = %+v; want %s", worker.Task.TaskID, task, want)
					}
				}
			}
		})
	}
}
