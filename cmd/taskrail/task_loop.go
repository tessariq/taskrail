package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newTaskLoopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "loop",
		Short: "Inspect and manage task-local unattended loop policy",
	}
	cmd.AddCommand(newTaskLoopListCmd())
	return cmd
}

func newTaskLoopListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Report task-local loop policy and unattended eligibility (read-only)",
		Args:  machineArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				report, err := svc.TaskLoopList()
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{
					shape: "TaskLoopListResult", value: report, text: renderTaskLoopList(report),
					gate: taskLoopListGate(report),
				}, nil
			})
		},
	}
	addMachineJSONFlag(cmd)
	return cmd
}

func taskLoopListGate(report taskrail.TaskLoopListResult) error {
	if len(report.Violations) == 0 {
		return nil
	}
	return fmt.Errorf("task loop list found %d repository violation(s)", len(report.Violations))
}

func renderTaskLoopList(report taskrail.TaskLoopListResult) string {
	var b strings.Builder
	for _, row := range report.Tasks {
		fmt.Fprintf(&b, "%s: status=%s active_spec=%t source=%s effective_policy=%s reason=%q eligible=%t held_dependencies=%s disposition=%s\n",
			row.TaskID, row.Status, row.ActiveSpec, row.Source, row.EffectivePolicy,
			row.Reason, row.Eligible, strings.Join(row.HeldDependencies, ","), row.Disposition)
	}
	for _, violation := range report.Violations {
		if violation.Path == nil {
			fmt.Fprintf(&b, "violation: %s\n", violation.Message)
			continue
		}
		fmt.Fprintf(&b, "violation: %s: %s\n", *violation.Path, violation.Message)
	}
	return strings.TrimRight(b.String(), "\n")
}
