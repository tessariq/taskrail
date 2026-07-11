package taskrail

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CoverageArea is one coverable feature area (a `###` heading under
// `## Potential Features`) and whether a non-cancelled task links to it, either
// directly or through the roll-up of its `####` sub-areas. Covered is the
// decomposition state (>=1 linked task); Implemented is the stricter release
// state (every linked task completed), so the two flags encode three per-area
// states: uncovered (!Covered), decomposed-not-implemented (Covered &&
// !Implemented), and implemented (Implemented, which always implies Covered).
type CoverageArea struct {
	Anchor      string   `json:"anchor"`
	Title       string   `json:"title"`
	Covered     bool     `json:"covered"`
	Implemented bool     `json:"implemented"`
	LinkedTasks []string `json:"linked_tasks"`
}

// CoverageOrphan is a task whose spec_ref resolves to a spec other than the
// active one — the deterministic orphan rule. A ref whose anchor does not exist
// at all is a hard `validate` failure, not an orphan.
type CoverageOrphan struct {
	TaskID  string `json:"task_id"`
	SpecRef string `json:"spec_ref"`
}

// DriftSummary is the two-directional drift signal: active-spec areas that
// gained no task, and tasks pointing away from the active spec.
type DriftSummary struct {
	UncoveredAreaCount int `json:"uncovered_area_count"`
	AwayTaskCount      int `json:"away_task_count"`
}

// CoverageReport is the advisory linkage analysis for the active spec. Percent
// is the decomposition figure ("is the work planned?") and gates via
// `coverage --min`; ImplementationPercent is the report-only release figure
// ("can we release it?") and never gates. Both share the coverable-area
// denominator and are nil when the spec has no coverable areas (reported as
// N/A), never 0 or 100, so an unstructured spec is not scored as a false gap or
// a hollow full mark.
type CoverageReport struct {
	ActiveSpecPath        string           `json:"active_spec_path"`
	Percent               *float64         `json:"coverage_percent"`
	ImplementationPercent *float64         `json:"implementation_percent"`
	CoveredAreas          int              `json:"covered_areas"`
	ImplementedAreas      int              `json:"implemented_areas"`
	CoverableAreas        int              `json:"coverable_areas"`
	Areas                 []CoverageArea   `json:"areas"`
	UncoveredAreas        []string         `json:"uncovered_areas"`
	Orphans               []CoverageOrphan `json:"orphans"`
	Drift                 DriftSummary     `json:"drift"`
}

// Coverage computes the read-only decomposition-coverage, orphan, and drift
// signals for the active spec. It never writes state or task files.
func (s *Service) Coverage() (CoverageReport, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return CoverageReport{}, err
	}
	return s.coverageFor(state, tasks)
}

// coverageFor computes coverage from an already-loaded state and task set, so
// callers that have paid the load cost (status) reuse the same computation
// without a second read of STATE.md and the task files.
func (s *Service) coverageFor(state *State, tasks []*Task) (CoverageReport, error) {
	activePath := state.Frontmatter.ActiveSpecPath
	data, err := os.ReadFile(filepath.Join(s.paths.RepoRoot, filepath.Clean(activePath)))
	if err != nil {
		return CoverageReport{}, fmt.Errorf("read active spec %s: %w", activePath, fsCause(err))
	}
	return computeCoverage(string(data), activePath, tasks), nil
}

// parsedArea is a coverable `###` area with the anchors (its own plus every
// `####` sub-area) that count toward its roll-up coverage.
type parsedArea struct {
	anchor     string
	title      string
	subAnchors []string
}

// computeCoverage is the shared, IO-free coverage computation. `coverage` and
// `status` reuse it so the spec-coverage metric is computed in exactly one
// place. specMarkdown is the active spec's content; activeSpecPath is its
// repo-relative path used to classify orphans.
func computeCoverage(specMarkdown, activeSpecPath string, tasks []*Task) CoverageReport {
	areas := parseCoverableAreas(specMarkdown)

	// Map every coverable anchor (area or sub-area) to its owning area so a
	// task linking a #### sub-area rolls up to its ### parent.
	areaOf := make(map[string]int, len(areas))
	for i, a := range areas {
		areaOf[a.anchor] = i
		for _, sub := range a.subAnchors {
			areaOf[sub] = i
		}
	}

	linked := make([][]string, len(areas))
	// hasOpen[i] records that area i has at least one linked, non-cancelled task
	// that is not yet completed, so implementation coverage can require every
	// such task (roll-up included) to be completed.
	hasOpen := make([]bool, len(areas))
	orphans := make([]CoverageOrphan, 0)

	cleanActive := filepath.Clean(activeSpecPath)
	for _, task := range tasks {
		if task.Frontmatter.Status == "cancelled" {
			continue
		}
		path, anchor, err := parseSpecRef(task.Frontmatter.SpecRef)
		if err != nil {
			// Malformed spec_ref is a validate concern, not an advisory signal.
			continue
		}
		if filepath.Clean(path) != cleanActive {
			// Orphan/drift is a live-work signal: a completed task pointing at a
			// prior spec is delivered history, not a task drifting away from
			// current intent, so it is not reported.
			if task.Frontmatter.Status != "completed" {
				orphans = append(orphans, CoverageOrphan{TaskID: task.Frontmatter.ID, SpecRef: task.Frontmatter.SpecRef})
			}
			continue
		}
		if idx, ok := areaOf[anchor]; ok {
			linked[idx] = append(linked[idx], task.Frontmatter.ID)
			if task.Frontmatter.Status != "completed" {
				hasOpen[idx] = true
			}
		}
	}

	reportAreas := make([]CoverageArea, len(areas))
	uncovered := make([]string, 0)
	coveredCount := 0
	implementedCount := 0
	for i, a := range areas {
		covered := len(linked[i]) > 0
		// Implemented requires the area to be decomposed and every linked task
		// (roll-up included) completed; an uncovered area is never implemented.
		implemented := covered && !hasOpen[i]
		linkedTasks := linked[i]
		if linkedTasks == nil {
			// Emit [] rather than null for an uncovered area, consistent with
			// the other report slices.
			linkedTasks = []string{}
		}
		reportAreas[i] = CoverageArea{Anchor: a.anchor, Title: a.title, Covered: covered, Implemented: implemented, LinkedTasks: linkedTasks}
		if covered {
			coveredCount++
		} else {
			uncovered = append(uncovered, a.anchor)
		}
		if implemented {
			implementedCount++
		}
	}

	report := CoverageReport{
		ActiveSpecPath:   activeSpecPath,
		CoveredAreas:     coveredCount,
		ImplementedAreas: implementedCount,
		CoverableAreas:   len(areas),
		Areas:            reportAreas,
		UncoveredAreas:   uncovered,
		Orphans:          orphans,
		Drift:            DriftSummary{UncoveredAreaCount: len(uncovered), AwayTaskCount: len(orphans)},
	}
	if len(areas) > 0 {
		denom := float64(len(areas))
		pct := float64(coveredCount) / denom * 100
		report.Percent = &pct
		ipct := float64(implementedCount) / denom * 100
		report.ImplementationPercent = &ipct
	}
	return report
}

// parseCoverableAreas returns the coverable `###` feature areas under the
// `## Potential Features` section. Areas directly followed by a `> Deferred to`
// or `> Subsumed by` marker are excluded from the denominator; both markers are
// detected generically, never from hardcoded heading names, so the rule
// survives future specs. Meta sections outside Potential Features never count.
func parseCoverableAreas(markdown string) []parsedArea {
	lines := strings.Split(markdown, "\n")
	areas := make([]parsedArea, 0)
	inSection := false
	current := -1
	for i, line := range lines {
		level, title := headingLevelTitle(strings.TrimSpace(line))
		switch {
		case level == 0:
			continue
		case level <= 2:
			// A level-1/2 heading opens or closes the Potential Features section.
			inSection = slugHeading(title) == "potential-features"
			current = -1
		case !inSection:
			continue
		case level == 3:
			if markerExcludes(lines, i) {
				current = -1
				continue
			}
			areas = append(areas, parsedArea{anchor: slugHeading(title), title: title})
			current = len(areas) - 1
		case level == 4 && current >= 0:
			areas[current].subAnchors = append(areas[current].subAnchors, slugHeading(title))
		}
	}
	return areas
}

// headingLevelTitle reports the ATX heading level (number of leading `#`) and
// the trimmed title for a markdown line, or (0, "") when the line is not a
// heading. A `#` run must be followed by a space to count, so `#hashtag` and
// bare `###` are not headings; a heading with no title text is likewise not a
// coverable heading (no spec_ref can resolve to an empty anchor).
func headingLevelTitle(trimmed string) (int, string) {
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level >= len(trimmed) || trimmed[level] != ' ' {
		return 0, ""
	}
	title := strings.TrimSpace(trimmed[level+1:])
	if title == "" {
		return 0, ""
	}
	return level, title
}

// markerExcludes reports whether the heading at headingIdx is directly followed
// (ignoring blank lines) by a `> Deferred to` or `> Subsumed by` blockquote
// marker that removes the area from the coverage denominator.
func markerExcludes(lines []string, headingIdx int) bool {
	for _, line := range lines[headingIdx+1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		return strings.HasPrefix(trimmed, "> Deferred to") || strings.HasPrefix(trimmed, "> Subsumed by")
	}
	return false
}
