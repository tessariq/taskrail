package taskrail

import (
	"errors"
	"fmt"
	"strings"
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
func (s *Service) RepointTask(input RepointTaskInput) (RepointTaskResult, error) {
	taskID := strings.TrimSpace(input.TaskID)
	if taskID == "" {
		return RepointTaskResult{}, errors.New("task id is required")
	}
	area, specRef, err := repointSelector(input)
	if err != nil {
		return RepointTaskResult{}, err
	}

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return RepointTaskResult{}, err
	}
	target, ok := taskByID(tasks, taskID)
	if !ok {
		return RepointTaskResult{}, fmt.Errorf("task %s not found", taskID)
	}
	// Completed and cancelled tasks are delivered history, excluded from drift by
	// the coverage orphan rule; re-pointing them would rewrite the record of what
	// was actually delivered.
	if !isOpenStatus(target.Frontmatter.Status) {
		return RepointTaskResult{}, fmt.Errorf("task %s is %s: re-pointing targets open work, and terminal tasks are delivered history", taskID, target.Frontmatter.Status)
	}

	if area != "" {
		specRef, err = s.resolveAreaSpecRef(state, area)
		if err != nil {
			return RepointTaskResult{}, err
		}
	} else if specRef, err = s.validateSpecRef(specRef); err != nil {
		return RepointTaskResult{}, fmt.Errorf("invalid spec_ref: %w", err)
	}

	oldSpecRef := target.Frontmatter.SpecRef
	// A no-op re-point would still rewrite STATE.md's updated_at and dirty the
	// working tree for no change, so reject it as `rename` rejects a no-op re-slug.
	// specRef is canonical by here, so another spelling of an already-canonical
	// stored ref is caught as the no-op it is; a legacy stored ref still differs and
	// is rewritten, which is the sanctioned way to canonicalize one.
	if specRef == oldSpecRef {
		return RepointTaskResult{}, fmt.Errorf("task %s already points at %s", taskID, specRef)
	}

	// Both branches report the post-apply state: an apply re-validates what
	// actually landed on disk, while a dry run previews the same rules against
	// the pending edit held in memory. An operator previewing a fix for a broken
	// spec_ref is asking "would this make the repo valid?", so answering with the
	// validity of the state being replaced would invert the answer.
	var validation ValidationResult
	if input.DryRun {
		validation = s.validateInMemory(state, withSpecRef(tasks, target, specRef))
	} else {
		if err := s.applyRepoint(state, tasks, target, specRef); err != nil {
			return RepointTaskResult{}, err
		}
		validation, err = s.Validate()
		if err != nil {
			return RepointTaskResult{}, err
		}
	}
	return RepointTaskResult{
		TaskID:     taskID,
		OldSpecRef: oldSpecRef,
		NewSpecRef: specRef,
		Applied:    !input.DryRun,
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
		return "", "", errors.New("--area and --spec-ref are mutually exclusive")
	}
	if area == "" && specRef == "" {
		return "", "", errors.New("one of --area or --spec-ref is required")
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

// applyRepoint writes the single-field task edit, then re-projects STATE.md from
// the in-memory task set (target is a member, so the projection sees the new
// reference). The task file is written first so a failed state write leaves a
// real edit with a stale projection the next state-writing command heals, never
// a projection describing an edit that never landed.
func (s *Service) applyRepoint(state *State, tasks []*Task, target *Task, specRef string) error {
	now := timestamp(s.now())
	target.Frontmatter.SpecRef = specRef
	target.Frontmatter.UpdatedAt = now
	if err := s.saveTask(target); err != nil {
		return err
	}
	state.Frontmatter.UpdatedAt = now
	state.Body = renderStateBody(state.Frontmatter, tasks)
	return s.saveState(state)
}
