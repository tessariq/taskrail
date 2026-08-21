package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newInitCmd() *cobra.Command {
	var apply bool
	var withSkills bool
	var forceSkills bool
	var local bool
	var confirmQuiescent bool
	var extractContinuationNotes bool
	var dropContinuationNotes bool
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
			"agent-tool directories is opt-in and never happens on a default init. " +
			"A repository at layout 1 previews the read-only layout 2 upgrade: the " +
			"preview resolves every operator decision before apply, and apply " +
			"requires --confirm-quiescent plus the note and skill decisions the " +
			"preview reports.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCommand(cmd, func(svc *taskrail.Service) (commandResult, error) {
				result, err := svc.Init(taskrail.InitInput{
					Apply:                    apply,
					WithSkills:               withSkills,
					ForceSkills:              forceSkills,
					Local:                    local,
					SkillVersion:             version,
					ConfirmQuiescent:         confirmQuiescent,
					ExtractContinuationNotes: extractContinuationNotes,
					DropContinuationNotes:    dropContinuationNotes,
				})
				if err != nil {
					if withSkills && skillsTouched(result.SkillInstall) {
						// Report what was installed before the failure so the user
						// knows the partial state, then propagate the error. The
						// layout is already on disk and the skill set is not, so
						// this is a partial write rather than a refusal. A refusal
						// before any install produced nothing to report.
						fmt.Fprintln(cmd.ErrOrStderr(), skillsSummary(result.SkillInstall))
					}
					return commandResult{}, err
				}
				summary := initSummary(result)
				if withSkills {
					summary += "\n" + skillsSummary(result.SkillInstall)
				}
				return commandResult{shape: "InitResult", value: result, text: summary}, nil
			})
		},
	}
	addMachineJSONFlag(cmd)
	cmd.Flags().BoolVar(&apply, "apply", false, "apply a pending layout migration instead of a dry run")
	cmd.Flags().BoolVar(&withSkills, "with-skills", false, "install the embedded repo-agnostic tracked-work skills (opt-in; installed paths are reported in both text and --json output)")
	cmd.Flags().BoolVar(&local, "local", false, "initialize ignored local planning storage in this Git worktree")
	cmd.Flags().BoolVar(&forceSkills, "force", false, "with --with-skills, reinstall embedded skills over existing copies, backing up locally-modified files first")
	cmd.Flags().BoolVar(&confirmQuiescent, "confirm-quiescent", false, "with --apply, assert every older Taskrail process able to touch this repository has stopped (required by the layout 2 upgrade)")
	cmd.Flags().BoolVar(&extractContinuationNotes, "extract-continuation-notes", false, "with the layout 2 upgrade apply, import decoded continuation notes into the planning NOTES.md sidecar")
	cmd.Flags().BoolVar(&dropContinuationNotes, "drop-continuation-notes", false, "with the layout 2 upgrade apply, drop decoded continuation notes instead of importing them")
	return cmd
}

// skillsTouched reports whether an install attempt actually wrote, overwrote,
// or backed up anything: only then is a failure path's summary evidence of a
// partial install rather than a false claim about work that never ran.
func skillsTouched(res taskrail.SkillInstallResult) bool {
	return len(res.Written) > 0 || len(res.Overwritten) > 0 || len(res.BackedUp) > 0
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
// re-run reminder when a migration is pending. The diff is rendered from the
// reported write inventory, so a human and an agent read one set of facts.
func initSummary(result taskrail.InitResult) string {
	switch {
	case result.Outcome == taskrail.InitMigrationPreview && result.ToVersion == 2 && result.FromVersion == 1:
		return layout2PreviewSummary(result)
	case result.Outcome == taskrail.InitMigrated && result.ToVersion == 2 && result.FromVersion == 1:
		return layout2AppliedSummary(result)
	case result.Outcome == taskrail.InitAdopted:
		return fmt.Sprintf("adopted existing layout; wrote marker (layout_version %d)", result.ToVersion)
	case result.Outcome == taskrail.InitCurrent:
		return fmt.Sprintf("taskrail structure already current (layout_version %d)", result.ToVersion)
	case result.Outcome == taskrail.InitMigrationPreview:
		return fmt.Sprintf("migration available %d -> %d (dry run)\n%sre-run with --apply to migrate",
			result.FromVersion, result.ToVersion, writeLines(result))
	case result.Outcome == taskrail.InitMigrated:
		return fmt.Sprintf("migrated layout %d -> %d\n%svalidation: %s",
			result.FromVersion, result.ToVersion, writeLines(result), validationLabel(result.Validation))
	case result.Outcome == taskrail.InitRetrofitPreview:
		return fmt.Sprintf("non-standard layout detected; proposed mapping (dry run)\n%s%sexisting content is not moved; re-run with --apply to retrofit",
			mappingLines(result.Mapping), writeLines(result))
	case result.Outcome == taskrail.InitRetrofitApplied:
		return fmt.Sprintf("retrofit applied (existing content was not moved)\n%s%svalidation: %s",
			mappingLines(result.Mapping), writeLines(result), validationLabel(result.Validation))
	default:
		return "initialized taskrail structure"
	}
}

// layout2PreviewSummary renders the complete layout-2 upgrade preview a human
// acts on: what the migration would rewrite, the note decision it waits for,
// which installed skills move with it, and every gate the apply demands.
func layout2PreviewSummary(result taskrail.InitResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "layout 2 upgrade available %d -> %d (dry run)\n", result.FromVersion, result.ToVersion)
	if result.Layout2Facts != nil {
		fmt.Fprintf(&b, "storage: %s (specs at %s/, planning at %s/ beneath %s); implementation review rounds: %d\n",
			result.StorageMode, result.Layout2Facts.SpecsDir, result.Layout2Facts.PlanningDir,
			result.Layout2Facts.StorageRoot, result.Layout2Facts.ReviewMaxRounds)
	}
	b.WriteString(changeLines(upgradeChangeLines(result)))
	if note := result.Notes[0]; len(note.ContinuationChoices) > 0 {
		fmt.Fprintf(&b, "continuation notes: %d decoded; apply requires one of %s\n",
			len(result.ContinuationNotes), strings.Join(flagNames(note.ContinuationChoices), " or "))
	} else {
		fmt.Fprintf(&b, "continuation notes: none to decide\n")
	}
	for _, skill := range result.Skills {
		detail := ""
		if skill.Action == "refresh" {
			detail = " (apply requires --with-skills --force)"
		}
		fmt.Fprintf(&b, "  - %s %s%s\n", skill.Action, skill.Path, detail)
	}
	b.WriteString("apply requires --confirm-quiescent (all older Taskrail processes stopped)")
	return b.String()
}

// layout2AppliedSummary renders the completed migration: what it published,
// the recorded note decision, and the downgrade path. Downgrade is complete
// Git reversion of the upgrade, never hand-editing the marker.
func layout2AppliedSummary(result taskrail.InitResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "migrated layout %d -> %d\n", result.FromVersion, result.ToVersion)
	if result.Layout2Facts != nil {
		fmt.Fprintf(&b, "storage: %s (specs at %s/, planning at %s/); implementation review rounds: %d\n",
			result.StorageMode, result.Layout2Facts.SpecsDir, result.Layout2Facts.PlanningDir,
			result.Layout2Facts.ReviewMaxRounds)
	}
	b.WriteString(changeLines(upgradeChangeLines(result)))
	if note := result.Notes[0]; note.ContinuationAction != nil {
		fmt.Fprintf(&b, "continuation notes: %s\n", *note.ContinuationAction)
	}
	for _, skill := range result.Skills {
		fmt.Fprintf(&b, "  - %s %s\n", skill.Action, skill.Path)
	}
	fmt.Fprintf(&b, "validation: %s\n", validationLabel(result.Validation))
	b.WriteString("downgrade by reverting the complete upgrade commit with git; never edit .taskrail/config.yml by hand")
	return b.String()
}

// upgradeChangeLines names the candidate paths the migration itself rewrites or
// creates; preserved task files are summarized by count rather than listed,
// because the answer to "what changes here?" must stay readable. The marker
// rewrite changes layout_version while the state rewrite publishes state schema
// 2, so each line names the version that actually moves.
func upgradeChangeLines(result taskrail.InitResult) []string {
	var changes []string
	preserved := 0
	for _, write := range result.Writes {
		switch {
		case write.Action == "create":
			changes = append(changes, "create "+write.Path)
		case write.Action == "refresh" && write.Kind == "state":
			// The state rewrite publishes state schema 2 — the layout-2 preview
			// this summary serves is defined by that schema, not by whichever
			// layout version happens to pair with it.
			changes = append(changes, fmt.Sprintf("update %s to state schema 2", write.Path))
		case write.Action == "refresh":
			changes = append(changes, fmt.Sprintf("update %s layout_version %d -> %d",
				write.Path, result.FromVersion, result.ToVersion))
		case write.Action == "preserve" && write.Kind == "task":
			preserved++
		}
	}
	if preserved > 0 {
		changes = append(changes, fmt.Sprintf("preserve %d task file(s) byte-for-byte", preserved))
	}
	return changes
}

func flagNames(choices []string) []string {
	names := make([]string, 0, len(choices))
	for _, choice := range choices {
		names = append(names, "--"+choice+"-continuation-notes")
	}
	return names
}

// writeLines lists the paths the outcome creates or rewrites. Paths it leaves
// exactly as they are stay out: the diff answers "what changes here?", and
// listing untouched files would bury that answer.
func writeLines(result taskrail.InitResult) string {
	var changes []string
	for _, write := range result.Writes {
		switch write.Action {
		case "create":
			changes = append(changes, "create "+write.Path)
		case "refresh":
			changes = append(changes, fmt.Sprintf("update %s layout_version %d -> %d",
				write.Path, result.FromVersion, result.ToVersion))
		}
	}
	return changeLines(changes)
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
