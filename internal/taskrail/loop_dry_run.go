package taskrail

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
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
	Parallel     *ParallelPlan        `json:"parallel"`
}

type ParallelWorkspace struct {
	Root       string `json:"root"`
	CloneDepth *int   `json:"clone_depth"`
	Retention  string `json:"retention"`
}

type ParallelTaskPlan struct {
	Rank int         `json:"rank"`
	Task TaskLoopRow `json:"task"`
}

type ParallelPlan struct {
	RequestedWidth int                `json:"requested_width"`
	EffectiveWidth int                `json:"effective_width"`
	BaseRef        string             `json:"base_ref"`
	BaseHead       string             `json:"base_head"`
	Workspace      ParallelWorkspace  `json:"workspace"`
	Delivery       string             `json:"delivery"`
	ReviewAdapter  *string            `json:"review_adapter"`
	Frontier       []ParallelTaskPlan `json:"frontier"`
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
	if invocation.Parallel > 1 {
		plan, err := s.loopParallelPlan(snapshot, selection.Tasks)
		if err != nil {
			return LoopDryRunResult{}, err
		}
		report.Parallel = &plan
		report.SelectedTask = nil
		if len(plan.Frontier) > 0 {
			report.Action = "run"
			report.Reason = "selected allowed parallel frontier by priority and stable task id"
		} else {
			report.Action = "none"
			report.Reason = "no eligible allowed task"
		}
	}
	_, source, _, err := s.loopDryRunTemplate(snapshot)
	if err != nil {
		return LoopDryRunResult{}, err
	}
	ids := []string{"task-implementation"}
	if invocation.Parallel > 1 {
		ids = append(ids, "loop-integration")
	}
	replacements, err := s.loopPromptReplacementDigests(snapshot, ids...)
	if err != nil {
		return LoopDryRunResult{}, err
	}
	if len(replacements) == 0 && invocation.AllowPromptOverrideSHA256 != "" {
		return LoopDryRunResult{}, invalidArgumentsf("--allow-prompt-override-sha256 requires a replacement prompt")
	}
	if len(replacements) != 0 {
		if invocation.AllowPromptOverrideSHA256 == "" {
			report.Action = "invalid"
			report.Reason = "replacement prompt requires --allow-prompt-override-sha256"
			return report, nil
		}
		if len(replacements) != 1 {
			report.Action = "invalid"
			report.Reason = "parallel replacement prompts require one shared authorized template SHA-256"
			return report, nil
		}
		if _, ok := replacements[invocation.AllowPromptOverrideSHA256]; !ok {
			report.Action = "invalid"
			report.Reason = "replacement prompt authorization does not match its template SHA-256"
			return report, nil
		}
	}
	if report.Action != "run" || invocation.Parallel > 1 {
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

func (s *Service) loopParallelPlan(snapshot LoopPreflightSnapshot, rows []TaskLoopRow) (ParallelPlan, error) {
	invocation := snapshot.Invocation()
	git := snapshot.Git()
	storage := snapshot.Storage()
	if storage.Mode != string(StorageCommitted) {
		return ParallelPlan{}, WithMachineErrorCode(MachineCodeUnsupported, fmt.Errorf("parallel loop requires committed storage"))
	}
	if s.paths.GitDir != s.paths.GitCommonDir {
		return ParallelPlan{}, WithMachineErrorCode(MachineCodeUnsupported, fmt.Errorf("parallel loop does not support linked worktrees"))
	}
	if err := parallelSubmoduleCheck(s.paths.WorktreeRoot); err != nil {
		return ParallelPlan{}, err
	}
	refHead, err := gitCommand(s.paths.WorktreeRoot, "rev-parse", "--verify", git.Ref)
	if err != nil || strings.TrimSpace(refHead) != git.Head {
		return ParallelPlan{}, WithMachineErrorCode(MachineCodeGitState, fmt.Errorf("parallel loop requires branch tip to equal HEAD"))
	}
	workspace, err := parallelWorkspace(invocation, s.paths)
	if err != nil {
		return ParallelPlan{}, err
	}
	priorities, err := s.frozenTaskPriorities(snapshot)
	if err != nil {
		return ParallelPlan{}, err
	}
	candidates := make([]TaskLoopRow, 0)
	for _, row := range rows {
		if row.Eligible && row.Source == "explicit" && row.EffectivePolicy == "allow" {
			candidates = append(candidates, row)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if priorities[candidates[i].TaskID] != priorities[candidates[j].TaskID] {
			return priorities[candidates[i].TaskID] < priorities[candidates[j].TaskID]
		}
		return candidates[i].TaskID < candidates[j].TaskID
	})
	width := min(invocation.Parallel, invocation.MaxIterations)
	if len(candidates) > width {
		candidates = candidates[:width]
	}
	frontier := make([]ParallelTaskPlan, len(candidates))
	for i, task := range candidates {
		frontier[i] = ParallelTaskPlan{Rank: i + 1, Task: task}
	}
	var adapter *string
	if invocation.Delivery == "review" {
		resolved, err := resolveReviewAdapter(invocation.ReviewAdapter)
		if err != nil {
			return ParallelPlan{}, err
		}
		adapter = &resolved
	}
	return ParallelPlan{RequestedWidth: invocation.Parallel, EffectiveWidth: width, BaseRef: git.Ref, BaseHead: git.Head,
		Workspace: workspace, Delivery: invocation.Delivery, ReviewAdapter: adapter, Frontier: frontier}, nil
}

func resolveReviewAdapter(adapter string) (string, error) {
	if adapter == "" {
		return "", invalidArgumentsf("--delivery review requires exactly one --review-adapter")
	}
	resolved, err := exec.LookPath(adapter)
	if err != nil {
		return "", invalidArgumentsf("resolve --review-adapter: %v", err)
	}
	absolute, err := filepath.Abs(resolved)
	if err != nil {
		return "", invalidArgumentsf("resolve --review-adapter: %v", err)
	}
	info, err := os.Stat(absolute)
	if err != nil || !executableReviewAdapter(info.Mode(), runtime.GOOS) {
		return "", invalidArgumentsf("--review-adapter must resolve to an executable regular file")
	}
	return absolute, nil
}

func executableReviewAdapter(mode os.FileMode, goos string) bool {
	return mode.IsRegular() && (goos == "windows" || mode.Perm()&0o111 != 0)
}

func (s *Service) frozenTaskPriorities(snapshot LoopPreflightSnapshot) (map[string]int, error) {
	priorities := make(map[string]int)
	taskRoot := filepath.ToSlash(relPath(s.paths.RepoRoot, s.paths.TasksDir)) + "/"
	for inputPath, data := range snapshot.Inputs() {
		filename, found := strings.CutPrefix(inputPath, taskRoot)
		if !found || path.Dir(filename) != "." || path.Ext(filename) != ".md" {
			continue
		}
		frontmatter, _, err := parseFrontmatter[TaskFrontmatter](data)
		if err != nil {
			return nil, WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("read frozen loop task %s: %w", inputPath, err))
		}
		priorities[frontmatter.ID] = priorityRank[frontmatter.Priority]
	}
	return priorities, nil
}

func parallelWorkspace(invocation LoopInvocation, paths Paths) (ParallelWorkspace, error) {
	root := os.TempDir()
	if invocation.WorkspaceRootSet {
		root = invocation.WorkspaceRoot
		if !filepath.IsAbs(root) {
			absolute, err := filepath.Abs(root)
			if err != nil {
				return ParallelWorkspace{}, invalidArgumentsf("resolve --workspace-root: %v", err)
			}
			root = absolute
		}
		if err := validateParallelWorkspaceRoot(root, paths); err != nil {
			return ParallelWorkspace{}, err
		}
	}
	depth := (*int)(nil)
	if invocation.CloneDepth != "full" {
		n, _ := positiveLoopInt("--clone-depth", invocation.CloneDepth)
		depth = &n
	}
	return ParallelWorkspace{Root: filepath.Clean(root), CloneDepth: depth, Retention: invocation.KeepWorkspaces}, nil
}

func validateParallelWorkspaceRoot(root string, paths Paths) error {
	if err := validateTargetPath(root); err != nil {
		return invalidArgumentsf("validate --workspace-root: %v", err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return invalidArgumentsf("--workspace-root must be an existing directory")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return invalidArgumentsf("--workspace-root must not grant group or other access")
	}
	if err := validateParallelWorkspaceOwnership(info); err != nil {
		return invalidArgumentsf("%v", err)
	}
	for current := filepath.Clean(root); ; current = filepath.Dir(current) {
		entry, err := os.Lstat(current)
		macOSVarAlias := runtime.GOOS == "darwin" && current == "/var" && err == nil && entry.Mode()&os.ModeSymlink != 0
		if err != nil || (entry.Mode()&os.ModeSymlink != 0 && !macOSVarAlias) {
			return invalidArgumentsf("--workspace-root contains a symbolic link")
		}
		if current == filepath.Dir(current) {
			break
		}
	}
	for _, blocked := range []string{paths.WorktreeRoot, paths.GitDir, paths.GitCommonDir, paths.Storage.Root, paths.SpecsDir, paths.PlanningDir, paths.PromptsDir} {
		inside, err := filepath.Rel(blocked, root)
		if err == nil && inside != ".." && !strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
			return invalidArgumentsf("--workspace-root must be outside managed repository inputs")
		}
	}
	return nil
}

func parallelSubmoduleCheck(root string) error {
	status, err := gitCommand(root, "submodule", "status", "--recursive")
	if err != nil {
		return WithMachineErrorCode(MachineCodeUnsupported, fmt.Errorf("inspect recursive submodules: %w", err))
	}
	for _, line := range strings.Split(status, "\n") {
		if strings.HasPrefix(line, "-") {
			return WithMachineErrorCode(MachineCodeUnsupported, fmt.Errorf("parallel loop requires initialized recursive submodules"))
		}
	}
	return nil
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
	return s.loopPromptTemplate(snapshot, "task-implementation")
}

// loopPromptTemplate reads only the preflight input snapshot so a parallel child
// cannot combine a later replacement with the invocation it was authorized for.
func (s *Service) loopPromptTemplate(snapshot LoopPreflightSnapshot, id string) ([]byte, string, *string, error) {
	definition, err := promptDefinitionFor(id, "v1")
	if err != nil {
		return nil, "", nil, err
	}
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
	templatePath := path.Join(promptRoot, definition.contract, definition.id+".md")
	template, replacement := inputs[templatePath]
	source := "replacement"
	var replacementPath *string
	if !replacement {
		template, err = builtinPrompts.ReadFile(definition.asset)
		if err != nil {
			return nil, "", nil, fmt.Errorf("read embedded %s prompt: %w", definition.id, err)
		}
		source = "builtin"
	} else {
		replacementPath = stringPtr(path.Join(s.paths.LogicalPromptsDir, definition.contract, definition.id+".md"))
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
