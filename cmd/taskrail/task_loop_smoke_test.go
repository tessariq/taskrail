package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/taskrail"
)

func TestTaskLoopListPublishesRowsAndStaysReadOnly(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-001-held", "todo", "")
	writeTask(t, root, "T-002-ready", "todo", "")
	before := readAllFiles(t, root)

	stdout, err := runRoot(t, "task", "loop", "list", "--json")
	if err != nil {
		t.Fatalf("task loop list: %v (stdout %q)", err, stdout)
	}
	envelope, err := taskrail.DecodeMachineEnvelope([]byte(stdout))
	if err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Command != "task loop list" {
		t.Fatalf("command = %q", envelope.Command)
	}
	var report taskrail.TaskLoopListResult
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if len(report.Tasks) != 2 || report.Tasks[0].Source != "default" || report.Tasks[0].Disposition != "held" {
		t.Fatalf("report rows = %+v", report.Tasks)
	}
	if after := readAllFiles(t, root); !reflect.DeepEqual(after, before) {
		t.Fatal("task loop list changed repository files")
	}
}

func TestTaskLoopMutationPublishesPreviewAndApply(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-001-target", "todo", "")
	taskPath := filepath.Join(root, "planning", "tasks", "T-001-target.md")
	before := readAllFiles(t, root)

	stdout, err := runRoot(t, "task", "loop", "allow", "T-001-target", "--reason", "bounded change", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("allow preview: %v (%s)", err, stdout)
	}
	var preview taskrail.LoopPolicyMutationResult
	decodeMachineResult(t, stdout, &preview)
	if preview.Applied || preview.Operation != taskrail.LoopPolicyAllow || preview.Prior.Source != "default" || preview.Candidate.EffectivePolicy != "allow" || preview.Candidate.PersistedReason == nil || *preview.Candidate.PersistedReason != "bounded change" || !preview.Validation.Valid {
		t.Fatalf("allow preview = %+v", preview)
	}
	if after := readAllFiles(t, root); !reflect.DeepEqual(after, before) {
		t.Fatal("allow preview changed repository files")
	}

	stdout, err = runRoot(t, "task", "loop", "allow", "T-001-target", "--reason", "bounded change", "--json")
	if err != nil {
		t.Fatalf("allow apply: %v (%s)", err, stdout)
	}
	var applied taskrail.LoopPolicyMutationResult
	decodeMachineResult(t, stdout, &applied)
	if !applied.Applied || !reflect.DeepEqual(applied.Prior, preview.Prior) || !reflect.DeepEqual(applied.Candidate, preview.Candidate) {
		t.Fatalf("allow apply = %+v, preview = %+v", applied, preview)
	}
	data, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "loop_policy: allow") || !strings.Contains(string(data), "loop_reason: \"bounded change\"") {
		t.Fatalf("allow did not persist pair:\n%s", data)
	}

	stdout, err = runRoot(t, "task", "loop", "clear", "T-001-target", "--json")
	if err != nil {
		t.Fatalf("clear apply: %v (%s)", err, stdout)
	}
	var cleared taskrail.LoopPolicyMutationResult
	decodeMachineResult(t, stdout, &cleared)
	if !cleared.Applied || cleared.Candidate.Source != "default" || cleared.Candidate.PersistedPolicy != nil || cleared.Candidate.PersistedReason != nil {
		t.Fatalf("clear result = %+v", cleared)
	}
}

func TestLoopDryRunPublishesRunAndInvalidReportsWithoutMutation(t *testing.T) {
	root := setupLoopDryRunRepo(t)
	writeTask(t, root, "T-001-ready", "todo", "")
	taskPath := filepath.Join(root, "planning", "tasks", "T-001-ready.md")
	task, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	if err := os.WriteFile(taskPath, []byte(strings.Replace(string(task), "updated_at:", "loop_policy: allow\nloop_reason: independent work\nupdated_at:", 1)), 0o644); err != nil {
		t.Fatalf("allow task: %v", err)
	}
	if _, err := runRoot(t, "repair", "--apply"); err != nil {
		t.Fatalf("repair fixture state: %v", err)
	}
	runLoopGit(t, root, "add", ".")
	runLoopGit(t, root, "commit", "-m", "allow loop task")
	if output, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate loop fixture: %v (%s)", err, output)
	}

	before := readAllFiles(t, root)
	statusBefore := loopGitOutput(t, root, "status", "--porcelain=v1")
	refsBefore := loopGitOutput(t, root, "for-each-ref", "--format=%(refname):%(objectname)")
	stdout, _, err := runRootSplit(t, "loop", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("loop dry run: %v (stdout %q)", err, stdout)
	}
	var report struct {
		Action       string                        `json:"action"`
		SelectedTask *taskrail.TaskLoopRow         `json:"selected_task"`
		Prompt       *taskrail.LoopPromptExecution `json:"prompt"`
		Review       *taskrail.LoopReviewPolicy    `json:"review"`
		Parallel     any                           `json:"parallel"`
	}
	decodeMachineResult(t, stdout, &report)
	if report.Action != "run" || report.SelectedTask == nil || report.SelectedTask.TaskID != "T-001-ready" || report.Prompt == nil || report.Prompt.Source != "builtin" || !report.Prompt.OverrideAuthorized || report.Review == nil || report.Review.EffectiveMaxRounds != 1 || report.Parallel != nil {
		t.Fatalf("dry-run report = %+v", report)
	}
	if after := readAllFiles(t, root); !reflect.DeepEqual(after, before) || loopGitOutput(t, root, "status", "--porcelain=v1") != statusBefore || loopGitOutput(t, root, "for-each-ref", "--format=%(refname):%(objectname)") != refsBefore {
		t.Fatal("loop dry run changed the sandbox")
	}
	stdout, _, err = runRootSplit(t, "loop", "--dry-run", "--parallel", "2", "--max-iterations", "2", "--json")
	if err != nil {
		t.Fatalf("parallel loop dry run: %v (stdout %q)", err, stdout)
	}
	var parallel struct {
		Action       string                        `json:"action"`
		SelectedTask *taskrail.TaskLoopRow         `json:"selected_task"`
		Prompt       *taskrail.LoopPromptExecution `json:"prompt"`
		Parallel     *taskrail.ParallelPlan        `json:"parallel"`
	}
	decodeMachineResult(t, stdout, &parallel)
	if parallel.Action != "run" || parallel.SelectedTask != nil || parallel.Prompt != nil || parallel.Parallel == nil || parallel.Parallel.RequestedWidth != 2 || parallel.Parallel.EffectiveWidth != 2 || len(parallel.Parallel.Frontier) != 1 || parallel.Parallel.Frontier[0].Task.TaskID != "T-001-ready" {
		t.Fatalf("parallel dry-run report = %+v", parallel)
	}
	if after := readAllFiles(t, root); !reflect.DeepEqual(after, before) || loopGitOutput(t, root, "status", "--porcelain=v1") != statusBefore || loopGitOutput(t, root, "for-each-ref", "--format=%(refname):%(objectname)") != refsBefore {
		t.Fatal("parallel loop dry run changed the sandbox")
	}
	stdout, _, err = runRootSplit(t, "loop", "--dry-run", "--max-review-rounds", "2", "--timeout", "30s", "--json")
	if err != nil {
		t.Fatalf("overridden loop dry run: %v (stdout %q)", err, stdout)
	}
	var overridden struct {
		Action string `json:"action"`
		Review struct {
			Configured int    `json:"configured_max_rounds"`
			Effective  int    `json:"effective_max_rounds"`
			Source     string `json:"source"`
		} `json:"review"`
		Execution struct {
			Timeout *string `json:"timeout"`
			Source  string  `json:"timeout_source"`
		} `json:"execution"`
	}
	decodeMachineResult(t, stdout, &overridden)
	if overridden.Action != "run" || overridden.Review.Configured != 1 || overridden.Review.Effective != 2 || overridden.Review.Source != "flag" || overridden.Execution.Timeout == nil || *overridden.Execution.Timeout != "30s" || overridden.Execution.Source != "flag" {
		t.Fatalf("overridden dry-run report = %+v", overridden)
	}

	stdout, _, err = runRootSplit(t, "loop", "--dry-run", "--allow-prompt-override-sha256", strings.Repeat("0", 64), "--json")
	if err == nil {
		t.Fatal("stale built-in authorization must gate the dry run")
	}
	envelope, decodeErr := taskrail.DecodeMachineEnvelope([]byte(stdout))
	if decodeErr != nil || envelope.Error == nil || envelope.Error.Code != taskrail.MachineCodeInvalidArguments {
		t.Fatalf("invalid authorization envelope = %+v (decode error %v)", envelope, decodeErr)
	}
}

func TestLoopExecutionRejectsJSONBeforeRepositoryDiscovery(t *testing.T) {
	_, _, err := runRootSplit(t, "loop", "--json", "--", "/bin/true")
	if err == nil {
		t.Fatal("loop execution accepted --json")
	}
}

func TestLoopExecutionPublishesTerminalResultOutOfBand(t *testing.T) {
	clearLoopExecutionEnvironment(t)
	root := setupLoopDryRunRepo(t)
	result := filepath.Join(t.TempDir(), "loop-result.json")
	before := readAllFiles(t, root)

	stdout, _, err := runRootSplit(t, "loop", "--result-file", result, "--", "not-run")
	if err != nil {
		t.Fatalf("loop execution: %v (stdout %q)", err, stdout)
	}
	if strings.Contains(stdout, `"schema_version"`) {
		t.Fatalf("loop stdout contains result document: %q", stdout)
	}
	document, err := os.ReadFile(result)
	if err != nil {
		t.Fatalf("read result file: %v", err)
	}
	envelope := decodeEnvelope(t, string(document))
	if envelope.Command != "loop" || envelope.Error != nil {
		t.Fatalf("result envelope = %+v", envelope)
	}
	var diagnostic taskrail.LoopDiagnostic
	if err := json.Unmarshal(envelope.Result, &diagnostic); err != nil {
		t.Fatalf("decode diagnostic: %v", err)
	}
	if diagnostic.Outcome != "no_work" || diagnostic.LastIteration != nil {
		t.Fatalf("diagnostic = %+v", diagnostic)
	}
	if after := readAllFiles(t, root); !reflect.DeepEqual(after, before) {
		t.Fatal("result publication changed repository files")
	}
}

func TestLoopExecutionPublishesPostflightFailureOutOfBand(t *testing.T) {
	clearLoopExecutionEnvironment(t)
	root := setupLoopDryRunRepo(t)
	writeTask(t, root, "T-001-ready", "todo", "")
	taskPath := filepath.Join(root, "planning", "tasks", "T-001-ready.md")
	task, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	if err := os.WriteFile(taskPath, []byte(strings.Replace(string(task), "updated_at:", "loop_policy: allow\nloop_reason: test execution\nupdated_at:", 1)), 0o644); err != nil {
		t.Fatalf("allow task: %v", err)
	}
	if _, err := runRoot(t, "repair", "--apply"); err != nil {
		t.Fatalf("repair fixture state: %v", err)
	}
	runLoopGit(t, root, "add", ".")
	runLoopGit(t, root, "commit", "-m", "allow loop task")
	result := filepath.Join(t.TempDir(), "loop-result.json")

	_, _, err = runRootSplit(t, "loop", "--result-file", result, "--", filepath.Join(t.TempDir(), "missing-child"))
	if err == nil {
		t.Fatal("loop launch failure exited zero")
	}
	document, err := os.ReadFile(result)
	if err != nil {
		t.Fatalf("read result file: %v", err)
	}
	var envelope struct {
		Command string `json:"command"`
		Error   struct {
			Code    string `json:"code"`
			Details struct {
				Outcome string `json:"outcome"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(document, &envelope); err != nil {
		t.Fatalf("decode postflight result: %v", err)
	}
	if envelope.Command != "loop" || envelope.Error.Code != "invalid_postflight" || envelope.Error.Code != envelope.Error.Details.Outcome {
		t.Fatalf("postflight result = %+v", envelope)
	}
}

func clearLoopExecutionEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{"TASKRAIL", "TASKRAIL_EXECUTABLE_SHA256", "TASKRAIL_DELEGATION_ID", "TASKRAIL_DELEGATION_TOKEN"} {
		old, present := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
		t.Cleanup(func() {
			if present {
				_ = os.Setenv(name, old)
			}
		})
	}
}

func TestLoopExecutionRefusesManagedResultFileWithoutPublication(t *testing.T) {
	root := setupLoopDryRunRepo(t)
	result := filepath.Join(root, "result.json")

	_, _, err := runRootSplit(t, "loop", "--result-file", result, "--", "not-run")
	if err == nil {
		t.Fatal("loop accepted a managed result destination")
	}
	if _, err := os.Stat(result); !os.IsNotExist(err) {
		t.Fatalf("managed result destination was published: %v", err)
	}
	if status := loopGitOutput(t, root, "status", "--porcelain=v1"); status != "" {
		t.Fatalf("managed result destination dirtied repository: %q", status)
	}
}

func TestLoopRejectsRepeatedParallelFlag(t *testing.T) {
	stdout, _, err := runRootSplit(t, "loop", "--dry-run", "--parallel", "2", "--parallel", "3", "--max-iterations", "3", "--json")
	if err == nil {
		t.Fatal("repeated --parallel must fail before repository discovery")
	}
	envelope, decodeErr := taskrail.DecodeMachineEnvelope([]byte(stdout))
	if decodeErr != nil || envelope.Error == nil || envelope.Error.Code != taskrail.MachineCodeInvalidArguments {
		t.Fatalf("repeated parallel envelope = %+v (decode error %v)", envelope, decodeErr)
	}
}

func setupLoopDryRunRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runLoopGit(t, root, "init", "--quiet")
	runLoopGit(t, root, "config", "user.email", "taskrail@example.test")
	runLoopGit(t, root, "config", "user.name", "Taskrail Test")
	t.Chdir(root)
	if _, err := runRoot(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".taskrail", "config.yml"), []byte("layout_version: 2\nspecs_dir: specs\nplanning_dir: planning\nstorage_mode: committed\nimplementation_review_max_rounds: 1\n"), 0o644); err != nil {
		t.Fatalf("write layout-2 fixture: %v", err)
	}
	runLoopGit(t, root, "add", ".")
	runLoopGit(t, root, "commit", "-m", "taskrail layout")
	return root
}

func runLoopGit(t *testing.T, root string, args ...string) {
	t.Helper()
	if output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v (%s)", strings.Join(args, " "), err, output)
	}
}

func loopGitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	output, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(output)
}

func TestTaskLoopListReturnsGatedReportForInvalidTask(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-001-invalid", "wat", "")

	stdout, err := runRoot(t, "task", "loop", "list", "--json")
	if err == nil {
		t.Fatal("invalid list must exit non-zero")
	}
	envelope, decodeErr := taskrail.DecodeMachineEnvelope([]byte(stdout))
	if decodeErr != nil {
		t.Fatalf("decode gated envelope: %v (stdout %q)", decodeErr, stdout)
	}
	var report taskrail.TaskLoopListResult
	if decodeErr := json.Unmarshal(envelope.Result, &report); decodeErr != nil {
		t.Fatalf("decode gated result: %v", decodeErr)
	}
	if len(report.Violations) == 0 || len(report.Tasks) != 1 || report.Tasks[0].Disposition != "invalid" {
		t.Fatalf("gated report = %+v", report)
	}
}
