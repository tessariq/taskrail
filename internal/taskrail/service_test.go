package taskrail

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitCreatesStructureAndIsIdempotent(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.Init(false); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := svc.Init(false); err != nil {
		t.Fatalf("init second run: %v", err)
	}

	for _, path := range []string{
		filepath.Join(repo, "specs", "README.md"),
		filepath.Join(repo, "planning", "STATE.md"),
		filepath.Join(repo, "planning", "tasks"),
		filepath.Join(repo, "planning", "artifacts", "verify"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
	validation, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !validation.Valid {
		t.Fatalf("expected valid repo, got %v", validation.Violations)
	}
}

func TestValidateCleanCheckoutOfCommittedRepo(t *testing.T) {
	t.Parallel()

	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available")
	}
	runGit := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(git, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
			"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
		)
		if out, gitErr := cmd.CombinedOutput(); gitErr != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), gitErr, out)
		}
	}

	// init + scaffold + a tracked task, then commit only the planning content;
	// artifacts are gitignored output and stay out of the commit. A real repo
	// always carries at least one task file, so tasks/ travels — only the empty
	// gitignored artifact dirs are dropped, isolating exactly the T-024 fix.
	repo := t.TempDir()
	runGit(repo, "init", "-q")
	svc := newTestService(t, repo, time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(false); err != nil {
		t.Fatalf("init: %v", err)
	}
	writeTask(t, repo, "T-001", "Seed task", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeFile(t, filepath.Join(repo, ".gitignore"), "/planning/artifacts/\n")
	runGit(repo, "add", "-A")
	runGit(repo, "commit", "-q", "-m", "init taskrail")

	// A clone carries only committed content. Git cannot track the empty
	// gitignored artifact dirs, so they do not travel — exactly the fresh
	// clone / CI clean-checkout case the acceptance criterion describes.
	clone := t.TempDir()
	runGit(repo, "clone", "-q", repo, clone)
	if _, err := os.Stat(filepath.Join(clone, "planning", "artifacts")); !os.IsNotExist(err) {
		t.Fatalf("expected artifacts tree absent in clone, stat err=%v", err)
	}
	// Non-vacuous: the committed planning content (tasks/) must be present, so a
	// pass proves the artifacts requirement was lifted, not that nothing travels.
	if _, err := os.Stat(filepath.Join(clone, "planning", "tasks", "T-001.md")); err != nil {
		t.Fatalf("expected committed task present in clone: %v", err)
	}

	result, err := newTestService(t, clone, time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)).Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !result.Valid || len(result.Violations) != 0 {
		t.Fatalf("expected valid clean checkout, got valid=%v violations=%v", result.Valid, result.Violations)
	}
}

func TestVerifyCreatesArtifactDirsWhenAbsent(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Verified item", "completed", "high", "specs/v0.1.0.md#summary", nil)

	// Simulate a clean checkout where the gitignored artifacts tree is absent;
	// verify must still create planning/artifacts/verify/<id>/<ts>/ on demand.
	if err := os.RemoveAll(filepath.Join(repo, "planning", "artifacts")); err != nil {
		t.Fatalf("remove artifacts tree: %v", err)
	}

	svc := newTestService(t, repo, time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC))
	result, err := svc.Verify(VerifyInput{
		TaskID:  "T-002",
		Result:  "pass",
		Summary: "Creates dirs on demand",
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	for _, rel := range []string{result.PlanPath, result.ReportPath, result.ReportMarkdown} {
		if _, err := os.Stat(filepath.Join(repo, rel)); err != nil {
			t.Fatalf("expected artifact %s on demand: %v", rel, err)
		}
	}
}

func TestValidateDetectsBrokenDependencyAndSpecRef(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	brokenTask := `---
id: T-002
title: Broken task
status: todo
priority: high
spec_ref: specs/v0.1.0.md#missing-heading
dependencies:
  - T-999
updated_at: "2026-03-31T00:00:00Z"
---

# T-002 Broken task
`
	writeFile(t, filepath.Join(repo, "planning", "tasks", "T-002.md"), brokenTask)

	svc := newTestService(t, repo, time.Now().UTC())
	result, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.Valid {
		t.Fatalf("expected invalid state")
	}
	if len(result.Violations) < 2 {
		t.Fatalf("expected multiple violations, got %v", result.Violations)
	}
}

func TestNextSelectsByDependencyPriorityAndStableID(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "High task", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-003", "Blocked by dependency", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-002"})
	writeTask(t, repo, "T-004", "Medium task", "todo", "medium", "specs/v0.1.0.md#summary", nil)

	svc := newTestService(t, repo, time.Now().UTC())
	result, err := svc.Next()
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if result.TaskID != "T-002" {
		t.Fatalf("expected T-002, got %s", result.TaskID)
	}
}

func TestStartCompleteAndBlockUpdateState(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Work item", "todo", "high", "specs/v0.1.0.md#summary", nil)

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Start("T-002"); err != nil {
		t.Fatalf("start: %v", err)
	}
	state, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load after start: %v", err)
	}
	if state.Frontmatter.CurrentTask != "T-002" {
		t.Fatalf("expected current task T-002, got %s", state.Frontmatter.CurrentTask)
	}
	task, _ := taskByID(tasks, "T-002")
	if task.Frontmatter.Status != "in_progress" {
		t.Fatalf("expected task in_progress, got %s", task.Frontmatter.Status)
	}

	if _, err := svc.Complete("T-002", "implemented"); err != nil {
		t.Fatalf("complete: %v", err)
	}
	state, tasks, err = svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load after complete: %v", err)
	}
	task, _ = taskByID(tasks, "T-002")
	if task.Frontmatter.Status != "completed" {
		t.Fatalf("expected completed, got %s", task.Frontmatter.Status)
	}
	if state.Frontmatter.CurrentTask != "" {
		t.Fatalf("expected no active task, got %s", state.Frontmatter.CurrentTask)
	}

	writeTask(t, repo, "T-003", "Blocked item", "todo", "medium", "specs/v0.1.0.md#summary", nil)
	if _, err := svc.Block("T-003", "waiting on decision"); err != nil {
		t.Fatalf("block: %v", err)
	}
	_, tasks, err = svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load after block: %v", err)
	}
	task, _ = taskByID(tasks, "T-003")
	if task.Frontmatter.Status != "blocked" {
		t.Fatalf("expected blocked, got %s", task.Frontmatter.Status)
	}
}

func TestVerifyWritesArtifactsAndCreatesFollowup(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Verified item", "completed", "high", "specs/v0.1.0.md#summary", nil)

	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	result, err := svc.Verify(VerifyInput{
		TaskID:           "T-002",
		Result:           "fail",
		Summary:          "Need one follow-up",
		Details:          "A missing edge case remains.",
		CreateFollowup:   true,
		FollowupTitle:    "Handle missing edge case",
		FollowupPriority: "medium",
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.FollowupTaskID == "" {
		t.Fatalf("expected follow-up task id")
	}
	if _, err := os.Stat(filepath.Join(repo, result.PlanPath)); err != nil {
		t.Fatalf("expected plan artifact: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, result.ReportPath)); err != nil {
		t.Fatalf("expected report artifact: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repo, result.ReportPath))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var artifact VerificationArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if artifact.FollowupTaskID != result.FollowupTaskID {
		t.Fatalf("expected follow-up %s, got %s", result.FollowupTaskID, artifact.FollowupTaskID)
	}
	if _, err := os.Stat(filepath.Join(repo, "planning", "tasks", result.FollowupTaskID+".md")); err != nil {
		t.Fatalf("expected follow-up task file: %v", err)
	}
}

func TestVerifyWritesPortableCommittedState(t *testing.T) {
	t.Parallel()

	const gitignoredPrefix = "planning/artifacts/"
	now := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	wantTimestamp := timestamp(now)

	// Both result branches share the frontmatter writer, so the portability
	// contract must hold for each — including "fail", which also sets NextAction.
	for _, result := range []string{"pass", "fail"} {
		t.Run(result, func(t *testing.T) {
			t.Parallel()

			repo := seedFixtureRepo(t)
			writeTask(t, repo, "T-002", "Verified item", "completed", "high", "specs/v0.1.0.md#summary", nil)

			svc := newTestService(t, repo, now)
			if _, err := svc.Verify(VerifyInput{
				TaskID:  "T-002",
				Result:  result,
				Summary: "Summary for " + result,
			}); err != nil {
				t.Fatalf("verify: %v", err)
			}

			state, err := svc.loadState()
			if err != nil {
				t.Fatalf("load state: %v", err)
			}

			// Exact shape: result, task id, and timestamp, with no path.
			want := fmt.Sprintf("%s for %s at %s", result, "T-002", wantTimestamp)
			if got := state.Frontmatter.LastVerificationResult; got != want {
				t.Fatalf("last_verification_result = %q, want %q", got, want)
			}
			if strings.Contains(state.Frontmatter.LastVerificationResult, gitignoredPrefix) {
				t.Fatalf("last_verification_result must not embed gitignored path: %q", state.Frontmatter.LastVerificationResult)
			}

			// relevant_artifacts must be empty (no gitignored paths), asserted
			// non-vacuously so a populated slice would fail.
			if n := len(state.Frontmatter.RelevantArtifacts); n != 0 {
				t.Fatalf("relevant_artifacts must be empty, got %d: %v", n, state.Frontmatter.RelevantArtifacts)
			}

			// The verify-time task note is a second committed sink and must be
			// portable too: it records the result and timestamp but no path into
			// gitignored artifacts (else cloned repos point at missing files).
			_, tasks, err := svc.loadStateAndTasks()
			if err != nil {
				t.Fatalf("load tasks: %v", err)
			}
			task, ok := taskByID(tasks, "T-002")
			if !ok {
				t.Fatalf("expected T-002 in tasks")
			}
			if strings.Contains(task.Body, gitignoredPrefix) {
				t.Fatalf("task note must not embed gitignored path:\n%s", task.Body)
			}
			if !strings.Contains(task.Body, wantTimestamp) {
				t.Fatalf("task note must record verification timestamp %q:\n%s", wantTimestamp, task.Body)
			}
			if !strings.Contains(task.Body, "verification "+result) {
				t.Fatalf("task note must record verification result %q:\n%s", result, task.Body)
			}
		})
	}
}

func TestCreateTaskScaffoldsValidTaskAndUpdatesCounts(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-002", "Existing item", "todo", "high", "specs/v0.1.0.md#summary", nil)

	svc := newTestService(t, repo, time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC))
	result, err := svc.CreateTask(CreateTaskInput{
		Title:        "Scaffolded item",
		SpecRef:      "specs/v0.1.0.md#summary",
		Priority:     "high",
		Dependencies: []string{"T-002"},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Deterministic id allocation: max existing (T-002) + 1, zero-padded.
	if result.TaskID != "T-003" {
		t.Fatalf("expected T-003, got %s", result.TaskID)
	}
	if _, err := os.Stat(filepath.Join(repo, result.Path)); err != nil {
		t.Fatalf("expected task file at %s: %v", result.Path, err)
	}

	// The scaffolded file must be valid and carry the requested frontmatter.
	_, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	created, ok := taskByID(tasks, "T-003")
	if !ok {
		t.Fatalf("expected T-003 in tasks")
	}
	if created.Frontmatter.Status != "todo" || created.Frontmatter.Priority != "high" {
		t.Fatalf("unexpected frontmatter: %+v", created.Frontmatter)
	}
	if created.Frontmatter.SpecRef != "specs/v0.1.0.md#summary" {
		t.Fatalf("unexpected spec_ref: %q", created.Frontmatter.SpecRef)
	}
	if len(created.Frontmatter.Dependencies) != 1 || created.Frontmatter.Dependencies[0] != "T-002" {
		t.Fatalf("unexpected dependencies: %v", created.Frontmatter.Dependencies)
	}
	for _, section := range []string{"## Description", "## Acceptance", "## Verification Notes", "## Implementation Notes"} {
		if !strings.Contains(created.Body, section) {
			t.Fatalf("expected body section %q, got %q", section, created.Body)
		}
	}

	// State counts reuse existing logic: two todo tasks now tracked.
	state, err := svc.loadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if !strings.Contains(state.Body, "- todo: 2") {
		t.Fatalf("expected todo count 2 in state body, got %q", state.Body)
	}

	validation, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !validation.Valid {
		t.Fatalf("expected valid repo after scaffold, got %v", validation.Violations)
	}
}

func TestCreateTaskRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input CreateTaskInput
	}{
		{"empty title", CreateTaskInput{Title: "  ", SpecRef: "specs/v0.1.0.md#summary"}},
		{"empty spec ref", CreateTaskInput{Title: "x", SpecRef: ""}},
		{"unknown spec anchor", CreateTaskInput{Title: "x", SpecRef: "specs/v0.1.0.md#nope"}},
		{"bad priority", CreateTaskInput{Title: "x", SpecRef: "specs/v0.1.0.md#summary", Priority: "urgent"}},
		{"missing dependency", CreateTaskInput{Title: "x", SpecRef: "specs/v0.1.0.md#summary", Dependencies: []string{"T-999"}}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := seedFixtureRepo(t)
			// Seed one real task so the dir is non-empty: a regression that writes
			// before validating would add a *second* file (T-002), so the absence
			// checks below are load-bearing rather than passing on an empty dir.
			writeTask(t, repo, "T-001", "Existing item", "todo", "high", "specs/v0.1.0.md#summary", nil)

			svc := newTestService(t, repo, time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC))
			if _, err := svc.CreateTask(tc.input); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
			// The forthcoming id (T-002) must not have been written on rejection.
			if _, err := os.Stat(filepath.Join(repo, "planning", "tasks", "T-002.md")); !os.IsNotExist(err) {
				t.Fatalf("expected no T-002.md written on rejection, stat err=%v", err)
			}
			entries, err := os.ReadDir(filepath.Join(repo, "planning", "tasks"))
			if err != nil {
				t.Fatalf("read tasks dir: %v", err)
			}
			if len(entries) != 1 {
				t.Fatalf("expected only the seeded task to remain, found %d", len(entries))
			}
		})
	}
}

func newTestService(t *testing.T, repo string, now time.Time) *Service {
	t.Helper()
	paths, err := DiscoverPaths(repo)
	if err != nil {
		t.Fatalf("discover paths: %v", err)
	}
	return &Service{paths: paths, now: func() time.Time { return now }}
}

func seedFixtureRepo(t *testing.T) string {
	t.Helper()
	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, "specs", "README.md"), "# Specs\n")
	writeFile(t, filepath.Join(repo, "specs", "v0.1.0.md"), "# Taskrail v0.1.0\n\n## Summary\n\nFixture spec.\n")
	writeFile(t, filepath.Join(repo, "planning", "STATE.md"), `---
schema_version: 1
updated_at: "2026-03-31T00:00:00Z"
active_spec_version: v0.1.0
active_spec_path: specs/v0.1.0.md
current_task: ""
current_task_title: ""
status_summary: idle
blockers: []
next_action: Start the next task
last_verification_result: Not yet run
relevant_artifacts: []
continuation_notes:
  - Fixture repo.
---

# STATE
`)
	if err := os.MkdirAll(filepath.Join(repo, "planning", "tasks"), 0o755); err != nil {
		t.Fatalf("mkdir tasks: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "planning", "artifacts", "verify"), 0o755); err != nil {
		t.Fatalf("mkdir verify: %v", err)
	}
	return repo
}

func writeTask(t *testing.T, repo, id, title, status, priority, specRef string, deps []string) {
	t.Helper()
	depText := "[]"
	if len(deps) > 0 {
		depText = "\n"
		for _, dep := range deps {
			depText += "  - " + dep + "\n"
		}
	}
	content := `---
id: ` + id + `
title: ` + title + `
status: ` + status + `
priority: ` + priority + `
spec_ref: ` + specRef + `
dependencies: ` + depText + `
updated_at: "2026-03-31T00:00:00Z"
---

# ` + id + ` ` + title + `

## Description

Fixture task.
`
	writeFile(t, filepath.Join(repo, "planning", "tasks", id+".md"), content)
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	return repo
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
