package main

import (
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
		opt      jsonOption
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Scaffold a new task file with the next free id",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			result, err := svc.CreateTask(taskrail.CreateTaskInput{
				Title:        title,
				SpecRef:      specRef,
				Priority:     priority,
				Dependencies: deps,
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
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("spec-ref")
	return cmd
}
