package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

// newRecoverCmd is the one explicit recovery boundary for every retained
// durable transaction: preview names the single mechanically safe action the
// recorded and current snapshots derive, and --apply performs exactly that
// action. Retained state fences every other semantic command, so this command
// runs through the fence-tolerant service and no post-operation fence check
// (specs/v0.5.0.md#repository-discovery-locking-and-recovery).
func newRecoverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recover <transaction-id>",
		Short: "Preview or apply the one safe recovery action for a retained transaction",
		Args:  machineArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			apply, err := cmd.Flags().GetBool("apply")
			if err != nil {
				return err
			}
			return runRecoveryTolerantCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				takeOverLock, err := cmd.Flags().GetString("take-over-lock")
				if err != nil {
					return commandResult{}, err
				}
				expectSHA256, err := cmd.Flags().GetString("expect-sha256")
				if err != nil {
					return commandResult{}, err
				}
				result, err := svc.RecoverTransaction(cmd.Context(), args[0], apply,
					taskrail.RecoverRequest{TakeOverLockID: takeOverLock, ExpectSHA256: expectSHA256})
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "RecoverResult", value: result, text: renderRecoverText(result)}, nil
			})
		},
	}
	cmd.Flags().Bool("apply", false, "perform the previewed action instead of only reporting it")
	cmd.Flags().String("take-over-lock", "", "exact abandoned lock id authorizing recovery takeover")
	cmd.Flags().String("expect-sha256", "", "raw lock digest observed via lock status")
	cmd.MarkFlagsRequiredTogether("take-over-lock", "expect-sha256")
	addMachineJSONFlag(cmd)
	return cmd
}

// renderRecoverText states the derived action, the evidence it came from, and —
// for a preview — the exact command that would perform it.
func renderRecoverText(r taskrail.RecoverResult) string {
	var b strings.Builder
	state := "preview"
	if r.Applied {
		state = "applied"
	}
	fmt.Fprintf(&b, "transaction %s (%s) %s: %s\n", r.TransactionID, r.Command, state, r.Action)
	if r.Takeover != "none" {
		fmt.Fprintf(&b, "  takeover: %s\n", r.Takeover)
	}
	for _, snapshot := range r.Snapshots {
		fmt.Fprintf(&b, "  %s %s\n", snapshot.PathKind, snapshot.Path)
	}
	if !r.Applied {
		fmt.Fprintf(&b, "apply with: taskrail recover %s", r.TransactionID)
		if r.TakeOverLockID != nil && r.TakeOverSHA256 != nil {
			fmt.Fprintf(&b, " --take-over-lock %s --expect-sha256 %s", *r.TakeOverLockID, *r.TakeOverSHA256)
		}
		b.WriteString(" --apply")
	}
	return b.String()
}
