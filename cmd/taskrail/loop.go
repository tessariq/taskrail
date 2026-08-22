package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newLoopCmd() *cobra.Command {
	var input taskrail.LoopInvocation
	var maxReviewRounds int
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "loop",
		Short: "Preview deterministic unattended task execution",
		Args:  machineArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().Changed("max-review-rounds") {
				input.MaxReviewRounds = &maxReviewRounds
			}
			if cmd.Flags().Changed("timeout") {
				input.Timeout = &timeout
			}
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				report, err := svc.LoopDryRun(input)
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "LoopDryRunResult", value: report, text: renderLoopDryRun(report), gate: loopDryRunGate(report)}, nil
			})
		},
	}
	cmd.Flags().BoolVar(&input.DryRun, "dry-run", false, "report the selected task without launching a child")
	cmd.Flags().IntVar(&input.MaxIterations, "max-iterations", 1, "maximum tasks to run")
	cmd.Flags().StringVar(&input.AllowPromptOverrideSHA256, "allow-prompt-override-sha256", "", "authorize an exact replacement prompt template")
	cmd.Flags().IntVar(&maxReviewRounds, "max-review-rounds", 0, "override implementation review rounds (1-2)")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "per-child timeout")
	addMachineJSONFlag(cmd)
	return cmd
}

func loopDryRunGate(report taskrail.LoopDryRunResult) error {
	if report.Action == "invalid" {
		return fmt.Errorf("loop dry-run is invalid: %s", report.Reason)
	}
	return nil
}

func renderLoopDryRun(report taskrail.LoopDryRunResult) string {
	if report.SelectedTask == nil {
		return fmt.Sprintf("action:%s reason:%s", report.Action, report.Reason)
	}
	return fmt.Sprintf("action:%s task:%s reason:%s", report.Action, report.SelectedTask.TaskID, report.Reason)
}
