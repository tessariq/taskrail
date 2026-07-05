package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newImportCmd() *cobra.Command {
	var (
		to    string
		apply bool
		out   string
		opt   jsonOption
	)

	cmd := &cobra.Command{
		Use:   "import <source>",
		Short: "Structurally import markdown into task, spec, or planning drafts (no LLM)",
		Long: "Deterministically parse a markdown source into T-032 draft form: headings " +
			"become spec sections and subheadings plus list items become task drafts. " +
			"Previews by default; pass --apply to write reviewable draft files. The " +
			"source file is never modified.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			result, err := svc.Import(taskrail.ImportInput{
				SourcePath: args[0],
				Target:     to,
				Apply:      apply,
				OutPath:    out,
			})
			if err != nil {
				return err
			}
			return printImportResult(cmd, opt.json, result)
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "import target: tasks, spec, or planning")
	cmd.Flags().BoolVar(&apply, "apply", false, "write reviewable draft files instead of previewing")
	cmd.Flags().StringVar(&out, "out", "", "override the draft output path (used with --apply)")
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

// printImportResult renders the import outcome. In JSON mode it emits the full
// result envelope; in preview it prints the draft (the reviewable artifact); on
// apply it reports the written paths.
func printImportResult(cmd *cobra.Command, asJSON bool, result taskrail.ImportResult) error {
	if asJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return err
	}

	if result.Applied {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", result.DraftPath); err != nil {
			return err
		}
		if result.SeedPath != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", result.SeedPath); err != nil {
				return err
			}
		}
		return nil
	}

	data, err := json.MarshalIndent(result.Draft, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal draft: %w", err)
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}
