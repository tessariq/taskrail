package taskrail

import (
	"fmt"
	"strings"
)

// RepairInput drives the conservative state-repair surface. Repair defaults to a
// dry run; Apply must be set explicitly to write.
type RepairInput struct {
	Apply bool
}

// RepairChange records one proposed mechanical correction to STATE.md
// frontmatter, named so a reviewer can inspect the dry-run diff before applying.
type RepairChange struct {
	Field  string `json:"field"`
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"`
}

// RepairResult reports what repair proposed (dry run) or wrote (apply). BodyDiff
// carries the STATE.md body line changes when task counts or the focus section
// drift from the task files. Validation reflects the current state on a dry run
// and the post-apply state when Applied is true, so a reviewer always sees what
// remains unrepaired (e.g. non-mechanical violations repair deliberately skips).
type RepairResult struct {
	Applied    bool              `json:"applied"`
	Changes    []RepairChange    `json:"changes"`
	BodyDiff   []string          `json:"body_diff,omitempty"`
	Validation *ValidationResult `json:"validation,omitempty"`
}

// Repair conservatively heals mechanical STATE.md inconsistencies: a current_task
// pointer disagreeing with the task files, and a stale rendered body (task counts
// / focus). It only ever rewrites STATE.md — never a task file — so it structurally
// cannot advance a status or fabricate work (the Explicitly-Excluded guard). Any
// violation it cannot mechanically resolve (missing spec_ref, dependency cycles,
// multiple in_progress tasks) is left for the reviewer and surfaced through
// Validation rather than guessed at.
func (s *Service) Repair(input RepairInput) (RepairResult, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return RepairResult{}, err
	}

	corrected := state.Frontmatter
	changes := repairCurrentTask(&corrected, tasks)
	newBody := renderStateBody(corrected, tasks)
	bodyDiff := lineDiff(state.Body, newBody)

	// Apply only when there is something mechanical to fix, so a no-op repair never
	// dirties updated_at. All three paths (no-op, dry run, apply) close with the
	// same validate-and-report tail.
	applied := false
	if input.Apply && (len(changes) > 0 || len(bodyDiff) > 0) {
		corrected.UpdatedAt = timestamp(s.now())
		state.Frontmatter = corrected
		state.Body = newBody
		if err := s.saveState(state); err != nil {
			return RepairResult{}, err
		}
		applied = true
	}

	validation, err := s.Validate()
	if err != nil {
		return RepairResult{}, err
	}
	return RepairResult{Applied: applied, Changes: changes, BodyDiff: bodyDiff, Validation: &validation}, nil
}

// repairCurrentTask reconciles the current_task pointer with the task files,
// mutating fm in place and returning the corrections it made. The task files are
// the source of truth: repair follows them, never the reverse. It refuses to act
// when more than one task is in_progress, since picking one would regress the
// other's status — that ambiguity is left to validation.
func repairCurrentTask(fm *StateFrontmatter, tasks []*Task) []RepairChange {
	inProgress := inProgressTasks(tasks)
	if len(inProgress) > 1 {
		return nil
	}

	// The expected pointer: the single in_progress task, or empty when none is.
	wantID, wantTitle := "", ""
	reason := "no task is in_progress"
	if len(inProgress) == 1 {
		wantID = inProgress[0].Frontmatter.ID
		wantTitle = inProgress[0].Frontmatter.Title
		reason = "match the in_progress task file"
	}

	changes := make([]RepairChange, 0, 2)
	if fm.CurrentTask != wantID {
		changes = append(changes, RepairChange{Field: "current_task", From: fm.CurrentTask, To: wantID, Reason: reason})
		fm.CurrentTask = wantID
	}
	if fm.CurrentTaskTitle != wantTitle {
		changes = append(changes, RepairChange{Field: "current_task_title", From: fm.CurrentTaskTitle, To: wantTitle, Reason: reason})
		fm.CurrentTaskTitle = wantTitle
	}
	return changes
}

// lineDiff returns a compact, human-readable set of line changes between two
// rendered bodies: lines dropped from old are prefixed "- ", lines new in new are
// prefixed "+ ". It is a review hint, not a formal minimal diff; STATE.md body
// lines are effectively unique, so order-preserving set differences read clearly.
func lineDiff(oldText, newText string) []string {
	if oldText == newText {
		return nil
	}
	oldLines := nonBlankLines(oldText)
	newLines := nonBlankLines(newText)
	oldSet := make(map[string]struct{}, len(oldLines))
	for _, l := range oldLines {
		oldSet[l] = struct{}{}
	}
	newSet := make(map[string]struct{}, len(newLines))
	for _, l := range newLines {
		newSet[l] = struct{}{}
	}
	diff := make([]string, 0)
	for _, l := range oldLines {
		if _, ok := newSet[l]; !ok {
			diff = append(diff, fmt.Sprintf("- %s", l))
		}
	}
	for _, l := range newLines {
		if _, ok := oldSet[l]; !ok {
			diff = append(diff, fmt.Sprintf("+ %s", l))
		}
	}
	return diff
}

// nonBlankLines splits text into its non-empty lines, so the body diff ignores
// the blank spacer lines the STATE.md renderer inserts between sections.
func nonBlankLines(text string) []string {
	lines := make([]string, 0)
	for _, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines
}
