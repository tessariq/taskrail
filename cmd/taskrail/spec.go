package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

// newSpecCmd defines the shared parent for the spec command family. It is the
// single attachment point the spec subcommands (activate, list/show, add) hang
// off, so those tasks do not each re-introduce and collide on a parent. Invoked
// bare it only renders help; its one writer is the activate subcommand. RunE
// keeps a bare `spec` (no subcommand) printing usage rather than an empty short
// line, and its NoArgs guard rejects an unknown positional.
func newSpecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Inspect and author Taskrail specs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newSpecActivateCmd())
	return cmd
}

// newSpecActivateCmd repoints STATE.md's active spec to a versioned target. It
// is the CLI-only writer of active_spec_version/active_spec_path and the
// sanctioned replacement for hand-editing that state.
func newSpecActivateCmd() *cobra.Command {
	var opt jsonOption
	cmd := &cobra.Command{
		Use:   "activate <version>",
		Short: "Repoint STATE.md's active spec to <version> and re-validate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			result, err := svc.ActivateSpec(args[0])
			if err != nil {
				return err
			}
			return printResult(cmd, opt.json, result, renderSpecActivateText(result))
		},
	}
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	return cmd
}

// renderSpecActivateText summarizes the repoint and the re-run validation
// outcome for humans.
func renderSpecActivateText(r taskrail.SpecActivateResult) string {
	state := "valid"
	if !r.Validation.Valid {
		state = fmt.Sprintf("invalid (%d violation(s))", len(r.Validation.Violations))
	}
	return fmt.Sprintf("activated %s -> %s; state %s", r.ActiveSpecVersion, r.ActiveSpecPath, state)
}
