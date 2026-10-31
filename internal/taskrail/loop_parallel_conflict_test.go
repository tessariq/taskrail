package taskrail

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoopExecuteParallelConflictBoundary(t *testing.T) {
	binary := buildTaskrailExecutable(t)
	previous := loopExecutablePath
	loopExecutablePath = func() (string, error) { return binary, nil }
	t.Cleanup(func() { loopExecutablePath = previous })
	for _, mode := range []string{"commit", "move-head", "unmerged", "missing-fetch-head", "resolve"} {
		t.Run(mode, func(t *testing.T) {
			clearLoopChildEnvironment(t)
			t.Setenv("GO_WANT_LOOP_CHILD", "1")
			repo, svc := loopFixture(t)
			for _, id := range []string{"T-001-ready", "T-002-ready", "T-003-ready"} {
				writeTask(t, repo, id, id, "todo", "high", "specs/v0.1.0.md#summary", nil)
				setLoopPolicy(t, repo, id, "allow", "independent work")
			}
			// Preserve exact fixture bytes regardless of the host's checkout conversion.
			writeFile(t, filepath.Join(repo, ".gitattributes"), "shared.txt text eol=lf\nindependent.txt text eol=lf\n")
			writeFile(t, filepath.Join(repo, "shared.txt"), "base\n")
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-m", "prepare conflict batch")
			helper, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			workspaceRoot := t.TempDir()
			// Windows Chmod cannot satisfy the explicit workspace's Unix mode guard.
			// Use the default workspace path, but retain test-owned cleanup.
			for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
				t.Setenv(name, workspaceRoot)
			}
			report, err := svc.LoopExecute(context.Background(), LoopInvocation{MaxIterations: 3, Parallel: 3, CloneDepth: "full", KeepWorkspaces: "always",
				Child: []string{helper, "-test.run=^TestLoopLaunchChildHelper$", "--", "parallel-conflict-worker", mode}})
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.MarshalIndent(report, "", "  ")
			wantOutcome, wantChild, wantShared := "batch_partial", "fail", "T-001-ready\n"
			wantTasks := "T-001-ready,T-003-ready"
			if mode == "resolve" {
				wantOutcome, wantChild, wantShared = "batch_pass", "pass", "T-002-ready\n"
				wantTasks = "T-001-ready,T-002-ready,T-003-ready"
			}
			if report.Outcome != wantOutcome || report.Parallel == nil {
				t.Fatalf("report = %s", encoded)
			}
			batch := report.Parallel
			if strings.Join(batch.Delivery.PublishedTasks, ",") != wantTasks {
				t.Fatalf("published tasks = %s", encoded)
			}
			children := batch.Integration.Children
			if len(children) != 2 || children[0].Role != "conflict_resolution" || children[0].Outcome != wantChild || children[0].TaskID == nil || *children[0].TaskID != "T-002-ready" || children[0].BoundHead != *batch.Workers[0].IntegratedCommit || children[1].Outcome != "pass" {
				t.Fatalf("children = %s", encoded)
			}
			for _, child := range children {
				if child.Child.ExitCode == nil || *child.Child.ExitCode != 0 {
					t.Fatalf("expected successful subprocess, got %+v", child)
				}
			}
			if mode == "missing-fetch-head" && !strings.Contains(string(encoded), "read candidate commit metadata") {
				t.Fatalf("expected downstream metadata failure: %s", encoded)
			}
			for _, root := range []string{repo, *batch.Integration.Workspace} {
				data, err := os.ReadFile(filepath.Join(root, "shared.txt"))
				if err != nil || string(data) != wantShared {
					t.Fatalf("%s shared bytes = %q, %v; report %s", root, data, err, encoded)
				}
				data, err = os.ReadFile(filepath.Join(root, "independent.txt"))
				if err != nil || string(data) != "T-003-ready\n" {
					t.Fatalf("later candidate bytes = %q, %v", data, err)
				}
				if status, err := gitCommand(root, "status", "--porcelain"); err != nil || status != "" || parallelUnmerged(root) {
					t.Fatalf("dirty integration/source: %s, %v", status, err)
				}
				head, err := gitCommand(root, "rev-parse", "HEAD")
				if err != nil || strings.TrimSpace(head) != *batch.Integration.Head {
					t.Fatalf("head = %q, %v", head, err)
				}
			}
			tasks, err := svc.loadTasks()
			if err != nil {
				t.Fatal(err)
			}
			for _, task := range tasks {
				want := "completed"
				if task.Frontmatter.ID == "T-002-ready" && mode != "resolve" {
					want = "todo"
				}
				if task.Frontmatter.Status != want {
					t.Fatalf("task %s status = %s, want %s", task.Frontmatter.ID, task.Frontmatter.Status, want)
				}
			}
			if mode != "resolve" {
				parent, err := gitCommand(repo, "rev-parse", *batch.Workers[2].IntegratedCommit+"^")
				if err != nil || strings.TrimSpace(parent) != children[0].BoundHead {
					t.Fatalf("later parent %q != rollback head %s: %v", parent, children[0].BoundHead, err)
				}
			}
		})
	}
}
