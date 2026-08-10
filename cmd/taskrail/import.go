package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newImportCmd() *cobra.Command {
	var (
		to         string
		emitPrompt bool
		apply      string
	)

	cmd := &cobra.Command{
		Use:   "import [source]",
		Short: "Import markdown into task/spec/planning drafts, agent-assisted or structural (no LLM)",
		Long: "Turn a markdown source into Taskrail structure without any built-in LLM call.\n\n" +
			"Three modes:\n" +
			"  import <src> --to <target>                structural preview: prints a T-032 draft\n" +
			"  import <src> --to <target> --emit-prompt  prints a ready-to-paste agent prompt\n" +
			"  import --apply <draft.json>               writes real spec/task files from a draft\n\n" +
			"The agent does the semantic lift and returns a draft; the binary stays " +
			"provider-agnostic. The thin --llm adapter (the binary calling a model directly) " +
			"is deferred to v0.3 and is intentionally not implemented here. The source file " +
			"is never modified.",
		Args: machineArgs(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateImportArgs(args, to, emitPrompt, apply); err != nil {
				return publishMachineError(cmd, err)
			}
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				if applyPath := strings.TrimSpace(apply); applyPath != "" {
					return applyDraftResult(cmd, svc, applyPath)
				}
				if emitPrompt {
					return emitPromptResult(svc, args[0], to)
				}
				result, err := svc.Import(taskrail.ImportInput{SourcePath: args[0], Target: to})
				if err != nil {
					return commandResult{}, err
				}
				// Text mode prints the draft alone: it is the reviewable artifact a
				// caller redirects to a file and later feeds to --apply.
				draft, err := json.MarshalIndent(result.Draft, "", "  ")
				if err != nil {
					return commandResult{}, fmt.Errorf("marshal draft: %w", err)
				}
				return commandResult{shape: "ImportPreviewResult", value: result, text: string(draft)}, nil
			})
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "import target: tasks, spec, or planning (preview and --emit-prompt)")
	cmd.Flags().BoolVar(&emitPrompt, "emit-prompt", false, "print an agent prompt instead of a structural draft")
	cmd.Flags().StringVar(&apply, "apply", "", "write real spec/task files from an agent-produced draft JSON file")
	addMachineJSONFlag(cmd)
	return cmd
}

// validateImportArgs rejects a mode combination import cannot honour, before any
// repository work.
func validateImportArgs(args []string, to string, emitPrompt bool, apply string) error {
	if strings.TrimSpace(apply) != "" {
		if len(args) > 0 || to != "" || emitPrompt {
			return invalidArgumentsf("--apply ingests a draft file; do not combine it with a source, --to, or --emit-prompt")
		}
		return nil
	}
	if len(args) == 0 {
		return invalidArgumentsf("import requires a source file, or --apply <draft.json>")
	}
	if to == "" {
		return invalidArgumentsf("import requires --to (tasks, spec, or planning)")
	}
	return nil
}

// applyDraftResult writes the draft and reports what landed. A failure after
// artifacts were written is an error envelope carrying those paths, not a result:
// a partial apply never committed the complete import, so it must not read back
// as one. Text mode still lists the artifacts, which is the only place a human
// sees them.
func applyDraftResult(cmd *cobra.Command, svc *taskrail.Service, path string) (commandResult, error) {
	result, err := svc.ApplyImportDraft(taskrail.ApplyDraftInput{DraftPath: path})
	printWarnings(cmd, result.Warnings)
	if err != nil {
		if result.Partial && !machineJSONRequested(cmd) {
			fmt.Fprint(cmd.OutOrStdout(), renderApplyArtifacts(result))
		}
		return commandResult{}, err
	}
	return commandResult{
		shape: "ImportV1ApplyResult", value: result,
		text: strings.TrimSuffix(renderApplyArtifacts(result), "\n"),
	}, nil
}

// emitPromptResult renders the agent prompt shared by `import --emit-prompt` and
// `retrofit --emit-prompt`. Text mode prints the prompt itself, which is content
// for a human or another agent rather than a report about one.
func emitPromptResult(svc *taskrail.Service, source, target string) (commandResult, error) {
	result, err := svc.EmitImportPrompt(taskrail.EmitPromptInput{SourcePath: source, Target: target})
	if err != nil {
		return commandResult{}, err
	}
	return commandResult{
		shape: "EmitPromptResult", value: result,
		text: strings.TrimSuffix(result.Prompt, "\n"),
	}, nil
}

// renderApplyArtifacts lists one line per artifact the apply wrote, marking a
// partial apply's spec as one to review rather than one cleanly written.
func renderApplyArtifacts(result taskrail.ApplyDraftResult) string {
	var b strings.Builder
	if result.SpecPath != "" {
		verb := "wrote"
		if result.Partial {
			verb = "review"
		}
		fmt.Fprintf(&b, "%s spec %s\n", verb, result.SpecPath)
	}
	for _, task := range result.Tasks {
		fmt.Fprintf(&b, "created %s %s\n", task.TaskID, task.Path)
	}
	return b.String()
}
