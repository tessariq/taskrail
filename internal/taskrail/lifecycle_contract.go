package taskrail

import "fmt"

// LifecycleBranch names every run-ending branch recognized by the v0.5 contract.
type LifecycleBranch string

const (
	BranchCompletedPass LifecycleBranch = "completed-pass"
	BranchBlockedFail   LifecycleBranch = "blocked-fail"
	BranchReworkFail    LifecycleBranch = "rework-fail"
)

// LifecycleStep is one semantic operation in a canonical branch.
type LifecycleStep string

const (
	StepValidate   LifecycleStep = "validate"
	StepStart      LifecycleStep = "start"
	StepImplement  LifecycleStep = "implement"
	StepChecks     LifecycleStep = "checks"
	StepComplete   LifecycleStep = "complete"
	StepBlock      LifecycleStep = "block"
	StepVerifyPass LifecycleStep = "verify-pass"
	StepVerifyFail LifecycleStep = "verify-fail"
	StepStop       LifecycleStep = "stop"
)

var canonicalLifecycleBranches = map[LifecycleBranch][]LifecycleStep{
	BranchCompletedPass: {StepValidate, StepStart, StepImplement, StepChecks, StepComplete, StepVerifyPass},
	BranchBlockedFail:   {StepValidate, StepStart, StepBlock, StepVerifyFail, StepStop},
	BranchReworkFail:    {StepValidate, StepStart, StepVerifyFail, StepStop},
}

func CanonicalLifecycleSteps(branch LifecycleBranch) []LifecycleStep {
	return append([]LifecycleStep(nil), canonicalLifecycleBranches[branch]...)
}

// ValidateLifecycleBranch rejects invented branches and any ordering that gives
// verify lifecycle semantics or completes failing work.
func ValidateLifecycleBranch(branch LifecycleBranch, steps []LifecycleStep) error {
	want, ok := canonicalLifecycleBranches[branch]
	if !ok {
		return fmt.Errorf("unknown lifecycle branch %q", branch)
	}
	if len(steps) != len(want) {
		return fmt.Errorf("lifecycle branch %q has %d steps, want %d", branch, len(steps), len(want))
	}
	for i := range want {
		if steps[i] != want[i] {
			return fmt.Errorf("lifecycle branch %q step %d is %q, want %q", branch, i+1, steps[i], want[i])
		}
	}
	return nil
}

// TerminatesAutonomousRun means the current run stops after the branch. It does
// not make blocked work immutable; unblock remains a separate later operation.
func TerminatesAutonomousRun(branch LifecycleBranch) bool {
	_, ok := canonicalLifecycleBranches[branch]
	return ok
}

type LifecycleActor string

const (
	ActorDirectOperator LifecycleActor = "direct-operator"
	ActorDelegatedChild LifecycleActor = "delegated-child"
)

type LifecycleCapability string

const (
	CapabilityTaskNew              LifecycleCapability = "task-new"
	CapabilityVerifyCreateFollowup LifecycleCapability = "verify-create-followup"
	CapabilityTaskRelease          LifecycleCapability = "task-release"
)

func LifecycleCapabilityAllowed(actor LifecycleActor, capability LifecycleCapability) bool {
	if actor == ActorDirectOperator {
		return capability == CapabilityTaskNew || capability == CapabilityVerifyCreateFollowup || capability == CapabilityTaskRelease
	}
	return actor == ActorDelegatedChild && capability == CapabilityVerifyCreateFollowup
}

// CompletionVerificationMetadata is the lifecycle subset shared by persisted
// task frontmatter, validation, and lifecycle writers.
type CompletionVerificationMetadata struct {
	CompletionID               string `yaml:"completion_id,omitempty" json:"completion_id,omitempty"`
	LastVerificationID         string `yaml:"last_verification_id,omitempty" json:"last_verification_id,omitempty"`
	LastVerificationPreviousID string `yaml:"last_verification_previous_id,omitempty" json:"last_verification_previous_id,omitempty"`
	LastVerificationResult     string `yaml:"last_verification_result,omitempty" json:"last_verification_result,omitempty"`
	LastVerifiedAt             string `yaml:"last_verified_at,omitempty" json:"last_verified_at,omitempty"`
	LastVerifiedCompletionID   string `yaml:"last_verified_completion_id,omitempty" json:"last_verified_completion_id,omitempty"`
}

func ValidateCompletionVerificationMetadata(status string, meta CompletionVerificationMetadata, chain ...VerificationChainLink) []string {
	if meta == (CompletionVerificationMetadata{}) {
		return nil // legacy pre-v0.5 task, adopted by a later lifecycle writer
	}

	var violations []string
	hasTuple := meta.LastVerificationID != "" || meta.LastVerificationPreviousID != "" || meta.LastVerificationResult != "" || meta.LastVerifiedAt != ""
	completeTuple := meta.LastVerificationID != "" && meta.LastVerificationResult != "" && meta.LastVerifiedAt != ""
	if hasTuple && !completeTuple {
		violations = append(violations, "verification metadata must contain id, result, and verified_at")
	}
	if meta.LastVerificationResult != "" && meta.LastVerificationResult != "pass" && meta.LastVerificationResult != "fail" {
		violations = append(violations, "verification result must be pass or fail")
	}
	if meta.LastVerificationPreviousID != "" {
		if err := ValidateVerificationChain(chain); err != nil || len(chain) == 0 || chain[len(chain)-1].ID != meta.LastVerificationID || chain[len(chain)-1].PreviousID != meta.LastVerificationPreviousID {
			violations = append(violations, "verification predecessor must identify the exact prior verification evidence")
		}
	}
	if status != "completed" && meta.CompletionID != "" {
		violations = append(violations, "non-completed task must not have a completion id")
	}
	if meta.LastVerifiedCompletionID != "" {
		if status != "completed" || meta.LastVerificationResult != "pass" {
			violations = append(violations, "only a completed passing verification may bind a completion id")
		}
		if meta.CompletionID == "" || meta.LastVerifiedCompletionID != meta.CompletionID {
			violations = append(violations, "verified completion id must equal the current completion id")
		}
	}
	if status == "completed" && meta.LastVerificationResult == "pass" && meta.CompletionID == "" {
		violations = append(violations, "completed passing verification must have a current completion id")
	}
	return violations
}

type VerificationChainLink struct {
	ID         string
	PreviousID string
}

func ValidateVerificationChain(chain []VerificationChainLink) error {
	seen := make(map[string]struct{}, len(chain))
	for i, link := range chain {
		if link.ID == "" {
			return fmt.Errorf("verification chain link %d has an empty id", i+1)
		}
		if _, exists := seen[link.ID]; exists {
			return fmt.Errorf("verification chain repeats id %q", link.ID)
		}
		seen[link.ID] = struct{}{}
		if i == 0 && link.PreviousID != "" {
			return fmt.Errorf("first verification %q must not name a predecessor", link.ID)
		}
		if i > 0 && link.PreviousID != chain[i-1].ID {
			return fmt.Errorf("verification %q predecessor is %q, want %q", link.ID, link.PreviousID, chain[i-1].ID)
		}
	}
	return nil
}
