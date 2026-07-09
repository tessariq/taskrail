package taskrail

import "testing"

// depTask builds a task with an explicit dependency list, extending the
// dependency-free fixtureTask used by the coverage tests.
func depTask(id, status string, deps ...string) *Task {
	return &Task{Frontmatter: TaskFrontmatter{ID: id, Title: "Task " + id, Status: status, Priority: "high", Dependencies: deps}}
}

func TestBuildStatsStatusDistributionAndPercentages(t *testing.T) {
	tasks := []*Task{
		fixtureTask("T-1", "todo", ""),
		fixtureTask("T-2", "todo", ""),
		fixtureTask("T-3", "in_progress", ""),
		fixtureTask("T-4", "completed", ""),
		fixtureTask("T-5", "cancelled", ""),
	}
	report := buildStats(&State{}, tasks, CoverageReport{})

	if report.TotalTasks != 5 {
		t.Fatalf("total = %d, want 5", report.TotalTasks)
	}
	want := map[string]struct {
		count   int
		percent float64
	}{
		"todo":        {2, 40},
		"in_progress": {1, 20},
		"blocked":     {0, 0},
		"completed":   {1, 20},
		"cancelled":   {1, 20},
	}
	if len(report.Statuses) != len(want) {
		t.Fatalf("statuses = %d entries, want %d", len(report.Statuses), len(want))
	}
	for _, s := range report.Statuses {
		w, ok := want[s.Status]
		if !ok {
			t.Errorf("unexpected status bucket %q", s.Status)
			continue
		}
		if s.Count != w.count || s.Percent != w.percent {
			t.Errorf("%s = %d (%v%%), want %d (%v%%)", s.Status, s.Count, s.Percent, w.count, w.percent)
		}
	}
}

func TestBuildStatsBlockedRatioAndRecordedBlockers(t *testing.T) {
	tasks := []*Task{
		fixtureTask("T-1", "blocked", ""),
		fixtureTask("T-2", "blocked", ""),
		fixtureTask("T-3", "todo", ""),
		fixtureTask("T-4", "completed", ""),
	}
	// Only T-1 has a recorded blocker reason; T-2 is blocked with none retained.
	state := &State{Frontmatter: StateFrontmatter{Blockers: []string{"T-1: waiting on upstream"}}}
	report := buildStats(state, tasks, CoverageReport{})

	if report.BlockedRatio != 0.5 {
		t.Errorf("blocked ratio = %v, want 0.5", report.BlockedRatio)
	}
	if report.RecordedBlockerCount != 1 {
		t.Errorf("recorded blocker count = %d, want 1", report.RecordedBlockerCount)
	}
}

func TestBuildStatsDependencyShapeOnSeededDAG(t *testing.T) {
	// Chain T-4 -> T-3 -> T-2 -> T-1 (4 nodes) is the longest path. T-1 is
	// completed, so T-2's dependency is met but T-3/T-4 have unmet dependencies.
	// T-5 stands alone; a cancelled T-6 dangles off the graph.
	tasks := []*Task{
		depTask("T-1", "completed"),
		depTask("T-2", "todo", "T-1"),
		depTask("T-3", "todo", "T-2"),
		depTask("T-4", "todo", "T-3"),
		depTask("T-5", "todo"),
		depTask("T-6", "cancelled", "T-4"),
	}
	report := buildStats(&State{}, tasks, CoverageReport{})

	if report.Dependencies.LongestChain != 4 {
		t.Errorf("longest chain = %d, want 4", report.Dependencies.LongestChain)
	}
	// T-3 and T-4 have unmet (non-completed) dependencies; T-2's is met, T-5 has
	// none, and cancelled T-6 is excluded even though its dependency is unmet.
	if report.Dependencies.UnmetDependencyTaskCount != 2 {
		t.Errorf("unmet dependency task count = %d, want 2", report.Dependencies.UnmetDependencyTaskCount)
	}
}

func TestBuildStatsDependencyShapeDiamondAndCancelledDep(t *testing.T) {
	// Diamond: T-4 depends on both T-2 and T-3, which both depend on base T-1.
	// The longest path is 3 nodes (T-4 -> T-2/T-3 -> T-1); max-over-deps, not sum.
	diamond := []*Task{
		depTask("T-1", "todo"),
		depTask("T-2", "todo", "T-1"),
		depTask("T-3", "todo", "T-1"),
		depTask("T-4", "todo", "T-2", "T-3"),
	}
	report := buildStats(&State{}, diamond, CoverageReport{})
	if report.Dependencies.LongestChain != 3 {
		t.Errorf("diamond longest chain = %d, want 3", report.Dependencies.LongestChain)
	}

	// A non-cancelled task depending on a cancelled task: the dependency is not
	// completed, so the dependent is unmet; the cancelled predecessor is excluded
	// from the graph, so the chain is just the dependent (1 node).
	onCancelled := []*Task{
		depTask("T-1", "cancelled"),
		depTask("T-2", "todo", "T-1"),
	}
	report = buildStats(&State{}, onCancelled, CoverageReport{})
	if report.Dependencies.UnmetDependencyTaskCount != 1 {
		t.Errorf("dep-on-cancelled unmet count = %d, want 1", report.Dependencies.UnmetDependencyTaskCount)
	}
	if report.Dependencies.LongestChain != 1 {
		t.Errorf("dep-on-cancelled longest chain = %d, want 1", report.Dependencies.LongestChain)
	}
}

func TestBuildStatsToleratesEmptyGraphAndDependencyCycle(t *testing.T) {
	// Empty graph must not divide by zero.
	empty := buildStats(&State{}, nil, CoverageReport{})
	if empty.TotalTasks != 0 || empty.BlockedRatio != 0 || empty.Dependencies.LongestChain != 0 {
		t.Errorf("empty graph = %+v, want zeroed metrics", empty)
	}

	// A dependency cycle (validate rejects these, but stats must terminate).
	cyclic := []*Task{
		depTask("T-1", "todo", "T-2"),
		depTask("T-2", "todo", "T-1"),
	}
	report := buildStats(&State{}, cyclic, CoverageReport{})
	if report.Dependencies.LongestChain < 1 {
		t.Errorf("cyclic longest chain = %d, want >= 1 (must terminate)", report.Dependencies.LongestChain)
	}
}

func TestBuildStatsCarriesCoverageAndVerification(t *testing.T) {
	pct := 75.0
	coverage := CoverageReport{
		Percent:        &pct,
		CoveredAreas:   3,
		CoverableAreas: 4,
		Areas:          []CoverageArea{{Anchor: "alpha", Title: "Alpha", Covered: true, LinkedTasks: []string{"T-1"}}},
	}
	state := &State{Frontmatter: StateFrontmatter{LastVerificationResult: "pass: shipped"}}
	report := buildStats(state, nil, coverage)

	if report.Coverage.DecompositionPercent == nil || *report.Coverage.DecompositionPercent != 75 {
		t.Errorf("decomposition percent = %v, want 75", report.Coverage.DecompositionPercent)
	}
	if report.Coverage.CoveredAreas != 3 || report.Coverage.CoverableAreas != 4 {
		t.Errorf("coverage areas = %d/%d, want 3/4", report.Coverage.CoveredAreas, report.Coverage.CoverableAreas)
	}
	if len(report.Coverage.Areas) != 1 || report.Coverage.Areas[0].Anchor != "alpha" {
		t.Errorf("coverage areas breakdown = %+v, want [alpha]", report.Coverage.Areas)
	}
	if report.LastVerificationResult != "pass: shipped" {
		t.Errorf("last verification = %q, want 'pass: shipped'", report.LastVerificationResult)
	}
}
