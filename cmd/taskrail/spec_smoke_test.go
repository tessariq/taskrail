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

// TestSpecActivateRepoints exercises the happy path through the CLI: with a
// second versioned spec present, `spec activate` moves STATE.md's active spec to
// it and the repo still validates.
func TestSpecActivateRepoints(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.2.0.md"), []byte("# Taskrail v0.2.0\n\n## Summary\n\nNext.\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	out, err := runRoot(t, "spec", "activate", "v0.2.0")
	if err != nil {
		t.Fatalf("spec activate: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "v0.2.0") {
		t.Fatalf("expected activation output naming the version, got %q", out)
	}

	state, err := os.ReadFile(filepath.Join(root, "planning", "STATE.md"))
	if err != nil {
		t.Fatalf("read STATE.md: %v", err)
	}
	if !strings.Contains(string(state), "active_spec_version: v0.2.0") ||
		!strings.Contains(string(state), "active_spec_path: specs/v0.2.0.md") {
		t.Fatalf("STATE.md not repointed:\n%s", state)
	}

	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after activation: %v (output %q)", err, out)
	}
}

// TestSpecActivateJSON verifies the machine-readable result carries the repoint
// and the re-run validation outcome.
func TestSpecActivateJSON(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.2.0.md"), []byte("# Taskrail v0.2.0\n\n## Summary\n\nNext.\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	out, err := runRoot(t, "spec", "activate", "v0.2.0", "--json")
	if err != nil {
		t.Fatalf("spec activate --json: %v (output %q)", err, out)
	}
	for _, want := range []string{`"active_spec_version": "v0.2.0"`, `"active_spec_path": "specs/v0.2.0.md"`, `"valid": true`} {
		if !strings.Contains(out, want) {
			t.Fatalf("json output missing %q:\n%s", want, out)
		}
	}
}

// TestSpecActivateRejectsMissing confirms a version with no spec file errors and
// leaves STATE.md untouched.
func TestSpecActivateRejectsMissing(t *testing.T) {
	root := setupRepo(t)
	statePath := filepath.Join(root, "planning", "STATE.md")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read STATE.md: %v", err)
	}

	if _, err := runRoot(t, "spec", "activate", "v0.2.0"); err == nil {
		t.Fatal("expected error activating a missing spec version")
	}

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("re-read STATE.md: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("rejected activation must not write STATE.md")
	}
}
