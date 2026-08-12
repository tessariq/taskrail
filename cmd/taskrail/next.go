package main

import (
	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newNextCmd() *cobra.Command {
	var includeOffSpec bool
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Select the next eligible task deterministically",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := selectNext(svc, includeOffSpec)
				if err != nil {
					return commandResult{}, err
				}
				fallback := result.TaskID
				if fallback == "" {
					fallback = "no eligible task"
				}
				return commandResult{
					shape: "NextResult", value: result, text: fallback, warnings: result.Warnings,
				}, nil
			})
		},
	}
	addMachineJSONFlag(cmd)
	cmd.Flags().BoolVar(&includeOffSpec, "include-off-spec", false, "rank eligible todo tasks across all specs, flagging an off-spec pick")
	return cmd
}

func selectNext(svc *taskrail.Service, includeOffSpec bool) (taskrail.NextResult, error) {
	if includeOffSpec {
		return svc.NextIncludingOffSpec()
	}
	return svc.Next()
}
