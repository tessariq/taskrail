package main

import "github.com/spf13/cobra"

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
			return printResult(cmd, opt.json, result, fallback)
		},
	}
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	return cmd
}
