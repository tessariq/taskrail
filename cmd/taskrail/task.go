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
	return cmd
}

func newTaskNewCmd() *cobra.Command {
	var (
		title    string
		slug     string
		specRef  string
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
			// A follow-up inherits its parent's spec_ref, so --spec-ref is only
			// required when no parent is named.
			if strings.TrimSpace(followUp) == "" && strings.TrimSpace(specRef) == "" {
				return errors.New("either --spec-ref or --follow-up is required")
			}
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			// An explicit --slug wins; otherwise the title is the slug source, so a
			// plain `task new --title "X"` still yields a slugged, scannable id.
			slugSource := slug
			if strings.TrimSpace(slugSource) == "" {
				slugSource = title
			}
			result, err := svc.CreateTask(taskrail.CreateTaskInput{
				Title:        title,
				Slug:         slugSource,
				SpecRef:      specRef,
				Priority:     priority,
				Dependencies: deps,
				FollowUpOf:   followUp,
			})
			if err != nil {
				return err
			}
			return printResult(cmd, opt.json, result, result.Path)
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "task title; also the default slug source for the id")
	cmd.Flags().StringVar(&slug, "slug", "", "curated slug for the id suffix; overrides the title-derived slug")
	cmd.Flags().StringVar(&specRef, "spec-ref", "", "spec reference as path#anchor")
	cmd.Flags().StringVar(&priority, "priority", "medium", "task priority: high, medium, or low")
	cmd.Flags().StringArrayVar(&deps, "dep", nil, "dependency task id (repeatable)")
	cmd.Flags().StringVar(&followUp, "follow-up", "", "parent task id: inherit its spec_ref and depend on it")
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	_ = cmd.RegisterFlagCompletionFunc("spec-ref", completeSpecRef)
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
			"via the same slugify as `task new` and never rewrites the frontmatter " +
			"title). The numeric T-<n> prefix is preserved; only the slug segment " +
			"changes. A target id that collides with an existing task fails before any " +
			"write. --dry-run reports the change set without writing. Rename never " +
			"advances a task's status.",
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
