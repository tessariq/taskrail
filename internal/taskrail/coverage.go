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

// AreaAnchorIssue is an advisory diagnostic about a degenerate `###` area
// heading that distorts the coverage denominator: a punctuation-only title that
// slugs to the empty string (Kind "empty_slug", Anchor ""), or two or more
// headings that slug to the same anchor (Kind "duplicate_slug"). It names the
// offending heading title(s) in document order so the spec author fixes the
// heading rather than having the denominator silently misreport. Like every
// coverage signal it is advisory only and never makes `validate` fail; this
// mirrors validate's own leniency, which dedupes by slug and skips empty slugs
// when accepting spec_refs.
type AreaAnchorIssue struct {
	Kind   string   `json:"kind"`
	Anchor string   `json:"anchor"`
	Titles []string `json:"titles"`
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
	// AreaAnchorIssues names degenerate `###` area headings (empty or duplicate
	// slugs) that inflate the denominator; advisory only, never gates.
	AreaAnchorIssues []AreaAnchorIssue `json:"area_anchor_issues"`
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

// CoverageForArea computes the full read-only coverage report and narrows it to
// the single coverable area named by anchor, for focused "is this feature
// decomposed?" checks. The anchor is matched against the already-slugged area
// anchors (no re-slugging); an anchor that is not a coverable ### area is
// rejected with no write, and the rejection names its case (unknown, #### sub-
// area roll-up, or deferred/subsumed area) from the same single spec parse. An
// empty anchor (a punctuation-only ### title that slugs to "") and an anchor
// shared by two ### areas are rejected as invalid and ambiguous respectively,
// rather than binding to a degenerate or first-of-N area.
func (s *Service) CoverageForArea(anchor string) (CoverageReport, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return CoverageReport{}, err
	}
	activePath := state.Frontmatter.ActiveSpecPath
	markdown, err := s.readActiveSpec(activePath)
	if err != nil {
		return CoverageReport{}, err
	}
	areas, deferred := parseSpecAreas(markdown)
	report := coverageFromAreas(areas, activePath, tasks)
	// The empty anchor is never a requestable area: a punctuation-only ### title
	// slugs to "" and a bare --area "" would otherwise bind to that degenerate
	// parse. Reject it before the match scan so no empty-slug area is reachable.
	if anchor == "" {
		return CoverageReport{}, fmt.Errorf("--area %q is empty and cannot name an area of %s; run spec show --anchors to list the spec's anchors", anchor, activePath)
	}
	// Count matches so two ### areas that slug to the same anchor are rejected as
	// ambiguous rather than silently resolving to the first-scanned one.
	matches := make([]CoverageArea, 0, 1)
	for _, a := range report.Areas {
		if a.Anchor == anchor {
			matches = append(matches, a)
		}
	}
	switch len(matches) {
	case 0:
		return CoverageReport{}, areaRejectionError(areas, deferred, anchor, activePath)
	case 1:
		return narrowToArea(report, matches[0]), nil
	default:
		return CoverageReport{}, fmt.Errorf("--area %q is ambiguous in %s: %d ### areas slug to the same anchor; rename the colliding headings so each area has a unique anchor", anchor, activePath, len(matches))
	}
}

// areaRejectionError explains why anchor is not a coverable area, tailoring the
// message to the case so the fix is discoverable: a #### sub-area points at the
// ### parent it rolls up into, a deferred/subsumed area is named as intentionally
// excluded from the denominator, and anything else is an unknown anchor pointed
// at `spec show --anchors`. Classification reuses the already-parsed areas and
// deferred anchors (no second parse, no re-slugging).
func areaRejectionError(areas []parsedArea, deferred []string, anchor, specPath string) error {
	for _, a := range areas {
		for _, sub := range a.subAnchors {
			if sub == anchor {
				return fmt.Errorf("--area %q is a #### sub-area of %s that only rolls up into its ### parent; run coverage --area %s to score the parent area", anchor, specPath, a.anchor)
			}
		}
	}
	for _, d := range deferred {
		if d == anchor {
			return fmt.Errorf("--area %q is a deferred or subsumed area of %s, intentionally excluded from the coverage denominator", anchor, specPath)
		}
	}
	return fmt.Errorf("--area %q is not an area of %s; run spec show --anchors to list the spec's anchors", anchor, specPath)
}

// narrowToArea narrows a full coverage report to the one coverable area. Spec-
// wide orphans belong to no area, so the narrowed view drops them and reports
// zero away-drift, keeping the report an internally consistent picture of just
// that area.
func narrowToArea(r CoverageReport, area CoverageArea) CoverageReport {
	covered, implemented := 0, 0
	uncovered := []string{}
	if area.Covered {
		covered = 1
	} else {
		uncovered = append(uncovered, area.Anchor)
	}
	if area.Implemented {
		implemented = 1
	}
	pct := float64(covered) * 100
	ipct := float64(implemented) * 100
	return CoverageReport{
		ActiveSpecPath:        r.ActiveSpecPath,
		Percent:               &pct,
		ImplementationPercent: &ipct,
		CoveredAreas:          covered,
		ImplementedAreas:      implemented,
		CoverableAreas:        1,
		Areas:                 []CoverageArea{area},
		UncoveredAreas:        uncovered,
		Orphans:               []CoverageOrphan{},
		Drift:                 DriftSummary{UncoveredAreaCount: len(uncovered), AwayTaskCount: 0},
		// Anchor issues are a spec-wide diagnostic, not a property of one area, so
		// the narrowed single-area view drops them like it drops spec-wide orphans.
		AreaAnchorIssues: []AreaAnchorIssue{},
	}
}

// detectAreaAnchorIssues reports the degenerate `###` area headings in areas:
// every punctuation-only title that slugs to "" is collected into one empty_slug
// issue, and each non-empty slug shared by two or more headings becomes one
// duplicate_slug issue naming its colliding titles. Titles and issues preserve
// document order so the diagnostic is stable. It is read-only over the parsed
// areas and never alters the coverage denominator.
func detectAreaAnchorIssues(areas []parsedArea) []AreaAnchorIssue {
	issues := make([]AreaAnchorIssue, 0)
	emptyTitles := make([]string, 0)
	order := make([]string, 0)
	titlesByAnchor := make(map[string][]string)
	for _, a := range areas {
		if a.anchor == "" {
			emptyTitles = append(emptyTitles, a.title)
			continue
		}
		if _, seen := titlesByAnchor[a.anchor]; !seen {
			order = append(order, a.anchor)
		}
		titlesByAnchor[a.anchor] = append(titlesByAnchor[a.anchor], a.title)
	}
	if len(emptyTitles) > 0 {
		issues = append(issues, AreaAnchorIssue{Kind: "empty_slug", Anchor: "", Titles: emptyTitles})
	}
	for _, anchor := range order {
		if titles := titlesByAnchor[anchor]; len(titles) > 1 {
			issues = append(issues, AreaAnchorIssue{Kind: "duplicate_slug", Anchor: anchor, Titles: titles})
		}
	}
	return issues
}

// readActiveSpec reads the active spec's markdown, wrapping any IO error with
// the same path-portable context both the spec-wide and area-scoped coverage
// paths report.
func (s *Service) readActiveSpec(activePath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(s.paths.RepoRoot, filepath.Clean(activePath)))
	if err != nil {
		return "", fmt.Errorf("read active spec %s: %w", activePath, fsCause(err))
	}
	return string(data), nil
}

// coverageFor computes coverage from an already-loaded state and task set, so
// callers that have paid the load cost (status) reuse the same computation
// without a second read of STATE.md and the task files.
func (s *Service) coverageFor(state *State, tasks []*Task) (CoverageReport, error) {
	activePath := state.Frontmatter.ActiveSpecPath
	markdown, err := s.readActiveSpec(activePath)
	if err != nil {
		return CoverageReport{}, err
	}
	return computeCoverage(markdown, activePath, tasks), nil
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
	areas, _ := parseSpecAreas(specMarkdown)
	return coverageFromAreas(areas, activeSpecPath, tasks)
}

// coverageFromAreas computes the coverage report from areas already parsed from
// the active spec, so a caller that also needs the parse's deferred anchors
// (CoverageForArea, to classify a rejected anchor) computes coverage from the
// same single parse.
func coverageFromAreas(areas []parsedArea, activeSpecPath string, tasks []*Task) CoverageReport {
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
		AreaAnchorIssues: detectAreaAnchorIssues(areas),
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

// parseCoverableAreas returns just the coverable `###` feature areas, for
// callers that do not need the excluded (deferred/subsumed) anchors.
func parseCoverableAreas(markdown string) []parsedArea {
	areas, _ := parseSpecAreas(markdown)
	return areas
}

// parseSpecAreas returns the coverable `###` feature areas under the
// `## Potential Features` section plus the anchors of `###` areas excluded from
// the denominator by a `> Deferred to` / `> Subsumed by` marker. Both markers
// are detected generically, never from hardcoded heading names, so the rule
// survives future specs. Meta sections outside Potential Features never count.
// The deferred anchors are retained (not discarded) so a rejected `--area` can
// be classified from this one parse. Sub-areas of an excluded area are not
// recorded — that area contributes no coverable anchor at all.
func parseSpecAreas(markdown string) (areas []parsedArea, deferredAnchors []string) {
	lines := strings.Split(markdown, "\n")
	areas = make([]parsedArea, 0)
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
				deferredAnchors = append(deferredAnchors, slugHeading(title))
				current = -1
				continue
			}
			areas = append(areas, parsedArea{anchor: slugHeading(title), title: title})
			current = len(areas) - 1
		case level == 4 && current >= 0:
			areas[current].subAnchors = append(areas[current].subAnchors, slugHeading(title))
		}
	}
	return areas, deferredAnchors
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
