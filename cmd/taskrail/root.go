package main

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "taskrail",
		Short:         "Deterministic execution harness for repo-native tracked work",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetVersionTemplate("{{.Version}}\n")

	cmd.AddCommand(
		newInitCmd(),
		newRetrofitCmd(),
		newValidateCmd(),
		newRepairCmd(),
		newCoverageCmd(),
		newNextCmd(),
		newStartCmd(),
		newCompleteCmd(),
		newBlockCmd(),
		newVerifyCmd(),
		newTaskCmd(),
		newImportCmd(),
		newVersionCmd(),
	)

	return cmd
}
