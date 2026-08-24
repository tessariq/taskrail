package taskrail

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRemoveParallelRootClosesHandleBeforeRemovingDirectory(t *testing.T) {
	parent := t.TempDir()
	path, err := os.MkdirTemp(parent, "taskrail-parallel-")
	if err != nil {
		t.Fatalf("create parallel root: %v", err)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatalf("open parallel root: %v", err)
	}
	owner := parallelWorkspaceOwner{path: path, root: root}
	if err := owner.root.WriteFile(".taskrail-parallel-owner", []byte(path), 0o600); err != nil {
		t.Fatalf("write ownership marker: %v", err)
	}

	if err := removeParallelRoot(owner); err != nil {
		t.Fatalf("removeParallelRoot: %v", err)
	}
	if _, err := owner.root.Lstat("."); err == nil {
		t.Fatal("parallel root handle remained open after removal")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("parallel root remains after removal: %v", err)
	}
}

func TestCleanupParallelWorkspacesClosesRootOnEveryDisposition(t *testing.T) {
	retainedWorkspace := "retained"
	tests := []struct {
		name       string
		retention  string
		batch      ParallelBatch
		badMarker  bool
		wantExists bool
		wantErrors int
	}{
		{name: "retention never", retention: "never"},
		{name: "no retained workspaces", retention: "failure"},
		{name: "retained workspaces", retention: "failure", batch: ParallelBatch{Workers: []ParallelWorker{{Workspace: &retainedWorkspace}}}, wantExists: true},
		{name: "ownership error", retention: "never", badMarker: true, wantExists: true, wantErrors: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := t.TempDir()
			path, err := os.MkdirTemp(parent, "taskrail-parallel-")
			if err != nil {
				t.Fatalf("create parallel root: %v", err)
			}
			root, err := os.OpenRoot(path)
			if err != nil {
				t.Fatalf("open parallel root: %v", err)
			}
			owner := parallelWorkspaceOwner{path: path, root: root}
			marker := path
			if tt.badMarker {
				marker = path + "-changed"
			}
			if err := owner.root.WriteFile(".taskrail-parallel-owner", []byte(marker), 0o600); err != nil {
				t.Fatalf("write ownership marker: %v", err)
			}

			violations := (&Service{}).cleanupParallelWorkspaces(owner, tt.retention, &tt.batch)
			if len(violations) != tt.wantErrors {
				t.Fatalf("cleanup violations = %+v, want %d", violations, tt.wantErrors)
			}
			for _, violation := range violations {
				if violation.Code != "cleanup_failed" {
					t.Fatalf("cleanup violation = %+v, want cleanup_failed", violation)
				}
			}
			if tt.badMarker && !strings.Contains(violations[0].Message, "ownership marker does not match") {
				t.Fatalf("ownership violation = %+v", violations[0])
			}
			if _, err := owner.root.Lstat("."); err == nil {
				t.Fatal("parallel root handle remained open after cleanup")
			}
			_, err = os.Stat(path)
			if tt.wantExists && err != nil {
				t.Fatalf("retained parallel root: %v", err)
			}
			if !tt.wantExists && !os.IsNotExist(err) {
				t.Fatalf("parallel root remains after cleanup: %v", err)
			}
		})
	}
}

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
	t.Setenv("GO_WANT_LOOP_CHILD", "1")
	isolatedConfig := t.TempDir()
	t.Setenv("HOME", isolatedConfig)
	t.Setenv("USERPROFILE", isolatedConfig)
	t.Setenv("XDG_CONFIG_HOME", isolatedConfig)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(isolatedConfig, "missing-global-config"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	helper, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test helper: %v", err)
	}
	child := []string{helper, "-test.run=^TestLoopLaunchChildHelper$", "--", "parallel-worker", filepath.Join(t.TempDir(), "unused")}

	report, err := svc.LoopExecute(context.Background(), LoopInvocation{MaxIterations: 2, Parallel: 2, Child: child})
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
	metadata, err := gitCommand(repo, "show", "-s", "--format=%an%x00%ae%x00%aI%x00%cn%x00%ce%x00%cI%x00%B", *report.Parallel.Integration.Head)
	if err != nil {
		t.Fatalf("read integrated metadata: %v", err)
	}
	fields := strings.SplitN(strings.TrimSpace(metadata), "\x00", 7)
	if len(fields) != 7 {
		t.Fatalf("integrated metadata fields = %q", fields)
	}
	if fields[0] != "Parallel Worker" || fields[1] != "parallel-worker@example.com" ||
		fields[3] != "Parallel Worker" || fields[4] != "parallel-worker@example.com" {
		t.Fatalf("integrated author/committer identity = %q", fields[:6])
	}
	wantTimestamp := time.Date(2001, time.February, 3, 4, 5, 6, 0, time.UTC)
	for _, field := range []int{2, 5} {
		got, err := time.Parse(time.RFC3339, fields[field])
		if err != nil || !got.Equal(wantTimestamp) {
			t.Fatalf("integrated timestamp %q = %v, %v; want %v", fields[field], got, err, wantTimestamp)
		}
	}
	if fields[6] != "complete T-002-ready" {
		t.Fatalf("integrated commit message = %q", fields[6])
	}
	for _, taskID := range []string{"T-001-ready", "T-002-ready"} {
		tasks, err := svc.loadTasks()
		task, found := taskByIDFromSlice(tasks, taskID)
		if err != nil || !found || task.Frontmatter.Status != "completed" {
			t.Fatalf("task %s after parallel delivery = %+v, %v", taskID, task, err)
		}
	}
}

func TestCanonicalParallelRootResolvesSymlinkedParent(t *testing.T) {
	realParent := t.TempDir()
	aliasParent := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(realParent, aliasParent); err != nil {
		t.Skipf("create parent symlink: %v", err)
	}
	original, err := os.MkdirTemp(aliasParent, "taskrail-parallel-")
	if err != nil {
		t.Fatalf("create parallel root: %v", err)
	}
	canonical, err := canonicalParallelRoot(original)
	if err != nil {
		t.Fatalf("canonicalize parallel root: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(canonical) })
	want, err := filepath.EvalSymlinks(original)
	if err != nil {
		t.Fatalf("resolve expected parallel root: %v", err)
	}
	if canonical != want {
		t.Fatalf("canonical parallel root = %q, want %q", canonical, want)
	}
	worker := filepath.Join(canonical, "worker-01")
	rel, err := filepath.Rel(canonical, worker)
	if err != nil || rel != "worker-01" {
		t.Fatalf("canonical worker containment = %q, %v", rel, err)
	}
}

func buildParallelTaskrail(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}
	binary := filepath.Join(t.TempDir(), "taskrail")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	command := exec.Command("go", "build", "-o", binary, "./cmd/taskrail")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build taskrail: %v\n%s", err, output)
	}
	return binary
}

func runParallelLoopChild() int {
	prompt, err := io.ReadAll(os.Stdin)
	if err != nil {
		return 98
	}
	first := strings.SplitN(string(prompt), "\n", 2)[0]
	if strings.HasPrefix(first, "Run the repository's full aggregate validation") {
		return 0
	}
	fields := strings.Fields(first)
	if len(fields) < 5 || strings.Join(fields[:4], " ") != "Implement the selected task" {
		return 97
	}
	taskID := strings.TrimSuffix(fields[4], ".")
	commands := [][]string{
		{os.Getenv("TASKRAIL"), "start", taskID},
		{os.Getenv("TASKRAIL"), "complete", taskID, "--note", "parallel test"},
		{os.Getenv("TASKRAIL"), "verify", taskID, "--result", "pass", "--summary", "parallel test", "--details", "parallel test"},
		{"git", "add", "planning"},
		{"git", "commit", "-m", "complete " + taskID},
	}
	for _, args := range commands {
		command := exec.Command(args[0], args[1:]...)
		if args[0] == "git" && args[1] == "commit" {
			command.Env = append(os.Environ(),
				"GIT_AUTHOR_NAME=Parallel Worker", "GIT_AUTHOR_EMAIL=parallel-worker@example.com", "GIT_AUTHOR_DATE=2001-02-03T04:05:06Z",
				"GIT_COMMITTER_NAME=Parallel Worker", "GIT_COMMITTER_EMAIL=parallel-worker@example.com", "GIT_COMMITTER_DATE=2001-02-03T04:05:06Z")
		}
		if output, err := command.CombinedOutput(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "%v: %v\n%s", args, err, output)
			return 96
		}
	}
	return 0
}
