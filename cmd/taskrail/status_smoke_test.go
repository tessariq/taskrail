package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/taskrail"
)

const statusSmokeSpec = `# Fixture

## Summary

Meta.

## Potential Features

### Alpha

Covered.

### Beta

Uncovered gap.
`

// statusJSON is the machine-readable shape asserted by the status smoke tests.
// It intentionally re-declares only the fields the acceptance criteria pin down.
type statusJSON struct {
	ActiveSpecVersion string `json:"active_spec_version"`
	ActiveSpecPath    string `json:"active_spec_path"`
	Counts            struct {
		Done    int `json:"done"`
		Active  int `json:"active"`
		Blocked int `json:"blocked"`
		Todo    int `json:"todo"`
	} `json:"counts"`
	Next struct {
		TaskID    string `json:"task_id"`
		Reason    string `json:"reason"`
		Persisted bool   `json:"persisted"`
	} `json:"next"`
	Blocked []struct {
		TaskID string `json:"task_id"`
		Reason string `json:"reason"`
	} `json:"blocked"`
	LastVerificationResult string `json:"last_verification_result"`
	Coverage               struct {
		DecompositionPercent  *float64 `json:"decomposition_percent"`
		ImplementationPercent *float64 `json:"implementation_percent"`
		CoveredAreas          int      `json:"covered_areas"`
		ImplementedAreas      int      `json:"implemented_areas"`
		CoverableAreas        int      `json:"coverable_areas"`
		OrphanTaskCount       int      `json:"orphan_task_count"`
		UncoveredAreaCount    int      `json:"uncovered_area_count"`
	} `json:"coverage"`
}

func TestStatusReportsSnapshotAndStaysReadOnly(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.1.0.md"), []byte(statusSmokeSpec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	// One todo covering Alpha (leaves Beta uncovered), one completed, and one
	// blocked-with-reason task driven through the real transitions.
	writeCoverageTaskFile(t, root, "T-100", "todo", "specs/v0.1.0.md#alpha")
	writeCoverageTaskFile(t, root, "T-091", "completed", "specs/v0.1.0.md#alpha")
	writeCoverageTaskFile(t, root, "T-102", "todo", "specs/v0.1.0.md#alpha")
	if _, err := runRoot(t, "start", "T-102"); err != nil {
		t.Fatalf("start T-102: %v", err)
	}
	if _, err := runRoot(t, "block", "T-102", "--reason", "waiting on upstream"); err != nil {
		t.Fatalf("block T-102: %v", err)
	}

	before := readAllFiles(t, root)

	out, err := runRoot(t, "status")
	if err != nil {
		t.Fatalf("status: %v (output %q)", err, out)
	}
	for _, want := range []string{"v0.1.0", "next: T-100", "not persisted", "T-102", "waiting on upstream", "50%", "implementation 0% (0/2 implemented)"} {
		if !strings.Contains(out, want) {
			t.Errorf("human view missing %q: %q", want, out)
		}
	}

	after := readAllFiles(t, root)
	if len(before) != len(after) {
		t.Fatalf("status changed the file set")
	}
	for path, content := range before {
		if after[path] != content {
			t.Errorf("status mutated %s", path)
		}
	}
}

func TestStatusJSONMirrorsHumanView(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.1.0.md"), []byte(statusSmokeSpec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	writeCoverageTaskFile(t, root, "T-100", "todo", "specs/v0.1.0.md#alpha")
	writeCoverageTaskFile(t, root, "T-091", "completed", "specs/v0.1.0.md#alpha")
	writeCoverageTaskFile(t, root, "T-102", "todo", "specs/v0.1.0.md#alpha")
	if _, err := runRoot(t, "start", "T-102"); err != nil {
		t.Fatalf("start T-102: %v", err)
	}
	if _, err := runRoot(t, "block", "T-102", "--reason", "waiting on upstream"); err != nil {
		t.Fatalf("block T-102: %v", err)
	}

	out, err := runRoot(t, "status", "--json")
	if err != nil {
		t.Fatalf("status --json: %v (output %q)", err, out)
	}

	var report statusJSON
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("parse json: %v (output %q)", err, out)
	}
	if report.ActiveSpecVersion != "v0.1.0" || report.ActiveSpecPath != "specs/v0.1.0.md" {
		t.Errorf("spec = %q/%q, want v0.1.0/specs/v0.1.0.md", report.ActiveSpecVersion, report.ActiveSpecPath)
	}
	if report.Counts.Done != 1 || report.Counts.Active != 0 || report.Counts.Blocked != 1 || report.Counts.Todo != 1 {
		t.Errorf("counts = %+v, want done1 active0 blocked1 todo1", report.Counts)
	}
	if report.Next.TaskID != "T-100" || report.Next.Persisted {
		t.Errorf("next = %+v, want T-100 not persisted", report.Next)
	}
	if len(report.Blocked) != 1 || report.Blocked[0].TaskID != "T-102" || report.Blocked[0].Reason != "waiting on upstream" {
		t.Errorf("blocked = %+v, want [T-102: waiting on upstream]", report.Blocked)
	}
	if report.Coverage.DecompositionPercent == nil || *report.Coverage.DecompositionPercent != 50 {
		t.Errorf("decomposition_percent = %v, want 50", report.Coverage.DecompositionPercent)
	}
	// Alpha has a completed and open tasks → decomposed-not-implemented; Beta
	// uncovered. Implementation is 0/2 over the same denominator.
	if report.Coverage.ImplementationPercent == nil || *report.Coverage.ImplementationPercent != 0 {
		t.Errorf("implementation_percent = %v, want 0", report.Coverage.ImplementationPercent)
	}
	if report.Coverage.ImplementedAreas != 0 {
		t.Errorf("implemented_areas = %d, want 0", report.Coverage.ImplementedAreas)
	}
	if report.Coverage.CoveredAreas != 1 || report.Coverage.CoverableAreas != 2 {
		t.Errorf("coverage areas = %d/%d, want 1/2", report.Coverage.CoveredAreas, report.Coverage.CoverableAreas)
	}
	if report.Coverage.UncoveredAreaCount != 1 || report.Coverage.OrphanTaskCount != 0 {
		t.Errorf("coverage drift = %d uncovered / %d orphan, want 1/0", report.Coverage.UncoveredAreaCount, report.Coverage.OrphanTaskCount)
	}
}

// TestStatusMultipleBlockedTasksShowAllReasons locks in that when more than one
// task is blocked, status reports each task's own recorded reason (T-083 makes
// STATE.md retain them all rather than only the most recent).
func TestStatusMultipleBlockedTasksShowAllReasons(t *testing.T) {
	root := setupRepo(t)
	writeCoverageTaskFile(t, root, "T-201", "todo", "specs/v0.1.0.md#summary")
	writeCoverageTaskFile(t, root, "T-202", "todo", "specs/v0.1.0.md#summary")
	for _, tc := range []struct{ id, reason string }{{"T-201", "first blocker"}, {"T-202", "second blocker"}} {
		if _, err := runRoot(t, "start", tc.id); err != nil {
			t.Fatalf("start %s: %v", tc.id, err)
		}
		if _, err := runRoot(t, "block", tc.id, "--reason", tc.reason); err != nil {
			t.Fatalf("block %s: %v", tc.id, err)
		}
	}

	out, err := runRoot(t, "status", "--json")
	if err != nil {
		t.Fatalf("status --json: %v (output %q)", err, out)
	}
	var report statusJSON
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("parse json: %v (output %q)", err, out)
	}
	if len(report.Blocked) != 2 {
		t.Fatalf("blocked = %+v, want 2 entries", report.Blocked)
	}
	if report.Blocked[0].TaskID != "T-201" || report.Blocked[0].Reason != "first blocker" {
		t.Errorf("T-201 blocked entry = %+v, want reason 'first blocker'", report.Blocked[0])
	}
	if report.Blocked[1].TaskID != "T-202" || report.Blocked[1].Reason != "second blocker" {
		t.Errorf("T-202 blocked entry = %+v, want reason 'second blocker'", report.Blocked[1])
	}

	human, err := runRoot(t, "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	for _, want := range []string{"T-201: first blocker", "T-202: second blocker"} {
		if !strings.Contains(human, want) {
			t.Errorf("human view missing %q: %q", want, human)
		}
	}
	if strings.Contains(human, taskrail.UnrecordedBlockerReason) {
		t.Errorf("no blocked task should be marked unrecorded: %q", human)
	}
}

func TestStatusCoverageNAWhenNoCoverableAreas(t *testing.T) {
	setupRepo(t) // starter spec has only a Summary section → no coverable areas

	out, err := runRoot(t, "status")
	if err != nil {
		t.Fatalf("status: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "coverage: N/A") {
		t.Errorf("expected N/A coverage line: %q", out)
	}
	if !strings.Contains(out, "next: none") {
		t.Errorf("expected no eligible next task: %q", out)
	}

	out, err = runRoot(t, "status", "--json")
	if err != nil {
		t.Fatalf("status --json: %v (output %q)", err, out)
	}
	var report statusJSON
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("parse json: %v (output %q)", err, out)
	}
	if report.Coverage.DecompositionPercent != nil {
		t.Errorf("decomposition_percent = %v, want nil", *report.Coverage.DecompositionPercent)
	}
	if report.Coverage.ImplementationPercent != nil {
		t.Errorf("implementation_percent = %v, want nil (N/A)", *report.Coverage.ImplementationPercent)
	}
}
