package main

import (
	"fmt"
	"slices"
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

// commandResult is one command's finished work — a produced report or a
// completed write: the payload an agent reads, the text a human reads, and
// whether the completed report gates.
type commandResult struct {
	// shape is the companion result shape, such as "StatusResult". The boundary
	// holds it to the command's inventory entry.
	shape string
	value any
	text  string
	// exactText bypasses the ordinary report newline for content commands whose
	// text mode contract is byte-for-byte output.
	exactText bool
	// warnings are the advisories the command raised. They are honored even
	// alongside an error, because a failure after a warned-about step still has
	// to report the advisory that explains it.
	warnings []taskrail.Warning
	// gate is set when the command contract makes this completed report's
	// findings gating. The document stays a result envelope and the process
	// exits non-zero, identically in text and JSON mode. No writer sets it: a
	// write either commits and returns a result, or fails as an error envelope.
	gate error
}

// runCommand runs one command: it discovers the repository, does the work, and
// publishes the outcome in the mode the invocation asked for. Failures on either
// path become the command's registered error envelope, and every path returns
// the same error human mode would, so the two modes classify one outcome
// identically. Because produce runs before the mode is consulted, a writer
// persists exactly the same bytes either way.
func runCommand(cmd *cobra.Command, produce func(*taskrail.Service) (commandResult, error)) error {
	svc, err := serviceFromCmd(cmd)
	if err != nil {
		return publishMachineError(cmd, err, nil)
	}
	return finishCommand(cmd, svc, produce, true)
}

// runUnfencedCommand runs `recover`, the one command retained transaction state
// exists to be handed to. The service is constructed past the admission fence,
// and no post-operation fence re-check runs, because a successful recovery is
// exactly the snapshot change that check would refuse.
func runUnfencedCommand(cmd *cobra.Command, produce func(*taskrail.Service) (commandResult, error)) error {
	svc, err := taskrail.NewRecoveryService(".")
	if err != nil {
		return publishMachineError(cmd, err, nil)
	}
	return finishCommand(cmd, svc, produce, false)
}

func finishCommand(cmd *cobra.Command, svc *taskrail.Service, produce func(*taskrail.Service) (commandResult, error), fenced bool) error {
	result, err := produce(svc)
	if fenced {
		if recoveryErr := svc.CheckRecovery(); recoveryErr != nil {
			return publishMachineError(cmd, recoveryErr, result.warnings)
		}
	}
	// Advisories reach a human on stderr in either mode, including on the failure
	// path: an advisory that explains a failure is exactly the one worth keeping.
	// An agent reads the same advisories from the envelope's warning array, and
	// neither stream can contaminate the document on stdout.
	printWarnings(cmd, result.warnings)
	if err != nil {
		return publishMachineError(cmd, err, result.warnings)
	}
	if !machineJSONRequested(cmd) {
		var err error
		if result.exactText {
			_, err = fmt.Fprint(cmd.OutOrStdout(), result.text)
		} else {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), result.text)
		}
		if err != nil {
			return err
		}
		return result.gate
	}
	outcome := taskrail.MachineOutcome{
		Command:  machineCommandPath(cmd),
		Surface:  taskrail.MachineSurfaceStdout,
		Warnings: envelopeWarnings(cmd, result.warnings),
		Result:   &taskrail.MachineResult{Shape: result.shape, Value: result.value, Gated: result.gate != nil},
	}
	if err := taskrail.EmitMachineDocument(cmd.OutOrStdout(), outcome); err != nil {
		return err
	}
	return result.gate
}

// publishMachineError writes cause as this command's error envelope when the
// invocation asked for machine output, and always returns cause so the process
// exit stays the one human mode would use. The failure itself carries the facts
// the details report: the registered code, whether the complete semantic
// operation committed, and the managed paths a partial write left behind.
func publishMachineError(cmd *cobra.Command, cause error, warnings []taskrail.Warning) error {
	if !machineJSONRequested(cmd) {
		return cause
	}
	failure := taskrail.MachineFailureFor(cause)
	outcome := taskrail.MachineOutcome{
		Command:  machineCommandPath(cmd),
		Surface:  taskrail.MachineSurfaceStdout,
		Warnings: envelopeWarnings(cmd, warnings),
		Error: &taskrail.MachineError{
			Code:    failure.Code,
			Message: cause.Error(),
			Details: taskrail.MachineErrorDetails{
				Applied: failure.Applied,
				// append onto an empty slice, not slices.Clone: a required array
				// is `[]` on the wire, and cloning nil would leave it null.
				Violations: append([]taskrail.MachineViolation{}, failure.Violations...),
				Paths:      append([]string{}, failure.Paths...),
				Snapshots:  append([]taskrail.MachineSnapshot{}, failure.Snapshots...),
				Recovery:   failure.Recovery,
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
	return publishMachineError(cmd, taskrail.WithMachineErrorCode(taskrail.MachineCodeInvalidArguments, cause), nil)
}

// envelopeWarnings assembles the document's warning array: the ambient
// advisories every registry command carries after repository discovery, then the
// ones this command raised. It is the only place a warning becomes machine
// output, so a command cannot publish an advisory the envelope does not carry.
func envelopeWarnings(cmd *cobra.Command, raised []taskrail.Warning) []taskrail.MachineWarning {
	// Clone before appending: the ambient advisories are one memoized slice
	// shared by every reader of this invocation, and appending into its spare
	// capacity would rewrite what another reader already published.
	warnings := append(slices.Clone(ambientWarnings(cmd)), raised...)
	return taskrail.MachineWarnings(warnings)
}

// machineArgs publishes cobra's own argument rejections through the envelope.
// Cobra validates positionals after flag parsing, so `--json` is already known
// here — but it validates required flags and flag groups only after this hook
// has passed, and reports those failures with no hook of their own. Running
// cobra's validators here keeps their exact messages while making them reachable
// as envelopes; returning the failure aborts the run before cobra repeats it.
//
// Failing here also pre-empts the PersistentPreRun that reports skill skew, so
// the skew is reported first: a rejected invocation is exactly when a skill
// written by a newer binary is the explanation (root.go).
func machineArgs(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		for _, check := range []func() error{
			func() error { return validate(cmd, args) },
			cmd.ValidateRequiredFlags,
			cmd.ValidateFlagGroups,
		} {
			if err := check(); err != nil {
				warnOnSkillSkew(cmd, args)
				return publishSelectionError(cmd, err)
			}
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
	cmd.Flags().Bool("json", false, "print the versioned machine-result envelope")
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
