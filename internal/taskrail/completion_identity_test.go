package taskrail

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	firstCompletionID  = "11111111111111111111111111111111"
	secondCompletionID = "22222222222222222222222222222222"
	verificationID     = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

func TestCompletePersistsInjectedIdentityAndClearsTaskVerification(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "One", "in_progress", "high", "specs/v0.1.0.md#summary", nil)
	writeFixtureState(t, repo, "v0.1.0", "T-001", "One", "in_progress")
	taskPath := filepath.Join(repo, "planning", "tasks", "T-001.md")
	writeFile(t, taskPath, strings.Replace(readBytes(t, taskPath), "updated_at:", strings.Join([]string{
		"last_verification_id: " + verificationID,
		"last_verification_result: fail",
		`last_verified_at: "2026-08-18T10:00:00Z"`,
		"updated_at:",
	}, "\n"), 1))
	statePath := filepath.Join(repo, "planning", "STATE.md")
	stateBefore := readBytes(t, statePath)
	svc := newTestService(t, repo, time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC))
	svc.completionID = func() (string, error) { return firstCompletionID, nil }

	result, err := svc.Complete("T-001", "shipped")
	if err != nil {
		t.Fatal(err)
	}
	if result.CompletionID != firstCompletionID {
		t.Fatalf("completion id = %q, want %q", result.CompletionID, firstCompletionID)
	}
	got := readBytes(t, taskPath)
	if !strings.Contains(got, `completion_id: "`+firstCompletionID+`"`) {
		t.Fatalf("task did not persist completion id:\n%s", got)
	}
	for _, field := range []string{"last_verification_id:", "last_verification_previous_id:", "last_verification_result:", "last_verified_at:", "last_verified_completion_id:"} {
		if strings.Contains(got, field) {
			t.Errorf("complete retained %s\n%s", field, got)
		}
	}
	stateAfter := readBytes(t, statePath)
	beforeHistory := stateVerificationHistory(stateBefore)
	afterHistory := stateVerificationHistory(stateAfter)
	if afterHistory != beforeHistory {
		t.Fatalf("repository verification history changed from %q to %q", beforeHistory, afterHistory)
	}
}

func TestCompleteRegeneratesPreflightCollisionAndReplacesCompletionID(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "One", "in_progress", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002", "Historical", "completed", "low", "specs/v0.1.0.md#summary", nil)
	historical := filepath.Join(repo, "planning", "tasks", "T-002.md")
	writeFile(t, historical, strings.Replace(readBytes(t, historical), "updated_at:", "completion_id: "+firstCompletionID+"\nupdated_at:", 1))
	writeFixtureState(t, repo, "v0.1.0", "T-001", "One", "in_progress")
	svc := newTestService(t, repo, time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC))
	ids := []string{firstCompletionID, secondCompletionID}
	svc.completionID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}

	result, err := svc.Complete("T-001", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.CompletionID != secondCompletionID {
		t.Fatalf("completion id = %q, want non-colliding %q", result.CompletionID, secondCompletionID)
	}
}

func TestCompleteIdentityFailurePublishesNothing(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "One", "in_progress", "high", "specs/v0.1.0.md#summary", nil)
	writeFixtureState(t, repo, "v0.1.0", "T-001", "One", "in_progress")
	svc := newTestService(t, repo, time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC))
	svc.completionID = func() (string, error) { return "", errors.New("entropy unavailable") }
	before := snapshotTree(t, repo)

	if _, err := svc.Complete("T-001", ""); err == nil || !strings.Contains(err.Error(), "entropy unavailable") {
		t.Fatalf("complete error = %v, want entropy failure", err)
	}
	if got := snapshotTree(t, repo); !mapEqual(got, before) {
		t.Fatal("complete published bytes after identity generation failed")
	}
}

func TestRepeatedCompleteReplacesIdentityAndClearsVerificationAgain(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "One", "in_progress", "high", "specs/v0.1.0.md#summary", nil)
	writeFixtureState(t, repo, "v0.1.0", "T-001", "One", "in_progress")
	svc := newTestService(t, repo, time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC))
	svc.completionID = func() (string, error) { return firstCompletionID, nil }
	if _, err := svc.Complete("T-001", "first"); err != nil {
		t.Fatal(err)
	}

	taskPath := filepath.Join(repo, "planning", "tasks", "T-001.md")
	writeFile(t, taskPath, strings.NewReplacer(
		"status: completed", "status: in_progress",
		"updated_at:", "last_verification_id: "+verificationID+"\nlast_verification_result: fail\nlast_verified_at: \"2026-08-18T12:30:00Z\"\nupdated_at:",
	).Replace(readBytes(t, taskPath)))
	writeFixtureState(t, repo, "v0.1.0", "T-001", "One", "in_progress")
	svc = newTestService(t, repo, time.Date(2026, 8, 18, 13, 0, 0, 0, time.UTC))
	svc.completionID = func() (string, error) { return secondCompletionID, nil }

	result, err := svc.Complete("T-001", "second")
	if err != nil {
		t.Fatal(err)
	}
	if result.CompletionID != secondCompletionID {
		t.Fatalf("completion id = %q, want replacement %q", result.CompletionID, secondCompletionID)
	}
	got := readBytes(t, taskPath)
	if strings.Contains(got, firstCompletionID) || strings.Contains(got, "last_verification_") || strings.Contains(got, "last_verified_at:") {
		t.Fatalf("repeat complete retained superseded metadata:\n%s", got)
	}
}

func TestNonCompleteLifecycleWritersPreserveLifecycleMetadataBytes(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "One", "todo", "high", "specs/v0.1.0.md#summary", nil)
	taskPath := filepath.Join(repo, "planning", "tasks", "T-001.md")
	metadata := strings.Join([]string{
		`completion_id: "` + firstCompletionID + `"`,
		`last_verification_id: "` + verificationID + `"`,
		"last_verification_result: pass",
		`last_verified_at: "2026-08-18T10:00:00Z"`,
		`last_verified_completion_id: "` + firstCompletionID + `"`,
	}, "\n")
	writeFile(t, taskPath, strings.Replace(readBytes(t, taskPath), "updated_at:", metadata+"\nupdated_at:", 1))
	svc := newTestService(t, repo, time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC))

	if _, err := svc.Start("T-001"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Block("T-001", "pause"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Unblock("T-001", "resume"); err != nil {
		t.Fatal(err)
	}
	if got := lifecycleMetadataLines(readBytes(t, taskPath)); got != metadata {
		t.Fatalf("lifecycle metadata changed:\nwant:\n%s\ngot:\n%s", metadata, got)
	}
}

func stateVerificationHistory(state string) string {
	var history []string
	for _, line := range strings.Split(state, "\n") {
		if strings.HasPrefix(line, "last_verification_") || strings.HasPrefix(line, "last_verified_completion_id:") {
			history = append(history, line)
		}
	}
	return strings.Join(history, "\n")
}

func lifecycleMetadataLines(task string) string {
	var metadata []string
	for _, line := range strings.Split(task, "\n") {
		if strings.HasPrefix(line, "completion_id:") || strings.HasPrefix(line, "last_verification_") || strings.HasPrefix(line, "last_verified_") {
			metadata = append(metadata, line)
		}
	}
	return strings.Join(metadata, "\n")
}

func TestLifecycleMetadataRejectsPresentEmptyAndMalformedIDs(t *testing.T) {
	valid := CompletionVerificationMetadata{CompletionID: firstCompletionID}
	valid.completionIDPresent = true
	if violations := ValidateCompletionVerificationMetadata("completed", valid); len(violations) != 0 {
		t.Fatalf("valid completion rejected: %v", violations)
	}

	tests := []CompletionVerificationMetadata{
		{completionIDPresent: true},
		{CompletionID: "ABCDEF0123456789ABCDEF0123456789", completionIDPresent: true},
		{LastVerificationID: "not-hex", LastVerificationResult: "fail", LastVerifiedAt: "2026-08-18T12:00:00Z", lastVerificationIDPresent: true, lastVerificationResultPresent: true, lastVerifiedAtPresent: true},
		{LastVerificationPreviousID: "", lastVerificationPreviousIDPresent: true},
	}
	for i, meta := range tests {
		if violations := ValidateCompletionVerificationMetadata("completed", meta); len(violations) == 0 {
			t.Errorf("malformed metadata case %d accepted: %+v", i, meta)
		}
	}
}

func TestTaskFrontmatterTracksNullLifecycleMetadataAsPresent(t *testing.T) {
	data := []byte("---\nid: T-001\ntitle: One\nstatus: completed\npriority: high\nspec_ref: specs/v0.1.0.md#summary\ndependencies: []\nupdated_at: \"2026-08-18T12:00:00Z\"\ncompletion_id: null\n---\n")
	fm, _, err := parseFrontmatter[TaskFrontmatter](data)
	if err != nil {
		t.Fatal(err)
	}
	if !fm.completionIDPresent {
		t.Fatal("explicit null completion_id was treated as omitted")
	}
	if violations := ValidateCompletionVerificationMetadata(fm.Status, fm.CompletionVerificationMetadata); len(violations) == 0 {
		t.Fatal("explicit null completion_id was accepted")
	}
}
