package taskrail

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoopExecuteDeliversTwoFreshVerifications(t *testing.T) {
	clearLoopChildEnvironment(t)
	t.Setenv("GO_WANT_LOOP_CHILD", "1")
	binary := buildTaskrailExecutable(t)
	previous := loopExecutablePath
	loopExecutablePath = func() (string, error) { return binary, nil }
	t.Cleanup(func() { loopExecutablePath = previous })
	check := buildLoopCheck(t)
	for _, checked := range []bool{false, true} {
		t.Run(fmt.Sprint(checked), func(t *testing.T) {
			repo, svc := loopFixture(t)
			writeFile(t, filepath.Join(repo, ".gitattributes"), "* text eol=lf\n")
			ids := []string{"T-001-ready", "T-002-ready"}
			for _, id := range ids {
				writeTask(t, repo, id, "Sequential delivery", "todo", "high", "specs/v0.1.0.md#summary", nil)
				if _, err := svc.MutateTaskLoopPolicy(LoopPolicyMutationInput{TaskID: id, Operation: LoopPolicyAllow, Reason: "sequential delivery"}); err != nil {
					t.Fatal(err)
				}
			}
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-m", "allow sequential tasks")
			base := gitOutput(t, repo, "rev-parse", "HEAD")
			record := filepath.Join(t.TempDir(), "check-heads")
			t.Setenv("LOOP_CHECK_RECORD", record)
			t.Setenv("LOOP_CHECK_MODE", "pass")
			input := LoopInvocation{MaxIterations: 2, Child: []string{os.Args[0], "-test.run=^TestLoopLaunchChildHelper$", "--", "parallel-worker", "unused"}}
			if checked {
				input.Check = check
			}
			report, err := svc.LoopExecute(context.Background(), input)
			if err != nil || report.Outcome != "no_work" || report.IterationsCompleted != 2 {
				t.Fatalf("two-task delivery: %+v, %v", report, err)
			}
			commits := strings.Fields(gitOutput(t, repo, "rev-list", "--reverse", base+"..HEAD"))
			if len(commits) != 2 || gitOutput(t, repo, "status", "--porcelain") != "" {
				t.Fatalf("delivery commits/cleanliness: %v", commits)
			}
			seen := map[string]bool{}
			for i, id := range ids {
				task, state, reports, _ := svc.loopPostflightLifecycle(id)
				meta := task.Frontmatter.CompletionVerificationMetadata
				if task.Frontmatter.Status != "completed" || meta.LastVerificationResult != "pass" || meta.CompletionID == "" || meta.LastVerifiedCompletionID != meta.CompletionID || meta.LastVerificationPreviousID != "" {
					t.Fatalf("task %s completion metadata: %+v", id, meta)
				}
				if !lowerHex32.MatchString(meta.LastVerificationID) || seen[meta.LastVerificationID] {
					t.Fatalf("task %s verification identity is invalid or reused: %q", id, meta.LastVerificationID)
				}
				artifact, ok := reports[meta.LastVerificationID]
				if !ok || artifact.TaskID != id || artifact.Result != "pass" || valueOrEmpty(artifact.ObservedCompletionID) != meta.CompletionID || artifact.PreviousVerificationID != nil {
					t.Fatalf("task %s metadata/artifact: %+v %+v", id, meta, artifact)
				}
				seen[meta.LastVerificationID] = true
				committed := gitOutput(t, repo, "show", commits[i]+":planning/tasks/"+id+".md")
				if !strings.Contains(committed, meta.LastVerificationID) || !strings.Contains(committed, meta.CompletionID) {
					t.Fatalf("delivery commit lacks task bindings: %s", committed)
				}
				if i == 1 && (valueOrEmpty(report.LastIteration.VerificationIDAfter) != meta.LastVerificationID || valueOrEmpty(report.LastIteration.ObservedCompletionID) != meta.CompletionID || state.Frontmatter.LastVerificationID != meta.LastVerificationID) {
					t.Fatalf("final observations disagree: %+v", report.LastIteration)
				}
			}
			if checked && strings.Join(strings.Fields(readBytes(t, record)), "\n") != strings.Join(commits, "\n") {
				t.Fatalf("checks did not observe both deliveries in order: %s", readBytes(t, record))
			}
		})
	}
}

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
	if report.Outcome != "child_failed" || report.LastIteration == nil || report.LastIteration.TaskID != "T-001-ready" || report.IterationsCompleted != 1 {
		t.Fatalf("report = %+v, want one failed child", report)
	}
	if report.LastIteration.Outcome != "child_failed" || report.LastIteration.Child.ExitCode == nil || *report.LastIteration.Child.ExitCode == 0 || len(report.MutationViolations) != 0 {
		t.Fatalf("report = %+v, want nonzero child exit without mutation violations", report)
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

func TestLoopExecuteAcceptsUnverifiedLifecycle(t *testing.T) {
	clearLoopChildEnvironment(t)
	t.Setenv("GO_WANT_LOOP_CHILD", "1")
	binary := buildTaskrailExecutable(t)
	previous := loopExecutablePath
	loopExecutablePath = func() (string, error) { return binary, nil }
	t.Cleanup(func() { loopExecutablePath = previous })
	for _, test := range []struct{ action, outcome string }{
		{"start", "no_progress"},
		{"block", "no_progress"},
		{"complete", "completed_unverified"},
	} {
		t.Run(test.action, func(t *testing.T) {
			repo, svc := loopFixture(t)
			writeTask(t, repo, "T-001-ready", "Ready", "todo", "high", "specs/v0.1.0.md#summary", nil)
			if _, err := svc.MutateTaskLoopPolicy(LoopPolicyMutationInput{TaskID: "T-001-ready", Operation: LoopPolicyAllow, Reason: "exercise lifecycle"}); err != nil {
				t.Fatal(err)
			}
			if _, err := svc.Repair(RepairInput{Apply: true}); err != nil {
				t.Fatal(err)
			}
			template := []byte("Implement {{TASK_ID}}.\n" + strings.Repeat("Lifecycle child must consume its prompt.\n", 4096))
			writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-implementation.md"), string(template))
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-m", "allow task")
			head := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))
			child := []string{os.Args[0], "-test.run=^TestLoopLaunchChildHelper$", "--", "delegate-unverified", filepath.Join(t.TempDir(), "record"), test.action}
			report, err := svc.LoopExecute(context.Background(), LoopInvocation{MaxIterations: 1, Child: child, AllowPromptOverrideSHA256: promptDigest(template)})
			if err != nil {
				t.Fatal(err)
			}
			if report.Outcome != test.outcome || report.LastIteration == nil || report.LastIteration.LifecycleCandidate == nil || *report.LastIteration.LifecycleCandidate != test.outcome || len(report.MutationViolations) != 0 {
				t.Fatalf("report = %+v, child = %+v, want %s without integrity violations", report, report.LastIteration, test.outcome)
			}
			if !report.Git.Clean || report.Git.HeadBefore != head || report.Git.HeadAfter == head || len(report.Git.Commits) != 1 {
				t.Fatalf("Git = %+v, want one clean lifecycle commit", report.Git)
			}
		})
	}
}

func TestLoopExecuteDeliversLocalLifecycleWithOneProductCommit(t *testing.T) {
	clearLoopChildEnvironment(t)
	t.Setenv("GO_WANT_LOOP_CHILD", "1")
	binary := buildTaskrailExecutable(t)
	previous := loopExecutablePath
	loopExecutablePath = func() (string, error) { return binary, nil }
	t.Cleanup(func() { loopExecutablePath = previous })
	source := realGitRepo(t)
	repo := filepath.Join(t.TempDir(), "local-loop")
	runGit(t, source, "worktree", "add", "-b", "local-loop", repo)
	requireRecoveryDirectoryDurability(t, repo)
	initialHead := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))
	initial := newTestService(t, repo, time.Now())
	if _, err := initial.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("discover local service: %v", err)
	}
	if _, err := svc.CreateTask(CreateTaskInput{Title: "Deliver local product", SpecRef: "specs/v0.1.0.md#summary"}); err != nil {
		t.Fatalf("create local task: %v", err)
	}
	tasks, err := svc.loadTasks()
	if err != nil || len(tasks) != 1 {
		t.Fatalf("load local task: tasks=%+v err=%v", tasks, err)
	}
	taskID := tasks[0].Frontmatter.ID
	if _, err := svc.MutateTaskLoopPolicy(LoopPolicyMutationInput{TaskID: taskID, Operation: LoopPolicyAllow, Reason: "exercise local delivery"}); err != nil {
		t.Fatalf("allow local task: %v", err)
	}

	child := []string{os.Args[0], "-test.run=^TestLoopLaunchChildHelper$", "--", "delegate-local-lifecycle", filepath.Join(t.TempDir(), "record"), taskID}
	report, err := svc.LoopExecute(context.Background(), LoopInvocation{MaxIterations: 1, Child: child})
	if err != nil {
		t.Fatalf("LoopExecute: %v", err)
	}
	if report.Outcome != "no_work" || report.LastIteration == nil || report.LastIteration.Outcome != "completed_pass" || report.Storage.Mode != string(StorageLocal) {
		t.Fatalf("local loop report = %+v", report)
	}
	if !report.Git.Clean || !report.Git.Descendant || len(report.Git.Commits) != 1 || report.Git.HeadBefore != initialHead || report.Git.HeadAfter == initialHead {
		t.Fatalf("local delivery evidence = %+v", report.Git)
	}
	if paths := strings.Fields(gitOutput(t, repo, "diff-tree", "--no-commit-id", "--name-only", "-r", report.Git.HeadAfter)); len(paths) != 1 || paths[0] != "product.txt" {
		t.Fatalf("local delivery paths = %v, want product.txt only", paths)
	}
	if status := gitOutput(t, repo, "status", "--porcelain=v1", "--untracked-files=all", "--ignore-submodules=none"); status != "" {
		t.Fatalf("local delivery left visible Git changes: %q", status)
	}
	if tracked := gitOutput(t, repo, "ls-files", ".taskrail"); tracked != "" {
		t.Fatalf("local delivery tracked Taskrail metadata: %q", tracked)
	}
	if validation, err := svc.Validate(); err != nil || !validation.Valid {
		t.Fatalf("validate delivered local lifecycle: %+v, %v", validation, err)
	}
	final, err := svc.loadTasks()
	if err != nil || len(final) != 1 || final[0].Frontmatter.Status != "completed" || final[0].Frontmatter.LastVerificationResult != "pass" {
		t.Fatalf("local task lifecycle = %+v, %v", final, err)
	}
}
