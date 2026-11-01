package taskrail

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
)

// ReviewAdapterRequest binds a caller-owned operation to the locally checked
// combined commit. Approval tokens are adapter claims, not reviewer attestation.
type ReviewAdapterRequest struct {
	SchemaVersion      int            `json:"schema_version"`
	RequestID          string         `json:"request_id"`
	Operation          string         `json:"operation"`
	Repository         string         `json:"repository"`
	TargetID           string         `json:"target_id"`
	TargetRef          string         `json:"target_ref"`
	SourceRef          string         `json:"source_ref"`
	ChangeID           *string        `json:"change_id"`
	ExpectedTargetHead string         `json:"expected_target_head"`
	CandidateHead      string         `json:"candidate_head"`
	Check              *ParallelCheck `json:"check"`
	ApprovalToken      *string        `json:"approval_token"`
	Timeout            *string        `json:"timeout"`
}

type ReviewAdapterResult struct {
	SchemaVersion int     `json:"schema_version"`
	RequestID     string  `json:"request_id"`
	Operation     string  `json:"operation"`
	State         string  `json:"state"`
	TargetID      string  `json:"target_id"`
	ChangeID      *string `json:"change_id"`
	SourceRef     string  `json:"source_ref"`
	TargetRef     string  `json:"target_ref"`
	SourceHead    string  `json:"source_head"`
	TargetHead    string  `json:"target_head"`
	Checks        string  `json:"checks"`
	Approval      string  `json:"approval"`
	ApprovalToken *string `json:"approval_token"`
	MergeHead     *string `json:"merge_head"`
	Message       string  `json:"message"`
}

type ReviewCapabilityRequest struct {
	SchemaVersion int     `json:"schema_version"`
	RequestID     string  `json:"request_id"`
	Operation     string  `json:"operation"`
	Repository    string  `json:"repository"`
	TargetRef     string  `json:"target_ref"`
	Timeout       *string `json:"timeout"`
}

type ReviewCapabilityResult struct {
	SchemaVersion int      `json:"schema_version"`
	RequestID     string   `json:"request_id"`
	Operation     string   `json:"operation"`
	TargetID      string   `json:"target_id"`
	Capabilities  []string `json:"capabilities"`
	Message       string   `json:"message"`
}

type ReviewCapabilityReceipt struct {
	Request ReviewCapabilityRequest `json:"request"`
	Result  *ReviewCapabilityResult `json:"result"`
	Error   *string                 `json:"error"`
}

type ReviewOperationReceipt struct {
	Request ReviewAdapterRequest `json:"request"`
	Result  *ReviewAdapterResult `json:"result"`
	Outcome string               `json:"outcome"`
	Error   *string              `json:"error"`
}

// Adapter output is bounded even when it never forms a valid JSON document.
type reviewOutput struct{ buffer bytes.Buffer }

func (w *reviewOutput) Bytes() []byte { return w.buffer.Bytes() }

func (w *reviewOutput) Write(data []byte) (int, error) {
	if w.buffer.Len()+len(data) > reviewFileLimit {
		return 0, fmt.Errorf("adapter stdout exceeds 1 MiB")
	}
	return w.buffer.Write(data)
}

func runReviewDocument(ctx context.Context, adapter, repository string, request any, stderr io.Writer) ([]byte, error) {
	input, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	var stdout reviewOutput
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	execution, err := launchLoopChild(loopChildLaunch{Command: []string{adapter}, Context: ctx, Prompt: input,
		RepositoryRoot: repository, Stdout: &stdout, Stderr: stderr})
	if err != nil {
		return nil, fmt.Errorf("launch adapter: %w", err)
	}
	if !parallelChildPassed(execution) {
		child, _ := json.Marshal(loopIterationChild(execution))
		return nil, fmt.Errorf("adapter did not complete successfully (outcome unknown): %s captured_stdout_sha256=%x: %v", child, sha256.Sum256(stdout.Bytes()),
			errors.Join(ctx.Err(), execution.SpawnError, execution.StdinError, execution.StdoutError, execution.StderrError, execution.WaitError, execution.CancellationError))
	}
	return stdout.Bytes(), nil
}

func runReviewAdapter(ctx context.Context, adapter string, request ReviewAdapterRequest, stderr io.Writer) (*ReviewAdapterResult, error) {
	data, err := runReviewDocument(ctx, adapter, request.Repository, request, stderr)
	if err != nil {
		return nil, err
	}
	result, err := decodeReviewAdapterResult(data)
	if err != nil {
		return nil, err
	}
	// Keep a well-formed receipt even when its identity or merge claim is unsafe.
	return &result, validateReviewAdapterResult(request, result)
}

func decodeReviewDocument(data []byte, fields, nullable []string, result any) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("adapter stdout_sha256=%x: %w", sha256.Sum256(data), err)
		}
	}()
	obj, err := decodeReviewObject(data, "adapter result")
	if err != nil {
		return err
	}
	if err := exactMembers(obj, "adapter result", fields); err != nil {
		return err
	}
	for name, value := range obj {
		if bytes.Equal(value, []byte("null")) && !slices.Contains(nullable, name) {
			return fmt.Errorf("adapter result has null %s", name)
		}
	}
	if raw, ok := obj["capabilities"]; ok {
		elements, err := arrayMember(raw, "adapter result", "capabilities")
		if err != nil {
			return err
		}
		for _, element := range elements {
			if bytes.Equal(element, []byte("null")) {
				return fmt.Errorf("adapter capability is null")
			}
		}
	}
	return json.Unmarshal(data, result)
}

func decodeReviewAdapterResult(data []byte) (ReviewAdapterResult, error) {
	var result ReviewAdapterResult
	err := decodeReviewDocument(data, []string{"schema_version", "request_id", "operation", "state", "target_id", "change_id", "source_ref", "target_ref",
		"source_head", "target_head", "checks", "approval", "approval_token", "merge_head", "message"}, []string{"change_id", "approval_token", "merge_head"}, &result)
	return result, err
}

func validateReviewAdapterResult(request ReviewAdapterRequest, result ReviewAdapterResult) error {
	if request.SchemaVersion != 2 || !slices.Contains([]string{"publish_branch", "open_change", "inspect_change", "merge_change"}, request.Operation) ||
		request.RequestID == "" || request.Repository == "" || request.TargetID == "" || request.TargetRef == "" || request.SourceRef == "" ||
		request.ExpectedTargetHead == "" || request.CandidateHead == "" || request.Check == nil ||
		request.Check.Head != request.CandidateHead || !loopCheckPassed(request.Check.Evidence) {
		return fmt.Errorf("invalid checked adapter request")
	}
	if result.SchemaVersion != 2 || result.RequestID != request.RequestID || result.Operation != request.Operation ||
		result.TargetID != request.TargetID || result.SourceRef != request.SourceRef || result.TargetRef != request.TargetRef {
		return fmt.Errorf("adapter result does not match its request")
	}
	if !slices.Contains([]string{"applied", "refused", "unknown"}, result.State) ||
		!slices.Contains([]string{"pass", "fail", "pending", "unknown"}, result.Checks) ||
		!slices.Contains([]string{"approved", "refused", "pending", "unknown"}, result.Approval) {
		return fmt.Errorf("adapter result has invalid state")
	}
	if request.Operation == "open_change" && result.State == "applied" {
		if result.ChangeID == nil || *result.ChangeID == "" {
			return fmt.Errorf("open_change result is missing change_id")
		}
	} else if !equalOptionalString(request.ChangeID, result.ChangeID) {
		return fmt.Errorf("adapter result change_id does not match its request")
	}
	if (request.Operation != "merge_change" || result.State == "refused") && result.MergeHead != nil {
		return fmt.Errorf("non-merge or refused result includes merge_head")
	}
	if result.State != "applied" {
		return nil
	}
	if result.SourceHead != request.CandidateHead || result.TargetHead != request.ExpectedTargetHead {
		return fmt.Errorf("adapter observed source or target drift")
	}
	if request.Operation == "merge_change" && (result.MergeHead == nil || *result.MergeHead != request.CandidateHead ||
		!reviewApproved(result) || !equalOptionalString(request.ApprovalToken, result.ApprovalToken)) {
		return fmt.Errorf("adapter merge is not the exact checked and approved head")
	}
	return nil
}

func reviewApproved(result ReviewAdapterResult) bool {
	return result.State == "applied" && result.Checks == "pass" && result.Approval == "approved" &&
		result.ApprovalToken != nil && *result.ApprovalToken != ""
}

func equalOptionalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}
