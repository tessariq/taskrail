package taskrail

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/tessariq/taskrail/internal/repolock"
)

// LoopDiagnostic is the terminal report for one sequential loop invocation.
// It contains observations only; recovery remains an explicit operator action.
type LoopDiagnostic struct {
	Outcome             string              `json:"outcome"`
	LastIteration       *LoopIteration      `json:"last_iteration"`
	IterationsCompleted int                 `json:"iterations_completed"`
	RemainingTask       *TaskLoopRow        `json:"remaining_task"`
	Validation          LoopValidation      `json:"validation"`
	Git                 LoopGitDiagnostic   `json:"git"`
	Storage             LoopStorageSnapshot `json:"storage"`
	Review              LoopReviewPolicy    `json:"review"`
	Execution           LoopExecutionBudget `json:"execution"`
	Executable          LoopExecutable      `json:"executable"`
	MutationViolations  []MachineViolation  `json:"mutation_violations"`
	ProcessViolations   []MachineViolation  `json:"process_violations"`
	Remote              string              `json:"remote"`
	NextAction          string              `json:"next_action"`
	Parallel            *ParallelBatch      `json:"parallel"`
}

type LoopValidation struct {
	Valid      bool               `json:"valid"`
	Violations []MachineViolation `json:"violations"`
}

type LoopExecutable struct {
	InvocationID string `json:"invocation_id"`
	Path         string `json:"path"`
	SHA256       string `json:"sha256"`
}

type LoopGitDiagnostic struct {
	Ref        string   `json:"ref"`
	HeadBefore string   `json:"head_before"`
	HeadAfter  string   `json:"head_after"`
	Clean      bool     `json:"clean"`
	Descendant bool     `json:"descendant"`
	Commits    []string `json:"commits"`
}

// LoopExecute runs one foreground child at a time. Only a fully validated and
// delivered completed pass can reach another read-only selection.
func (s *Service) LoopExecute(ctx context.Context, invocation LoopInvocation) (LoopDiagnostic, error) {
	return s.loopExecute(ctx, invocation, false)
}

// LoopExecuteWithPreparedResultFile runs a loop after the CLI has retained the
// result destination identity that it will use for terminal publication.
func (s *Service) LoopExecuteWithPreparedResultFile(ctx context.Context, invocation LoopInvocation, result *LoopResultFile) (LoopDiagnostic, error) {
	if result == nil {
		return LoopDiagnostic{}, invalidArgumentsf("loop execution requires a prepared --result-file")
	}
	if err := s.ValidateLoopResultFile(result); err != nil {
		return LoopDiagnostic{}, err
	}
	invocation.ResultFile = result.path
	return s.loopExecute(ctx, invocation, true)
}

func (s *Service) loopExecute(ctx context.Context, invocation LoopInvocation, preparedResultFile bool) (LoopDiagnostic, error) {
	if invocation.DryRun {
		return LoopDiagnostic{}, invalidArgumentsf("loop execution does not accept --dry-run")
	}
	if invocation.Delivery == "review" && !preparedResultFile {
		return LoopDiagnostic{}, invalidArgumentsf("--delivery review requires a prepared --result-file")
	}
	snapshot, err := s.LoopPreflight(invocation)
	if err != nil {
		return LoopDiagnostic{}, err
	}
	invocation = snapshot.Invocation()
	if invocation.Parallel > 1 {
		return s.loopParallelExecute(ctx, invocation, snapshot)
	}
	if err := s.authorizeLoopExecutionPrompts(snapshot, "task-implementation"); err != nil {
		return LoopDiagnostic{}, err
	}
	selection, err := s.loopFrozenSelection(snapshot)
	if err != nil {
		return LoopDiagnostic{}, err
	}
	ownership, err := s.beginLoopOwnership(ctx)
	if err != nil {
		return LoopDiagnostic{}, err
	}
	defer ownership.close()

	diagnostic := loopDiagnosticBase(snapshot, ownership)
	if selection.Action == "none" {
		diagnostic.Outcome = "no_work"
		diagnostic.NextAction = "Inspect task loop policy and explicitly allow eligible work before another loop invocation."
		return diagnostic, nil
	}
	if selection.Action != "run" || selection.SelectedTask == nil {
		return LoopDiagnostic{}, WithMachineErrorCode(MachineCodeValidationFailed, fmt.Errorf("loop selection is invalid: %s", selection.Reason))
	}

	for {
		iteration, completed, mutation, process := s.runLoopIteration(ctx, snapshot, ownership, *selection.SelectedTask)
		diagnostic.LastIteration = &iteration
		diagnostic.IterationsCompleted += completed
		diagnostic.MutationViolations = mutation
		diagnostic.ProcessViolations = process
		diagnostic.Validation = loopValidation(iteration.Outcome == "invalid_postflight", s.loopPostflightValidation())
		diagnostic.Git = s.loopPostflightDelivery(snapshot, iteration, mutation)
		if iteration.Outcome != "completed_pass" {
			diagnostic.Outcome = iteration.Outcome
			diagnostic.NextAction = loopNextAction(iteration.Outcome, iteration.TaskID)
			return diagnostic, nil
		}

		selection, err = s.TaskLoopSelect()
		if err != nil {
			diagnostic.Outcome = "invalid_postflight"
			diagnostic.NextAction = "Inspect repository selection evidence before another loop invocation."
			return diagnostic, nil
		}
		if selection.Action == "none" {
			diagnostic.Outcome = "no_work"
			diagnostic.NextAction = "No eligible allowed task remains; inspect task loop policy before another invocation."
			return diagnostic, nil
		}
		if selection.Action != "run" || selection.SelectedTask == nil {
			diagnostic.Outcome = "invalid_postflight"
			diagnostic.NextAction = "Inspect repository validation and task loop policy before retrying manually."
			return diagnostic, nil
		}
		if diagnostic.IterationsCompleted >= invocation.MaxIterations {
			diagnostic.Outcome = "iteration_limit"
			diagnostic.RemainingTask = selection.SelectedTask
			diagnostic.NextAction = "Review the remaining allowed task and start a new loop invocation with an explicit iteration budget."
			return diagnostic, nil
		}
		snapshot, err = s.nextLoopIterationSnapshot(snapshot)
		if err != nil {
			diagnostic.Outcome = "invalid_postflight"
			diagnostic.NextAction = "Inspect repository postflight evidence before another loop invocation."
			return diagnostic, nil
		}
	}
}

func loopDiagnosticBase(snapshot LoopPreflightSnapshot, ownership *loopOwnership) LoopDiagnostic {
	git := snapshot.Git()
	review := snapshot.Review()
	return LoopDiagnostic{
		Validation:         LoopValidation{Valid: true, Violations: []MachineViolation{}},
		Git:                LoopGitDiagnostic{Ref: git.Ref, HeadBefore: git.Head, HeadAfter: git.Head, Clean: git.Clean, Descendant: true, Commits: []string{}},
		Storage:            snapshot.Storage(),
		Review:             LoopReviewPolicy{ConfiguredMaxRounds: review.ConfiguredMaxRounds, EffectiveMaxRounds: review.EffectiveMaxRounds, MaxReviewersPerRound: review.MaxReviewersPerRound, FinalDiffReviewRequiredOnChange: review.FinalDiffReviewRequiredOnChange, Source: review.Source},
		Execution:          loopExecutionBudget(snapshot.Invocation()),
		Executable:         LoopExecutable{InvocationID: ownership.invocation, Path: ownership.executable.Path, SHA256: ownership.executable.SHA256},
		MutationViolations: []MachineViolation{}, ProcessViolations: []MachineViolation{}, Remote: "not_checked", Parallel: nil,
	}
}

func (s *Service) runLoopIteration(ctx context.Context, snapshot LoopPreflightSnapshot, ownership *loopOwnership, selected TaskLoopRow) (LoopIteration, int, []MachineViolation, []MachineViolation) {
	return s.runLoopIterationTo(ctx, snapshot, ownership, selected, nil, nil)
}

func (s *Service) runLoopIterationTo(ctx context.Context, snapshot LoopPreflightSnapshot, ownership *loopOwnership, selected TaskLoopRow, stdout, stderr io.Writer, sequence ...*repolock.FollowupSequence) (LoopIteration, int, []MachineViolation, []MachineViolation) {
	task, _ := taskByIDFromSlice(s.mustLoadTasks(), selected.TaskID)
	if task == nil {
		return LoopIteration{TaskID: selected.TaskID, Outcome: "invalid_postflight", Policy: selected}, 0, []MachineViolation{{Code: "postflight_evidence_missing", Message: "selected task is unavailable"}}, []MachineViolation{}
	}
	prompt, err := s.loopDryRunPrompt(snapshot, selected)
	if err != nil {
		return LoopIteration{TaskID: selected.TaskID, Outcome: "invalid_postflight", Policy: selected}, 0, []MachineViolation{{Code: "prompt_changed", Message: err.Error()}}, []MachineViolation{}
	}
	identity, err := ownership.delegate(s.loopDelegationGrant(selected.TaskID, sequence...))
	if err != nil {
		return LoopIteration{TaskID: selected.TaskID, Outcome: "invalid_postflight", Policy: selected}, 0, []MachineViolation{{Code: "executable_changed", Message: err.Error()}}, []MachineViolation{}
	}
	execution, launchErr := launchLoopChild(loopChildLaunch{Command: snapshot.Invocation().Child, Context: ctx, Timeout: snapshot.Invocation().Timeout, Prompt: []byte(prompt.Content), RepositoryRoot: s.paths.WorktreeRoot, Identity: identity, Stdout: stdout, Stderr: stderr})
	if launchErr != nil {
		return LoopIteration{TaskID: selected.TaskID, Outcome: "child_failed", Child: loopIterationChild(execution), Policy: selected, Prompt: loopIterationPrompt(prompt)}, 0, []MachineViolation{}, []MachineViolation{{Code: "launch_failed", Message: launchErr.Error()}}
	}
	planningDir, verifyDir := s.loopManagedInputPaths()
	finalValidation := s.loopPostflightValidation()
	finalTask, state, reports, paths := s.loopPostflightLifecycle(selected.TaskID)
	iteration := deriveLoopLifecycle(loopLifecycleEvidence{TaskID: selected.TaskID, StatusBefore: task.Frontmatter.Status,
		CompletionIDBefore: task.Frontmatter.CompletionID, VerificationIDBefore: task.Frontmatter.LastVerificationID,
		VerificationPreviousIDBefore: task.Frontmatter.LastVerificationPreviousID, Task: finalTask, State: state,
		Reports: reports, PreflightVerificationIDs: loopFrozenVerificationIDsSet(snapshot.Inputs(), planningDir, verifyDir),
		VerificationPaths: loopVerificationPathEvidence{Final: paths}, Validation: finalValidation, Child: execution, Policy: selected, Prompt: prompt})
	postInputs, inputErr := loopInputBytes(s.paths)
	postGit, gitErr := loopGitSnapshot(s.paths.WorktreeRoot, s.paths.GitDir)
	rootRefs, refErr := loopRootRefCandidates(s.paths.GitDir, s.paths.GitCommonDir)
	gitConfig, configErr := loopGitConfigSnapshot(s.paths.GitDir, s.paths.GitCommonDir)
	mutation := []MachineViolation{}
	if inputErr != nil || gitErr != nil || refErr != nil || configErr != nil {
		mutation = append(mutation, MachineViolation{Code: "postflight_evidence_missing", Message: fmt.Sprintf("could not collect postflight repository evidence: inputs=%v git=%v root_refs=%v git_config=%v", inputErr, gitErr, refErr, configErr)})
	} else {
		mutation = checkLoopIntegrity(loopIntegrityEvidence{Preflight: snapshot, SelectedTask: selected.TaskID, PlanningDir: planningDir,
			VerifyDir: verifyDir, Inputs: postInputs, Git: postGit, RootRefs: rootRefs,
			GitConfig: gitConfig,
			Storage:   loopStoragePointer(s.paths), Review: loopReviewPointer(snapshot), Prompt: &prompt, ExpectedPrompt: &prompt, Executable: &identity, ExpectedExecutable: &identity})
		_, deliveryViolations := validateLoopDelivery(loopDeliveryEvidence{Root: s.paths.WorktreeRoot, Preflight: snapshot, Postflight: postGit,
			PostflightInputs: postInputs, PlanningDir: planningDir, VerifyDir: verifyDir,
			SelectedTask: selected.TaskID, LifecycleCandidate: valueOrEmpty(iteration.LifecycleCandidate), IntegrityViolations: mutation, ChildFailed: iteration.Outcome == "child_failed"})
		mutation = append(mutation, deliveryViolations...)
	}
	if len(mutation) != 0 {
		iteration.Outcome = "invalid_postflight"
	}
	process := loopProcessViolations(execution)
	if len(process) != 0 && iteration.Outcome == "completed_pass" {
		iteration.Outcome = "child_failed"
	}
	completed := 0
	if execution.PID != 0 {
		completed = 1
	}
	return iteration, completed, mutation, process
}

func (s *Service) loopPostflightValidation() ValidationResult {
	result, err := s.Validate()
	if err != nil {
		return ValidationResult{Valid: false, Violations: []string{err.Error()}}
	}
	return result
}

func (s *Service) loopPostflightLifecycle(taskID string) (*Task, *State, map[string]VerificationArtifact, map[string]string) {
	state, _ := s.loadState()
	tasks, _ := s.loadTasks()
	task, _ := taskByIDFromSlice(tasks, taskID)
	inputs, _ := loopInputBytes(s.paths)
	_, verifyDir := s.loopManagedInputPaths()
	reports := make(map[string]VerificationArtifact)
	paths := make(map[string]string)
	for inputPath, report := range loopVerificationReports(inputs, verifyDir) {
		reports[report.VerificationID] = report
		paths[report.VerificationID] = inputPath
	}
	return task, state, reports, paths
}

func (s *Service) loopPostflightDelivery(snapshot LoopPreflightSnapshot, iteration LoopIteration, integrity []MachineViolation) LoopGitDiagnostic {
	postflight, err := loopGitSnapshot(s.paths.WorktreeRoot, s.paths.GitDir)
	if err != nil {
		return LoopGitDiagnostic{Ref: snapshot.Git().Ref, HeadBefore: snapshot.Git().Head}
	}
	inputs, _ := loopInputBytes(s.paths)
	planningDir, verifyDir := s.loopManagedInputPaths()
	delivery, _ := validateLoopDelivery(loopDeliveryEvidence{Root: s.paths.WorktreeRoot, Preflight: snapshot, Postflight: postflight,
		PostflightInputs: inputs, PlanningDir: planningDir, VerifyDir: verifyDir,
		SelectedTask: iteration.TaskID, LifecycleCandidate: valueOrEmpty(iteration.LifecycleCandidate), IntegrityViolations: integrity, ChildFailed: iteration.Outcome == "child_failed"})
	return loopGitDiagnostic(delivery)
}

func (s *Service) loopManagedInputPaths() (string, string) {
	planning := filepath.ToSlash(relPath(s.paths.RepoRoot, s.paths.PlanningDir))
	verify := filepath.ToSlash(relPath(s.paths.RepoRoot, s.paths.VerifyDir))
	return planning, verify
}

func loopGitDiagnostic(delivery LoopDelivery) LoopGitDiagnostic {
	return LoopGitDiagnostic{Ref: delivery.Ref, HeadBefore: delivery.HeadBefore, HeadAfter: delivery.HeadAfter,
		Clean: delivery.Clean, Descendant: delivery.Descendant, Commits: append([]string{}, delivery.Commits...)}
}

// nextLoopIterationSnapshot moves only the per-child evidence boundary after a
// delivered pass. Invocation policy remains the immutable values captured once.
func (s *Service) nextLoopIterationSnapshot(previous LoopPreflightSnapshot) (LoopPreflightSnapshot, error) {
	inputs, err := loopInputBytes(s.paths)
	if err != nil {
		return LoopPreflightSnapshot{}, err
	}
	git, err := loopGitSnapshot(s.paths.WorktreeRoot, s.paths.GitDir)
	if err != nil {
		return LoopPreflightSnapshot{}, err
	}
	rootRefs, err := loopRootRefCandidates(s.paths.GitDir, s.paths.GitCommonDir)
	if err != nil {
		return LoopPreflightSnapshot{}, err
	}
	gitConfig, err := loopGitConfigSnapshot(s.paths.GitDir, s.paths.GitCommonDir)
	if err != nil {
		return LoopPreflightSnapshot{}, err
	}
	return LoopPreflightSnapshot{invocation: previous.Invocation(), inputs: inputs, git: git,
		storage: previous.Storage(), review: previous.Review(), lock: previous.Lock(), rootRefs: rootRefs, gitConfig: gitConfig}, nil
}

func (s *Service) authorizeLoopExecutionPrompts(snapshot LoopPreflightSnapshot, ids ...string) error {
	authorization := snapshot.Invocation().AllowPromptOverrideSHA256
	replacements, err := s.loopPromptReplacementDigests(snapshot, ids...)
	if err != nil {
		return err
	}
	if len(replacements) == 0 && authorization != "" {
		return invalidArgumentsf("--allow-prompt-override-sha256 requires a replacement prompt")
	}
	if len(replacements) != 0 {
		if len(replacements) != 1 {
			return WithMachineErrorCode(MachineCodePromptInvalid, fmt.Errorf("parallel replacement prompts require one shared authorized template SHA-256"))
		}
		if _, ok := replacements[authorization]; ok {
			return nil
		}
		return WithMachineErrorCode(MachineCodePromptInvalid, fmt.Errorf("replacement prompt authorization does not match its template SHA-256"))
	}
	return nil
}

func (s *Service) loopPromptReplacementDigests(snapshot LoopPreflightSnapshot, ids ...string) (map[string]struct{}, error) {
	replacements := make(map[string]struct{})
	for _, id := range ids {
		template, source, _, err := s.loopPromptTemplate(snapshot, id)
		if err != nil {
			return nil, err
		}
		if source == "replacement" {
			replacements[promptDigest(template)] = struct{}{}
		}
	}
	return replacements, nil
}

func loopExecutionBudget(invocation LoopInvocation) LoopExecutionBudget {
	budget := LoopExecutionBudget{TimeoutSource: "none"}
	if invocation.Timeout != nil {
		value := invocation.Timeout.String()
		budget.Timeout = &value
		budget.TimeoutSource = "flag"
	}
	return budget
}

func loopValidation(invalid bool, result ValidationResult) LoopValidation {
	violations := make([]MachineViolation, 0, len(result.Violations))
	for _, message := range result.Violations {
		violations = append(violations, MachineViolation{Code: MachineCodeRepositoryInvalid, Message: message})
	}
	return LoopValidation{Valid: !invalid && result.Valid, Violations: violations}
}

func loopProcessViolations(execution loopChildExecution) []MachineViolation {
	violations := []MachineViolation{}
	if execution.TimedOut {
		violations = append(violations, MachineViolation{Code: "timeout", Message: "child exceeded its per-child timeout"})
	}
	if execution.Containment.Survivors {
		violations = append(violations, MachineViolation{Code: "survivors", Message: "contained child process has survivors"})
	}
	if execution.ContainmentError != nil {
		violations = append(violations, MachineViolation{Code: "containment_failed", Message: execution.ContainmentError.Error()})
	}
	return violations
}

func loopNextAction(outcome, taskID string) string {
	return fmt.Sprintf("Inspect %s and use existing lifecycle, verify, and Git recovery commands before another loop invocation.", taskID)
}

func taskByIDFromSlice(tasks []*Task, id string) (*Task, bool) {
	for _, task := range tasks {
		if task.Frontmatter.ID == id {
			return task, true
		}
	}
	return nil, false
}

func (s *Service) mustLoadTasks() []*Task {
	tasks, _ := s.loadTasks()
	return tasks
}

func loopFrozenVerificationIDsSet(inputs map[string][]byte, planningDir, verifyDir string) map[string]struct{} {
	ids := loopFrozenVerificationIDs(inputs, planningDir, verifyDir)
	result := make(map[string]struct{}, len(ids))
	for id := range ids {
		result[id] = struct{}{}
	}
	return result
}

func loopStoragePointer(paths Paths) *LoopStorageSnapshot {
	return &LoopStorageSnapshot{Mode: string(paths.Storage.Mode), Root: paths.Storage.Root}
}

func loopReviewPointer(snapshot LoopPreflightSnapshot) *LoopReviewSnapshot {
	review := snapshot.Review()
	return &review
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
