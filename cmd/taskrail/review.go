package main

import (
	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newReviewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "review", Short: "Publish reviewed planning evidence"}
	cmd.AddCommand(newReviewPublishCmd())
	return cmd
}

func newReviewPublishCmd() *cobra.Command {
	var input taskrail.ReviewPublishInput
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish one validated review proposal without replacing a session",
		Args:  machineArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.ReviewPublish(input)
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "ReviewPublishResult", value: result, text: result.Destination}, nil
			})
		},
	}
	cmd.Flags().StringVar(&input.Type, "type", "", "review type (task)")
	cmd.Flags().StringVar(&input.Proposal, "proposal", "", "transient proposal directory")
	cmd.Flags().StringVar(&input.Destination, "destination", "", "absent durable review directory")
	cmd.Flags().StringVar(&input.TaskID, "task", "", "reviewed task ID")
	cmd.Flags().StringVar(&input.ExpectTaskSHA256, "expect-task-sha256", "", "expected exact task digest")
	cmd.Flags().StringVar(&input.ExpectSpecSHA256, "expect-spec-sha256", "", "expected exact spec digest")
	cmd.Flags().BoolVar(&input.DryRun, "dry-run", false, "validate without publishing")
	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("proposal")
	_ = cmd.MarkFlagRequired("destination")
	_ = cmd.MarkFlagRequired("task")
	_ = cmd.MarkFlagRequired("expect-task-sha256")
	_ = cmd.MarkFlagRequired("expect-spec-sha256")
	addMachineJSONFlag(cmd)
	return cmd
}
