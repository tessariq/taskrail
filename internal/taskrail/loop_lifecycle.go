package taskrail

import (
	"fmt"
	"strings"
)

// LoopIteration is the per-child postflight record consumed by the terminal
// loop diagnostic. It retains a valid lifecycle candidate when child execution
// fails so operators can distinguish persisted work from the process failure.
type LoopIteration struct {
	TaskID                        string              `json:"task_id"`
	Outcome                       string              `json:"outcome"`
	LifecycleCandidate            *string             `json:"lifecycle_candidate"`
	Child                         LoopIterationChild  `json:"child"`
	StatusBefore                  *string             `json:"status_before"`
	StatusAfter                   *string             `json:"status_after"`
	CompletionIDBefore            *string             `json:"completion_id_before"`
	CompletionIDAfter             *string             `json:"completion_id_after"`
	VerificationIDBefore          *string             `json:"verification_id_before"`
	VerificationIDAfter           *string             `json:"verification_id_after"`
	VerificationPreviousIDBefore  *string             `json:"verification_previous_id_before"`
	VerificationPreviousIDAfter   *string             `json:"verification_previous_id_after"`
	ObservedCompletionID          *string             `json:"observed_completion_id"`
	AuditPassVerificationID       *string             `json:"audit_pass_verification_id"`
	AuditPassObservedCompletionID *string             `json:"audit_pass_observed_completion_id"`
	Policy                        TaskLoopRow         `json:"policy"`
	Prompt                        LoopIterationPrompt `json:"prompt"`
}

type LoopIterationChild struct {
	ExitCode *int    `json:"exit_code"`
	Signal   *string `json:"signal"`
	TimedOut bool    `json:"timed_out"`
}

// LoopIterationPrompt intentionally omits rendered prompt content. Postflight
// diagnostics identify frozen prompt bytes by digest without republishing them.
type LoopIterationPrompt struct {
	ID                 string  `json:"id"`
	Source             string  `json:"source"`
	Path               *string `json:"path"`
	TemplateSHA256     string  `json:"template_sha256"`
	RenderedSHA256     string  `json:"rendered_sha256"`
	OverrideAuthorized bool    `json:"override_authorized"`
}

// loopVerificationPathEvidence binds final report paths to the preflight
// artifact set without making this lifecycle classifier own T-312's wider scan.
type loopVerificationPathEvidence struct {
	Preflight map[string]struct{}
	Final     map[string]string
}

// loopLifecycleEvidence is the lifecycle-only subset of postflight. T-312 and
// T-313 add mutation and delivery evidence before an iteration is accepted.
type loopLifecycleEvidence struct {
	TaskID                       string
	StatusBefore                 string
	CompletionIDBefore           string
	VerificationIDBefore         string
	VerificationPreviousIDBefore string
	Task                         *Task
	State                        *State
	Reports                      map[string]VerificationArtifact
	PreflightVerificationIDs     map[string]struct{}
	VerificationPaths            loopVerificationPathEvidence
	Validation                   ValidationResult
	Child                        loopChildExecution
	Policy                       TaskLoopRow
	Prompt                       LoopPromptExecution
}

func deriveLoopLifecycle(evidence loopLifecycleEvidence) LoopIteration {
	iteration := LoopIteration{
		TaskID:                       evidence.TaskID,
		Child:                        loopIterationChild(evidence.Child),
		StatusBefore:                 loopOptionalString(evidence.StatusBefore),
		CompletionIDBefore:           loopOptionalString(evidence.CompletionIDBefore),
		VerificationIDBefore:         loopOptionalString(evidence.VerificationIDBefore),
		VerificationPreviousIDBefore: loopOptionalString(evidence.VerificationPreviousIDBefore),
		Policy:                       evidence.Policy,
		Prompt:                       loopIterationPrompt(evidence.Prompt),
	}
	if evidence.Task != nil {
		meta := evidence.Task.Frontmatter.CompletionVerificationMetadata
		iteration.StatusAfter = loopOptionalString(evidence.Task.Frontmatter.Status)
		iteration.CompletionIDAfter = loopOptionalString(meta.CompletionID)
		iteration.VerificationIDAfter = loopOptionalString(meta.LastVerificationID)
		iteration.VerificationPreviousIDAfter = loopOptionalString(meta.LastVerificationPreviousID)
	}
	if !evidence.Validation.Valid || evidence.Task == nil || evidence.State == nil {
		iteration.Outcome = "invalid_postflight"
		return iteration
	}

	candidate, valid, observed, auditID, auditObserved := loopLifecycleCandidate(evidence)
	if !valid {
		iteration.Outcome = "invalid_postflight"
		return iteration
	}
	iteration.LifecycleCandidate = &candidate
	iteration.ObservedCompletionID = observed
	iteration.AuditPassVerificationID = auditID
	iteration.AuditPassObservedCompletionID = auditObserved
	if loopChildFailed(evidence.Child) {
		iteration.Outcome = "child_failed"
		return iteration
	}
	iteration.Outcome = candidate
	return iteration
}

func loopLifecycleCandidate(evidence loopLifecycleEvidence) (string, bool, *string, *string, *string) {
	task := evidence.Task
	meta := task.Frontmatter.CompletionVerificationMetadata
	fresh := meta.LastVerificationID != "" && !loopVerificationKnown(evidence.PreflightVerificationIDs, meta.LastVerificationID)
	if fresh && !validFreshLoopVerification(evidence) {
		return "", false, nil, nil, nil
	}

	switch task.Frontmatter.Status {
	case "completed":
		if evidence.StatusBefore != "todo" || meta.CompletionID == "" || meta.CompletionID == evidence.CompletionIDBefore {
			return "", false, nil, nil, nil
		}
		if !fresh {
			return "completed_unverified", true, nil, nil, nil
		}
		if meta.LastVerificationResult == "pass" {
			if meta.LastVerifiedCompletionID != meta.CompletionID {
				return "", false, nil, nil, nil
			}
			return "completed_pass", true, optionalVerificationID(meta.LastVerifiedCompletionID), nil, nil
		}
		if meta.LastVerificationResult != "fail" {
			return "", false, nil, nil, nil
		}
		priorID := meta.LastVerificationPreviousID
		prior, ok := evidence.Reports[priorID]
		if priorID == "" || loopVerificationKnown(evidence.PreflightVerificationIDs, priorID) || !validFreshLoopVerificationPath(evidence, priorID) || !ok || prior.TaskID != task.Frontmatter.ID || prior.VerificationID != priorID || prior.Result != "pass" || prior.ObservedCompletionID == nil || *prior.ObservedCompletionID != meta.CompletionID {
			return "", false, nil, nil, nil
		}
		return "completed_audit_fail", true, nil, optionalVerificationID(priorID), optionalVerificationID(meta.CompletionID)
	case "blocked":
		if fresh && meta.LastVerificationResult == "fail" {
			return "blocked_fail", true, nil, nil, nil
		}
	case "in_progress":
		if fresh && meta.LastVerificationResult == "fail" {
			return "rework_fail", true, nil, nil, nil
		}
	}
	return "no_progress", true, nil, nil, nil
}

func validFreshLoopVerification(evidence loopLifecycleEvidence) bool {
	task := evidence.Task
	meta := task.Frontmatter.CompletionVerificationMetadata
	report, ok := evidence.Reports[meta.LastVerificationID]
	if !ok || report.TaskID != task.Frontmatter.ID || report.VerificationID != meta.LastVerificationID || report.Result != meta.LastVerificationResult || report.GeneratedAt != meta.LastVerifiedAt || !sameVerificationID(report.PreviousVerificationID, optionalVerificationID(meta.LastVerificationPreviousID)) {
		return false
	}
	if !validFreshLoopVerificationPath(evidence, meta.LastVerificationID) {
		return false
	}
	observed := ""
	if meta.LastVerificationResult == "pass" && task.Frontmatter.Status == "completed" {
		observed = meta.CompletionID
	}
	if meta.LastVerifiedCompletionID != observed || !sameVerificationID(report.ObservedCompletionID, optionalVerificationID(observed)) {
		return false
	}
	state := evidence.State.Frontmatter
	return state.LastVerificationID == meta.LastVerificationID &&
		state.LastVerificationPreviousID == meta.LastVerificationPreviousID &&
		state.LastVerifiedCompletionID == meta.LastVerifiedCompletionID &&
		state.LastVerificationResult == fmt.Sprintf("%s for %s at %s id %s", meta.LastVerificationResult, task.Frontmatter.ID, meta.LastVerifiedAt, meta.LastVerificationID)
}

func validFreshLoopVerificationPath(evidence loopLifecycleEvidence, verificationID string) bool {
	path, ok := evidence.VerificationPaths.Final[verificationID]
	return ok && !loopVerificationKnown(evidence.VerificationPaths.Preflight, path) &&
		strings.HasSuffix(loopPathSlash(path), "-"+verificationID+"/report.json")
}

func loopIterationChild(execution loopChildExecution) LoopIterationChild {
	child := LoopIterationChild{TimedOut: execution.TimedOut}
	if execution.Signal != "" {
		child.Signal = loopOptionalString(execution.Signal)
		return child
	}
	child.ExitCode = execution.ExitCode
	return child
}

func loopIterationPrompt(prompt LoopPromptExecution) LoopIterationPrompt {
	return LoopIterationPrompt{
		ID: prompt.ID, Source: prompt.Source, Path: prompt.Path,
		TemplateSHA256: prompt.TemplateSHA256, RenderedSHA256: prompt.RenderedSHA256,
		OverrideAuthorized: prompt.OverrideAuthorized,
	}
}

func loopPathSlash(path string) string { return strings.ReplaceAll(path, "\\", "/") }

func loopChildFailed(execution loopChildExecution) bool {
	return execution.Failed() || execution.Signal != "" || execution.ExitCode != nil && *execution.ExitCode != 0
}

func loopVerificationKnown(ids map[string]struct{}, id string) bool {
	_, known := ids[id]
	return known
}

func loopOptionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
