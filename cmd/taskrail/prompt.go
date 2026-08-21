package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newPromptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompt",
		Short: "Inspect versioned workflow prompts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newPromptListCmd(), newPromptShowCmd())
	return cmd
}

func newPromptListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List embedded workflow prompts and committed replacements (read-only)",
		Args:  machineArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.PromptList()
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "PromptListResult", value: result, text: renderPromptListText(result)}, nil
			})
		},
	}
	addMachineJSONFlag(cmd)
	return cmd
}

func newPromptShowCmd() *cobra.Command {
	var contract string
	var builtin bool
	cmd := &cobra.Command{
		Use:   "show <prompt-id>",
		Short: "Print one workflow prompt template (read-only)",
		Args:  machineArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.PromptShow(taskrail.PromptShowInput{ID: args[0], Contract: contract, Builtin: builtin})
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "PromptContentResult", value: result, text: result.Content, exactText: true}, nil
			})
		},
	}
	cmd.Flags().StringVar(&contract, "contract", "", "select an exact prompt contract version")
	cmd.Flags().BoolVar(&builtin, "builtin", false, "ignore a committed replacement")
	addMachineJSONFlag(cmd)
	return cmd
}

func renderPromptListText(result taskrail.PromptListResult) string {
	var b strings.Builder
	for _, prompt := range result.Prompts {
		if prompt.ReplacementPath == nil {
			fmt.Fprintf(&b, "%s %s %s\n", prompt.ID, prompt.ContractVersion, prompt.Source)
			continue
		}
		fmt.Fprintf(&b, "%s %s %s %s\n", prompt.ID, prompt.ContractVersion, prompt.Source, *prompt.ReplacementPath)
	}
	return strings.TrimSuffix(b.String(), "\n")
}
