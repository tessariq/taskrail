package main

import (
	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <task-id>",
		Short: "Mark a task as active",
		Args:  machineArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.Start(args[0])
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "StartResult", value: result, text: result.TaskID}, nil
			})
		},
	}
	addMachineJSONFlag(cmd)
	return cmd
}
