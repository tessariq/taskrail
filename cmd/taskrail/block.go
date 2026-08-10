package main

import (
	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newBlockCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "block <task-id>",
		Short: "Mark a task as blocked and record a reason",
		Args:  machineArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.Block(args[0], reason)
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "BlockResult", value: result, text: result.TaskID}, nil
			})
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "blocking reason")
	_ = cmd.MarkFlagRequired("reason")
	addMachineJSONFlag(cmd)
	return cmd
}
