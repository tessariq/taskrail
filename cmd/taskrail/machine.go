package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

// The CLI edge of the v0.5 machine API. `internal/taskrail` owns the envelope's
// bytes and the contract it is held to; this file owns how a cobra command
// reaches it — one publication point for results, gated reports, and failures,
// plus the two hooks that run before a command body (flag parsing and positional
// validation) and would otherwise fail with prose-only stderr after the command
// was already selected.

// report is one read-only command's finished work: the payload an agent reads,
// the text a human reads, and whether the completed report gates.
type report struct {
	// shape is the companion result shape, such as "StatusResult". The boundary
	// holds it to the command's inventory entry.
	shape string
	value any
	text  string
	// gate is set when the command contract makes this completed report's
	// findings gating. The document stays a result envelope and the process
	// exits non-zero, identically in text and JSON mode.
	gate error
}

// runReport runs one read-only report command: it discovers the repository,
// produces the report, and publishes it in the mode the invocation asked for.
// Failures on either path become the command's registered error envelope, and
// every path returns the same error human mode would, so the two modes classify
// one outcome identically.
func runReport(cmd *cobra.Command, produce func(*taskrail.Service) (report, error)) error {
	svc, err := serviceFromCmd(cmd)
	if err != nil {
		return publishMachineError(cmd, err)
	}
	rep, err := produce(svc)
	if err != nil {
		return publishMachineError(cmd, err)
	}
	if !machineJSONRequested(cmd) {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), rep.text); err != nil {
			return err
		}
		return rep.gate
	}
	outcome := taskrail.MachineOutcome{
		Command:  machineCommandPath(cmd),
		Surface:  taskrail.MachineSurfaceStdout,
		Warnings: []taskrail.MachineWarning{},
		Result:   &taskrail.MachineResult{Shape: rep.shape, Value: rep.value, Gated: rep.gate != nil},
	}
	if err := taskrail.EmitMachineDocument(cmd.OutOrStdout(), outcome); err != nil {
		return err
	}
	return rep.gate
}

// publishMachineError writes cause as this command's error envelope when the
// invocation asked for machine output, and always returns cause so the process
// exit stays the one human mode would use. Read-only commands publish nothing,
// so `applied` is false and the detail collections stay empty.
func publishMachineError(cmd *cobra.Command, cause error) error {
	if !machineJSONRequested(cmd) {
		return cause
	}
	outcome := taskrail.MachineOutcome{
		Command:  machineCommandPath(cmd),
		Surface:  taskrail.MachineSurfaceStdout,
		Warnings: []taskrail.MachineWarning{},
		Error: &taskrail.MachineError{
			Code:    taskrail.MachineErrorCodeFor(cause),
			Message: cause.Error(),
			Details: taskrail.MachineErrorDetails{
				Violations: []taskrail.MachineViolation{},
				Paths:      []string{},
				Snapshots:  []taskrail.MachineSnapshot{},
			},
		},
	}
	if err := taskrail.EmitMachineDocument(cmd.OutOrStdout(), outcome); err != nil {
		return err
	}
	return cause
}

// publishSelectionError publishes an argument failure cobra detects between
// selecting a command and running its body. It is limited to commands the
// inventory says already publish the envelope, so a command still on its
// inherited shape keeps failing exactly as it does today.
//
// As a flag-error handler it sees only the flags pflag parsed before the
// rejected one, so `status --json --nope` publishes an envelope while
// `status --nope --json` cannot: the mode was never selected. Recovering the
// intent would mean re-scanning the raw argument vector behind pflag's back,
// which is a worse trade than a prose failure for a malformed invocation.
func publishSelectionError(cmd *cobra.Command, cause error) error {
	if !machinePublishesEnvelope(cmd) {
		return cause
	}
	return publishMachineError(cmd, taskrail.WithMachineErrorCode(taskrail.MachineCodeInvalidArguments, cause))
}

// machineArgs publishes a positional-argument rejection through the envelope.
// Cobra validates positionals after flag parsing, so `--json` is already known
// here.
func machineArgs(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := validate(cmd, args); err != nil {
			return publishSelectionError(cmd, err)
		}
		return nil
	}
}

// invalidArgumentsf builds a command-detected argument failure already carrying
// the code its envelope publishes.
func invalidArgumentsf(format string, a ...any) error {
	return taskrail.WithMachineErrorCode(taskrail.MachineCodeInvalidArguments, fmt.Errorf(format, a...))
}

// addMachineJSONFlag registers `--json` for a command that publishes the common
// envelope. The boundary reads the flag back from cobra, so no binding variable
// has to be kept in step with it.
func addMachineJSONFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("json", false, "print machine-readable output")
}

// machineJSONRequested reports whether this invocation asked for machine output.
func machineJSONRequested(cmd *cobra.Command) bool {
	flag := cmd.Flags().Lookup("json")
	return flag != nil && flag.Value.String() == "true"
}

// machinePublishesEnvelope reports whether this invocation both asked for
// machine output and names a command the inventory records as publishing the
// common envelope today.
func machinePublishesEnvelope(cmd *cobra.Command) bool {
	if !machineJSONRequested(cmd) {
		return false
	}
	entry, ok := taskrail.MachineCommandEntryFor(machineCommandPath(cmd), taskrail.MachineSurfaceStdout)
	return ok && entry.JSONState == taskrail.MachineJSONEnvelope
}

// machineCommandPath is the canonical command path the envelope names: the
// selected command's path after the executable. Cobra resolves an alias to its
// command's own name, so the path is normalized by construction.
func machineCommandPath(cmd *cobra.Command) string {
	return strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name()+" ")
}
