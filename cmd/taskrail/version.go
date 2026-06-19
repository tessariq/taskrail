package main

import "github.com/spf13/cobra"

// version is the release version of the taskrail binary. It defaults to a
// development placeholder and is overridden at build time via:
//
//	go build -ldflags "-X main.version=v0.1.0" ./cmd/taskrail
var version = "0.0.0-dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the taskrail version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := cmd.OutOrStdout().Write([]byte(version + "\n"))
			return err
		},
	}
}
