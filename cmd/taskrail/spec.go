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
	cmd.AddCommand(newSpecActivateCmd(), newSpecListCmd(), newSpecShowCmd(), newSpecAddCmd(), newSpecDiffCmd())
	return cmd
}

// newSpecDiffCmd prints the mechanical anchor-set delta between two versioned
// specs. It is strictly read-only: it never writes STATE.md or task files and
// never gates validation.
func newSpecDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "diff <from-version> <to-version>",
		Short:             "Show the anchor-set delta between two specs (read-only)",
		Args:              machineArgs(cobra.ExactArgs(2)),
		ValidArgsFunction: completeSpecDiffVersions,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.SpecDiff(args[0], args[1])
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "SpecDiffResult", value: result, text: renderSpecDiffText(result)}, nil
			})
		},
	}
	addMachineJSONFlag(cmd)
	return cmd
}

// renderSpecDiffText summarizes the delta: added areas (need decomposition),
// removed areas (orphan existing tasks), and best-effort rename candidates. It
// labels each section by its migration meaning so the output is a worklist, not a
// bare set difference.
func renderSpecDiffText(r taskrail.SpecDiffResult) string {
	if len(r.Added) == 0 && len(r.Removed) == 0 && len(r.Renamed) == 0 {
		return fmt.Sprintf("no anchor changes between %s and %s", r.FromVersion, r.ToVersion)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "spec diff %s -> %s\n", r.FromVersion, r.ToVersion)
	fmt.Fprintf(&b, "added (%d, need decomposition into tasks):", len(r.Added))
	appendAnchorLines(&b, r.Added)
	fmt.Fprintf(&b, "\nremoved (%d, existing tasks now orphaned):", len(r.Removed))
	appendAnchorLines(&b, r.Removed)
	fmt.Fprintf(&b, "\nrename candidates (%d, best-effort, verify before acting):", len(r.Renamed))
	if len(r.Renamed) == 0 {
		b.WriteString("\n  (none)")
	}
	for _, rc := range r.Renamed {
		fmt.Fprintf(&b, "\n  - #%s -> #%s", rc.From.Anchor, rc.To.Anchor)
	}
	return b.String()
}

// appendAnchorLines writes one `#anchor heading` line per anchor, or a "(none)"
// placeholder when the section is empty.
func appendAnchorLines(b *strings.Builder, anchors []taskrail.SpecAnchor) {
	if len(anchors) == 0 {
		b.WriteString("\n  (none)")
		return
	}
	for _, a := range anchors {
		fmt.Fprintf(b, "\n  - #%s %s", a.Anchor, a.Heading)
	}
}

// newSpecAddCmd scaffolds specs/<version>.md and adds it to the specs/README.md
// reading order. It is the one writer in the spec family that authors a spec
// file; it never writes STATE.md and never activates the new spec.
func newSpecAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <version>",
		Short: "Scaffold specs/<version>.md and add it to the reading order (does not activate)",
		Args:  machineArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.AddSpec(args[0])
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{
					shape: "SpecAddResult", value: result, text: renderSpecAddText(result),
				}, nil
			})
		},
	}
	addMachineJSONFlag(cmd)
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
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List versioned specs and mark the active one (read-only)",
		Args:  machineArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.SpecList()
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "SpecListResult", value: result, text: renderSpecListText(result)}, nil
			})
		},
	}
	addMachineJSONFlag(cmd)
	return cmd
}

// newSpecShowCmd prints a versioned spec, or with --anchors its stable spec_ref
// anchor list. It is strictly read-only.
func newSpecShowCmd() *cobra.Command {
	var anchors bool
	cmd := &cobra.Command{
		Use:               "show <version>",
		Short:             "Print a spec, or with --anchors its spec_ref anchors (read-only)",
		Args:              machineArgs(cobra.ExactArgs(1)),
		ValidArgsFunction: completeSpecVersion,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.SpecShow(args[0], anchors)
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "SpecShowResult", value: result, text: renderSpecShowText(result)}, nil
			})
		},
	}
	cmd.Flags().BoolVar(&anchors, "anchors", false, "list the spec's spec_ref heading anchors instead of its body")
	addMachineJSONFlag(cmd)
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
	cmd := &cobra.Command{
		Use:               "activate <version>",
		Short:             "Repoint STATE.md's active spec to <version> and re-validate",
		Args:              machineArgs(cobra.ExactArgs(1)),
		ValidArgsFunction: completeSpecVersion,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.ActivateSpec(args[0])
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{
					shape: "SpecActivateResult", value: result, text: renderSpecActivateText(result),
				}, nil
			})
		},
	}
	addMachineJSONFlag(cmd)
	return cmd
}

// renderSpecActivateText summarizes the repoint, the re-run validation outcome,
// the coverage of the now-active spec, and the migration callout: tasks still
// pointing at the previous spec (T-075). The callout is informational — it never
// affects the exit code and never touches task files.
func renderSpecActivateText(r taskrail.SpecActivateResult) string {
	state := "valid"
	if !r.Validation.Valid {
		state = fmt.Sprintf("invalid (%d violation(s))", len(r.Validation.Violations))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "activated %s -> %s; state %s\n", r.ActiveSpecVersion, r.ActiveSpecPath, state)
	b.WriteString(coverageSummaryLine(r.Coverage))
	if len(r.PreviousSpecOrphans) > 0 {
		fmt.Fprintf(&b, "\nstill on previous spec %s (%d task(s)):", r.PreviousSpecPath, len(r.PreviousSpecOrphans))
		for _, o := range r.PreviousSpecOrphans {
			fmt.Fprintf(&b, "\n  - %s -> %s", o.TaskID, o.SpecRef)
		}
	}
	return b.String()
}
