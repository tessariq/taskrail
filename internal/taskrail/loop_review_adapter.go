package taskrail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

// ReviewAdapterRequest is the complete provider-neutral input for one adapter
// operation. The adapter owns authentication and provider-specific behavior.
type ReviewAdapterRequest struct {
	SchemaVersion      int     `json:"schema_version"`
	RequestID          string  `json:"request_id"`
	Operation          string  `json:"operation"`
	Repository         string  `json:"repository"`
	TargetRef          string  `json:"target_ref"`
	SourceRef          string  `json:"source_ref"`
	ChangeID           *string `json:"change_id"`
	ExpectedTargetHead string  `json:"expected_target_head"`
	CandidateHead      string  `json:"candidate_head"`
	Timeout            *string `json:"timeout"`
}

// ReviewAdapterResult is the sole trusted source for provider state.
type ReviewAdapterResult struct {
	SchemaVersion int     `json:"schema_version"`
	RequestID     string  `json:"request_id"`
	Operation     string  `json:"operation"`
	Applied       bool    `json:"applied"`
	ChangeID      *string `json:"change_id"`
	SourceRef     string  `json:"source_ref"`
	TargetRef     string  `json:"target_ref"`
	SourceHead    string  `json:"source_head"`
	TargetHead    string  `json:"target_head"`
	Checks        string  `json:"checks"`
	MergeHead     *string `json:"merge_head"`
	Message       string  `json:"message"`
}

func runReviewAdapter(ctx context.Context, adapter string, request ReviewAdapterRequest, stderr io.Writer) (ReviewAdapterResult, error) {
	input, err := json.Marshal(request)
	if err != nil {
		return ReviewAdapterResult{}, fmt.Errorf("encode adapter request: %w", err)
	}
	var stdout bytes.Buffer
	execution, err := launchLoopChild(loopChildLaunch{Command: []string{adapter}, Context: ctx, Prompt: input,
		RepositoryRoot: request.Repository, Stdout: &stdout, Stderr: stderr})
	if err != nil {
		return ReviewAdapterResult{}, fmt.Errorf("launch adapter: %w", err)
	}
	if execution.Failed() || execution.ExitCode == nil || *execution.ExitCode != 0 || execution.Signal != "" || execution.TimedOut {
		if ctx.Err() != nil {
			return ReviewAdapterResult{}, fmt.Errorf("adapter timed out: %w", ctx.Err())
		}
		return ReviewAdapterResult{}, fmt.Errorf("adapter exited unsuccessfully")
	}
	result, err := decodeReviewAdapterResult(stdout.Bytes())
	if err != nil {
		return ReviewAdapterResult{}, err
	}
	if err := validateReviewAdapterResult(request, result); err != nil {
		return ReviewAdapterResult{}, err
	}
	return result, nil
}

func decodeReviewAdapterResult(data []byte) (ReviewAdapterResult, error) {
	if !utf8.Valid(data) {
		return ReviewAdapterResult{}, fmt.Errorf("adapter result is not valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return ReviewAdapterResult{}, fmt.Errorf("adapter result must be one JSON object")
	}
	known := map[string]bool{"schema_version": true, "request_id": true, "operation": true, "applied": true,
		"change_id": true, "source_ref": true, "target_ref": true, "source_head": true, "target_head": true,
		"checks": true, "merge_head": true, "message": true}
	values := make(map[string]json.RawMessage, len(known))
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return ReviewAdapterResult{}, fmt.Errorf("read adapter result field: %w", err)
		}
		name, ok := key.(string)
		if !ok || !known[name] || values[name] != nil {
			return ReviewAdapterResult{}, fmt.Errorf("adapter result has duplicate or unknown field")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return ReviewAdapterResult{}, fmt.Errorf("read adapter result %s: %w", name, err)
		}
		values[name] = value
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
		return ReviewAdapterResult{}, fmt.Errorf("close adapter result: %w", err)
	}
	if decoder.More() {
		return ReviewAdapterResult{}, fmt.Errorf("adapter returned more than one result")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return ReviewAdapterResult{}, fmt.Errorf("adapter returned more than one result")
	}
	for name, value := range values {
		if value == nil || (name != "change_id" && name != "merge_head" && bytes.Equal(value, []byte("null"))) {
			return ReviewAdapterResult{}, fmt.Errorf("adapter result is missing required field %s", name)
		}
	}
	if len(values) != len(known) {
		return ReviewAdapterResult{}, fmt.Errorf("adapter result is missing required fields")
	}
	var result ReviewAdapterResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ReviewAdapterResult{}, fmt.Errorf("decode adapter result: %w", err)
	}
	return result, nil
}

func validateReviewAdapterResult(request ReviewAdapterRequest, result ReviewAdapterResult) error {
	if request.SchemaVersion != 1 || !reviewAdapterOperation(request.Operation) || request.RequestID == "" ||
		request.Repository == "" || request.TargetRef == "" || request.SourceRef == "" ||
		request.ExpectedTargetHead == "" || request.CandidateHead == "" {
		return fmt.Errorf("invalid adapter request")
	}
	if result.SchemaVersion != 1 || result.RequestID != request.RequestID || result.Operation != request.Operation ||
		result.SourceRef != request.SourceRef || result.TargetRef != request.TargetRef ||
		result.SourceHead != request.CandidateHead || result.TargetHead != request.ExpectedTargetHead {
		return fmt.Errorf("adapter result does not match its request")
	}
	if result.Checks != "pass" && result.Checks != "fail" && result.Checks != "pending" && result.Checks != "unknown" {
		return fmt.Errorf("adapter result has invalid checks state")
	}
	if request.Operation == "open_change" {
		if result.ChangeID == nil || *result.ChangeID == "" {
			return fmt.Errorf("open_change result is missing change_id")
		}
	} else if !equalOptionalString(request.ChangeID, result.ChangeID) {
		return fmt.Errorf("adapter result change_id does not match its request")
	}
	if request.Operation == "merge_change" {
		if result.Applied && (result.MergeHead == nil || *result.MergeHead == "") {
			return fmt.Errorf("merged result is missing merge_head")
		}
	} else if result.MergeHead != nil {
		return fmt.Errorf("non-merge result includes merge_head")
	}
	return nil
}

func reviewAdapterOperation(operation string) bool {
	return operation == "publish_branch" || operation == "open_change" || operation == "inspect_change" ||
		operation == "update_change" || operation == "merge_change"
}

func equalOptionalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

type reviewChange struct {
	changeID  *string
	sourceRef string
	ready     bool
}

func (s *Service) deliverParallelReviews(ctx context.Context, invocation LoopInvocation, snapshot LoopPreflightSnapshot, plan ParallelPlan, root string, batch *ParallelBatch, stdout, stderr io.Writer) (violations []MachineViolation) {
	changes := make([]reviewChange, len(batch.Workers))
	var wait sync.WaitGroup
	for i := range batch.Workers {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			worker := &batch.Workers[i]
			if worker.Outcome != "completed_pass" || worker.CandidateHead == nil || worker.Workspace == nil {
				return
			}
			sourceRef := reviewSourceRef(worker.Task.TaskID)
			changes[i] = reviewChange{sourceRef: sourceRef}
			published, err := callReviewAdapter(ctx, invocation, plan, stderr,
				reviewAdapterRequest("publish_branch", *worker.Workspace, plan.BaseRef, sourceRef, nil, plan.BaseHead, *worker.CandidateHead, invocation))
			if err != nil || !published.Applied {
				reviewAdapterFailure(worker, err, "adapter refused branch publication")
				return
			}
			opened, err := callReviewAdapter(ctx, invocation, plan, stderr,
				reviewAdapterRequest("open_change", *worker.Workspace, plan.BaseRef, sourceRef, nil, plan.BaseHead, *worker.CandidateHead, invocation))
			if err != nil || !opened.Applied {
				reviewAdapterFailure(worker, err, "adapter refused change creation")
				return
			}
			changes[i].changeID, changes[i].ready = opened.ChangeID, true
		}(i)
	}
	wait.Wait()

	integrationRoot := filepath.Join(root, "integration")
	if !hasReviewChange(changes) {
		appendReviewPending(batch)
		return
	}
	if err := parallelClone(s.paths.WorktreeRoot, integrationRoot, plan.BaseRef, plan.BaseHead, reviewCloneDepth(plan)); err != nil {
		for i := range batch.Workers {
			if changes[i].ready {
				reviewAdapterFailure(&batch.Workers[i], err, "create review integration workspace")
			}
		}
		appendReviewPending(batch)
		return
	}
	batch.Integration.Workspace = stringPtr(integrationRoot)
	targetHead := plan.BaseHead
	for i := range batch.Workers {
		worker := &batch.Workers[i]
		change := &changes[i]
		if !change.ready || worker.CandidateHead == nil || worker.Workspace == nil {
			continue
		}
		inspected, err := callReviewAdapter(ctx, invocation, plan, stderr, reviewAdapterRequest("inspect_change", *worker.Workspace, plan.BaseRef, change.sourceRef, change.changeID, targetHead, *worker.CandidateHead, invocation))
		if err != nil || !inspected.Applied || inspected.Checks != "pass" {
			reviewAdapterFailure(worker, err, "change is not approved with passing checks")
			continue
		}
		merged, err := callReviewAdapter(ctx, invocation, plan, stderr, reviewAdapterRequest("merge_change", *worker.Workspace, plan.BaseRef, change.sourceRef, change.changeID, targetHead, *worker.CandidateHead, invocation))
		if err != nil || !merged.Applied || merged.Checks != "pass" || merged.MergeHead == nil {
			reviewAdapterFailure(worker, err, "adapter refused change merge")
			continue
		}
		batch.Delivery.PublishedTasks = append(batch.Delivery.PublishedTasks, worker.Task.TaskID)
		targetHead = *merged.MergeHead
		batch.Delivery.HeadAfter = &targetHead
		if err := syncReviewTarget(s.paths.WorktreeRoot, integrationRoot, *worker.Workspace, targetHead); err != nil {
			reviewAdapterFailure(worker, err, "record merged change for refresh")
			appendReviewPending(batch)
			return
		}
		worker.Integrated = true
		worker.IntegratedCommit = &targetHead
		batch.Integration.AcceptedTasks = append(batch.Integration.AcceptedTasks, worker.Task.TaskID)
		if err := s.refreshOpenReviewChanges(ctx, invocation, snapshot, plan, integrationRoot, targetHead, batch, changes, i, stdout, stderr); err != nil {
			batch.Integration.AggregatePass = false
			return
		}
	}
	aggregatePass, violations := s.runParallelAggregateChild(ctx, invocation, snapshot, integrationRoot, batch, stdout, stderr)
	batch.Integration.AggregatePass = aggregatePass
	if batch.Integration.AggregatePass {
		batch.Integration.Head = batch.Delivery.HeadAfter
		batch.Delivery.Remote = "adapter_reported"
	}
	appendReviewPending(batch)
	return violations
}

func appendReviewPending(batch *ParallelBatch) {
	for _, worker := range batch.Workers {
		if !worker.Integrated && !containsReviewTask(batch.Delivery.PublishedTasks, worker.Task.TaskID) {
			batch.Delivery.PendingTasks = append(batch.Delivery.PendingTasks, worker.Task.TaskID)
		}
	}
}

func containsReviewTask(tasks []string, taskID string) bool {
	for _, candidate := range tasks {
		if candidate == taskID {
			return true
		}
	}
	return false
}

func reviewCloneDepth(plan ParallelPlan) int {
	if plan.Workspace.CloneDepth == nil {
		return 0
	}
	return *plan.Workspace.CloneDepth
}

func hasReviewChange(changes []reviewChange) bool {
	for _, change := range changes {
		if change.ready {
			return true
		}
	}
	return false
}

func reviewSourceRef(taskID string) string {
	return "refs/heads/taskrail-review/" + taskID
}

func reviewAdapterRequest(operation, repository, targetRef, sourceRef string, changeID *string, targetHead, candidateHead string, invocation LoopInvocation) ReviewAdapterRequest {
	timeout := (*string)(nil)
	if invocation.Timeout != nil {
		value := invocation.Timeout.String()
		timeout = &value
	}
	return ReviewAdapterRequest{SchemaVersion: 1, Operation: operation, Repository: repository, TargetRef: targetRef,
		SourceRef: sourceRef, ChangeID: changeID, ExpectedTargetHead: targetHead, CandidateHead: candidateHead, Timeout: timeout}
}

func callReviewAdapter(ctx context.Context, invocation LoopInvocation, plan ParallelPlan, stderr io.Writer, request ReviewAdapterRequest) (ReviewAdapterResult, error) {
	if plan.ReviewAdapter == nil {
		return ReviewAdapterResult{}, fmt.Errorf("review adapter is unavailable")
	}
	id, err := randomLoopInvocationID()
	if err != nil {
		return ReviewAdapterResult{}, err
	}
	request.RequestID = id
	adapterContext := ctx
	cancel := func() {}
	if invocation.Timeout != nil {
		adapterContext, cancel = context.WithTimeout(ctx, *invocation.Timeout)
	}
	defer cancel()
	return runReviewAdapter(adapterContext, *plan.ReviewAdapter, request, stderr)
}

func reviewAdapterFailure(worker *ParallelWorker, err error, fallback string) {
	message := fallback
	if err != nil {
		message = err.Error()
	}
	worker.Outcome = "integration_failed"
	worker.Violations = append(worker.Violations, parallelViolation("adapter_failed", message))
}

func syncReviewTarget(source, root, worker, targetHead string) error {
	if remote, err := reviewTargetRemote(source); err == nil {
		if output, fetchErr := exec.Command("git", "-C", root, "fetch", "--no-tags", remote, targetHead).CombinedOutput(); fetchErr == nil {
			if fetched, resolveErr := gitCommand(root, "rev-parse", "FETCH_HEAD"); resolveErr == nil && strings.TrimSpace(fetched) == targetHead {
				return resetReviewTarget(root)
			}
		} else {
			return fmt.Errorf("fetch adapter merge head: %w: %s", fetchErr, strings.TrimSpace(string(output)))
		}
	}
	if output, err := exec.Command("git", "-C", root, "fetch", "--no-tags", parallelFileURL(worker), targetHead).CombinedOutput(); err != nil {
		return fmt.Errorf("fetch adapter merge head: %w: %s", err, strings.TrimSpace(string(output)))
	}
	fetched, err := gitCommand(root, "rev-parse", "FETCH_HEAD")
	if err != nil || strings.TrimSpace(fetched) != targetHead {
		return fmt.Errorf("adapter merge head is unavailable in its repository")
	}
	return resetReviewTarget(root)
}

func reviewTargetRemote(source string) (string, error) {
	branch, err := gitCommand(source, "symbolic-ref", "--short", "HEAD")
	if err != nil || strings.TrimSpace(branch) == "" {
		return "", fmt.Errorf("read source branch remote")
	}
	remote, err := gitCommand(source, "config", "--get", "branch."+strings.TrimSpace(branch)+".remote")
	if err != nil || strings.TrimSpace(remote) == "" {
		return "", fmt.Errorf("source branch has no configured remote")
	}
	return strings.TrimSpace(remote), nil
}

func resetReviewTarget(root string) error {
	if output, err := exec.Command("git", "-C", root, "reset", "--hard", "FETCH_HEAD").CombinedOutput(); err != nil {
		return fmt.Errorf("check out adapter merge head: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (s *Service) refreshOpenReviewChanges(ctx context.Context, invocation LoopInvocation, snapshot LoopPreflightSnapshot, plan ParallelPlan, integrationRoot, targetHead string, batch *ParallelBatch, changes []reviewChange, mergedIndex int, stdout, stderr io.Writer) error {
	for i := range batch.Workers {
		worker := &batch.Workers[i]
		change := &changes[i]
		if i == mergedIndex || !change.ready || worker.Integrated || worker.Outcome != "completed_pass" || worker.Workspace == nil || worker.CandidateHead == nil {
			continue
		}
		if err := s.refreshReviewCandidate(ctx, invocation, snapshot, integrationRoot, worker, batch, stdout, stderr); err != nil {
			reviewAdapterFailure(worker, err, "refresh change against merged target")
			continue
		}
		head, err := gitCommand(*worker.Workspace, "rev-parse", "HEAD")
		if err != nil {
			reviewAdapterFailure(worker, err, "read refreshed change head")
			continue
		}
		head = strings.TrimSpace(head)
		worker.CandidateHead = &head
		updated, err := callReviewAdapter(ctx, invocation, plan, stderr, reviewAdapterRequest("update_change", *worker.Workspace, plan.BaseRef, change.sourceRef, change.changeID, targetHead, head, invocation))
		if err != nil || !updated.Applied || updated.Checks != "pass" {
			reviewAdapterFailure(worker, err, "adapter refused refreshed change")
		}
	}
	return nil
}

func (s *Service) refreshReviewCandidate(ctx context.Context, invocation LoopInvocation, snapshot LoopPreflightSnapshot, integrationRoot string, worker *ParallelWorker, batch *ParallelBatch, stdout, stderr io.Writer) error {
	if worker == nil || worker.Workspace == nil {
		return fmt.Errorf("review refresh worker is unavailable")
	}
	head, err := gitCommand(integrationRoot, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("read integration head: %w", err)
	}
	if output, err := exec.Command("git", "-C", *worker.Workspace, "fetch", "--no-tags", parallelFileURL(integrationRoot), strings.TrimSpace(head)).CombinedOutput(); err != nil {
		return fmt.Errorf("fetch merged target: %w: %s", err, strings.TrimSpace(string(output)))
	}
	childResolved := false
	if output, err := exec.Command("git", "-C", *worker.Workspace, "rebase", "FETCH_HEAD").CombinedOutput(); err != nil {
		if repairErr := repairReviewRebaseStateConflict(*worker.Workspace); repairErr != nil {
			binding, bindingErr := parallelConflictBindingAtHead(strings.TrimSpace(head), "FETCH_HEAD", worker)
			if bindingErr != nil {
				return bindingErr
			}
			child, childErr := s.runParallelIntegrationChild(ctx, invocation, snapshot, *worker.Workspace, worker, binding, stdout, stderr)
			batch.Integration.Children = append(batch.Integration.Children, child)
			if childErr != nil || child.Outcome != "pass" {
				return fmt.Errorf("rebase refreshed change: %w: %s", err, strings.TrimSpace(string(output)))
			}
			env, envErr := parallelCommitEnvironment(*worker.Workspace, "ORIG_HEAD")
			if envErr != nil {
				return envErr
			}
			continueRebase := exec.Command("git", "-C", *worker.Workspace, "-c", "core.editor=true", "rebase", "--continue")
			continueRebase.Env = append(os.Environ(), env...)
			if continueOutput, continueErr := continueRebase.CombinedOutput(); continueErr != nil {
				return fmt.Errorf("continue semantic review rebase: %w: %s", continueErr, strings.TrimSpace(string(continueOutput)))
			}
			childResolved = true
		}
	}
	service, err := NewService(*worker.Workspace)
	if err != nil {
		return fmt.Errorf("open refreshed change: %w", err)
	}
	if _, err := service.Repair(RepairInput{Apply: true}); err != nil {
		return fmt.Errorf("reproject refreshed state: %w", err)
	}
	if output, err := exec.Command("git", "-C", *worker.Workspace, "add", "--", service.reportedStatePath()).CombinedOutput(); err != nil {
		return fmt.Errorf("stage refreshed state: %w: %s", err, strings.TrimSpace(string(output)))
	}
	env, err := parallelCommitEnvironment(*worker.Workspace, "HEAD")
	if err != nil {
		return err
	}
	command := exec.Command("git", "-C", *worker.Workspace, "commit", "--amend", "--no-edit")
	command.Env = append(os.Environ(), env...)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("commit refreshed state: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if childResolved {
		return verifyReviewWorkspaceClean(*worker.Workspace)
	}
	refreshed, err := gitCommand(*worker.Workspace, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("read refreshed change head: %w", err)
	}
	worker.CandidateHead = stringPtr(strings.TrimSpace(refreshed))
	binding, err := parallelConflictBindingAtHead(strings.TrimSpace(head), "FETCH_HEAD", worker)
	if err != nil {
		return err
	}
	child, childErr := s.runParallelIntegrationChild(ctx, invocation, snapshot, *worker.Workspace, worker, binding, stdout, stderr)
	batch.Integration.Children = append(batch.Integration.Children, child)
	if childErr != nil || child.Outcome != "pass" {
		return fmt.Errorf("refreshed change checks failed")
	}
	return verifyReviewWorkspaceClean(*worker.Workspace)
}

func runReviewRefreshChild(ctx context.Context, invocation LoopInvocation, worker string, stdout, stderr io.Writer, prompt string) error {
	execution, err := launchLoopChild(loopChildLaunch{Command: invocation.Child, Context: ctx, Timeout: invocation.Timeout, Prompt: []byte(prompt), RepositoryRoot: worker, Stdout: stdout, Stderr: stderr})
	if err != nil {
		return fmt.Errorf("run refreshed change checks: %w", err)
	}
	if execution.Failed() || execution.ExitCode == nil || *execution.ExitCode != 0 || execution.Signal != "" || execution.TimedOut {
		return fmt.Errorf("refreshed change checks failed")
	}
	return verifyReviewWorkspaceClean(worker)
}

func verifyReviewWorkspaceClean(worker string) error {
	status, err := gitCommand(worker, "status", "--porcelain")
	if err != nil || strings.TrimSpace(status) != "" {
		return fmt.Errorf("refreshed change is not clean")
	}
	return nil
}

func repairReviewRebaseStateConflict(root string) error {
	service, err := NewService(root)
	if err != nil {
		return err
	}
	state := service.reportedStatePath()
	output, err := exec.Command("git", "-C", root, "diff", "--name-only", "--diff-filter=U").Output()
	if err != nil || strings.TrimSpace(string(output)) != state {
		return fmt.Errorf("conflict is not limited to generated state")
	}
	if output, err := exec.Command("git", "-C", root, "checkout", "--ours", "--", state).CombinedOutput(); err != nil {
		return fmt.Errorf("restore aggregate state: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if output, err := exec.Command("git", "-C", root, "add", "--", state).CombinedOutput(); err != nil {
		return fmt.Errorf("stage aggregate state: %w: %s", err, strings.TrimSpace(string(output)))
	}
	env, err := parallelCommitEnvironment(root, "ORIG_HEAD")
	if err != nil {
		return err
	}
	command := exec.Command("git", "-C", root, "-c", "core.editor=true", "rebase", "--continue")
	command.Env = append(os.Environ(), env...)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("continue state-only rebase: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
