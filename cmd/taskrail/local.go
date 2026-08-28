package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newLocalCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "local", Short: "Inspect and promote local planning storage"}
	cmd.AddCommand(newLocalStatusCmd(), newLocalPathCmd(), newLocalPromoteCmd())
	return cmd
}

func newLocalPromoteCmd() *cobra.Command {
	var apply, withSkills bool
	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Preview or publish local semantic planning into committed files",
		Args:  machineArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.LocalPromote(taskrail.LocalPromoteInput{Apply: apply, WithSkills: withSkills})
				if err != nil {
					return commandResult{}, err
				}
				return commandResult{shape: "LocalPromoteResult", value: result, text: renderLocalPromoteText(result)}, nil
			})
		},
	}
	addMachineJSONFlag(cmd)
	cmd.Flags().BoolVar(&apply, "apply", false, "publish the previewed semantic state without creating a Git commit")
	cmd.Flags().BoolVar(&withSkills, "with-skills", false, "make validated managed packaged skills visible without rewriting them")
	return cmd
}

func newLocalStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report local planning storage status (read-only)",
		Args:  machineArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.LocalStatus()
				return commandResult{shape: "LocalStatusResult", value: result, text: renderLocalStatusText(result)}, err
			})
		},
	}
	addMachineJSONFlag(cmd)
	return cmd
}

func newLocalPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Report local planning storage paths (read-only)",
		Args:  machineArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.LocalPath()
				return commandResult{shape: "LocalPathResult", value: result, text: renderLocalPathText(result)}, err
			})
		},
	}
	addMachineJSONFlag(cmd)
	return cmd
}

func renderLocalStatusText(r taskrail.LocalStatusResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "mode: %s\nstorage root: %s\nlogical root: %s\nworktree root: %s\ngit common dir: %s\n",
		r.Mode, r.StorageRoot, r.LogicalRoot, r.WorktreeRoot, r.GitCommonDir)
	fmt.Fprintf(&b, "origin: branch=%s head=%s\ncurrent: branch=%s head=%s\ndrift: %s\npromotion ready: %t\n",
		nullableText(r.Origin.Branch), nullableText(r.Origin.Head), nullableText(r.Current.Branch), nullableText(r.Current.Head), r.Drift, r.PromotionReady)
	b.WriteString("exclusions:\n")
	for _, exclusion := range r.Exclusions {
		fmt.Fprintf(&b, "  - %s (%s, effective=%t)\n", exclusion.Path, exclusion.Source, exclusion.Effective)
	}
	if len(r.Violations) > 0 {
		b.WriteString("violations:\n")
		for _, violation := range r.Violations {
			fmt.Fprintf(&b, "  - %s: %s\n", violation.Code, violation.Message)
		}
	}
	return b.String()
}

func renderLocalPathText(r taskrail.LocalPathResult) string {
	return fmt.Sprintf("mode: %s\nconfig path: %s\nstorage root: %s\nspecs dir: %s\nplanning dir: %s\nprompts dir: %s\nartifacts dir: %s\nruntime dir: %s",
		r.Mode, r.ConfigPath, r.StorageRoot, r.SpecsDir, r.PlanningDir, r.PromptsDir, r.ArtifactsDir, r.RuntimeDir)
}

func renderLocalPromoteText(r taskrail.LocalPromoteResult) string {
	state := "preview"
	if r.Applied {
		state = "applied"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "local promotion %s: %s -> %s\n", state, r.SourceMode, r.TargetMode)
	for _, entry := range r.Writes {
		fmt.Fprintf(&b, "write: %s (%s)\n", entry.Path, entry.Kind)
	}
	for _, entry := range r.Excluded {
		fmt.Fprintf(&b, "exclude: %s (%s)\n", entry.Path, entry.Kind)
	}
	for _, exclusion := range r.RemovedExclusions {
		fmt.Fprintf(&b, "remove exclusion: %s\n", exclusion)
	}
	for _, skill := range r.Skills {
		fmt.Fprintf(&b, "skill: %s (%s)\n", skill.Path, skill.Action)
	}
	fmt.Fprintf(&b, "validation: %s", validationLabel(&r.Validation))
	return b.String()
}

func nullableText(value *string) string {
	if value == nil {
		return "null"
	}
	return *value
}
