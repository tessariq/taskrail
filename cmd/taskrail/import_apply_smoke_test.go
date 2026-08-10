package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/taskrail"
)

// partialApplyDraft names two tasks and one spec section. Applied against a repo
// where a directory squats on the second task's file path, the write fails
// mid-loop with the spec and the first task already on disk — the only shape
// that reaches the residual partial-apply path (pre-flight rejects everything it
// can detect without writing).
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

// seedPartialApplyRepo additionally blocks the id the second draft task will be
// allocated (a fresh repo has no tasks, so ids start at T-001).
func seedPartialApplyRepo(t *testing.T) string {
	t.Helper()
	root := seedDraftRepo(t, partialApplyDraft)
	if err := os.MkdirAll(filepath.Join(root, "planning", "tasks", "T-002-beta-task.md"), 0o755); err != nil {
		t.Fatalf("seed blocking directory: %v", err)
	}
	return root
}

// A partial apply already moved the repository, so --json must still emit the
// artifacts it wrote: an operator (or script) reviewing before a retry needs the
// per-task paths, and getting nothing at all on stdout is indistinguishable from
// an apply that wrote nothing.
func TestImportApplyPartialFailureEmitsWrittenArtifactsJSON(t *testing.T) {
	root := seedPartialApplyRepo(t)

	stdout, _, err := runRootSplit(t, "import", "--apply", "draft.json", "--json")
	if err == nil {
		t.Fatalf("expected a non-zero exit on a partial apply, got output %q", stdout)
	}
	if !strings.Contains(err.Error(), "partial apply already wrote") {
		t.Fatalf("error must carry the partial-apply wrapper, got %v", err)
	}

	failure := decodeMachineError(t, stdout)
	if failure.Code != taskrail.MachineCodePartialWrite {
		t.Fatalf("code = %q, want %q", failure.Code, taskrail.MachineCodePartialWrite)
	}
	// The complete import never committed, so it must not read back as applied.
	if failure.Details.Applied {
		t.Error("a partial apply reported applied = true")
	}
	want := []string{"planning/tasks/T-001-alpha-task.md", "specs/notes.md"}
	if !slices.Equal(failure.Details.Paths, want) {
		t.Fatalf("paths = %v, want %v", failure.Details.Paths, want)
	}
	for _, path := range failure.Details.Paths {
		if _, statErr := os.Stat(filepath.Join(root, path)); statErr != nil {
			t.Errorf("reported path must exist on disk: %v", statErr)
		}
	}
}

// Text mode reports the same written artifacts, so a terminal operator sees the
// paths to review alongside the partial-apply error wrapper.
func TestImportApplyPartialFailureReportsWrittenArtifactsText(t *testing.T) {
	seedPartialApplyRepo(t)

	stdout, _, err := runRootSplit(t, "import", "--apply", "draft.json")
	if err == nil {
		t.Fatalf("expected a non-zero exit on a partial apply, got output %q", stdout)
	}
	if !strings.Contains(stdout, "review spec specs/notes.md") {
		t.Fatalf("text mode must report the written spec, got %q", stdout)
	}
	if !strings.Contains(stdout, "created T-001-alpha-task planning/tasks/T-001-alpha-task.md") {
		t.Fatalf("text mode must report the task written before the failure, got %q", stdout)
	}
	if !strings.Contains(err.Error(), "partial apply already wrote") {
		t.Fatalf("error must carry the partial-apply wrapper, got %v", err)
	}
}

func TestImportApplyPartialFailurePrintsEarlierEmptySlugWarning(t *testing.T) {
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
		t.Fatalf("expected partial apply failure, got stdout %q", stdout)
	}
	if !strings.Contains(stderr, `"!!!" produced no slug segment`) || !strings.Contains(stderr, "T-001") {
		t.Fatalf("expected earlier empty-slug warning on stderr, got %q", stderr)
	}
	failure := decodeMachineError(t, stdout)
	if failure.Code != taskrail.MachineCodePartialWrite {
		t.Fatalf("code = %q, want %q", failure.Code, taskrail.MachineCodePartialWrite)
	}
	if !slices.Contains(failure.Details.Paths, "planning/tasks/T-001.md") {
		t.Fatalf("paths must name the task written before the failure, got %v", failure.Details.Paths)
	}
}

// A failed spec write may have created or truncated its target, but it may also
// have failed before changing anything. Text output must name the path without
// claiming the write definitely succeeded.
func TestImportApplyFailedSpecWriteReportsPathWithoutClaimingSuccess(t *testing.T) {
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
	if !strings.Contains(stdout, "review spec specs/failed.md") {
		t.Fatalf("text mode must name the uncertain spec path for review, got %q", stdout)
	}
	if strings.Contains(stdout, "wrote spec") {
		t.Fatalf("text mode must not claim the failed spec write succeeded, got %q", stdout)
	}
	if !strings.Contains(err.Error(), "partial apply may have written specs/failed.md") {
		t.Fatalf("error must carry the uncertain partial-apply wrapper, got %v", err)
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
