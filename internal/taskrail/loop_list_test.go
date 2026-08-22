package taskrail

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTaskLoopListReportsPoliciesAndHeldDependencies(t *testing.T) {
	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	writeTask(t, repo, "T-001-held", "Held", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002-allowed", "Allowed", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-001-held"})
	writeTask(t, repo, "T-003-ready", "Ready", "todo", "high", "specs/v0.1.0.md#summary", nil)
	setLoopPolicy(t, repo, "T-002-allowed", "allow", "bounded work")
	setLoopPolicy(t, repo, "T-003-ready", "allow", "independent work")

	before := snapshotTree(t, repo)
	report, err := svc.TaskLoopList()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(report.Violations) != 0 {
		t.Fatalf("violations = %+v", report.Violations)
	}
	if got, want := report.Tasks[0].Disposition, "held_dependency"; got != want {
		t.Fatalf("held disposition = %q, want %q", got, want)
	}
	if got, want := report.Tasks[1].HeldDependencies, []string{"T-001-held"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("allowed held dependencies = %v, want %v", got, want)
	}
	if got, want := report.Tasks[1].Disposition, "waiting_dependency"; got != want {
		t.Fatalf("allowed disposition = %q, want %q", got, want)
	}
	if got, want := report.Tasks[2].Disposition, "eligible"; got != want || !report.Tasks[2].Eligible {
		t.Fatalf("ready row = %+v, want eligible", report.Tasks[2])
	}
	if after := snapshotTree(t, repo); !reflect.DeepEqual(after, before) {
		t.Fatal("task loop list changed repository bytes")
	}
}

func TestTaskLoopListReportsDecodableInvalidTaskAndUndecodableViolation(t *testing.T) {
	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	writeTask(t, repo, "T-001-invalid", "Invalid", "todo", "high", "specs/v0.1.0.md#summary", nil)
	setLoopPolicy(t, repo, "T-001-invalid", "ALLOW", "not valid")
	writeFile(t, repo+"/planning/tasks/broken.md", "not frontmatter\n")

	report, err := svc.TaskLoopList()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(report.Tasks) != 1 || report.Tasks[0].Disposition != "invalid" {
		t.Fatalf("tasks = %+v, want one invalid decoded row", report.Tasks)
	}
	if len(report.Violations) == 0 {
		t.Fatal("expected invalid-repository violations")
	}
	foundBroken := false
	for _, violation := range report.Violations {
		if violation.Path != nil && *violation.Path == "planning/tasks/broken.md" {
			foundBroken = true
		}
	}
	if !foundBroken {
		t.Fatalf("undecodable file was not a path-bearing violation: %+v", report.Violations)
	}
	if got := report.Tasks[0].EffectivePolicy; got != "hold" || report.Tasks[0].Source != "default" {
		t.Fatalf("invalid policy leaked into row: %+v", report.Tasks[0])
	}
}

func TestTaskLoopListKeepsIdentifiedParseFailuresAndOmitsMissingIdentity(t *testing.T) {
	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	writeTask(t, repo, "T-001-duplicate", "Duplicate", "todo", "high", "specs/v0.1.0.md#summary", nil)
	duplicatePath := repo + "/planning/tasks/T-001-duplicate.md"
	duplicate := strings.Replace(readFileString(t, duplicatePath), "priority: high", "priority: high\npriority: low", 1)
	writeFile(t, duplicatePath, duplicate)
	writeTask(t, repo, "T-002-no-id", "No identity", "todo", "high", "specs/v0.1.0.md#summary", nil)
	missingPath := repo + "/planning/tasks/T-002-no-id.md"
	missing := strings.Replace(readFileString(t, missingPath), "id: T-002-no-id\n", "", 1)
	writeFile(t, missingPath, missing)

	report, err := svc.TaskLoopList()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(report.Tasks) != 1 || report.Tasks[0].TaskID != "T-001-duplicate" || report.Tasks[0].Disposition != "invalid" {
		t.Fatalf("rows = %+v, want only identified invalid row", report.Tasks)
	}
	foundMissingID := false
	for _, violation := range report.Violations {
		if violation.Path != nil && *violation.Path == "planning/tasks/T-002-no-id.md" {
			foundMissingID = true
		}
	}
	if !foundMissingID {
		t.Fatalf("missing identity did not produce a path-bearing violation: %+v", report.Violations)
	}
}

func TestTaskLoopListMarksEveryCycleTaskInvalid(t *testing.T) {
	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	writeTask(t, repo, "T-001-a", "A", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-002-b"})
	writeTask(t, repo, "T-002-b", "B", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-001-a"})

	report, err := svc.TaskLoopList()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, row := range report.Tasks {
		if row.Disposition != "invalid" {
			t.Fatalf("cycle row = %+v, want invalid", row)
		}
	}
}

func TestTaskLoopListOmitsAmbiguousDuplicateIdentity(t *testing.T) {
	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	writeTask(t, repo, "T-001-ambiguous", "Ambiguous", "todo", "high", "specs/v0.1.0.md#summary", nil)
	path := repo + "/planning/tasks/T-001-ambiguous.md"
	data := strings.Replace(readFileString(t, path), "id: T-001-ambiguous", "id: T-001-ambiguous\nid: T-002-other", 1)
	writeFile(t, path, data)

	report, err := svc.TaskLoopList()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(report.Tasks) != 0 {
		t.Fatalf("ambiguous identity produced rows: %+v", report.Tasks)
	}
	if len(report.Violations) == 0 || report.Violations[0].Path == nil {
		t.Fatalf("ambiguous identity did not produce a path-bearing violation: %+v", report.Violations)
	}
}

func setLoopPolicy(t *testing.T, repo, id, policy, reason string) {
	t.Helper()
	path := repo + "/planning/tasks/" + id + ".md"
	data := readFileString(t, path)
	data = strings.Replace(data, "updated_at:", "loop_policy: "+policy+"\nloop_reason: "+reason+"\nupdated_at:", 1)
	writeFile(t, path, data)
}
