package main

import (
	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newVerifyCmd() *cobra.Command {
	var (
		result              string
		summary             string
		details             string
		createFollowup      bool
		followupTitle       string
		followupDescription string
		followupPriority    string
	)

	cmd := &cobra.Command{
		Use:   "verify <task-id>",
		Short: "Write verification artifacts for a task",
		Args:  machineArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				verifyResult, err := svc.Verify(taskrail.VerifyInput{
					TaskID:              args[0],
					Result:              result,
					Summary:             summary,
					Details:             details,
					CreateFollowup:      createFollowup,
					FollowupTitle:       followupTitle,
					FollowupDescription: followupDescription,
					FollowupPriority:    followupPriority,
				})
				if err != nil {
					return commandResult{}, err
				}
				// Reported on stderr only. The one advisory `--create-followup`
				// can raise is the follow-up's empty derived slug, and the
				// companion registry limits `empty_derived_slug` to the task
				// authoring commands, so publishing it in verify's envelope
				// would be a warning its contract does not allow. The bare id
				// itself is still machine-readable as `followup_task_id`.
				printWarnings(cmd, verifyResult.Warnings)
				return commandResult{
					shape: "VerifyResult", value: verifyResult, text: verifyResult.ReportPath,
				}, nil
			})
		},
	}

	cmd.Flags().StringVar(&result, "result", "", "verification result: pass or fail")
	cmd.Flags().StringVar(&summary, "summary", "", "short verification summary")
	cmd.Flags().StringVar(&details, "details", "", "optional detailed verification notes")
	cmd.Flags().BoolVar(&createFollowup, "create-followup", false, "create a follow-up task from this verification run")
	cmd.Flags().StringVar(&followupTitle, "followup-title", "", "title for the follow-up task")
	cmd.Flags().StringVar(&followupDescription, "followup-description", "", "description for the follow-up task")
	cmd.Flags().StringVar(&followupPriority, "followup-priority", "medium", "priority for the follow-up task")
	addMachineJSONFlag(cmd)
	_ = cmd.MarkFlagRequired("result")
	_ = cmd.MarkFlagRequired("summary")
	return cmd
}
