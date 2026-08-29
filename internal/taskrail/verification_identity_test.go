package taskrail

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	firstVerificationID  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	secondVerificationID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestVerifyPublishesStableIdentityAndPredecessor(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "One", "completed", "high", "specs/v0.1.0.md#summary", nil)
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	svc := newTestService(t, repo, now)
	ids := []string{firstVerificationID, secondVerificationID}
	svc.verificationID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}

	first, err := svc.Verify(VerifyInput{TaskID: "T-001", Result: "fail", Summary: "first"})
	if err != nil {
		t.Fatalf("first verify: %v", err)
	}
	if first.VerificationID != firstVerificationID || first.PreviousVerificationID != nil {
		t.Fatalf("first identity = (%q, %v)", first.VerificationID, first.PreviousVerificationID)
	}

	second, err := svc.Verify(VerifyInput{TaskID: "T-001", Result: "fail", Summary: "second"})
	if err != nil {
		t.Fatalf("second verify: %v", err)
	}
	if second.VerificationID != secondVerificationID || second.PreviousVerificationID == nil || *second.PreviousVerificationID != firstVerificationID {
		t.Fatalf("second identity = (%q, %v)", second.VerificationID, second.PreviousVerificationID)
	}

	for _, result := range []VerifyResult{first, second} {
		if !strings.HasSuffix(result.ArtifactDir, "-"+result.VerificationID) {
			t.Fatalf("artifact dir %q does not contain verification id %q", result.ArtifactDir, result.VerificationID)
		}
		data, err := os.ReadFile(filepath.Join(repo, result.ReportPath))
		if err != nil {
			t.Fatalf("read report: %v", err)
		}
		var report VerificationArtifact
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatalf("decode report: %v", err)
		}
		if report.VerificationID != result.VerificationID || !sameOptionalID(report.PreviousVerificationID, result.PreviousVerificationID) {
			t.Fatalf("report identity = (%q, %v), result = (%q, %v)", report.VerificationID, report.PreviousVerificationID, result.VerificationID, result.PreviousVerificationID)
		}
	}

	_, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	task, _ := taskByID(tasks, "T-001")
	if task.Frontmatter.LastVerificationID != secondVerificationID || task.Frontmatter.LastVerificationPreviousID != firstVerificationID {
		t.Fatalf("task verification tuple = %+v", task.Frontmatter.CompletionVerificationMetadata)
	}
	if !strings.Contains(task.Body, "verification fail id "+secondVerificationID+" previous "+firstVerificationID+" completion none") {
		t.Fatalf("task note omitted identity: %s", task.Body)
	}

	state, err := svc.loadState()
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	wantSummary := "fail for T-001 at 2026-08-21T12:00:00Z id " + secondVerificationID
	if state.Frontmatter.LastVerificationResult != wantSummary {
		t.Fatalf("state summary = %q, want %q", state.Frontmatter.LastVerificationResult, wantSummary)
	}
	if state.Frontmatter.LastVerificationID != secondVerificationID || state.Frontmatter.LastVerificationPreviousID != firstVerificationID {
		t.Fatalf("state verification tuple = (%q, %q)", state.Frontmatter.LastVerificationID, state.Frontmatter.LastVerificationPreviousID)
	}
	if validation, err := svc.Validate(); err != nil || !validation.Valid {
		t.Fatalf("valid chain rejected: validation=%+v err=%v", validation, err)
	}
	clone := seedFixtureRepo(t)
	writeTask(t, clone, "T-001", "One", "completed", "high", "specs/v0.1.0.md#summary", nil)
	cloneSvc := newTestService(t, clone, now)
	cloneState, cloneTasks, err := cloneSvc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load clone: %v", err)
	}
	cloneState.Frontmatter = state.Frontmatter
	cloneState.Body = state.Body
	cloneTask, _ := taskByID(cloneTasks, "T-001")
	cloneTask.Frontmatter = task.Frontmatter
	cloneTask.Body = task.Body
	if err := cloneSvc.saveState(cloneState); err != nil {
		t.Fatalf("write clone state: %v", err)
	}
	if err := cloneSvc.saveTask(cloneTask); err != nil {
		t.Fatalf("write clone task: %v", err)
	}
	if validation, err := cloneSvc.Validate(); err != nil || !validation.Valid {
		t.Fatalf("artifact-free clone rejected: validation=%+v err=%v", validation, err)
	}
	task.Body = strings.Replace(task.Body, "verification fail id "+secondVerificationID, "verification fail id "+firstVerificationID, 1)
	if err := svc.saveTask(task); err != nil {
		t.Fatalf("tamper task note: %v", err)
	}
	if validation, err := svc.Validate(); err != nil || validation.Valid || !hasViolation(validation.Violations, "task note") {
		t.Fatalf("tampered task note accepted: validation=%+v err=%v", validation, err)
	}
	task.Body = strings.Replace(task.Body, "verification fail id "+firstVerificationID, "verification fail id "+secondVerificationID, 1)
	if err := svc.saveTask(task); err != nil {
		t.Fatalf("restore task note: %v", err)
	}
	if err := os.Remove(filepath.Join(repo, first.ReportPath)); err != nil {
		t.Fatalf("remove predecessor report: %v", err)
	}
	if validation, err := svc.Validate(); err != nil || validation.Valid || !hasViolation(validation.Violations, "verification evidence") {
		t.Fatalf("missing report in available predecessor artifact rejected: validation=%+v err=%v", validation, err)
	}
}

func TestValidateAllowsMissingPredecessorArtifactInFreshClone(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "One", "completed", "high", "specs/v0.1.0.md#summary", nil)
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	svc := newTestService(t, repo, now)
	ids := []string{firstVerificationID, secondVerificationID}
	svc.verificationID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
	if _, err := svc.Verify(VerifyInput{TaskID: "T-001", Result: "fail", Summary: "first"}); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	second, err := svc.Verify(VerifyInput{TaskID: "T-001", Result: "fail", Summary: "second"})
	if err != nil {
		t.Fatalf("second verify: %v", err)
	}

	clone := seedFixtureRepo(t)
	writeTask(t, clone, "T-001", "One", "completed", "high", "specs/v0.1.0.md#summary", nil)
	cloneSvc := newTestService(t, clone, now)
	state, tasks, err := svc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load source: %v", err)
	}
	cloneState, cloneTasks, err := cloneSvc.loadStateAndTasks()
	if err != nil {
		t.Fatalf("load clone: %v", err)
	}
	cloneState.Frontmatter, cloneState.Body = state.Frontmatter, state.Body
	cloneTask, _ := taskByID(cloneTasks, "T-001")
	sourceTask, _ := taskByID(tasks, "T-001")
	cloneTask.Frontmatter, cloneTask.Body = sourceTask.Frontmatter, sourceTask.Body
	if err := cloneSvc.saveState(cloneState); err != nil {
		t.Fatalf("write clone state: %v", err)
	}
	if err := cloneSvc.saveTask(cloneTask); err != nil {
		t.Fatalf("write clone task: %v", err)
	}

	report, err := os.ReadFile(filepath.Join(repo, second.ReportPath))
	if err != nil {
		t.Fatalf("read latest report: %v", err)
	}
	writeFile(t, filepath.Join(clone, second.ReportPath), string(report))

	if validation, err := cloneSvc.Validate(); err != nil || !validation.Valid {
		t.Fatalf("fresh clone with only latest report rejected: validation=%+v err=%v", validation, err)
	}
}

func TestValidateRejectsAvailableMalformedOrContradictoryPredecessorEvidence(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(t *testing.T, path string)
	}{
		{
			name: "malformed report",
			mutate: func(t *testing.T, path string) {
				writeFile(t, path, "{")
			},
		},
		{
			name: "cross-task report",
			mutate: func(t *testing.T, path string) {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read predecessor report: %v", err)
				}
				var report VerificationArtifact
				if err := json.Unmarshal(data, &report); err != nil {
					t.Fatalf("decode predecessor report: %v", err)
				}
				report.TaskID = "T-999"
				data, err = json.Marshal(report)
				if err != nil {
					t.Fatalf("encode predecessor report: %v", err)
				}
				writeFile(t, path, string(data))
			},
		},
		{
			name: "cyclic report",
			mutate: func(t *testing.T, path string) {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read predecessor report: %v", err)
				}
				var report VerificationArtifact
				if err := json.Unmarshal(data, &report); err != nil {
					t.Fatalf("decode predecessor report: %v", err)
				}
				previous := secondVerificationID
				report.PreviousVerificationID = &previous
				data, err = json.Marshal(report)
				if err != nil {
					t.Fatalf("encode predecessor report: %v", err)
				}
				writeFile(t, path, string(data))
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := seedFixtureRepo(t)
			writeTask(t, repo, "T-001", "One", "completed", "high", "specs/v0.1.0.md#summary", nil)
			svc := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
			ids := []string{firstVerificationID, secondVerificationID}
			svc.verificationID = func() (string, error) {
				id := ids[0]
				ids = ids[1:]
				return id, nil
			}
			first, err := svc.Verify(VerifyInput{TaskID: "T-001", Result: "fail", Summary: "first"})
			if err != nil {
				t.Fatalf("first verify: %v", err)
			}
			if _, err := svc.Verify(VerifyInput{TaskID: "T-001", Result: "fail", Summary: "second"}); err != nil {
				t.Fatalf("second verify: %v", err)
			}

			tt.mutate(t, filepath.Join(repo, first.ReportPath))
			if validation, err := svc.Validate(); err != nil || validation.Valid || !hasViolation(validation.Violations, "verification evidence") {
				t.Fatalf("invalid available predecessor accepted: validation=%+v err=%v", validation, err)
			}
		})
	}
}

func sameOptionalID(left, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func TestVerifyRegeneratesArtifactIdentityCollision(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "One", "completed", "high", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 8, 21, 13, 0, 0, 0, time.UTC))
	if err := os.MkdirAll(filepath.Join(svc.paths.VerifyDir, "T-001", "legacy-"+firstVerificationID), 0o755); err != nil {
		t.Fatalf("seed collision: %v", err)
	}
	ids := []string{firstVerificationID, secondVerificationID}
	svc.verificationID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}

	result, err := svc.Verify(VerifyInput{TaskID: "T-001", Result: "fail", Summary: "collision retry"})
	if err != nil {
		t.Fatalf("verify after collision: %v", err)
	}
	if result.VerificationID != secondVerificationID {
		t.Fatalf("verification id = %q, want collision-free %q", result.VerificationID, secondVerificationID)
	}
}
