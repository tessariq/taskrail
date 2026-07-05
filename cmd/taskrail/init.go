package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newInitCmd() *cobra.Command {
	var opt jsonOption
	var apply bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize or upgrade Taskrail structure in the current repository",
		Long: "Initialize Taskrail in an empty repository, adopt an existing unmarked " +
			"layout, or migrate an older layout to the current version. Migration " +
			"defaults to a dry run; pass --apply to write the changes.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			result, err := svc.Init(apply)
			if err != nil {
				return err
			}
			return printResult(cmd, opt.json, result, initSummary(result))
		},
	}
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	cmd.Flags().BoolVar(&apply, "apply", false, "apply a pending layout migration instead of a dry run")
	return cmd
}

// initSummary renders the human-readable outcome, listing the diff and the
// re-run reminder when a migration is pending.
func initSummary(result taskrail.InitResult) string {
	switch result.Outcome {
	case taskrail.InitAdopted:
		return fmt.Sprintf("adopted existing layout; wrote marker (layout_version %d)", result.ToVersion)
	case taskrail.InitCurrent:
		return fmt.Sprintf("taskrail structure already current (layout_version %d)", result.ToVersion)
	case taskrail.InitMigrationPreview:
		return fmt.Sprintf("migration available %d -> %d (dry run)\n%sre-run with --apply to migrate",
			result.FromVersion, result.ToVersion, changeLines(result.Changes))
	case taskrail.InitMigrated:
		return fmt.Sprintf("migrated layout %d -> %d\n%svalidation: %s",
			result.FromVersion, result.ToVersion, changeLines(result.Changes), validationLabel(result.Validation))
	default:
		return "initialized taskrail structure"
	}
}

func changeLines(changes []string) string {
	out := ""
	for _, c := range changes {
		out += "  - " + c + "\n"
	}
	return out
}

func validationLabel(v *taskrail.ValidationResult) string {
	if v != nil && v.Valid {
		return "valid"
	}
	return "invalid"
}
