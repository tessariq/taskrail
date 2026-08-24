package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newLoopCmd() *cobra.Command {
	var input taskrail.LoopInvocation
	var maxReviewRounds int
	var timeout time.Duration
	parallel := loopIntFlag{target: &input.Parallel}
	workspaceRoot := loopStringFlag{target: &input.WorkspaceRoot}
	cloneDepth := loopStringFlag{target: &input.CloneDepth}
	keepWorkspaces := loopStringFlag{target: &input.KeepWorkspaces}
	delivery := loopStringFlag{target: &input.Delivery}
	reviewAdapter := loopStringFlag{target: &input.ReviewAdapter}
	cmd := &cobra.Command{
		Use:   "loop",
		Short: "Preview deterministic unattended task execution",
		Args:  machineArgs(cobra.ArbitraryArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("max-review-rounds") {
				input.MaxReviewRounds = &maxReviewRounds
			}
			if cmd.Flags().Changed("timeout") {
				input.Timeout = &timeout
			}
			input.Child = args
			if input.DryRun && input.ResultFile != "" {
				return invalidArgumentsf("loop --dry-run does not support --result-file")
			}
			if !input.DryRun && input.ResultFile != "" {
				return runLoopWithResultFile(cmd, input)
			}
			if !input.DryRun && len(input.Child) == 0 {
				return invalidArgumentsf("loop execution requires a child command after --")
			}
			if !input.DryRun && machineJSONRequested(cmd) {
				return invalidArgumentsf("loop execution does not support --json")
			}
			if input.DryRun && len(input.Child) != 0 {
				return invalidArgumentsf("loop --dry-run does not accept a child command")
			}
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				if !input.DryRun {
					report, err := svc.LoopExecute(cmd.Context(), input)
					if err != nil {
						return commandResult{}, err
					}
					return commandResult{shape: "LoopDiagnostic", value: report, text: renderLoopDiagnostic(report), gate: loopDiagnosticGate(report)}, nil
				}
				report, err := svc.LoopDryRun(input)
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "LoopDryRunResult", value: report, text: renderLoopDryRun(report), gate: loopDryRunGate(report)}, nil
			})
		},
	}
	cmd.Flags().BoolVar(&input.DryRun, "dry-run", false, "report the selected task without launching a child")
	cmd.Flags().IntVar(&input.MaxIterations, "max-iterations", 1, "maximum tasks to run")
	input.Parallel, input.CloneDepth, input.KeepWorkspaces, input.Delivery = 1, "1", "failure", "local"
	cmd.Flags().Var(&parallel, "parallel", "maximum isolated tasks to preview")
	cmd.Flags().Var(&workspaceRoot, "workspace-root", "existing private root for parallel workspaces")
	cmd.Flags().Var(&cloneDepth, "clone-depth", "parallel clone depth or full")
	cmd.Flags().Var(&keepWorkspaces, "keep-workspaces", "parallel workspace retention")
	cmd.Flags().Var(&delivery, "delivery", "parallel delivery mode")
	cmd.Flags().Var(&reviewAdapter, "review-adapter", "caller-owned review delivery adapter")
	cmd.Flags().StringVar(&input.AllowPromptOverrideSHA256, "allow-prompt-override-sha256", "", "authorize an exact replacement prompt template")
	cmd.Flags().IntVar(&maxReviewRounds, "max-review-rounds", 0, "override implementation review rounds (1-2)")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "per-child timeout")
	cmd.Flags().StringVar(&input.ResultFile, "result-file", "", "publish the terminal machine envelope outside the repository")
	addMachineJSONFlag(cmd)
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		for _, flag := range []struct {
			name  string
			count int
		}{{"--parallel", parallel.count}, {"--workspace-root", workspaceRoot.count}, {"--clone-depth", cloneDepth.count}, {"--keep-workspaces", keepWorkspaces.count}, {"--delivery", delivery.count}, {"--review-adapter", reviewAdapter.count}} {
			if flag.count > 1 {
				return publishSelectionError(cmd, invalidArgumentsf("loop flag %s is repeated", flag.name))
			}
		}
		input.WorkspaceRootSet = cmd.Flags().Changed("workspace-root")
		input.CloneDepthSet = cmd.Flags().Changed("clone-depth")
		input.KeepWorkspacesSet = cmd.Flags().Changed("keep-workspaces")
		input.DeliverySet = cmd.Flags().Changed("delivery")
		input.ReviewAdapterSet = cmd.Flags().Changed("review-adapter")
		return nil
	}
	return cmd
}

func runLoopWithResultFile(cmd *cobra.Command, input taskrail.LoopInvocation) error {
	result, err := taskrail.PrepareLoopResultFile(input.ResultFile)
	if err != nil {
		return err
	}
	svc, err := serviceFromCmd(cmd)
	if err != nil {
		if publishErr := publishLoopResultFile(cmd, result, nil, err); publishErr != nil {
			return publishErr
		}
		return err
	}
	if err := svc.ValidateLoopResultFile(result); err != nil {
		return err
	}
	if len(input.Child) == 0 {
		err := invalidArgumentsf("loop execution requires a child command after --")
		if publishErr := publishLoopResultFile(cmd, result, nil, err); publishErr != nil {
			return publishErr
		}
		return err
	}
	if machineJSONRequested(cmd) {
		err := invalidArgumentsf("loop execution does not support --json")
		if publishErr := publishLoopResultFile(cmd, result, nil, err); publishErr != nil {
			return publishErr
		}
		return err
	}
	report, executeErr := svc.LoopExecute(cmd.Context(), input)
	if executeErr == nil {
		executeErr = loopDiagnosticGate(report)
	}
	if recoveryErr := svc.CheckRecovery(); recoveryErr != nil {
		if report.LastIteration == nil {
			executeErr = recoveryErr
		} else {
			report.Outcome = "invalid_postflight"
			report.NextAction = "Inspect recovery state and the completed child before another loop invocation."
			executeErr = loopDiagnosticGate(report)
		}
	}
	if publishErr := publishLoopResultFile(cmd, result, &report, executeErr); publishErr != nil {
		return publishErr
	}
	fmt.Fprintln(cmd.OutOrStdout(), renderLoopDiagnostic(report))
	return executeErr
}

func publishLoopResultFile(cmd *cobra.Command, result *taskrail.LoopResultFile, report *taskrail.LoopDiagnostic, cause error) error {
	document, err := taskrail.EncodeLoopResultFileDocument(report, cause, envelopeWarnings(cmd, nil))
	if err != nil {
		return taskrail.WithMachineErrorCode("result_file_publish_failed", fmt.Errorf("encode result file: %w", err))
	}
	if err := result.Publish(document); err != nil {
		return taskrail.WithMachineErrorCode("result_file_publish_failed", fmt.Errorf("publish result file: %w", err))
	}
	return nil
}

type loopIntFlag struct {
	target *int
	count  int
}

func (f *loopIntFlag) String() string { return strconv.Itoa(*f.target) }
func (f *loopIntFlag) Set(value string) error {
	n, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	*f.target, f.count = n, f.count+1
	return nil
}
func (f *loopIntFlag) Type() string { return "int" }

type loopStringFlag struct {
	target *string
	count  int
}

func (f *loopStringFlag) String() string { return *f.target }
func (f *loopStringFlag) Set(value string) error {
	*f.target, f.count = value, f.count+1
	return nil
}
func (f *loopStringFlag) Type() string { return "string" }

func loopDiagnosticGate(report taskrail.LoopDiagnostic) error {
	if report.Outcome == "no_work" || report.Outcome == "iteration_limit" || report.Outcome == "batch_pass" {
		return nil
	}
	return taskrail.WithMachineErrorCode(report.Outcome, fmt.Errorf("loop stopped after %s", report.Outcome))
}

func renderLoopDiagnostic(report taskrail.LoopDiagnostic) string {
	if report.LastIteration == nil {
		return fmt.Sprintf("outcome:%s iterations:%d", report.Outcome, report.IterationsCompleted)
	}
	return fmt.Sprintf("outcome:%s task:%s iterations:%d", report.Outcome, report.LastIteration.TaskID, report.IterationsCompleted)
}

func loopDryRunGate(report taskrail.LoopDryRunResult) error {
	if report.Action == "invalid" {
		return fmt.Errorf("loop dry-run is invalid: %s", report.Reason)
	}
	return nil
}

func renderLoopDryRun(report taskrail.LoopDryRunResult) string {
	if report.SelectedTask == nil {
		return fmt.Sprintf("action:%s reason:%s", report.Action, report.Reason)
	}
	return fmt.Sprintf("action:%s task:%s reason:%s", report.Action, report.SelectedTask.TaskID, report.Reason)
}
