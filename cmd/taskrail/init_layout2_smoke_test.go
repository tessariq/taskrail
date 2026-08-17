package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The layout-2 upgrade preview through the real CLI: the flagless invocation on
// a layout-1 repository stays read-only, publishes the machine contract's
// migration_preview, and the operator gates are observable in both modes.

// seedLayout1CLIRepo builds a marked layout-1 repository the way a v0.4 adopter
// would have it: a real init, then one authored task.
func seedLayout1CLIRepo(t *testing.T) string {
	t.Helper()
	root := setupRepo(t)
	writeTaskCLI(t, root, "T-001-example")
	return root
}

func writeTaskCLI(t *testing.T, root, id string) {
	t.Helper()
	path := filepath.Join(root, "planning", "tasks", id+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create tasks dir: %v", err)
	}
	body := "---\nid: " + id + "\ntitle: Example\nstatus: todo\npriority: high\n" +
		"spec_ref: specs/v0.1.0.md#summary\ndependencies: []\n" +
		"updated_at: \"2026-08-17T00:00:00Z\"\n---\n\n# " + id + " Example\n\n## Description\n\nFixture.\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write task: %v", err)
	}
}

func TestInitLayout2UpgradePreviewCLI(t *testing.T) {
	root := seedLayout1CLIRepo(t)
	before := readAllFiles(t, root)

	stdout, _, err := runRootSplit(t, "init", "--json")
	if err != nil {
		t.Fatalf("init --json preview: %v", err)
	}
	var result struct {
		Outcome           string   `json:"outcome"`
		FromVersion       int      `json:"from_version"`
		ToVersion         int      `json:"to_version"`
		Applied           bool     `json:"applied"`
		StorageMode       string   `json:"storage_mode"`
		ContinuationNotes []string `json:"continuation_notes"`
		Notes             []struct {
			ContinuationAction  *string  `json:"continuation_action"`
			ContinuationChoices []string `json:"continuation_choices"`
		} `json:"notes"`
		Skills          []map[string]string `json:"skills"`
		SkillExclusions []map[string]string `json:"skill_exclusions"`
	}
	decodeMachineResult(t, stdout, &result)

	if result.Outcome != "migration_preview" || result.FromVersion != 1 || result.ToVersion != 2 || result.Applied {
		t.Fatalf("preview header = %+v", result)
	}
	if result.StorageMode != "committed" {
		t.Fatalf("storage_mode = %q", result.StorageMode)
	}
	if len(result.Notes) != 1 || result.Notes[0].ContinuationAction != nil ||
		len(result.Notes[0].ContinuationChoices) != 0 {
		// setupRepo's fresh state seeds no continuation notes, so the preview
		// exposes no note decision for it.
		t.Fatalf("notes = %+v, want null action and no choices for a note-free source", result.Notes)
	}
	if result.Skills == nil || result.SkillExclusions == nil {
		t.Fatalf("required arrays decoded as null: %+v", result)
	}
	if len(readAllFiles(t, root)) != len(before) {
		t.Fatal("preview wrote repository bytes")
	}
}

func TestInitLayout2UpgradePreviewTextNamesTheGates(t *testing.T) {
	root := seedLayout1CLIRepo(t)
	writeFileCLI(t, filepath.Join(root, "planning", "STATE.md"), strings.Replace(
		readFileCLI(t, filepath.Join(root, "planning", "STATE.md")),
		"continuation_notes: []", "continuation_notes:\n  - Authored note.", 1))
	// An absent sidecar is what makes both note decisions applicable.
	if err := os.Remove(filepath.Join(root, "planning", "NOTES.md")); err != nil {
		t.Fatalf("remove notes sidecar: %v", err)
	}

	out, err := runRoot(t, "init")
	if err != nil {
		t.Fatalf("init preview: %v", err)
	}
	for _, want := range []string{
		"layout 2 upgrade available 1 -> 2 (dry run)",
		"--confirm-quiescent",
		"--extract-continuation-notes or --drop-continuation-notes",
		"implementation review rounds: 1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("preview output missing %q:\n%s", want, out)
		}
	}
}

func TestInitLayout2UpgradeApplyGatesCLI(t *testing.T) {
	root := seedLayout1CLIRepo(t)

	// Text mode classifies through the same error the machine mode publishes;
	// the harness returns it rather than printing it.
	_, _, err := runRootSplit(t, "init", "--apply")
	if err == nil {
		t.Fatal("apply without quiescence succeeded")
	}
	if !strings.Contains(err.Error(), "--confirm-quiescent") {
		t.Fatalf("missing-quiescence refusal is not actionable: %v", err)
	}

	requireRecoveryDirectoryDurability(t, root)
	stdout, err := runRoot(t, "init", "--apply", "--confirm-quiescent", "--json")
	if err != nil {
		t.Fatalf("fully gated apply: %v", err)
	}
	var result struct {
		Outcome string `json:"outcome"`
		Applied bool   `json:"applied"`
		To      int    `json:"to_version"`
	}
	decodeMachineResult(t, stdout, &result)
	if result.Outcome != "migrated" || !result.Applied || result.To != 2 {
		t.Fatalf("applied result = %+v, want the applied layout-2 migration", result)
	}
	marker, err := os.ReadFile(filepath.Join(root, ".taskrail", "config.yml"))
	if err != nil || strings.Contains(string(marker), "migration_fence") {
		t.Fatalf("published marker still fences: %s (%v)", marker, err)
	}
}

// The applied layout-2 migration through the CLI names the Git-reversion
// downgrade path rather than marker editing.
func TestInitLayout2UpgradeAppliedSummaryCLI(t *testing.T) {
	root := seedLayout1CLIRepo(t)
	requireRecoveryDirectoryDurability(t, root)
	writeFileCLI(t, filepath.Join(root, "planning", "STATE.md"), strings.Replace(
		readFileCLI(t, filepath.Join(root, "planning", "STATE.md")),
		"continuation_notes: []", "continuation_notes:\n  - Authored note.", 1))
	if err := os.Remove(filepath.Join(root, "planning", "NOTES.md")); err != nil {
		t.Fatalf("remove notes sidecar: %v", err)
	}

	out, err := runRoot(t, "init", "--apply", "--confirm-quiescent", "--drop-continuation-notes")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	for _, want := range []string{"migrated layout 1 -> 2", "validation: valid", "reverting the complete upgrade commit"} {
		if !strings.Contains(out, want) {
			t.Errorf("applied summary missing %q:\n%s", want, out)
		}
	}
}

func TestInitRejectsQuiescenceOutsideTheUpgradeCLI(t *testing.T) {
	setupUnmarkedRepo(t)
	if out, err := runRoot(t, "init", "--confirm-quiescent"); err == nil {
		t.Fatalf("fresh init accepted the quiescence assertion: %q", out)
	}
}

func writeFileCLI(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFileCLI(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
