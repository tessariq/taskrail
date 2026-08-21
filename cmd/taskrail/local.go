package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newLocalCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "local", Short: "Inspect local planning storage"}
	cmd.AddCommand(newLocalStatusCmd(), newLocalPathCmd())
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

func nullableText(value *string) string {
	if value == nil {
		return "null"
	}
	return *value
}
