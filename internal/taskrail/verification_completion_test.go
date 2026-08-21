package taskrail

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVerifyPassBindsCurrentCompletionEverywhere(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "One", "completed", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	svc.completionID = func() (string, error) { return firstCompletionID, nil }
	svc.verificationID = func() (string, error) { return firstVerificationID, nil }

	result, err := svc.Verify(VerifyInput{TaskID: "T-001", Result: "pass", Summary: "verified"})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.ObservedCompletionID == nil || *result.ObservedCompletionID != firstCompletionID {
		t.Fatalf("observed completion = %v, want %q", result.ObservedCompletionID, firstCompletionID)
	}

	data, err := os.ReadFile(filepath.Join(repo, result.ReportPath))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var report VerificationArtifact
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.ObservedCompletionID == nil || *report.ObservedCompletionID != firstCompletionID {
		t.Fatalf("report observed completion = %v, want %q", report.ObservedCompletionID, firstCompletionID)
	}

	state, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	task, _ := taskByID(tasks, "T-001")
	if task.Frontmatter.CompletionID != firstCompletionID || task.Frontmatter.LastVerifiedCompletionID != firstCompletionID {
		t.Fatalf("task completion binding = %+v", task.Frontmatter.CompletionVerificationMetadata)
	}
	if state.Frontmatter.LastVerifiedCompletionID != firstCompletionID {
		t.Fatalf("state completion binding = %q", state.Frontmatter.LastVerifiedCompletionID)
	}
	if !strings.Contains(task.Body, "completion "+firstCompletionID) {
		t.Fatalf("task note omitted completion binding: %s", task.Body)
	}
	if validation, err := svc.Validate(); err != nil || !validation.Valid {
		t.Fatalf("bound pass rejected: validation=%+v err=%v", validation, err)
	}
}

func TestVerifyLeavesNonCurrentOutcomesUnbound(t *testing.T) {
	tests := []struct {
		name   string
		status string
		result string
	}{
		{"pass before completion", "in_progress", "pass"},
		{"completed fail", "completed", "fail"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := seedFixtureRepo(t)
			initialStatus := tt.status
			if initialStatus == "in_progress" {
				initialStatus = "todo"
			}
			writeTask(t, repo, "T-001", "One", initialStatus, "high", "specs/v0.1.0.md#summary", nil)
			svc := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
			svc.verificationID = func() (string, error) { return firstVerificationID, nil }
			if tt.status == "in_progress" {
				if _, err := svc.Start("T-001"); err != nil {
					t.Fatalf("start: %v", err)
				}
			}

			result, err := svc.Verify(VerifyInput{TaskID: "T-001", Result: tt.result, Summary: "verified"})
			if err != nil {
				t.Fatalf("verify: %v", err)
			}
			if result.ObservedCompletionID != nil {
				t.Fatalf("observed completion = %q, want nil", *result.ObservedCompletionID)
			}
			state, tasks, err := svc.loadStateAndTasks()
			if err != nil {
				t.Fatalf("reload: %v", err)
			}
			task, _ := taskByID(tasks, "T-001")
			if task.Frontmatter.Status != tt.status || task.Frontmatter.CompletionID != "" || task.Frontmatter.LastVerifiedCompletionID != "" || state.Frontmatter.LastVerifiedCompletionID != "" {
				t.Fatalf("unbound outcome mutated lifecycle or binding: task=%+v state=%+v", task.Frontmatter.CompletionVerificationMetadata, state.Frontmatter)
			}
			if validation, err := svc.Validate(); err != nil || !validation.Valid {
				t.Fatalf("unbound outcome rejected: validation=%+v err=%v", validation, err)
			}
		})
	}
}

func TestVerifyCompletedAuditFailRequiresCurrentBoundPass(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "One", "completed", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	ids := []string{firstVerificationID, secondVerificationID}
	svc.verificationID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
	if _, err := svc.Verify(VerifyInput{TaskID: "T-001", Result: "pass", Summary: "first"}); err != nil {
		t.Fatalf("bound pass: %v", err)
	}
	fail, err := svc.Verify(VerifyInput{TaskID: "T-001", Result: "fail", Summary: "audit"})
	if err != nil {
		t.Fatalf("audit fail: %v", err)
	}
	if fail.PreviousVerificationID == nil || *fail.PreviousVerificationID != firstVerificationID || fail.ObservedCompletionID != nil {
		t.Fatalf("audit fail links = previous %v observed %v", fail.PreviousVerificationID, fail.ObservedCompletionID)
	}
	if validation, err := svc.Validate(); err != nil || !validation.Valid {
		t.Fatalf("audit fail rejected: validation=%+v err=%v", validation, err)
	}
}

func TestVerifyLegacyCompletionAdoptionRollsBackOnCandidateFault(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "One", "completed", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	svc.completionID = func() (string, error) { return firstCompletionID, nil }
	svc.verificationID = func() (string, error) { return firstVerificationID, nil }
	taskPath := filepath.Join(repo, "planning", "tasks", "T-001.md")
	statePath := filepath.Join(repo, "planning", "STATE.md")
	taskBefore := readBytes(t, taskPath)
	stateBefore := readBytes(t, statePath)
	testHookWriterValidated = func() {
		writeTask(t, repo, "T-009", "Concurrent", "todo", "low", "specs/v0.1.0.md#summary", nil)
	}
	t.Cleanup(func() { testHookWriterValidated = nil })

	if _, err := svc.Verify(VerifyInput{TaskID: "T-001", Result: "pass", Summary: "verified"}); err == nil {
		t.Fatal("candidate fault accepted legacy adoption")
	}
	if got := readBytes(t, taskPath); got != taskBefore {
		t.Fatalf("failed adoption changed task bytes:\n%s", got)
	}
	if got := readBytes(t, statePath); got != stateBefore {
		t.Fatalf("failed adoption changed state bytes:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(repo, "planning", "artifacts", "verify", "T-001")); !os.IsNotExist(err) {
		t.Fatalf("failed adoption left verification artifacts: %v", err)
	}
}
