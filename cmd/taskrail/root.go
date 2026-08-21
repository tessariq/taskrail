package main

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "taskrail",
		Short:         "Deterministic execution harness for repo-native tracked work",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Skill skew is reported before the command runs, not after, so it is still
		// visible when the command fails — a skill written by a newer binary fails
		// exactly at the point of use, and the skew is the explanation.
		PersistentPreRun: warnOnSkillSkew,
	}
	cmd.SetVersionTemplate("{{.Version}}\n")
	// A rejected flag is an argument failure of an already-selected command, so
	// a machine invocation gets its error envelope rather than prose-only stderr.
	cmd.SetFlagErrorFunc(publishSelectionError)

	cmd.AddCommand(
		newInitCmd(),
		newRetrofitCmd(),
		newValidateCmd(),
		newRepairCmd(),
		newCoverageCmd(),
		newStatusCmd(),
		newStatsCmd(),
		newNextCmd(),
		newStartCmd(),
		newCompleteCmd(),
		newBlockCmd(),
		newUnblockCmd(),
		newVerifyCmd(),
		newTaskCmd(),
		newSpecCmd(),
		newPromptCmd(),
		newReviewCmd(),
		newLockCmd(),
		newRecoverCmd(),
		newImportCmd(),
		newLocalCmd(),
		newVersionCmd(),
	)

	return cmd
}
