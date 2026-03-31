package main

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "taskrail",
		Short:         "Deterministic execution harness for repo-native tracked work",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(
		newInitCmd(),
		newValidateCmd(),
		newNextCmd(),
		newStartCmd(),
		newCompleteCmd(),
		newBlockCmd(),
		newVerifyCmd(),
	)

	return cmd
}
