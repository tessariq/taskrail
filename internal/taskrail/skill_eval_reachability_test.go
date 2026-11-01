package taskrail

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// This executes setup only, not a skill, adapter, or release evaluation. The
// probes check preconditions; they cannot certify the later agent's behavior.
func TestSkillEvalRegistryReachableSetup(t *testing.T) {
	registry, err := loadSkillEvalRegistry(filepath.Join("testdata", "skill-evals", "v1", "cases"), shippableSkills)
	if err != nil {
		t.Fatal(err)
	}
	binary, err := skillEvalBuiltBinary(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	claims := map[string]string{
		"autonomous-backlog":            "placeholder task",
		"autonomous-task":               "placeholder task",
		"autonomous-manual-test":        "placeholder acceptance",
		"autonomous-verify":             "placeholder acceptance",
		"autonomous-recovery":           "no mechanical drift",
		"taskrail-repair":               "no mechanical drift",
		"taskrail-decompose":            "no published post-spec review",
		"taskrail-gap":                  "no structural signals",
		"taskrail-import":               "no source notes or import draft",
		"taskrail-loop":                 "explicitly allowed",
		"taskrail-retrofit":             "already initialized",
		"taskrail-sdd-handoff":          "no upstream SDD artifacts",
		"taskrail-spec":                 "v0.1.0",
		"taskrail-spec-review":          "v0.1.0",
		"taskrail-task-review":          "unauthored scaffold",
		"taskrail-workflow-adversarial": "v0.1.0",
	}
	for _, evaluation := range registry {
		t.Run(evaluation.CaseID, func(t *testing.T) {
			claim, ok := claims[evaluation.Skill]
			if !ok || !strings.Contains(evaluation.Prompt, claim) {
				t.Errorf("prompt must name its checked setup condition %q", claim)
			}
			if evaluation.BaselineRequired && (evaluation.Skill == "taskrail-gap" || evaluation.Skill == "taskrail-decompose") && strings.Contains(evaluation.Prompt, "covered") {
				t.Error("baseline init predates T-399: prompt cannot claim a covered summary area")
			}
			sandbox := t.TempDir()
			fixture := os.DirFS(filepath.Join(evaluation.fixtureRoot, evaluation.Scenario.Fixture))
			if err := fs.WalkDir(fixture, ".", func(path string, entry fs.DirEntry, err error) error {
				if err == nil && !entry.IsDir() && !slices.Contains([]string{"seed.json", "decoy/planning/STATE.md", "git-provenance.txt"}, path) {
					t.Errorf("unexpected source input %s contradicts the missing-input cases", path)
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
			if err := os.CopyFS(sandbox, fixture); err != nil {
				t.Fatal(err)
			}
			command := func(program string, args ...string) ([]byte, error) {
				t.Helper()
				cmd := exec.CommandContext(t.Context(), program, args...)
				cmd.Dir = sandbox
				return cmd.CombinedOutput()
			}
			run := func(program string, args ...string) []byte {
				t.Helper()
				output, err := command(program, args...)
				if err != nil {
					t.Fatalf("%s %v: %v\n%s", program, args, err, output)
				}
				return output
			}
			runJSON := func(result any, args ...string) {
				t.Helper()
				if err := json.Unmarshal(run(binary, args...), result); err != nil {
					t.Fatal(err)
				}
			}
			for _, action := range evaluation.Scenario.Setup {
				program := action.Command[0]
				if action.Operation == "taskrail-command" {
					program = binary
				}
				run(program, action.Command[1:]...)
			}
			if output := run("git", "status", "--porcelain", "--untracked-files=all"); len(output) != 0 {
				t.Fatalf("setup is not clean: %s", output)
			}
			run("git", "rev-parse", "--verify", "HEAD")
			var task struct {
				Result struct{ Content string }
			}
			runJSON(&task, "task", "show", "T-001-evaluation-subject", "--json")
			if !strings.Contains(task.Result.Content, "status: todo") || !strings.Contains(task.Result.Content, "## Acceptance\n\n- TODO: define observable acceptance criteria for the outcome.\n\n## Verification Notes") {
				t.Fatalf("expected todo placeholder subject, got %s", task.Result.Content)
			}
			if strings.Contains(task.Result.Content, "last_verification_") {
				t.Fatal("setup unexpectedly supplied prior verification evidence")
			}
			run(binary, "validate", "--json")
			var status struct {
				Result struct {
					Storage struct{ Mode, Root string }
				}
			}
			runJSON(&status, "status", "--json")
			if status.Result.Storage.Mode != evaluation.StorageMode {
				t.Fatalf("storage = %q, want %q", status.Result.Storage.Mode, evaluation.StorageMode)
			}
			for _, absent := range []string{"planning/reviews", "planning/artifacts/verify", "specs/v0.2.0.md"} {
				path := filepath.Join(sandbox, status.Result.Storage.Root, absent)
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("setup must not supply %s: %v", absent, err)
				}
			}
			var coverage struct {
				Result struct {
					CoverableAreas int `json:"coverable_areas"`
					CoveredAreas   int `json:"covered_areas"`
				}
			}
			runJSON(&coverage, "coverage", "--area", "summary", "--json")
			if coverage.Result.CoverableAreas != 1 || coverage.Result.CoveredAreas != 1 {
				t.Fatalf("starter summary must be coverable and covered: %+v", coverage.Result)
			}
			switch evaluation.Skill {
			case "autonomous-recovery", "taskrail-repair":
				var repair struct {
					Result *struct{ Changes json.RawMessage }
				}
				runJSON(&repair, "repair", "--json")
				if repair.Result == nil || len(repair.Result.Changes) == 0 {
					t.Fatal("repair did not report changes")
				}
				var changes []json.RawMessage
				if err := json.Unmarshal(repair.Result.Changes, &changes); err != nil || len(changes) != 0 {
					t.Fatalf("prompt claims no drift, repair found %s: %v", repair.Result.Changes, err)
				}
			case "taskrail-gap":
				var gaps struct {
					Result *struct{ Signals json.RawMessage }
				}
				runJSON(&gaps, "coverage", "--gaps", "--json")
				if gaps.Result == nil || len(gaps.Result.Signals) == 0 {
					t.Fatal("coverage did not report signals")
				}
				var signals []json.RawMessage
				if err := json.Unmarshal(gaps.Result.Signals, &signals); err != nil || len(signals) != 0 {
					t.Fatalf("prompt claims no structural signals, got %s: %v", gaps.Result.Signals, err)
				}
			case "taskrail-loop":
				var loop struct {
					Result struct {
						Action       string
						SelectedTask struct {
							TaskID string `json:"task_id"`
						} `json:"selected_task"`
					}
				}
				runJSON(&loop, "loop", "--dry-run", "--json")
				if loop.Result.Action != "run" || loop.Result.SelectedTask.TaskID != "T-001-evaluation-subject" {
					t.Fatalf("explicit allowance must select the subject: %+v", loop.Result)
				}
			case "taskrail-retrofit":
				output, err := command(binary, "retrofit", "--json")
				if err == nil || !strings.Contains(string(output), `"code": "destination_exists"`) {
					t.Fatalf("initialized destination must refuse retrofit: %v\n%s", err, output)
				}
			case "taskrail-decompose":
				output, err := command(binary, "review", "show", "planning/reviews/spec/v0.1.0/missing/manifest.json", "--json")
				if err == nil || !strings.Contains(string(output), `"code": "review_not_found"`) {
					t.Fatalf("missing post-spec review must refuse: %v\n%s", err, output)
				}
			case "taskrail-spec", "taskrail-spec-review", "taskrail-workflow-adversarial":
				run(binary, "spec", "show", "v0.1.0", "--json")
			}
		})
	}
}
