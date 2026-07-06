package main

import (
	"errors"
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
		opt        jsonOption
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
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}

			if applyPath := strings.TrimSpace(apply); applyPath != "" {
				if len(args) > 0 || to != "" || emitPrompt {
					return errors.New("--apply ingests a draft file; do not combine it with a source, --to, or --emit-prompt")
				}
				result, err := svc.ApplyImportDraft(taskrail.ApplyDraftInput{DraftPath: applyPath})
				if err != nil {
					return err
				}
				return printApplyResult(cmd, opt.json, result)
			}

			if len(args) == 0 {
				return errors.New("import requires a source file, or --apply <draft.json>")
			}
			if to == "" {
				return errors.New("import requires --to (tasks, spec, or planning)")
			}

			if emitPrompt {
				result, err := svc.EmitImportPrompt(taskrail.EmitPromptInput{SourcePath: args[0], Target: to})
				if err != nil {
					return err
				}
				return printPromptResult(cmd, opt.json, result)
			}

			result, err := svc.Import(taskrail.ImportInput{SourcePath: args[0], Target: to})
			if err != nil {
				return err
			}
			return printImportResult(cmd, opt.json, result)
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "import target: tasks, spec, or planning (preview and --emit-prompt)")
	cmd.Flags().BoolVar(&emitPrompt, "emit-prompt", false, "print an agent prompt instead of a structural draft")
	cmd.Flags().StringVar(&apply, "apply", "", "write real spec/task files from an agent-produced draft JSON file")
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	return cmd
}

// printImportResult renders a structural preview. In JSON mode it emits the full
// result envelope; otherwise it prints the draft, which is the reviewable artifact
// a caller redirects to a file and later feeds to --apply.
func printImportResult(cmd *cobra.Command, asJSON bool, result taskrail.ImportResult) error {
	// Both modes emit JSON: the full envelope, or just the draft (the reviewable
	// artifact a caller redirects to a file and later feeds to --apply).
	payload := any(result)
	if !asJSON {
		payload = result.Draft
	}
	return printJSON(cmd, payload)
}

// printPromptResult renders the emit-prompt output: the raw prompt in text mode,
// the full envelope (source, target, prompt) in JSON mode.
func printPromptResult(cmd *cobra.Command, asJSON bool, result taskrail.EmitPromptResult) error {
	if asJSON {
		return printJSON(cmd, result)
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), result.Prompt)
	return err
}

// printApplyResult reports what --apply wrote: the full envelope in JSON mode, or
// one line per written artifact otherwise.
func printApplyResult(cmd *cobra.Command, asJSON bool, result taskrail.ApplyDraftResult) error {
	if asJSON {
		return printJSON(cmd, result)
	}
	out := cmd.OutOrStdout()
	if result.SpecPath != "" {
		if _, err := fmt.Fprintf(out, "wrote spec %s\n", result.SpecPath); err != nil {
			return err
		}
	}
	for _, task := range result.Tasks {
		if _, err := fmt.Fprintf(out, "created %s %s\n", task.TaskID, task.Path); err != nil {
			return err
		}
	}
	return nil
}
