package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newStatsCmd() *cobra.Command {
	var opt jsonOption
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Report aggregate tracked-work statistics (read-only)",
		Long: "Print detailed aggregate statistics computed snapshot-only from the " +
			"current task files and STATE.md: counts and percentages by status, the " +
			"blocked ratio and recorded-blocker count, spec-coverage with a per-area " +
			"breakdown, and dependency shape (unmet dependencies, longest chain). " +
			"Taskrail keeps no event log, so stats reports the current distribution, " +
			"not historical trends. Never writes STATE.md or task files.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			report, err := svc.Stats()
			if err != nil {
				return err
			}
			return printResult(cmd, opt.json, report, renderStatsText(report))
		},
	}
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	return cmd
}

// renderStatsText builds the human-readable statistics table.
func renderStatsText(r taskrail.StatsReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "tasks: %d total\n", r.TotalTasks)
	for _, s := range r.Statuses {
		fmt.Fprintf(&b, "  %-12s %d (%s)\n", s.Status, s.Count, formatPercent(s.Percent))
	}
	fmt.Fprintf(&b, "blocked ratio: %s; %d task(s) with recorded blockers\n",
		formatPercent(r.BlockedRatio*100), r.RecordedBlockerCount)
	b.WriteString(renderStatsCoverage(r.Coverage))
	fmt.Fprintf(&b, "dependencies: %d task(s) with unmet dependencies, longest chain %d\n",
		r.Dependencies.UnmetDependencyTaskCount, r.Dependencies.LongestChain)

	last := r.LastVerificationResult
	if last == "" {
		last = "none"
	}
	fmt.Fprintf(&b, "last verification: %s", last)
	return b.String()
}

// renderStatsCoverage renders the coverage headline plus a per-area breakdown.
// The percentage is "N/A" when the active spec has no coverable areas.
func renderStatsCoverage(c taskrail.StatsCoverage) string {
	var b strings.Builder
	if c.DecompositionPercent == nil {
		b.WriteString("coverage: N/A (no coverable areas)\n")
	} else {
		fmt.Fprintf(&b, "coverage: %s (%d/%d areas)\n",
			formatPercent(*c.DecompositionPercent), c.CoveredAreas, c.CoverableAreas)
	}
	for _, area := range c.Areas {
		mark := "uncovered"
		if area.Covered {
			mark = "covered"
		}
		fmt.Fprintf(&b, "  %-9s %s\n", mark, area.Anchor)
	}
	return b.String()
}
