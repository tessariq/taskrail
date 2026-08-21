package taskrail

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/tessariq/taskrail/internal/repotx"
)

// RepointTaskInput drives a spec_ref re-point of one open task. Exactly one of
// Area (the active-spec anchor shorthand) or SpecRef (an explicit path#anchor,
// for the cross-spec case) selects the new reference. DryRun reports the planned
// change without writing.
type RepointTaskInput struct {
	TaskID  string
	Area    string
	SpecRef string
	DryRun  bool
}

// RepointTaskResult reports the reference change the command planned (dry run)
// or applied. Validation reflects the current state on a dry run and the
// post-apply state otherwise, so a reviewer always sees the resulting validity.
type RepointTaskResult struct {
	TaskID     string            `json:"task_id"`
	OldSpecRef string            `json:"old_spec_ref"`
	NewSpecRef string            `json:"new_spec_ref"`
	Applied    bool              `json:"applied"`
	Validation *ValidationResult `json:"validation,omitempty"`
}

// RepointTask rewrites one open task's spec_ref onto a new area, then re-projects
// STATE.md and re-runs validation. It closes the drift-recovery loop `spec
// activate` opens: moving an open task onto the active spec without hand-editing
// frontmatter. It re-encodes one reference field only — never the id, slug,
// filename, title, status, or dependencies, and never another task file — and is
// neither a status mutator nor a bulk migrator. An unresolvable target fails
// before any write.
func (s *Service) RepointTask(input RepointTaskInput) (result RepointTaskResult, err error) {
	taskID := input.TaskID
	if taskID == "" {
		return RepointTaskResult{}, WithMachineErrorCode(MachineCodeInvalidArguments,
			errors.New("task id is required"))
	}
	area, specRef, err := repointSelector(input)
	if err != nil {
		return RepointTaskResult{}, err
	}
	var own repotx.Ownership
	var release func() error
	if !input.DryRun {
		own, release, err = s.beginTaskWriterWrite(taskRepointWriter)
		if err != nil {
			return RepointTaskResult{}, err
		}
		defer func() {
			if releaseErr := release(); releaseErr != nil && err == nil {
				err = releaseErr
			}
		}()
		if testHookTaskWriterLocked != nil {
			testHookTaskWriterLocked()
		}
	}

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return RepointTaskResult{}, err
	}
	target, err := exactTaskByID(tasks, taskID)
	if err != nil {
		return RepointTaskResult{}, err
	}
	// Completed and cancelled tasks are delivered history, excluded from drift by
	// the coverage orphan rule; re-pointing them would rewrite the record of what
	// was actually delivered.
	if !isOpenStatus(target.Frontmatter.Status) {
		return RepointTaskResult{}, WithMachineErrorCode(MachineCodeInvalidStatus,
			fmt.Errorf("task %s is %s: re-pointing targets open work, and terminal tasks are delivered history", taskID, target.Frontmatter.Status))
	}

	if area != "" {
		specRef, err = s.resolveAreaSpecRef(state, area)
		if err != nil {
			return RepointTaskResult{}, err
		}
	} else if specRef, err = s.validateSpecRef(specRef); err != nil {
		return RepointTaskResult{}, invalidArgumentsf("invalid spec_ref: %w", err)
	}

	oldSpecRef := target.Frontmatter.SpecRef
	// A no-op re-point would still rewrite STATE.md's updated_at and dirty the
	// working tree for no change, so reject it as `rename` rejects a no-op re-slug.
	// specRef is canonical by here, so another spelling of an already-canonical
	// stored ref is caught as the no-op it is; a legacy stored ref still differs and
	// is rewritten, which is the sanctioned way to canonicalize one.
	if specRef == oldSpecRef {
		return RepointTaskResult{}, invalidArgumentsf("task %s already points at %s", taskID, specRef)
	}

	// Both branches report the post-apply state: an apply validates the exact
	// candidate it publishes, while a dry run previews the same rules against
	// the pending edit held in memory. The dry run is read-only and takes no
	// mutation lock. An operator previewing a fix for a broken spec_ref is
	// asking "would this make the repo valid?", so answering with the validity
	// of the state being replaced would invert the answer.
	previewTasks := withSpecRef(tasks, target, specRef)
	if input.DryRun {
		validation := s.validateInMemory(state, previewTasks)
		return RepointTaskResult{
			TaskID:     taskID,
			OldSpecRef: oldSpecRef,
			NewSpecRef: specRef,
			Applied:    false,
			Validation: &validation,
		}, nil
	}

	// The corpus and baseline are observed under the lock: the transaction's
	// recheck then refuses any candidate built from reads that predate it.
	corpus, err := snapshotTaskCorpus(tasks)
	if err != nil {
		return RepointTaskResult{}, err
	}
	baseline := s.validateInMemory(state, tasks)

	// One timestamp for the task file and the state projection, so a tick
	// between two reads can never leave them disagreeing.
	now := timestamp(s.now())
	taskBytes, err := repointTaskBytes(target, specRef, now)
	if err != nil {
		return RepointTaskResult{}, err
	}
	stateCandidate := *state
	stateCandidate.Frontmatter.UpdatedAt = now
	stateCandidate.Body = renderStateBody(stateCandidate.Frontmatter, previewTasks)
	validation, err := s.commitTaskWriter(own, taskRepointWriter, taskWriterLedger{
		state:     &stateCandidate,
		preview:   previewTasks,
		published: []repotx.Candidate{managedCandidate(target.Path, target.Filename, taskBytes)},
		selected:  taskID,
		tasks:     tasks,
		written:   []*Task{target},
		corpus:    corpus,
		baseline:  baseline,
	})
	if err != nil {
		return RepointTaskResult{}, err
	}
	return RepointTaskResult{
		TaskID:     taskID,
		OldSpecRef: oldSpecRef,
		NewSpecRef: specRef,
		Applied:    true,
		Validation: &validation,
	}, nil
}

// repointSelector resolves the new reference from exactly one selector. A task
// has exactly one resolved spec reference, so --area (the active-spec shorthand,
// mirroring `task new`) and --spec-ref cannot both be given, and a re-point with
// neither has no target. Being pure, it rejects before any load or write.
func repointSelector(input RepointTaskInput) (area, specRef string, err error) {
	area = strings.TrimSpace(input.Area)
	specRef = strings.TrimSpace(input.SpecRef)
	if area != "" && specRef != "" {
		return "", "", WithMachineErrorCode(MachineCodeInvalidArguments,
			errors.New("--area and --spec-ref are mutually exclusive"))
	}
	if area == "" && specRef == "" {
		return "", "", WithMachineErrorCode(MachineCodeInvalidArguments,
			errors.New("one of --area or --spec-ref is required"))
	}
	return area, specRef, nil
}

// withSpecRef returns the task set with target replaced by a copy carrying
// specRef, leaving the loaded tasks untouched. The dry run must not mutate what
// it only previews: the caller still reports the *current* spec_ref as the old
// value, and a shared pointer edit would make that read back as the new one.
func withSpecRef(tasks []*Task, target *Task, specRef string) []*Task {
	preview := make([]*Task, len(tasks))
	for i, task := range tasks {
		if task != target {
			preview[i] = task
			continue
		}
		edited := *task
		edited.Frontmatter.SpecRef = specRef
		preview[i] = &edited
	}
	return preview
}

// repointTaskBytes patches the selected task's spec_ref and updated_at lines in
// place, so every other byte — including fields no Taskrail struct models —
// survives the re-point exactly.
func repointTaskBytes(target *Task, specRef, now string) ([]byte, error) {
	data, err := os.ReadFile(target.Filename)
	if err != nil {
		return nil, fmt.Errorf("read task %s: %w", target.Path, fsCause(err))
	}
	frontmatter, body, newline, err := splitTaskDocument(data, target.Frontmatter.ID)
	if err != nil {
		return nil, err
	}
	if frontmatter, err = replaceTaskField(frontmatter, newline, target.Frontmatter.ID, "spec_ref", specRef); err != nil {
		return nil, err
	}
	frontmatter, err = replaceTaskField(frontmatter, newline, target.Frontmatter.ID, "updated_at", strconv.Quote(now))
	if err != nil {
		return nil, err
	}
	return []byte(frontmatter + body), nil
}
