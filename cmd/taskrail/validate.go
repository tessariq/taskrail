package main

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate Taskrail structure, state, and tasks",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.Validate()
				if err != nil {
					return commandResult{}, err
				}
				// An invalid repository is a completed report, not a failure to
				// produce one, so it stays a result envelope and gates.
				if !result.Valid {
					return commandResult{
						shape: "ValidateResult", value: result,
						text: "state invalid", gate: errors.New("state invalid"),
					}, nil
				}
				return commandResult{shape: "ValidateResult", value: result, text: "state valid"}, nil
			})
		},
	}
	addMachineJSONFlag(cmd)
	return cmd
}
