package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newRepairCmd() *cobra.Command {
	var opt jsonOption
	var apply bool
	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Conservatively repair mechanical STATE.md inconsistencies (dry run by default)",
		Long: "Reconcile STATE.md with the task files when they have drifted " +
			"mechanically: a current_task pointer that disagrees with the in_progress " +
			"task, or stale rendered task counts. It defaults to a dry run that prints " +
			"the proposed corrections and body diff; pass --apply to write STATE.md and " +
			"re-run validation.\n\n" +
			"Repair only ever rewrites STATE.md — never a task file — so it cannot " +
			"advance a status or fabricate work. Inconsistencies that need human " +
			"judgement (a missing spec_ref, a dependency cycle, more than one " +
			"in_progress task) are left untouched and reported through validation.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			result, err := svc.Repair(taskrail.RepairInput{Apply: apply})
			if err != nil {
				return err
			}
			return printResult(cmd, opt.json, result, repairSummary(result))
		},
	}
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	cmd.Flags().BoolVar(&apply, "apply", false, "apply the repair instead of a dry run")
	return cmd
}

// repairSummary renders the human-readable repair outcome: the proposed or applied
// corrections, the body diff, and the resulting validation status.
func repairSummary(result taskrail.RepairResult) string {
	var b strings.Builder
	if len(result.Changes) == 0 && len(result.BodyDiff) == 0 {
		b.WriteString("no mechanical repairs needed\n")
	} else if result.Applied {
		b.WriteString("repair applied\n")
	} else {
		b.WriteString("repair dry run (re-run with --apply to write)\n")
	}
	for _, ch := range result.Changes {
		fmt.Fprintf(&b, "- %s: %q -> %q (%s)\n", ch.Field, ch.From, ch.To, ch.Reason)
	}
	for _, line := range result.BodyDiff {
		fmt.Fprintf(&b, "  %s\n", line)
	}
	b.WriteString(fmt.Sprintf("validation: %s", validationLabel(result.Validation)))
	return b.String()
}
