package taskrail

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInitCreatesStructureAndIsIdempotent(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if err := svc.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := svc.Init(); err != nil {
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
