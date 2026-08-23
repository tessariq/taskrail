package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/tessariq/taskrail/internal/taskrail"
)

// nonMachineCommands are leaves that are deliberately outside the v0.5 machine
// API: they emit shell or version text rather than a schema-versioned envelope.
var nonMachineCommands = map[string]bool{
	"version":    true,
	"completion": true,
	"help":       true,
}

// constructedCommand pairs one command the CLI builds with its canonical path.
type constructedCommand struct {
	path string
	cmd  *cobra.Command
}

// constructedCommands walks the command tree once, so both the inventory's
// constructed/planned split and the set of documents it says the binary
// publishes are checked against the binary instead of a hand-maintained list.
func constructedCommands(cmd *cobra.Command, prefix string) []constructedCommand {
	var found []constructedCommand
	for _, child := range cmd.Commands() {
		if nonMachineCommands[child.Name()] {
			continue
		}
		path := child.Name()
		if prefix != "" {
			path = prefix + " " + child.Name()
		}
		found = append(found, constructedCommand{path: path, cmd: child})
		found = append(found, constructedCommands(child, path)...)
	}
	return found
}

func TestMachineRegistrationsMatchJSONCapableCommands(t *testing.T) {
	var registrations []taskrail.MachineRegistration
	for _, constructed := range constructedCommands(newRootCmd(), "") {
		if constructed.cmd.Flags().Lookup("json") == nil {
			continue
		}
		registrations = append(registrations, taskrail.MachineRegistration{
			Command: constructed.path,
			Surface: taskrail.MachineSurfaceStdout,
		})
	}
	registrations = append(registrations, taskrail.MachineRegistration{Command: "loop", Surface: taskrail.MachineSurfaceResultFile})
	if len(registrations) == 0 {
		t.Fatal("the CLI publishes no --json command")
	}
	if err := taskrail.CheckMachineRegistrations(registrations); err != nil {
		t.Fatalf("the CLI's --json commands drifted from the v0.5 inventory:\n%v", err)
	}
}

func TestMachineInventoryOriginMatchesConstructedCommands(t *testing.T) {
	constructed := map[string]bool{}
	for _, leaf := range constructedCommands(newRootCmd(), "") {
		if len(leaf.cmd.Commands()) == 0 {
			constructed[leaf.path] = true
		}
	}
	if len(constructed) == 0 {
		t.Fatal("the CLI constructs no commands")
	}

	inventoried := map[string]bool{}
	for _, entry := range taskrail.MachineCommandInventory() {
		inventoried[entry.Command] = true
		wantConstructed := entry.Origin == taskrail.MachineOriginConstructed
		if got := constructed[entry.Command]; got != wantConstructed {
			t.Errorf("%s is classified %q but the CLI constructed=%v", entry.CompanionRow, entry.Origin, got)
		}
	}
	for path := range constructed {
		if !inventoried[path] {
			t.Errorf("the CLI constructs %q with no v0.5 machine inventory entry", path)
		}
	}
}
