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
		opt                 jsonOption
	)

	cmd := &cobra.Command{
		Use:   "verify <task-id>",
		Short: "Write verification artifacts for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
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
				return err
			}
			return printResult(cmd, opt.json, verifyResult, verifyResult.ReportPath)
		},
	}

	cmd.Flags().StringVar(&result, "result", "", "verification result: pass or fail")
	cmd.Flags().StringVar(&summary, "summary", "", "short verification summary")
	cmd.Flags().StringVar(&details, "details", "", "optional detailed verification notes")
	cmd.Flags().BoolVar(&createFollowup, "create-followup", false, "create a follow-up task from this verification run")
	cmd.Flags().StringVar(&followupTitle, "followup-title", "", "title for the follow-up task")
	cmd.Flags().StringVar(&followupDescription, "followup-description", "", "description for the follow-up task")
	cmd.Flags().StringVar(&followupPriority, "followup-priority", "medium", "priority for the follow-up task")
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	_ = cmd.MarkFlagRequired("result")
	_ = cmd.MarkFlagRequired("summary")
	return cmd
}
