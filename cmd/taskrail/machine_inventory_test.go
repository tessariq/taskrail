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

// leafCommandPaths returns the canonical path of every command the CLI currently
// constructs, so the inventory's constructed/planned split is checked against the
// binary instead of a hand-maintained list.
func leafCommandPaths(cmd *cobra.Command, prefix string) []string {
	var paths []string
	for _, child := range cmd.Commands() {
		name := child.Name()
		if nonMachineCommands[name] {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + " " + name
		}
		if len(child.Commands()) == 0 {
			paths = append(paths, path)
			continue
		}
		paths = append(paths, leafCommandPaths(child, path)...)
	}
	return paths
}

func TestMachineInventoryOriginMatchesConstructedCommands(t *testing.T) {
	constructed := map[string]bool{}
	for _, path := range leafCommandPaths(newRootCmd(), "") {
		constructed[path] = true
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
