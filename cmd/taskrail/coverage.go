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
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Report advisory spec coverage, orphan, and drift signals (read-only)",
		Long: "Report read-only linkage analysis for the active spec: decomposition " +
			"coverage, orphan tasks, and two-directional drift. Signals are advisory " +
			"and never make validate fail; the command never writes STATE.md or task files. " +
			"--min <pct> opts into CI gating: the command exits non-zero when decomposition " +
			"coverage is below the threshold, leaving validate and the report unchanged.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().Changed("min") && (minPct < 0 || minPct > 100) {
				return fmt.Errorf("--min must be a percentage between 0 and 100, got %s", formatPercent(minPct))
			}
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			report, err := svc.Coverage()
			if err != nil {
				return err
			}
			if err := printResult(cmd, opt.json, report, renderCoverageText(report)); err != nil {
				return err
			}
			return coverageGate(report, cmd.Flags().Changed("min"), minPct)
		},
	}
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	cmd.Flags().Float64Var(&minPct, "min", 0, "fail (non-zero exit) when decomposition coverage is below this percentage (0–100); report stays unchanged")
	return cmd
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

// formatPercent prints a whole percentage without a decimal and otherwise keeps
// one decimal place, so "100%" stays clean while "66.7%" is not misreported.
func formatPercent(p float64) string {
	if p == float64(int(p)) {
		return fmt.Sprintf("%d%%", int(p))
	}
	return fmt.Sprintf("%.1f%%", p)
}
