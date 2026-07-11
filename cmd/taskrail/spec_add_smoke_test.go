package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSpecAddScaffoldsAndValidates exercises the happy path through the CLI: a
// new version is scaffolded, the reading order names it, STATE.md is untouched
// (add does not activate), and the repo still validates.
func TestSpecAddScaffoldsAndValidates(t *testing.T) {
	root := setupRepo(t)
	statePath := filepath.Join(root, "planning", "STATE.md")
	stateBefore, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read STATE.md: %v", err)
	}

	out, err := runRoot(t, "spec", "add", "v0.2.0")
	if err != nil {
		t.Fatalf("spec add: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "v0.2.0") {
		t.Fatalf("expected output naming the version, got %q", out)
	}

	if !fileExistsTest(filepath.Join(root, "specs", "v0.2.0.md")) {
		t.Fatal("spec add did not create the scaffold file")
	}
	readme, err := os.ReadFile(filepath.Join(root, "specs", "README.md"))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	if !strings.Contains(string(readme), "`specs/v0.2.0.md`") {
		t.Fatalf("reading order not updated:\n%s", readme)
	}

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("re-read STATE.md: %v", err)
	}
	if string(stateBefore) != string(after) {
		t.Fatal("spec add must not write STATE.md")
	}

	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate after scaffold: %v (output %q)", err, out)
	}
}

// TestSpecAddCoverageReportsNA is the end-to-end T-059 alignment: after
// activating a freshly scaffolded spec, coverage reports N/A because the
// scaffold has zero coverable areas — no false gap, no hollow 0%.
func TestSpecAddCoverageReportsNA(t *testing.T) {
	setupRepo(t)
	if out, err := runRoot(t, "spec", "add", "v0.2.0"); err != nil {
		t.Fatalf("spec add: %v (output %q)", err, out)
	}
	if out, err := runRoot(t, "spec", "activate", "v0.2.0"); err != nil {
		t.Fatalf("spec activate: %v (output %q)", err, out)
	}

	out, err := runRoot(t, "coverage")
	if err != nil {
		t.Fatalf("coverage: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "N/A") {
		t.Fatalf("expected coverage N/A for area-free scaffold, got %q", out)
	}
}

// TestSpecAddRejectsExistingVersion confirms an existing version errors through
// the CLI and leaves the existing spec file unchanged.
func TestSpecAddRejectsExistingVersion(t *testing.T) {
	root := setupRepo(t)
	specPath := filepath.Join(root, "specs", "v0.1.0.md")
	before, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}

	if _, err := runRoot(t, "spec", "add", "v0.1.0"); err == nil {
		t.Fatal("expected error adding an existing spec version")
	}

	after, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("re-read spec: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("rejected add must not overwrite the existing spec")
	}
}

// TestSpecAddRejectsBadVersion confirms a non-conforming version errors.
func TestSpecAddRejectsBadVersion(t *testing.T) {
	setupRepo(t)
	if _, err := runRoot(t, "spec", "add", "garbage"); err == nil {
		t.Fatal("expected error for a non-conforming version")
	}
}

func fileExistsTest(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
