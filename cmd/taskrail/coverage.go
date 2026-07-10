package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newCoverageCmd() *cobra.Command {
	var opt jsonOption
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Report advisory spec coverage, orphan, and drift signals (read-only)",
		Long: "Report read-only linkage analysis for the active spec: decomposition " +
			"coverage, orphan tasks, and two-directional drift. Signals are advisory " +
			"and never make validate fail; the command never writes STATE.md or task files.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			report, err := svc.Coverage()
			if err != nil {
				return err
			}
			return printResult(cmd, opt.json, report, renderCoverageText(report))
		},
	}
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	return cmd
}

// renderCoverageText builds the human-readable coverage report. The percentage
// is "N/A" when the active spec has no coverable areas.
func renderCoverageText(r taskrail.CoverageReport) string {
	var b strings.Builder
	if r.Percent == nil {
		fmt.Fprintf(&b, "coverage: N/A (no coverable areas) — %s\n", r.ActiveSpecPath)
		fmt.Fprintf(&b, "implementation: N/A (no coverable areas) — %s\n", r.ActiveSpecPath)
	} else {
		fmt.Fprintf(&b, "coverage: %s (%d/%d areas) — %s\n", formatPercent(*r.Percent), r.CoveredAreas, r.CoverableAreas, r.ActiveSpecPath)
		fmt.Fprintf(&b, "implementation: %s (%d/%d areas) — %s\n", formatPercent(*r.ImplementationPercent), r.ImplementedAreas, r.CoverableAreas, r.ActiveSpecPath)
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
