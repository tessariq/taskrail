package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

// newLockCmd defines the operator surface over the repository mutation lock.
// Both subcommands are the sanctioned recovery path for an abandoned lock: no
// Taskrail command ever clears one automatically, because PID, host, and age
// are evidence rather than a lease
// (specs/v0.5.0.md#repository-discovery-locking-and-recovery).
func newLockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Inspect and clear the repository mutation lock",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newLockStatusCmd(), newLockClearCmd())
	return cmd
}

// newLockStatusCmd reports absence or the exact owner metadata and raw-file
// digest of the repository mutation lock. It is strictly read-only: it takes
// no lock and writes nothing, so it is always safe to run against a lock a
// live writer holds.
func newLockStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report the repository mutation lock (read-only)",
		Args:  machineArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRecoveryTolerantCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.LockStatus()
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "LockStatusResult", value: result, text: renderLockStatusText(result)}, nil
			})
		},
	}
	addMachineJSONFlag(cmd)
	return cmd
}

// renderLockStatusText prints the operator worklist: who holds the lock, the
// digest to clear it with, and — the next step when the owner is gone — the
// guarded clear command naming both. Metadata exposes only a delegation-token
// digest, so printing it leaks no authority.
func renderLockStatusText(r taskrail.LockStatusResult) string {
	if !r.Held || r.Owner == nil || r.SHA256 == nil {
		return "no repository mutation lock is held"
	}
	owner := *r.Owner
	var b strings.Builder
	fmt.Fprintf(&b, "held by lock %s\n", owner.LockID)
	fmt.Fprintf(&b, "  command: %s\n", owner.Command)
	fmt.Fprintf(&b, "  pid: %d on %s\n", owner.PID, owner.Host)
	fmt.Fprintf(&b, "  started_at: %s\n", owner.StartedAt)
	fmt.Fprintf(&b, "  repository_root: %s\n", owner.RepositoryRoot)
	fmt.Fprintf(&b, "  storage: %s at %s\n", owner.StorageMode, owner.StorageRoot)
	fmt.Fprintf(&b, "  transaction_id: %s\n", nullableLockText(owner.TransactionID))
	if owner.ExecutableSHA256 != nil {
		fmt.Fprintf(&b, "  executable_sha256: %s\n", *owner.ExecutableSHA256)
	}
	if owner.DelegationDigest != nil {
		fmt.Fprintf(&b, "  delegation_digest: %s\n", *owner.DelegationDigest)
	}
	fmt.Fprintf(&b, "  sha256: %s\n", *r.SHA256)
	if owner.TransactionID != nil {
		fmt.Fprintf(&b, "recover: taskrail recover %s --take-over-lock %s --expect-sha256 %s", *owner.TransactionID, owner.LockID, *r.SHA256)
		return b.String()
	}
	b.WriteString("clear (only when the owner is gone): taskrail lock clear " + owner.LockID + " --expect-sha256 " + *r.SHA256)
	return b.String()
}

func nullableLockText(value *string) string {
	if value == nil {
		return "none"
	}
	return *value
}

// newLockClearCmd removes exactly the unchanged named lock an operator
// observed through `lock status`. The ID and digest are both required: they
// are the compare in this compare-and-delete, and a provably live same-host
// owner still refuses.
func newLockClearCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear <lock-id>",
		Short: "Remove one unchanged, observed repository mutation lock",
		Args:  machineArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				digest, err := cmd.Flags().GetString("expect-sha256")
				if err != nil {
					return commandResult{}, err
				}
				result, err := svc.LockClear(args[0], digest)
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "LockClearResult", value: result, text: renderLockClearText(result)}, nil
			})
		},
	}
	cmd.Flags().String("expect-sha256", "", "raw lock digest observed via lock status")
	_ = cmd.MarkFlagRequired("expect-sha256")
	addMachineJSONFlag(cmd)
	return cmd
}

func renderLockClearText(r taskrail.LockClearResult) string {
	return fmt.Sprintf("cleared repository mutation lock %s (prior sha256 %s)", r.LockID, r.PriorSHA256)
}
