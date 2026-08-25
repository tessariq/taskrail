package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskAuthorPreviewAndApplyOnlyReplaceReviewedSections(t *testing.T) {
	repo := realGitRepo(t)
	seedFixtureTree(t, repo)
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), layout2Marker("committed", "specs", "planning"))
	writeAuthorableTask(t, repo)
	taskPath := filepath.Join(repo, "planning", "tasks", "T-215-author.md")
	before, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(repo, "planning", "STATE.md")
	stateBefore, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	proposal := filepath.Join(repo, "proposal.md")
	writeFile(t, proposal, "## Description\n\nApply the reviewed outcome.\n\n## Acceptance\n\n- The operator sees the reviewed change.\n\n## Verification Notes\n\n- Run the focused service test.\n")
	svc, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	input := TaskAuthorInput{TaskID: "T-215-author", BodyPath: "proposal.md", ExpectSHA256: digestRaw(before)}
	preview, err := svc.TaskAuthor(TaskAuthorInput{TaskID: input.TaskID, BodyPath: input.BodyPath, ExpectSHA256: input.ExpectSHA256, DryRun: true})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Applied || !preview.Validation.Valid || preview.TaskSHA256Before != input.ExpectSHA256 || preview.TaskSHA256After == input.ExpectSHA256 || !strings.Contains(preview.Diff, "--- planning/tasks/T-215-author.md\n+++ planning/tasks/T-215-author.md\n") {
		t.Fatalf("preview = %+v", preview)
	}
	if strings.Contains(preview.Diff, "-id: T-215-author") || strings.Contains(preview.Diff, "+loop_policy: hold") {
		t.Fatalf("diff includes unchanged managed bytes:\n%s", preview.Diff)
	}
	if got, err := os.ReadFile(taskPath); err != nil || string(got) != string(before) {
		t.Fatalf("preview task bytes = %q, err=%v", got, err)
	}
	if got, err := os.ReadFile(statePath); err != nil || string(got) != string(stateBefore) {
		t.Fatalf("preview state bytes = %q, err=%v", got, err)
	}

	applied, err := svc.TaskAuthor(input)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !applied.Applied || applied.TaskSHA256Before != preview.TaskSHA256Before || applied.TaskSHA256After != preview.TaskSHA256After || applied.Diff != preview.Diff {
		t.Fatalf("apply = %+v, preview = %+v", applied, preview)
	}
	after, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), string(mustReadFile(t, proposal))) || !strings.HasSuffix(string(after), "## Implementation Notes\n") {
		t.Fatalf("applied task = %q", after)
	}
	if got, err := os.ReadFile(statePath); err != nil || string(got) != string(stateBefore) {
		t.Fatalf("apply state bytes = %q, err=%v", got, err)
	}
}

func TestTaskAuthorRefusesInvalidProposalAndStaleDigestWithoutWriting(t *testing.T) {
	for _, test := range []struct {
		name     string
		proposal string
		digest   string
		code     string
	}{
		{"extra heading", "## Description\n\nBody.\n\n## Acceptance\n\n- Works.\n\n## Verification Notes\n\n- Test.\n\n## Other\n\nNo.\n", strings.Repeat("a", 64), MachineCodeInvalidProposal},
		{"stale digest", "## Description\n\nBody.\n\n## Acceptance\n\n- Works.\n\n## Verification Notes\n\n- Test.\n", strings.Repeat("a", 64), MachineCodeWriteConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := realGitRepo(t)
			seedFixtureTree(t, repo)
			writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), layout2Marker("committed", "specs", "planning"))
			writeAuthorableTask(t, repo)
			taskPath := filepath.Join(repo, "planning", "tasks", "T-215-author.md")
			before := mustReadFile(t, taskPath)
			proposal := filepath.Join(repo, "proposal.md")
			writeFile(t, proposal, test.proposal)
			digest := test.digest
			if test.name == "extra heading" {
				digest = digestRaw(before)
			}
			svc, err := NewService(repo)
			if err != nil {
				t.Fatal(err)
			}
			_, err = svc.TaskAuthor(TaskAuthorInput{TaskID: "T-215-author", BodyPath: "proposal.md", ExpectSHA256: digest})
			if err == nil || MachineFailureFor(err).Code != test.code {
				t.Fatalf("TaskAuthor error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, test.code)
			}
			if got := mustReadFile(t, taskPath); string(got) != string(before) {
				t.Fatalf("task changed after refusal:\n%s", got)
			}
		})
	}
}

func TestTaskAuthorRejectsNonTodoArtifactProposalAndLateTaskChange(t *testing.T) {
	for _, test := range []struct {
		name   string
		status string
		body   string
		race   bool
		code   string
	}{
		{"non todo", "in_progress", "proposal.md", false, MachineCodeInvalidStatus},
		{"artifact body", "todo", "planning/artifacts/proposal.md", false, MachineCodeInvalidProposal},
		{"late task change", "todo", "proposal.md", true, MachineCodeWriteConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := realGitRepo(t)
			seedFixtureTree(t, repo)
			writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), layout2Marker("committed", "specs", "planning"))
			writeAuthorableTask(t, repo)
			taskPath := filepath.Join(repo, "planning", "tasks", "T-215-author.md")
			if test.status != "todo" {
				writeFile(t, taskPath, strings.Replace(string(mustReadFile(t, taskPath)), "status: todo", "status: "+test.status, 1))
			}
			proposalPath := filepath.Join(repo, filepath.FromSlash(test.body))
			writeFile(t, proposalPath, "## Description\n\nBody.\n\n## Acceptance\n\n- Works.\n\n## Verification Notes\n\n- Test.\n")
			before := mustReadFile(t, taskPath)
			svc, err := NewService(repo)
			if err != nil {
				t.Fatal(err)
			}
			if test.race {
				testHookWriterValidated = func() {
					writeFile(t, taskPath, string(before)+"\nconcurrent change\n")
				}
				t.Cleanup(func() { testHookWriterValidated = nil })
			}
			_, err = svc.TaskAuthor(TaskAuthorInput{TaskID: "T-215-author", BodyPath: test.body, ExpectSHA256: digestRaw(before)})
			if err == nil || MachineFailureFor(err).Code != test.code {
				t.Fatalf("TaskAuthor error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, test.code)
			}
			if !test.race && string(mustReadFile(t, taskPath)) != string(before) {
				t.Fatal("refused author request changed task bytes")
			}
		})
	}
}

func TestTaskAuthorIgnoresScaffoldHeadingsInsideFencedTargetContent(t *testing.T) {
	repo := realGitRepo(t)
	seedFixtureTree(t, repo)
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), layout2Marker("committed", "specs", "planning"))
	writeAuthorableTask(t, repo)
	taskPath := filepath.Join(repo, "planning", "tasks", "T-215-author.md")
	before := strings.Replace(string(mustReadFile(t, taskPath)), "## Description\n\nFixture task.", "```markdown\n## Description\nexample only\n```\n\n## Description\n\nFixture task.", 1)
	writeFile(t, taskPath, before)
	writeFile(t, filepath.Join(repo, "proposal.md"), "## Description\n\nUpdated body.\n\n## Acceptance\n\n- Works.\n\n## Verification Notes\n\n- Test.\n")
	svc, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TaskAuthor(TaskAuthorInput{TaskID: "T-215-author", BodyPath: "proposal.md", ExpectSHA256: digestRaw([]byte(before))}); err != nil {
		t.Fatalf("TaskAuthor: %v", err)
	}
	after := string(mustReadFile(t, taskPath))
	if !strings.Contains(after, "```markdown\n## Description\nexample only\n```") || !strings.Contains(after, "## Description\n\nUpdated body.") || !strings.Contains(after, "## Implementation Notes\n") {
		t.Fatalf("authoring damaged fenced or preserved content:\n%s", after)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeAuthorableTask(t *testing.T, repo string) {
	t.Helper()
	writeFile(t, filepath.Join(repo, "planning", "tasks", "T-215-author.md"), `---
id: T-215-author
title: Author
status: todo
priority: high
spec_ref: specs/v0.1.0.md#summary
dependencies: []
updated_at: "2026-03-31T00:00:00Z"
loop_policy: hold
loop_reason: "operator decision"
---

# T-215-author Author

## Description

Fixture task.

## Acceptance

- The fixture is valid.

## Verification Notes

- Run the fixture test.

## Implementation Notes
`)
}
