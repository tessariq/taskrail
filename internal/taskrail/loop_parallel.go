package taskrail

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
)

// ParallelWorker records one immutable worker result in frontier order.
type ParallelWorker struct {
	Rank             int                `json:"rank"`
	Task             TaskLoopRow        `json:"task"`
	Outcome          string             `json:"outcome"`
	Child            LoopIterationChild `json:"child"`
	CandidateHead    *string            `json:"candidate_head"`
	CandidateCommit  *string            `json:"candidate_commit"`
	Integrated       bool               `json:"integrated"`
	IntegratedCommit *string            `json:"integrated_commit"`
	Workspace        *string            `json:"workspace"`
	Violations       []MachineViolation `json:"violations"`
}

type ParallelIntegration struct {
	BaseHead      string              `json:"base_head"`
	Head          *string             `json:"head"`
	AcceptedTasks []string            `json:"accepted_tasks"`
	AggregatePass bool                `json:"aggregate_pass"`
	Child         *LoopIterationChild `json:"child"`
	Workspace     *string             `json:"workspace"`
}

type ParallelDelivery struct {
	Mode           string   `json:"mode"`
	Adapter        *string  `json:"adapter"`
	TargetRef      string   `json:"target_ref"`
	HeadBefore     string   `json:"head_before"`
	HeadAfter      *string  `json:"head_after"`
	PublishedTasks []string `json:"published_tasks"`
	PendingTasks   []string `json:"pending_tasks"`
	Remote         string   `json:"remote"`
}

type ParallelBatch struct {
	Plan        ParallelPlan        `json:"plan"`
	Workers     []ParallelWorker    `json:"workers"`
	Integration ParallelIntegration `json:"integration"`
	Delivery    ParallelDelivery    `json:"delivery"`
}

func parallelClone(source, destination, ref, expectedHead string, depth int) error {
	if source == "" || destination == "" || ref == "" {
		return fmt.Errorf("parallel clone requires source, destination, and ref")
	}
	args := []string{"clone", "--no-local", "--single-branch", "--no-tags", "--branch", strings.TrimPrefix(ref, "refs/heads/")}
	if depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", depth))
	}
	args = append(args, parallelFileURL(source), destination)
	command := exec.Command("git", args...)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("clone worker: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if depth > 0 {
		shallow, err := gitCommand(destination, "rev-parse", "--is-shallow-repository")
		if err != nil || strings.TrimSpace(shallow) != "true" {
			return fmt.Errorf("worker clone is not shallow")
		}
	}
	head, err := gitCommand(destination, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(head) != expectedHead {
		return fmt.Errorf("worker clone does not match frozen base HEAD")
	}
	return nil
}

func parallelFileURL(path string) string {
	return (&url.URL{Scheme: "file", Path: path}).String()
}

type lockedLoopWriter struct {
	mu sync.Mutex
	w  io.Writer
}

type parallelWorkspaceOwner struct {
	path string
	root *os.Root
}

func (w *lockedLoopWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(data)
}

func (s *Service) loopParallelExecute(ctx context.Context, invocation LoopInvocation, snapshot LoopPreflightSnapshot) (LoopDiagnostic, error) {
	if invocation.Delivery != "local" {
		return LoopDiagnostic{}, WithMachineErrorCode(MachineCodeUnsupported, fmt.Errorf("parallel review delivery is not available"))
	}
	if err := s.authorizeLoopExecutionPrompt(snapshot); err != nil {
		return LoopDiagnostic{}, err
	}
	selection, err := s.loopFrozenSelection(snapshot)
	if err != nil {
		return LoopDiagnostic{}, err
	}
	plan, err := s.loopParallelPlan(snapshot, selection.Tasks)
	if err != nil {
		return LoopDiagnostic{}, err
	}
	ownership, err := s.beginLoopOwnership(ctx)
	if err != nil {
		return LoopDiagnostic{}, err
	}
	defer ownership.close()
	diagnostic := loopDiagnosticBase(snapshot, ownership)
	diagnostic.LastIteration = nil
	batch := ParallelBatch{Plan: plan, Workers: make([]ParallelWorker, len(plan.Frontier)),
		Integration: ParallelIntegration{BaseHead: plan.BaseHead, AcceptedTasks: []string{}},
		Delivery:    ParallelDelivery{Mode: "local", TargetRef: plan.BaseRef, HeadBefore: plan.BaseHead, PublishedTasks: []string{}, PendingTasks: []string{}, Remote: "not_checked"}}
	diagnostic.Parallel = &batch
	if len(plan.Frontier) == 0 {
		diagnostic.Outcome = "no_work"
		diagnostic.NextAction = "No eligible allowed task remains; inspect task loop policy before another invocation."
		return diagnostic, nil
	}

	root, err := os.MkdirTemp(plan.Workspace.Root, "taskrail-parallel-")
	if err != nil {
		return LoopDiagnostic{}, fmt.Errorf("create parallel workspace: %w", err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		_ = os.RemoveAll(root)
		return LoopDiagnostic{}, fmt.Errorf("secure parallel workspace: %w", err)
	}
	ownerRoot, err := os.OpenRoot(root)
	if err != nil {
		_ = os.RemoveAll(root)
		return LoopDiagnostic{}, fmt.Errorf("open parallel workspace root: %w", err)
	}
	owner := parallelWorkspaceOwner{path: root, root: ownerRoot}
	if err := owner.root.WriteFile(".taskrail-parallel-owner", []byte(root), 0o600); err != nil {
		_ = owner.root.Close()
		_ = os.RemoveAll(root)
		return LoopDiagnostic{}, fmt.Errorf("bind parallel workspace identity: %w", err)
	}
	for i, item := range plan.Frontier {
		workspace := filepath.Join(root, fmt.Sprintf("worker-%02d", item.Rank))
		value := workspace
		batch.Workers[i] = ParallelWorker{Rank: item.Rank, Task: item.Task, Outcome: "child_failed", Workspace: &value, Violations: []MachineViolation{}}
	}
	stdout, stderr := &lockedLoopWriter{w: os.Stdout}, &lockedLoopWriter{w: os.Stderr}
	var wait sync.WaitGroup
	for i := range batch.Workers {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			batch.Workers[i] = s.runParallelWorker(ctx, invocation, plan, batch.Workers[i], stdout, stderr)
		}(i)
	}
	wait.Wait()

	integrationRoot := filepath.Join(root, "integration")
	if hasParallelCandidate(batch.Workers) {
		value := integrationRoot
		batch.Integration.Workspace = &value
		s.integrateParallelWorkers(ctx, invocation, plan, integrationRoot, &batch, stdout, stderr)
	}
	s.finishParallelDelivery(snapshot, &diagnostic, &batch)
	if cleanup := s.cleanupParallelWorkspaces(owner, invocation.KeepWorkspaces, &batch); len(cleanup) != 0 {
		diagnostic.MutationViolations = append(diagnostic.MutationViolations, cleanup...)
		diagnostic.Outcome = "batch_partial"
		diagnostic.NextAction = "Inspect retained workspaces because parallel cleanup failed."
	}
	diagnostic.Parallel = &batch
	return diagnostic, nil
}

func (s *Service) runParallelWorker(ctx context.Context, invocation LoopInvocation, plan ParallelPlan, worker ParallelWorker, stdout, stderr io.Writer) ParallelWorker {
	if worker.Workspace == nil {
		worker.Violations = append(worker.Violations, parallelViolation("workspace_missing", "worker workspace is missing"))
		return worker
	}
	depth := 0
	if plan.Workspace.CloneDepth != nil {
		depth = *plan.Workspace.CloneDepth
	}
	if err := parallelClone(s.paths.WorktreeRoot, *worker.Workspace, plan.BaseRef, plan.BaseHead, depth); err != nil {
		worker.Violations = append(worker.Violations, parallelViolation("clone_failed", err.Error()))
		return worker
	}
	service, err := NewService(*worker.Workspace)
	if err != nil {
		worker.Violations = append(worker.Violations, parallelViolation("worker_preflight_failed", err.Error()))
		return worker
	}
	cloneInvocation := invocation
	cloneInvocation.Parallel = 1
	cloneInvocation.WorkspaceRootSet, cloneInvocation.CloneDepthSet, cloneInvocation.KeepWorkspacesSet = false, false, false
	cloneInvocation.DeliverySet, cloneInvocation.ReviewAdapterSet = false, false
	cloneInvocation.Delivery, cloneInvocation.ReviewAdapter = "local", ""
	snapshot, err := service.LoopPreflight(cloneInvocation)
	if err != nil {
		worker.Violations = append(worker.Violations, parallelViolation("worker_preflight_failed", err.Error()))
		return worker
	}
	ownership, err := service.beginLoopOwnership(ctx)
	if err != nil {
		worker.Violations = append(worker.Violations, parallelViolation("worker_lock_failed", err.Error()))
		return worker
	}
	iteration, _, mutation, process := service.runLoopIterationTo(ctx, snapshot, ownership, worker.Task, stdout, stderr)
	if err := ownership.close(); err != nil {
		mutation = append(mutation, parallelViolation("worker_cleanup_failed", err.Error()))
	}
	worker.Outcome, worker.Child = iteration.Outcome, iteration.Child
	worker.Violations = append(worker.Violations, mutation...)
	worker.Violations = append(worker.Violations, process...)
	if worker.Outcome != "completed_pass" || len(worker.Violations) != 0 {
		return worker
	}
	head, err := gitCommand(*worker.Workspace, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(head) == plan.BaseHead {
		worker.Outcome = "invalid_postflight"
		worker.Violations = append(worker.Violations, parallelViolation("candidate_missing", "worker did not produce a direct candidate commit"))
		return worker
	}
	head = strings.TrimSpace(head)
	worker.CandidateHead, worker.CandidateCommit = &head, &head
	return worker
}

func hasParallelCandidate(workers []ParallelWorker) bool {
	for _, worker := range workers {
		if worker.Outcome == "completed_pass" && worker.CandidateCommit != nil {
			return true
		}
	}
	return false
}

func (s *Service) integrateParallelWorkers(ctx context.Context, invocation LoopInvocation, plan ParallelPlan, root string, batch *ParallelBatch, stdout, stderr io.Writer) {
	depth := 0
	if plan.Workspace.CloneDepth != nil {
		depth = *plan.Workspace.CloneDepth
	}
	if err := parallelClone(s.paths.WorktreeRoot, root, plan.BaseRef, plan.BaseHead, depth); err != nil {
		batch.Integration.AggregatePass = false
		return
	}
	for i := range batch.Workers {
		worker := &batch.Workers[i]
		if worker.Outcome != "completed_pass" || worker.CandidateCommit == nil || worker.Workspace == nil {
			continue
		}
		child, err := integrateParallelCandidate(ctx, invocation, root, *worker.Workspace, *worker.CandidateCommit, stdout, stderr)
		if child != nil {
			batch.Integration.Child = child
		}
		if err != nil {
			worker.Outcome = "integration_failed"
			worker.Violations = append(worker.Violations, parallelViolation("integration_failed", err.Error()))
			continue
		}
		head, err := gitCommand(root, "rev-parse", "HEAD")
		if err != nil {
			worker.Outcome = "integration_failed"
			worker.Violations = append(worker.Violations, parallelViolation("integration_failed", err.Error()))
			continue
		}
		head = strings.TrimSpace(head)
		worker.Integrated, worker.IntegratedCommit = true, &head
		batch.Integration.AcceptedTasks = append(batch.Integration.AcceptedTasks, worker.Task.TaskID)
	}
	batch.Integration.AggregatePass = runParallelAggregateChild(ctx, invocation, root, batch, stdout, stderr)
	if batch.Integration.AggregatePass {
		if head, err := gitCommand(root, "rev-parse", "HEAD"); err == nil {
			value := strings.TrimSpace(head)
			batch.Integration.Head = &value
		}
	}
}

func integrateParallelCandidate(ctx context.Context, invocation LoopInvocation, root, worker, commit string, stdout, stderr io.Writer) (*LoopIterationChild, error) {
	if output, err := exec.Command("git", "-C", root, "fetch", "--no-tags", parallelFileURL(worker), commit).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("fetch candidate: %w: %s", err, strings.TrimSpace(string(output)))
	}
	fetched, err := gitCommand(root, "rev-parse", "FETCH_HEAD")
	if err != nil || strings.TrimSpace(fetched) != commit {
		return nil, fmt.Errorf("fetched candidate does not match frozen candidate commit")
	}
	if output, err := exec.Command("git", "-C", root, "cherry-pick", "--no-commit", "FETCH_HEAD").CombinedOutput(); err != nil {
		if repairErr := repairParallelStateConflict(root); repairErr != nil {
			if prepareErr := prepareParallelSemanticConflict(root); prepareErr != nil {
				_ = exec.Command("git", "-C", root, "cherry-pick", "--abort").Run()
				return nil, fmt.Errorf("prepare semantic replay conflict: %w", prepareErr)
			}
			child := runParallelIntegrationChild(ctx, invocation, root, commit, stdout, stderr)
			if child.ExitCode == nil || *child.ExitCode != 0 || child.Signal != nil || child.TimedOut || parallelUnmerged(root) {
				_ = exec.Command("git", "-C", root, "cherry-pick", "--abort").Run()
				return &child, fmt.Errorf("replay candidate %s: %w: %s", commit, err, strings.TrimSpace(string(output)))
			}
			_ = exec.Command("git", "-C", root, "cherry-pick", "--quit").Run()
		}
	}
	service, err := NewService(root)
	if err != nil {
		parallelResetIntegration(root)
		return nil, fmt.Errorf("open integration repository: %w", err)
	}
	if _, err := service.Repair(RepairInput{Apply: true}); err != nil {
		parallelResetIntegration(root)
		return nil, fmt.Errorf("reproject integration state: %w", err)
	}
	if output, err := exec.Command("git", "-C", root, "commit", "-C", "FETCH_HEAD").CombinedOutput(); err != nil {
		parallelResetIntegration(root)
		return nil, fmt.Errorf("commit integrated candidate: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil, nil
}

func runParallelIntegrationChild(ctx context.Context, invocation LoopInvocation, root, commit string, stdout, stderr io.Writer) LoopIterationChild {
	prompt := []byte("Resolve exactly one parallel integration conflict for candidate " + commit + ". Preserve task acceptance and detecting tests. Do not edit generated state or task policy, do not integrate another candidate, and leave the resolution staged without committing.\n")
	execution, err := launchLoopChild(loopChildLaunch{Command: invocation.Child, Context: ctx, Timeout: invocation.Timeout, Prompt: prompt, RepositoryRoot: root, Stdout: stdout, Stderr: stderr})
	if err != nil {
		return LoopIterationChild{Signal: stringPtr(err.Error())}
	}
	return loopIterationChild(execution)
}

func runParallelAggregateChild(ctx context.Context, invocation LoopInvocation, root string, batch *ParallelBatch, stdout, stderr io.Writer) bool {
	validation, err := NewService(root)
	if err != nil {
		return false
	}
	result, err := validation.Validate()
	if err != nil || !result.Valid {
		return false
	}
	before, err := loopGitSnapshot(root, validation.paths.GitDir)
	if err != nil {
		return false
	}
	prompt := []byte("Run the repository's full aggregate validation on this completed parallel integration head. Do not modify files or Git state.\n")
	execution, err := launchLoopChild(loopChildLaunch{Command: invocation.Child, Context: ctx, Timeout: invocation.Timeout, Prompt: prompt, RepositoryRoot: root, Stdout: stdout, Stderr: stderr})
	if err != nil {
		return false
	}
	child := loopIterationChild(execution)
	batch.Integration.Child = &child
	after, err := loopGitSnapshot(root, validation.paths.GitDir)
	return err == nil && reflect.DeepEqual(before, after) && child.ExitCode != nil && *child.ExitCode == 0 && child.Signal == nil && !child.TimedOut && !execution.Failed()
}

func parallelUnmerged(root string) bool {
	output, err := exec.Command("git", "-C", root, "diff", "--name-only", "--diff-filter=U").Output()
	return err != nil || strings.TrimSpace(string(output)) != ""
}

func prepareParallelSemanticConflict(root string) error {
	service, err := NewService(root)
	if err != nil {
		return err
	}
	state := service.reportedStatePath()
	output, err := exec.Command("git", "-C", root, "diff", "--name-only", "--diff-filter=U").Output()
	if err != nil {
		return err
	}
	for _, path := range strings.Fields(string(output)) {
		if path != state {
			continue
		}
		if output, err := exec.Command("git", "-C", root, "checkout", "--ours", "--", state).CombinedOutput(); err != nil {
			return fmt.Errorf("restore aggregate state: %w: %s", err, strings.TrimSpace(string(output)))
		}
		if output, err := exec.Command("git", "-C", root, "add", "--", state).CombinedOutput(); err != nil {
			return fmt.Errorf("stage aggregate state: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}

func parallelResetIntegration(root string) {
	_ = exec.Command("git", "-C", root, "cherry-pick", "--abort").Run()
	_ = exec.Command("git", "-C", root, "reset", "--hard", "HEAD").Run()
}

// repairParallelStateConflict accepts only the generated aggregate projection;
// task and product conflicts remain semantic candidate failures.
func repairParallelStateConflict(root string) error {
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
	if output, err := exec.Command("git", "-C", root, "cherry-pick", "--quit").CombinedOutput(); err != nil {
		return fmt.Errorf("clear replay state: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (s *Service) finishParallelDelivery(snapshot LoopPreflightSnapshot, diagnostic *LoopDiagnostic, batch *ParallelBatch) {
	for _, worker := range batch.Workers {
		if !worker.Integrated {
			batch.Delivery.PendingTasks = append(batch.Delivery.PendingTasks, worker.Task.TaskID)
		}
	}
	if !batch.Integration.AggregatePass || batch.Integration.Head == nil {
		parallelOutcome(diagnostic, batch)
		return
	}
	inputs, inputErr := loopInputBytes(s.paths)
	git, gitErr := loopGitSnapshot(s.paths.WorktreeRoot, s.paths.GitDir)
	if inputErr != nil || gitErr != nil || !reflect.DeepEqual(inputs, snapshot.Inputs()) || !reflect.DeepEqual(git, snapshot.Git()) {
		diagnostic.MutationViolations = append(diagnostic.MutationViolations, parallelViolation("source_drift", "source repository changed before parallel publication"))
		parallelOutcome(diagnostic, batch)
		return
	}
	if batch.Integration.Workspace == nil {
		diagnostic.MutationViolations = append(diagnostic.MutationViolations, parallelViolation("publish_failed", "integration workspace is unavailable"))
		parallelOutcome(diagnostic, batch)
		return
	}
	if output, err := exec.Command("git", "-C", s.paths.WorktreeRoot, "fetch", "--no-tags", "--no-write-fetch-head", parallelFileURL(*batch.Integration.Workspace), *batch.Integration.Head).CombinedOutput(); err != nil {
		diagnostic.MutationViolations = append(diagnostic.MutationViolations, parallelViolation("publish_failed", strings.TrimSpace(string(output))))
		parallelOutcome(diagnostic, batch)
		return
	}
	if output, err := exec.Command("git", "-C", s.paths.WorktreeRoot, "merge", "--ff-only", *batch.Integration.Head).CombinedOutput(); err != nil {
		diagnostic.MutationViolations = append(diagnostic.MutationViolations, parallelViolation("publish_failed", strings.TrimSpace(string(output))))
		parallelOutcome(diagnostic, batch)
		return
	}
	head := *batch.Integration.Head
	batch.Delivery.HeadAfter = &head
	for _, worker := range batch.Workers {
		if worker.Integrated {
			batch.Delivery.PublishedTasks = append(batch.Delivery.PublishedTasks, worker.Task.TaskID)
		}
	}
	diagnostic.Git = LoopGitDiagnostic{Ref: snapshot.Git().Ref, HeadBefore: snapshot.Git().Head, HeadAfter: head, Clean: true, Descendant: true, Commits: parallelIntegratedCommits(batch.Workers)}
	parallelOutcome(diagnostic, batch)
}

func parallelIntegratedCommits(workers []ParallelWorker) []string {
	commits := make([]string, 0, len(workers))
	for _, worker := range workers {
		if worker.IntegratedCommit != nil {
			commits = append(commits, *worker.IntegratedCommit)
		}
	}
	return commits
}

func parallelOutcome(diagnostic *LoopDiagnostic, batch *ParallelBatch) {
	if len(batch.Delivery.PublishedTasks) == len(batch.Workers) && len(batch.Workers) != 0 && batch.Delivery.HeadAfter != nil {
		diagnostic.Outcome = "batch_pass"
		diagnostic.NextAction = "All selected parallel tasks were integrated and published."
		return
	}
	if len(batch.Delivery.PublishedTasks) != 0 {
		diagnostic.Outcome = "batch_partial"
		diagnostic.NextAction = "Inspect retained failed worker or integration workspaces before a new invocation."
		return
	}
	diagnostic.Outcome = "batch_failed"
	diagnostic.NextAction = "Inspect failed worker and integration diagnostics before a new invocation."
}

func (s *Service) cleanupParallelWorkspaces(owner parallelWorkspaceOwner, retention string, batch *ParallelBatch) []MachineViolation {
	violations := []MachineViolation{}
	for i := range batch.Workers {
		worker := &batch.Workers[i]
		if worker.Workspace == nil || (retention == "failure" && !worker.Integrated) || retention == "always" {
			continue
		}
		if err := removeParallelWorkspace(owner, *worker.Workspace); err != nil {
			violation := parallelViolation("cleanup_failed", err.Error())
			worker.Violations = append(worker.Violations, violation)
			violations = append(violations, violation)
			continue
		}
		worker.Workspace = nil
	}
	if batch.Integration.Workspace != nil && retention != "always" && (retention == "never" || batch.Integration.AggregatePass) {
		if err := removeParallelWorkspace(owner, *batch.Integration.Workspace); err == nil {
			batch.Integration.Workspace = nil
		} else {
			violations = append(violations, parallelViolation("cleanup_failed", err.Error()))
		}
	}
	if retention == "never" {
		if err := removeParallelRoot(owner); err != nil {
			violations = append(violations, parallelViolation("cleanup_failed", err.Error()))
		}
		if err := owner.root.Close(); err != nil {
			violations = append(violations, parallelViolation("cleanup_failed", err.Error()))
		}
		return violations
	}
	if batch.Integration.Workspace == nil {
		retained := false
		for _, worker := range batch.Workers {
			if worker.Workspace != nil {
				retained = true
				break
			}
		}
		if !retained {
			if err := removeParallelRoot(owner); err != nil {
				violations = append(violations, parallelViolation("cleanup_failed", err.Error()))
			}
		}
	}
	if err := owner.root.Close(); err != nil {
		violations = append(violations, parallelViolation("cleanup_failed", err.Error()))
	}
	return violations
}

func removeParallelWorkspace(owner parallelWorkspaceOwner, workspace string) error {
	if err := verifyParallelWorkspaceRoot(owner); err != nil {
		return err
	}
	rel, err := filepath.Rel(owner.path, workspace)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("refuse cleanup outside owned parallel workspace")
	}
	info, err := owner.root.Lstat(rel)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refuse cleanup of changed parallel workspace")
	}
	return owner.root.RemoveAll(rel)
}

func removeParallelRoot(owner parallelWorkspaceOwner) error {
	if err := verifyParallelWorkspaceRoot(owner); err != nil {
		return err
	}
	if err := owner.root.Remove(".taskrail-parallel-owner"); err != nil {
		return err
	}
	return os.Remove(owner.path)
}

func verifyParallelWorkspaceRoot(owner parallelWorkspaceOwner) error {
	markerInfo, err := owner.root.Lstat(".taskrail-parallel-owner")
	if err != nil || !markerInfo.Mode().IsRegular() || markerInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("parallel workspace ownership marker changed")
	}
	contents, err := owner.root.ReadFile(".taskrail-parallel-owner")
	if err != nil || string(contents) != owner.path {
		return fmt.Errorf("parallel workspace ownership marker does not match")
	}
	return nil
}

func parallelViolation(code, message string) MachineViolation {
	return MachineViolation{Code: code, Message: message}
}
