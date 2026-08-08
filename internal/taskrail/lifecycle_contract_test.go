package taskrail

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCanonicalLifecycleBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		branch LifecycleBranch
		steps  []LifecycleStep
	}{
		{BranchCompletedPass, []LifecycleStep{StepValidate, StepStart, StepImplement, StepChecks, StepComplete, StepVerifyPass}},
		{BranchBlockedFail, []LifecycleStep{StepValidate, StepStart, StepBlock, StepVerifyFail, StepStop}},
		{BranchReworkFail, []LifecycleStep{StepValidate, StepStart, StepVerifyFail, StepStop}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.branch), func(t *testing.T) {
			t.Parallel()
			if got := CanonicalLifecycleSteps(tt.branch); !reflect.DeepEqual(got, tt.steps) {
				t.Fatalf("steps = %v, want %v", got, tt.steps)
			}
			if err := ValidateLifecycleBranch(tt.branch, tt.steps); err != nil {
				t.Fatalf("canonical branch rejected: %v", err)
			}
			if !TerminatesAutonomousRun(tt.branch) {
				t.Fatal("canonical branch must terminate the current run")
			}
		})
	}
}

func TestLifecycleContractRejectsInventedOrReorderedBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		branch LifecycleBranch
		steps  []LifecycleStep
	}{
		{"complete on failure", BranchBlockedFail, []LifecycleStep{StepValidate, StepStart, StepComplete, StepVerifyFail, StepStop}},
		{"verify as transition", BranchCompletedPass, []LifecycleStep{StepValidate, StepStart, StepImplement, StepChecks, StepVerifyPass, StepComplete}},
		{"unknown branch", LifecycleBranch("paused-fail"), []LifecycleStep{StepValidate, StepStart, StepStop}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := ValidateLifecycleBranch(tt.branch, tt.steps); err == nil {
				t.Fatal("expected lifecycle contract refusal")
			}
		})
	}
}

func TestExactTaskIdentity(t *testing.T) {
	t.Parallel()

	tasks := []*Task{{Frontmatter: TaskFrontmatter{ID: "T-229-canonicalize-lifecycle"}}}
	if task, err := exactTaskByID(tasks, "T-229-canonicalize-lifecycle"); err != nil || task.Frontmatter.ID != "T-229-canonicalize-lifecycle" {
		t.Fatalf("exact id rejected: task=%v err=%v", task, err)
	}
	for _, id := range []string{"T-229", "229", "canonicalize-lifecycle", " T-229-canonicalize-lifecycle "} {
		if _, err := exactTaskByID(tasks, id); err == nil || !strings.Contains(err.Error(), "exact full persisted task ID") {
			t.Errorf("identity %q error = %v, want stable exact-id refusal", id, err)
		}
	}

	bare := []*Task{{Frontmatter: TaskFrontmatter{ID: "T-230"}}}
	if _, err := exactTaskByID(bare, "T-230"); err != nil {
		t.Fatalf("persisted bare id rejected: %v", err)
	}
}

func TestWritersRejectNormalizedTaskIdentities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*Service) error
	}{
		{"start", func(svc *Service) error { _, err := svc.Start(" T-001-slugged "); return err }},
		{"verify", func(svc *Service) error {
			_, err := svc.Verify(VerifyInput{TaskID: " T-001-slugged ", Result: "fail", Summary: "x"})
			return err
		}},
		{"rename", func(svc *Service) error {
			_, err := svc.RenameTask(RenameTaskInput{OldID: " T-001-slugged ", Slug: "renamed", SlugExplicit: true})
			return err
		}},
		{"repoint", func(svc *Service) error {
			_, err := svc.RepointTask(RepointTaskInput{TaskID: " T-001-slugged ", Area: "summary"})
			return err
		}},
		{"follow-up", func(svc *Service) error {
			_, err := svc.CreateTask(CreateTaskInput{Title: "Child", FollowUpOf: " T-001-slugged "})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := seedFixtureRepo(t)
			writeTask(t, repo, "T-001-slugged", "Slugged", "todo", "medium", "specs/v0.1.0.md#summary", nil)
			svc := newTestService(t, repo, time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC))
			if err := tt.run(svc); err == nil || !strings.Contains(err.Error(), "exact full persisted task ID") {
				t.Fatalf("error = %v, want exact-id refusal", err)
			}
		})
	}
}

func TestLifecycleCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		actor      LifecycleActor
		capability LifecycleCapability
		want       bool
	}{
		{ActorDirectOperator, CapabilityTaskNew, true},
		{ActorDirectOperator, CapabilityVerifyCreateFollowup, true},
		{ActorDirectOperator, CapabilityTaskRelease, true},
		{ActorDelegatedChild, CapabilityTaskNew, false},
		{ActorDelegatedChild, CapabilityVerifyCreateFollowup, true},
		{ActorDelegatedChild, CapabilityTaskRelease, false},
	}
	for _, tt := range tests {
		if got := LifecycleCapabilityAllowed(tt.actor, tt.capability); got != tt.want {
			t.Errorf("allowed(%q, %q) = %v, want %v", tt.actor, tt.capability, got, tt.want)
		}
	}
}

func TestValidateCompletionVerificationMetadata(t *testing.T) {
	t.Parallel()

	verification := func(result string) CompletionVerificationMetadata {
		return CompletionVerificationMetadata{
			LastVerificationID:         "verification-2",
			LastVerificationPreviousID: "verification-1",
			LastVerificationResult:     result,
			LastVerifiedAt:             "2026-08-08T00:00:00Z",
		}
	}
	tests := []struct {
		name   string
		status string
		meta   CompletionVerificationMetadata
		valid  bool
	}{
		{"legacy todo", "todo", CompletionVerificationMetadata{}, true},
		{"legacy completed", "completed", CompletionVerificationMetadata{}, true},
		{"non-completed fail", "in_progress", verification("fail"), true},
		{"newly completed", "completed", CompletionVerificationMetadata{CompletionID: "completion-1"}, true},
		{"completed fail", "completed", func() CompletionVerificationMetadata {
			m := verification("fail")
			m.CompletionID = "completion-1"
			return m
		}(), true},
		{"completed bound pass", "completed", func() CompletionVerificationMetadata {
			m := verification("pass")
			m.CompletionID = "completion-1"
			m.LastVerifiedCompletionID = "completion-1"
			return m
		}(), true},
		{"partial tuple", "in_progress", CompletionVerificationMetadata{LastVerificationID: "verification-1"}, false},
		{"non-completed completion", "blocked", CompletionVerificationMetadata{CompletionID: "completion-1"}, false},
		{"fail binding", "completed", func() CompletionVerificationMetadata {
			m := verification("fail")
			m.CompletionID = "completion-1"
			m.LastVerifiedCompletionID = "completion-1"
			return m
		}(), false},
		{"mismatched binding", "completed", func() CompletionVerificationMetadata {
			m := verification("pass")
			m.CompletionID = "completion-1"
			m.LastVerifiedCompletionID = "completion-2"
			return m
		}(), false},
		{"unbound pass", "completed", func() CompletionVerificationMetadata {
			m := verification("pass")
			m.CompletionID = "completion-1"
			return m
		}(), true},
		{"pass without completion", "completed", verification("pass"), false},
		{"missing predecessor evidence", "in_progress", verification("fail"), false},
		{"self predecessor", "in_progress", func() CompletionVerificationMetadata {
			m := verification("fail")
			m.LastVerificationPreviousID = m.LastVerificationID
			return m
		}(), false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var chain []VerificationChainLink
			if tt.name != "missing predecessor evidence" && tt.meta.LastVerificationPreviousID == "verification-1" {
				chain = []VerificationChainLink{{ID: "verification-1"}, {ID: "verification-2", PreviousID: "verification-1"}}
			}
			violations := ValidateCompletionVerificationMetadata(tt.status, tt.meta, chain...)
			if got := len(violations) == 0; got != tt.valid {
				t.Fatalf("valid = %v, want %v; violations: %v", got, tt.valid, violations)
			}
		})
	}
}

func TestValidateVerificationChain(t *testing.T) {
	t.Parallel()

	valid := []VerificationChainLink{{ID: "v1"}, {ID: "v2", PreviousID: "v1"}, {ID: "v3", PreviousID: "v2"}}
	if err := ValidateVerificationChain(valid); err != nil {
		t.Fatalf("valid chain rejected: %v", err)
	}
	for _, chain := range [][]VerificationChainLink{
		{{ID: "v1", PreviousID: "missing"}},
		{{ID: "v1"}, {ID: "v2", PreviousID: "other"}},
		{{ID: "v1"}, {ID: "v1", PreviousID: "v1"}},
	} {
		if err := ValidateVerificationChain(chain); err == nil {
			t.Errorf("broken chain accepted: %+v", chain)
		}
	}
}

func TestRepositoryValidationConsumesLifecycleMetadataContract(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "One", "completed", "medium", "specs/v0.1.0.md#summary", nil)
	svc := newTestService(t, repo, time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC))
	tasks, err := svc.loadTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	task, err := exactTaskByID(tasks, "T-001")
	if err != nil {
		t.Fatal(err)
	}
	task.Frontmatter.LastVerificationID = "verification-1"
	task.Frontmatter.LastVerificationResult = "pass"
	task.Frontmatter.LastVerifiedAt = "2026-08-08T00:00:00Z"
	if err := svc.saveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	result, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.Valid || !hasViolation(result.Violations, "completed passing verification must have") {
		t.Fatalf("validation did not apply lifecycle metadata contract: %v", result.Violations)
	}
}
