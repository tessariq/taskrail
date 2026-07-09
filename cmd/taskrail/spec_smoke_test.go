package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSpecParentPrintsUsage verifies the shared spec parent command exists,
// prints usage when invoked without a subcommand, and is read-only: it writes
// nothing to the tracked-work state and leaves the working tree clean.
func TestSpecParentPrintsUsage(t *testing.T) {
	root := setupRepo(t)

	statePath := filepath.Join(root, "planning", "STATE.md")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read STATE.md: %v", err)
	}

	out, err := runRoot(t, "spec")
	if err != nil {
		t.Fatalf("spec parent: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "Usage:") || !strings.Contains(out, "taskrail spec") {
		t.Fatalf("expected usage output naming the spec command, got %q", out)
	}

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("re-read STATE.md: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("spec parent must be read-only but STATE.md changed")
	}
}

// TestSpecParentRejectsUnexpectedArg locks the cobra.NoArgs guard: a stray
// positional (an unknown subcommand) must error rather than silently rendering
// help, so a future accidental relaxation of the guard is caught.
func TestSpecParentRejectsUnexpectedArg(t *testing.T) {
	if _, err := runRoot(t, "spec", "bogus"); err == nil {
		t.Fatal("expected error for unexpected positional argument to spec")
	}
}

// TestSpecParentHelpFlag verifies `spec --help` also renders usage without error.
func TestSpecParentHelpFlag(t *testing.T) {
	out, err := runRoot(t, "spec", "--help")
	if err != nil {
		t.Fatalf("spec --help: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "taskrail spec") {
		t.Fatalf("expected help output naming the spec command, got %q", out)
	}
}
