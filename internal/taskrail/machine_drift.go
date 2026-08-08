package taskrail

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Drift enforcement for the v0.5 machine API. The inventory in
// `machine_contract.go` states what each command may publish and the decoder in
// `machine_envelope.go` states what a common document must look like; this file
// makes a disagreement between either of them and the binary a deterministic
// failure instead of a document an agent has to discover is wrong.
//
// Two checks cover the two ways a producer drifts: one static over the set of
// documents the binary publishes, one per document about to be published.

// machineNoNonzeroResult is the companion registry's "no report-result exception"
// cell.
const machineNoNonzeroResult = "never"

// MachineExitPolicy classifies how one published document relates to the process
// exit status.
type MachineExitPolicy string

const (
	// MachineExitResultZero is the default: a result envelope exits zero and
	// every non-zero exit is an error envelope.
	MachineExitResultZero MachineExitPolicy = "result-zero"
	// MachineExitReportGated is the report-result exception: a completed report
	// whose findings gate may exit non-zero and stays a result envelope.
	MachineExitReportGated MachineExitPolicy = "report-gated"
	// MachineExitNotApplicable belongs to the loop result file, which is a
	// published document rather than the outcome of the process that wrote it.
	MachineExitNotApplicable MachineExitPolicy = "not-applicable"
)

// machineReportGatedCommands is the feature spec's closed list of commands whose
// completed report may exit non-zero. Any other companion row claiming a gate is
// drift, so the classification below cannot quietly grow an exception.
var machineReportGatedCommands = []string{"coverage", "loop", "task loop list", "validate"}

// ExitPolicy classifies the entry's companion "nonzero result" cell.
func (e MachineCommandEntry) ExitPolicy() MachineExitPolicy {
	switch {
	case e.Surface == MachineSurfaceResultFile:
		return MachineExitNotApplicable
	case e.NonzeroResult == machineNoNonzeroResult:
		return MachineExitResultZero
	default:
		return MachineExitReportGated
	}
}

// checkMachineEntryPolicy rejects an entry whose exit and coverage claims are not
// the ones the normative sources allow.
func checkMachineEntryPolicy(e MachineCommandEntry) error {
	switch e.JSONState {
	case MachineJSONEnvelope, MachineJSONInherited:
		if e.Origin != MachineOriginConstructed {
			return fmt.Errorf("command %q publishes %q but is not constructed", e.Command, e.JSONState)
		}
	case MachineJSONAbsent:
	default:
		return fmt.Errorf("command %q has unknown JSON state %q", e.Command, e.JSONState)
	}
	if e.Surface == MachineSurfaceResultFile {
		if e.NonzeroResult == machineNoNonzeroResult {
			return fmt.Errorf("command %q publishes a result file, which has no report-result exit exception to waive", e.Command)
		}
		return nil
	}
	gated := slices.Contains(machineReportGatedCommands, e.Command)
	switch {
	case gated && e.ExitPolicy() != MachineExitReportGated:
		return fmt.Errorf("command %q gates its report but claims no report-result exit exception", e.Command)
	case !gated && e.ExitPolicy() == MachineExitReportGated:
		return fmt.Errorf("command %q claims report-result exit exception %q outside the closed gating list", e.Command, e.NonzeroResult)
	}
	return nil
}

// MachineRegistration is one machine document the binary publishes today. The
// CLI derives these from its own command tree, so a registration is a fact about
// the binary rather than a second hand-maintained contract.
type MachineRegistration struct {
	Command string
	Surface MachineSurface
}

func (r MachineRegistration) String() string {
	return r.Command + " " + string(r.Surface)
}

// CheckMachineRegistrations reports every disagreement between the documents the
// binary publishes and the JSON-capable half of the inventory. Diagnostics are
// ordered by registration and then by inventory position, so repeated runs
// report the same drift in the same order.
func CheckMachineRegistrations(registrations []MachineRegistration) error {
	sorted := slices.Clone(registrations)
	slices.SortFunc(sorted, func(a, b MachineRegistration) int {
		return strings.Compare(a.String(), b.String())
	})

	var problems []error
	registered := map[MachineRegistration]bool{}
	for _, registration := range sorted {
		if registered[registration] {
			problems = append(problems, fmt.Errorf("the CLI registers document %q twice", registration))
			continue
		}
		registered[registration] = true
		entry, ok := MachineCommandEntryFor(registration.Command, registration.Surface)
		switch {
		case !ok:
			problems = append(problems, fmt.Errorf("the CLI publishes %q with no v0.5 machine inventory entry", registration))
		case entry.JSONState == MachineJSONAbsent:
			problems = append(problems, fmt.Errorf("the inventory says %q publishes no machine document yet, but the CLI publishes it", registration))
		}
	}
	for _, entry := range machineInventory {
		registration := MachineRegistration{Command: entry.Command, Surface: entry.Surface}
		if entry.JSONState != MachineJSONAbsent && !registered[registration] {
			problems = append(problems, fmt.Errorf("%s is inventoried as publishing %q but the CLI publishes no %q document", entry.CompanionRow, entry.JSONState, registration))
		}
	}
	return errors.Join(problems...)
}

// MachinePublication is one document a producer is about to hand to an agent,
// together with the result shape it built and the exit status it would use.
type MachinePublication struct {
	Command string
	Surface MachineSurface
	// Result names the companion result shape the producer built, and is empty
	// for an error envelope. It is the producer's own claim: the common decoder
	// leaves the result payload as exact bytes, so the check holds the claim to
	// the inventory rather than decoding the shape, which each command owns.
	Result   string
	ExitCode int
	Document []byte
}

// CheckMachinePublication holds one publication to the strict common decoder and
// to the inventory entry it claims. It reports the first disagreement, so the
// diagnostic names one cause rather than a cascade. An entry that still emits its
// inherited shape may call it, because a producer needs the check while it moves
// onto the envelope; an entry that publishes nothing at all may not.
func CheckMachinePublication(p MachinePublication) error {
	registration := MachineRegistration{Command: p.Command, Surface: p.Surface}
	entry, ok := MachineCommandEntryFor(p.Command, p.Surface)
	if !ok {
		return fmt.Errorf("no schema-1 machine contract for %q", registration)
	}
	if entry.JSONState == MachineJSONAbsent {
		return fmt.Errorf("%q publishes no machine document yet, so it must publish no schema-1 document", registration)
	}
	envelope, err := DecodeMachineEnvelope(p.Document)
	if err != nil {
		return fmt.Errorf("command %q publishes an invalid schema-1 document: %w", p.Command, err)
	}
	if envelope.Command != p.Command {
		return fmt.Errorf("command %q publishes a document naming command %q", p.Command, envelope.Command)
	}
	for _, warning := range envelope.Warnings {
		if !slices.Contains(entry.Warnings, warning.Code) {
			return fmt.Errorf("command %q publishes warning %q, which its contract does not allow", p.Command, warning.Code)
		}
	}
	if envelope.Error != nil {
		return checkMachineErrorPublication(p, entry, envelope.Error.Code)
	}
	return checkMachineResultPublication(p, entry)
}

func checkMachineErrorPublication(p MachinePublication, entry MachineCommandEntry, code string) error {
	if p.Result != "" {
		return fmt.Errorf("command %q publishes an error envelope naming result shape %q", p.Command, p.Result)
	}
	if !slices.Contains(entry.Errors, code) {
		return fmt.Errorf("command %q publishes error %q, which its contract does not allow", p.Command, code)
	}
	if entry.ExitPolicy() != MachineExitNotApplicable && p.ExitCode == 0 {
		return fmt.Errorf("command %q exits 0 with an error envelope", p.Command)
	}
	return nil
}

func checkMachineResultPublication(p MachinePublication, entry MachineCommandEntry) error {
	if p.Result == "" {
		return fmt.Errorf("command %q publishes a result envelope naming no result shape", p.Command)
	}
	if !slices.Contains(entry.Results, p.Result) {
		return fmt.Errorf("command %q publishes result shape %q, which its contract does not name", p.Command, p.Result)
	}
	if p.ExitCode != 0 && entry.ExitPolicy() == MachineExitResultZero {
		return fmt.Errorf("command %q exits %d with a result envelope, which its contract never gates", p.Command, p.ExitCode)
	}
	return nil
}
