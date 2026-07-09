package taskrail

import "strings"

// UnrecordedBlockerReason is the explicit placeholder status shows for a blocked
// task whose reason STATE.md does not retain. The transitions keep one blockers
// entry per blocked task, so this is now only reachable for legacy or
// hand-repaired state where a blocked task has no entry; status surfaces that
// honestly rather than emitting a silent empty string.
const UnrecordedBlockerReason = "(reason not recorded in STATE.md)"

// StatusCounts is the task distribution across the four operational buckets the
// snapshot reports. Cancelled tasks are deliberately excluded from every bucket.
type StatusCounts struct {
	Done    int `json:"done"`
	Active  int `json:"active"`
	Blocked int `json:"blocked"`
	Todo    int `json:"todo"`
}

// StatusNext is the next-task selection surfaced by status. Persisted is always
// false: unlike `next`, status computes the selection without writing it, so the
// field makes the read-only guarantee explicit to agents.
type StatusNext struct {
	TaskID    string `json:"task_id,omitempty"`
	Title     string `json:"title,omitempty"`
	Priority  string `json:"priority,omitempty"`
	Reason    string `json:"reason"`
	Persisted bool   `json:"persisted"`
}

// BlockedTask is a blocked task paired with its recorded reason.
type BlockedTask struct {
	TaskID string `json:"task_id"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
}

// StatusCoverage is the one-line coverage summary embedded in status. It carries
// the decomposition figure plus the orphan/drift counts drawn from the shared
// Coverage capability. DecompositionPercent is nil (rendered N/A) when the active
// spec has no coverable areas. The field is named specifically so a later
// implementation ("can we release") figure can be added without a breaking
// rename (T-080).
type StatusCoverage struct {
	DecompositionPercent *float64 `json:"decomposition_percent"`
	CoveredAreas         int      `json:"covered_areas"`
	CoverableAreas       int      `json:"coverable_areas"`
	OrphanTaskCount      int      `json:"orphan_task_count"`
	UncoveredAreaCount   int      `json:"uncovered_area_count"`
}

// StatusReport is the strictly read-only snapshot of current tracked-work state.
type StatusReport struct {
	ActiveSpecVersion      string         `json:"active_spec_version"`
	ActiveSpecPath         string         `json:"active_spec_path"`
	Counts                 StatusCounts   `json:"counts"`
	Next                   StatusNext     `json:"next"`
	Blocked                []BlockedTask  `json:"blocked"`
	LastVerificationResult string         `json:"last_verification_result"`
	Coverage               StatusCoverage `json:"coverage"`
}

// Status returns the current tracked-work snapshot. It is strictly read-only: it
// never writes STATE.md or task files, so the working tree stays clean.
func (s *Service) Status() (StatusReport, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return StatusReport{}, err
	}
	coverage, err := s.coverageFor(state, tasks)
	if err != nil {
		return StatusReport{}, err
	}

	next := computeNext(state, tasks)
	return StatusReport{
		ActiveSpecVersion: state.Frontmatter.ActiveSpecVersion,
		ActiveSpecPath:    state.Frontmatter.ActiveSpecPath,
		Counts:            countByStatus(tasks),
		Next: StatusNext{
			TaskID:    next.TaskID,
			Title:     next.Title,
			Priority:  next.Priority,
			Reason:    next.Reason,
			Persisted: false,
		},
		Blocked:                blockedTasks(tasks, state.Frontmatter.Blockers),
		LastVerificationResult: state.Frontmatter.LastVerificationResult,
		Coverage: StatusCoverage{
			DecompositionPercent: coverage.Percent,
			CoveredAreas:         coverage.CoveredAreas,
			CoverableAreas:       coverage.CoverableAreas,
			OrphanTaskCount:      coverage.Drift.AwayTaskCount,
			UncoveredAreaCount:   coverage.Drift.UncoveredAreaCount,
		},
	}, nil
}

func countByStatus(tasks []*Task) StatusCounts {
	var counts StatusCounts
	for _, task := range tasks {
		switch task.Frontmatter.Status {
		case "completed":
			counts.Done++
		case "in_progress":
			counts.Active++
		case "blocked":
			counts.Blocked++
		case "todo":
			counts.Todo++
		}
	}
	return counts
}

// blockedTasks pairs each blocked task with its recorded reason. Reasons are
// read from the same STATE.md blockers list `block` writes ("<id>: <reason>"),
// so status never re-derives them from task bodies.
func blockedTasks(tasks []*Task, stateBlockers []string) []BlockedTask {
	reasons := blockerReasons(stateBlockers)
	blocked := make([]BlockedTask, 0)
	for _, task := range tasks {
		if task.Frontmatter.Status != "blocked" {
			continue
		}
		reason, ok := reasons[task.Frontmatter.ID]
		if !ok {
			reason = UnrecordedBlockerReason
		}
		blocked = append(blocked, BlockedTask{
			TaskID: task.Frontmatter.ID,
			Title:  task.Frontmatter.Title,
			Reason: reason,
		})
	}
	return blocked
}

func blockerReasons(blockers []string) map[string]string {
	reasons := make(map[string]string, len(blockers))
	for _, entry := range blockers {
		id, reason, ok := strings.Cut(entry, ":")
		if !ok {
			continue
		}
		reasons[strings.TrimSpace(id)] = strings.TrimSpace(reason)
	}
	return reasons
}

// blockerID returns the task id an "<id>: <reason>" blockers entry belongs to.
func blockerID(entry string) string {
	id, _, _ := strings.Cut(entry, ":")
	return strings.TrimSpace(id)
}

// upsertBlocker records taskID's reason in the blockers list, replacing any
// existing entry for that task so the list holds exactly one entry per blocked
// task and never loses another task's reason.
func upsertBlocker(blockers []string, taskID, reason string) []string {
	return append(removeBlocker(blockers, taskID), taskID+": "+reason)
}

// removeBlocker returns blockers without taskID's entry, preserving order.
func removeBlocker(blockers []string, taskID string) []string {
	kept := make([]string, 0, len(blockers))
	for _, entry := range blockers {
		if blockerID(entry) == taskID {
			continue
		}
		kept = append(kept, entry)
	}
	return kept
}
