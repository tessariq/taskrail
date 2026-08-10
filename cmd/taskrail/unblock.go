package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newUnblockCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "unblock <task-id>",
		Short: "Return a blocked task to todo and re-validate",
		Args:  machineArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.Unblock(args[0], reason)
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{
					shape: "UnblockResult", value: result, text: renderUnblockText(result),
				}, nil
			})
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "optional note appended to the task's Implementation Notes")
	addMachineJSONFlag(cmd)
	return cmd
}

// renderUnblockText summarizes the transition and the re-run validation outcome
// for humans (mirrors renderSpecActivateText).
func renderUnblockText(r taskrail.UnblockResult) string {
	state := "valid"
	if !r.Validation.Valid {
		state = fmt.Sprintf("invalid (%d violation(s))", len(r.Validation.Violations))
	}
	return fmt.Sprintf("unblocked %s -> %s; state %s", r.TaskID, r.Status, state)
}
