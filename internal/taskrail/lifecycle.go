package taskrail

import (
	"errors"
	"fmt"
	"strings"
)

// The lifecycle writers: the transitions that move one task between statuses and
// re-project STATE.md around it. They share finishTask, which owns the
// completed/blocked write, and each reports the exact result fields its own v0.5
// machine contract names (specs/v0.5.0.md#uniform-agent-machine-results). Every
// writer publishes through the shared normal transaction in lifecycle_tx.go,
// so its bytes land under the repository mutation lock as one validated
// candidate ledger rather than a save-all rewrite.

// validateAfterWrite re-runs validation once a write has committed. A read-back
// failure here is the outcome of an operation that is already on disk, so it is
// tagged applied — otherwise a committed transition would publish as a refusal
// that changed nothing. The transactional writers (lifecycle, verification, and
// the task mutation family in lifecycle_tx.go, verify_tx.go, and task_tx.go) no
// longer use it: they validate the complete candidate before publication
// instead. `spec activate` still does.
func (s *Service) validateAfterWrite() (ValidationResult, error) {
	validation, err := s.Validate()
	if err != nil {
		return ValidationResult{}, WithMachineFailure(
			MachineFailure{Code: MachineCodeRepositoryInvalid, Applied: true}, err)
	}
	return validation, nil
}

func (s *Service) Start(taskID string) (result StartResult, err error) {
	own, release, err := s.beginWriterWrite(lifecycleStart, taskID,
		[]string{s.reportedStatePath(), s.reportedTaskPath(taskID)})
	if err != nil {
		return StartResult{}, err
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil && err == nil {
			err = releaseErr
		}
	}()

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return StartResult{}, err
	}
	corpus, err := snapshotTaskCorpus(tasks)
	if err != nil {
		return StartResult{}, err
	}
	baseline := s.validateInMemory(state, tasks)
	if state.Frontmatter.CurrentTask != "" {
		return StartResult{}, WithMachineErrorCode(MachineCodeInvalidStatus,
			fmt.Errorf("task %s is already active", state.Frontmatter.CurrentTask))
	}

	task, err := exactTaskByID(tasks, taskID)
	if err != nil {
		return StartResult{}, err
	}
	if task.Frontmatter.Status != "todo" {
		return StartResult{}, WithMachineErrorCode(MachineCodeInvalidStatus,
			fmt.Errorf("task %s is not todo", taskID))
	}
	// A dependency that is not yet delivered makes this transition illegal in the
	// current status configuration, which is what invalid_status names; the
	// dependency-specific codes belong to the commands that edit dependencies.
	if !dependenciesResolved(task, tasks) {
		return StartResult{}, WithMachineErrorCode(MachineCodeInvalidStatus,
			fmt.Errorf("task %s has unresolved dependencies", taskID))
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

	validation, err := s.commitLifecycle(own, lifecycleStart, lifecycleLedger{state: state, task: task, preview: tasks, corpus: corpus, baseline: baseline})
	if err != nil {
		return StartResult{}, err
	}
	return StartResult{
		TaskID:     task.Frontmatter.ID,
		Status:     task.Frontmatter.Status,
		UpdatedAt:  now,
		Validation: validation,
	}, nil
}

func (s *Service) Complete(taskID, note string) (CompleteResult, error) {
	out, err := s.finishTask(lifecycleComplete, taskID, "completed", strings.TrimSpace(note))
	if err != nil {
		return CompleteResult{}, err
	}
	return CompleteResult{
		TaskID:       out.task.Frontmatter.ID,
		Status:       out.task.Frontmatter.Status,
		UpdatedAt:    out.updatedAt,
		CompletionID: out.task.Frontmatter.CompletionID,
		Validation:   out.validation,
	}, nil
}

func (s *Service) Block(taskID, reason string) (BlockResult, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return BlockResult{}, WithMachineErrorCode(MachineCodeInvalidReason,
			errors.New("block reason must not be empty"))
	}
	out, err := s.finishTask(lifecycleBlock, taskID, "blocked", reason)
	if err != nil {
		return BlockResult{}, err
	}
	return BlockResult{
		TaskID:     out.task.Frontmatter.ID,
		Status:     out.task.Frontmatter.Status,
		Reason:     reason,
		UpdatedAt:  out.updatedAt,
		Validation: out.validation,
	}, nil
}

// Unblock is the inverse of Block: it returns a blocked task to todo so it
// re-enters next selection, drops only that task's blockers entry (other blocked
// tasks keep their reasons), and, when reason is non-empty, records a timestamped
// Implementation Notes line — the reason is never re-added to the blockers list.
// It then re-renders STATE.md and re-runs validation, reporting the result
// (mirrors ActivateSpec per specs/v0.3.0.md#task-unblocking).
func (s *Service) Unblock(taskID, reason string) (result UnblockResult, err error) {
	own, release, err := s.beginWriterWrite(lifecycleUnblock, taskID,
		[]string{s.reportedStatePath(), s.reportedTaskPath(taskID)})
	if err != nil {
		return UnblockResult{}, err
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil && err == nil {
			err = releaseErr
		}
	}()

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return UnblockResult{}, err
	}
	corpus, err := snapshotTaskCorpus(tasks)
	if err != nil {
		return UnblockResult{}, err
	}
	baseline := s.validateInMemory(state, tasks)
	task, err := exactTaskByID(tasks, taskID)
	if err != nil {
		return UnblockResult{}, err
	}
	if task.Frontmatter.Status != "blocked" {
		return UnblockResult{}, WithMachineErrorCode(MachineCodeInvalidStatus,
			fmt.Errorf("task %s is not blocked", taskID))
	}

	// Unblock's reason is an optional note, not a blocking reason, so its contract
	// classifies a rejected one as an invalid argument.
	if err := ensurePortableNote("reason", strings.TrimSpace(reason)); err != nil {
		return UnblockResult{}, WithMachineErrorCode(MachineCodeInvalidArguments, err)
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

	validation, err := s.commitLifecycle(own, lifecycleUnblock, lifecycleLedger{state: state, task: task, preview: tasks, corpus: corpus, baseline: baseline})
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

// noteField labels the note argument the way the operator typed it — complete
// takes a --note, block a --reason — and names the code a rejected one
// publishes. Only block's reason is a blocking reason, so only it is
// invalid_reason; complete's contract does not admit that code.
func noteField(status string) (label, code string) {
	if status == "blocked" {
		return "reason", MachineCodeInvalidReason
	}
	return "note", MachineCodeInvalidArguments
}

// transitionOutcome is what a completed/blocked transition left behind: the task
// it wrote, the timestamp it stamped, and the validation it re-ran. Each caller
// projects it onto the result fields its own machine contract names.
type transitionOutcome struct {
	task       *Task
	updatedAt  string
	validation ValidationResult
}

func (s *Service) finishTask(w writerCommand, taskID, status, note string) (outcome transitionOutcome, err error) {
	own, release, err := s.beginWriterWrite(w, taskID,
		[]string{s.reportedStatePath(), s.reportedTaskPath(taskID)})
	if err != nil {
		return transitionOutcome{}, err
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil && err == nil {
			err = releaseErr
		}
	}()

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return transitionOutcome{}, err
	}
	corpus, err := snapshotTaskCorpus(tasks)
	if err != nil {
		return transitionOutcome{}, err
	}
	baseline := s.validateInMemory(state, tasks)
	task, err := exactTaskByID(tasks, taskID)
	if err != nil {
		return transitionOutcome{}, err
	}
	if task.Frontmatter.Status != "in_progress" && !(status == "blocked" && task.Frontmatter.Status == "todo") {
		return transitionOutcome{}, WithMachineErrorCode(MachineCodeInvalidStatus,
			fmt.Errorf("task %s is not in a transitionable state", taskID))
	}

	label, code := noteField(status)
	if err := ensurePortableNote(label, note); err != nil {
		return transitionOutcome{}, WithMachineErrorCode(code, err)
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

	validation, err := s.commitLifecycle(own, w, lifecycleLedger{state: state, task: task, preview: tasks, corpus: corpus, baseline: baseline})
	if err != nil {
		return transitionOutcome{}, err
	}
	return transitionOutcome{task: task, updatedAt: now, validation: validation}, nil
}
