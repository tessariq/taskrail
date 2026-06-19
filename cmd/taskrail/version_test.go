package main

import (
	"bytes"
	"strings"
	"testing"
)

func runRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
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
