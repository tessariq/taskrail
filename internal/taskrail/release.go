package taskrail

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tessariq/taskrail/internal/repotx"
)

// ReleaseTaskInput selects the one active task an operator deliberately returns
// to todo. DryRun builds the identical candidate without acquiring a write lock.
type ReleaseTaskInput struct {
	TaskID string
	Reason string
	DryRun bool
}

// testHookReleaseCandidatePrepared exposes the preimage/snapshot boundary to
// transaction tests. It is nil outside tests.
var testHookReleaseCandidatePrepared func()

// ReleaseTask relinquishes a direct operator's active task without inventing a
// blocker or cancellation record. It requires a fully consistent active pointer
// so an interrupted repository is never "repaired" by clearing unrelated state.
func (s *Service) ReleaseTask(input ReleaseTaskInput) (result ReleaseTaskResult, err error) {
	if input.TaskID == "" {
		return ReleaseTaskResult{}, WithMachineErrorCode(MachineCodeInvalidArguments, errors.New("task id is required"))
	}
	reason, err := validateReleaseReason(input.Reason)
	if err != nil {
		return ReleaseTaskResult{}, err
	}
	if delegatedInvocation() {
		return ReleaseTaskResult{}, WithMachineErrorCode(MachineCodeDelegatedRefused,
			errors.New("delegated loop children cannot release work"))
	}

	var release func() error
	var own repotx.Ownership
	if !input.DryRun {
		own, release, err = s.beginWriterWrite(lifecycleRelease, input.TaskID,
			[]string{s.reportedStatePath(), s.reportedTaskPath(input.TaskID)})
		if err != nil {
			return ReleaseTaskResult{}, err
		}
		defer func() {
			if releaseErr := release(); releaseErr != nil && err == nil {
				err = releaseErr
			}
		}()
	}

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return ReleaseTaskResult{}, err
	}
	task, err := exactTaskByID(tasks, input.TaskID)
	if err != nil {
		return ReleaseTaskResult{}, err
	}
	if task.Frontmatter.Status != "in_progress" {
		return ReleaseTaskResult{}, WithMachineErrorCode(MachineCodeInvalidStatus,
			fmt.Errorf("task %s is not in_progress", input.TaskID))
	}
	if state.Frontmatter.CurrentTask != task.Frontmatter.ID || state.Frontmatter.CurrentTaskTitle != task.Frontmatter.Title || len(inProgressTasks(tasks)) != 1 {
		return ReleaseTaskResult{}, WithMachineErrorCode(MachineCodeRepositoryInvalid,
			fmt.Errorf("active pointer does not consistently name task %s", input.TaskID))
	}

	before, err := os.ReadFile(task.Filename)
	if err != nil {
		return ReleaseTaskResult{}, fmt.Errorf("read task %s: %w", task.Path, fsCause(err))
	}
	candidate := *task
	now := timestamp(s.now())
	candidate.Frontmatter.Status = "todo"
	candidate.Frontmatter.UpdatedAt = now
	appendTaskNote(&candidate, fmt.Sprintf("- %s: %s", now, reason))
	after, err := patchLifecycleTaskBytes(before, &candidate, map[string]string{
		"status": "todo", "updated_at": fmt.Sprintf("%q", now),
	})
	if err != nil {
		return ReleaseTaskResult{}, err
	}
	if testHookReleaseCandidatePrepared != nil {
		testHookReleaseCandidatePrepared()
	}

	stateCandidate := *state
	stateCandidate.Frontmatter.CurrentTask = ""
	stateCandidate.Frontmatter.CurrentTaskTitle = ""
	stateCandidate.Frontmatter.UpdatedAt = now
	stateCandidate.Frontmatter.Blockers = removeBlocker(stateCandidate.Frontmatter.Blockers, input.TaskID)
	reconcileIdlePointers(&stateCandidate.Frontmatter)
	preview := replaceReleaseTask(tasks, task, &candidate)
	stateCandidate.Body = renderStateBody(stateCandidate.Frontmatter, preview)
	validation := s.validateInMemory(&stateCandidate, preview)
	currentBefore := task.Frontmatter.ID
	result = ReleaseTaskResult{
		TaskID:             task.Frontmatter.ID,
		PriorStatus:        task.Frontmatter.Status,
		Status:             candidate.Frontmatter.Status,
		Reason:             reason,
		Applied:            !input.DryRun,
		CurrentTaskBefore:  &currentBefore,
		CurrentTaskCleared: true,
		TaskSHA256Before:   digestBytes(before),
		TaskSHA256After:    digestBytes(after),
		Validation:         validation,
	}
	if err := s.CheckRecovery(); err != nil {
		return ReleaseTaskResult{}, err
	}
	if input.DryRun {
		return result, nil
	}

	corpus, err := snapshotTaskCorpus(tasks)
	if err != nil {
		return ReleaseTaskResult{}, err
	}
	baseline := s.validateInMemory(state, tasks)
	validation, err = s.commitLifecycle(own, lifecycleRelease, lifecycleLedger{
		state: &stateCandidate, task: &candidate, preview: preview, corpus: corpus, baseline: baseline,
		taskSHA256Before: digestBytes(before),
	})
	if err != nil {
		return ReleaseTaskResult{}, err
	}
	result.Validation = validation
	return result, nil
}

func validateReleaseReason(reason string) (string, error) {
	if !utf8.ValidString(reason) || len(reason) == 0 || len(reason) > 512 || strings.TrimSpace(reason) != reason || strings.IndexFunc(reason, unicode.IsControl) >= 0 {
		return "", WithMachineErrorCode(MachineCodeInvalidReason,
			errors.New("release reason must be trimmed UTF-8 of 1 through 512 bytes without control characters"))
	}
	if err := ensurePortableNote("reason", reason); err != nil {
		return "", WithMachineErrorCode(MachineCodeInvalidReason, err)
	}
	return reason, nil
}

func replaceReleaseTask(tasks []*Task, target, candidate *Task) []*Task {
	preview := make([]*Task, len(tasks))
	for i, task := range tasks {
		if task == target {
			preview[i] = candidate
		} else {
			preview[i] = task
		}
	}
	return preview
}
