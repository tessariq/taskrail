package main

import (
	"context"
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
//
// It prints in machine mode too. The envelope's warning array is where an agent
// reads the advisory; stderr is where the human running the same invocation on a
// terminal sees it, and neither can contaminate the document on stdout.
func warnOnSkillSkew(cmd *cobra.Command, _ []string) {
	printWarnings(cmd, ambientWarnings(cmd))
}

// ambientWarningsKey addresses the advisories observed at repository discovery,
// held on the invocation's own context so nothing leaks between invocations in a
// single process.
type ambientWarningsKey struct{}

// ambientWarnings are the advisories that accompany every registry command after
// repository discovery rather than being raised by its own work.
//
// They are observed once and reused: the contract ties them to discovery, and a
// command whose own work resolves the skew — `init --with-skills --force` is the
// one that does — would otherwise report it on stderr before the refresh and
// omit it from the envelope published after, leaving one invocation's two
// channels contradicting each other.
func ambientWarnings(cmd *cobra.Command) []taskrail.Warning {
	if cached, ok := cmd.Context().Value(ambientWarningsKey{}).([]taskrail.Warning); ok {
		return cached
	}
	warnings := observeAmbientWarnings(cmd)
	cmd.SetContext(context.WithValue(cmd.Context(), ambientWarningsKey{}, warnings))
	return warnings
}

func observeAmbientWarnings(cmd *cobra.Command) []taskrail.Warning {
	svc, err := serviceFromCmd(cmd)
	if err != nil {
		return nil
	}
	warnings, err := svc.SkillSkewWarnings(version)
	if err != nil {
		return nil
	}
	if err := svc.CheckRecovery(); err != nil {
		return nil
	}
	return warnings
}
