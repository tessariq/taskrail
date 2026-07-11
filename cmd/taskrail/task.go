package main

import (
	"errors"
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
	return cmd
}

func newTaskNewCmd() *cobra.Command {
	var (
		title    string
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
			result, err := svc.CreateTask(taskrail.CreateTaskInput{
				Title:        title,
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

	cmd.Flags().StringVar(&title, "title", "", "task title")
	cmd.Flags().StringVar(&specRef, "spec-ref", "", "spec reference as path#anchor")
	cmd.Flags().StringVar(&priority, "priority", "medium", "task priority: high, medium, or low")
	cmd.Flags().StringArrayVar(&deps, "dep", nil, "dependency task id (repeatable)")
	cmd.Flags().StringVar(&followUp, "follow-up", "", "parent task id: inherit its spec_ref and depend on it")
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.RegisterFlagCompletionFunc("spec-ref", completeSpecRef)
	return cmd
}
