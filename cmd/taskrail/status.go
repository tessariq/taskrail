package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newStatusCmd() *cobra.Command {
	var opt jsonOption
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report the current tracked-work snapshot (read-only)",
		Long: "Print a strictly read-only overview of current tracked-work state: " +
			"active spec, task counts, the next eligible task (computed but not " +
			"persisted), blocked tasks with reasons, the last verification result, " +
			"and a one-line coverage summary. Never writes STATE.md or task files.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			report, err := svc.Status()
			if err != nil {
				return err
			}
			return printResult(cmd, opt.json, report, renderStatusText(report))
		},
	}
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	return cmd
}

// renderStatusText builds the human-readable snapshot.
func renderStatusText(r taskrail.StatusReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "spec: %s — %s\n", r.ActiveSpecVersion, r.ActiveSpecPath)
	fmt.Fprintf(&b, "tasks: %d done, %d active, %d blocked, %d todo\n",
		r.Counts.Done, r.Counts.Active, r.Counts.Blocked, r.Counts.Todo)
	b.WriteString(renderStatusNext(r.Next))

	if len(r.Blocked) > 0 {
		b.WriteString("blocked:\n")
		for _, blocked := range r.Blocked {
			fmt.Fprintf(&b, "  - %s: %s\n", blocked.TaskID, blocked.Reason)
		}
	}

	last := r.LastVerificationResult
	if last == "" {
		last = "none"
	}
	fmt.Fprintf(&b, "last verification: %s\n", last)
	b.WriteString(renderStatusCoverage(r.Coverage))
	return b.String()
}

// renderStatusNext renders the next-task line, always marked "not persisted" so
// the read-only guarantee is visible at a glance.
func renderStatusNext(n taskrail.StatusNext) string {
	if n.TaskID == "" {
		return "next: none (no eligible task) — not persisted\n"
	}
	return fmt.Sprintf("next: %s %s (%s) — not persisted\n", n.TaskID, n.Title, n.Priority)
}

func renderStatusCoverage(c taskrail.StatusCoverage) string {
	if c.DecompositionPercent == nil {
		return fmt.Sprintf("coverage: N/A (no coverable areas); %d orphan(s), %d uncovered area(s)\n",
			c.OrphanTaskCount, c.UncoveredAreaCount)
	}
	return fmt.Sprintf("coverage: %s (%d/%d areas); %d orphan(s), %d uncovered area(s)\n",
		formatPercent(*c.DecompositionPercent), c.CoveredAreas, c.CoverableAreas,
		c.OrphanTaskCount, c.UncoveredAreaCount)
}
