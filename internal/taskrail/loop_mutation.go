package taskrail

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/tessariq/taskrail/internal/repotx"
)

type LoopPolicyOperation string

const (
	LoopPolicyAllow LoopPolicyOperation = "allow"
	LoopPolicyHold  LoopPolicyOperation = "hold"
	LoopPolicyClear LoopPolicyOperation = "clear"
)

type LoopPolicyMutationInput struct {
	TaskID    string
	Operation LoopPolicyOperation
	Reason    string
	DryRun    bool
}

type LoopPolicyMutationPolicy struct {
	Source          string  `json:"source"`
	EffectivePolicy string  `json:"effective_policy"`
	Reason          string  `json:"reason"`
	PersistedPolicy *string `json:"persisted_policy"`
	PersistedReason *string `json:"persisted_reason"`
}

type LoopPolicyMutationResult struct {
	TaskID     string                   `json:"task_id"`
	Operation  LoopPolicyOperation      `json:"operation"`
	Applied    bool                     `json:"applied"`
	Prior      LoopPolicyMutationPolicy `json:"prior"`
	Candidate  LoopPolicyMutationPolicy `json:"candidate"`
	Validation ValidationResult         `json:"validation"`
}

// testHookLoopPolicyCandidatePrepared exposes the stale-preimage boundary to
// transaction tests. It is nil outside tests.
var testHookLoopPolicyCandidatePrepared func()

// MutateTaskLoopPolicy changes only a selected open task's policy pair. The
// task and its STATE.md projection publish through the normal task transaction.
func (s *Service) MutateTaskLoopPolicy(input LoopPolicyMutationInput) (result LoopPolicyMutationResult, err error) {
	if input.TaskID == "" {
		return result, invalidArgumentsf("task id is required")
	}
	if err := validateLoopPolicyMutationInput(input); err != nil {
		return result, err
	}

	writer := loopPolicyTaskWriter(input.Operation)
	own, release, err := s.beginTaskWriterWrite(writer)
	if err != nil {
		return result, err
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil && err == nil {
			err = releaseErr
		}
	}()

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return result, err
	}
	corpus, err := snapshotTaskCorpus(tasks)
	if err != nil {
		return result, err
	}
	baseline := s.validateInMemory(state, tasks)
	target, err := exactTaskByID(tasks, input.TaskID)
	if err != nil {
		return result, err
	}
	if target.Frontmatter.Status != "todo" && target.Frontmatter.Status != "blocked" {
		return result, WithMachineErrorCode(MachineCodeInvalidStatus,
			fmt.Errorf("task %s is %s: loop policy mutation requires todo or blocked work", input.TaskID, target.Frontmatter.Status))
	}

	before, err := os.ReadFile(target.Filename)
	if err != nil {
		return result, fmt.Errorf("read task %s: %w", target.Path, fsCause(err))
	}
	candidate := *target
	setCandidateLoopPolicy(&candidate.Frontmatter, input)
	candidate.Frontmatter.UpdatedAt = timestamp(s.now())
	preview := replaceLoopPolicyTask(tasks, target, &candidate)
	stateCandidate := *state
	stateCandidate.Frontmatter.UpdatedAt = candidate.Frontmatter.UpdatedAt
	stateCandidate.Body = renderStateBody(stateCandidate.Frontmatter, preview)
	validation := s.validateInMemory(&stateCandidate, preview)
	after, err := replaceLoopPolicyFields(before, target.Frontmatter.ID, input, candidate.Frontmatter.UpdatedAt)
	if err != nil {
		return result, WithMachineErrorCode(MachineCodeRepositoryInvalid, err)
	}
	if testHookLoopPolicyCandidatePrepared != nil {
		testHookLoopPolicyCandidatePrepared()
	}
	result = LoopPolicyMutationResult{
		TaskID: input.TaskID, Operation: input.Operation,
		Prior:      loopPolicyMutationPolicy(target.Frontmatter.LoopPolicyMetadata),
		Candidate:  loopPolicyMutationPolicy(candidate.Frontmatter.LoopPolicyMetadata),
		Validation: validation,
	}
	if !validation.Valid {
		return result, WithMachineErrorCode(MachineCodeValidationFailed,
			fmt.Errorf("task loop %s candidate failed validation: %s", input.Operation, strings.Join(validation.Violations, "; ")))
	}
	if input.DryRun {
		return result, nil
	}

	validation, err = s.commitTaskWriter(own, writer, taskWriterLedger{
		state: &stateCandidate, preview: preview,
		published: []repotx.Candidate{managedCandidate(target.Path, target.Filename, after)},
		selected:  input.TaskID, tasks: tasks, written: []*Task{target}, corpus: corpus, baseline: baseline,
		taskSHA256Before: digestBytes(before), strict: true,
	})
	if err != nil {
		return LoopPolicyMutationResult{}, err
	}
	result.Applied = true
	result.Validation = validation
	return result, nil
}

func validateLoopPolicyMutationInput(input LoopPolicyMutationInput) error {
	switch input.Operation {
	case LoopPolicyAllow, LoopPolicyHold:
		policy, reason := string(input.Operation), input.Reason
		if violations := ValidateLoopPolicyMetadata(LoopPolicyMetadata{Policy: &policy, Reason: &reason}); len(violations) > 0 {
			return WithMachineErrorCode(MachineCodeInvalidReason, errors.New(strings.Join(violations, "; ")))
		}
	case LoopPolicyClear:
		if input.Reason != "" {
			return invalidArgumentsf("task loop clear does not accept a reason")
		}
	default:
		return WithMachineErrorCode(MachineCodePolicyInvalid, fmt.Errorf("unknown loop policy operation %q", input.Operation))
	}
	return nil
}

func setCandidateLoopPolicy(frontmatter *TaskFrontmatter, input LoopPolicyMutationInput) {
	if input.Operation == LoopPolicyClear {
		frontmatter.Policy = nil
		frontmatter.Reason = nil
		frontmatter.policyPresent = false
		frontmatter.reasonPresent = false
		return
	}
	policy, reason := string(input.Operation), input.Reason
	frontmatter.Policy = &policy
	frontmatter.Reason = &reason
	frontmatter.policyPresent = true
	frontmatter.reasonPresent = true
}

func loopPolicyMutationPolicy(meta LoopPolicyMetadata) LoopPolicyMutationPolicy {
	effective := ResolveLoopPolicy(meta)
	policy, reason := copyLoopPolicyValue(meta.Policy), copyLoopPolicyValue(meta.Reason)
	return LoopPolicyMutationPolicy{
		Source: effective.Source, EffectivePolicy: effective.Policy, Reason: effective.Reason,
		PersistedPolicy: policy, PersistedReason: reason,
	}
}

func copyLoopPolicyValue(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func replaceLoopPolicyTask(tasks []*Task, target, candidate *Task) []*Task {
	preview := slices.Clone(tasks)
	for i, task := range preview {
		if task == target {
			preview[i] = candidate
			break
		}
	}
	return preview
}

func replaceLoopPolicyFields(data []byte, taskID string, input LoopPolicyMutationInput, updatedAt string) ([]byte, error) {
	frontmatter, body, newline, err := splitTaskDocument(data, taskID)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(frontmatter, newline)
	result := make([]string, 0, len(lines)+2)
	policyCount, reasonCount, updated := 0, 0, false
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "loop_policy:"):
			policyCount++
			continue
		case strings.HasPrefix(line, "loop_reason:"):
			reasonCount++
			continue
		case strings.HasPrefix(line, "updated_at:"):
			if input.Operation != LoopPolicyClear {
				result = append(result, "loop_policy: "+string(input.Operation), "loop_reason: "+strconv.Quote(input.Reason))
			}
			result = append(result, "updated_at: "+strconv.Quote(updatedAt))
			updated = true
			continue
		}
		result = append(result, line)
	}
	if policyCount > 1 || reasonCount > 1 || !updated {
		return nil, fmt.Errorf("task %s has unpatchable loop policy frontmatter", taskID)
	}
	return []byte(strings.Join(result, newline) + body), nil
}
