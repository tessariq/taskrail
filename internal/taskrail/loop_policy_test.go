package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveLoopPolicy(t *testing.T) {
	t.Parallel()

	allow := "allow"
	reason := "bounded change"
	tests := []struct {
		name string
		meta LoopPolicyMetadata
		want EffectiveLoopPolicy
	}{
		{
			name: "absent pair is implicit hold",
			want: EffectiveLoopPolicy{Source: "default", Policy: "hold", Reason: DefaultLoopReason},
		},
		{
			name: "explicit pair is returned verbatim",
			meta: LoopPolicyMetadata{Policy: &allow, Reason: &reason},
			want: EffectiveLoopPolicy{Source: "explicit", Policy: "allow", Reason: "bounded change"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveLoopPolicy(tt.meta); got != tt.want {
				t.Fatalf("ResolveLoopPolicy() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidateLoopPolicyMetadata(t *testing.T) {
	t.Parallel()

	value := func(s string) *string { return &s }
	tests := []struct {
		name string
		meta LoopPolicyMetadata
		want string
	}{
		{name: "absent", meta: LoopPolicyMetadata{}},
		{name: "allow", meta: LoopPolicyMetadata{Policy: value("allow"), Reason: value("bounded")}},
		{name: "hold", meta: LoopPolicyMetadata{Policy: value("hold"), Reason: value("operator decision")}},
		{name: "missing reason", meta: LoopPolicyMetadata{Policy: value("allow")}, want: "both be present or both absent"},
		{name: "missing policy", meta: LoopPolicyMetadata{Reason: value("bounded")}, want: "both be present or both absent"},
		{name: "null policy", meta: LoopPolicyMetadata{policyPresent: true, reasonPresent: true, Reason: value("bounded")}, want: "loop_policy must be a string"},
		{name: "null reason", meta: LoopPolicyMetadata{policyPresent: true, reasonPresent: true, Policy: value("allow")}, want: "loop_reason must be a string"},
		{name: "unknown policy", meta: LoopPolicyMetadata{Policy: value("ALLOW"), Reason: value("bounded")}, want: `loop_policy must be "allow" or "hold"`},
		{name: "empty reason", meta: LoopPolicyMetadata{Policy: value("allow"), Reason: value("")}, want: "between 1 and 512 bytes"},
		{name: "untrimmed reason", meta: LoopPolicyMetadata{Policy: value("allow"), Reason: value(" bounded ")}, want: "must be trimmed"},
		{name: "too long", meta: LoopPolicyMetadata{Policy: value("allow"), Reason: value(strings.Repeat("x", 513))}, want: "between 1 and 512 bytes"},
		{name: "invalid UTF-8", meta: LoopPolicyMetadata{Policy: value("allow"), Reason: value(string([]byte{0xff}))}, want: "valid UTF-8"},
		{name: "newline", meta: LoopPolicyMetadata{Policy: value("allow"), Reason: value("one\ntwo")}, want: "newline or control"},
		{name: "control", meta: LoopPolicyMetadata{Policy: value("allow"), Reason: value("one\ttwo")}, want: "newline or control"},
		{name: "artifact path", meta: LoopPolicyMetadata{Policy: value("allow"), Reason: value("see planning/artifacts/verify/T-001/run/report.json")}, want: "references gitignored artifact path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := ValidateLoopPolicyMetadata(tt.meta)
			if tt.want == "" && len(violations) != 0 {
				t.Fatalf("violations = %v, want none", violations)
			}
			if tt.want != "" && !containsString(violations, tt.want) {
				t.Fatalf("violations = %v, want substring %q", violations, tt.want)
			}
		})
	}
}

func TestValidateTaskLoopPolicyFrontmatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		extra string
		body  string
		want  string
	}{
		{name: "valid explicit pair", extra: "loop_policy: allow\nloop_reason: bounded change\n"},
		{name: "implicit hold"},
		{name: "half pair", extra: "loop_policy: allow\n", want: "both be present or both absent"},
		{name: "null pair", extra: "loop_policy: null\nloop_reason: ~\n", want: "must be a string"},
		{name: "empty scalars", extra: "loop_policy:\nloop_reason:\n", want: "must be a string"},
		{name: "field in body", body: "loop_policy: allow\n", want: "outside frontmatter"},
		{name: "duplicate key", extra: "loop_policy: allow\nloop_policy: hold\nloop_reason: bounded\n", want: "mapping key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := seedFixtureRepo(t)
			content := "---\nid: T-001\ntitle: Policy\nstatus: todo\npriority: high\nspec_ref: specs/v0.1.0.md#summary\ndependencies: []\n" + tt.extra + "updated_at: \"2026-03-31T00:00:00Z\"\n---\n\n# T-001 Policy\n\n" + tt.body
			writeFile(t, filepath.Join(repo, "planning", "tasks", "T-001.md"), content)

			result, err := newTestService(t, repo, time.Now()).Validate()
			if err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if tt.want == "" && !result.Valid {
				t.Fatalf("violations = %v, want valid", result.Violations)
			}
			if tt.want != "" && !containsString(result.Violations, tt.want) {
				t.Fatalf("violations = %v, want substring %q", result.Violations, tt.want)
			}
		})
	}
}

func TestTaskWritersPreserveLoopPolicyAndStateOmitsIt(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "Policy", "todo", "high", "specs/v0.1.0.md#summary", nil)
	path := filepath.Join(repo, "planning", "tasks", "T-001.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "updated_at:", "loop_policy: allow\nloop_reason: bounded change\nupdated_at:", 1))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestService(t, repo, time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC))
	writeFile(t, filepath.Join(repo, "specs", "v0.1.0.md"), "# Taskrail v0.1.0\n\n## Summary\n\nFixture spec.\n\n## Details\n\nMore.\n")
	if _, err := svc.RepointTask(RepointTaskInput{TaskID: "T-001", Area: "details"}); err != nil {
		t.Fatalf("RepointTask: %v", err)
	}
	assertTaskLoopPolicy(t, svc, "T-001")
	result, err := svc.RenameTask(RenameTaskInput{OldID: "T-001", Slug: "renamed", SlugExplicit: true})
	if err != nil {
		t.Fatalf("RenameTask: %v", err)
	}
	assertTaskLoopPolicy(t, svc, result.NewID)
	if _, err := svc.Start(result.NewID); err != nil {
		t.Fatalf("Start: %v", err)
	}
	assertTaskLoopPolicy(t, svc, result.NewID)
	if _, err := svc.Block(result.NewID, "manual stop"); err != nil {
		t.Fatalf("Block: %v", err)
	}
	assertTaskLoopPolicy(t, svc, result.NewID)
	stateData, err := os.ReadFile(filepath.Join(repo, "planning", "STATE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stateData), "loop_policy") || strings.Contains(string(stateData), "loop_reason") {
		t.Fatalf("STATE.md duplicated task-local policy:\n%s", stateData)
	}
}

func TestLifecycleWriterDoesNotHealExplicitNullLoopPolicy(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	content := `---
id: T-001
title: Null policy
status: todo
priority: high
spec_ref: specs/v0.1.0.md#summary
dependencies: []
loop_policy: null
loop_reason: null
updated_at: "2026-03-31T00:00:00Z"
---

# T-001 Null policy
`
	path := filepath.Join(repo, "planning", "tasks", "T-001.md")
	writeFile(t, path, content)
	svc := newTestService(t, repo, time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC))
	result, err := svc.Start("T-001")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if result.Validation.Valid || !containsString(result.Validation.Violations, "loop_policy must be a string") {
		t.Fatalf("post-write validation = %+v, want preserved null policy violation", result.Validation)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"loop_policy: null", "loop_reason: null"} {
		if !strings.Contains(string(data), field) {
			t.Fatalf("task write erased %q:\n%s", field, data)
		}
	}
}

func assertTaskLoopPolicy(t *testing.T, svc *Service, taskID string) {
	t.Helper()
	tasks, err := svc.loadTasks()
	if err != nil {
		t.Fatal(err)
	}
	task, err := exactTaskByID(tasks, taskID)
	if err != nil {
		t.Fatal(err)
	}
	got := ResolveLoopPolicy(task.Frontmatter.LoopPolicyMetadata)
	if got != (EffectiveLoopPolicy{Source: "explicit", Policy: "allow", Reason: "bounded change"}) {
		t.Fatalf("policy after task write = %+v", got)
	}
}

func TestValidateRefusesLegacyAutonomyFileWithoutReadingIt(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	legacy := filepath.Join(repo, "planning", "AUTONOMY.tsv")
	if err := os.Symlink(filepath.Join(repo, "missing-target"), legacy); err != nil {
		t.Fatal(err)
	}
	result, err := newTestService(t, repo, time.Now()).Validate()
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	for _, want := range []string{"AUTONOMY.tsv", "remove", "taskrail task loop allow|hold"} {
		if !containsString(result.Violations, want) {
			t.Fatalf("violations = %v, want remediation substring %q", result.Violations, want)
		}
	}
}

func containsString(values []string, substring string) bool {
	for _, value := range values {
		if strings.Contains(value, substring) {
			return true
		}
	}
	return false
}
