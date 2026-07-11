package main

import (
	"fmt"
	"strings"

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
	cmd.AddCommand(newSpecActivateCmd(), newSpecListCmd(), newSpecShowCmd(), newSpecAddCmd())
	return cmd
}

// newSpecAddCmd scaffolds specs/<version>.md and adds it to the specs/README.md
// reading order. It is the one writer in the spec family that authors a spec
// file; it never writes STATE.md and never activates the new spec.
func newSpecAddCmd() *cobra.Command {
	var opt jsonOption
	cmd := &cobra.Command{
		Use:   "add <version>",
		Short: "Scaffold specs/<version>.md and add it to the reading order (does not activate)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			result, err := svc.AddSpec(args[0])
			if err != nil {
				return err
			}
			return printResult(cmd, opt.json, result, renderSpecAddText(result))
		},
	}
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	return cmd
}

// renderSpecAddText summarizes the scaffold and reiterates that add does not
// activate, so the follow-up `spec activate` step stays explicit.
func renderSpecAddText(r taskrail.SpecAddResult) string {
	return fmt.Sprintf("scaffolded %s (%s); added to %s reading order — not activated", r.Version, r.SpecPath, r.ReadmePath)
}

// newSpecListCmd lists the versioned specs under specs/ and marks the active one.
// It is strictly read-only.
func newSpecListCmd() *cobra.Command {
	var opt jsonOption
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List versioned specs and mark the active one (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			result, err := svc.SpecList()
			if err != nil {
				return err
			}
			return printResult(cmd, opt.json, result, renderSpecListText(result))
		},
	}
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	return cmd
}

// newSpecShowCmd prints a versioned spec, or with --anchors its stable spec_ref
// anchor list. It is strictly read-only.
func newSpecShowCmd() *cobra.Command {
	var opt jsonOption
	var anchors bool
	cmd := &cobra.Command{
		Use:               "show <version>",
		Short:             "Print a spec, or with --anchors its spec_ref anchors (read-only)",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSpecVersion,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			result, err := svc.SpecShow(args[0], anchors)
			if err != nil {
				return err
			}
			return printResult(cmd, opt.json, result, renderSpecShowText(result))
		},
	}
	cmd.Flags().BoolVar(&anchors, "anchors", false, "list the spec's spec_ref heading anchors instead of its body")
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	return cmd
}

// renderSpecListText renders one line per spec, flagging the active one.
func renderSpecListText(r taskrail.SpecListResult) string {
	if len(r.Specs) == 0 {
		return "no versioned specs found"
	}
	var b strings.Builder
	for _, spec := range r.Specs {
		marker := ""
		if spec.Active {
			marker = " (active)"
		}
		fmt.Fprintf(&b, "%s%s — %s\n", spec.Version, marker, spec.Path)
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderSpecShowText prints the spec body, or in --anchors mode one line per
// anchor (`#anchor (Hn) heading`) so a human can copy a real spec_ref anchor.
func renderSpecShowText(r taskrail.SpecShowResult) string {
	if r.Content != "" {
		return strings.TrimRight(r.Content, "\n")
	}
	if len(r.Anchors) == 0 {
		return "no anchors found"
	}
	var b strings.Builder
	for _, a := range r.Anchors {
		fmt.Fprintf(&b, "#%s (H%d) %s\n", a.Anchor, a.Level, a.Heading)
	}
	return strings.TrimRight(b.String(), "\n")
}

// newSpecActivateCmd repoints STATE.md's active spec to a versioned target. It
// is the CLI-only writer of active_spec_version/active_spec_path and the
// sanctioned replacement for hand-editing that state.
func newSpecActivateCmd() *cobra.Command {
	var opt jsonOption
	cmd := &cobra.Command{
		Use:               "activate <version>",
		Short:             "Repoint STATE.md's active spec to <version> and re-validate",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSpecVersion,
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
	return fmt.Sprintf("activated %s -> %s; state %s\n%s", r.ActiveSpecVersion, r.ActiveSpecPath, state, coverageSummaryLine(r.Coverage))
}
