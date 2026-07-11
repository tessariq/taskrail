package toolchain_test

import (
	"regexp"
	"strings"
	"testing"
)

// This repository dogfoods coverage --min (T-066) as a CI gate: the v0.3.0 spec
// decomposes to full self-coverage, so a future spec area shipped without a
// linked task must red a push, not just the manual pre-release gate (T-061,
// T-079). The gate lives only on the coverage command's exit code; validate
// stays advisory (a coverage gap never fails validate).

// The coverage:gate Taskfile target must run `coverage --min 100` against the
// active spec so the gate fails when self-coverage drops below 100%. Asserting
// --min 100 pins the threshold to the current clean self-coverage; a narrower
// --area or a spec override would scope the gate away from the whole active spec.
func TestTaskfileDefinesCoverageGate(t *testing.T) {
	root := repoRoot(t)
	taskfile := readFile(t, root, "Taskfile.yml")

	if !strings.Contains(taskfile, "coverage:gate:") {
		t.Fatal("Taskfile.yml must define a coverage:gate target for CI self-coverage gating")
	}
	gateRe := regexp.MustCompile(`coverage\s+--min\s+100\b`)
	if !gateRe.MatchString(taskfile) {
		t.Error("coverage:gate must run `coverage --min 100` against the active spec")
	}
	// The gate must not narrow to a single area; it gates the whole active spec.
	if strings.Contains(taskfile, "coverage --min 100 --area") {
		t.Error("coverage:gate must gate the whole active spec, not a single --area")
	}
}

// CI must invoke the gate through the Taskfile target (per T-074's `task ...`
// contract), not an inline coverage invocation, so Taskfile.yml stays the single
// source of the gate command for local and CI alike.
func TestCIInvokesCoverageGateViaTask(t *testing.T) {
	root := repoRoot(t)
	runs := ciRunSteps(readFile(t, root, ".github/workflows/ci.yml"))

	taskRe := regexp.MustCompile(`^task\s+coverage:gate\b`)
	for _, cmd := range runs {
		if taskRe.MatchString(cmd) {
			return
		}
	}
	t.Errorf("ci.yml must run `task coverage:gate` to gate self-coverage; run steps: %v", runs)
}
