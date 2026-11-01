package taskrail

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"time"
)

type LoopCheck struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type LoopCheckEvidence struct {
	Path               string             `json:"path"`
	SHA256             string             `json:"sha256"`
	ExitCode           *int               `json:"exit_code"`
	Signal             *string            `json:"signal"`
	TimedOut           bool               `json:"timed_out"`
	DurationMS         int64              `json:"duration_ms"`
	StdoutSHA256       string             `json:"stdout_sha256"`
	StderrSHA256       string             `json:"stderr_sha256"`
	MutationViolations []MachineViolation `json:"mutation_violations"`
	ProcessViolations  []MachineViolation `json:"process_violations"`
}

func resolveLoopCheck(invocation LoopInvocation) (*LoopCheck, error) {
	if invocation.Check == "" {
		return nil, nil
	}
	resolved, err := exec.LookPath(invocation.Check)
	if err != nil {
		return nil, invalidArgumentsf("resolve --check: %v", err)
	}
	absolute, err := filepath.Abs(resolved)
	if err != nil {
		return nil, invalidArgumentsf("resolve --check: %v", err)
	}
	info, err := os.Stat(absolute)
	if err != nil || !executableReviewAdapter(info.Mode(), runtime.GOOS) {
		return nil, invalidArgumentsf("--check must resolve to an executable regular file")
	}
	data, err := os.ReadFile(absolute)
	if err != nil {
		return nil, invalidArgumentsf("read --check: %v", err)
	}
	return &LoopCheck{Path: absolute, SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}, nil
}

func (s *Service) runLoopCheck(ctx context.Context, snapshot LoopPreflightSnapshot) (LoopCheckEvidence, bool) {
	check := *snapshot.check
	evidence := LoopCheckEvidence{Path: check.Path, SHA256: check.SHA256,
		MutationViolations: []MachineViolation{}, ProcessViolations: []MachineViolation{}}
	stdout, stderr := sha256.New(), sha256.New()
	started := time.Now()
	before, beforeErr := s.loopCheckSnapshot()
	current, executableErr := resolveLoopCheck(LoopInvocation{Check: check.Path})
	var execution loopChildExecution
	var launchErr error
	if beforeErr != nil {
		launchErr = fmt.Errorf("collect check preflight: %w", beforeErr)
	} else if executableErr != nil || *current != check {
		launchErr = fmt.Errorf("check executable changed or became unavailable: %v", executableErr)
	} else {
		execution, launchErr = launchLoopChild(loopChildLaunch{
			Command: []string{check.Path}, Context: ctx, Timeout: snapshot.Invocation().Timeout,
			RepositoryRoot: s.paths.WorktreeRoot, Environment: os.Environ(),
			Stdout: io.MultiWriter(stdout, os.Stderr), Stderr: io.MultiWriter(stderr, os.Stderr),
		})
	}
	child := loopIterationChild(execution)
	evidence.ExitCode, evidence.Signal, evidence.TimedOut = child.ExitCode, child.Signal, child.TimedOut
	evidence.DurationMS = time.Since(started).Milliseconds()
	evidence.StdoutSHA256, evidence.StderrSHA256 = fmt.Sprintf("%x", stdout.Sum(nil)), fmt.Sprintf("%x", stderr.Sum(nil))
	evidence.ProcessViolations = loopProcessViolations(execution)
	processErr := errors.Join(launchErr, execution.SpawnError, execution.StdinError, execution.StdoutError,
		execution.StderrError, execution.WaitError, execution.CancellationError)
	if processErr != nil {
		evidence.ProcessViolations = append(evidence.ProcessViolations, MachineViolation{Code: "check_process_failed", Message: processErr.Error()})
	}
	after, afterErr := s.loopCheckSnapshot()
	if beforeErr != nil || afterErr != nil {
		evidence.MutationViolations = append(evidence.MutationViolations, MachineViolation{Code: "check_evidence_missing", Message: fmt.Sprintf("check snapshots unavailable: before=%v after=%v", beforeErr, afterErr)})
	} else if !reflect.DeepEqual(before, after) {
		evidence.MutationViolations = append(evidence.MutationViolations, MachineViolation{Code: "check_mutation", Message: "check changed repository inputs, Git state, worktree content, root refs, or repository-owned Git configuration"})
	}
	evidence.MutationViolations = sortedByContractKey(evidence.MutationViolations, violationOrderKey)
	evidence.ProcessViolations = sortedByContractKey(evidence.ProcessViolations, violationOrderKey)
	return evidence, loopCheckPassed(evidence)
}

func loopCheckPassed(evidence LoopCheckEvidence) bool {
	return evidence.ExitCode != nil && *evidence.ExitCode == 0 && evidence.Signal == nil && !evidence.TimedOut &&
		len(evidence.MutationViolations) == 0 && len(evidence.ProcessViolations) == 0
}

type loopCheckSnapshot struct {
	Inputs    map[string][]byte
	Git       LoopGitSnapshot
	RootRefs  map[string][]byte
	GitConfig map[string]loopGitConfigFile
	Worktree  string
}

func (s *Service) loopCheckSnapshot() (loopCheckSnapshot, error) {
	inputs, inputErr := loopInputBytes(s.paths)
	git, gitErr := loopGitSnapshot(s.paths.WorktreeRoot, s.paths.GitDir)
	refs, refErr := loopRootRefCandidates(s.paths.GitDir, s.paths.GitCommonDir)
	config, configErr := loopGitConfigSnapshot(s.paths.GitDir, s.paths.GitCommonDir)
	worktree, worktreeErr := parallelWorktreeDigest(s.paths.WorktreeRoot)
	return loopCheckSnapshot{Inputs: inputs, Git: git, RootRefs: refs, GitConfig: config, Worktree: worktree},
		errors.Join(inputErr, gitErr, refErr, configErr, worktreeErr)
}
