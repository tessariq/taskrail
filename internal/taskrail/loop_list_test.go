package taskrail

import (
	"os"
	"path/filepath"
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

func TestTaskLoopSelectFiltersAllowedTasksAndPreservesStatusRanking(t *testing.T) {
	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	writeFile(t, repo+"/specs/v0.2.0.md", "# Taskrail v0.2.0\n\n## Summary\n")
	writeTask(t, repo, "T-001-held", "Held", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002-waiting", "Waiting", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-001-held"})
	writeTask(t, repo, "T-003-low", "Low", "todo", "low", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-004-high", "High", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-005-off-spec", "Off spec", "todo", "high", "specs/v0.2.0.md#summary", nil)
	setLoopPolicy(t, repo, "T-002-waiting", "allow", "blocked by held dependency")
	setLoopPolicy(t, repo, "T-003-low", "allow", "independent low priority work")
	setLoopPolicy(t, repo, "T-004-high", "allow", "independent high priority work")
	setLoopPolicy(t, repo, "T-005-off-spec", "allow", "old spec work")

	before := snapshotTree(t, repo)
	first, err := svc.TaskLoopSelect()
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if first.Action != "run" || first.SelectedTask == nil || first.SelectedTask.TaskID != "T-004-high" {
		t.Fatalf("selection = %+v, want T-004-high", first)
	}
	if first.Tasks[1].Disposition != "waiting_dependency" || !reflect.DeepEqual(first.Tasks[1].HeldDependencies, []string{"T-001-held"}) {
		t.Fatalf("waiting row = %+v", first.Tasks[1])
	}
	if first.Tasks[4].Disposition != "off_spec" {
		t.Fatalf("off-spec row = %+v", first.Tasks[4])
	}
	second, err := svc.TaskLoopSelect()
	if err != nil {
		t.Fatalf("repeat select: %v", err)
	}
	if !reflect.DeepEqual(second, first) {
		t.Fatalf("repeat selection = %+v, want %+v", second, first)
	}
	if after := snapshotTree(t, repo); !reflect.DeepEqual(after, before) {
		t.Fatal("task loop selection changed repository bytes")
	}
}

func TestTaskLoopSelectReportsNoneAndInvalid(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		repo := initGitRepo(t)
		svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
		if _, err := svc.Init(InitInput{}); err != nil {
			t.Fatalf("init: %v", err)
		}
		writeTask(t, repo, "T-001-held", "Held", "todo", "high", "specs/v0.1.0.md#summary", nil)

		result, err := svc.TaskLoopSelect()
		if err != nil {
			t.Fatalf("select: %v", err)
		}
		if result.Action != "none" || result.SelectedTask != nil || result.Reason != "no eligible allowed task" {
			t.Fatalf("result = %+v", result)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		repo := initGitRepo(t)
		svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
		if _, err := svc.Init(InitInput{}); err != nil {
			t.Fatalf("init: %v", err)
		}
		writeTask(t, repo, "T-001-invalid", "Invalid", "todo", "high", "specs/v0.1.0.md#summary", nil)
		setLoopPolicy(t, repo, "T-001-invalid", "ALLOW", "not valid")

		result, err := svc.TaskLoopSelect()
		if err != nil {
			t.Fatalf("select: %v", err)
		}
		if result.Action != "invalid" || result.SelectedTask != nil || len(result.Violations) == 0 {
			t.Fatalf("result = %+v", result)
		}
	})
}

func TestTaskLoopSelectSkipsAllowedLifecycleStates(t *testing.T) {
	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	writeTask(t, repo, "T-001-blocked", "Blocked", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002-completed", "Completed", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-003-active", "Active", "todo", "high", "specs/v0.1.0.md#summary", nil)
	for _, id := range []string{"T-001-blocked", "T-002-completed", "T-003-active"} {
		setLoopPolicy(t, repo, id, "allow", "lifecycle state fixture")
	}
	if _, err := svc.Block("T-001-blocked", "operator decision"); err != nil {
		t.Fatalf("block: %v", err)
	}
	if _, err := svc.Start("T-002-completed"); err != nil {
		t.Fatalf("start completed fixture: %v", err)
	}
	if _, err := svc.Complete("T-002-completed", "delivered"); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if _, err := svc.Start("T-003-active"); err != nil {
		t.Fatalf("start active fixture: %v", err)
	}

	result, err := svc.TaskLoopSelect()
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if result.Action != "none" || result.SelectedTask != nil {
		t.Fatalf("result = %+v", result)
	}
	for _, row := range result.Tasks {
		if row.Disposition != "status_ineligible" {
			t.Fatalf("lifecycle row = %+v, want status_ineligible", row)
		}
	}
}

func TestTaskLoopSelectRejectsInvalidVerificationEvidence(t *testing.T) {
	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	writeTask(t, repo, "T-001-verified", "Verified", "completed", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002-candidate", "Candidate", "todo", "high", "specs/v0.1.0.md#summary", nil)
	setLoopPolicy(t, repo, "T-002-candidate", "allow", "independent work")
	verification, err := svc.Verify(VerifyInput{TaskID: "T-001-verified", Result: "fail", Summary: "fixture verification"})
	if err != nil {
		t.Fatalf("verify fixture: %v", err)
	}
	if err := os.Remove(filepath.Join(repo, verification.ReportPath)); err != nil {
		t.Fatalf("remove verification report: %v", err)
	}

	result, err := svc.TaskLoopSelect()
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if result.Action != "invalid" || result.SelectedTask != nil || !hasMachineViolation(result.Violations, "verification evidence") {
		t.Fatalf("result = %+v", result)
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

func hasMachineViolation(violations []MachineViolation, want string) bool {
	for _, violation := range violations {
		if strings.Contains(violation.Message, want) {
			return true
		}
	}
	return false
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
