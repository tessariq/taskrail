package taskrail

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func (s *Service) Next() (NextResult, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return NextResult{}, err
	}

	result := computeNext(state, tasks)
	state.Frontmatter.UpdatedAt = timestamp(s.now())
	state.Frontmatter.NextAction = nextAction(result)
	state.Body = renderStateBody(state.Frontmatter, tasks)
	if err := s.saveState(state); err != nil {
		return NextResult{}, err
	}
	return result, nil
}

// computeNext resolves the next-task selection without persisting anything.
// Next() wraps it to also record next_action/updated_at; status reuses it to
// report the selection read-only, so the selection logic lives in one place.
func computeNext(state *State, tasks []*Task) NextResult {
	if state.Frontmatter.CurrentTask != "" {
		if task, ok := taskByID(tasks, state.Frontmatter.CurrentTask); ok && task.Frontmatter.Status == "in_progress" {
			return NextResult{
				TaskID:     task.Frontmatter.ID,
				Title:      task.Frontmatter.Title,
				Priority:   task.Frontmatter.Priority,
				Reason:     "active task already in progress",
				Candidates: []string{task.Frontmatter.ID},
				Warnings:   nextSelectionWarnings(state, task),
			}
		}
	}

	candidates := eligibleTasks(tasks)
	ids := make([]string, 0, len(candidates))
	for _, task := range candidates {
		ids = append(ids, task.Frontmatter.ID)
	}
	if len(candidates) == 0 {
		return NextResult{Reason: "no eligible task", Candidates: ids}
	}

	selected := candidates[0]
	return NextResult{
		TaskID:     selected.Frontmatter.ID,
		Title:      selected.Frontmatter.Title,
		Priority:   selected.Frontmatter.Priority,
		Reason:     "next eligible todo by priority and stable task id",
		Candidates: ids,
		Warnings:   nextSelectionWarnings(state, selected),
	}
}

func nextSelectionWarnings(state *State, task *Task) []Warning {
	activeSpecPath := strings.TrimSpace(state.Frontmatter.ActiveSpecPath)
	if activeSpecPath == "" || task == nil {
		return nil
	}
	specPath, _, err := parseSpecRef(task.Frontmatter.SpecRef)
	if err != nil {
		return nil
	}
	if normalizeSpecPath(specPath) == normalizeSpecPath(activeSpecPath) {
		return nil
	}
	return []Warning{{
		Code:           "selected_non_active_spec",
		Message:        fmt.Sprintf("warning: selected task %s points at %s while active spec is %s", task.Frontmatter.ID, task.Frontmatter.SpecRef, activeSpecPath),
		TaskID:         task.Frontmatter.ID,
		SpecRef:        task.Frontmatter.SpecRef,
		ActiveSpecPath: activeSpecPath,
	}}
}

func normalizeSpecPath(path string) string {
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
}

// nextAction renders the STATE.md next_action string that `next` persists for a
// given selection. It is the write-side counterpart to computeNext.
func nextAction(result NextResult) string {
	switch result.Reason {
	case "active task already in progress":
		return fmt.Sprintf("Continue task %s", result.TaskID)
	case "no eligible task":
		return "No eligible task is ready"
	default:
		return fmt.Sprintf("Start task %s: %s", result.TaskID, result.Title)
	}
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
	state.Frontmatter.StatusSummary = statusSummaryInProgress
	// Starting a task clears only its own stale blocker entry (if any); other
	// tasks may still be blocked and must keep their recorded reasons.
	state.Frontmatter.Blockers = removeBlocker(state.Frontmatter.Blockers, task.Frontmatter.ID)
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

// Unblock is the inverse of Block: it returns a blocked task to todo so it
// re-enters next selection, drops only that task's blockers entry (other blocked
// tasks keep their reasons), and, when reason is non-empty, records a timestamped
// Implementation Notes line — the reason is never re-added to the blockers list.
// It then re-renders STATE.md and re-runs validation, reporting the result
// (mirrors ActivateSpec per specs/v0.3.0.md#task-unblocking).
func (s *Service) Unblock(taskID, reason string) (UnblockResult, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return UnblockResult{}, err
	}
	task, ok := taskByID(tasks, taskID)
	if !ok {
		return UnblockResult{}, fmt.Errorf("task %s not found", taskID)
	}
	if task.Frontmatter.Status != "blocked" {
		return UnblockResult{}, fmt.Errorf("task %s is not blocked", taskID)
	}

	now := timestamp(s.now())
	task.Frontmatter.Status = "todo"
	task.Frontmatter.UpdatedAt = now
	if note := strings.TrimSpace(reason); note != "" {
		appendTaskNote(task, fmt.Sprintf("- %s: %s", now, note))
	}

	state.Frontmatter.UpdatedAt = now
	// Drop only this task's stale blocker entry; other tasks may still be blocked
	// and must keep their recorded reasons (mirrors finishTask's drop-only path).
	state.Frontmatter.Blockers = removeBlocker(state.Frontmatter.Blockers, taskID)
	// An active task owns the summary/next_action pointers, so leave them; only with
	// no active task does reconcileIdlePointers re-derive them from the ledger just
	// updated above (never pointing at the task we unblocked, whose entry is gone).
	if state.Frontmatter.CurrentTask == "" {
		reconcileIdlePointers(&state.Frontmatter)
	}
	state.Body = renderStateBody(state.Frontmatter, tasks)

	if err := s.saveAll(state, tasks); err != nil {
		return UnblockResult{}, err
	}

	validation, err := s.Validate()
	if err != nil {
		return UnblockResult{}, err
	}
	return UnblockResult{
		TaskID:     taskID,
		Status:     task.Frontmatter.Status,
		UpdatedAt:  now,
		Validation: validation,
	}, nil
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
	if err := ensureDir(s.paths.RepoRoot, artifactDir); err != nil {
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

	relPlan := relPath(s.paths.RepoRoot, planPath)
	relReport := relPath(s.paths.RepoRoot, reportPath)
	relReportMarkdown := relPath(s.paths.RepoRoot, reportMarkdownPath)

	plan := renderVerificationPlan(task, input, followupTaskID)
	if err := os.WriteFile(planPath, []byte(plan), 0o644); err != nil {
		return VerifyResult{}, fmt.Errorf("write verification plan %s: %w", relPlan, fsCause(err))
	}

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
		return VerifyResult{}, fmt.Errorf("write verification report %s: %w", relReport, fsCause(err))
	}

	reportMarkdown := renderVerificationReportMarkdown(report)
	if err := os.WriteFile(reportMarkdownPath, []byte(reportMarkdown), 0o644); err != nil {
		return VerifyResult{}, fmt.Errorf("write verification markdown report %s: %w", relReportMarkdown, fsCause(err))
	}

	nowText := timestamp(now)
	// Task files are committed, so the note must stay portable: record the
	// result and timestamp without a path into gitignored artifacts (mirrors
	// the path-free state summary below).
	appendTaskNote(task, verificationNoteLine(nowText, input.Result))
	task.Frontmatter.UpdatedAt = nowText

	state.Frontmatter.UpdatedAt = nowText
	// Keep committed state portable: record a path-free summary and list no
	// gitignored artifact paths. Local evidence still lives under
	// planning/artifacts/verify/ for the producer (see VerifyResult).
	state.Frontmatter.LastVerificationResult = fmt.Sprintf("%s for %s at %s", input.Result, task.Frontmatter.ID, nowText)
	state.Frontmatter.RelevantArtifacts = nil
	if input.Result == "fail" && followupTaskID != "" {
		state.Frontmatter.NextAction = fmt.Sprintf("Review follow-up task %s", followupTaskID)
	} else if input.Result == "fail" {
		state.Frontmatter.NextAction = fmt.Sprintf("Resolve verification findings for %s", task.Frontmatter.ID)
	} else {
		state.Frontmatter.NextAction = nextActionSelectEligible
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

// taskValidationOpts carries the import-specific relaxations validateTaskCreatable
// honors: resolving a spec_ref against a not-yet-written imported spec, and
// accepting a dependency that names an in-draft key a sibling task will create.
// The zero value is the strict, on-disk mode CreateTask uses.
type taskValidationOpts struct {
	pending   *pendingSpec
	draftKeys map[string]struct{}
}

// validateTaskCreatable runs the spec-and-dependency live-repo checks CreateTask
// enforces before it writes — non-empty spec_ref with a resolvable heading, a
// valid priority, and existing dependencies — and returns the normalized
// priority. Writing nothing, it is the shared pre-write validator import
// pre-flight (T-041) reuses to reject a whole draft before any file lands, so
// any check added *here* is enforced on both paths. Title emptiness is
// deliberately not checked on either path: CreateTask allows a bare, title-less
// scaffold (T-095), while the import path independently requires a non-empty
// title via ValidateImportDraft.
func (s *Service) validateTaskCreatable(tasks []*Task, specRef, priority string, deps []string, opts taskValidationOpts) (string, error) {
	if err := s.validateSpecRefWithPending(specRef, opts.pending); err != nil {
		return "", fmt.Errorf("invalid spec_ref: %w", err)
	}
	priority = strings.TrimSpace(priority)
	if priority == "" {
		priority = "medium"
	}
	if _, ok := validPriorites[priority]; !ok {
		return "", fmt.Errorf("invalid priority %q", priority)
	}
	for _, dep := range deps {
		if _, ok := opts.draftKeys[dep]; ok {
			continue // an in-draft key: a sibling draft task will create it
		}
		if _, ok := taskByID(tasks, dep); !ok {
			return "", fmt.Errorf("dependency %s does not exist", dep)
		}
	}
	return priority, nil
}

// resolveAreaSpecRef turns a `--area <anchor>` shorthand into the full
// `<active_spec_path>#<anchor>` spec_ref, validating the anchor through the same
// path as an explicit `--spec-ref` so the set of accepted anchors never diverges.
// On an unknown anchor it points the operator at the active spec's real anchors.
func (s *Service) resolveAreaSpecRef(state *State, area string) (string, error) {
	activePath := strings.TrimSpace(state.Frontmatter.ActiveSpecPath)
	if activePath == "" {
		return "", errors.New("--area requires an active spec, but planning/STATE.md has none set")
	}
	specRef := activePath + "#" + area
	if err := s.validateSpecRef(specRef); err != nil {
		return "", fmt.Errorf("unknown active-spec area %q: %w; run `taskrail spec show %s --anchors` to list valid anchors", area, err, state.Frontmatter.ActiveSpecVersion)
	}
	return specRef, nil
}

// CreateTask scaffolds a well-formed task file with the next free id. It mirrors
// the validation `validate` would apply (spec anchor, dependency existence,
// priority) at creation time so an invalid task never lands on disk.
func (s *Service) CreateTask(input CreateTaskInput) (CreateTaskResult, error) {
	// Title is optional: a scaffold with neither a title nor a slug is a legitimate
	// bare `T-<n>` task, matching the id form validate already accepts.
	title := strings.TrimSpace(input.Title)

	// A task has exactly one resolved spec reference: --area is the active-spec
	// shorthand for --spec-ref, so the two cannot both be given. Reject before any
	// load or write so a conflicting request lands nothing on disk.
	area := strings.TrimSpace(input.Area)
	specRef := strings.TrimSpace(input.SpecRef)
	if area != "" && specRef != "" {
		return CreateTaskResult{}, errors.New("--area and --spec-ref are mutually exclusive")
	}

	// Load first: a follow-up needs the parent task to inherit spec_ref and wire
	// the dependency before the shared validation below runs.
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return CreateTaskResult{}, err
	}

	// Resolve --area against STATE.md's active spec before follow-up inheritance so
	// an explicit area overrides a parent's inherited ref.
	if area != "" {
		specRef, err = s.resolveAreaSpecRef(state, area)
		if err != nil {
			return CreateTaskResult{}, err
		}
	}

	deps := append([]string(nil), input.Dependencies...)
	followUpOf := strings.TrimSpace(input.FollowUpOf)
	if followUpOf != "" {
		parent, ok := taskByID(tasks, followUpOf)
		if !ok {
			return CreateTaskResult{}, fmt.Errorf("follow-up parent %s does not exist", followUpOf)
		}
		if specRef == "" {
			specRef = parent.Frontmatter.SpecRef
		}
		if !slices.Contains(deps, followUpOf) {
			deps = append(deps, followUpOf)
		}
	}

	priority, err := s.validateTaskCreatable(tasks, specRef, input.Priority, deps, taskValidationOpts{})
	if err != nil {
		return CreateTaskResult{}, err
	}

	// The id and filename are two encodings of one identifier: bake the slug (if
	// any) into the id so `filename == "<id>.md"` holds. nextTaskID keys on the
	// numeric prefix, so a slug suffix never affects id allocation or collision.
	nextID := nextTaskID(tasks)
	if slug := slugify(input.Slug); slug != "" {
		nextID = nextID + "-" + slug
	}
	now := timestamp(s.now())
	var provenance string
	if followUpOf != "" {
		provenance = fmt.Sprintf("Follow-up derived from %s's verification or discovery.", followUpOf)
	}
	body := renderNewTaskBody(nextID, title, provenance)
	newTask := &Task{
		Frontmatter: TaskFrontmatter{
			ID:           nextID,
			Title:        title,
			Status:       "todo",
			Priority:     priority,
			SpecRef:      specRef,
			Dependencies: deps,
			UpdatedAt:    now,
		},
		Body:     body,
		Filename: filepath.Join(s.paths.TasksDir, nextID+".md"),
	}

	// Write the durable task file first, then re-render STATE.md counts from the
	// full set (existing task files are left untouched). Ordering the task write
	// first means a failed state write leaves a real task with a stale count that
	// the next state-writing command heals, never a counted-but-absent task.
	if err := s.saveTask(newTask); err != nil {
		return CreateTaskResult{}, err
	}
	state.Frontmatter.UpdatedAt = now
	state.Body = renderStateBody(state.Frontmatter, append(tasks, newTask))
	if err := s.saveState(state); err != nil {
		return CreateTaskResult{}, err
	}

	return CreateTaskResult{
		TaskID:   nextID,
		Title:    title,
		Priority: priority,
		SpecRef:  specRef,
		Path:     relPath(s.paths.RepoRoot, newTask.Filename),
	}, nil
}

// reconcileIdlePointers sets status_summary/next_action for the no-active-task
// state from the blockers ledger, the single reconciliation Unblock and both
// finishTask branches share. Callers must upsert/drop the ledger for the current
// transition first, then invoke this only when current_task is empty (an active
// task owns those pointers). While any blocker remains, stay "blocked" pointing at
// the most-recently recorded one — for the block branch that is the just-blocked
// task (upsertBlocker appends it last); for complete/unblock it is a still-blocked
// sibling. Only once the ledger is empty do the neutral idle pointers apply.
func reconcileIdlePointers(fm *StateFrontmatter) {
	if remaining := fm.Blockers; len(remaining) > 0 {
		fm.StatusSummary = statusSummaryBlocked
		fm.NextAction = fmt.Sprintf("Resolve blocker on %s", blockerID(remaining[len(remaining)-1]))
		return
	}
	fm.StatusSummary = statusSummaryIdle
	fm.NextAction = nextActionSelectEligible
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
	// The blockers ledger is per-task and must always reflect this transition,
	// even when a different task stays active.
	if status == "blocked" {
		state.Frontmatter.Blockers = upsertBlocker(state.Frontmatter.Blockers, taskID, note)
	} else {
		// Completing one task must not erase reasons recorded for other tasks that
		// are still blocked; drop only this task's own entry.
		state.Frontmatter.Blockers = removeBlocker(state.Frontmatter.Blockers, taskID)
	}

	// status_summary/next_action belong to the active task, so only reconcile them
	// when this transition left none in progress (current_task cleared above iff the
	// finished task was itself active). Mirrors Unblock's guard so blocking a todo
	// never clobbers a still-active task's summary; the ledger reconciliation itself
	// lives in reconcileIdlePointers.
	if state.Frontmatter.CurrentTask == "" {
		reconcileIdlePointers(&state.Frontmatter)
	}
	state.Body = renderStateBody(state.Frontmatter, tasks)

	if err := s.saveAll(state, tasks); err != nil {
		return TransitionResult{}, err
	}

	return TransitionResult{TaskID: taskID, Status: status, UpdatedAt: now}, nil
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

	body := renderFollowupTaskBody(nextID, title, description)
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
