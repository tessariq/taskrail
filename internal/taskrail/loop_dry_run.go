package taskrail

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// LoopDryRunResult is the complete read-only loop plan. It contains no launch
// inputs beyond the frozen prompt content selected for the one eligible task.
type LoopDryRunResult struct {
	Action       string               `json:"action"`
	Reason       string               `json:"reason"`
	SelectedTask *TaskLoopRow         `json:"selected_task"`
	Tasks        []TaskLoopRow        `json:"tasks"`
	Violations   []MachineViolation   `json:"violations"`
	Prompt       *LoopPromptExecution `json:"prompt"`
	Git          *LoopDryRunGit       `json:"git"`
	Lock         *LoopDryRunLock      `json:"lock"`
	Storage      *LoopDryRunStorage   `json:"storage"`
	Review       *LoopReviewPolicy    `json:"review"`
	Execution    LoopExecutionBudget  `json:"execution"`
	Delivery     *LoopDryRunDelivery  `json:"delivery"`
	Parallel     any                  `json:"parallel"`
}

type LoopPromptExecution struct {
	ID                 string  `json:"id"`
	Source             string  `json:"source"`
	Path               *string `json:"path"`
	TemplateSHA256     string  `json:"template_sha256"`
	RenderedSHA256     string  `json:"rendered_sha256"`
	OverrideAuthorized bool    `json:"override_authorized"`
	Content            string  `json:"content"`
}

type LoopDryRunGit struct {
	Head     string  `json:"head"`
	Branch   *string `json:"branch"`
	Clean    bool    `json:"clean"`
	Detached bool    `json:"detached"`
}

type LoopDryRunLock struct {
	Available bool    `json:"available"`
	Owner     *string `json:"owner"`
}

type LoopDryRunStorage struct {
	Mode string `json:"mode"`
	Root string `json:"root"`
}

type LoopReviewPolicy struct {
	ConfiguredMaxRounds             int    `json:"configured_max_rounds"`
	EffectiveMaxRounds              int    `json:"effective_max_rounds"`
	MaxReviewersPerRound            int    `json:"max_reviewers_per_round"`
	FinalDiffReviewRequiredOnChange bool   `json:"final_diff_review_required_on_change"`
	Source                          string `json:"source"`
}

type LoopExecutionBudget struct {
	Timeout       *string `json:"timeout"`
	TimeoutSource string  `json:"timeout_source"`
}

type LoopDryRunDelivery struct {
	TaskrailMetadataCommitRequired bool   `json:"taskrail_metadata_commit_required"`
	ProductCommitRequired          bool   `json:"product_commit_required"`
	Remote                         string `json:"remote"`
}

// LoopDryRun validates and freezes repository inputs, then publishes the one
// task and prompt a later execution would be eligible to use. It never launches
// a process or takes the mutation lock.
func (s *Service) LoopDryRun(invocation LoopInvocation) (LoopDryRunResult, error) {
	if !invocation.DryRun {
		return LoopDryRunResult{}, invalidArgumentsf("loop dry-run requires --dry-run")
	}
	snapshot, err := s.LoopPreflight(invocation)
	if err != nil {
		return LoopDryRunResult{}, err
	}
	selection, err := s.loopFrozenSelection(snapshot)
	if err != nil {
		return LoopDryRunResult{}, err
	}
	report := loopDryRunReport(snapshot, selection)
	template, source, _, err := s.loopDryRunTemplate(snapshot)
	if err != nil {
		return LoopDryRunResult{}, err
	}
	if source == "builtin" && invocation.AllowPromptOverrideSHA256 != "" {
		return LoopDryRunResult{}, invalidArgumentsf("--allow-prompt-override-sha256 requires a replacement prompt")
	}
	if source == "replacement" {
		if invocation.AllowPromptOverrideSHA256 == "" {
			report.Action = "invalid"
			report.Reason = "replacement prompt requires --allow-prompt-override-sha256"
			return report, nil
		}
		if invocation.AllowPromptOverrideSHA256 != promptDigest(template) {
			report.Action = "invalid"
			report.Reason = "replacement prompt authorization does not match its template SHA-256"
			return report, nil
		}
	}
	if selection.Action != "run" {
		return report, nil
	}
	prompt, err := s.loopDryRunPrompt(snapshot, *selection.SelectedTask)
	if err != nil {
		return LoopDryRunResult{}, err
	}
	if source == "replacement" {
		prompt.OverrideAuthorized = true
	}
	report.Prompt = &prompt
	return report, nil
}

func loopDryRunReport(snapshot LoopPreflightSnapshot, selection TaskLoopSelectionResult) LoopDryRunResult {
	git := snapshot.Git()
	review := snapshot.Review()
	storage := snapshot.Storage()
	invocation := snapshot.Invocation()
	var branch *string
	if git.Branch != "" {
		branch = stringPtr(git.Branch)
	}
	var timeout *string
	timeoutSource := "none"
	if invocation.Timeout != nil {
		value := invocation.Timeout.String()
		timeout = &value
		timeoutSource = "flag"
	}
	return LoopDryRunResult{
		Action: selection.Action, Reason: selection.Reason, SelectedTask: selection.SelectedTask,
		Tasks: append([]TaskLoopRow{}, selection.Tasks...), Violations: append([]MachineViolation{}, selection.Violations...),
		Git:  &LoopDryRunGit{Head: git.Head, Branch: branch, Clean: git.Clean, Detached: git.Detached},
		Lock: &LoopDryRunLock{Available: true}, Storage: &LoopDryRunStorage{Mode: storage.Mode, Root: storage.Root},
		Review: &LoopReviewPolicy{ConfiguredMaxRounds: review.ConfiguredMaxRounds, EffectiveMaxRounds: review.EffectiveMaxRounds,
			MaxReviewersPerRound: review.MaxReviewersPerRound, FinalDiffReviewRequiredOnChange: review.FinalDiffReviewRequiredOnChange, Source: review.Source},
		Execution: LoopExecutionBudget{Timeout: timeout, TimeoutSource: timeoutSource},
		Delivery:  &LoopDryRunDelivery{TaskrailMetadataCommitRequired: true, ProductCommitRequired: true, Remote: "not_checked"},
	}
}

func (s *Service) loopDryRunPrompt(snapshot LoopPreflightSnapshot, task TaskLoopRow) (LoopPromptExecution, error) {
	template, source, replacementPath, err := s.loopDryRunTemplate(snapshot)
	if err != nil {
		return LoopPromptExecution{}, err
	}
	statePath := filepath.ToSlash(relPath(s.paths.RepoRoot, s.paths.StateFile))
	state, _, err := parseFrontmatter[StateFrontmatter](snapshot.Inputs()[statePath])
	if err != nil {
		return LoopPromptExecution{}, WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("read frozen loop state: %w", err))
	}
	review := snapshot.Review()
	storage := snapshot.Storage()
	rendered, err := RenderPrompt(PromptRenderInput{Template: template, DeclaredTokens: promptTokenDeclarations["task-implementation"], Values: map[string]string{
		"TASK_ID": task.TaskID, "TASK_PATH": path.Join(s.paths.LogicalPlanningDir, "tasks", task.TaskID+".md"),
		"ACTIVE_SPEC_VERSION": state.ActiveSpecVersion, "ACTIVE_SPEC_PATH": state.ActiveSpecPath,
		"IMPLEMENTATION_REVIEW_MAX_ROUNDS": fmt.Sprintf("%d", review.EffectiveMaxRounds), "STORAGE_MODE": storage.Mode,
	}})
	if err != nil {
		return LoopPromptExecution{}, WithMachineErrorCode(MachineCodePromptInvalid, err)
	}
	return LoopPromptExecution{ID: "task-implementation", Source: source, Path: replacementPath,
		TemplateSHA256: rendered.TemplateSHA256, RenderedSHA256: rendered.SHA256,
		OverrideAuthorized: source == "builtin", Content: rendered.Content}, nil
}

func (s *Service) loopDryRunTemplate(snapshot LoopPreflightSnapshot) ([]byte, string, *string, error) {
	inputs := snapshot.Inputs()
	promptRoot := filepath.ToSlash(relPath(s.paths.RepoRoot, s.paths.PromptsDir))
	for inputPath, data := range inputs {
		rel, found := strings.CutPrefix(inputPath, promptRoot+"/")
		if !found {
			continue
		}
		if path.Dir(rel) != "v1" {
			return nil, "", nil, WithMachineErrorCode(MachineCodePromptInvalid, fmt.Errorf("unknown prompt contract entry %q", inputPath))
		}
		if _, ok := promptDefinitionForFilename(path.Base(rel), "v1"); !ok {
			return nil, "", nil, WithMachineErrorCode(MachineCodePromptInvalid, fmt.Errorf("unknown prompt replacement %q", inputPath))
		}
		if err := validatePromptReplacement(data); err != nil {
			return nil, "", nil, WithMachineErrorCode(MachineCodePromptInvalid, fmt.Errorf("invalid prompt replacement %s: %w", inputPath, err))
		}
	}
	templatePath := path.Join(promptRoot, "v1", "task-implementation.md")
	template, replacement := inputs[templatePath]
	source := "replacement"
	var replacementPath *string
	if !replacement {
		var err error
		template, err = builtinPrompts.ReadFile("prompts/v1/task-implementation.md")
		if err != nil {
			return nil, "", nil, fmt.Errorf("read embedded task implementation prompt: %w", err)
		}
		source = "builtin"
	} else {
		replacementPath = stringPtr(path.Join(s.paths.LogicalPromptsDir, "v1", "task-implementation.md"))
	}
	return template, source, replacementPath, nil
}

func (s *Service) loopFrozenSelection(snapshot LoopPreflightSnapshot) (TaskLoopSelectionResult, error) {
	inputs := snapshot.Inputs()
	statePath := filepath.ToSlash(relPath(s.paths.RepoRoot, s.paths.StateFile))
	frontmatter, _, err := parseFrontmatter[StateFrontmatter](inputs[statePath])
	if err != nil {
		return TaskLoopSelectionResult{}, WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("read frozen loop state: %w", err))
	}
	taskRoot := filepath.ToSlash(relPath(s.paths.RepoRoot, s.paths.TasksDir)) + "/"
	tasks := make([]*Task, 0)
	for inputPath, data := range inputs {
		filename, found := strings.CutPrefix(inputPath, taskRoot)
		if !found || path.Dir(filename) != "." || path.Ext(filename) != ".md" {
			continue
		}
		frontmatter, body, err := parseFrontmatter[TaskFrontmatter](data)
		if err != nil {
			return TaskLoopSelectionResult{}, WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("read frozen loop task %s: %w", inputPath, err))
		}
		tasks = append(tasks, &Task{Frontmatter: frontmatter, Body: body, Path: path.Join(s.paths.LogicalPlanningDir, "tasks", filename)})
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Frontmatter.ID < tasks[j].Frontmatter.ID })
	state := &State{Frontmatter: frontmatter}
	result := TaskLoopSelectionResult{Tasks: make([]TaskLoopRow, 0, len(tasks)), Violations: []MachineViolation{}}
	for _, task := range tasks {
		policy := loopRowPolicy(task.Frontmatter.LoopPolicyMetadata)
		held := heldDependencyClosure(task, tasks)
		disposition := loopDisposition(task, state, tasks, policy, held, false)
		result.Tasks = append(result.Tasks, TaskLoopRow{TaskID: task.Frontmatter.ID, Status: task.Frontmatter.Status,
			ActiveSpec: taskMatchesActiveSpec(task, state.Frontmatter.ActiveSpecPath), Source: policy.Source,
			EffectivePolicy: policy.Policy, Reason: policy.Reason, Eligible: disposition == "eligible",
			HeldDependencies: held, Disposition: disposition})
	}
	candidates := make([]int, 0)
	priorities := make(map[string]int, len(tasks))
	for _, task := range tasks {
		priorities[task.Frontmatter.ID] = priorityRank[task.Frontmatter.Priority]
	}
	for i, row := range result.Tasks {
		if row.Eligible && row.Source == "explicit" && row.EffectivePolicy == "allow" {
			candidates = append(candidates, i)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		left, right := result.Tasks[candidates[i]], result.Tasks[candidates[j]]
		if priorities[left.TaskID] != priorities[right.TaskID] {
			return priorities[left.TaskID] < priorities[right.TaskID]
		}
		return left.TaskID < right.TaskID
	})
	if len(candidates) == 0 {
		result.Action = "none"
		result.Reason = "no eligible allowed task"
		return result, nil
	}
	selected := result.Tasks[candidates[0]]
	result.Action = "run"
	result.Reason = "selected allowed task by priority and stable task id"
	result.SelectedTask = &selected
	return result, nil
}
