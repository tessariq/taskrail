package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newInitCmd() *cobra.Command {
	var opt jsonOption
	var apply bool
	var withSkills bool
	var forceSkills bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize or upgrade Taskrail structure in the current repository",
		Long: "Initialize Taskrail in an empty repository, adopt an existing unmarked " +
			"layout, migrate an older layout to the current version, or retrofit a " +
			"non-standard repository (one with a specs/, planning/, or notes/ " +
			"directory) by proposing a mapping. Migration and retrofit default to a " +
			"dry run; pass --apply to write the changes. Retrofit scaffolds the " +
			"Taskrail layout without moving existing content. Pass --with-skills to " +
			"install the embedded repo-agnostic tracked-work skills; installing " +
			"agent-tool directories is opt-in and never happens on a default init.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			result, err := svc.Init(apply)
			if err != nil {
				return err
			}
			summary := initSummary(result)
			if withSkills {
				res, wErr := svc.WriteShippableSkills(forceSkills)
				if wErr != nil {
					// Report what was installed before the failure so the user
					// knows the partial state, then propagate the error.
					fmt.Fprintln(cmd.ErrOrStderr(), skillsSummary(res))
					return wErr
				}
				summary += "\n" + skillsSummary(res)
			}
			return printResult(cmd, opt.json, result, summary)
		},
	}
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	cmd.Flags().BoolVar(&apply, "apply", false, "apply a pending layout migration instead of a dry run")
	cmd.Flags().BoolVar(&withSkills, "with-skills", false, "install the embedded repo-agnostic tracked-work skills (opt-in; installed paths are listed in text output only, not --json)")
	cmd.Flags().BoolVar(&forceSkills, "force", false, "with --with-skills, reinstall embedded skills over existing copies, backing up locally-modified files first")
	return cmd
}

// skillsSummary reports what --with-skills changed. Without --force a re-run is
// non-destructive, so an empty result means every skill was already present; with
// --force it also lists the files overwritten from the embedded set and the
// timestamped backups written before each overwrite.
func skillsSummary(res taskrail.SkillInstallResult) string {
	if len(res.Written) == 0 && len(res.Overwritten) == 0 && len(res.BackedUp) == 0 {
		return "skills: already installed (no files written)"
	}
	var b strings.Builder
	for _, g := range []struct {
		verb  string
		files []string
	}{
		{"installed", res.Written},
		{"overwrote", res.Overwritten},
		{"backed up", res.BackedUp},
	} {
		if len(g.files) > 0 {
			fmt.Fprintf(&b, "skills: %s %d file(s)\n%s", g.verb, len(g.files), changeLines(g.files))
		}
	}
	return strings.TrimRight(b.String(), "\n")
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
	case taskrail.InitRetrofitPreview:
		return fmt.Sprintf("non-standard layout detected; proposed mapping (dry run)\n%s%sexisting content is not moved; re-run with --apply to retrofit",
			mappingLines(result.Mapping), changeLines(result.Changes))
	case taskrail.InitRetrofitApplied:
		return fmt.Sprintf("retrofit applied (existing content was not moved)\n%s%svalidation: %s",
			mappingLines(result.Mapping), changeLines(result.Changes), validationLabel(result.Validation))
	default:
		return "initialized taskrail structure"
	}
}

// mappingLines renders the proposed retrofit mapping so the human can confirm
// how each detected directory maps onto the Taskrail layout before applying.
func mappingLines(mapping []taskrail.RetrofitMapping) string {
	var out strings.Builder
	for _, m := range mapping {
		fmt.Fprintf(&out, "  %s/ -> %s/ (%s)\n", m.Source, m.Target, m.Role)
	}
	return out.String()
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
