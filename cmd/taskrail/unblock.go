package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newUnblockCmd() *cobra.Command {
	var (
		reason string
		opt    jsonOption
	)
	cmd := &cobra.Command{
		Use:   "unblock <task-id>",
		Short: "Return a blocked task to todo and re-validate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			result, err := svc.Unblock(args[0], reason)
			if err != nil {
				return err
			}
			return printResult(cmd, opt.json, result, renderUnblockText(result))
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "optional note appended to the task's Implementation Notes")
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
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
