package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if err := os.MkdirAll(filepath.Join(root, "planning", "tasks", "T-002.md"), 0o755); err != nil {
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

	var payload struct {
		Partial  bool   `json:"partial"`
		SpecPath string `json:"spec_path"`
		Tasks    []struct {
			TaskID string `json:"task_id"`
			Path   string `json:"path"`
		} `json:"tasks"`
	}
	if jsonErr := json.Unmarshal([]byte(stdout), &payload); jsonErr != nil {
		t.Fatalf("partial apply --json must stay parseable: %v (stdout %q)", jsonErr, stdout)
	}
	if !payload.Partial {
		t.Fatalf("envelope must be marked partial so a script cannot read it as a clean apply, got %q", stdout)
	}
	if payload.SpecPath != "specs/notes.md" {
		t.Fatalf("envelope must name the written spec, got %q", payload.SpecPath)
	}
	if len(payload.Tasks) != 1 || payload.Tasks[0].TaskID != "T-001" || payload.Tasks[0].Path != "planning/tasks/T-001.md" {
		t.Fatalf("envelope must name the task written before the failure, got %+v", payload.Tasks)
	}
	if _, statErr := os.Stat(filepath.Join(root, payload.Tasks[0].Path)); statErr != nil {
		t.Fatalf("reported task path must exist on disk: %v", statErr)
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
	if !strings.Contains(stdout, "created T-001 planning/tasks/T-001.md") {
		t.Fatalf("text mode must report the task written before the failure, got %q", stdout)
	}
	if !strings.Contains(err.Error(), "partial apply already wrote") {
		t.Fatalf("error must carry the partial-apply wrapper, got %v", err)
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

// A failure that writes nothing (pre-flight rejection) must stay silent on
// stdout: there is no partial state to review, and an empty envelope would
// invite a pointless cleanup pass.
func TestImportApplyPreflightFailureEmitsNoEnvelope(t *testing.T) {
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
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("a failure that wrote nothing must emit no envelope, got %q", stdout)
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
