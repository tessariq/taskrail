package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func exeName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func seed(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o755); err != nil {
		t.Fatalf("seed %s: %v", p, err)
	}
	return p
}

// The install guard passes when the directory it just built into is the one a
// bare `taskrail` resolves from.
func TestRunAcceptsReachableBuild(t *testing.T) {
	dir := t.TempDir()
	built := seed(t, dir, exeName("taskrail"), "working-tree build")
	t.Setenv("TASKRAIL", "")
	t.Setenv("PATH", dir)

	if err := run([]string{built}); err != nil {
		t.Errorf("run must accept a build reachable as `taskrail`; got %v", err)
	}
}

// The core of AC-1: building succeeded but left the caller no better off,
// because a bare `taskrail` still runs a different binary. That must fail loudly
// and name the two working fixes.
func TestRunRejectsUnreachableBuild(t *testing.T) {
	buildDir := t.TempDir()
	built := seed(t, buildDir, exeName("taskrail"), "working-tree build")
	pathDir := t.TempDir()
	seed(t, pathDir, exeName("taskrail"), "installed release")
	t.Setenv("TASKRAIL", "")
	t.Setenv("PATH", pathDir)

	err := run([]string{built})
	if err == nil {
		t.Fatal("run must fail when the build is not what a bare `taskrail` resolves to")
	}
	for _, want := range []string{"mise run setup", "TASKRAIL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("failure %q must name the fix %q", err, want)
		}
	}
}

// With no taskrail on PATH at all the build is equally unreachable, and the
// remedies are the same two.
func TestRunRejectsWhenNothingOnPath(t *testing.T) {
	built := seed(t, t.TempDir(), exeName("taskrail"), "working-tree build")
	t.Setenv("TASKRAIL", "")
	t.Setenv("PATH", t.TempDir())

	err := run([]string{built})
	if err == nil {
		t.Fatal("run must fail when no taskrail is on PATH")
	}
	for _, want := range []string{"mise run setup", "TASKRAIL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("failure %q must name the fix %q", err, want)
		}
	}
}

// An explicit TASKRAIL override is one of the two sanctioned fixes, so PATH
// resolution is no longer what decides which binary runs: the guard must not
// fail a contributor who already took that route.
func TestRunAcceptsExplicitOverride(t *testing.T) {
	built := seed(t, t.TempDir(), exeName("taskrail"), "working-tree build")
	t.Setenv("PATH", t.TempDir()) // nothing resolvable
	t.Setenv("TASKRAIL", built)

	if err := run([]string{built}); err != nil {
		t.Errorf("run must defer to an explicit TASKRAIL override; got %v", err)
	}
}

func TestRunRejectsBadArgs(t *testing.T) {
	if err := run(nil); err == nil {
		t.Error("run must reject a missing path argument")
	}
}

// A missing build output is a real failure, not a PATH problem: the guard must
// surface it rather than reporting the shadowing remedy.
func TestRunReportsMissingBuild(t *testing.T) {
	dir := t.TempDir()
	seed(t, dir, exeName("taskrail"), "installed release")
	t.Setenv("TASKRAIL", "")
	t.Setenv("PATH", dir)

	err := run([]string{filepath.Join(t.TempDir(), exeName("taskrail"))})
	if err == nil {
		t.Fatal("run must fail when the build output does not exist")
	}
	if strings.Contains(err.Error(), "mise run setup") {
		t.Errorf("a missing build output must not be reported as a PATH problem; got %q", err)
	}
}
