package taskrail

// statusOrder fixes the reporting order of the status distribution so stats
// renders and serializes deterministically. Unlike status counts, stats reports
// every status the task graph can hold, including cancelled, since it describes
// the distribution of the current files rather than the operational snapshot.
var statusOrder = []string{"todo", "in_progress", "blocked", "completed", "cancelled"}

// StatusStat is one status bucket's count and its share of all tasks. Percent is
// the count over the total task count, in [0, 100]; it is 0 when there are no
// tasks.
type StatusStat struct {
	Status  string  `json:"status"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

// StatsCoverage is both coverage figures plus the per-area breakdown, reused
// from the shared Coverage capability. DecompositionPercent ("is the work
// planned?") gates via `coverage --min`; ImplementationPercent ("can we
// release?") is report-only. Both are nil (rendered N/A) when the active spec
// has no coverable areas and share the CoverableAreas denominator; each area's
// Implemented flag distinguishes decomposed-not-implemented from implemented.
type StatsCoverage struct {
	DecompositionPercent  *float64       `json:"decomposition_percent"`
	ImplementationPercent *float64       `json:"implementation_percent"`
	CoveredAreas          int            `json:"covered_areas"`
	ImplementedAreas      int            `json:"implemented_areas"`
	CoverableAreas        int            `json:"coverable_areas"`
	Areas                 []CoverageArea `json:"areas"`
}

// DependencyShape describes the current dependency graph: how many tasks are
// waiting on unfinished dependencies, and the longest dependency chain.
type DependencyShape struct {
	// UnmetDependencyTaskCount counts non-cancelled tasks with at least one
	// dependency that is not completed (or does not exist).
	UnmetDependencyTaskCount int `json:"unmet_dependency_task_count"`
	// LongestChain is the number of tasks in the longest dependency path.
	LongestChain int `json:"longest_chain"`
}

// StatsReport is the strictly read-only aggregate view of current tracked-work
// state. It is a snapshot of the current task files and STATE.md, not a
// historical trend: Taskrail keeps no event log, so stats reports distribution,
// never throughput.
type StatsReport struct {
	TotalTasks             int             `json:"total_tasks"`
	Statuses               []StatusStat    `json:"statuses"`
	BlockedRatio           float64         `json:"blocked_ratio"`
	RecordedBlockerCount   int             `json:"recorded_blocker_count"`
	Coverage               StatsCoverage   `json:"coverage"`
	Dependencies           DependencyShape `json:"dependencies"`
	LastVerificationResult string          `json:"last_verification_result"`
}

// Stats returns the aggregate tracked-work metrics. It is strictly read-only: it
// never writes STATE.md or task files, so the working tree stays clean.
func (s *Service) Stats() (StatsReport, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return StatsReport{}, err
	}
	coverage, err := s.coverageFor(state, tasks)
	if err != nil {
		return StatsReport{}, err
	}
	return buildStats(state, tasks, coverage), nil
}

// buildStats assembles the report from an already-loaded state, task set, and
// coverage computation, so the IO-free metric logic is testable in isolation and
// reuses the same coverage figure `coverage` and `status` surface.
func buildStats(state *State, tasks []*Task, coverage CoverageReport) StatsReport {
	total := len(tasks)
	return StatsReport{
		TotalTasks:           total,
		Statuses:             statusStats(tasks, total),
		BlockedRatio:         ratio(countStatus(tasks, "blocked"), total),
		RecordedBlockerCount: recordedBlockerCount(tasks, state.Frontmatter.Blockers),
		Coverage: StatsCoverage{
			DecompositionPercent:  coverage.Percent,
			ImplementationPercent: coverage.ImplementationPercent,
			CoveredAreas:          coverage.CoveredAreas,
			ImplementedAreas:      coverage.ImplementedAreas,
			CoverableAreas:        coverage.CoverableAreas,
			Areas:                 coverage.Areas,
		},
		Dependencies: DependencyShape{
			UnmetDependencyTaskCount: unmetDependencyTaskCount(tasks),
			LongestChain:             longestDependencyChain(tasks),
		},
		LastVerificationResult: state.Frontmatter.LastVerificationResult,
	}
}

// statusStats builds the distribution in the fixed statusOrder so both the JSON
// and text views are stable.
func statusStats(tasks []*Task, total int) []StatusStat {
	stats := make([]StatusStat, 0, len(statusOrder))
	for _, status := range statusOrder {
		count := countStatus(tasks, status)
		stats = append(stats, StatusStat{Status: status, Count: count, Percent: ratio(count, total) * 100})
	}
	return stats
}

func countStatus(tasks []*Task, status string) int {
	count := 0
	for _, task := range tasks {
		if task.Frontmatter.Status == status {
			count++
		}
	}
	return count
}

// recordedBlockerCount counts blocked tasks whose reason STATE.md still retains,
// distinguishing them from blocked tasks whose reason was never recorded.
func recordedBlockerCount(tasks []*Task, stateBlockers []string) int {
	reasons := blockerReasons(stateBlockers)
	count := 0
	for _, task := range tasks {
		if task.Frontmatter.Status != "blocked" {
			continue
		}
		if _, ok := reasons[task.Frontmatter.ID]; ok {
			count++
		}
	}
	return count
}

// unmetDependencyTaskCount counts non-cancelled tasks that are waiting on at
// least one dependency that is not completed. Cancelled tasks are excluded: a
// cancelled task's unmet dependency is not live work.
func unmetDependencyTaskCount(tasks []*Task) int {
	count := 0
	for _, task := range tasks {
		if task.Frontmatter.Status == "cancelled" {
			continue
		}
		if !dependenciesResolved(task, tasks) {
			count++
		}
	}
	return count
}

// longestDependencyChain returns the number of tasks in the longest path through
// the dependency graph (a standalone task is a chain of 1). Cancelled tasks are
// excluded from the graph entirely, consistent with unmetDependencyTaskCount:
// cancelled work is not part of the live dependency shape. It memoizes per task
// and guards against cycles so a malformed graph terminates; validate rejects
// cycles, so the guard is only a safety net.
func longestDependencyChain(tasks []*Task) int {
	byID := make(map[string]*Task, len(tasks))
	for _, task := range tasks {
		byID[task.Frontmatter.ID] = task
	}
	memo := make(map[string]int, len(tasks))
	onPath := make(map[string]bool, len(tasks))

	var depth func(id string) int
	depth = func(id string) int {
		if d, ok := memo[id]; ok {
			return d
		}
		task, ok := byID[id]
		if !ok || onPath[id] || task.Frontmatter.Status == "cancelled" {
			// Missing dependency (a validate concern), a cycle, or a cancelled
			// task outside the live graph: contribute no further depth.
			return 0
		}
		onPath[id] = true
		best := 0
		for _, dep := range task.Frontmatter.Dependencies {
			if d := depth(dep); d > best {
				best = d
			}
		}
		onPath[id] = false
		memo[id] = best + 1
		return memo[id]
	}

	longest := 0
	for _, task := range tasks {
		if d := depth(task.Frontmatter.ID); d > longest {
			longest = d
		}
	}
	return longest
}

// ratio returns numerator/denominator as a fraction, or 0 when denominator is 0.
func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
