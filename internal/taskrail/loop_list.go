package taskrail

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// TaskLoopRow is the complete policy and unattended-eligibility view of one
// decodable task. It is a report projection only; it never changes task state.
type TaskLoopRow struct {
	TaskID           string   `json:"task_id"`
	Status           string   `json:"status"`
	ActiveSpec       bool     `json:"active_spec"`
	Source           string   `json:"source"`
	EffectivePolicy  string   `json:"effective_policy"`
	Reason           string   `json:"reason"`
	Eligible         bool     `json:"eligible"`
	HeldDependencies []string `json:"held_dependencies"`
	Disposition      string   `json:"disposition"`
}

// TaskLoopListResult is the deterministic partial report for `task loop list`.
// Violations do not suppress decodable rows, so an operator can inspect the
// policy ledger before repairing malformed files.
type TaskLoopListResult struct {
	Tasks      []TaskLoopRow      `json:"tasks"`
	Violations []MachineViolation `json:"violations"`
}

// TaskLoopSelectionResult is the read-only policy-filtered selection that loop
// preflight uses. The full row is retained so later command surfaces do not need
// a second ranking or policy projection.
type TaskLoopSelectionResult struct {
	Action       string             `json:"action"`
	Reason       string             `json:"reason"`
	SelectedTask *TaskLoopRow       `json:"selected_task"`
	Tasks        []TaskLoopRow      `json:"tasks"`
	Violations   []MachineViolation `json:"violations"`
}

// TaskLoopList reports every decodable task's policy without changing ordinary
// lifecycle selection or writing any managed file.
func (s *Service) TaskLoopList() (TaskLoopListResult, error) {
	state, err := s.loadState()
	if err != nil {
		return TaskLoopListResult{}, err
	}
	tasks, violations, decodedInvalid, err := s.loadLoopListTasks()
	if err != nil {
		return TaskLoopListResult{}, err
	}
	return buildTaskLoopListResult(s, state, tasks, violations, decodedInvalid), nil
}

func buildTaskLoopListResult(s *Service, state *State, tasks []*Task, violations []MachineViolation, decodedInvalid map[*Task]bool) TaskLoopListResult {
	validation := append(s.layoutViolations(), s.validateState(state)...)
	validation = append(validation, s.validateTasks(state, tasks)...)
	validation = append(validation, s.validateVerificationEvidence(state, tasks)...)
	invalid := invalidLoopTasks(tasks, validation, decodedInvalid)
	for _, message := range validation {
		violations = append(violations, MachineViolation{Code: MachineCodeRepositoryInvalid, Message: message})
	}
	sortLoopViolations(violations)

	rows := make([]TaskLoopRow, 0, len(tasks))
	for _, task := range tasks {
		policy := loopRowPolicy(task.Frontmatter.LoopPolicyMetadata)
		held := heldDependencyClosure(task, tasks)
		disposition := loopDisposition(task, state, tasks, policy, held, invalid[task])
		rows = append(rows, TaskLoopRow{
			TaskID: task.Frontmatter.ID, Status: task.Frontmatter.Status,
			ActiveSpec: taskMatchesActiveSpec(task, state.Frontmatter.ActiveSpecPath),
			Source:     policy.Source, EffectivePolicy: policy.Policy, Reason: policy.Reason,
			Eligible: disposition == "eligible", HeldDependencies: held, Disposition: disposition,
		})
	}
	return TaskLoopListResult{Tasks: rows, Violations: violations}
}

// TaskLoopSelect chooses an explicitly allowed, active-spec task with the same
// priority and ID ordering as ordinary status selection. It is read-only and
// returns invalid input as an inspectable report for loop dry-run to gate.
func (s *Service) TaskLoopSelect() (TaskLoopSelectionResult, error) {
	state, err := s.loadState()
	if err != nil {
		return TaskLoopSelectionResult{}, err
	}
	tasks, violations, decodedInvalid, err := s.loadLoopListTasks()
	if err != nil {
		return TaskLoopSelectionResult{}, err
	}
	report := buildTaskLoopListResult(s, state, tasks, violations, decodedInvalid)
	result := TaskLoopSelectionResult{Tasks: report.Tasks, Violations: report.Violations}
	if len(result.Violations) != 0 {
		result.Action = "invalid"
		result.Reason = "repository contains loop-policy violations"
		return result, nil
	}

	byID := make(map[string]*Task, len(tasks))
	for _, task := range tasks {
		byID[task.Frontmatter.ID] = task
	}
	candidates := make([]int, 0)
	for i, row := range result.Tasks {
		if row.Eligible && row.Source == "explicit" && row.EffectivePolicy == "allow" {
			candidates = append(candidates, i)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		left := byID[result.Tasks[candidates[i]].TaskID]
		right := byID[result.Tasks[candidates[j]].TaskID]
		leftRank := priorityRank[left.Frontmatter.Priority]
		rightRank := priorityRank[right.Frontmatter.Priority]
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return left.Frontmatter.ID < right.Frontmatter.ID
	})
	if len(candidates) == 0 {
		result.Action = "none"
		result.Reason = "no eligible allowed task"
		return result, nil
	}

	result.Action = "run"
	result.Reason = "selected allowed task by priority and stable task id"
	selected := result.Tasks[candidates[0]]
	result.SelectedTask = &selected
	return result, nil
}

func (s *Service) loadLoopListTasks() ([]*Task, []MachineViolation, map[*Task]bool, error) {
	if err := s.paths.ensureStorageCapability(); err != nil {
		return nil, nil, nil, err
	}
	entries, err := os.ReadDir(s.paths.TasksDir)
	if err != nil {
		return nil, nil, nil, WithMachineErrorCode(missingOrInvalidCode(err, MachineCodeNotInitialized),
			fmt.Errorf("read tasks dir %s: %w", s.paths.logicalManagedPath(s.paths.TasksDir), fsCause(err)))
	}
	tasks := make([]*Task, 0, len(entries))
	violations := make([]MachineViolation, 0)
	decodedInvalid := make(map[*Task]bool)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		filename := filepath.Join(s.paths.TasksDir, entry.Name())
		logical := s.paths.logicalManagedPath(filename)
		data, readErr := os.ReadFile(filename)
		if readErr != nil {
			violations = append(violations, loopViolation(logical, fmt.Sprintf("read task: %v", fsCause(readErr))))
			continue
		}
		frontmatter, body, parseErr := parseFrontmatter[TaskFrontmatter](data)
		if parseErr != nil {
			id, ok := taskIDFromFrontmatter(data)
			if !ok {
				violations = append(violations, loopViolation(logical, fmt.Sprintf("parse task: %v", parseErr)))
				continue
			}
			task := &Task{Frontmatter: TaskFrontmatter{ID: id}, Path: logical, Filename: filename}
			tasks = append(tasks, task)
			decodedInvalid[task] = true
			violations = append(violations, loopViolation(logical, fmt.Sprintf("task %s parse frontmatter: %v", id, parseErr)))
			continue
		}
		if frontmatter.ID == "" {
			violations = append(violations, loopViolation(logical, "task missing required id"))
			continue
		}
		tasks = append(tasks, &Task{Frontmatter: frontmatter, Body: body, Path: logical, Filename: filename})
	}
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Frontmatter.ID == tasks[j].Frontmatter.ID {
			return tasks[i].Path < tasks[j].Path
		}
		return tasks[i].Frontmatter.ID < tasks[j].Frontmatter.ID
	})
	return tasks, violations, decodedInvalid, nil
}

func loopViolation(path, message string) MachineViolation {
	return MachineViolation{Code: MachineCodeRepositoryInvalid, Message: message, Path: &path}
}

func sortLoopViolations(violations []MachineViolation) {
	sort.Slice(violations, func(i, j int) bool {
		return slices.Compare(violationOrderKey(violations[i]), violationOrderKey(violations[j])) < 0
	})
}

func invalidLoopTasks(tasks []*Task, violations []string, invalid map[*Task]bool) map[*Task]bool {
	for _, task := range tasks {
		id := task.Frontmatter.ID
		for _, violation := range violations {
			if strings.Contains(violation, "task "+id+" ") ||
				strings.Contains(violation, "task id "+id) ||
				strings.Contains(violation, task.Path) ||
				(strings.HasPrefix(violation, "dependency cycle detected:") && taskIDInViolation(id, violation)) ||
				(strings.HasPrefix(violation, "tasks ") && strings.Contains(violation, " share numeric id prefix") && taskIDInViolation(id, violation)) {
				invalid[task] = true
			}
		}
	}
	return invalid
}

func taskIDInViolation(id, violation string) bool {
	for _, token := range strings.FieldsFunc(violation, func(r rune) bool {
		return r != '-' && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z')
	}) {
		if token == id {
			return true
		}
	}
	return false
}

func loopRowPolicy(meta LoopPolicyMetadata) EffectiveLoopPolicy {
	policy := ResolveLoopPolicy(meta)
	if len(ValidateLoopPolicyMetadata(meta)) != 0 {
		return EffectiveLoopPolicy{Source: "default", Policy: "hold", Reason: DefaultLoopReason}
	}
	return policy
}

func heldDependencyClosure(task *Task, tasks []*Task) []string {
	held := make(map[string]bool)
	seen := make(map[string]bool)
	var visit func(string)
	visit = func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		dependency, ok := taskByID(tasks, id)
		if !ok {
			return
		}
		if dependency.Frontmatter.Status != "completed" && loopRowPolicy(dependency.Frontmatter.LoopPolicyMetadata).Policy == "hold" {
			held[id] = true
		}
		for _, dep := range dependency.Frontmatter.Dependencies {
			visit(dep)
		}
	}
	for _, dep := range task.Frontmatter.Dependencies {
		visit(dep)
	}
	ids := make([]string, 0, len(held))
	for id := range held {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func loopDisposition(task *Task, state *State, tasks []*Task, policy EffectiveLoopPolicy, held []string, invalid bool) string {
	if invalid {
		return "invalid"
	}
	if policy.Policy == "hold" {
		for _, candidate := range tasks {
			candidatePolicy := loopRowPolicy(candidate.Frontmatter.LoopPolicyMetadata)
			if candidate.Frontmatter.Status == "todo" && taskMatchesActiveSpec(candidate, state.Frontmatter.ActiveSpecPath) && candidatePolicy.Policy == "allow" && slices.Contains(heldDependencyClosure(candidate, tasks), task.Frontmatter.ID) {
				return "held_dependency"
			}
		}
		return "held"
	}
	if task.Frontmatter.Status != "todo" {
		return "status_ineligible"
	}
	if !taskMatchesActiveSpec(task, state.Frontmatter.ActiveSpecPath) {
		return "off_spec"
	}
	if !dependenciesResolved(task, tasks) {
		return "waiting_dependency"
	}
	return "eligible"
}
