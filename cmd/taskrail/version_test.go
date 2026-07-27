package main

import (
	"bytes"
	"strings"
	"testing"
)

// runRoot reports both streams as one blob, which is what most assertions want:
// they search the output for a fragment and do not care which stream carried it.
func runRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	stdout, stderr, err := runRootSplit(t, args...)
	return stdout + stderr, err
}

// runRootSplit keeps stdout and stderr apart, for the cases where the stream a
// message lands on is itself the contract — a warning must not corrupt
// machine-readable stdout.
func runRootSplit(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := newRootCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errOut.String(), err
}

func TestVersionSubcommand(t *testing.T) {
	out, err := runRoot(t, "version")
	if err != nil {
		t.Fatalf("version subcommand: %v", err)
	}
	if !strings.Contains(out, version) {
		t.Fatalf("expected output to contain version %q, got %q", version, out)
	}
}

func TestVersionFlag(t *testing.T) {
	out, err := runRoot(t, "--version")
	if err != nil {
		t.Fatalf("--version flag: %v", err)
	}
	if !strings.Contains(out, version) {
		t.Fatalf("expected output to contain version %q, got %q", version, out)
	}
}

func TestVersionDefaultNonEmpty(t *testing.T) {
	if strings.TrimSpace(version) == "" {
		t.Fatal("version fallback must not be empty")
	}
}
