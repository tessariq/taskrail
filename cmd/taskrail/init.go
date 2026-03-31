package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize Taskrail structure in the current repository",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := serviceFromCmd(cmd)
			if err != nil {
				return err
			}
			if err := svc.Init(); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "initialized taskrail structure")
			return err
		},
	}
}
