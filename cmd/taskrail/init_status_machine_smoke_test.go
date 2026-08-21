package main

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// The init and status documents an agent actually receives from the CLI, rather
// than the service results behind them
// (specs/contracts/v0.5.0-machine-api.md).

func TestInitJSONPublishesTheContractedResult(t *testing.T) {
	setupUnmarkedRepo(t)

	stdout, _, err := runRootSplit(t, "init", "--json")
	if err != nil {
		t.Fatalf("init --json: %v", err)
	}
	var result struct {
		Outcome     string `json:"outcome"`
		FromVersion int    `json:"from_version"`
		ToVersion   int    `json:"to_version"`
		Applied     bool   `json:"applied"`
		StorageMode string `json:"storage_mode"`
		// The exact member set of each write and of `config` is pinned by the
		// service-level golden; here only the published path list matters.
		Writes []struct {
			Path string `json:"path"`
		} `json:"writes"`
		Notes []struct {
			Path                string   `json:"path"`
			FileAction          string   `json:"file_action"`
			ContinuationAction  *string  `json:"continuation_action"`
			ContinuationChoices []string `json:"continuation_choices"`
		} `json:"notes"`
		Skills            []json.RawMessage `json:"skills"`
		SkillExclusions   []json.RawMessage `json:"skill_exclusions"`
		ContinuationNotes []string          `json:"continuation_notes"`
		Validation        json.RawMessage   `json:"validation"`
	}
	decodeMachineResult(t, stdout, &result)

	if result.Outcome != "created" || result.FromVersion != 0 || result.ToVersion != 1 || !result.Applied {
		t.Fatalf("init result header = %+v", result)
	}
	if result.StorageMode != "committed" {
		t.Fatalf("storage_mode = %q, want committed", result.StorageMode)
	}
	if len(result.Writes) != 5 || result.Writes[0].Path != ".taskrail/config.yml" {
		t.Fatalf("writes = %+v", result.Writes)
	}
	if len(result.Notes) != 1 || result.Notes[0].FileAction != "create_template" ||
		result.Notes[0].ContinuationAction != nil {
		t.Fatalf("notes = %+v", result.Notes)
	}
	// Required arrays are `[]`, never null, and `validation` is explicitly null
	// for an outcome that does not re-run it.
	if result.Skills == nil || result.SkillExclusions == nil || result.ContinuationNotes == nil {
		t.Fatalf("required arrays decoded as null: %+v", result)
	}
	if string(result.Validation) != "null" {
		t.Fatalf("validation = %s, want null", result.Validation)
	}
}

func TestInitWithSkillsJSONPublishesTheInstalledInventory(t *testing.T) {
	setupUnmarkedRepo(t)

	stdout, _, err := runRootSplit(t, "init", "--with-skills", "--json")
	if err != nil {
		t.Fatalf("init --with-skills --json: %v", err)
	}
	var result struct {
		Skills []struct {
			Path   string `json:"path"`
			Action string `json:"action"`
		} `json:"skills"`
		SkillExclusions []json.RawMessage `json:"skill_exclusions"`
	}
	decodeMachineResult(t, stdout, &result)

	if len(result.Skills) == 0 {
		t.Fatal("init --with-skills --json reported no installed skills")
	}
	if len(result.SkillExclusions) != 0 {
		t.Fatalf("committed install reported exclusions %v", result.SkillExclusions)
	}
	for _, skill := range result.Skills {
		if skill.Action != "create" {
			t.Fatalf("skill %+v was not reported as created", skill)
		}
		if !strings.HasPrefix(skill.Path, ".agents/skills/") && !strings.HasPrefix(skill.Path, ".claude/skills/") {
			t.Fatalf("skill %+v is not at an assistant discovery path", skill)
		}
	}
}

func TestStatusJSONReportsTheActiveStorage(t *testing.T) {
	setupRepo(t)

	stdout, _, err := runRootSplit(t, "status", "--json")
	if err != nil {
		t.Fatalf("status --json: %v", err)
	}
	var result struct {
		Storage struct {
			Mode         string `json:"mode"`
			Root         string `json:"root"`
			ArtifactsDir string `json:"artifacts_dir"`
		} `json:"storage"`
	}
	decodeMachineResult(t, stdout, &result)

	if result.Storage.Mode != "committed" || result.Storage.Root != "." ||
		result.Storage.ArtifactsDir != "planning/artifacts" {
		t.Fatalf("storage = %+v", result.Storage)
	}
}

func TestLocalInspectionJSONReportsTheActiveLocalContext(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "--quiet", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, output)
	}
	t.Chdir(root)
	if _, err := runRoot(t, "init", "--local"); err != nil {
		t.Fatalf("init --local: %v", err)
	}

	stdout, _, err := runRootSplit(t, "local", "status", "--json")
	if err != nil {
		t.Fatalf("local status --json: %v", err)
	}
	var status struct {
		Mode           string `json:"mode"`
		StorageRoot    string `json:"storage_root"`
		PromotionReady bool   `json:"promotion_ready"`
		Violations     []any  `json:"violations"`
	}
	decodeMachineResult(t, stdout, &status)
	if status.Mode != "local" || status.StorageRoot != ".taskrail/local" || !status.PromotionReady || status.Violations == nil || len(status.Violations) != 0 {
		t.Fatalf("local status = %+v", status)
	}

	stdout, _, err = runRootSplit(t, "local", "path", "--json")
	if err != nil {
		t.Fatalf("local path --json: %v", err)
	}
	var paths struct {
		Mode         string `json:"mode"`
		SpecsDir     string `json:"specs_dir"`
		PlanningDir  string `json:"planning_dir"`
		ArtifactsDir string `json:"artifacts_dir"`
	}
	decodeMachineResult(t, stdout, &paths)
	if paths.Mode != "local" || paths.SpecsDir != "specs" || paths.PlanningDir != "planning" || paths.ArtifactsDir != ".taskrail/local/planning/artifacts" {
		t.Fatalf("local paths = %+v", paths)
	}
}
