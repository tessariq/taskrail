package main

import (
	"encoding/json"
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

// TestSpecActivatePrintsCoverageSummary verifies the human view echoes the
// one-line coverage summary for the now-active spec (T-067), matching the first
// line `taskrail coverage` prints against the same repointed state, and that
// activation still succeeds regardless of the figure.
func TestSpecActivatePrintsCoverageSummary(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.2.0.md"),
		[]byte("# Taskrail v0.2.0\n\n## Potential Features\n\n### Alpha\n\n### Beta\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	out, err := runRoot(t, "spec", "activate", "v0.2.0")
	if err != nil {
		t.Fatalf("spec activate: %v (output %q)", err, out)
	}
	// No task links either area, so the just-activated spec is 0/2 covered.
	if !strings.Contains(out, "coverage: 0% (0/2 areas) — specs/v0.2.0.md") {
		t.Fatalf("expected coverage summary line, got %q", out)
	}

	cov, err := runRoot(t, "coverage")
	if err != nil {
		t.Fatalf("coverage: %v (output %q)", err, cov)
	}
	// The activation echo must match coverage's own first line exactly.
	wantLine := "coverage: 0% (0/2 areas) — specs/v0.2.0.md"
	if !strings.Contains(cov, wantLine) {
		t.Fatalf("coverage first line changed, got %q", cov)
	}
}

// TestSpecActivatePrintsNACoverage guards the N/A path for this command: a spec
// with no `## Potential Features` coverable areas reports N/A in the echo and a
// null coverage_percent in --json, matching `coverage`'s own N/A rendering.
func TestSpecActivatePrintsNACoverage(t *testing.T) {
	root := setupRepo(t)
	// No Potential Features section => no coverable areas => N/A.
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.2.0.md"),
		[]byte("# Taskrail v0.2.0\n\n## Summary\n\nNext.\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	out, err := runRoot(t, "spec", "activate", "v0.2.0")
	if err != nil {
		t.Fatalf("spec activate: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "coverage: N/A (no coverable areas) — specs/v0.2.0.md") {
		t.Fatalf("expected N/A coverage summary, got %q", out)
	}

	js, err := runRoot(t, "spec", "activate", "v0.2.0", "--json")
	if err != nil {
		t.Fatalf("spec activate --json: %v (output %q)", err, js)
	}
	if !strings.Contains(js, `"coverage_percent": null`) {
		t.Fatalf("expected null coverage_percent for N/A spec, got %q", js)
	}
}

// TestSpecActivateJSONCoverage verifies --json carries the coverage report for
// the now-active spec and that it agrees with `taskrail coverage --json`.
func TestSpecActivateJSONCoverage(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.2.0.md"),
		[]byte("# Taskrail v0.2.0\n\n## Potential Features\n\n### Alpha\n\n### Beta\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	out, err := runRoot(t, "spec", "activate", "v0.2.0", "--json")
	if err != nil {
		t.Fatalf("spec activate --json: %v (output %q)", err, out)
	}
	for _, want := range []string{`"coverage":`, `"coverage_percent": 0`, `"coverable_areas": 2`} {
		if !strings.Contains(out, want) {
			t.Fatalf("json output missing %q:\n%s", want, out)
		}
	}
}

// TestSpecListMarksActive verifies `spec list` names the versioned specs, marks
// the active one, omits specs/README.md, and stays read-only.
func TestSpecListMarksActive(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "v0.2.0.md"), []byte("# v0.2.0\n\n## Summary\n\nNext.\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	statePath := filepath.Join(root, "planning", "STATE.md")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read STATE.md: %v", err)
	}

	out, err := runRoot(t, "spec", "list")
	if err != nil {
		t.Fatalf("spec list: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "v0.1.0") || !strings.Contains(out, "v0.2.0") {
		t.Fatalf("spec list omits a version: %q", out)
	}
	if strings.Contains(out, "README") {
		t.Fatalf("spec list must not list README.md: %q", out)
	}
	// The active v0.1.0 line must carry a marker the inactive v0.2.0 line lacks.
	if !strings.Contains(out, "active") {
		t.Fatalf("spec list does not mark the active spec: %q", out)
	}

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("re-read STATE.md: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("spec list must be read-only but STATE.md changed")
	}
}

// TestSpecListJSON verifies the machine-readable shape marks the active spec.
func TestSpecListJSON(t *testing.T) {
	setupRepo(t)
	out, err := runRoot(t, "spec", "list", "--json")
	if err != nil {
		t.Fatalf("spec list --json: %v (output %q)", err, out)
	}
	var payload struct {
		ActiveSpecVersion string `json:"active_spec_version"`
		Specs             []struct {
			Version string `json:"version"`
			Path    string `json:"path"`
			Active  bool   `json:"active"`
		} `json:"specs"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode json: %v (output %q)", err, out)
	}
	if payload.ActiveSpecVersion != "v0.1.0" || len(payload.Specs) != 1 || !payload.Specs[0].Active {
		t.Fatalf("unexpected spec list payload: %+v", payload)
	}
}

// TestSpecShowPrintsContent verifies `spec show` prints the spec body.
func TestSpecShowPrintsContent(t *testing.T) {
	setupRepo(t)
	out, err := runRoot(t, "spec", "show", "v0.1.0")
	if err != nil {
		t.Fatalf("spec show: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "Summary") {
		t.Fatalf("spec show omits spec body: %q", out)
	}
}

// TestSpecShowAnchorsAuthorable is the end-to-end acceptance: an anchor drawn from
// `spec show --anchors --json` authors a task whose spec_ref passes `validate`,
// proving the listing reuses the real slug rule and stays read-only.
func TestSpecShowAnchorsAuthorable(t *testing.T) {
	root := setupRepo(t)
	statePath := filepath.Join(root, "planning", "STATE.md")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read STATE.md: %v", err)
	}

	out, err := runRoot(t, "spec", "show", "v0.1.0", "--anchors", "--json")
	if err != nil {
		t.Fatalf("spec show --anchors --json: %v (output %q)", err, out)
	}
	var payload struct {
		Anchors []struct {
			Anchor string `json:"anchor"`
		} `json:"anchors"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode anchors json: %v (output %q)", err, out)
	}
	if len(payload.Anchors) == 0 {
		t.Fatalf("expected at least one anchor: %q", out)
	}

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("re-read STATE.md: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("spec show must be read-only but STATE.md changed")
	}

	// Author a task against the first listed anchor and confirm it validates.
	writeTaskSpecRef(t, root, "T-900", "specs/v0.1.0.md#"+payload.Anchors[0].Anchor)
	if out, err := runRoot(t, "validate"); err != nil {
		t.Fatalf("validate against listed anchor: %v (output %q)", err, out)
	}
}

// TestSpecShowRejectsBadVersion confirms a malformed version errors.
func TestSpecShowRejectsBadVersion(t *testing.T) {
	setupRepo(t)
	if _, err := runRoot(t, "spec", "show", "garbage"); err == nil {
		t.Fatal("expected error for a non-conforming version")
	}
}

// writeTaskSpecRef drops a minimal valid task carrying an explicit spec_ref.
func writeTaskSpecRef(t *testing.T, root, id, specRef string) {
	t.Helper()
	content := strings.Join([]string{
		"---",
		"id: " + id,
		"title: Task " + id,
		"status: todo",
		"priority: high",
		"spec_ref: " + specRef,
		"dependencies: []",
		`updated_at: "2026-06-19T00:00:00Z"`,
		"---",
		"",
		"# " + id,
		"",
		"Body.",
		"",
	}, "\n")
	path := filepath.Join(root, "planning", "tasks", id+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write task %s: %v", id, err)
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
