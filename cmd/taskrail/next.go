package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

func newNextCmd() *cobra.Command {
	var opt jsonOption
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Select the next eligible task deterministically",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			result, err := svc.Next()
			if err != nil {
				return err
			}
			fallback := result.TaskID
			if fallback == "" {
				fallback = "no eligible task"
			}
			return printResult(cmd, opt.json, result, renderNextText(result, fallback))
		},
	}
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	return cmd
}

func renderNextText(result taskrail.NextResult, fallback string) string {
	if len(result.Warnings) == 0 {
		return fallback
	}
	var b strings.Builder
	fmt.Fprintln(&b, fallback)
	for _, warning := range result.Warnings {
		fmt.Fprintln(&b, warning.Message)
	}
	return strings.TrimRight(b.String(), "\n")
}
