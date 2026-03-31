package taskrail

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	paths Paths
	now   func() time.Time
}

func NewService(start string) (*Service, error) {
	paths, err := DiscoverPaths(start)
	if err != nil {
		return nil, err
	}
	return &Service{paths: paths, now: time.Now}, nil
}

func (s *Service) Init() error {
	if err := ensureDir(s.paths.SpecsDir); err != nil {
		return err
	}
	if err := ensureDir(s.paths.TasksDir); err != nil {
		return err
	}
	if err := ensureDir(s.paths.VerifyDir); err != nil {
		return err
	}
	if err := ensureDir(filepath.Join(s.paths.ArtifactsDir, "runs")); err != nil {
		return err
	}

	if err := writeFileIfMissing(filepath.Join(s.paths.SpecsDir, "README.md"), []byte(starterSpecsReadme())); err != nil {
		return err
	}
	if err := writeFileIfMissing(filepath.Join(s.paths.SpecsDir, "v0.1.0.md"), []byte(starterSpecV010())); err != nil {
		return err
	}
	if _, err := os.Stat(s.paths.StateFile); errors.Is(err, os.ErrNotExist) {
		state := starterState(s.now())
		if err := s.saveState(state); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("stat state file: %w", err)
	}

	return nil
}

func (s *Service) Validate() (ValidationResult, error) {
	violations := make([]string, 0)
	for _, requiredDir := range []string{s.paths.SpecsDir, s.paths.PlanningDir, s.paths.TasksDir, s.paths.ArtifactsDir, s.paths.VerifyDir} {
		if !dirExists(requiredDir) {
			violations = append(violations, fmt.Sprintf("missing required directory %s", relPath(s.paths.RepoRoot, requiredDir)))
		}
	}
	for _, requiredFile := range []string{filepath.Join(s.paths.SpecsDir, "README.md"), s.paths.StateFile} {
		if !fileExists(requiredFile) {
			violations = append(violations, fmt.Sprintf("missing required file %s", relPath(s.paths.RepoRoot, requiredFile)))
		}
	}

	state, stateErr := s.loadState()
	if stateErr != nil {
		violations = append(violations, stateErr.Error())
	}
	tasks, taskErr := s.loadTasks()
	if taskErr != nil {
		violations = append(violations, taskErr.Error())
	}
	if stateErr != nil || taskErr != nil {
		return ValidationResult{Valid: len(violations) == 0, Violations: violations}, nil
	}

	violations = append(violations, s.validateState(state)...)
	violations = append(violations, s.validateTasks(state, tasks)...)

	return ValidationResult{Valid: len(violations) == 0, Violations: violations}, nil
}

func (s *Service) Next() (NextResult, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return NextResult{}, err
	}

	if state.Frontmatter.CurrentTask != "" {
		if task, ok := taskByID(tasks, state.Frontmatter.CurrentTask); ok && task.Frontmatter.Status == "in_progress" {
			state.Frontmatter.UpdatedAt = timestamp(s.now())
			state.Frontmatter.NextAction = fmt.Sprintf("Continue task %s", task.Frontmatter.ID)
			state.Body = renderStateBody(state.Frontmatter, tasks)
			if err := s.saveState(state); err != nil {
				return NextResult{}, err
			}
			return NextResult{
				TaskID:     task.Frontmatter.ID,
				Title:      task.Frontmatter.Title,
				Priority:   task.Frontmatter.Priority,
				Reason:     "active task already in progress",
				Candidates: []string{task.Frontmatter.ID},
			}, nil
		}
	}

	candidates := eligibleTasks(tasks)
	ids := make([]string, 0, len(candidates))
	for _, task := range candidates {
		ids = append(ids, task.Frontmatter.ID)
	}

	state.Frontmatter.UpdatedAt = timestamp(s.now())
	if len(candidates) == 0 {
		state.Frontmatter.NextAction = "No eligible task is ready"
		state.Body = renderStateBody(state.Frontmatter, tasks)
		if err := s.saveState(state); err != nil {
			return NextResult{}, err
		}
		return NextResult{Reason: "no eligible task", Candidates: ids}, nil
	}

	selected := candidates[0]
	state.Frontmatter.NextAction = fmt.Sprintf("Start task %s: %s", selected.Frontmatter.ID, selected.Frontmatter.Title)
	state.Body = renderStateBody(state.Frontmatter, tasks)
	if err := s.saveState(state); err != nil {
		return NextResult{}, err
	}

	return NextResult{
		TaskID:     selected.Frontmatter.ID,
		Title:      selected.Frontmatter.Title,
		Priority:   selected.Frontmatter.Priority,
		Reason:     "next eligible todo by priority and stable task id",
		Candidates: ids,
	}, nil
}

func (s *Service) Start(taskID string) (TransitionResult, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return TransitionResult{}, err
	}
	if state.Frontmatter.CurrentTask != "" {
		return TransitionResult{}, fmt.Errorf("task %s is already active", state.Frontmatter.CurrentTask)
	}

	task, ok := taskByID(tasks, taskID)
	if !ok {
		return TransitionResult{}, fmt.Errorf("task %s not found", taskID)
	}
	if task.Frontmatter.Status != "todo" {
		return TransitionResult{}, fmt.Errorf("task %s is not todo", taskID)
	}
	if !dependenciesResolved(task, tasks) {
		return TransitionResult{}, fmt.Errorf("task %s has unresolved dependencies", taskID)
	}

	now := timestamp(s.now())
	task.Frontmatter.Status = "in_progress"
	task.Frontmatter.UpdatedAt = now

	state.Frontmatter.UpdatedAt = now
	state.Frontmatter.CurrentTask = task.Frontmatter.ID
	state.Frontmatter.CurrentTaskTitle = task.Frontmatter.Title
	state.Frontmatter.StatusSummary = "in_progress"
	state.Frontmatter.Blockers = []string{}
	state.Frontmatter.NextAction = fmt.Sprintf("Implement %s and run targeted tests", task.Frontmatter.ID)
	state.Body = renderStateBody(state.Frontmatter, tasks)

	if err := s.saveAll(state, tasks); err != nil {
		return TransitionResult{}, err
	}

	return TransitionResult{TaskID: taskID, Status: task.Frontmatter.Status, UpdatedAt: now}, nil
}

func (s *Service) Complete(taskID, note string) (TransitionResult, error) {
	return s.finishTask(taskID, "completed", strings.TrimSpace(note))
}

func (s *Service) Block(taskID, reason string) (TransitionResult, error) {
	if strings.TrimSpace(reason) == "" {
		return TransitionResult{}, errors.New("block reason must not be empty")
	}
	return s.finishTask(taskID, "blocked", strings.TrimSpace(reason))
}

func (s *Service) Verify(input VerifyInput) (VerifyResult, error) {
	if input.Result != "pass" && input.Result != "fail" {
		return VerifyResult{}, fmt.Errorf("invalid verify result %q", input.Result)
	}
	if strings.TrimSpace(input.Summary) == "" {
		return VerifyResult{}, errors.New("verify summary must not be empty")
	}

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return VerifyResult{}, err
	}
	task, ok := taskByID(tasks, input.TaskID)
	if !ok {
		return VerifyResult{}, fmt.Errorf("task %s not found", input.TaskID)
	}

	now := s.now().UTC()
	ts := now.Format("20060102T150405Z")
	artifactDir := filepath.Join(s.paths.VerifyDir, task.Frontmatter.ID, ts)
	if err := ensureDir(artifactDir); err != nil {
		return VerifyResult{}, err
	}

	planPath := filepath.Join(artifactDir, "plan.md")
	reportPath := filepath.Join(artifactDir, "report.json")
	reportMarkdownPath := filepath.Join(artifactDir, "report.md")

	followupTaskID := ""
	if input.CreateFollowup {
		newTask, err := s.createFollowupTask(tasks, task, input)
		if err != nil {
			return VerifyResult{}, err
		}
		tasks = append(tasks, newTask)
		followupTaskID = newTask.Frontmatter.ID
	}

	plan := renderVerificationPlan(task, input, followupTaskID)
	if err := os.WriteFile(planPath, []byte(plan), 0o644); err != nil {
		return VerifyResult{}, fmt.Errorf("write verification plan: %w", err)
	}

	relPlan := relPath(s.paths.RepoRoot, planPath)
	relReport := relPath(s.paths.RepoRoot, reportPath)
	relReportMarkdown := relPath(s.paths.RepoRoot, reportMarkdownPath)

	report := VerificationArtifact{
		SchemaVersion:  stateSchemaVersion,
		TaskID:         task.Frontmatter.ID,
		TaskTitle:      task.Frontmatter.Title,
		Result:         input.Result,
		Summary:        strings.TrimSpace(input.Summary),
		Details:        strings.TrimSpace(input.Details),
		GeneratedAt:    timestamp(now),
		SpecRef:        task.Frontmatter.SpecRef,
		Artifacts:      []string{relPlan, relReportMarkdown},
		FollowupTaskID: followupTaskID,
	}

	reportBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return VerifyResult{}, fmt.Errorf("marshal verification report: %w", err)
	}
	if err := os.WriteFile(reportPath, reportBytes, 0o644); err != nil {
		return VerifyResult{}, fmt.Errorf("write verification report: %w", err)
	}

	reportMarkdown := renderVerificationReportMarkdown(report)
	if err := os.WriteFile(reportMarkdownPath, []byte(reportMarkdown), 0o644); err != nil {
		return VerifyResult{}, fmt.Errorf("write verification markdown report: %w", err)
	}

	nowText := timestamp(now)
	appendTaskNote(task, fmt.Sprintf("- %s: verification %s (%s)", nowText, input.Result, relReport))
	task.Frontmatter.UpdatedAt = nowText

	state.Frontmatter.UpdatedAt = nowText
	state.Frontmatter.LastVerificationResult = fmt.Sprintf("%s for %s (%s)", input.Result, task.Frontmatter.ID, relReport)
	state.Frontmatter.RelevantArtifacts = []string{relPlan, relReport, relReportMarkdown}
	if input.Result == "fail" && followupTaskID != "" {
		state.Frontmatter.NextAction = fmt.Sprintf("Review follow-up task %s", followupTaskID)
	} else if input.Result == "fail" {
		state.Frontmatter.NextAction = fmt.Sprintf("Resolve verification findings for %s", task.Frontmatter.ID)
	} else {
		state.Frontmatter.NextAction = "Select the next eligible task"
	}
	state.Body = renderStateBody(state.Frontmatter, tasks)

	if err := s.saveAll(state, tasks); err != nil {
		return VerifyResult{}, err
	}

	return VerifyResult{
		TaskID:         task.Frontmatter.ID,
		Result:         input.Result,
		ArtifactDir:    relPath(s.paths.RepoRoot, artifactDir),
		PlanPath:       relPlan,
		ReportPath:     relReport,
		ReportMarkdown: relReportMarkdown,
		FollowupTaskID: followupTaskID,
	}, nil
}

func (s *Service) finishTask(taskID, status, note string) (TransitionResult, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return TransitionResult{}, err
	}
	task, ok := taskByID(tasks, taskID)
	if !ok {
		return TransitionResult{}, fmt.Errorf("task %s not found", taskID)
	}
	if task.Frontmatter.Status != "in_progress" && !(status == "blocked" && task.Frontmatter.Status == "todo") {
		return TransitionResult{}, fmt.Errorf("task %s is not in a transitionable state", taskID)
	}

	now := timestamp(s.now())
	task.Frontmatter.Status = status
	task.Frontmatter.UpdatedAt = now
	if note != "" {
		appendTaskNote(task, fmt.Sprintf("- %s: %s", now, note))
	}

	if state.Frontmatter.CurrentTask == taskID {
		state.Frontmatter.CurrentTask = ""
		state.Frontmatter.CurrentTaskTitle = ""
	}
	state.Frontmatter.UpdatedAt = now
	state.Frontmatter.StatusSummary = "idle"
	if status == "blocked" {
		state.Frontmatter.Blockers = []string{fmt.Sprintf("%s: %s", taskID, note)}
		state.Frontmatter.StatusSummary = "blocked"
		state.Frontmatter.NextAction = fmt.Sprintf("Resolve blocker on %s", taskID)
	} else {
		state.Frontmatter.Blockers = []string{}
		state.Frontmatter.NextAction = "Select the next eligible task"
	}
	state.Body = renderStateBody(state.Frontmatter, tasks)

	if err := s.saveAll(state, tasks); err != nil {
		return TransitionResult{}, err
	}

	return TransitionResult{TaskID: taskID, Status: status, UpdatedAt: now}, nil
}

func (s *Service) validateState(state *State) []string {
	violations := make([]string, 0)
	if state.Frontmatter.SchemaVersion != stateSchemaVersion {
		violations = append(violations, fmt.Sprintf("state schema_version must be %d", stateSchemaVersion))
	}
	if strings.TrimSpace(state.Frontmatter.ActiveSpecPath) == "" {
		violations = append(violations, "state active_spec_path must not be empty")
	} else if !fileExists(filepath.Join(s.paths.RepoRoot, filepath.Clean(state.Frontmatter.ActiveSpecPath))) {
		violations = append(violations, fmt.Sprintf("state active_spec_path does not exist: %s", state.Frontmatter.ActiveSpecPath))
	}
	if strings.TrimSpace(state.Frontmatter.StatusSummary) == "" {
		violations = append(violations, "state status_summary must not be empty")
	}
	return violations
}

func (s *Service) validateTasks(state *State, tasks []*Task) []string {
	violations := make([]string, 0)
	seen := make(map[string]struct{}, len(tasks))
	inProgress := make([]*Task, 0)
	for _, task := range tasks {
		if _, ok := seen[task.Frontmatter.ID]; ok {
			violations = append(violations, fmt.Sprintf("duplicate task id %s", task.Frontmatter.ID))
			continue
		}
		seen[task.Frontmatter.ID] = struct{}{}

		if task.Frontmatter.ID == "" {
			violations = append(violations, fmt.Sprintf("task file %s missing id", relPath(s.paths.RepoRoot, task.Filename)))
		}
		if base := filepath.Base(task.Filename); base != task.Frontmatter.ID+".md" {
			violations = append(violations, fmt.Sprintf("task %s filename must be %s.md", task.Frontmatter.ID, task.Frontmatter.ID))
		}
		if _, ok := validStatuses[task.Frontmatter.Status]; !ok {
			violations = append(violations, fmt.Sprintf("task %s has invalid status %q", task.Frontmatter.ID, task.Frontmatter.Status))
		}
		if _, ok := validPriorites[task.Frontmatter.Priority]; !ok {
			violations = append(violations, fmt.Sprintf("task %s has invalid priority %q", task.Frontmatter.ID, task.Frontmatter.Priority))
		}
		if task.Frontmatter.Status == "in_progress" {
			inProgress = append(inProgress, task)
		}
		for _, dep := range task.Frontmatter.Dependencies {
			if dep == task.Frontmatter.ID {
				violations = append(violations, fmt.Sprintf("task %s cannot depend on itself", task.Frontmatter.ID))
			}
		}
		if strings.TrimSpace(task.Frontmatter.SpecRef) == "" {
			violations = append(violations, fmt.Sprintf("task %s missing spec_ref", task.Frontmatter.ID))
		} else if err := s.validateSpecRef(task.Frontmatter.SpecRef); err != nil {
			violations = append(violations, fmt.Sprintf("task %s invalid spec_ref: %v", task.Frontmatter.ID, err))
		}
	}

	for _, task := range tasks {
		for _, dep := range task.Frontmatter.Dependencies {
			if _, ok := seen[dep]; !ok {
				violations = append(violations, fmt.Sprintf("task %s depends on missing task %s", task.Frontmatter.ID, dep))
			}
		}
	}

	if len(inProgress) > 1 {
		ids := make([]string, 0, len(inProgress))
		for _, task := range inProgress {
			ids = append(ids, task.Frontmatter.ID)
		}
		violations = append(violations, fmt.Sprintf("multiple in_progress tasks: %s", strings.Join(ids, ", ")))
	}
	if len(inProgress) == 1 {
		if state.Frontmatter.CurrentTask != inProgress[0].Frontmatter.ID {
			violations = append(violations, fmt.Sprintf("state current_task %q does not match in_progress task %q", state.Frontmatter.CurrentTask, inProgress[0].Frontmatter.ID))
		}
	} else if state.Frontmatter.CurrentTask != "" {
		violations = append(violations, fmt.Sprintf("state current_task %q set but no task is in_progress", state.Frontmatter.CurrentTask))
	}

	return violations
}

func (s *Service) validateSpecRef(specRef string) error {
	parts := strings.SplitN(specRef, "#", 2)
	if len(parts) != 2 {
		return errors.New("spec_ref must include a path and heading anchor")
	}

	pathPart := filepath.Clean(parts[0])
	anchor := strings.TrimSpace(parts[1])
	if anchor == "" {
		return errors.New("spec_ref anchor must not be empty")
	}

	fullPath := filepath.Join(s.paths.RepoRoot, pathPart)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("read spec file: %w", err)
	}
	anchors := collectHeadingAnchors(string(data))
	if _, ok := anchors[anchor]; !ok {
		return fmt.Errorf("heading #%s not found in %s", anchor, pathPart)
	}
	return nil
}

func (s *Service) createFollowupTask(tasks []*Task, source *Task, input VerifyInput) (*Task, error) {
	priority := strings.TrimSpace(input.FollowupPriority)
	if priority == "" {
		priority = "medium"
	}
	if _, ok := validPriorites[priority]; !ok {
		return nil, fmt.Errorf("invalid follow-up priority %q", priority)
	}

	nextID := nextTaskID(tasks)
	title := strings.TrimSpace(input.FollowupTitle)
	if title == "" {
		title = fmt.Sprintf("Follow-up for %s: %s", source.Frontmatter.ID, input.Summary)
	}
	description := strings.TrimSpace(input.FollowupDescription)
	if description == "" {
		description = strings.TrimSpace(input.Details)
	}
	if description == "" {
		description = "Investigate and resolve the verification finding recorded for this task."
	}

	body := fmt.Sprintf("# %s %s\n\n## Description\n\n%s\n\n## Acceptance\n\n- The follow-up issue described by verification is resolved.\n- Verification evidence is updated.\n\n## Verification Notes\n\n- Re-run task-scoped verification after implementing the fix.\n", nextID, title, description)
	task := &Task{
		Frontmatter: TaskFrontmatter{
			ID:           nextID,
			Title:        title,
			Status:       "todo",
			Priority:     priority,
			SpecRef:      source.Frontmatter.SpecRef,
			Dependencies: []string{source.Frontmatter.ID},
			UpdatedAt:    timestamp(s.now()),
		},
		Body:     body,
		Filename: filepath.Join(s.paths.TasksDir, nextID+".md"),
	}
	return task, nil
}

func renderVerificationPlan(task *Task, input VerifyInput, followupTaskID string) string {
	var builder strings.Builder
	builder.WriteString("# Verification Plan\n\n")
	builder.WriteString(fmt.Sprintf("- Task: `%s`\n", task.Frontmatter.ID))
	builder.WriteString(fmt.Sprintf("- Title: %s\n", task.Frontmatter.Title))
	builder.WriteString(fmt.Sprintf("- Requested result: %s\n", input.Result))
	builder.WriteString(fmt.Sprintf("- Summary: %s\n", input.Summary))
	if input.Details != "" {
		builder.WriteString(fmt.Sprintf("- Details: %s\n", input.Details))
	}
	if followupTaskID != "" {
		builder.WriteString(fmt.Sprintf("- Follow-up task to create: `%s`\n", followupTaskID))
	}
	return builder.String()
}

func renderVerificationReportMarkdown(report VerificationArtifact) string {
	var builder strings.Builder
	builder.WriteString("# Verification Report\n\n")
	builder.WriteString(fmt.Sprintf("- Task: `%s`\n", report.TaskID))
	builder.WriteString(fmt.Sprintf("- Title: %s\n", report.TaskTitle))
	builder.WriteString(fmt.Sprintf("- Result: %s\n", report.Result))
	builder.WriteString(fmt.Sprintf("- Summary: %s\n", report.Summary))
	if report.Details != "" {
		builder.WriteString(fmt.Sprintf("- Details: %s\n", report.Details))
	}
	builder.WriteString(fmt.Sprintf("- Generated at: %s\n", report.GeneratedAt))
	builder.WriteString(fmt.Sprintf("- Spec ref: `%s`\n", report.SpecRef))
	for _, artifact := range report.Artifacts {
		builder.WriteString(fmt.Sprintf("- Artifact: `%s`\n", artifact))
	}
	if report.FollowupTaskID != "" {
		builder.WriteString(fmt.Sprintf("- Follow-up task: `%s`\n", report.FollowupTaskID))
	}
	return builder.String()
}

func (s *Service) loadStateAndTasks() (*State, []*Task, error) {
	state, err := s.loadState()
	if err != nil {
		return nil, nil, err
	}
	tasks, err := s.loadTasks()
	if err != nil {
		return nil, nil, err
	}
	return state, tasks, nil
}

func (s *Service) loadState() (*State, error) {
	data, err := os.ReadFile(s.paths.StateFile)
	if err != nil {
		return nil, fmt.Errorf("read state file: %w", err)
	}
	frontmatter, body, err := parseFrontmatter[StateFrontmatter](data)
	if err != nil {
		return nil, fmt.Errorf("parse state file: %w", err)
	}
	return &State{Frontmatter: frontmatter, Body: body}, nil
}

func (s *Service) loadTasks() ([]*Task, error) {
	entries, err := os.ReadDir(s.paths.TasksDir)
	if err != nil {
		return nil, fmt.Errorf("read tasks dir: %w", err)
	}
	tasks := make([]*Task, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		filename := filepath.Join(s.paths.TasksDir, entry.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("read task %s: %w", entry.Name(), err)
		}
		frontmatter, body, err := parseFrontmatter[TaskFrontmatter](data)
		if err != nil {
			return nil, fmt.Errorf("parse task %s: %w", entry.Name(), err)
		}
		tasks = append(tasks, &Task{Frontmatter: frontmatter, Body: body, Filename: filename})
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Frontmatter.ID < tasks[j].Frontmatter.ID
	})
	return tasks, nil
}

func (s *Service) saveAll(state *State, tasks []*Task) error {
	state.Body = renderStateBody(state.Frontmatter, tasks)
	if err := s.saveState(state); err != nil {
		return err
	}
	for _, task := range tasks {
		if err := s.saveTask(task); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) saveState(state *State) error {
	if strings.TrimSpace(state.Body) == "" {
		state.Body = renderStateBody(state.Frontmatter, nil)
	}
	data, err := marshalFrontmatter(state.Frontmatter, state.Body)
	if err != nil {
		return err
	}
	return os.WriteFile(s.paths.StateFile, data, 0o644)
}

func (s *Service) saveTask(task *Task) error {
	data, err := marshalFrontmatter(task.Frontmatter, task.Body)
	if err != nil {
		return err
	}
	return os.WriteFile(task.Filename, data, 0o644)
}

func ensureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", path, err)
	}
	return nil
}

func writeFileIfMissing(path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func relPath(root, target string) string {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return target
	}
	return filepath.ToSlash(rel)
}

func taskByID(tasks []*Task, id string) (*Task, bool) {
	for _, task := range tasks {
		if task.Frontmatter.ID == id {
			return task, true
		}
	}
	return nil, false
}

func eligibleTasks(tasks []*Task) []*Task {
	eligible := make([]*Task, 0)
	for _, task := range tasks {
		if task.Frontmatter.Status != "todo" {
			continue
		}
		if dependenciesResolved(task, tasks) {
			eligible = append(eligible, task)
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		left := priorityRank[eligible[i].Frontmatter.Priority]
		right := priorityRank[eligible[j].Frontmatter.Priority]
		if left != right {
			return left < right
		}
		return eligible[i].Frontmatter.ID < eligible[j].Frontmatter.ID
	})
	return eligible
}

func dependenciesResolved(task *Task, tasks []*Task) bool {
	for _, dep := range task.Frontmatter.Dependencies {
		found, ok := taskByID(tasks, dep)
		if !ok || found.Frontmatter.Status != "completed" {
			return false
		}
	}
	return true
}

func appendTaskNote(task *Task, line string) {
	section := "## Implementation Notes\n\n"
	if strings.Contains(task.Body, section) {
		task.Body = strings.TrimRight(task.Body, "\n") + "\n" + line + "\n"
		return
	}
	task.Body = strings.TrimRight(task.Body, "\n") + "\n\n" + section + line + "\n"
}

func nextTaskID(tasks []*Task) string {
	max := 0
	for _, task := range tasks {
		if !strings.HasPrefix(task.Frontmatter.ID, "T-") {
			continue
		}
		num, err := strconv.Atoi(strings.TrimPrefix(task.Frontmatter.ID, "T-"))
		if err == nil && num > max {
			max = num
		}
	}
	return fmt.Sprintf("T-%03d", max+1)
}

func collectHeadingAnchors(markdown string) map[string]struct{} {
	anchors := make(map[string]struct{})
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		if heading == "" {
			continue
		}
		anchors[slugHeading(heading)] = struct{}{}
	}
	return anchors
}

func slugHeading(heading string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(heading) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-':
			if !lastDash && builder.Len() > 0 {
				builder.WriteRune('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}
