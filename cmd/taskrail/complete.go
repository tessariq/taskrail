package main

import (
	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newCompleteCmd() *cobra.Command {
	var note string
	cmd := &cobra.Command{
		Use:   "complete <task-id>",
		Short: "Mark a task as completed from an implementation perspective",
		Args:  machineArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.Complete(args[0], note)
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "CompleteResult", value: result, text: result.TaskID}, nil
			})
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "optional completion note")
	addMachineJSONFlag(cmd)
	return cmd
}
