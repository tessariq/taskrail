package taskrail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
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
	BaseHead      string                     `json:"base_head"`
	Head          *string                    `json:"head"`
	AcceptedTasks []string                   `json:"accepted_tasks"`
	AggregatePass bool                       `json:"aggregate_pass"`
	Children      []ParallelIntegrationChild `json:"children"`
	Workspace     *string                    `json:"workspace"`
}

// ParallelIntegrationChild binds one coordinator-launched integration child to
// the candidate or final head it was permitted to assess.
type ParallelIntegrationChild struct {
	Role                 string              `json:"role"`
	TaskID               *string             `json:"task_id"`
	BoundHead            string              `json:"bound_head"`
	CandidateHead        *string             `json:"candidate_head"`
	WorkerEvidenceSHA256 *string             `json:"worker_evidence_sha256"`
	Prompt               LoopIterationPrompt `json:"prompt"`
	Child                LoopIterationChild  `json:"child"`
	Outcome              string              `json:"outcome"`
	AffectedChecks       []string            `json:"affected_checks"`
}

type parallelIntegrationBinding struct {
	role                 string
	taskID               *string
	boundHead            string
	boundRef             string
	conflictPaths        string
	candidateHead        *string
	workerEvidenceSHA256 *string
	affectedChecks       []string
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

var testHookBeforeParallelAggregate func()

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
	if invocation.Delivery != "local" && invocation.Delivery != "review" {
		return LoopDiagnostic{}, invalidArgumentsf("unsupported parallel delivery mode")
	}
	if err := s.authorizeLoopExecutionPrompts(snapshot, "task-implementation", "loop-integration"); err != nil {
		return LoopDiagnostic{}, err
	}
	_, implementationPromptSource, _, err := s.loopPromptTemplate(snapshot, "task-implementation")
	if err != nil {
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
		Integration: ParallelIntegration{BaseHead: plan.BaseHead, AcceptedTasks: []string{}, Children: []ParallelIntegrationChild{}},
		Delivery:    ParallelDelivery{Mode: plan.Delivery, Adapter: plan.ReviewAdapter, TargetRef: plan.BaseRef, HeadBefore: plan.BaseHead, PublishedTasks: []string{}, PendingTasks: []string{}, Remote: "not_checked"}}
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
	root, err = canonicalParallelRoot(root)
	if err != nil {
		return LoopDiagnostic{}, fmt.Errorf("resolve parallel workspace: %w", err)
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
			batch.Workers[i] = s.runParallelWorker(ctx, invocation, plan, implementationPromptSource, batch.Workers[i], stdout, stderr)
		}(i)
	}
	wait.Wait()

	if invocation.Delivery == "review" {
		diagnostic.MutationViolations = append(diagnostic.MutationViolations, s.deliverParallelReviews(ctx, invocation, snapshot, plan, root, &batch, stdout, stderr)...)
	} else {
		integrationRoot := filepath.Join(root, "integration")
		if hasParallelCandidate(batch.Workers) {
			value := integrationRoot
			batch.Integration.Workspace = &value
			diagnostic.MutationViolations = append(diagnostic.MutationViolations, s.integrateParallelWorkers(ctx, invocation, snapshot, plan, integrationRoot, &batch, stdout, stderr)...)
		}
		s.finishParallelDelivery(snapshot, &diagnostic, &batch)
	}
	if invocation.Delivery == "review" {
		if inputs, inputErr := loopInputBytes(s.paths); inputErr != nil || !reflect.DeepEqual(inputs, snapshot.Inputs()) {
			diagnostic.MutationViolations = append(diagnostic.MutationViolations, parallelViolation("source_drift", "source repository inputs changed during review delivery"))
		} else if git, gitErr := loopGitSnapshot(s.paths.WorktreeRoot, s.paths.GitDir); gitErr != nil || !reflect.DeepEqual(git, snapshot.Git()) {
			diagnostic.MutationViolations = append(diagnostic.MutationViolations, parallelViolation("source_drift", "source repository Git state changed during review delivery"))
			if gitErr == nil {
				diagnostic.Git = LoopGitDiagnostic{Ref: git.Ref, HeadBefore: snapshot.Git().Head, HeadAfter: git.Head, Clean: git.Clean, Descendant: false, Commits: []string{}}
			}
		} else if configs, configErr := loopGitConfigSnapshot(s.paths.GitDir, s.paths.GitCommonDir); configErr != nil || !reflect.DeepEqual(configs, snapshot.GitConfig()) {
			diagnostic.MutationViolations = append(diagnostic.MutationViolations, parallelViolation("source_drift", "source repository Git configuration changed during review delivery"))
		}
		parallelOutcome(&diagnostic, &batch)
		if len(diagnostic.MutationViolations) != 0 {
			diagnostic.Outcome = "batch_partial"
			diagnostic.NextAction = "Inspect source drift and adapter diagnostics before another invocation."
		}
	}
	if cleanup := s.cleanupParallelWorkspaces(owner, invocation.KeepWorkspaces, &batch); len(cleanup) != 0 {
		diagnostic.MutationViolations = append(diagnostic.MutationViolations, cleanup...)
		diagnostic.Outcome = "batch_partial"
		diagnostic.NextAction = "Inspect retained workspaces because parallel cleanup failed."
	}
	diagnostic.Parallel = &batch
	return diagnostic, nil
}

func canonicalParallelRoot(root string) (string, error) {
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		_ = os.RemoveAll(root)
		return "", err
	}
	return canonical, nil
}

func (s *Service) runParallelWorker(ctx context.Context, invocation LoopInvocation, plan ParallelPlan, implementationPromptSource string, worker ParallelWorker, stdout, stderr io.Writer) ParallelWorker {
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
	if implementationPromptSource == "builtin" {
		cloneInvocation.AllowPromptOverrideSHA256 = ""
	}
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

func (s *Service) integrateParallelWorkers(ctx context.Context, invocation LoopInvocation, snapshot LoopPreflightSnapshot, plan ParallelPlan, root string, batch *ParallelBatch, stdout, stderr io.Writer) []MachineViolation {
	violations := []MachineViolation{}
	depth := 0
	if plan.Workspace.CloneDepth != nil {
		depth = *plan.Workspace.CloneDepth
	}
	if err := parallelClone(s.paths.WorktreeRoot, root, plan.BaseRef, plan.BaseHead, depth); err != nil {
		batch.Integration.AggregatePass = false
		return violations
	}
	for i := range batch.Workers {
		worker := &batch.Workers[i]
		if worker.Outcome != "completed_pass" || worker.CandidateCommit == nil || worker.Workspace == nil {
			continue
		}
		child, err := s.integrateParallelCandidate(ctx, invocation, snapshot, root, worker, stdout, stderr)
		if child != nil {
			batch.Integration.Children = append(batch.Integration.Children, *child)
		}
		if err != nil {
			if errors.Is(err, errLoopGitConfigChanged) {
				worker.Outcome = "invalid_postflight"
				worker.Violations = append(worker.Violations, parallelViolation("git_config_changed", "parallel integration child changed Git configuration"))
				violations = append(violations, parallelViolation("git_config_changed", "parallel integration child changed Git configuration"))
			} else {
				worker.Outcome = "integration_failed"
				worker.Violations = append(worker.Violations, parallelViolation("integration_failed", err.Error()))
			}
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
	if len(violations) != 0 {
		batch.Integration.AggregatePass = false
		return violations
	}
	aggregatePass, aggregateViolations := s.runParallelAggregateChild(ctx, invocation, snapshot, root, batch, stdout, stderr)
	batch.Integration.AggregatePass = aggregatePass
	violations = append(violations, aggregateViolations...)
	if batch.Integration.AggregatePass {
		if head, err := gitCommand(root, "rev-parse", "HEAD"); err == nil {
			value := strings.TrimSpace(head)
			batch.Integration.Head = &value
		}
	}
	return violations
}

func (s *Service) integrateParallelCandidate(ctx context.Context, invocation LoopInvocation, snapshot LoopPreflightSnapshot, root string, worker *ParallelWorker, stdout, stderr io.Writer) (*ParallelIntegrationChild, error) {
	if worker.Workspace == nil || worker.CandidateCommit == nil {
		return nil, fmt.Errorf("parallel candidate binding is incomplete")
	}
	commit := *worker.CandidateCommit
	if output, err := exec.Command("git", "-C", root, "fetch", "--no-tags", parallelFileURL(*worker.Workspace), commit).CombinedOutput(); err != nil {
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
			binding, bindingErr := parallelConflictBinding(root, worker)
			if bindingErr != nil {
				return nil, bindingErr
			}
			child, childErr := s.runParallelIntegrationChild(ctx, invocation, snapshot, root, worker, binding, stdout, stderr)
			if childErr != nil {
				_ = exec.Command("git", "-C", root, "cherry-pick", "--abort").Run()
				return &child, childErr
			}
			if child.Outcome != "pass" || parallelUnmerged(root) {
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
	commitEnvironment, err := parallelCandidateCommitEnvironment(root)
	if err != nil {
		parallelResetIntegration(root)
		return nil, err
	}
	command := exec.Command("git", "-C", root, "commit", "-C", "FETCH_HEAD")
	command.Env = append(os.Environ(), commitEnvironment...)
	if output, err := command.CombinedOutput(); err != nil {
		parallelResetIntegration(root)
		return nil, fmt.Errorf("commit integrated candidate: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil, nil
}

func parallelCandidateCommitEnvironment(root string) ([]string, error) {
	return parallelCommitEnvironment(root, "FETCH_HEAD")
}

func parallelCommitEnvironment(root, revision string) ([]string, error) {
	output, err := exec.Command("git", "-C", root, "show", "-s", "--format=%cn%x00%ce%x00%cI", revision).Output()
	if err != nil {
		return nil, fmt.Errorf("read candidate commit metadata: %w", err)
	}
	fields := strings.Split(strings.TrimRight(string(output), "\r\n"), "\x00")
	if len(fields) != 3 || fields[0] == "" || fields[1] == "" || fields[2] == "" {
		return nil, fmt.Errorf("candidate commit has invalid committer metadata")
	}
	return []string{
		"GIT_COMMITTER_NAME=" + fields[0],
		"GIT_COMMITTER_EMAIL=" + fields[1],
		"GIT_COMMITTER_DATE=" + fields[2],
	}, nil
}

var errLoopGitConfigChanged = errors.New("Git configuration changed")

func (s *Service) runParallelIntegrationChild(ctx context.Context, invocation LoopInvocation, snapshot LoopPreflightSnapshot, root string, worker *ParallelWorker, binding parallelIntegrationBinding, stdout, stderr io.Writer) (ParallelIntegrationChild, error) {
	prompt, err := s.loopIntegrationPrompt(snapshot, binding)
	child := ParallelIntegrationChild{Role: binding.role, TaskID: binding.taskID, BoundHead: binding.boundHead,
		CandidateHead: binding.candidateHead, WorkerEvidenceSHA256: binding.workerEvidenceSHA256,
		Prompt: loopIterationPrompt(prompt), Outcome: "fail", AffectedChecks: append([]string{}, binding.affectedChecks...)}
	if err != nil {
		child.Child.Signal = stringPtr(err.Error())
		return child, err
	}
	if err := validateParallelIntegrationBinding(root, worker, binding); err != nil {
		child.Child.Signal = stringPtr(err.Error())
		return child, err
	}
	service, err := NewService(root)
	if err != nil {
		child.Child.Signal = stringPtr(err.Error())
		return child, err
	}
	configs, err := loopGitConfigSnapshot(service.paths.GitDir, service.paths.GitCommonDir)
	if err != nil {
		child.Child.Signal = stringPtr(err.Error())
		return child, err
	}
	execution, err := launchLoopChild(loopChildLaunch{Command: invocation.Child, Context: ctx, Timeout: invocation.Timeout, Prompt: []byte(prompt.Content), RepositoryRoot: root, Stdout: stdout, Stderr: stderr})
	if err != nil {
		child.Child = LoopIterationChild{Signal: stringPtr(err.Error())}
		return child, nil
	}
	child.Child = loopIterationChild(execution)
	postConfigs, configErr := loopGitConfigSnapshot(service.paths.GitDir, service.paths.GitCommonDir)
	if configErr != nil || !reflect.DeepEqual(configs, postConfigs) {
		return child, errLoopGitConfigChanged
	}
	if !parallelChildPassed(execution) {
		return child, nil
	}
	child.Outcome = "pass"
	return child, nil
}

func (s *Service) runParallelAggregateChild(ctx context.Context, invocation LoopInvocation, snapshot LoopPreflightSnapshot, root string, batch *ParallelBatch, stdout, stderr io.Writer) (bool, []MachineViolation) {
	if testHookBeforeParallelAggregate != nil {
		testHookBeforeParallelAggregate()
	}
	head, headErr := gitCommand(root, "rev-parse", "HEAD")
	if headErr != nil {
		return false, nil
	}
	binding := parallelIntegrationBinding{role: "aggregate_gate", boundHead: strings.TrimSpace(head), affectedChecks: []string{"aggregate_validation"}}
	validation, err := NewService(root)
	if err != nil {
		return false, nil
	}
	result, err := validation.Validate()
	if err != nil || !result.Valid {
		return false, nil
	}
	before, err := loopGitSnapshot(root, validation.paths.GitDir)
	if err != nil {
		return false, nil
	}
	configs, err := loopGitConfigSnapshot(validation.paths.GitDir, validation.paths.GitCommonDir)
	if err != nil {
		return false, nil
	}
	child, childErr := s.runParallelIntegrationChild(ctx, invocation, snapshot, root, nil, binding, stdout, stderr)
	batch.Integration.Children = append(batch.Integration.Children, child)
	if childErr != nil {
		if errors.Is(childErr, errLoopGitConfigChanged) {
			return false, []MachineViolation{parallelViolation("git_config_changed", "parallel aggregate child changed Git configuration")}
		}
		return false, nil
	}
	after, err := loopGitSnapshot(root, validation.paths.GitDir)
	postConfigs, configErr := loopGitConfigSnapshot(validation.paths.GitDir, validation.paths.GitCommonDir)
	if configErr != nil || !reflect.DeepEqual(configs, postConfigs) {
		return false, []MachineViolation{parallelViolation("git_config_changed", "parallel aggregate child changed Git configuration")}
	}
	return err == nil && reflect.DeepEqual(before, after) && child.Outcome == "pass", nil
}

func (s *Service) loopIntegrationPrompt(snapshot LoopPreflightSnapshot, binding parallelIntegrationBinding) (LoopPromptExecution, error) {
	template, source, replacementPath, err := s.loopPromptTemplate(snapshot, "loop-integration")
	if err != nil {
		return LoopPromptExecution{}, err
	}
	values, err := s.loopIntegrationValues(snapshot, binding)
	if err != nil {
		return LoopPromptExecution{}, err
	}
	declared := loopIntegrationPromptTokens(binding.role)
	values = loopIntegrationPromptValues(values, declared)
	rendered, err := RenderPrompt(PromptRenderInput{Template: template, DeclaredTokens: declared, Values: values})
	if err != nil {
		return LoopPromptExecution{}, WithMachineErrorCode(MachineCodePromptInvalid, err)
	}
	return LoopPromptExecution{ID: "loop-integration", Source: source, Path: replacementPath,
		TemplateSHA256: rendered.TemplateSHA256, RenderedSHA256: rendered.SHA256, OverrideAuthorized: true, Content: rendered.Content}, nil
}

func loopIntegrationPromptTokens(role string) []string {
	if role == "aggregate_gate" {
		return []string{"INTEGRATION_ROLE", "BASE_HEAD", "CURRENT_HEAD", "STORAGE_MODE"}
	}
	return promptTokenDeclarations["loop-integration"]
}

func loopIntegrationPromptValues(values map[string]string, declared []string) map[string]string {
	filtered := make(map[string]string, len(declared))
	for _, name := range declared {
		filtered[name] = values[name]
	}
	return filtered
}

func (s *Service) loopIntegrationValues(snapshot LoopPreflightSnapshot, binding parallelIntegrationBinding) (map[string]string, error) {
	values := map[string]string{
		"INTEGRATION_ROLE": binding.role, "TASK_ID": valueOrEmpty(binding.taskID), "TASK_PATH": "", "SPEC_VERSION": "", "SPEC_PATH": "",
		"BASE_HEAD": snapshot.Git().Head, "CURRENT_HEAD": binding.boundHead, "CANDIDATE_HEAD": valueOrEmpty(binding.candidateHead),
		"CONFLICT_PATHS": binding.conflictPaths, "WORKER_EVIDENCE_PATH": "", "STORAGE_MODE": snapshot.Storage().Mode,
	}
	if binding.taskID == nil {
		return values, nil
	}
	taskPath := path.Join(s.paths.LogicalPlanningDir, "tasks", *binding.taskID+".md")
	inputPath := filepath.ToSlash(relPath(s.paths.RepoRoot, filepath.Join(s.paths.TasksDir, *binding.taskID+".md")))
	data, ok := snapshot.Inputs()[inputPath]
	if !ok {
		return nil, WithMachineErrorCode(MachineCodePromptInvalid, fmt.Errorf("frozen integration task %s is unavailable", taskPath))
	}
	frontmatter, _, err := parseFrontmatter[TaskFrontmatter](data)
	if err != nil || frontmatter.ID != *binding.taskID {
		return nil, WithMachineErrorCode(MachineCodePromptInvalid, fmt.Errorf("frozen integration task %s is invalid", taskPath))
	}
	specPath, _, err := parseSpecRef(frontmatter.SpecRef)
	if err != nil {
		return nil, WithMachineErrorCode(MachineCodePromptInvalid, fmt.Errorf("frozen integration task spec_ref is invalid: %w", err))
	}
	specLogical := filepath.ToSlash(specPath)
	specInput := filepath.ToSlash(relPath(s.paths.RepoRoot, filepath.Join(s.paths.RepoRoot, specPath)))
	if _, ok := snapshot.Inputs()[specInput]; !ok {
		return nil, WithMachineErrorCode(MachineCodePromptInvalid, fmt.Errorf("frozen integration spec %s is unavailable", specLogical))
	}
	values["TASK_PATH"] = taskPath
	values["SPEC_PATH"] = specLogical
	values["SPEC_VERSION"] = strings.TrimSuffix(path.Base(specLogical), path.Ext(specLogical))
	values["WORKER_EVIDENCE_PATH"] = path.Join(s.paths.LogicalPlanningDir, "artifacts", "verify", *binding.taskID)
	return values, nil
}

func parallelConflictBinding(root string, worker *ParallelWorker) (parallelIntegrationBinding, error) {
	if worker == nil || worker.CandidateHead == nil {
		return parallelIntegrationBinding{}, fmt.Errorf("parallel conflict is missing candidate evidence")
	}
	head, err := gitCommand(root, "rev-parse", "HEAD")
	if err != nil {
		return parallelIntegrationBinding{}, err
	}
	binding, err := parallelConflictBindingAtHead(strings.TrimSpace(head), "HEAD", worker)
	if err != nil {
		return parallelIntegrationBinding{}, err
	}
	binding.conflictPaths = parallelUnmergedPaths(root)
	return binding, nil
}

func parallelUnmergedPaths(root string) string {
	output, err := exec.Command("git", "-C", root, "diff", "--name-only", "--diff-filter=U").Output()
	if err != nil {
		return ""
	}
	return strings.Join(strings.Fields(string(output)), ",")
}

func parallelConflictBindingAtHead(head, ref string, worker *ParallelWorker) (parallelIntegrationBinding, error) {
	if worker == nil || worker.CandidateHead == nil {
		return parallelIntegrationBinding{}, fmt.Errorf("parallel conflict is missing candidate evidence")
	}
	evidence, err := parallelWorkerEvidence(*worker)
	if err != nil {
		return parallelIntegrationBinding{}, err
	}
	taskID := worker.Task.TaskID
	return parallelIntegrationBinding{role: "conflict_resolution", taskID: &taskID, boundHead: head, boundRef: ref,
		candidateHead: stringPtr(*worker.CandidateHead), workerEvidenceSHA256: &evidence, affectedChecks: []string{"candidate_replay"}}, nil
}

func parallelWorkerEvidence(worker ParallelWorker) (string, error) {
	encoded, err := json.Marshal(worker)
	if err != nil {
		return "", fmt.Errorf("encode worker evidence: %w", err)
	}
	return promptDigest(encoded), nil
}

func validateParallelIntegrationBinding(root string, worker *ParallelWorker, binding parallelIntegrationBinding) error {
	ref := binding.boundRef
	if ref == "" {
		ref = "HEAD"
	}
	head, err := gitCommand(root, "rev-parse", ref)
	if err != nil || strings.TrimSpace(head) != binding.boundHead {
		return fmt.Errorf("parallel integration head no longer matches its binding")
	}
	if binding.role == "aggregate_gate" {
		if binding.taskID != nil || binding.candidateHead != nil || binding.workerEvidenceSHA256 != nil {
			return fmt.Errorf("aggregate integration binding includes worker evidence")
		}
		return nil
	}
	if binding.role != "conflict_resolution" || worker == nil || binding.taskID == nil || binding.candidateHead == nil || binding.workerEvidenceSHA256 == nil ||
		worker.Task.TaskID != *binding.taskID || worker.CandidateHead == nil || *worker.CandidateHead != *binding.candidateHead {
		return fmt.Errorf("parallel conflict binding no longer matches worker evidence")
	}
	evidence, err := parallelWorkerEvidence(*worker)
	if err != nil || evidence != *binding.workerEvidenceSHA256 {
		return fmt.Errorf("parallel worker evidence no longer matches its binding")
	}
	return nil
}

func parallelChildPassed(execution loopChildExecution) bool {
	return !execution.Failed() && execution.ExitCode != nil && *execution.ExitCode == 0 && execution.Signal == "" && !execution.TimedOut
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
	configs, configErr := loopGitConfigSnapshot(s.paths.GitDir, s.paths.GitCommonDir)
	if inputErr != nil || gitErr != nil || configErr != nil || !reflect.DeepEqual(inputs, snapshot.Inputs()) || !reflect.DeepEqual(git, snapshot.Git()) || !reflect.DeepEqual(configs, snapshot.GitConfig()) {
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
	if len(batch.Delivery.PublishedTasks) == len(batch.Workers) && len(batch.Workers) != 0 &&
		batch.Delivery.HeadAfter != nil && batch.Integration.AggregatePass {
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
	removeRoot := retention == "never"
	if !removeRoot && batch.Integration.Workspace == nil {
		retained := false
		for _, worker := range batch.Workers {
			if worker.Workspace != nil {
				retained = true
				break
			}
		}
		removeRoot = !retained
	}
	var err error
	if removeRoot {
		err = removeParallelRoot(owner)
	} else {
		err = owner.root.Close()
	}
	if err != nil {
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
		return errors.Join(err, owner.root.Close())
	}
	if err := owner.root.Remove(".taskrail-parallel-owner"); err != nil {
		return errors.Join(err, owner.root.Close())
	}
	if err := owner.root.Close(); err != nil {
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
