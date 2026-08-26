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
		to                 string
		emitPrompt         bool
		apply              string
		expectSHA256       string
		reviewManifest     string
		expectReviewSHA256 string
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
				return publishMachineError(cmd, err, nil)
			}
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				if applyPath := strings.TrimSpace(apply); applyPath != "" {
					return applyDraftResult(cmd, svc, taskrail.ApplyDraftInput{DraftPath: applyPath, ExpectSHA256: expectSHA256, ReviewManifestPath: reviewManifest, ExpectReviewSHA256: expectReviewSHA256})
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
	cmd.Flags().StringVar(&expectSHA256, "expect-sha256", "", "expected SHA-256 for a published ImportDraft v2")
	cmd.Flags().StringVar(&reviewManifest, "review-manifest", "", "published decomposition manifest for an ImportDraft v2")
	cmd.Flags().StringVar(&expectReviewSHA256, "expect-review-sha256", "", "expected SHA-256 for a published decomposition manifest")
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

// applyDraftResult writes a complete import transaction and reports only its
// committed spec and task files.
func applyDraftResult(cmd *cobra.Command, svc *taskrail.Service, input taskrail.ApplyDraftInput) (commandResult, error) {
	result, err := svc.ApplyImportDraft(input)
	if err != nil {
		return commandResult{warnings: result.Warnings}, err
	}
	return commandResult{
		shape: "ImportV1ApplyResult", value: result,
		text:     strings.TrimSuffix(renderApplyArtifacts(result), "\n"),
		warnings: result.Warnings,
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

// renderApplyArtifacts lists one line per artifact the import committed.
func renderApplyArtifacts(result taskrail.ApplyDraftResult) string {
	var b strings.Builder
	if result.SpecPath != "" {
		fmt.Fprintf(&b, "wrote spec %s\n", result.SpecPath)
	}
	for _, task := range result.Tasks {
		fmt.Fprintf(&b, "created %s %s\n", task.TaskID, task.Path)
	}
	return b.String()
}
