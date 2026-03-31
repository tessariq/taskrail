package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

type jsonOption struct {
	json bool
}

func serviceFromCmd(cmd *cobra.Command) (*taskrail.Service, error) {
	return taskrail.NewService(".")
}

func printResult(cmd *cobra.Command, asJSON bool, value any, fallback string) error {
	if !asJSON {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), fallback)
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}
