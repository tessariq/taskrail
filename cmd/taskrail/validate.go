package main

import (
	"errors"

	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	var opt jsonOption
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate Taskrail structure, state, and tasks",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			result, err := svc.Validate()
			if err != nil {
				return err
			}
			fallback := "state valid"
			if !result.Valid {
				fallback = "state invalid"
			}
			if err := printResult(cmd, opt.json, result, fallback); err != nil {
				return err
			}
			if !result.Valid {
				return errors.New("state invalid")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&opt.json, "json", false, "print machine-readable output")
	return cmd
}
