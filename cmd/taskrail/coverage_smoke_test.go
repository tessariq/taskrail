package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const coverageSmokeSpec = `# Fixture

## Summary

Meta.

## Potential Features

### Alpha

Covered.

### Beta

Uncovered gap.

### Gamma

> Deferred to v9.9

Deferred, not a gap.
`

// writeCoverageTaskFile drops a task whose spec_ref anchor matches a heading in
// the smoke spec, bypassing the fixed #summary ref of the shared writeTask.
func writeCoverageTaskFile(t *testing.T, root, id, status, specRef string) {
	t.Helper()
	content := strings.Join([]string{
		"---",
		"id: " + id,
		"title: Task " + id,
		"status: " + status,
		"priority: high",
		"spec_ref: " + specRef,
		"dependencies: []",
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

func TestCoverageReportsGapAndStaysReadOnly(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.1.0.md"), []byte(coverageSmokeSpec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	writeCoverageTaskFile(t, root, "T-1", "todo", "specs/v0.1.0.md#alpha")

	before := readAllFiles(t, root)

	out, err := runRoot(t, "coverage")
	if err != nil {
		t.Fatalf("coverage: %v (output %q)", err, out)
	}
	// Alpha covered, Beta uncovered, Gamma deferred → 1/2 = 50%.
	if !strings.Contains(out, "coverage: 50% (1/2 areas)") {
		t.Errorf("unexpected coverage line: %q", out)
	}
	// Alpha's only task is todo, so implementation is 0/2 and Alpha is
	// decomposed-not-implemented.
	if !strings.Contains(out, "implementation: 0% (0/2 areas)") {
		t.Errorf("expected implementation figure line: %q", out)
	}
	if !strings.Contains(out, "decomposed, not implemented:") || !strings.Contains(out, "alpha") {
		t.Errorf("expected alpha reported as decomposed-not-implemented: %q", out)
	}
	if !strings.Contains(out, "beta") {
		t.Errorf("expected beta reported as uncovered gap: %q", out)
	}
	if strings.Contains(out, "gamma") {
		t.Errorf("deferred area gamma must not be reported as a gap: %q", out)
	}

	after := readAllFiles(t, root)
	if len(before) != len(after) {
		t.Fatalf("coverage changed the file set")
	}
	for path, content := range before {
		if after[path] != content {
			t.Errorf("coverage mutated %s", path)
		}
	}
}

func TestCoverageJSONMirrorsReport(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.1.0.md"), []byte(coverageSmokeSpec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	writeCoverageTaskFile(t, root, "T-1", "todo", "specs/v0.1.0.md#alpha")
	writeCoverageTaskFile(t, root, "T-9", "todo", "specs/v0.2.0.md#retrofit")

	out, err := runRoot(t, "coverage", "--json")
	if err != nil {
		t.Fatalf("coverage --json: %v (output %q)", err, out)
	}

	var report struct {
		Percent               *float64 `json:"coverage_percent"`
		ImplementationPercent *float64 `json:"implementation_percent"`
		CoveredAreas          int      `json:"covered_areas"`
		ImplementedAreas      int      `json:"implemented_areas"`
		CoverableAreas        int      `json:"coverable_areas"`
		UncoveredAreas        []string `json:"uncovered_areas"`
		Areas                 []struct {
			Anchor      string   `json:"anchor"`
			Covered     bool     `json:"covered"`
			Implemented bool     `json:"implemented"`
			LinkedTasks []string `json:"linked_tasks"`
		} `json:"areas"`
		Orphans []struct {
			TaskID string `json:"task_id"`
		} `json:"orphans"`
		Drift struct {
			UncoveredAreaCount int `json:"uncovered_area_count"`
			AwayTaskCount      int `json:"away_task_count"`
		} `json:"drift"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("parse json: %v (output %q)", err, out)
	}
	// An uncovered area must emit linked_tasks: [] (not null), consistent with
	// the other report slices. json.Unmarshal maps both [] and null to a nil
	// slice, so assert on the raw payload.
	if !strings.Contains(out, `"linked_tasks": []`) {
		t.Errorf("expected an uncovered area to emit linked_tasks: [], got %q", out)
	}
	if strings.Contains(out, `"linked_tasks": null`) {
		t.Errorf("linked_tasks must never serialize as null, got %q", out)
	}
	if report.Percent == nil || *report.Percent != 50 {
		t.Errorf("percent = %v, want 50", report.Percent)
	}
	// Alpha's only task (T-1) is todo, so implementation is 0/2 = 0% over the
	// same denominator as the 50% decomposition figure.
	if report.ImplementationPercent == nil || *report.ImplementationPercent != 0 {
		t.Errorf("implementation percent = %v, want 0", report.ImplementationPercent)
	}
	if report.ImplementedAreas != 0 {
		t.Errorf("implemented areas = %d, want 0", report.ImplementedAreas)
	}
	if report.CoverableAreas != 2 || report.CoveredAreas != 1 {
		t.Errorf("areas = %d/%d, want 1/2", report.CoveredAreas, report.CoverableAreas)
	}
	for _, a := range report.Areas {
		if a.Anchor == "alpha" && (!a.Covered || a.Implemented) {
			t.Errorf("alpha = covered %v implemented %v, want covered true implemented false", a.Covered, a.Implemented)
		}
	}
	if len(report.Orphans) != 1 || report.Orphans[0].TaskID != "T-9" {
		t.Errorf("orphans = %+v, want [T-9]", report.Orphans)
	}
	if report.Drift.AwayTaskCount != 1 || report.Drift.UncoveredAreaCount != 1 {
		t.Errorf("drift = %+v, want 1 uncovered / 1 away", report.Drift)
	}
}

// threeStateSpec has three coverable areas so a single fixture can exercise all
// three per-area states at once: Alpha (fully implemented), Beta (mixed →
// decomposed-not-implemented), and Gamma (no task → uncovered).
const threeStateSpec = `# Fixture

## Potential Features

### Alpha

Fully implemented.

### Beta

Mixed completion.

### Gamma

Uncovered.
`

func TestCoverageThreeStatesAndReportOnly(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.1.0.md"), []byte(threeStateSpec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	writeCoverageTaskFile(t, root, "T-1", "completed", "specs/v0.1.0.md#alpha")
	writeCoverageTaskFile(t, root, "T-2", "completed", "specs/v0.1.0.md#beta")
	writeCoverageTaskFile(t, root, "T-3", "todo", "specs/v0.1.0.md#beta")

	// coverage always exits zero: a low implementation figure never gates.
	out, err := runRoot(t, "coverage")
	if err != nil {
		t.Fatalf("coverage must exit zero regardless of the implementation figure: %v (output %q)", err, out)
	}
	// Decomposition 2/3 (Alpha+Beta covered), implementation 1/3 (only Alpha).
	for _, want := range []string{"coverage: 66.7% (2/3 areas)", "implementation: 33.3% (1/3 areas)", "decomposed, not implemented:", "beta", "gamma"} {
		if !strings.Contains(out, want) {
			t.Errorf("coverage human view missing %q: %q", want, out)
		}
	}

	stats, err := runRoot(t, "stats")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	for _, want := range []string{"implemented alpha", "decomposed  beta", "uncovered   gamma"} {
		if !strings.Contains(stats, want) {
			t.Errorf("stats per-area breakdown missing %q: %q", want, stats)
		}
	}
}

func readAllFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		files[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
	return files
}
