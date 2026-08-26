package taskrail

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoopExecuteReportsInitialNoWorkWithoutLaunchingChild(t *testing.T) {
	clearLoopChildEnvironment(t)
	_, svc := loopFixture(t)

	report, err := svc.LoopExecute(context.Background(), LoopInvocation{MaxIterations: 1, Child: []string{"not-run"}})
	if err != nil {
		t.Fatalf("LoopExecute: %v", err)
	}
	if report.Outcome != "no_work" || report.LastIteration != nil || report.IterationsCompleted != 0 || report.RemainingTask != nil {
		t.Fatalf("report = %+v, want initial no-work diagnostic", report)
	}
	if report.Remote != "not_checked" || report.NextAction == "" || report.Executable.Path == "" {
		t.Fatalf("unsafe or incomplete diagnostic = %+v", report)
	}
}

func TestLoopExecuteRefusesUnauthorizedReplacementPrompt(t *testing.T) {
	clearLoopChildEnvironment(t)
	repo, svc := loopFixture(t)
	promptPath := filepath.Join(repo, ".taskrail", "prompts", "v1", "task-implementation.md")
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("create prompt directory: %v", err)
	}
	template, err := builtinPrompts.ReadFile("prompts/v1/task-implementation.md")
	if err != nil {
		t.Fatalf("read builtin prompt: %v", err)
	}
	if err := os.WriteFile(promptPath, append(template, []byte("\nReplacement authorization test.\n")...), 0o644); err != nil {
		t.Fatalf("write replacement prompt: %v", err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "replacement prompt")

	_, err = svc.LoopExecute(context.Background(), LoopInvocation{MaxIterations: 1, Child: []string{"not-run"}})
	if MachineFailureFor(err).Code != MachineCodePromptInvalid {
		t.Fatalf("LoopExecute error = %v, want prompt_invalid", err)
	}
}

func TestLoopDiagnosticGitHasNoRemoteMember(t *testing.T) {
	report := loopDiagnosticBase(LoopPreflightSnapshot{git: LoopGitSnapshot{Ref: "refs/heads/main", Head: "before", Clean: true}, storage: LoopStorageSnapshot{Mode: "committed", Root: "."}, review: LoopReviewSnapshot{ConfiguredMaxRounds: 1, EffectiveMaxRounds: 1, MaxReviewersPerRound: 3, FinalDiffReviewRequiredOnChange: true, Source: "config"}}, &loopOwnership{invocation: "id", executable: loopStagedExecutable{Path: "/staged", SHA256: "digest"}})
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal diagnostic: %v", err)
	}
	var raw struct {
		Git map[string]json.RawMessage `json:"git"`
	}
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("decode diagnostic: %v", err)
	}
	if _, present := raw.Git["remote"]; present {
		t.Fatalf("terminal git includes remote: %s", encoded)
	}
}

func TestNextLoopIterationSnapshotAdvancesGitEvidence(t *testing.T) {
	clearLoopChildEnvironment(t)
	repo, svc := loopFixture(t)
	before, err := svc.LoopPreflight(LoopInvocation{MaxIterations: 2, Child: []string{"not-run"}})
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	runGit(t, repo, "commit", "--allow-empty", "-m", "delivered first task")
	after, err := svc.nextLoopIterationSnapshot(before)
	if err != nil {
		t.Fatalf("next iteration snapshot: %v", err)
	}
	if after.Git().Head == before.Git().Head || after.Git().Head == "" {
		t.Fatalf("Git evidence did not advance: before=%+v after=%+v", before.Git(), after.Git())
	}
}

func TestLoopExecuteStopsAfterFailedChild(t *testing.T) {
	clearLoopChildEnvironment(t)
	t.Setenv("GO_WANT_LOOP_CHILD", "1")
	repo, svc := loopFixture(t)
	writeTask(t, repo, "T-001-ready", "Ready", "todo", "high", "specs/v0.1.0.md#summary", nil)
	taskPath := repo + "/planning/tasks/T-001-ready.md"
	task := string(readBytes(t, taskPath))
	writeFile(t, taskPath, strings.Replace(task, "updated_at:", "loop_policy: allow\nloop_reason: test execution\nupdated_at:", 1))
	if _, err := svc.Repair(RepairInput{Apply: true}); err != nil {
		t.Fatalf("repair loop fixture: %v", err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "allow task")

	record := filepath.Join(t.TempDir(), "failed-child")
	report, err := svc.LoopExecute(context.Background(), LoopInvocation{MaxIterations: 2, Child: []string{os.Args[0], "-test.run=^TestLoopLaunchChildHelper$", "--", "nonzero", record}})
	if err != nil {
		t.Fatalf("LoopExecute: %v", err)
	}
	if report.Outcome != "invalid_postflight" || report.LastIteration == nil || report.LastIteration.TaskID != "T-001-ready" || report.IterationsCompleted != 1 {
		t.Fatalf("report = %+v, want one failed child and invalid delivery", report)
	}
	if report.RemainingTask != nil || report.NextAction == "" {
		t.Fatalf("report recovery shape = %+v", report)
	}
}

func TestLoopExecuteRejectsGitConfigurationMutation(t *testing.T) {
	clearLoopChildEnvironment(t)
	t.Setenv("GO_WANT_LOOP_CHILD", "1")
	repo, svc := loopFixture(t)
	writeTask(t, repo, "T-001-ready", "Ready", "todo", "high", "specs/v0.1.0.md#summary", nil)
	taskPath := filepath.Join(repo, "planning", "tasks", "T-001-ready.md")
	writeFile(t, taskPath, strings.Replace(string(readBytes(t, taskPath)), "updated_at:", "loop_policy: allow\nloop_reason: test execution\nupdated_at:", 1))
	if _, err := svc.Repair(RepairInput{Apply: true}); err != nil {
		t.Fatalf("repair loop fixture: %v", err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "allow task")

	report, err := svc.LoopExecute(context.Background(), LoopInvocation{MaxIterations: 1, Child: []string{os.Args[0], "-test.run=^TestLoopLaunchChildHelper$", "--", "mutate-git-config", filepath.Join(t.TempDir(), "record")}})
	if err != nil {
		t.Fatalf("LoopExecute: %v", err)
	}
	if report.Outcome != "invalid_postflight" || !hasLoopIntegrityCode(report.MutationViolations, "git_config_changed") {
		t.Fatalf("report = %+v, want invalid Git configuration postflight", report)
	}
}
