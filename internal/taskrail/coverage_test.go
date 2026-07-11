package taskrail

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// coverageSpecFixture exercises every branch of the coverable-area convention:
// a plain area (Alpha), a > Deferred area (Beta), a > Subsumed area (Gamma),
// and a grouped area covered only through a #### sub-area (Delta).
const coverageSpecFixture = `# Fixture

## Summary

Meta section, never coverable.

## Potential Features

### Alpha

Prose.

### Beta

> Deferred to v9.9

Deferred feature.

### Gamma

> Subsumed by Alpha

Subsumed feature.

### Delta

#### Delta One

#### Delta Two

## Explicitly Excluded

Meta section, never coverable.
`

func fixtureTask(id, status, specRef string) *Task {
	return &Task{Frontmatter: TaskFrontmatter{ID: id, Status: status, Priority: "high", SpecRef: specRef}}
}

func TestComputeCoverageConvention(t *testing.T) {
	tests := []struct {
		name          string
		tasks         []*Task
		wantPercent   *float64
		wantCovered   int
		wantCoverable int
		wantUncovered []string
		wantOrphanIDs []string
		wantAwayCount int
	}{
		{
			name: "full decomposition through direct link and sub-area roll-up",
			tasks: []*Task{
				fixtureTask("T-1", "todo", "specs/v0.3.0.md#alpha"),
				fixtureTask("T-2", "completed", "specs/v0.3.0.md#delta-two"),
			},
			wantPercent:   ptrFloat(100),
			wantCovered:   2,
			wantCoverable: 2,
			wantUncovered: nil,
		},
		{
			name: "partial coverage records the gap",
			tasks: []*Task{
				fixtureTask("T-1", "todo", "specs/v0.3.0.md#alpha"),
			},
			wantPercent:   ptrFloat(50),
			wantCovered:   1,
			wantCoverable: 2,
			wantUncovered: []string{"delta"},
		},
		{
			name: "subsumed area is excluded from the denominator purely from the marker",
			tasks: []*Task{
				fixtureTask("T-1", "todo", "specs/v0.3.0.md#alpha"),
				fixtureTask("T-2", "todo", "specs/v0.3.0.md#delta-one"),
			},
			// Gamma (subsumed) and Beta (deferred) never enter the denominator,
			// so 2/2 despite four ### headings under Potential Features.
			wantPercent:   ptrFloat(100),
			wantCovered:   2,
			wantCoverable: 2,
		},
		{
			name: "cancelled task does not cover its area",
			tasks: []*Task{
				fixtureTask("T-1", "cancelled", "specs/v0.3.0.md#alpha"),
				fixtureTask("T-2", "todo", "specs/v0.3.0.md#delta-two"),
			},
			wantPercent:   ptrFloat(50),
			wantCovered:   1,
			wantCoverable: 2,
			wantUncovered: []string{"alpha"},
		},
		{
			name: "open task pointing at a non-active spec is an orphan and drift",
			tasks: []*Task{
				fixtureTask("T-1", "todo", "specs/v0.3.0.md#alpha"),
				fixtureTask("T-2", "todo", "specs/v0.3.0.md#delta-two"),
				fixtureTask("T-9", "todo", "specs/v0.2.0.md#retrofit"),
			},
			wantPercent:   ptrFloat(100),
			wantCovered:   2,
			wantCoverable: 2,
			wantOrphanIDs: []string{"T-9"},
			wantAwayCount: 1,
		},
		{
			name: "completed task pointing at a prior spec is delivered history, not drift",
			tasks: []*Task{
				fixtureTask("T-1", "todo", "specs/v0.3.0.md#alpha"),
				fixtureTask("T-2", "todo", "specs/v0.3.0.md#delta-two"),
				// A finished v0.1.0 task legitimately points at its own spec.
				fixtureTask("T-hist", "completed", "specs/v0.1.0.md#summary"),
			},
			wantPercent:   ptrFloat(100),
			wantCovered:   2,
			wantCoverable: 2,
			wantOrphanIDs: nil,
			wantAwayCount: 0,
		},
		{
			name: "cancelled task pointing at a non-active spec is not an orphan",
			tasks: []*Task{
				fixtureTask("T-1", "todo", "specs/v0.3.0.md#alpha"),
				fixtureTask("T-2", "todo", "specs/v0.3.0.md#delta-two"),
				// An abandoned task pointing away is dead, not drifting.
				fixtureTask("T-x", "cancelled", "specs/v0.2.0.md#retrofit"),
			},
			wantPercent:   ptrFloat(100),
			wantCovered:   2,
			wantCoverable: 2,
			wantOrphanIDs: nil,
			wantAwayCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := computeCoverage(coverageSpecFixture, "specs/v0.3.0.md", tt.tasks)

			if !equalFloatPtr(report.Percent, tt.wantPercent) {
				t.Fatalf("percent = %v, want %v", fmtFloatPtr(report.Percent), fmtFloatPtr(tt.wantPercent))
			}
			if report.CoveredAreas != tt.wantCovered {
				t.Errorf("covered = %d, want %d", report.CoveredAreas, tt.wantCovered)
			}
			if report.CoverableAreas != tt.wantCoverable {
				t.Errorf("coverable = %d, want %d", report.CoverableAreas, tt.wantCoverable)
			}
			if !equalStrings(report.UncoveredAreas, tt.wantUncovered) {
				t.Errorf("uncovered = %v, want %v", report.UncoveredAreas, tt.wantUncovered)
			}
			gotOrphans := make([]string, 0, len(report.Orphans))
			for _, o := range report.Orphans {
				gotOrphans = append(gotOrphans, o.TaskID)
			}
			if !equalStrings(gotOrphans, tt.wantOrphanIDs) {
				t.Errorf("orphan ids = %v, want %v", gotOrphans, tt.wantOrphanIDs)
			}
			if report.Drift.AwayTaskCount != tt.wantAwayCount {
				t.Errorf("drift away count = %d, want %d", report.Drift.AwayTaskCount, tt.wantAwayCount)
			}
			if report.Drift.UncoveredAreaCount != len(report.UncoveredAreas) {
				t.Errorf("drift uncovered count = %d, want %d", report.Drift.UncoveredAreaCount, len(report.UncoveredAreas))
			}
			// The subsumed and deferred anchors must never surface as areas.
			for _, a := range report.Areas {
				if a.Anchor == "gamma" || a.Anchor == "beta" {
					t.Errorf("excluded area %q leaked into report", a.Anchor)
				}
			}
		})
	}
}

func TestComputeCoverageImplementationDimension(t *testing.T) {
	// Implementation coverage shares the decomposition denominator but only
	// counts an area once every non-cancelled linked task (roll-up respected) is
	// completed. It reads three per-area states: uncovered, decomposed (planned
	// but open work remains), and implemented.
	tests := []struct {
		name                 string
		tasks                []*Task
		wantDecomp           *float64
		wantImpl             *float64
		wantImplementedAreas int
		wantImplemented      map[string]bool
	}{
		{
			name: "area implemented only when all linked tasks completed (direct + roll-up)",
			tasks: []*Task{
				fixtureTask("T-1", "completed", "specs/v0.3.0.md#alpha"),
				fixtureTask("T-2", "completed", "specs/v0.3.0.md#delta-two"),
			},
			wantDecomp:           ptrFloat(100),
			wantImpl:             ptrFloat(100),
			wantImplementedAreas: 2,
			wantImplemented:      map[string]bool{"alpha": true, "delta": true},
		},
		{
			name: "partially-completed area reads decomposed-not-implemented",
			tasks: []*Task{
				fixtureTask("T-1", "completed", "specs/v0.3.0.md#alpha"),
				fixtureTask("T-2", "todo", "specs/v0.3.0.md#alpha"),
				fixtureTask("T-3", "completed", "specs/v0.3.0.md#delta-one"),
			},
			// alpha has an open task → decomposed but not implemented; delta fully done.
			wantDecomp:           ptrFloat(100),
			wantImpl:             ptrFloat(50),
			wantImplementedAreas: 1,
			wantImplemented:      map[string]bool{"alpha": false, "delta": true},
		},
		{
			name: "cancelled linked task is ignored for the implementation check",
			tasks: []*Task{
				fixtureTask("T-1", "completed", "specs/v0.3.0.md#alpha"),
				fixtureTask("T-2", "cancelled", "specs/v0.3.0.md#alpha"),
				fixtureTask("T-3", "completed", "specs/v0.3.0.md#delta-two"),
			},
			wantDecomp:           ptrFloat(100),
			wantImpl:             ptrFloat(100),
			wantImplementedAreas: 2,
			wantImplemented:      map[string]bool{"alpha": true, "delta": true},
		},
		{
			name: "uncovered area is neither decomposed nor implemented",
			tasks: []*Task{
				fixtureTask("T-1", "completed", "specs/v0.3.0.md#alpha"),
			},
			// delta has no linked task at all.
			wantDecomp:           ptrFloat(50),
			wantImpl:             ptrFloat(50),
			wantImplementedAreas: 1,
			wantImplemented:      map[string]bool{"alpha": true, "delta": false},
		},
		{
			name: "decomposed but zero completed reads 0% implementation",
			tasks: []*Task{
				fixtureTask("T-1", "todo", "specs/v0.3.0.md#alpha"),
				fixtureTask("T-2", "in_progress", "specs/v0.3.0.md#delta-two"),
			},
			wantDecomp:           ptrFloat(100),
			wantImpl:             ptrFloat(0),
			wantImplementedAreas: 0,
			wantImplemented:      map[string]bool{"alpha": false, "delta": false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := computeCoverage(coverageSpecFixture, "specs/v0.3.0.md", tt.tasks)

			if !equalFloatPtr(report.Percent, tt.wantDecomp) {
				t.Errorf("decomposition percent = %s, want %s", fmtFloatPtr(report.Percent), fmtFloatPtr(tt.wantDecomp))
			}
			if !equalFloatPtr(report.ImplementationPercent, tt.wantImpl) {
				t.Errorf("implementation percent = %s, want %s", fmtFloatPtr(report.ImplementationPercent), fmtFloatPtr(tt.wantImpl))
			}
			if report.ImplementedAreas != tt.wantImplementedAreas {
				t.Errorf("implemented areas = %d, want %d", report.ImplementedAreas, tt.wantImplementedAreas)
			}
			// Both figures share the coverable-area denominator.
			if report.CoverableAreas != 2 {
				t.Errorf("coverable areas = %d, want 2 (shared denominator)", report.CoverableAreas)
			}
			for _, area := range report.Areas {
				want, ok := tt.wantImplemented[area.Anchor]
				if !ok {
					continue
				}
				if area.Implemented != want {
					t.Errorf("area %q implemented = %v, want %v", area.Anchor, area.Implemented, want)
				}
				if area.Implemented && !area.Covered {
					t.Errorf("area %q implemented but not covered: implementation must imply decomposition", area.Anchor)
				}
			}
		})
	}
}

func TestComputeCoverageImplementationNAWhenNoCoverableAreas(t *testing.T) {
	// The N/A rule applies to both figures alike: an unscoreable spec never
	// reports a false 0%/100% implementation figure.
	const spec = "# Old Spec\n\n## Summary\n\nNothing coverable here.\n"
	report := computeCoverage(spec, "specs/v0.1.0.md", []*Task{
		fixtureTask("T-1", "completed", "specs/v0.1.0.md#summary"),
	})
	if report.Percent != nil {
		t.Errorf("decomposition percent = %v, want nil (N/A)", *report.Percent)
	}
	if report.ImplementationPercent != nil {
		t.Errorf("implementation percent = %v, want nil (N/A)", *report.ImplementationPercent)
	}
	if report.ImplementedAreas != 0 {
		t.Errorf("implemented areas = %d, want 0", report.ImplementedAreas)
	}
}

func TestValidatePassesWithLowImplementationCoverage(t *testing.T) {
	// Report-only guarantee: a low implementation figure (every area decomposed
	// but none implemented) must never make validate exit non-zero.
	repo := seedCoverageRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC))

	report, err := svc.Coverage()
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	if report.ImplementationPercent == nil || *report.ImplementationPercent != 0 {
		t.Fatalf("implementation percent = %s, want 0 (fixture task is todo)", fmtFloatPtr(report.ImplementationPercent))
	}

	result, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !result.Valid {
		t.Fatalf("validate must pass despite low implementation coverage: %v", result.Violations)
	}
}

func TestComputeCoverageIgnoresEmptyTitleHeading(t *testing.T) {
	// A heading marker with no title text must not become a phantom, forever
	// uncovered area (no spec_ref can resolve to an empty anchor).
	const spec = "# Fixture\n\n## Potential Features\n\n### Alpha\n\n### \n\nStray marker.\n"
	report := computeCoverage(spec, "specs/v0.3.0.md", []*Task{
		fixtureTask("T-1", "todo", "specs/v0.3.0.md#alpha"),
	})
	if report.CoverableAreas != 1 {
		t.Fatalf("coverable = %d, want 1 (empty-title heading ignored)", report.CoverableAreas)
	}
	if report.Percent == nil || *report.Percent != 100 {
		t.Errorf("percent = %v, want 100", report.Percent)
	}
}

func TestComputeCoverageUncoveredAreaLinkedTasksNotNil(t *testing.T) {
	// LinkedTasks must serialize as [] (not null) for an uncovered area, to
	// stay consistent with the report-level slices.
	report := computeCoverage(coverageSpecFixture, "specs/v0.3.0.md", nil)
	for _, area := range report.Areas {
		if area.LinkedTasks == nil {
			t.Errorf("area %q has nil LinkedTasks; want non-nil empty slice", area.Anchor)
		}
	}
}

func TestComputeCoverageNoCoverableAreas(t *testing.T) {
	// A spec without a Potential Features section has an empty denominator.
	const spec = "# Old Spec\n\n## Summary\n\nNothing coverable here.\n"
	report := computeCoverage(spec, "specs/v0.1.0.md", []*Task{
		fixtureTask("T-1", "todo", "specs/v0.1.0.md#summary"),
	})
	if report.Percent != nil {
		t.Fatalf("percent = %v, want nil (N/A)", *report.Percent)
	}
	if report.CoverableAreas != 0 {
		t.Errorf("coverable = %d, want 0", report.CoverableAreas)
	}
}

func TestServiceCoverageIsReadOnly(t *testing.T) {
	repo := seedCoverageRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC))

	before := snapshotTree(t, repo)
	if _, err := svc.Coverage(); err != nil {
		t.Fatalf("coverage: %v", err)
	}
	after := snapshotTree(t, repo)

	for path, content := range before {
		if after[path] != content {
			t.Errorf("coverage mutated %s", path)
		}
	}
	if len(before) != len(after) {
		t.Errorf("coverage changed the file set: before %d, after %d", len(before), len(after))
	}
}

func TestValidatePassesWithAdvisoryCoverageGap(t *testing.T) {
	repo := seedCoverageRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC))

	report, err := svc.Coverage()
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	if len(report.UncoveredAreas) == 0 {
		t.Fatal("fixture must have an uncovered area to prove validate still passes")
	}

	result, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !result.Valid {
		t.Fatalf("validate must pass despite advisory coverage gap: %v", result.Violations)
	}
}

// seedCoverageRepo builds a repo whose active spec has one covered and one
// uncovered coverable area, so coverage reports a real gap while validate stays
// clean (every task's spec_ref resolves to a live anchor).
func seedCoverageRepo(t *testing.T) string {
	t.Helper()
	repo := seedFixtureRepo(t)
	writeFile(t, filepath.Join(repo, "specs", "v0.1.0.md"), coverageSpecFixture)
	// One valid task covering Alpha; Delta stays an advisory gap.
	writeTask(t, repo, "T-1", "Cover alpha", "todo", "high", "specs/v0.1.0.md#alpha", nil)
	return repo
}

func ptrFloat(f float64) *float64 { return &f }

func equalFloatPtr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func fmtFloatPtr(p *float64) string {
	if p == nil {
		return "N/A"
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCoverageForAreaCoveredArea(t *testing.T) {
	repo := seedCoverageRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC))

	report, err := svc.CoverageForArea("alpha")
	if err != nil {
		t.Fatalf("CoverageForArea: %v", err)
	}
	if report.CoverableAreas != 1 || len(report.Areas) != 1 {
		t.Fatalf("expected a single-area report, got coverable=%d areas=%d", report.CoverableAreas, len(report.Areas))
	}
	if report.Areas[0].Anchor != "alpha" || !report.Areas[0].Covered {
		t.Fatalf("expected covered alpha, got %+v", report.Areas[0])
	}
	if report.CoveredAreas != 1 || !equalFloatPtr(report.Percent, ptrFloat(100)) {
		t.Fatalf("covered area must score 100%%: covered=%d percent=%s", report.CoveredAreas, fmtFloatPtr(report.Percent))
	}
	if len(report.UncoveredAreas) != 0 || report.Drift.UncoveredAreaCount != 0 {
		t.Fatalf("covered area must have no gap, got %v", report.UncoveredAreas)
	}
}

func TestCoverageForAreaUncoveredArea(t *testing.T) {
	repo := seedCoverageRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC))

	report, err := svc.CoverageForArea("delta")
	if err != nil {
		t.Fatalf("CoverageForArea: %v", err)
	}
	if report.CoverableAreas != 1 || report.CoveredAreas != 0 {
		t.Fatalf("uncovered area report shape wrong: coverable=%d covered=%d", report.CoverableAreas, report.CoveredAreas)
	}
	if !equalStrings(report.UncoveredAreas, []string{"delta"}) || report.Drift.UncoveredAreaCount != 1 {
		t.Fatalf("expected delta as the sole gap, got %v", report.UncoveredAreas)
	}
	if !equalFloatPtr(report.Percent, ptrFloat(0)) {
		t.Fatalf("uncovered area must score 0%%, got %s", fmtFloatPtr(report.Percent))
	}
}

func TestCoverageForAreaRejectsUnknownAnchor(t *testing.T) {
	repo := seedCoverageRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC))

	// Sub-area anchors roll up into their parent and are not coverable areas.
	for _, anchor := range []string{"nope", "summary", "delta-one"} {
		if _, err := svc.CoverageForArea(anchor); err == nil {
			t.Errorf("anchor %q should be rejected as non-coverable", anchor)
		}
	}
}

func TestCoverageForAreaDropsSpecWideOrphans(t *testing.T) {
	repo := seedCoverageRepo(t)
	// A live task pointing at a non-active spec is a spec-wide orphan; it belongs
	// to no area and must not surface in a single-area view.
	writeTask(t, repo, "T-9", "Away", "todo", "high", "specs/v0.2.0.md#retrofit", nil)
	svc := newTestService(t, repo, time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC))

	full, err := svc.Coverage()
	if err != nil {
		t.Fatalf("Coverage: %v", err)
	}
	if len(full.Orphans) == 0 {
		t.Fatal("fixture must have a spec-wide orphan to prove it is dropped")
	}

	report, err := svc.CoverageForArea("alpha")
	if err != nil {
		t.Fatalf("CoverageForArea: %v", err)
	}
	if len(report.Orphans) != 0 || report.Drift.AwayTaskCount != 0 {
		t.Fatalf("area view must drop spec-wide orphans, got %v", report.Orphans)
	}
}
