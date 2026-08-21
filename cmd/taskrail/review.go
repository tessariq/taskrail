package main

import (
	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newReviewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "review", Short: "Inspect and publish durable review artifacts"}
	cmd.AddCommand(newReviewPublishCmd())
	cmd.AddCommand(newReviewShowCmd())
	return cmd
}

func newReviewPublishCmd() *cobra.Command {
	var input taskrail.ReviewPublishInput
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish one validated review proposal without replacing a session",
		Args:  machineArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			input.SpecFlagSet = cmd.Flags().Changed("spec")
			input.TaskFlagSet = cmd.Flags().Changed("task")
			input.ExpectTaskSHA256FlagSet = cmd.Flags().Changed("expect-task-sha256")
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.ReviewPublish(input)
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "ReviewPublishResult", value: result, text: result.Destination}, nil
			})
		},
	}
	cmd.Flags().StringVar(&input.Type, "type", "", "review type (task, spec)")
	cmd.Flags().StringVar(&input.Proposal, "proposal", "", "transient proposal directory")
	cmd.Flags().StringVar(&input.Destination, "destination", "", "absent durable review directory")
	cmd.Flags().StringVar(&input.Spec, "spec", "", "reviewed spec version or path")
	cmd.Flags().StringVar(&input.TaskID, "task", "", "reviewed task ID")
	cmd.Flags().StringVar(&input.ExpectTaskSHA256, "expect-task-sha256", "", "expected exact task digest")
	cmd.Flags().StringVar(&input.ExpectSpecSHA256, "expect-spec-sha256", "", "expected exact spec digest")
	cmd.Flags().BoolVar(&input.DryRun, "dry-run", false, "validate without publishing")
	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("proposal")
	_ = cmd.MarkFlagRequired("destination")
	addMachineJSONFlag(cmd)
	return cmd
}

func newReviewShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <logical-review-path>",
		Short: "Print one exact durable review artifact (read-only)",
		Args:  machineArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.ReviewShow(args[0])
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "ReviewShowResult", value: result, text: result.Content, exactText: true}, nil
			})
		},
	}
	addMachineJSONFlag(cmd)
	return cmd
}
