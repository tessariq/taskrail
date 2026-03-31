package main

import "github.com/spf13/cobra"

func newBlockCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "block <task-id>",
		Short: "Mark a task as blocked and record a reason",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			result, err := svc.Block(args[0], reason)
			if err != nil {
				return err
			}
			return printResult(cmd, false, result, result.TaskID)
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "blocking reason")
	_ = cmd.MarkFlagRequired("reason")
	return cmd
}
