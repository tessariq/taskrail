package main

import "github.com/spf13/cobra"

func newCompleteCmd() *cobra.Command {
	var note string
	cmd := &cobra.Command{
		Use:   "complete <task-id>",
		Short: "Mark a task as completed from an implementation perspective",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			result, err := svc.Complete(args[0], note)
			if err != nil {
				return err
			}
			return printResult(cmd, false, result, result.TaskID)
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "optional completion note")
	return cmd
}
