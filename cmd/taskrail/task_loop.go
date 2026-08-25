package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newTaskLoopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "loop",
		Short: "Inspect and manage task-local unattended loop policy",
	}
	cmd.AddCommand(newTaskLoopListCmd())
	cmd.AddCommand(newTaskLoopMutationCmd(taskrail.LoopPolicyAllow))
	cmd.AddCommand(newTaskLoopMutationCmd(taskrail.LoopPolicyHold))
	cmd.AddCommand(newTaskLoopMutationCmd(taskrail.LoopPolicyClear))
	return cmd
}

func newTaskLoopMutationCmd(operation taskrail.LoopPolicyOperation) *cobra.Command {
	var (
		reason string
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:   string(operation) + " <task-id>",
		Short: strings.ToUpper(string(operation[:1])) + string(operation[1:]) + " one task's unattended loop policy",
		Args:  machineArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.MutateTaskLoopPolicy(taskrail.LoopPolicyMutationInput{
					TaskID: args[0], Operation: operation, Reason: reason, DryRun: dryRun,
				})
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "LoopPolicyMutationResult", value: result, text: loopPolicyMutationSummary(result)}, nil
			})
		},
	}
	if operation != taskrail.LoopPolicyClear {
		cmd.Flags().StringVar(&reason, "reason", "", "trimmed operator reason for this policy decision")
		_ = cmd.MarkFlagRequired("reason")
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate and report the candidate without writing")
	addMachineJSONFlag(cmd)
	return cmd
}

func loopPolicyMutationSummary(result taskrail.LoopPolicyMutationResult) string {
	action := "dry run"
	if result.Applied {
		action = "applied"
	}
	return fmt.Sprintf("loop policy %s %s: %s -> %s\nvalidation: %s", result.Operation, action,
		result.Prior.EffectivePolicy, result.Candidate.EffectivePolicy, validationLabel(&result.Validation))
}

func newTaskLoopListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Report task-local loop policy and unattended eligibility (read-only)",
		Args:  machineArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				report, err := svc.TaskLoopList()
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{
					shape: "TaskLoopListResult", value: report, text: renderTaskLoopList(report),
					gate: taskLoopListGate(report),
				}, nil
			})
		},
	}
	addMachineJSONFlag(cmd)
	return cmd
}

func taskLoopListGate(report taskrail.TaskLoopListResult) error {
	if len(report.Violations) == 0 {
		return nil
	}
	return fmt.Errorf("task loop list found %d repository violation(s)", len(report.Violations))
}

func renderTaskLoopList(report taskrail.TaskLoopListResult) string {
	var b strings.Builder
	for _, row := range report.Tasks {
		fmt.Fprintf(&b, "%s: status=%s active_spec=%t source=%s effective_policy=%s reason=%q eligible=%t held_dependencies=%s disposition=%s\n",
			row.TaskID, row.Status, row.ActiveSpec, row.Source, row.EffectivePolicy,
			row.Reason, row.Eligible, strings.Join(row.HeldDependencies, ","), row.Disposition)
	}
	for _, violation := range report.Violations {
		if violation.Path == nil {
			fmt.Fprintf(&b, "violation: %s\n", violation.Message)
			continue
		}
		fmt.Fprintf(&b, "violation: %s: %s\n", *violation.Path, violation.Message)
	}
	return strings.TrimRight(b.String(), "\n")
}
