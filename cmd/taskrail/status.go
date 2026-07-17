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
			"a one-line coverage summary, a one-line orphan/drift summary " +
			"alongside it, and an active-spec drift breakdown counting open work " +
			"(todo/in_progress/blocked) on the active spec versus away from it, " +
			"listing the away tasks and their spec_ref. The away set matches the " +
			"active-spec filter next uses for idle selection. Never writes STATE.md " +
			"or task files.",
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
	b.WriteString(renderStatusDrift(r.Coverage))
	b.WriteString(renderAreaAnchorIssueHint(r.Coverage.AreaAnchorIssueCount))
	b.WriteString(renderActiveSpecDrift(r.ActiveSpecDrift))
	return b.String()
}

// renderActiveSpecDrift renders the active-spec drift breakdown: a concise line
// counting open work on versus away from the active spec, plus an inspectable
// section naming each away task and its spec_ref. It is reporting only and never
// describes selection filtering.
func renderActiveSpecDrift(d taskrail.StatusActiveSpecDrift) string {
	var b strings.Builder
	fmt.Fprintf(&b, "active-spec: %d open on active spec, %d open away\n",
		d.ActiveOpenCount, d.AwayOpenCount)
	if len(d.Away) > 0 {
		b.WriteString("away from active spec:\n")
		for _, task := range d.Away {
			fmt.Fprintf(&b, "  - %s %s\n", task.TaskID, task.SpecRef)
		}
	}
	return b.String()
}

// renderStatusNext renders the next-task line, always marked "not persisted" so
// the read-only guarantee is visible at a glance.
func renderStatusNext(n taskrail.StatusNext) string {
	var b strings.Builder
	if n.TaskID == "" {
		b.WriteString("next: none (no eligible task) — not persisted\n")
	} else {
		fmt.Fprintf(&b, "next: %s %s (%s) — not persisted\n", n.TaskID, n.Title, n.Priority)
	}
	for _, warning := range n.Warnings {
		fmt.Fprintf(&b, "%s\n", warning.Message)
	}
	return b.String()
}

func renderStatusCoverage(c taskrail.StatusCoverage) string {
	if c.DecompositionPercent == nil {
		return "coverage: N/A (no coverable areas); implementation N/A\n"
	}
	return fmt.Sprintf("coverage: %s (%d/%d areas); implementation %s (%d/%d implemented)\n",
		formatPercent(*c.DecompositionPercent), c.CoveredAreas, c.CoverableAreas,
		formatPercent(*c.ImplementationPercent), c.ImplementedAreas, c.CoverableAreas)
}

// renderStatusDrift renders the orphan/drift signals as their own one-line
// summary alongside the coverage line. The counts come from the same shared
// coverage computation; the signals are advisory and never make status fail.
func renderStatusDrift(c taskrail.StatusCoverage) string {
	return fmt.Sprintf("drift: %d orphan task(s), %d uncovered area(s)\n",
		c.OrphanTaskCount, c.UncoveredAreaCount)
}
