package taskrail

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"slices"
)

func (s *Service) negotiateReviewAdapter(ctx context.Context, invocation LoopInvocation, batch *ParallelBatch, stderr io.Writer) error {
	id, err := randomLoopInvocationID()
	if err != nil {
		return err
	}
	request := ReviewCapabilityRequest{SchemaVersion: 2, RequestID: id, Operation: "capabilities", Repository: s.paths.WorktreeRoot,
		TargetRef: batch.Plan.BaseRef, Timeout: stringPtr(invocation.Timeout.String())}
	receipt := &ReviewCapabilityReceipt{Request: request}
	batch.Delivery.Capability = receipt
	adapterContext, cancel := context.WithTimeout(ctx, *invocation.Timeout)
	defer cancel()
	data, err := runReviewDocument(adapterContext, *batch.Plan.ReviewAdapter, request.Repository, request, stderr)
	if err == nil {
		var result ReviewCapabilityResult
		err = decodeReviewDocument(data, []string{"schema_version", "request_id", "operation", "target_id", "capabilities", "message"}, nil, &result)
		if err == nil {
			receipt.Result = &result
			if result.SchemaVersion != 2 || result.RequestID != id || result.Operation != "capabilities" || result.TargetID == "" ||
				!slices.Contains(result.Capabilities, "checked_combined_ff_v1") {
				err = fmt.Errorf("adapter does not support checked_combined_ff_v1")
			}
		}
	}
	if err != nil {
		receipt.Error = stringPtr(err.Error())
	}
	return err
}

func callCheckedReviewAdapter(ctx context.Context, invocation LoopInvocation, batch *ParallelBatch, request ReviewAdapterRequest, stderr io.Writer) (ReviewAdapterResult, error) {
	id, err := randomLoopInvocationID()
	if err != nil {
		return ReviewAdapterResult{}, err
	}
	request.RequestID = id
	receipt := ReviewOperationReceipt{Request: request, Outcome: "unknown"}
	adapterContext, cancel := context.WithTimeout(ctx, *invocation.Timeout)
	defer cancel()
	result, err := runReviewAdapter(adapterContext, *batch.Plan.ReviewAdapter, request, stderr)
	receipt.Result = result
	if err != nil {
		receipt.Error = stringPtr(err.Error())
	} else {
		receipt.Outcome = result.State
		batch.Delivery.Remote = "adapter_reported"
	}
	batch.Delivery.Operations = append(batch.Delivery.Operations, receipt)
	if result == nil {
		return ReviewAdapterResult{}, err
	}
	return *result, err
}

// The adapter must implement the final compare-and-merge itself. A coordinator
// inspection alone cannot prevent hosted target or approval TOCTOU races.
func (s *Service) deliverParallelReviews(ctx context.Context, invocation LoopInvocation, snapshot LoopPreflightSnapshot, batch *ParallelBatch, sourceBefore loopCheckSnapshot, stderr io.Writer) []MachineViolation {
	defer func() {
		for _, worker := range batch.Workers {
			if !slices.Contains(batch.Delivery.PublishedTasks, worker.Task.TaskID) {
				batch.Delivery.PendingTasks = append(batch.Delivery.PendingTasks, worker.Task.TaskID)
			}
		}
	}()
	if !batch.Integration.AggregatePass || batch.Integration.Head == nil || batch.Integration.Check == nil || len(batch.Integration.AcceptedTasks) == 0 {
		batch.Delivery.Publication = "refused"
		return nil
	}
	integration, err := NewService(*batch.Integration.Workspace)
	if err != nil {
		return reviewDeliveryRefusal(batch, err)
	}
	if batch.Integration.checked == nil || batch.Integration.checked.Git.Head != *batch.Integration.Head || batch.Integration.Check.Head != *batch.Integration.Head {
		return reviewDeliveryRefusal(batch, fmt.Errorf("checked integration binding is unavailable"))
	}
	checked := *batch.Integration.checked
	request := ReviewAdapterRequest{SchemaVersion: 2, Repository: *batch.Integration.Workspace, TargetID: batch.Delivery.Capability.Result.TargetID,
		TargetRef: batch.Plan.BaseRef, SourceRef: "refs/heads/taskrail-review/" + batch.Delivery.Capability.Request.RequestID,
		ExpectedTargetHead: batch.Plan.BaseHead, CandidateHead: checked.Git.Head, Check: batch.Integration.Check,
		Timeout: stringPtr(invocation.Timeout.String())}
	for _, operation := range []string{"publish_branch", "open_change", "inspect_change", "inspect_change", "merge_change"} {
		if err := s.reviewPublicationBinding(snapshot, sourceBefore, integration, checked); err != nil {
			return reviewDeliveryRefusal(batch, err)
		}
		if err := ctx.Err(); err != nil {
			return reviewDeliveryRefusal(batch, err)
		}
		request.Operation = operation
		result, callErr := callCheckedReviewAdapter(ctx, invocation, batch, request, stderr)
		if callErr != nil || result.State != "applied" {
			// An inspection cannot change publication; a failed mutating call may
			// have applied even when its result is absent or untrustworthy.
			if operation != "inspect_change" && (callErr != nil || result.State == "unknown") {
				batch.Delivery.Publication = "unknown"
			}
			if callErr == nil {
				callErr = fmt.Errorf("adapter %s: %s: %s", operation, result.State, result.Message)
			}
			return reviewDeliveryRefusal(batch, callErr)
		}
		switch operation {
		case "open_change":
			request.ChangeID = result.ChangeID
		case "inspect_change":
			if !reviewApproved(result) || (request.ApprovalToken != nil && !equalOptionalString(request.ApprovalToken, result.ApprovalToken)) {
				return reviewDeliveryRefusal(batch, fmt.Errorf("adapter approval/check is pending, refused, or changed"))
			}
			request.ApprovalToken = result.ApprovalToken
		case "merge_change":
			batch.Delivery.Publication = "applied"
			batch.Delivery.HeadAfter = result.MergeHead
			batch.Delivery.PublishedTasks = append(batch.Delivery.PublishedTasks, batch.Integration.AcceptedTasks...)
		}
		if err := s.reviewPublicationBinding(snapshot, sourceBefore, integration, checked); err != nil {
			return reviewDeliveryRefusal(batch, err)
		}
	}
	return nil
}

func reviewDeliveryRefusal(batch *ParallelBatch, err error) []MachineViolation {
	if batch.Delivery.Publication == "not_attempted" {
		batch.Delivery.Publication = "refused"
	}
	return []MachineViolation{parallelViolation("adapter_failed", err.Error())}
}

func (s *Service) reviewPublicationBinding(snapshot LoopPreflightSnapshot, sourceBefore loopCheckSnapshot, integration *Service, checked loopCheckSnapshot) error {
	sourceAfter, err := s.loopCheckSnapshot()
	if err != nil || !reflect.DeepEqual(sourceBefore, sourceAfter) {
		return fmt.Errorf("source repository changed before/during review publication: %v", err)
	}
	if !reflect.DeepEqual(sourceAfter.Inputs, snapshot.Inputs()) || !reflect.DeepEqual(sourceAfter.Git, snapshot.Git()) ||
		!reflect.DeepEqual(sourceAfter.RootRefs, snapshot.RootRefs()) || !reflect.DeepEqual(sourceAfter.GitConfig, snapshot.GitConfig()) {
		return fmt.Errorf("source repository no longer matches frozen preflight")
	}
	if integration != nil {
		after, err := integration.loopCheckSnapshot()
		if err != nil || !reflect.DeepEqual(checked, after) {
			return fmt.Errorf("checked integration repository changed before/during review publication: %v", err)
		}
	}
	check, err := resolveLoopCheck(LoopInvocation{Check: snapshot.check.Path})
	if err != nil || !reflect.DeepEqual(check, snapshot.check) {
		return fmt.Errorf("check executable changed before/during review publication: %v", err)
	}
	return nil
}
