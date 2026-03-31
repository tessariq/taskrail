package main

import "github.com/spf13/cobra"

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <task-id>",
		Short: "Mark a task as active",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			result, err := svc.Start(args[0])
			if err != nil {
				return err
			}
			return printResult(cmd, false, result, result.TaskID)
		},
	}
}
