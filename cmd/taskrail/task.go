package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage Taskrail task files",
	}
	cmd.AddCommand(newTaskNewCmd())
	cmd.AddCommand(newTaskRenameCmd())
	cmd.AddCommand(newTaskRepointCmd())
	return cmd
}

func newTaskNewCmd() *cobra.Command {
	var (
		title    string
		slug     string
		specRef  string
		area     string
		priority string
		deps     []string
		followUp string
		opt      jsonOption
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Scaffold a new task file with the next free id",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// A follow-up inherits its parent's spec_ref, and --area resolves one
			// from the active spec, so an explicit --spec-ref is only required when
			// neither is given.
			if strings.TrimSpace(followUp) == "" && strings.TrimSpace(specRef) == "" && strings.TrimSpace(area) == "" {
				return errors.New("one of --spec-ref, --area, or --follow-up is required")
			}
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			// An explicit --slug wins; otherwise the title is the slug source, so a
			// plain `task new --title "X"` still yields a slugged, scannable id. The
			// explicit slug is written verbatim; the title fallback is length-capped.
			slugSource := slug
			slugExplicit := strings.TrimSpace(slug) != ""
			if !slugExplicit {
				slugSource = title
			}
			result, err := svc.CreateTask(taskrail.CreateTaskInput{
				Title:        title,
				Slug:         slugSource,
				SlugExplicit: slugExplicit,
				SpecRef:      specRef,
				Area:         area,
				Priority:     priority,
				Dependencies: deps,
				FollowUpOf:   followUp,
			})
			if err != nil {
				return err
			}
			printWarnings(cmd, result.Warnings)
			return printResult(cmd, opt.json, result, result.Path)
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "task title; also the default slug source for the id")
	cmd.Flags().StringVar(&slug, "slug", "", "curated slug for the id suffix; overrides the title-derived slug")
	cmd.Flags().StringVar(&specRef, "spec-ref", "", "spec reference as path#anchor")
	cmd.Flags().StringVar(&area, "area", "", "active-spec anchor shorthand; resolves spec_ref to the active spec path plus this anchor (see `spec show <active-version> --anchors`)")
	cmd.Flags().StringVar(&priority, "priority", "medium", "task priority: high, medium, or low")
	cmd.Flags().StringArrayVar(&deps, "dep", nil, "dependency task id (repeatable)")
	cmd.Flags().StringVar(&followUp, "follow-up", "", "parent task id: inherit its spec_ref and depend on it")
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	// --area is the active-spec shorthand for --spec-ref; a task has one resolved ref.
	cmd.MarkFlagsMutuallyExclusive("spec-ref", "area")
	_ = cmd.RegisterFlagCompletionFunc("spec-ref", completeSpecRef)
	_ = cmd.RegisterFlagCompletionFunc("area", completeArea)
	return cmd
}

func newTaskRenameCmd() *cobra.Command {
	var (
		slug   string
		title  string
		dryRun bool
		opt    jsonOption
	)

	cmd := &cobra.Command{
		Use:   "rename <id>",
		Short: "Atomically re-slug a task's id, filename, and inbound dependency refs",
		Long: "Re-slug an existing task: rewrite its id: frontmatter, rename the file " +
			"to <new-id>.md (git mv under version control, plain rename otherwise), " +
			"rewrite every inbound dependencies: reference from the old id to the new " +
			"one, re-project planning/STATE.md, and re-run validation — all as one " +
			"outcome.\n\n" +
			"Exactly one of --slug or --title selects the new slug (--title derives it " +
			"via the same slugify and length cap as `task new` and never rewrites the " +
			"frontmatter title; --slug is written verbatim, uncapped). The numeric " +
			"T-<n> prefix is preserved; only the slug segment changes. A selector " +
			"that normalizes to no slug de-slugs the task back to " +
			"the bare T-<n> id and warns on stderr. A target id that collides with an " +
			"existing task fails before any write. --dry-run reports the change set " +
			"without writing. Rename never advances a task's status.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			result, err := svc.RenameTask(taskrail.RenameTaskInput{
				OldID:  args[0],
				Slug:   slug,
				Title:  title,
				DryRun: dryRun,
			})
			if err != nil {
				return err
			}
			printWarnings(cmd, result.Warnings)
			return printResult(cmd, opt.json, result, renameSummary(result))
		},
	}

	cmd.Flags().StringVar(&slug, "slug", "", "new slug for the id suffix")
	cmd.Flags().StringVar(&title, "title", "", "title-like source for the new slug (derived via slugify); does not rewrite the task title")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report the planned change set without writing")
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	return cmd
}

// renameSummary renders the human-readable rename outcome: the planned or applied
// change set and the resulting validation status.
func renameSummary(result taskrail.RenameTaskResult) string {
	var b strings.Builder
	if result.Applied {
		fmt.Fprintf(&b, "renamed %s -> %s\n", result.OldID, result.NewID)
	} else {
		fmt.Fprintf(&b, "rename dry run: %s -> %s (re-run without --dry-run to write)\n", result.OldID, result.NewID)
	}
	for _, ch := range result.Changes {
		if ch.Kind == "dependency_ref" {
			fmt.Fprintf(&b, "- %s in %s: %q -> %q\n", ch.Kind, ch.TaskID, ch.From, ch.To)
			continue
		}
		fmt.Fprintf(&b, "- %s: %q -> %q\n", ch.Kind, ch.From, ch.To)
	}
	fmt.Fprintf(&b, "validation: %s", validationLabel(result.Validation))
	return b.String()
}

func newTaskRepointCmd() *cobra.Command {
	var (
		area    string
		specRef string
		dryRun  bool
		opt     jsonOption
	)

	cmd := &cobra.Command{
		Use:   "repoint <id>",
		Short: "Re-point an open task's spec_ref onto a new area",
		Long: "Rewrite one open task's spec_ref frontmatter field, re-project " +
			"planning/STATE.md, and re-run validation.\n\n" +
			"Exactly one of --area or --spec-ref selects the new reference: --area " +
			"<anchor> resolves it against STATE.md's active spec using the same " +
			"anchor resolution as `task new --area` (an unknown anchor fails before " +
			"any write and points at `spec show <active-version> --anchors`), while " +
			"--spec-ref <path#anchor> sets an explicit reference for the cross-spec " +
			"case. Only spec_ref changes: the id, slug, filename, title, status, and " +
			"dependencies are untouched, and no other task file is modified. " +
			"Completed and cancelled tasks are delivered history and are rejected. " +
			"--dry-run reports the change without writing.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			result, err := svc.RepointTask(taskrail.RepointTaskInput{
				TaskID:  args[0],
				Area:    area,
				SpecRef: specRef,
				DryRun:  dryRun,
			})
			if err != nil {
				return err
			}
			return printResult(cmd, opt.json, result, repointSummary(result))
		},
	}

	cmd.Flags().StringVar(&area, "area", "", "active-spec anchor shorthand; resolves spec_ref to the active spec path plus this anchor (see `spec show <active-version> --anchors`)")
	cmd.Flags().StringVar(&specRef, "spec-ref", "", "explicit spec reference as path#anchor, for the cross-spec case")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report the planned change without writing")
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	// --area is the active-spec shorthand for --spec-ref; a task has one resolved ref.
	cmd.MarkFlagsMutuallyExclusive("spec-ref", "area")
	_ = cmd.RegisterFlagCompletionFunc("spec-ref", completeSpecRef)
	_ = cmd.RegisterFlagCompletionFunc("area", completeArea)
	return cmd
}

// repointSummary renders the human-readable re-point outcome: the planned or
// applied reference change and the resulting validation status.
func repointSummary(result taskrail.RepointTaskResult) string {
	var b strings.Builder
	if result.Applied {
		fmt.Fprintf(&b, "repointed %s: %q -> %q\n", result.TaskID, result.OldSpecRef, result.NewSpecRef)
	} else {
		fmt.Fprintf(&b, "repoint dry run: %s: %q -> %q (re-run without --dry-run to write)\n", result.TaskID, result.OldSpecRef, result.NewSpecRef)
	}
	fmt.Fprintf(&b, "validation: %s", validationLabel(result.Validation))
	return b.String()
}
