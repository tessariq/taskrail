package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newRetrofitCmd() *cobra.Command {
	var opt jsonOption
	var apply bool
	var emitPrompt bool
	cmd := &cobra.Command{
		Use:   "retrofit [notes]",
		Short: "Bootstrap Taskrail structure from an existing repository and human notes",
		Long: "Run the guided retrofit bootstrap flow on a non-standard repository: " +
			"detect an existing layout and propose a mapping, import the optional " +
			"human notes markdown into a reviewable planning bootstrap draft, and " +
			"scaffold specs/, planning/tasks/, and an initial STATE.md. It defaults " +
			"to a dry run that shows the proposed mapping and diff; pass --apply to " +
			"write the scaffold and marker and re-run validation. Existing files are " +
			"never overwritten and the notes file is only read. The imported " +
			"bootstrap is a proposal to review, not tracked work the retrofit " +
			"creates; adopt it through the CLI.\n\n" +
			"Pass --emit-prompt with a notes source to print the same agent prompt as " +
			"`import <notes> --to planning --emit-prompt` (reads only, scaffolds " +
			"nothing; safe on any repo, managed or not). Save the agent's draft and " +
			"run `taskrail import --apply <draft.json>` to land real spec/task files, " +
			"so retrofit is the single guided entry point for detect -> scaffold -> " +
			"import -> adopt. Without --emit-prompt, refuses an already-managed " +
			"repository (use `taskrail init` there instead).",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate flag combinations before any service I/O (path discovery,
			// config load), so a misuse fails fast without touching the filesystem.
			if emitPrompt {
				if apply {
					return errors.New("--emit-prompt prints a read-only prompt; do not combine it with --apply")
				}
				if len(args) == 0 {
					return errors.New("retrofit --emit-prompt requires a notes source")
				}
			}

			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}

			if emitPrompt {
				result, err := svc.EmitImportPrompt(taskrail.EmitPromptInput{SourcePath: args[0], Target: "planning"})
				if err != nil {
					return err
				}
				return printPromptResult(cmd, opt.json, result)
			}

			input := taskrail.RetrofitInput{Apply: apply}
			if len(args) > 0 {
				input.NotesPath = args[0]
			}
			result, err := svc.Retrofit(input)
			if err != nil {
				return err
			}
			return printResult(cmd, opt.json, result, retrofitSummary(result))
		},
	}
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	cmd.Flags().BoolVar(&apply, "apply", false, "apply the scaffold instead of a dry run")
	cmd.Flags().BoolVar(&emitPrompt, "emit-prompt", false, "print the planning-target import agent prompt instead of scaffolding")
	return cmd
}

// retrofitSummary renders the human-readable guided-retrofit outcome: the
// proposed mapping, the notes bootstrap summary, the diff, and either the apply
// validation result or the re-run reminder for a dry run.
func retrofitSummary(result taskrail.RetrofitResult) string {
	var b strings.Builder
	if result.Applied {
		b.WriteString("retrofit applied (existing content was not moved)\n")
	} else {
		b.WriteString("guided retrofit (dry run)\n")
	}
	b.WriteString(mappingLines(result.Mapping))
	b.WriteString(bootstrapLine(result.Bootstrap))
	b.WriteString(changeLines(result.Changes))
	if result.Applied {
		fmt.Fprintf(&b, "validation: %s", validationLabel(result.Validation))
	} else {
		b.WriteString("existing content is not moved; re-run with --apply to retrofit")
	}
	return b.String()
}

// bootstrapLine summarizes the notes-derived planning bootstrap, or notes that
// none was produced when no notes file was given.
func bootstrapLine(bootstrap *taskrail.ImportResult) string {
	if bootstrap == nil {
		return "planning bootstrap: none (no notes provided)\n"
	}
	return fmt.Sprintf("planning bootstrap from %s: %d task draft(s), %d spec section(s) "+
		"— a proposal to review and adopt via the CLI, not created by retrofit\n",
		bootstrap.Source, len(bootstrap.Draft.Tasks), len(bootstrap.Draft.SpecSections))
}
