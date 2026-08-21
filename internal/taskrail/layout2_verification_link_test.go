package taskrail

import "testing"

func TestMigrationRejectsStateVerificationTupleMismatchingTask(t *testing.T) {
	const (
		stateID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		taskID  = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		time    = "2026-08-21T12:00:00Z"
	)
	task := &Task{Frontmatter: TaskFrontmatter{
		ID: "T-001", CompletionVerificationMetadata: CompletionVerificationMetadata{
			LastVerificationID: taskID, LastVerificationResult: "fail", LastVerifiedAt: time,
		},
	}, Body: verificationNoteLine(time, "fail", taskID, "")}
	state := stateV2Frontmatter{
		LastVerificationID:     stateID,
		LastVerificationResult: "fail for T-001 at " + time + " id " + stateID,
	}
	if err := validateMigrationStateTaskLinks(state, []*Task{task}); err == nil {
		t.Fatal("migration accepted a state verification tuple that does not match its task")
	}
}
