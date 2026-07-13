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

// reverseMapSpec exercises the reverse map: alpha covered by two tasks
// (double-covered), delta covered only through a #### child (roll-up), and
// epsilon with no covering task at all.
const reverseMapSpec = `# Fixture

## Potential Features

### Alpha

Double covered.

### Delta

#### Delta One

### Epsilon

Uncovered.
`

func TestCoverageReverseMapShowsCoveringTasks(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.1.0.md"), []byte(reverseMapSpec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	writeCoverageTaskFile(t, root, "T-1", "todo", "specs/v0.1.0.md#alpha")
	writeCoverageTaskFile(t, root, "T-2", "todo", "specs/v0.1.0.md#alpha")
	writeCoverageTaskFile(t, root, "T-3", "todo", "specs/v0.1.0.md#delta-one")

	out, err := runRoot(t, "coverage")
	if err != nil {
		t.Fatalf("coverage: %v (output %q)", err, out)
	}
	// The reverse map lists, per coverable area, the covering task id(s).
	for _, want := range []string{
		"coverage map:",
		"alpha: T-1, T-2 (double-covered)", // more than one task is distinguishable
		"delta: T-3",                       // roll-up: a #### child's task covers its parent
		"epsilon: (uncovered)",             // an area with no covering task is marked
	} {
		if !strings.Contains(out, want) {
			t.Errorf("coverage map missing %q: %q", want, out)
		}
	}

	// --json carries the same per-area covering-task list.
	jsonOut, err := runRoot(t, "coverage", "--json")
	if err != nil {
		t.Fatalf("coverage --json: %v (output %q)", err, jsonOut)
	}
	var report struct {
		Areas []struct {
			Anchor      string   `json:"anchor"`
			LinkedTasks []string `json:"linked_tasks"`
		} `json:"areas"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &report); err != nil {
		t.Fatalf("parse json: %v (output %q)", err, jsonOut)
	}
	byAnchor := map[string][]string{}
	for _, a := range report.Areas {
		byAnchor[a.Anchor] = a.LinkedTasks
	}
	if got := byAnchor["alpha"]; len(got) != 2 || got[0] != "T-1" || got[1] != "T-2" {
		t.Errorf("alpha linked_tasks = %v, want [T-1 T-2]", got)
	}
	if got := byAnchor["delta"]; len(got) != 1 || got[0] != "T-3" {
		t.Errorf("delta linked_tasks = %v, want [T-3] (roll-up)", got)
	}
	if got, ok := byAnchor["epsilon"]; !ok || len(got) != 0 {
		t.Errorf("epsilon linked_tasks = %v, want [] (uncovered)", got)
	}
}

// naSpec has no coverable areas, so coverage is N/A and --min has nothing to
// gate on.
const naSpec = `# Fixture

## Summary

Meta only, no coverable feature areas.
`

func TestCoverageMinGatesOnDecompositionThreshold(t *testing.T) {
	// The smoke spec yields 50% decomposition coverage (Alpha covered, Beta
	// uncovered, Gamma deferred). Table cases drive --min above, at, and below
	// that figure and assert the exit code while the report stays emitted.
	cases := []struct {
		name    string
		min     string
		asJSON  bool
		wantErr bool
	}{
		{name: "below threshold fails", min: "80", wantErr: true},
		{name: "at threshold passes", min: "50", wantErr: false},
		{name: "above actual passes", min: "40", wantErr: false},
		{name: "below threshold with json still fails", min: "80", asJSON: true, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := setupRepo(t)
			if err := os.WriteFile(filepath.Join(root, "specs", "v0.1.0.md"), []byte(coverageSmokeSpec), 0o644); err != nil {
				t.Fatalf("write spec: %v", err)
			}
			writeCoverageTaskFile(t, root, "T-1", "todo", "specs/v0.1.0.md#alpha")

			args := []string{"coverage", "--min", tc.min}
			if tc.asJSON {
				args = append(args, "--json")
			}
			out, err := runRoot(t, args...)
			if tc.wantErr && err == nil {
				t.Fatalf("expected non-zero exit for --min %s, got nil (output %q)", tc.min, out)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected exit zero for --min %s, got %v (output %q)", tc.min, err, out)
			}
			// The report is always emitted regardless of the exit code.
			marker := "coverage: 50% (1/2 areas)"
			if tc.asJSON {
				marker = `"coverage_percent": 50`
			}
			if !strings.Contains(out, marker) {
				t.Errorf("report not emitted under --min: %q", out)
			}
			// validate stays advisory: a coverage gap never makes validate fail.
			if vout, verr := runRoot(t, "validate"); verr != nil {
				t.Errorf("validate must still pass under a coverage gap: %v (output %q)", verr, vout)
			}
		})
	}
}

func TestCoverageMinGatesOnDisplayedFractionalFigure(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.1.0.md"), []byte(threeStateSpec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	// Cover 2 of 3 areas → decomposition 2/3, displayed as 66.7%.
	writeCoverageTaskFile(t, root, "T-1", "todo", "specs/v0.1.0.md#alpha")
	writeCoverageTaskFile(t, root, "T-2", "todo", "specs/v0.1.0.md#beta")

	// --min set to the figure the user reads (66.7) must pass: the gate compares
	// the reported percentage, not the hidden 66.666… float behind it.
	out, err := runRoot(t, "coverage", "--min", "66.7")
	if err != nil {
		t.Fatalf("--min at the displayed figure must pass: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "coverage: 66.7% (2/3 areas)") {
		t.Errorf("expected 66.7%% report: %q", out)
	}

	// A threshold above the displayed figure still fails.
	if _, err := runRoot(t, "coverage", "--min", "66.8"); err == nil {
		t.Errorf("--min above the displayed figure must fail")
	}
}

func TestCoverageMinRejectsOutOfRange(t *testing.T) {
	cases := []struct {
		name    string
		min     string
		wantErr bool
	}{
		{name: "negative rejected", min: "-5", wantErr: true},
		{name: "above 100 rejected", min: "150", wantErr: true},
		{name: "zero accepted", min: "0", wantErr: false},
		{name: "hundred accepted", min: "100", wantErr: false},
	}
	// rangeErr is the substring unique to a bounds-validation rejection, so an
	// in-range value that merely fails the gate is not mistaken for a rejection.
	const rangeErr = "between 0 and 100"
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := setupRepo(t)
			if err := os.WriteFile(filepath.Join(root, "specs", "v0.1.0.md"), []byte(coverageSmokeSpec), 0o644); err != nil {
				t.Fatalf("write spec: %v", err)
			}
			writeCoverageTaskFile(t, root, "T-1", "todo", "specs/v0.1.0.md#alpha")

			_, err := runRoot(t, "coverage", "--min", tc.min)
			rejected := err != nil && strings.Contains(err.Error(), rangeErr)
			if rejected != tc.wantErr {
				t.Fatalf("--min %s: rejected=%v, want %v (err %v)", tc.min, rejected, tc.wantErr, err)
			}
		})
	}
}

func TestCoverageMinExitsZeroWhenNotApplicable(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.1.0.md"), []byte(naSpec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	// An unscoreable spec (coverage N/A) has nothing to gate: --min exits zero.
	out, err := runRoot(t, "coverage", "--min", "100")
	if err != nil {
		t.Fatalf("--min on an N/A spec must exit zero: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "coverage: N/A") {
		t.Errorf("expected N/A report: %q", out)
	}
}

func TestCoverageAreaFiltersReportAndStaysReadOnly(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.1.0.md"), []byte(coverageSmokeSpec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	writeCoverageTaskFile(t, root, "T-1", "todo", "specs/v0.1.0.md#alpha")
	// A live task pointing at another spec is a spec-wide orphan the area view drops.
	writeCoverageTaskFile(t, root, "T-9", "todo", "specs/v0.2.0.md#retrofit")

	before := readAllFiles(t, root)

	// Covered area: filtered report scores 100% over a denominator of one.
	out, err := runRoot(t, "coverage", "--area", "alpha")
	if err != nil {
		t.Fatalf("coverage --area alpha: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "coverage: 100% (1/1 areas)") {
		t.Errorf("expected single-area 100%% line: %q", out)
	}
	if !strings.Contains(out, "alpha: T-1") {
		t.Errorf("expected alpha's reverse-map row: %q", out)
	}
	if strings.Contains(out, "beta") {
		t.Errorf("filtered report must not mention other areas: %q", out)
	}
	if strings.Contains(out, "orphans:") {
		t.Errorf("area view must drop spec-wide orphans: %q", out)
	}

	// Uncovered area: filtered report scores 0% and lists the gap.
	betaOut, err := runRoot(t, "coverage", "--area", "beta")
	if err != nil {
		t.Fatalf("coverage --area beta: %v (output %q)", err, betaOut)
	}
	if !strings.Contains(betaOut, "coverage: 0% (0/1 areas)") {
		t.Errorf("expected uncovered single-area 0%% line: %q", betaOut)
	}

	// --area composes with --json.
	jsonOut, err := runRoot(t, "coverage", "--area", "alpha", "--json")
	if err != nil {
		t.Fatalf("coverage --area --json: %v (output %q)", err, jsonOut)
	}
	var report struct {
		CoverableAreas int `json:"coverable_areas"`
		Areas          []struct {
			Anchor string `json:"anchor"`
		} `json:"areas"`
		Orphans []struct{} `json:"orphans"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &report); err != nil {
		t.Fatalf("parse json: %v (output %q)", err, jsonOut)
	}
	if report.CoverableAreas != 1 || len(report.Areas) != 1 || report.Areas[0].Anchor != "alpha" {
		t.Errorf("--json area view not narrowed to alpha: %+v", report)
	}
	if len(report.Orphans) != 0 {
		t.Errorf("--json area view must drop orphans, got %+v", report.Orphans)
	}

	// A deferred area is rejected, and the message names it as intentionally excluded.
	if _, err := runRoot(t, "coverage", "--area", "gamma"); err == nil {
		t.Error("deferred area gamma is not coverable and must be rejected")
	} else if !strings.Contains(err.Error(), "deferred or subsumed") {
		t.Errorf("deferred rejection must name its case, got %q", err.Error())
	}
	// An unknown anchor is rejected, and the message points at spec show --anchors.
	if _, err := runRoot(t, "coverage", "--area", "nope"); err == nil {
		t.Error("unknown anchor must be rejected")
	} else if !strings.Contains(err.Error(), "spec show --anchors") {
		t.Errorf("unknown rejection must point at spec show --anchors, got %q", err.Error())
	}

	after := readAllFiles(t, root)
	if len(before) != len(after) {
		t.Fatalf("coverage --area changed the file set")
	}
	for path, content := range before {
		if after[path] != content {
			t.Errorf("coverage --area mutated %s", path)
		}
	}
}

func TestCoverageAreaOnSpecWithNoCoverableAreas(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.1.0.md"), []byte(naSpec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	// A spec with no coverable areas has nothing to narrow to, so any --area
	// anchor is rejected rather than crashing or reporting N/A.
	if _, err := runRoot(t, "coverage", "--area", "alpha"); err == nil {
		t.Fatal("--area against a spec with no coverable areas must be rejected")
	}
}

func TestCoverageAreaComposesWithMin(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.1.0.md"), []byte(coverageSmokeSpec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	writeCoverageTaskFile(t, root, "T-1", "todo", "specs/v0.1.0.md#alpha")

	// A covered single area scores 100%, so --min 100 passes.
	if _, err := runRoot(t, "coverage", "--area", "alpha", "--min", "100"); err != nil {
		t.Errorf("covered area at 100%% must satisfy --min 100: %v", err)
	}
	// An uncovered single area scores 0%, so --min 50 gates non-zero.
	if _, err := runRoot(t, "coverage", "--area", "beta", "--min", "50"); err == nil {
		t.Error("uncovered area at 0%% must fail --min 50")
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
