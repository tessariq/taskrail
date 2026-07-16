package taskrail

import (
	"fmt"
	"regexp"
)

// verificationNotePattern matches the committed verification note line that
// `verify` appends via verificationNoteLine (`- <ts>: verification pass|fail`),
// anchored to the start of a line. Anchoring keeps detection mechanical: only
// the CLI's own note line counts, not an incidental "verification pass" phrase
// in a task's prose.
var verificationNotePattern = regexp.MustCompile(`(?m)^- .+: verification (pass|fail)$`)

// Under-decomposed heuristic: a covered area is flagged when its `Requirements:`
// bullet count both reaches an absolute floor (so tiny areas are never flagged)
// and is at least this many times its linked-task count (so "far exceeds" is a
// fixed, inspectable ratio, not a judgement call).
const (
	gapUnderDecomposedFactor = 3
	gapUnderDecomposedFloor  = 4
)

// GapSignal is one advisory, report-only structural gap candidate over a covered
// active-spec area. Kind is the mechanical category (missing-verification,
// dependency-anomaly, under-decomposed-area); Anchor names the covered area;
// TaskID names the specific task for task-scoped signals and is empty for area-
// scoped ones; Detail is a short mechanical explanation. A signal is always a
// candidate to promote into a real task, never auto-created state.
type GapSignal struct {
	Kind   string `json:"kind"`
	Anchor string `json:"anchor"`
	TaskID string `json:"task_id,omitempty"`
	Detail string `json:"detail"`
}

// GapReport is the advisory structural-gap analysis for the active spec: the
// mechanical companion signals over covered areas. It is strictly read-only and
// never a source of committed state.
type GapReport struct {
	ActiveSpecPath string      `json:"active_spec_path"`
	Signals        []GapSignal `json:"signals"`
}

// CoverageGaps computes the read-only structural gap signals for the active
// spec's covered areas. It never writes STATE.md or task files.
func (s *Service) CoverageGaps() (GapReport, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return GapReport{}, err
	}
	activePath := state.Frontmatter.ActiveSpecPath
	markdown, err := s.readActiveSpec(activePath)
	if err != nil {
		return GapReport{}, err
	}
	areas, _ := parseSpecAreas(markdown)
	return GapReport{ActiveSpecPath: activePath, Signals: computeGaps(areas, activePath, tasks)}, nil
}

// CoverageGapsForArea computes the gap report narrowed to a single coverable
// area, using the same anchor resolution and rejection rules as coverage --area
// so focused gap review stays aligned with the active spec. A non-coverable
// anchor is rejected before any signal is produced.
func (s *Service) CoverageGapsForArea(anchor string) (GapReport, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return GapReport{}, err
	}
	activePath := state.Frontmatter.ActiveSpecPath
	markdown, err := s.readActiveSpec(activePath)
	if err != nil {
		return GapReport{}, err
	}
	areas, deferred := parseSpecAreas(markdown)
	if err := validateGapAnchor(areas, deferred, anchor, activePath); err != nil {
		return GapReport{}, err
	}
	all := computeGaps(areas, activePath, tasks)
	scoped := make([]GapSignal, 0)
	for _, sig := range all {
		if sig.Anchor == anchor {
			scoped = append(scoped, sig)
		}
	}
	return GapReport{ActiveSpecPath: activePath, Signals: scoped}, nil
}

// validateGapAnchor rejects an anchor that is not a coverable ### area, reusing
// coverage's rejection classification so --gaps --area and --area report the
// same reason (unknown, #### sub-area, or deferred/subsumed). An empty or
// ambiguous anchor is rejected on the same terms as CoverageForArea.
func validateGapAnchor(areas []parsedArea, deferred []string, anchor, specPath string) error {
	if anchor == "" {
		return fmt.Errorf("--area %q is empty and cannot name an area of %s; run spec show --anchors to list the spec's anchors", anchor, specPath)
	}
	matches := 0
	for _, a := range areas {
		if a.anchor == anchor {
			matches++
		}
	}
	switch {
	case matches == 1:
		return nil
	case matches == 0:
		return areaRejectionError(areas, deferred, anchor, specPath)
	default:
		return fmt.Errorf("--area %q is ambiguous in %s: %d ### areas slug to the same anchor; rename the colliding headings so each area has a unique anchor", anchor, specPath, matches)
	}
}

// computeGaps is the IO-free gap computation. It emits, in document order per
// covered area, the missing-verification, dependency-anomaly, and
// under-decomposed-area candidates. Uncovered areas raise nothing — an
// undecomposed area is coverage's concern, not a structural-companion gap.
func computeGaps(areas []parsedArea, activeSpecPath string, tasks []*Task) []GapSignal {
	report := coverageFromAreas(areas, activeSpecPath, tasks)
	byID := make(map[string]*Task, len(tasks))
	for _, t := range tasks {
		byID[t.Frontmatter.ID] = t
	}

	signals := make([]GapSignal, 0)
	for i, area := range report.Areas {
		if !area.Covered {
			continue
		}
		if area.Implemented && !areaHasVerification(area.LinkedTasks, byID) {
			signals = append(signals, GapSignal{
				Kind:   "missing-verification",
				Anchor: area.Anchor,
				Detail: "area is fully completed but no linked task records a verification result (run verify, or promote a verification task)",
			})
		}
		signals = append(signals, dependencyAnomalies(area, byID)...)
		if reqs := areas[i].requirements; reqs >= gapUnderDecomposedFloor && reqs >= gapUnderDecomposedFactor*len(area.LinkedTasks) {
			signals = append(signals, GapSignal{
				Kind:   "under-decomposed-area",
				Anchor: area.Anchor,
				Detail: fmt.Sprintf("area lists %d requirement bullets but only %d linked task(s); consider decomposing further", reqs, len(area.LinkedTasks)),
			})
		}
	}
	return signals
}

// areaHasVerification reports whether any of the area's linked tasks carries a
// committed verification note.
func areaHasVerification(linked []string, byID map[string]*Task) bool {
	for _, id := range linked {
		if t, ok := byID[id]; ok && taskVerificationRecorded(t.Body) {
			return true
		}
	}
	return false
}

// taskVerificationRecorded reports whether a task body carries the committed
// verification note `verify` appends (verificationNoteLine). It matches the
// CLI's own fixed note-line shape, so this is mechanical detection, not content
// interpretation of prose that merely mentions verification.
func taskVerificationRecorded(body string) bool {
	return verificationNotePattern.MatchString(body)
}

// dependencyAnomalies returns the advisory graph signals for one covered area
// that validate does not already treat as fatal: a blocked task with no
// incomplete dependency that would clear it, and a task isolated from the area's
// dependency cluster.
func dependencyAnomalies(area CoverageArea, byID map[string]*Task) []GapSignal {
	signals := make([]GapSignal, 0)
	for _, id := range area.LinkedTasks {
		task, ok := byID[id]
		if !ok || task.Frontmatter.Status != "blocked" {
			continue
		}
		if !hasPendingDependency(task, byID) {
			signals = append(signals, GapSignal{
				Kind:   "dependency-anomaly",
				Anchor: area.Anchor,
				TaskID: id,
				Detail: "blocked task has no incomplete dependency that would unblock it",
			})
		}
	}
	for _, id := range isolatedInCluster(area, byID) {
		signals = append(signals, GapSignal{
			Kind:   "dependency-anomaly",
			Anchor: area.Anchor,
			TaskID: id,
			Detail: "task shares no dependency edge with the area's clustered tasks",
		})
	}
	return signals
}

// hasPendingDependency reports whether the task depends on at least one task
// that is still incomplete (not completed, not cancelled) — a dependency whose
// completion would plausibly clear the block. A blocked task with none is the
// anomaly: nothing in the graph explains or would resolve the block.
func hasPendingDependency(task *Task, byID map[string]*Task) bool {
	for _, dep := range task.Frontmatter.Dependencies {
		d, ok := byID[dep]
		if !ok {
			// A dangling dependency is a validate concern, not a gap signal; treat
			// it as unresolved so it is not read as an anomaly here.
			return true
		}
		if d.Frontmatter.Status != "completed" && d.Frontmatter.Status != "cancelled" {
			return true
		}
	}
	return false
}

// isolatedInCluster returns the linked tasks of an area that share no dependency
// edge with any other linked task in the same area, but only when the area's
// other tasks form at least one intra-area edge (an actual cluster). With no
// cluster (all tasks independent) nothing is flagged, since "isolated" is
// meaningless without a cluster to be isolated from.
func isolatedInCluster(area CoverageArea, byID map[string]*Task) []string {
	if len(area.LinkedTasks) < 2 {
		return nil
	}
	inArea := make(map[string]bool, len(area.LinkedTasks))
	for _, id := range area.LinkedTasks {
		inArea[id] = true
	}
	degree := make(map[string]int, len(area.LinkedTasks))
	edges := 0
	for _, id := range area.LinkedTasks {
		task, ok := byID[id]
		if !ok {
			continue
		}
		for _, dep := range task.Frontmatter.Dependencies {
			if dep == id || !inArea[dep] {
				continue
			}
			degree[id]++
			degree[dep]++
			edges++
		}
	}
	if edges == 0 {
		return nil
	}
	isolated := make([]string, 0)
	for _, id := range area.LinkedTasks {
		if degree[id] == 0 {
			isolated = append(isolated, id)
		}
	}
	return isolated
}
