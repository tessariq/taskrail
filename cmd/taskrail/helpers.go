package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func serviceFromCmd(cmd *cobra.Command) (*taskrail.Service, error) {
	return taskrail.NewService(".")
}

// printWarnings writes non-fatal signals to stderr, so they stay visible on a
// terminal without corrupting the machine-readable stdout an agent parses.
func printWarnings(cmd *cobra.Command, warnings []taskrail.Warning) {
	for _, warning := range warnings {
		fmt.Fprintln(cmd.ErrOrStderr(), warning.Message)
	}
}

// warnOnSkillSkew reports materialized skills the running binary did not write.
// It never gates: a repository Taskrail cannot even discover, or a skew read that
// fails, stays silent rather than turning an advisory signal into a command
// failure (specs/v0.4.0.md#version-skew-detection).
func warnOnSkillSkew(cmd *cobra.Command, _ []string) {
	svc, err := serviceFromCmd(cmd)
	if err != nil {
		return
	}
	warnings, err := svc.SkillSkewWarnings(version)
	if err != nil {
		return
	}
	printWarnings(cmd, warnings)
}
