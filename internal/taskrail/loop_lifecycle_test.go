package taskrail

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDeriveLoopLifecycleClassifiesFreshEvidence(t *testing.T) {
	const (
		taskID       = "T-001-example"
		completionID = "11111111111111111111111111111111"
		passID       = "22222222222222222222222222222222"
		failID       = "33333333333333333333333333333333"
		timestamp    = "2026-08-23T12:00:00Z"
	)
	tests := []struct {
		name      string
		status    string
		result    string
		candidate string
		audit     bool
	}{
		{name: "completed pass", status: "completed", result: "pass", candidate: "completed_pass"},
		{name: "blocked fail", status: "blocked", result: "fail", candidate: "blocked_fail"},
		{name: "rework fail", status: "in_progress", result: "fail", candidate: "rework_fail"},
		{name: "completed audit fail", status: "completed", result: "fail", candidate: "completed_audit_fail", audit: true},
		{name: "no progress", status: "todo", result: "fail", candidate: "no_progress"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			task := &Task{Frontmatter: TaskFrontmatter{ID: taskID, Status: test.status, CompletionVerificationMetadata: CompletionVerificationMetadata{
				CompletionID: completionID, LastVerificationID: failID, LastVerificationResult: test.result, LastVerifiedAt: timestamp,
			}}}
			if test.result == "pass" {
				task.Frontmatter.LastVerificationID = passID
				task.Frontmatter.LastVerifiedCompletionID = completionID
			}
			reports := map[string]VerificationArtifact{
				task.Frontmatter.LastVerificationID: {TaskID: taskID, VerificationID: task.Frontmatter.LastVerificationID, Result: test.result, GeneratedAt: timestamp},
			}
			if test.result == "pass" {
				report := reports[passID]
				report.ObservedCompletionID = optionalVerificationID(completionID)
				reports[passID] = report
			}
			if test.audit {
				task.Frontmatter.LastVerificationPreviousID = passID
				report := reports[failID]
				report.PreviousVerificationID = optionalVerificationID(passID)
				reports[failID] = report
				reports[passID] = VerificationArtifact{TaskID: taskID, VerificationID: passID, Result: "pass", GeneratedAt: timestamp, ObservedCompletionID: optionalVerificationID(completionID)}
			}
			state := &State{Frontmatter: StateFrontmatter{LastVerificationResult: test.result + " for " + taskID + " at " + timestamp + " id " + task.Frontmatter.LastVerificationID, LastVerificationID: task.Frontmatter.LastVerificationID, LastVerificationPreviousID: task.Frontmatter.LastVerificationPreviousID}}
			if test.result == "pass" {
				state.Frontmatter.LastVerifiedCompletionID = completionID
			}

			paths := loopTestReportPaths(task.Frontmatter.LastVerificationID)
			if test.audit {
				paths[passID] = loopTestReportPath(passID)
			}
			got := deriveLoopLifecycle(loopLifecycleEvidence{TaskID: taskID, StatusBefore: "todo", Task: task, State: state, Reports: reports, PreflightVerificationIDs: map[string]struct{}{}, VerificationPaths: loopVerificationPathEvidence{Final: paths}, Validation: ValidationResult{Valid: true}})
			if got.LifecycleCandidate == nil || *got.LifecycleCandidate != test.candidate || got.Outcome != test.candidate {
				t.Fatalf("iteration = %+v, want %s", got, test.candidate)
			}
			if test.audit != (got.AuditPassVerificationID != nil) {
				t.Fatalf("audit fields = (%v, %v), audit = %t", got.AuditPassVerificationID, got.AuditPassObservedCompletionID, test.audit)
			}
		})
	}
}

func TestDeriveLoopLifecycleRejectsInvalidEvidenceBeforeChildFailure(t *testing.T) {
	const taskID = "T-001-example"
	passID := "22222222222222222222222222222222"
	completionID := "11111111111111111111111111111111"
	timestamp := "2026-08-23T12:00:00Z"
	task := &Task{Frontmatter: TaskFrontmatter{ID: taskID, Status: "completed", CompletionVerificationMetadata: CompletionVerificationMetadata{
		CompletionID: completionID, LastVerificationID: passID, LastVerificationResult: "pass", LastVerifiedAt: timestamp, LastVerifiedCompletionID: completionID,
	}}}
	state := &State{Frontmatter: StateFrontmatter{LastVerificationResult: "pass for " + taskID + " at " + timestamp + " id " + passID, LastVerificationID: passID, LastVerifiedCompletionID: completionID}}
	reports := map[string]VerificationArtifact{passID: {TaskID: taskID, VerificationID: passID, Result: "pass", GeneratedAt: timestamp, ObservedCompletionID: optionalVerificationID(completionID)}}
	exitCode := 1

	for _, test := range []struct {
		name      string
		mutate    func(*Task, *State, map[string]VerificationArtifact)
		preflight map[string]struct{}
	}{
		{name: "state mismatch", mutate: func(_ *Task, state *State, _ map[string]VerificationArtifact) {
			state.Frontmatter.LastVerificationID = "different"
		}},
		{name: "partial report", mutate: func(_ *Task, _ *State, reports map[string]VerificationArtifact) { delete(reports, passID) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			copyTask := *task
			copyState := *state
			copyReports := map[string]VerificationArtifact{}
			for id, report := range reports {
				copyReports[id] = report
			}
			if test.mutate != nil {
				test.mutate(&copyTask, &copyState, copyReports)
			}
			got := deriveLoopLifecycle(loopLifecycleEvidence{TaskID: taskID, StatusBefore: "todo", Task: &copyTask, State: &copyState, Reports: copyReports, PreflightVerificationIDs: test.preflight, VerificationPaths: loopVerificationPathEvidence{Final: loopTestReportPaths(passID)}, Validation: ValidationResult{Valid: true}, Child: loopChildExecution{ExitCode: &exitCode, WaitError: errors.New("child failed")}})
			if got.LifecycleCandidate != nil || got.Outcome != "invalid_postflight" {
				t.Fatalf("iteration = %+v, want invalid_postflight before child_failed", got)
			}
		})
	}
}

func TestDeriveLoopLifecycleTreatsStalePassAsCompletedUnverified(t *testing.T) {
	const taskID = "T-001-example"
	passID := "22222222222222222222222222222222"
	completionID := "11111111111111111111111111111111"
	timestamp := "2026-08-23T12:00:00Z"
	task := &Task{Frontmatter: TaskFrontmatter{ID: taskID, Status: "completed", CompletionVerificationMetadata: CompletionVerificationMetadata{
		CompletionID: completionID, LastVerificationID: passID, LastVerificationResult: "pass", LastVerifiedAt: timestamp, LastVerifiedCompletionID: completionID,
	}}}
	state := &State{Frontmatter: StateFrontmatter{LastVerificationResult: "pass for " + taskID + " at " + timestamp + " id " + passID, LastVerificationID: passID, LastVerifiedCompletionID: completionID}}
	reports := map[string]VerificationArtifact{passID: {TaskID: taskID, VerificationID: passID, Result: "pass", GeneratedAt: timestamp, ObservedCompletionID: optionalVerificationID(completionID)}}

	got := deriveLoopLifecycle(loopLifecycleEvidence{TaskID: taskID, StatusBefore: "todo", Task: task, State: state, Reports: reports, PreflightVerificationIDs: map[string]struct{}{passID: {}}, Validation: ValidationResult{Valid: true}})
	if got.LifecycleCandidate == nil || *got.LifecycleCandidate != "completed_unverified" || got.Outcome != "completed_unverified" {
		t.Fatalf("iteration = %+v, want completed_unverified", got)
	}
}

func TestDeriveLoopLifecycleClassifiesChildFailureAfterValidCandidate(t *testing.T) {
	const taskID = "T-001-example"
	exitCode := 1
	got := deriveLoopLifecycle(loopLifecycleEvidence{TaskID: taskID, StatusBefore: "todo", Task: loopTestCompletedTask(taskID), State: &State{}, Validation: ValidationResult{Valid: true}, Child: loopChildExecution{ExitCode: &exitCode, WaitError: errors.New("child failed")}})
	if got.LifecycleCandidate == nil || *got.LifecycleCandidate != "completed_unverified" || got.Outcome != "child_failed" {
		t.Fatalf("iteration = %+v, want valid candidate and child_failed", got)
	}
}

func TestDeriveLoopLifecyclePreservesValidCandidateForEveryChildFailure(t *testing.T) {
	const taskID = "T-001-example"
	exitCode := 1
	for _, test := range []struct {
		name  string
		child loopChildExecution
	}{
		{name: "non-zero exit", child: loopChildExecution{ExitCode: &exitCode}},
		{name: "signal", child: loopChildExecution{Signal: "terminated"}},
		{name: "stream copy", child: loopChildExecution{StdoutError: errors.New("copy stdout")}},
		{name: "timeout", child: loopChildExecution{TimedOut: true, CancellationError: errors.New("deadline")}},
		{name: "containment", child: loopChildExecution{ContainmentError: errors.New("survivor")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := deriveLoopLifecycle(loopLifecycleEvidence{TaskID: taskID, StatusBefore: "todo", Task: loopTestCompletedTask(taskID), State: &State{}, Validation: ValidationResult{Valid: true}, Child: test.child})
			if got.LifecycleCandidate == nil || *got.LifecycleCandidate != "completed_unverified" || got.Outcome != "child_failed" {
				t.Fatalf("iteration = %+v, want completed_unverified and child_failed", got)
			}
		})
	}
}

func TestDeriveLoopLifecycleRejectsIncompleteCompletedPass(t *testing.T) {
	const taskID = "T-001-example"
	passID := "22222222222222222222222222222222"
	timestamp := "2026-08-23T12:00:00Z"
	task := &Task{Frontmatter: TaskFrontmatter{ID: taskID, Status: "completed", CompletionVerificationMetadata: CompletionVerificationMetadata{
		LastVerificationID: passID, LastVerificationResult: "pass", LastVerifiedAt: timestamp,
	}}}
	state := &State{Frontmatter: StateFrontmatter{LastVerificationResult: "pass for " + taskID + " at " + timestamp + " id " + passID, LastVerificationID: passID}}
	reports := map[string]VerificationArtifact{passID: {TaskID: taskID, VerificationID: passID, Result: "pass", GeneratedAt: timestamp}}

	got := deriveLoopLifecycle(loopLifecycleEvidence{TaskID: taskID, StatusBefore: "todo", Task: task, State: state, Reports: reports, VerificationPaths: loopVerificationPathEvidence{Final: loopTestReportPaths(passID)}, Validation: ValidationResult{Valid: true}})
	if got.LifecycleCandidate != nil || got.Outcome != "invalid_postflight" {
		t.Fatalf("iteration = %+v, want invalid partial completion", got)
	}
}

func TestDeriveLoopLifecycleRejectsInvalidFinalValidation(t *testing.T) {
	got := deriveLoopLifecycle(loopLifecycleEvidence{TaskID: "T-001-example", Task: &Task{}, State: &State{}, Validation: ValidationResult{Valid: false}})
	if got.LifecycleCandidate != nil || got.Outcome != "invalid_postflight" {
		t.Fatalf("iteration = %+v, want invalid final state", got)
	}
}

func TestDeriveLoopLifecycleProjectsPostflightPromptWithoutContent(t *testing.T) {
	iteration := deriveLoopLifecycle(loopLifecycleEvidence{TaskID: "T-001-example", Task: &Task{Frontmatter: TaskFrontmatter{ID: "T-001-example", Status: "completed"}}, State: &State{}, Validation: ValidationResult{Valid: true}, Prompt: LoopPromptExecution{ID: "implementation", Source: "builtin", Content: "secret prompt bytes"}})
	encoded, err := json.Marshal(iteration)
	if err != nil {
		t.Fatalf("marshal iteration: %v", err)
	}
	if strings.Contains(string(encoded), "content") || strings.Contains(string(encoded), "secret prompt bytes") {
		t.Fatalf("postflight iteration leaked prompt content: %s", encoded)
	}
}

func TestDeriveLoopLifecycleRejectsAuditFailWithoutCompletionBinding(t *testing.T) {
	const taskID = "T-001-example"
	passID := "22222222222222222222222222222222"
	failID := "33333333333333333333333333333333"
	timestamp := "2026-08-23T12:00:00Z"
	task := &Task{Frontmatter: TaskFrontmatter{ID: taskID, Status: "completed", CompletionVerificationMetadata: CompletionVerificationMetadata{
		LastVerificationID: failID, LastVerificationPreviousID: passID, LastVerificationResult: "fail", LastVerifiedAt: timestamp,
	}}}
	state := &State{Frontmatter: StateFrontmatter{LastVerificationResult: "fail for " + taskID + " at " + timestamp + " id " + failID, LastVerificationID: failID, LastVerificationPreviousID: passID}}
	reports := map[string]VerificationArtifact{
		failID: {TaskID: taskID, VerificationID: failID, Result: "fail", GeneratedAt: timestamp, PreviousVerificationID: optionalVerificationID(passID)},
		passID: {TaskID: taskID, VerificationID: passID, Result: "pass", GeneratedAt: timestamp},
	}

	got := deriveLoopLifecycle(loopLifecycleEvidence{TaskID: taskID, StatusBefore: "todo", Task: task, State: state, Reports: reports, VerificationPaths: loopVerificationPathEvidence{Final: loopTestReportPaths(failID, passID)}, Validation: ValidationResult{Valid: true}})
	if got.LifecycleCandidate != nil || got.Outcome != "invalid_postflight" {
		t.Fatalf("iteration = %+v, want invalid unbound audit fail", got)
	}
}

func TestDeriveLoopLifecycleUsesSignalInsteadOfExitCode(t *testing.T) {
	const taskID = "T-001-example"
	exitCode := -1
	got := deriveLoopLifecycle(loopLifecycleEvidence{TaskID: taskID, StatusBefore: "todo", Task: loopTestCompletedTask(taskID), State: &State{}, Validation: ValidationResult{Valid: true}, Child: loopChildExecution{ExitCode: &exitCode, Signal: "terminated"}})
	if got.Child.ExitCode != nil || got.Child.Signal == nil || *got.Child.Signal != "terminated" {
		t.Fatalf("child = %+v, want signal-only evidence", got.Child)
	}
}

func TestDeriveLoopLifecycleRejectsPreviouslyCompletedTask(t *testing.T) {
	got := deriveLoopLifecycle(loopLifecycleEvidence{TaskID: "T-001-example", StatusBefore: "completed", CompletionIDBefore: "11111111111111111111111111111111", Task: &Task{Frontmatter: TaskFrontmatter{ID: "T-001-example", Status: "completed", CompletionVerificationMetadata: CompletionVerificationMetadata{CompletionID: "11111111111111111111111111111111"}}}, State: &State{}, Validation: ValidationResult{Valid: true}})
	if got.LifecycleCandidate != nil || got.Outcome != "invalid_postflight" {
		t.Fatalf("iteration = %+v, want invalid pre-existing completion", got)
	}
}

func TestDeriveLoopLifecycleRejectsReusedOrMalformedVerificationPath(t *testing.T) {
	const taskID = "T-001-example"
	passID := "22222222222222222222222222222222"
	completionID := "11111111111111111111111111111111"
	timestamp := "2026-08-23T12:00:00Z"
	task := &Task{Frontmatter: TaskFrontmatter{ID: taskID, Status: "completed", CompletionVerificationMetadata: CompletionVerificationMetadata{
		CompletionID: completionID, LastVerificationID: passID, LastVerificationResult: "pass", LastVerifiedAt: timestamp, LastVerifiedCompletionID: completionID,
	}}}
	state := &State{Frontmatter: StateFrontmatter{LastVerificationResult: "pass for " + taskID + " at " + timestamp + " id " + passID, LastVerificationID: passID, LastVerifiedCompletionID: completionID}}
	reports := map[string]VerificationArtifact{passID: {TaskID: taskID, VerificationID: passID, Result: "pass", GeneratedAt: timestamp, ObservedCompletionID: optionalVerificationID(completionID)}}
	path := loopTestReportPath(passID)
	for _, test := range []struct {
		name      string
		preflight map[string]struct{}
		final     string
	}{
		{name: "reused path", preflight: map[string]struct{}{path: {}}, final: path},
		{name: "malformed path", final: "planning/artifacts/verify/report.json"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := deriveLoopLifecycle(loopLifecycleEvidence{TaskID: taskID, StatusBefore: "todo", Task: task, State: state, Reports: reports, VerificationPaths: loopVerificationPathEvidence{Preflight: test.preflight, Final: map[string]string{passID: test.final}}, Validation: ValidationResult{Valid: true}})
			if got.LifecycleCandidate != nil || got.Outcome != "invalid_postflight" {
				t.Fatalf("iteration = %+v, want invalid report path", got)
			}
		})
	}
}

func loopTestCompletedTask(taskID string) *Task {
	return &Task{Frontmatter: TaskFrontmatter{ID: taskID, Status: "completed", CompletionVerificationMetadata: CompletionVerificationMetadata{CompletionID: "11111111111111111111111111111111"}}}
}

func loopTestReportPaths(ids ...string) map[string]string {
	paths := make(map[string]string, len(ids))
	for _, id := range ids {
		paths[id] = loopTestReportPath(id)
	}
	return paths
}

func loopTestReportPath(id string) string {
	return "planning/artifacts/verify/T-001-example/20260823T120000Z-" + id + "/report.json"
}
