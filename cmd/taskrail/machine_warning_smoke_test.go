package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// Every advisory an agent can act on reaches it through the envelope's warning
// array, and never as a command-local result field
// (specs/v0.5.0.md#uniform-agent-machine-results).

// envelopeWarningCodes returns the codes the published document carries.
func envelopeWarningCodes(t *testing.T, stdout string) []string {
	t.Helper()
	warnings := decodeEnvelope(t, stdout).Warnings
	codes := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		codes = append(codes, warning.Code)
	}
	return codes
}

func TestSelectionWarningsPublishInTheEnvelope(t *testing.T) {
	root := setupRepo(t)
	activateSecondSpec(t, root)
	writeTask(t, root, "T-100", "todo", "")

	stdout, _, err := runRootSplit(t, "next", "--include-off-spec", "--json")
	if err != nil {
		t.Fatalf("next --include-off-spec --json: %v", err)
	}
	warnings := decodeEnvelope(t, stdout).Warnings
	if len(warnings) != 1 || warnings[0].Code != "selected_off_spec" {
		t.Fatalf("envelope warnings = %+v, want one selected_off_spec", warnings)
	}
	got := warnings[0]
	if got.TaskID != "T-100" || got.SpecRef != "specs/v0.1.0.md#summary" || got.ActiveSpecPath != "specs/v0.2.0.md" {
		t.Fatalf("selection warning members = %+v", got)
	}
	if got.Message == "" {
		t.Fatal("selection warning carries no message")
	}

	// The same advisory must not also ride inside the result payload.
	var payload map[string]json.RawMessage
	decodeMachineResult(t, stdout, &payload)
	if _, present := payload["warnings"]; present {
		t.Fatalf("NextResult still publishes a command-local warnings field: %v", payload)
	}
}

func TestStatusPublishesSkippedSelectionWarningsInTheEnvelope(t *testing.T) {
	root := setupRepo(t)
	activateSecondSpec(t, root)
	writeTask(t, root, "T-100", "todo", "")

	stdout, _, err := runRootSplit(t, "status", "--json")
	if err != nil {
		t.Fatalf("status --json: %v", err)
	}
	if codes := envelopeWarningCodes(t, stdout); len(codes) != 1 || codes[0] != "skipped_non_active_spec" {
		t.Fatalf("envelope warning codes = %v, want one skipped_non_active_spec", codes)
	}
	var payload struct {
		Next map[string]json.RawMessage `json:"next"`
	}
	decodeMachineResult(t, stdout, &payload)
	if _, present := payload.Next["warnings"]; present {
		t.Fatalf("StatusResult.next still publishes a command-local warnings field: %v", payload.Next)
	}
}

func TestTaskNewPublishesSlugWarningInTheEnvelope(t *testing.T) {
	setupRepo(t)

	stdout, stderr, err := runRootSplit(t, "task", "new", "--slug", "!!!", "--spec-ref", "specs/v0.1.0.md#summary", "--json")
	if err != nil {
		t.Fatalf("task new --json: %v (stderr %q)", err, stderr)
	}
	warnings := decodeEnvelope(t, stdout).Warnings
	if len(warnings) != 1 || warnings[0].Code != "empty_derived_slug" || warnings[0].TaskID != "T-001" {
		t.Fatalf("envelope warnings = %+v, want one empty_derived_slug for T-001", warnings)
	}
}

// A warned invocation still succeeds: warnings never change exit status, and the
// advisory is published alongside the result rather than instead of it.
func TestSkillSkewWarningPublishesInTheEnvelopeWithoutGating(t *testing.T) {
	root := setupRepo(t)
	stageSkillSkew(t, root)

	stdout, stderr, err := runRootSplit(t, "status", "--json")
	if err != nil {
		t.Fatalf("status --json: %v (stderr %q)", err, stderr)
	}
	codes := envelopeWarningCodes(t, stdout)
	if len(codes) != 1 || codes[0] != "skill_version_skew" {
		t.Fatalf("envelope warning codes = %v, want one skill_version_skew", codes)
	}
	// The same advisory stays visible to a human on stderr.
	if !strings.Contains(stderr, "taskrail init --with-skills --force") {
		t.Fatalf("skew warning missing from stderr: %q", stderr)
	}
}

// An error envelope carries the ambient advisories too: a skew is exactly the
// explanation a failing invocation may need.
func TestErrorEnvelopeCarriesAmbientWarnings(t *testing.T) {
	root := setupRepo(t)
	stageSkillSkew(t, root)

	stdout, _, err := runRootSplit(t, "start", "T-404", "--json")
	if err == nil {
		t.Fatal("start of an unknown task must fail")
	}
	if code := decodeMachineError(t, stdout).Code; code != "task_not_found" {
		t.Fatalf("error code = %q, want task_not_found", code)
	}
	if codes := envelopeWarningCodes(t, stdout); len(codes) != 1 || codes[0] != "skill_version_skew" {
		t.Fatalf("envelope warning codes = %v, want one skill_version_skew", codes)
	}
}

// The ambient advisories are observed once, at repository discovery. A command
// whose own work resolves the skew must not report it on stderr and then omit it
// from the envelope it publishes afterwards: one invocation's two channels would
// contradict each other.
func TestAmbientWarningsAreConsistentAcrossBothChannels(t *testing.T) {
	root := setupRepo(t)
	stageSkillSkew(t, root)

	// --force refreshes the very skills the skew warning is about, so this is the
	// one invocation whose before/after observations differ.
	stdout, stderr, err := runRootSplit(t, "init", "--with-skills", "--force", "--json")
	if err != nil {
		t.Fatalf("init --with-skills --force --json: %v (stderr %q)", err, stderr)
	}
	codes := envelopeWarningCodes(t, stdout)
	if len(codes) != 1 || codes[0] != "skill_version_skew" {
		t.Fatalf("envelope warning codes = %v, want the skew observed at discovery", codes)
	}
	if !strings.Contains(stderr, "taskrail init --with-skills --force") {
		t.Fatalf("skew warning missing from stderr: %q", stderr)
	}
}
