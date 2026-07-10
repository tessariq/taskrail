package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// statsJSON re-declares only the stats fields the acceptance criteria pin down.
type statsJSON struct {
	TotalTasks int `json:"total_tasks"`
	Statuses   []struct {
		Status  string  `json:"status"`
		Count   int     `json:"count"`
		Percent float64 `json:"percent"`
	} `json:"statuses"`
	BlockedRatio         float64 `json:"blocked_ratio"`
	RecordedBlockerCount int     `json:"recorded_blocker_count"`
	Coverage             struct {
		DecompositionPercent  *float64 `json:"decomposition_percent"`
		ImplementationPercent *float64 `json:"implementation_percent"`
		CoveredAreas          int      `json:"covered_areas"`
		ImplementedAreas      int      `json:"implemented_areas"`
		CoverableAreas        int      `json:"coverable_areas"`
		Areas                 []struct {
			Anchor      string `json:"anchor"`
			Covered     bool   `json:"covered"`
			Implemented bool   `json:"implemented"`
		} `json:"areas"`
	} `json:"coverage"`
	Dependencies struct {
		UnmetDependencyTaskCount int `json:"unmet_dependency_task_count"`
		LongestChain             int `json:"longest_chain"`
	} `json:"dependencies"`
}

// writeStatsTask drops a task with an explicit spec_ref and dependency list so
// stats smoke tests can seed a known dependency DAG.
func writeStatsTask(t *testing.T, root, id, status, specRef string, deps ...string) {
	t.Helper()
	depLine := "dependencies: []"
	if len(deps) > 0 {
		depLine = "dependencies: [" + strings.Join(deps, ", ") + "]"
	}
	content := strings.Join([]string{
		"---",
		"id: " + id,
		"title: Task " + id,
		"status: " + status,
		"priority: high",
		"spec_ref: " + specRef,
		depLine,
		`updated_at: "2026-07-09T00:00:00Z"`,
		"---",
		"",
		"# " + id,
		"",
		"Body.",
		"",
	}, "\n")
	path := filepath.Join(root, "planning", "tasks", id+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write task %s: %v", id, err)
	}
}

func statusCountPercent(t *testing.T, report statsJSON, status string) (int, float64) {
	t.Helper()
	for _, s := range report.Statuses {
		if s.Status == status {
			return s.Count, s.Percent
		}
	}
	t.Fatalf("status %q missing from stats: %+v", status, report.Statuses)
	return 0, 0
}

// seedStatsDAG writes the Alpha-covering chain T-4 -> T-3 -> T-2 -> T-1
// (T-1 completed), giving 4 total tasks, a longest chain of 4, and two tasks
// (T-3, T-4) waiting on non-completed dependencies.
func seedStatsDAG(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.1.0.md"), []byte(coverageSmokeSpec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	writeStatsTask(t, root, "T-1", "completed", "specs/v0.1.0.md#alpha")
	writeStatsTask(t, root, "T-2", "todo", "specs/v0.1.0.md#alpha", "T-1")
	writeStatsTask(t, root, "T-3", "todo", "specs/v0.1.0.md#alpha", "T-2")
	writeStatsTask(t, root, "T-4", "todo", "specs/v0.1.0.md#alpha", "T-3")
}

func TestStatsReportsMetricsAndStaysReadOnly(t *testing.T) {
	root := setupRepo(t)
	seedStatsDAG(t, root)

	before := readAllFiles(t, root)

	out, err := runRoot(t, "stats")
	if err != nil {
		t.Fatalf("stats: %v (output %q)", err, out)
	}
	for _, want := range []string{"tasks: 4 total", "longest chain 4", "unmet dependencies", "coverage: 50%"} {
		if !strings.Contains(out, want) {
			t.Errorf("human view missing %q: %q", want, out)
		}
	}

	after := readAllFiles(t, root)
	if len(before) != len(after) {
		t.Fatalf("stats changed the file set")
	}
	for path, content := range before {
		if after[path] != content {
			t.Errorf("stats mutated %s", path)
		}
	}
}

func TestStatsJSONMirrorsMetricsOnSeededDAG(t *testing.T) {
	root := setupRepo(t)
	seedStatsDAG(t, root)

	out, err := runRoot(t, "stats", "--json")
	if err != nil {
		t.Fatalf("stats --json: %v (output %q)", err, out)
	}
	var report statsJSON
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("parse json: %v (output %q)", err, out)
	}
	if report.TotalTasks != 4 {
		t.Errorf("total = %d, want 4", report.TotalTasks)
	}
	if count, pct := statusCountPercent(t, report, "todo"); count != 3 || pct != 75 {
		t.Errorf("todo = %d (%v%%), want 3 (75%%)", count, pct)
	}
	if count, pct := statusCountPercent(t, report, "completed"); count != 1 || pct != 25 {
		t.Errorf("completed = %d (%v%%), want 1 (25%%)", count, pct)
	}
	// T-3 and T-4 wait on non-completed deps; longest chain spans all 4 nodes.
	if report.Dependencies.UnmetDependencyTaskCount != 2 {
		t.Errorf("unmet dependency count = %d, want 2", report.Dependencies.UnmetDependencyTaskCount)
	}
	if report.Dependencies.LongestChain != 4 {
		t.Errorf("longest chain = %d, want 4", report.Dependencies.LongestChain)
	}
	if report.Coverage.DecompositionPercent == nil || *report.Coverage.DecompositionPercent != 50 {
		t.Errorf("decomposition percent = %v, want 50", report.Coverage.DecompositionPercent)
	}
	// Alpha is linked by a completed (T-1) plus three todo tasks, so it is
	// decomposed but not implemented → implementation 0/2 = 0%.
	if report.Coverage.ImplementationPercent == nil || *report.Coverage.ImplementationPercent != 0 {
		t.Errorf("implementation percent = %v, want 0", report.Coverage.ImplementationPercent)
	}
	if report.Coverage.ImplementedAreas != 0 {
		t.Errorf("implemented areas = %d, want 0", report.Coverage.ImplementedAreas)
	}
	if len(report.Coverage.Areas) == 0 {
		t.Errorf("expected a per-area coverage breakdown, got none")
	}
	for _, a := range report.Coverage.Areas {
		if a.Anchor == "alpha" && (!a.Covered || a.Implemented) {
			t.Errorf("alpha = covered %v implemented %v, want covered true implemented false", a.Covered, a.Implemented)
		}
	}
}

func TestStatsCoverageNAWhenNoCoverableAreas(t *testing.T) {
	setupRepo(t) // starter spec has only a Summary section → no coverable areas

	out, err := runRoot(t, "stats")
	if err != nil {
		t.Fatalf("stats: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "coverage: N/A") {
		t.Errorf("expected N/A coverage line: %q", out)
	}

	out, err = runRoot(t, "stats", "--json")
	if err != nil {
		t.Fatalf("stats --json: %v (output %q)", err, out)
	}
	var report statsJSON
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("parse json: %v (output %q)", err, out)
	}
	if report.Coverage.DecompositionPercent != nil {
		t.Errorf("decomposition_percent = %v, want nil", *report.Coverage.DecompositionPercent)
	}
}
