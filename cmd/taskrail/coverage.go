package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newCoverageCmd() *cobra.Command {
	var opt jsonOption
	var minPct float64
	var area string
	var gaps bool
	var failOn []string
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Report advisory spec coverage, orphan, and drift signals (read-only)",
		Long: "Report read-only linkage analysis for the active spec: decomposition " +
			"coverage, orphan tasks, and two-directional drift. Signals are advisory " +
			"and never make validate fail; the command never writes STATE.md or task files. " +
			"--min <pct> opts into CI gating: the command exits non-zero when decomposition " +
			"coverage is below the threshold, leaving validate and the report unchanged. " +
			"--area <anchor> narrows the report to a single coverable spec area for a " +
			"focused \"is this feature decomposed?\" check. " +
			"--gaps switches to advisory structural gap analysis (missing-verification, " +
			"dependency-anomaly, under-decomposed-area) over covered areas; it composes " +
			"with --area and is advisory by default. --fail-on <category> opts into an " +
			"exit-code gate for --gaps, mirroring --min: the command exits non-zero when a " +
			"signal of the named category is present, leaving the report and validate unchanged.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			minSet := cmd.Flags().Changed("min")
			areaSet := cmd.Flags().Changed("area")
			failOnSet := cmd.Flags().Changed("fail-on")
			if gaps && minSet {
				return fmt.Errorf("--gaps analyses structural gaps and does not gate; --min cannot be combined with it")
			}
			if failOnSet && !gaps {
				return fmt.Errorf("--fail-on gates gap analysis and requires --gaps")
			}
			if failOnSet {
				if err := validateFailOn(failOn); err != nil {
					return err
				}
			}
			if minSet && (minPct < 0 || minPct > 100) {
				return fmt.Errorf("--min must be a percentage between 0 and 100, got %s", formatPercent(minPct))
			}
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			if gaps {
				gr, err := gapReport(svc, areaSet, area)
				if err != nil {
					return err
				}
				if err := printResult(cmd, opt.json, gr, renderGapText(gr)); err != nil {
					return err
				}
				return gapGate(gr, failOn)
			}
			report, err := coverageReport(svc, areaSet, area)
			if err != nil {
				return err
			}
			if err := printResult(cmd, opt.json, report, renderCoverageText(report)); err != nil {
				return err
			}
			return coverageGate(report, minSet, minPct)
		},
	}
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	cmd.Flags().Float64Var(&minPct, "min", 0, "fail (non-zero exit) when decomposition coverage is below this percentage (0–100); report stays unchanged")
	cmd.Flags().StringVar(&area, "area", "", "narrow the report to a single coverable spec area (its anchor); rejects a non-coverable anchor")
	cmd.Flags().BoolVar(&gaps, "gaps", false, "report advisory structural gap candidates over covered areas instead of coverage; composes with --area; advisory unless --fail-on is set")
	cmd.Flags().StringSliceVar(&failOn, "fail-on", nil, "with --gaps, fail (non-zero exit) when any gap signal of a named category is present (missing-verification, dependency-anomaly, under-decomposed-area); repeatable or comma-separated; report stays unchanged")
	return cmd
}

// gapGateCategories is the set of gap-signal kinds --fail-on accepts, matching
// the T-099 signal kinds computeGaps emits.
var gapGateCategories = map[string]bool{
	"missing-verification":  true,
	"dependency-anomaly":    true,
	"under-decomposed-area": true,
}

// validateFailOn rejects an unknown --fail-on category before any work, so a
// typo'd selector fails fast rather than silently gating on nothing.
func validateFailOn(categories []string) error {
	for _, c := range categories {
		if !gapGateCategories[c] {
			return fmt.Errorf("--fail-on category %q is not a gap category (missing-verification, dependency-anomaly, under-decomposed-area)", c)
		}
	}
	return nil
}

// gapGate returns a non-zero-exit error when opt-in --fail-on gating is active
// and at least one gap signal matches a selected category. Like --min it is an
// exit-code-only gate: the report (text and --json) is unchanged, gap signals
// stay advisory to validate, and no state is written. Gating is off unless
// --fail-on is passed.
func gapGate(r taskrail.GapReport, failOn []string) error {
	if len(failOn) == 0 {
		return nil
	}
	selected := make(map[string]bool, len(failOn))
	for _, c := range failOn {
		selected[c] = true
	}
	var matched []string
	for _, sig := range r.Signals {
		if selected[sig.Kind] {
			matched = append(matched, gapSignalLine(sig))
		}
	}
	if len(matched) == 0 {
		return nil
	}
	return fmt.Errorf("gap analysis found %d signal(s) matching --fail-on:\n  - %s", len(matched), strings.Join(matched, "\n  - "))
}

// coverageReport returns the full report, or the report narrowed to one coverable
// area when --area is set. A non-coverable anchor is rejected before any output.
func coverageReport(svc *taskrail.Service, areaSet bool, area string) (taskrail.CoverageReport, error) {
	if areaSet {
		return svc.CoverageForArea(area)
	}
	return svc.Coverage()
}

// gapReport returns the full structural gap report, or the report narrowed to
// one coverable area when --area is set. A non-coverable anchor is rejected
// before any output, using the same rules as coverage --area.
func gapReport(svc *taskrail.Service, areaSet bool, area string) (taskrail.GapReport, error) {
	if areaSet {
		return svc.CoverageGapsForArea(area)
	}
	return svc.CoverageGaps()
}

// renderGapText builds the human-readable structural gap report. Each row is an
// advisory candidate to promote into a real task, never auto-created state; a
// clean report says so explicitly rather than printing nothing.
func renderGapText(r taskrail.GapReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "gap analysis (advisory; promote a candidate to a task if it is real) — %s\n", r.ActiveSpecPath)
	if len(r.Signals) == 0 {
		b.WriteString("no structural gap candidates")
		return b.String()
	}
	for _, sig := range r.Signals {
		fmt.Fprintf(&b, "  - %s\n", gapSignalLine(sig))
	}
	return strings.TrimRight(b.String(), "\n")
}

// gapSignalLine renders one gap candidate: its kind, the area anchor, the
// specific task for task-scoped signals, and the mechanical detail.
func gapSignalLine(sig taskrail.GapSignal) string {
	scope := sig.Anchor
	if sig.TaskID != "" {
		scope += "/" + sig.TaskID
	}
	return fmt.Sprintf("%s: %s — %s", sig.Kind, scope, sig.Detail)
}

// coverageGate returns a non-zero-exit error when opt-in --min gating is active
// and the decomposition figure is below the threshold. Gating is off unless
// --min is passed, and never fires on an unscoreable spec (Percent nil => N/A,
// nothing to gate). The report-only implementation figure never affects the
// exit code — only decomposition coverage does.
//
// The comparison uses the percentage rounded to the displayed precision, not the
// raw float, so the exit code never contradicts the printed figure: a user who
// sets --min to the "66.7%" they read must not be failed by the hidden 66.666…
// value behind it.
func coverageGate(r taskrail.CoverageReport, gateRequested bool, min float64) error {
	if !gateRequested || r.Percent == nil {
		return nil
	}
	if displayedPercent(*r.Percent) < min {
		return fmt.Errorf("coverage %s is below --min %s", formatPercent(*r.Percent), formatPercent(min))
	}
	return nil
}

// displayedPercent rounds to one decimal place, matching formatPercent's
// rendering, so gating decisions align with the figure the user sees.
func displayedPercent(p float64) float64 {
	return math.Round(p*10) / 10
}

// renderCoverageText builds the human-readable coverage report. The percentage
// is "N/A" when the active spec has no coverable areas.
func renderCoverageText(r taskrail.CoverageReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", coverageSummaryLine(r))
	if r.Percent == nil {
		fmt.Fprintf(&b, "implementation: N/A (no coverable areas) — %s\n", r.ActiveSpecPath)
	} else {
		fmt.Fprintf(&b, "implementation: %s (%d/%d areas) — %s\n", formatPercent(*r.ImplementationPercent), r.ImplementedAreas, r.CoverableAreas, r.ActiveSpecPath)
	}

	// Reverse map: per coverable area, the covering task id(s). This makes the
	// aggregate percentage auditable — which task covers what, and where an area
	// is covered by an unexpected task or by more than one (worth reviewing).
	if len(r.Areas) > 0 {
		b.WriteString("coverage map:\n")
		for _, a := range r.Areas {
			fmt.Fprintf(&b, "  - %s\n", coverageMapLine(a))
		}
	}

	if len(r.UncoveredAreas) > 0 {
		b.WriteString("uncovered areas:\n")
		for _, area := range r.UncoveredAreas {
			fmt.Fprintf(&b, "  - %s\n", area)
		}
	}

	// Areas that are decomposed but not yet implemented — the report-only gap
	// between the two figures, surfaced as a per-area state in human output.
	if notImplemented := decomposedNotImplemented(r.Areas); len(notImplemented) > 0 {
		b.WriteString("decomposed, not implemented:\n")
		for _, area := range notImplemented {
			fmt.Fprintf(&b, "  - %s\n", area)
		}
	}

	if len(r.Orphans) > 0 {
		b.WriteString("orphans:\n")
		for _, o := range r.Orphans {
			fmt.Fprintf(&b, "  - %s -> %s\n", o.TaskID, o.SpecRef)
		}
	}

	// Advisory only: these degenerate ### headings inflate the denominator above.
	// Naming them (rather than silently dropping or silently counting them) tells
	// the author which heading to fix without hiding a typo'd real feature area.
	if len(r.AreaAnchorIssues) > 0 {
		b.WriteString("area heading issues (advisory; fix these ### headings — they inflate the coverage denominator):\n")
		for _, issue := range r.AreaAnchorIssues {
			fmt.Fprintf(&b, "  - %s\n", areaAnchorIssueLine(issue))
		}
	}

	fmt.Fprintf(&b, "drift: %d uncovered area(s), %d task(s) pointing away", r.Drift.UncoveredAreaCount, r.Drift.AwayTaskCount)
	return b.String()
}

// coverageSummaryLine is the one-line decomposition-coverage summary shared by
// `coverage` (its first line) and `spec activate` (its post-repoint echo), so
// the two never drift apart. It reports N/A when the active spec has no
// coverable areas.
func coverageSummaryLine(r taskrail.CoverageReport) string {
	if r.Percent == nil {
		return fmt.Sprintf("coverage: N/A (no coverable areas) — %s", r.ActiveSpecPath)
	}
	return fmt.Sprintf("coverage: %s (%d/%d areas) — %s", formatPercent(*r.Percent), r.CoveredAreas, r.CoverableAreas, r.ActiveSpecPath)
}

// coverageMapLine renders one reverse-map row: an area's anchor followed by its
// covering task id(s), "(uncovered)" when none cover it, and "(double-covered)"
// when more than one does so the ambiguity is flagged for review.
func coverageMapLine(a taskrail.CoverageArea) string {
	if len(a.LinkedTasks) == 0 {
		return fmt.Sprintf("%s: (uncovered)", a.Anchor)
	}
	line := fmt.Sprintf("%s: %s", a.Anchor, strings.Join(a.LinkedTasks, ", "))
	if len(a.LinkedTasks) > 1 {
		line += " (double-covered)"
	}
	return line
}

// decomposedNotImplemented returns the anchors of areas that are covered (have a
// linked task) but not yet implemented (some linked task is still open).
func decomposedNotImplemented(areas []taskrail.CoverageArea) []string {
	gap := make([]string, 0)
	for _, a := range areas {
		if a.Covered && !a.Implemented {
			gap = append(gap, a.Anchor)
		}
	}
	return gap
}

// areaAnchorIssueLine renders one advisory anchor-issue row, quoting the
// offending ### heading title(s): an empty-slug heading has no slug-able text,
// and a duplicate-slug pair names the shared anchor and every colliding title.
func areaAnchorIssueLine(issue taskrail.AreaAnchorIssue) string {
	quoted := make([]string, len(issue.Titles))
	for i, title := range issue.Titles {
		quoted[i] = fmt.Sprintf("%q", title)
	}
	joined := strings.Join(quoted, ", ")
	if issue.Kind == "duplicate_slug" {
		return fmt.Sprintf("duplicate slug %q shared by ### headings %s", issue.Anchor, joined)
	}
	return fmt.Sprintf("empty slug (### title has no slug-able text): %s", joined)
}

// renderAreaAnchorIssueHint is the one-line pointer status and stats print when
// the shared coverage computation found degenerate `###` area headings. Those
// terse dashboards carry only the count, not the per-heading naming (that lives
// in `coverage`), so an operator reading only status/stats still learns the
// figure above may be inflated and where to get the detail. Empty when there are
// no issues, so a clean spec stays quiet.
func renderAreaAnchorIssueHint(count int) string {
	if count <= 0 {
		return ""
	}
	return fmt.Sprintf("area heading issues: %d degenerate ### heading(s) may inflate coverage — run 'taskrail coverage' to list\n", count)
}

// formatPercent prints a whole percentage without a decimal and otherwise keeps
// one decimal place, so "100%" stays clean while "66.7%" is not misreported.
func formatPercent(p float64) string {
	if p == float64(int(p)) {
		return fmt.Sprintf("%d%%", int(p))
	}
	return fmt.Sprintf("%.1f%%", p)
}
