package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/taskrail"
)

// partialApplyDraft names two tasks and one spec section. A directory squatting
// on the second task's destination must reject the whole transaction.
const partialApplyDraft = `{
  "schema_version": 1,
  "target": "planning",
  "source": "notes.md",
  "spec_sections": [{"heading": "Overview", "body": "Ship it."}],
  "tasks": [
    {"key": "alpha", "title": "Alpha task", "spec_ref": "specs/v0.1.0.md#summary", "priority": "medium"},
    {"key": "beta", "title": "Beta task", "spec_ref": "specs/v0.1.0.md#summary", "priority": "medium"}
  ]
}`

// seedDraftRepo initializes a repo and drops draft at draft.json, the path every
// test below feeds to --apply.
func seedDraftRepo(t *testing.T, draft string) string {
	t.Helper()
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, "draft.json"), []byte(draft), 0o644); err != nil {
		t.Fatalf("write draft: %v", err)
	}
	return root
}

// seedPartialApplyRepo additionally blocks the id the second draft task would
// be allocated (a fresh repo has no tasks, so ids start at T-001).
func seedPartialApplyRepo(t *testing.T) string {
	t.Helper()
	root := seedDraftRepo(t, partialApplyDraft)
	if err := os.MkdirAll(filepath.Join(root, "planning", "tasks", "T-002-beta-task.md"), 0o755); err != nil {
		t.Fatalf("seed blocking directory: %v", err)
	}
	return root
}

func TestImportApplyCollisionEmitsNoPublishedArtifactsJSON(t *testing.T) {
	root := seedPartialApplyRepo(t)

	stdout, _, err := runRootSplit(t, "import", "--apply", "draft.json", "--json")
	if err == nil {
		t.Fatalf("expected a non-zero exit on a collision, got output %q", stdout)
	}

	failure := decodeMachineError(t, stdout)
	if failure.Code != taskrail.MachineCodeRepositoryInvalid {
		t.Fatalf("code = %q, want %q", failure.Code, taskrail.MachineCodeRepositoryInvalid)
	}
	if failure.Details.Applied {
		t.Error("a refused import reported applied = true")
	}
	if len(failure.Details.Paths) != 0 {
		t.Fatalf("collision reported published paths %v", failure.Details.Paths)
	}
	if _, statErr := os.Stat(filepath.Join(root, "specs", "notes.md")); !os.IsNotExist(statErr) {
		t.Fatalf("collision published a spec: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, "planning", "tasks", "T-001-alpha-task.md")); !os.IsNotExist(statErr) {
		t.Fatalf("collision published a task: %v", statErr)
	}
}

func TestImportApplyCollisionReportsNoPublishedArtifactsText(t *testing.T) {
	seedPartialApplyRepo(t)

	stdout, _, err := runRootSplit(t, "import", "--apply", "draft.json")
	if err == nil {
		t.Fatalf("expected a non-zero exit on a collision, got output %q", stdout)
	}
	if stdout != "" {
		t.Fatalf("text mode reported unpublished artifacts: %q", stdout)
	}
}

func TestImportApplyCollisionDoesNotEmitUnpublishedWarnings(t *testing.T) {
	root := seedDraftRepo(t, `{
  "schema_version": 1,
  "target": "tasks",
  "source": "notes.md",
  "tasks": [
    {"key": "empty", "title": "!!!", "spec_ref": "specs/v0.1.0.md#summary", "priority": "medium"},
    {"key": "beta", "title": "Beta task", "spec_ref": "specs/v0.1.0.md#summary", "priority": "medium"}
  ]
}`)
	if err := os.MkdirAll(filepath.Join(root, "planning", "tasks", "T-002-beta-task.md"), 0o755); err != nil {
		t.Fatalf("seed blocking directory: %v", err)
	}

	stdout, stderr, err := runRootSplit(t, "import", "--apply", "draft.json", "--json")
	if err == nil {
		t.Fatalf("expected collision failure, got stdout %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("collision emitted unpublished warning: %q", stderr)
	}
	failure := decodeMachineError(t, stdout)
	if failure.Code != taskrail.MachineCodeRepositoryInvalid || len(failure.Details.Paths) != 0 {
		t.Fatalf("collision result = %+v", failure)
	}
}

func TestImportApplyFailedSpecDestinationReportsNoSuccess(t *testing.T) {
	root := seedDraftRepo(t, `{
  "schema_version": 1,
  "target": "spec",
  "source": "failed.md",
  "spec_sections": [{"heading": "Overview", "body": "Ship it."}]
}`)
	if err := os.Mkdir(filepath.Join(root, "specs", "failed.md"), 0o755); err != nil {
		t.Fatalf("block imported spec write: %v", err)
	}

	stdout, _, err := runRootSplit(t, "import", "--apply", "draft.json")
	if err == nil {
		t.Fatalf("expected a non-zero exit on a failed spec write, got output %q", stdout)
	}
	if stdout != "" {
		t.Fatalf("text mode reported unpublished spec: %q", stdout)
	}
}

// A failure that writes nothing (pre-flight rejection) publishes the draft's own
// refusal with no paths: there is no partial state to review, so a cleanup pass
// has nothing to act on.
func TestImportApplyPreflightFailureReportsNoWrittenPaths(t *testing.T) {
	seedDraftRepo(t, `{
  "schema_version": 1,
  "target": "planning",
  "source": "notes.md",
  "tasks": [{"key": "alpha", "title": "Alpha task", "spec_ref": "specs/v0.1.0.md#nope", "priority": "medium"}]
}`)

	stdout, _, err := runRootSplit(t, "import", "--apply", "draft.json", "--json")
	if err == nil {
		t.Fatalf("expected a non-zero exit for an unresolvable spec_ref, got %q", stdout)
	}
	failure := decodeMachineError(t, stdout)
	if failure.Code != taskrail.MachineCodeInvalidProposal {
		t.Fatalf("code = %q, want %q", failure.Code, taskrail.MachineCodeInvalidProposal)
	}
	if len(failure.Details.Paths) != 0 || failure.Details.Applied {
		t.Fatalf("a failure that wrote nothing must report no paths and applied = false, got %+v", failure.Details)
	}
}

// Successful applies keep their existing envelope: no partial marker, so scripts
// written against the clean-apply shape are unaffected.
func TestImportApplySuccessEnvelopeCarriesNoPartialMarker(t *testing.T) {
	seedDraftRepo(t, `{
  "schema_version": 1,
  "target": "planning",
  "source": "notes.md",
  "spec_sections": [{"heading": "Overview", "body": "Ship it."}],
  "tasks": [{"key": "alpha", "title": "Alpha task", "spec_ref": "specs/v0.1.0.md#summary", "priority": "medium"}]
}`)

	stdout, _, err := runRootSplit(t, "import", "--apply", "draft.json", "--json")
	if err != nil {
		t.Fatalf("apply: %v (output %q)", err, stdout)
	}
	if strings.Contains(stdout, "partial") {
		t.Fatalf("a clean apply must not carry a partial marker, got %q", stdout)
	}
}

func TestImportApplyEmptyTitleSlugWarnsWithoutCorruptingJSON(t *testing.T) {
	root := seedDraftRepo(t, `{
  "schema_version": 1,
  "target": "tasks",
  "source": "notes.md",
  "tasks": [{"key": "empty", "title": "!!!", "spec_ref": "specs/v0.1.0.md#summary", "priority": "medium"}]
}`)

	stdout, stderr, err := runRootSplit(t, "import", "--apply", "draft.json", "--json")
	if err != nil {
		t.Fatalf("apply: %v (stdout %q stderr %q)", err, stdout, stderr)
	}
	if !strings.Contains(stderr, `"!!!" produced no slug segment`) || !strings.Contains(stderr, "T-001") {
		t.Fatalf("expected empty-slug warning on stderr, got %q", stderr)
	}
	var payload struct {
		Tasks []struct {
			TaskID string `json:"task_id"`
			Path   string `json:"path"`
		} `json:"tasks"`
	}
	decodeMachineResult(t, stdout, &payload)
	if len(payload.Tasks) != 1 || payload.Tasks[0].TaskID != "T-001" || payload.Tasks[0].Path != "planning/tasks/T-001.md" {
		t.Fatalf("unexpected bare-id JSON result: %+v", payload.Tasks)
	}
	if _, err := os.Stat(filepath.Join(root, payload.Tasks[0].Path)); err != nil {
		t.Fatalf("reported task path must exist: %v", err)
	}
}
