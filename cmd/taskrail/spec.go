package main

import "github.com/spf13/cobra"

// newSpecCmd defines the shared parent for the spec command family. It is the
// single attachment point the spec subcommands (activate, list/show, add) hang
// off, so those tasks do not each re-introduce and collide on a parent. It is
// read-only: invoked bare it only renders help and writes nothing. RunE exists
// because until those subcommands land the parent has none, and a childless
// parent needs it to print usage rather than an empty short line.
func newSpecCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "spec",
		Short: "Inspect and author Taskrail specs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
}
