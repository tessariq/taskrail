package taskrail

import (
	"path/filepath"
	"testing"
	"time"
)

// gapTask builds a task linked to anchor under the active spec, with the given
// dependencies and body (bodies carry the committed verification note the gap
// check reads for the missing-verification signal).
func gapTask(id, status, anchor string, deps []string, body string) *Task {
	return &Task{Frontmatter: TaskFrontmatter{
		ID:           id,
		Title:        id,
		Status:       status,
		Priority:     "high",
		SpecRef:      "specs/v0.1.0.md#" + anchor,
		Dependencies: deps,
	}, Body: body}
}

func signalKinds(signals []GapSignal, anchor string) []string {
	kinds := make([]string, 0)
	for _, s := range signals {
		if s.Anchor == anchor {
			kinds = append(kinds, s.Kind)
		}
	}
	return kinds
}

func hasSignal(signals []GapSignal, kind, anchor, taskID string) bool {
	for _, s := range signals {
		if s.Kind == kind && s.Anchor == anchor && s.TaskID == taskID {
			return true
		}
	}
	return false
}

func TestComputeGapsMissingVerification(t *testing.T) {
	areas := []parsedArea{
		{anchor: "verified", title: "Verified"},
		{anchor: "unverified", title: "Unverified"},
	}
	tasks := []*Task{
		gapTask("T-1", "completed", "verified", nil, "## Implementation Notes\n\n- 2026-01-01T00:00:00Z: verification pass\n"),
		gapTask("T-2", "completed", "unverified", nil, "no note here"),
	}
	signals := computeGaps(areas, "specs/v0.1.0.md", tasks)

	if hasSignal(signals, "missing-verification", "verified", "") {
		t.Fatalf("verified area must not raise missing-verification: %+v", signals)
	}
	if !hasSignal(signals, "missing-verification", "unverified", "") {
		t.Fatalf("implemented-but-unverified area must raise missing-verification: %+v", signals)
	}
}

func TestComputeGapsMissingVerificationOnlyImplementedAreas(t *testing.T) {
	// Covered but not fully completed (a todo remains) => not implemented, so
	// missing-verification must stay silent (it is a done-work signal).
	areas := []parsedArea{{anchor: "wip", title: "WIP"}}
	tasks := []*Task{gapTask("T-1", "todo", "wip", nil, "no note")}
	signals := computeGaps(areas, "specs/v0.1.0.md", tasks)
	if hasSignal(signals, "missing-verification", "wip", "") {
		t.Fatalf("in-progress area must not raise missing-verification: %+v", signals)
	}
}

func TestComputeGapsBlockedWithoutPendingBlocker(t *testing.T) {
	areas := []parsedArea{{anchor: "area", title: "Area"}}
	tests := []struct {
		name     string
		depOf    []string
		depState string
		want     bool
	}{
		{"blocked with no deps is anomalous", nil, "", true},
		{"blocked with completed dep is anomalous", []string{"T-9"}, "completed", true},
		{"blocked with pending dep is explained", []string{"T-9"}, "todo", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tasks := []*Task{gapTask("T-1", "blocked", "area", tc.depOf, "")}
			if tc.depOf != nil {
				tasks = append(tasks, gapTask("T-9", tc.depState, "area", nil, ""))
			}
			signals := computeGaps(areas, "specs/v0.1.0.md", tasks)
			got := hasSignal(signals, "dependency-anomaly", "area", "T-1")
			if got != tc.want {
				t.Fatalf("blocked anomaly for T-1 = %v, want %v: %+v", got, tc.want, signals)
			}
		})
	}
}

func TestComputeGapsIsolatedInCluster(t *testing.T) {
	areas := []parsedArea{{anchor: "area", title: "Area"}}
	// T-2 depends on T-1 forming a cluster; T-3 shares no intra-area edge.
	tasks := []*Task{
		gapTask("T-1", "todo", "area", nil, ""),
		gapTask("T-2", "todo", "area", []string{"T-1"}, ""),
		gapTask("T-3", "todo", "area", nil, ""),
	}
	signals := computeGaps(areas, "specs/v0.1.0.md", tasks)
	if !hasSignal(signals, "dependency-anomaly", "area", "T-3") {
		t.Fatalf("T-3 isolated from the area cluster must be flagged: %+v", signals)
	}
	if hasSignal(signals, "dependency-anomaly", "area", "T-1") || hasSignal(signals, "dependency-anomaly", "area", "T-2") {
		t.Fatalf("clustered tasks T-1/T-2 must not be flagged isolated: %+v", signals)
	}
}

func TestComputeGapsNoIsolationWithoutCluster(t *testing.T) {
	// No intra-area edges at all => no cluster exists, so independent tasks are
	// not "isolated" (that would flag every task and be pure noise).
	areas := []parsedArea{{anchor: "area", title: "Area"}}
	tasks := []*Task{
		gapTask("T-1", "todo", "area", nil, ""),
		gapTask("T-2", "todo", "area", nil, ""),
	}
	signals := computeGaps(areas, "specs/v0.1.0.md", tasks)
	for _, s := range signals {
		if s.Kind == "dependency-anomaly" {
			t.Fatalf("no cluster exists; isolation must not fire: %+v", signals)
		}
	}
}

func TestComputeGapsUnderDecomposed(t *testing.T) {
	tests := []struct {
		name  string
		reqs  int
		tasks int
		want  bool
	}{
		{"5 reqs 1 task far exceeds", 5, 1, true},
		{"2 reqs 1 task within norm", 2, 1, false},
		{"6 reqs 2 tasks far exceeds", 6, 2, true},
		{"5 reqs 2 tasks within factor", 5, 2, false},
		{"3 reqs 1 task below floor", 3, 1, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			areas := []parsedArea{{anchor: "area", title: "Area", requirements: tc.reqs}}
			tasks := make([]*Task, 0, tc.tasks)
			for i := 0; i < tc.tasks; i++ {
				tasks = append(tasks, gapTask("T-"+string(rune('1'+i)), "todo", "area", nil, ""))
			}
			signals := computeGaps(areas, "specs/v0.1.0.md", tasks)
			got := hasSignal(signals, "under-decomposed-area", "area", "")
			if got != tc.want {
				t.Fatalf("under-decomposed for %d reqs / %d tasks = %v, want %v", tc.reqs, tc.tasks, got, tc.want)
			}
		})
	}
}

func TestTaskVerificationRecordedAnchoredToNoteLine(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"canonical pass note", "## Implementation Notes\n\n- 2026-01-01T00:00:00Z: verification pass\n", true},
		{"canonical fail note", "- 2026-01-01T00:00:00Z: verification fail\n", true},
		{"prose mention is not a record", "This task adds a verification pass gate to CI.", false},
		{"no note", "nothing here", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := taskVerificationRecorded(tc.body); got != tc.want {
				t.Fatalf("taskVerificationRecorded(%q) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}

func TestParseSpecAreasRequirementsStopAtSubHeading(t *testing.T) {
	md := `# S

## Potential Features

### Parent

Requirements:

- one
- two

#### Child

Requirements:

- a
- b
- c
`
	areas, _ := parseSpecAreas(md)
	if len(areas) != 1 || areas[0].anchor != "parent" {
		t.Fatalf("expected a single parent area, got %+v", areas)
	}
	// Sub-area Requirements bullets must not inflate the parent's count — the
	// parent lists exactly two of its own.
	if areas[0].requirements != 2 {
		t.Fatalf("parent requirements = %d, want 2 (sub-area bullets must not roll up)", areas[0].requirements)
	}
}

func TestComputeGapsUncoveredAreaHasNoSignals(t *testing.T) {
	// Gap analysis only speaks about covered areas; an uncovered area is the
	// coverage command's business, not a structural companion gap.
	areas := []parsedArea{{anchor: "empty", title: "Empty", requirements: 9}}
	signals := computeGaps(areas, "specs/v0.1.0.md", nil)
	if len(signalKinds(signals, "empty")) != 0 {
		t.Fatalf("uncovered area must raise no gap signals: %+v", signals)
	}
}

const gapsSpecFixture = `# Gap Fixture

## Potential Features

### Verified Area

Requirements:

- one
- two

### Unverified Area

Requirements:

- one

### Under Decomposed

Requirements:

- a
- b
- c
- d
- e

Rationale: this trailing prose is not a requirement bullet.
`

func seedGapsRepo(t *testing.T) string {
	t.Helper()
	repo := seedFixtureRepo(t)
	writeFile(t, filepath.Join(repo, "specs", "v0.1.0.md"), gapsSpecFixture)
	// T-1 carries a committed verification note so its area is not flagged.
	writeFile(t, filepath.Join(repo, "planning", "tasks", "T-1.md"), `---
id: T-1
title: Cover verified
status: completed
priority: high
spec_ref: specs/v0.1.0.md#verified-area
dependencies: []
updated_at: "2026-03-31T00:00:00Z"
---

# T-1 Cover verified

## Implementation Notes

- 2026-01-01T00:00:00Z: verification pass
`)
	writeTask(t, repo, "T-2", "Cover unverified", "completed", "high", "specs/v0.1.0.md#unverified-area", nil)
	writeTask(t, repo, "T-3", "Cover underdecomposed", "todo", "high", "specs/v0.1.0.md#under-decomposed", nil)
	return repo
}

func TestServiceCoverageGaps(t *testing.T) {
	repo := seedGapsRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC))

	report, err := svc.CoverageGaps()
	if err != nil {
		t.Fatalf("CoverageGaps: %v", err)
	}
	if report.ActiveSpecPath != "specs/v0.1.0.md" {
		t.Fatalf("active spec path = %q", report.ActiveSpecPath)
	}
	if hasSignal(report.Signals, "missing-verification", "verified-area", "") {
		t.Fatalf("verified-area must not be flagged: %+v", report.Signals)
	}
	if !hasSignal(report.Signals, "missing-verification", "unverified-area", "") {
		t.Fatalf("unverified-area must be flagged missing-verification: %+v", report.Signals)
	}
	if !hasSignal(report.Signals, "under-decomposed-area", "under-decomposed", "") {
		t.Fatalf("under-decomposed must be flagged: %+v", report.Signals)
	}
}

func TestServiceCoverageGapsIsReadOnly(t *testing.T) {
	repo := seedGapsRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC))

	before := snapshotTree(t, filepath.Join(repo, "planning"))
	if _, err := svc.CoverageGaps(); err != nil {
		t.Fatalf("CoverageGaps: %v", err)
	}
	after := snapshotTree(t, filepath.Join(repo, "planning"))

	if len(before) != len(after) {
		t.Fatalf("CoverageGaps changed the planning file set: %d -> %d", len(before), len(after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Fatalf("CoverageGaps must not write %s", path)
		}
	}
}

func TestServiceCoverageGapsForAreaFilters(t *testing.T) {
	repo := seedGapsRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC))

	report, err := svc.CoverageGapsForArea("unverified-area")
	if err != nil {
		t.Fatalf("CoverageGapsForArea: %v", err)
	}
	for _, s := range report.Signals {
		if s.Anchor != "unverified-area" {
			t.Fatalf("area-scoped gaps must only cover unverified-area, got %+v", s)
		}
	}
	if !hasSignal(report.Signals, "missing-verification", "unverified-area", "") {
		t.Fatalf("expected the unverified-area signal, got %+v", report.Signals)
	}
}

func TestServiceCoverageGapsForAreaRejectsUnknownAnchor(t *testing.T) {
	repo := seedGapsRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC))
	if _, err := svc.CoverageGapsForArea("nope"); err == nil {
		t.Fatalf("unknown anchor must be rejected")
	}
}
