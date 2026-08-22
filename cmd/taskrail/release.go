package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newTaskReleaseCmd() *cobra.Command {
	var (
		reason string
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:   "release <task-id>",
		Short: "Return interrupted active work to todo without blocking it",
		Long: "Relinquish one exact in-progress task after interruption or deliberate rework. " +
			"Release records the required reason in Implementation Notes, clears only the matching active-task pointers, " +
			"and reprojects STATE.md without creating blocker or cancellation history. Use --dry-run to inspect the exact candidate without writing.",
		Args: machineArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.ReleaseTask(taskrail.ReleaseTaskInput{TaskID: args[0], Reason: reason, DryRun: dryRun})
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "TaskReleaseResult", value: result, text: releaseSummary(result)}, nil
			})
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "portable reason for relinquishing the active task")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate and report the candidate without writing")
	_ = cmd.MarkFlagRequired("reason")
	addMachineJSONFlag(cmd)
	return cmd
}

func releaseSummary(result taskrail.ReleaseTaskResult) string {
	if result.Applied {
		return fmt.Sprintf("released %s: %s -> %s", result.TaskID, result.PriorStatus, result.Status)
	}
	return fmt.Sprintf("release dry run: %s: %s -> %s", result.TaskID, result.PriorStatus, result.Status)
}
