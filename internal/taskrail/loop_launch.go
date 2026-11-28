package taskrail

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tessariq/taskrail/internal/repolock"
)

// loopChildLaunch holds the frozen values a single generic child receives.
// Later loop stages own selection, containment, and lifecycle interpretation.
type loopChildLaunch struct {
	Command        []string
	Context        context.Context
	Timeout        *time.Duration
	Prompt         []byte
	RepositoryRoot string
	Identity       loopChildIdentity
	Environment    []string
	Stdout         io.Writer
	Stderr         io.Writer
}

// loopChildExecution distinguishes launch and transport failures so postflight
// can preserve the process evidence without treating a failed child as success.
type loopChildExecution struct {
	PID               int
	ExitCode          *int
	TimedOut          bool
	Containment       loopChildContainmentEvidence
	ContainmentError  error
	CancellationError error
	Signal            string
	SpawnError        error
	StdinError        error
	StdoutError       error
	StderrError       error
	WaitError         error
}

func (e loopChildExecution) Failed() bool {
	return e.ContainmentError != nil || e.CancellationError != nil || e.SpawnError != nil || e.StdinError != nil || e.StdoutError != nil || e.StderrError != nil || e.WaitError != nil
}

func launchLoopChild(input loopChildLaunch) (loopChildExecution, error) {
	if len(input.Command) == 0 || input.Command[0] == "" {
		return loopChildExecution{}, errors.New("loop child command is required")
	}
	if !filepath.IsAbs(input.RepositoryRoot) {
		return loopChildExecution{}, errors.New("loop child repository root must be absolute")
	}
	command, err := resolveLoopChildCommand(input.Command)
	if err != nil {
		return loopChildExecution{}, err
	}
	stdout := input.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := input.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	child := exec.Command(command[0], command[1:]...)
	child.Dir = input.RepositoryRoot
	child.Env = input.Environment
	if child.Env == nil {
		child.Env = loopChildEnvironment(input.Identity)
	}
	containment, err := newLoopChildContainment()
	if err != nil {
		return loopChildExecution{}, err
	}
	containment.configure(child)
	stdin, err := child.StdinPipe()
	if err != nil {
		return loopChildExecution{}, closeLoopChildContainment(containment, fmt.Errorf("open child stdin: %w", err))
	}
	childStdout, err := child.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return loopChildExecution{}, closeLoopChildContainment(containment, fmt.Errorf("open child stdout: %w", err))
	}
	childStderr, err := child.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = childStdout.Close()
		return loopChildExecution{}, closeLoopChildContainment(containment, fmt.Errorf("open child stderr: %w", err))
	}
	if err := child.Start(); err != nil {
		_ = stdin.Close()
		_ = childStdout.Close()
		_ = childStderr.Close()
		execution := loopChildExecution{SpawnError: err}
		cleanupLoopChildContainment(&execution, containment)
		return execution, nil
	}

	execution := loopChildExecution{PID: child.Process.Pid}
	containmentEvidence, err := containment.verify(execution.PID)
	execution.Containment = containmentEvidence
	if err != nil {
		execution.ContainmentError = err
		cleanupLoopChildContainment(&execution, containment)
		closeLoopChildTransport(stdin, childStdout, childStderr)
		if execution.Containment.leaderReaped {
			execution.ExitCode = &execution.Containment.leaderExitCode
			execution.Signal = execution.Containment.leaderSignal
			execution.ContainmentError = errors.Join(execution.ContainmentError, child.Process.Release())
		} else {
			execution.WaitError = child.Wait()
			execution.Signal = containment.signal(execution.WaitError)
		}
		return execution, nil
	}
	supervisor := &loopChildSupervisor{execution: execution, child: child, containment: containment, stdin: stdin, stdout: childStdout, stderr: childStderr}
	return supervisor.run(input, stdout, stderr), nil
}

// loopChildSupervisor owns a started, contained child until its leader exit
// and every transport stream have been observed, so cleanup runs exactly once.
type loopChildSupervisor struct {
	execution    loopChildExecution
	child        *exec.Cmd
	containment  loopChildContainment
	stdin        io.WriteCloser
	stdout       io.ReadCloser
	stderr       io.ReadCloser
	cleanupDone  bool
	leaderExited bool
}

func (s *loopChildSupervisor) run(input loopChildLaunch, stdout, stderr io.Writer) loopChildExecution {
	stdoutResult := copyLoopChildStream(s.stdout, stdout)
	stderrResult := copyLoopChildStream(s.stderr, stderr)
	stdinResult := writeLoopChildPromptAsync(s.stdin, input.Prompt)
	contextDone, stopContext := loopChildContext(input)
	defer stopContext()
	checkExit := time.NewTicker(10 * time.Millisecond)
	defer checkExit.Stop()
	for !s.leaderExited && !s.cleanupDone || stdinResult != nil || stdoutResult != nil || stderrResult != nil {
		select {
		case err := <-stdinResult:
			stdinResult = nil
			s.execution.StdinError = err
			s.cleanupOnError(err)
		case err := <-stdoutResult:
			stdoutResult = nil
			s.execution.StdoutError = err
			s.cleanupOnError(err)
		case err := <-stderrResult:
			stderrResult = nil
			s.execution.StderrError = err
			s.cleanupOnError(err)
		case <-contextDone.Done():
			s.execution.TimedOut = loopChildTimedOut(input, contextDone)
			s.execution.CancellationError = contextDone.Err()
			s.cleanup()
		case <-checkExit.C:
			s.observeExit()
		}
	}
	return s.execution
}

func (s *loopChildSupervisor) cleanupOnError(err error) {
	if err != nil {
		s.cleanup()
	}
}

func (s *loopChildSupervisor) observeExit() {
	if s.leaderExited {
		return
	}
	exited, exitCode, signal, err := s.containment.exited(s.execution.PID)
	if err != nil {
		s.execution.ContainmentError = errors.Join(s.execution.ContainmentError, fmt.Errorf("observe child exit: %w", err))
		s.cleanup()
		return
	}
	if exited {
		s.releaseLeader(exitCode, signal)
		s.cleanup()
	}
}

func (s *loopChildSupervisor) releaseLeader(exitCode int, signal string) {
	if s.leaderExited {
		return
	}
	s.leaderExited = true
	s.execution.ExitCode = &exitCode
	s.execution.Signal = signal
	if exitCode != 0 {
		s.execution.WaitError = fmt.Errorf("child exited with code %d", exitCode)
	}
	if err := s.child.Process.Release(); err != nil {
		s.execution.ContainmentError = errors.Join(s.execution.ContainmentError, fmt.Errorf("release reaped child: %w", err))
	}
}

func (s *loopChildSupervisor) cleanup() {
	if s.cleanupDone {
		return
	}
	s.cleanupDone = true
	cleanupLoopChildContainment(&s.execution, s.containment)
	if s.execution.ContainmentError != nil {
		closeLoopChildTransport(s.stdin, s.stdout, s.stderr)
	}
	if s.execution.Containment.leaderReaped {
		s.releaseLeader(s.execution.Containment.leaderExitCode, s.execution.Containment.leaderSignal)
	} else if !s.leaderExited {
		s.execution.ContainmentError = errors.Join(s.execution.ContainmentError,
			errors.New("leader termination was not observed during containment cleanup"), s.child.Process.Release())
		closeLoopChildTransport(s.stdin, s.stdout, s.stderr)
	}
}

func loopChildContext(input loopChildLaunch) (context.Context, context.CancelFunc) {
	parent := input.Context
	if parent == nil {
		parent = context.Background()
	}
	if input.Timeout == nil {
		return parent, func() {}
	}
	return context.WithTimeout(parent, *input.Timeout)
}

func loopChildTimedOut(input loopChildLaunch, ctx context.Context) bool {
	return input.Timeout != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) &&
		(input.Context == nil || input.Context.Err() == nil)
}

func resolveLoopChildCommand(command []string) ([]string, error) {
	resolved := append([]string{}, command...)
	if !containsLoopPathSeparator(resolved[0]) {
		return resolved, nil
	}
	path, err := filepath.Abs(resolved[0])
	if err != nil {
		return nil, fmt.Errorf("resolve child executable %s: %w", resolved[0], err)
	}
	resolved[0] = path
	return resolved, nil
}

func containsLoopPathSeparator(value string) bool {
	return strings.Contains(value, string(filepath.Separator)) ||
		filepath.Separator == '\\' && strings.Contains(value, "/")
}

func loopChildEnvironment(identity loopChildIdentity) []string {
	values := make([]string, 0, len(os.Environ())+len(loopChildEnvironmentNames))
	for _, entry := range os.Environ() {
		name, _, found := strings.Cut(entry, "=")
		if found && !containsLoopChildEnvironmentName(name) {
			values = append(values, entry)
		}
	}
	values = append(values,
		"TASKRAIL="+identity.Executable,
		"TASKRAIL_EXECUTABLE_SHA256="+identity.SHA256,
		"TASKRAIL_DELEGATION_ID="+identity.InvocationID,
		"TASKRAIL_DELEGATION_TOKEN="+identity.Token,
	)
	return append(values, loopFollowupSequenceEnvironment(identity.FollowupSequence)...)
}

func loopFollowupSequenceEnvironment(sequence *repolock.FollowupSequence) []string {
	if sequence == nil {
		return nil
	}
	return []string{
		"TASKRAIL_FOLLOWUP_MAX=" + strconv.Itoa(sequence.Maximum),
		"TASKRAIL_FOLLOWUP_WIDTH=" + strconv.Itoa(sequence.Width),
		"TASKRAIL_FOLLOWUP_RANK=" + strconv.Itoa(sequence.Rank),
	}
}

func containsLoopChildEnvironmentName(name string) bool {
	for _, candidate := range append(loopChildEnvironmentNames, loopFollowupSequenceEnvironmentNames...) {
		if name == candidate {
			return true
		}
	}
	return false
}

func copyLoopChildStream(source io.ReadCloser, destination io.Writer) <-chan error {
	result := make(chan error, 1)
	go func() {
		_, err := io.Copy(destination, source)
		_ = source.Close()
		result <- err
	}()
	return result
}

func writeLoopChildPromptAsync(stdin io.WriteCloser, prompt []byte) <-chan error {
	result := make(chan error, 1)
	go func() { result <- writeLoopChildPrompt(stdin, prompt) }()
	return result
}

func cleanupLoopChildContainment(execution *loopChildExecution, containment loopChildContainment) {
	if execution.Containment.NormalDrain || execution.Containment.TerminationRequested || execution.Containment.ForcedTermination || execution.Containment.Survivors {
		return
	}
	evidence, err := containment.cleanup()
	execution.Containment = evidence
	if err != nil {
		execution.ContainmentError = errors.Join(execution.ContainmentError, err)
	}
}

func closeLoopChildContainment(containment loopChildContainment, cause error) error {
	_, cleanupErr := containment.cleanup()
	return errors.Join(cause, cleanupErr)
}

func closeLoopChildTransport(stdin io.WriteCloser, streams ...io.ReadCloser) {
	_ = stdin.Close()
	for _, stream := range streams {
		_ = stream.Close()
	}
}

func writeLoopChildPrompt(stdin io.WriteCloser, prompt []byte) error {
	written, writeErr := stdin.Write(prompt)
	if writeErr == nil && written != len(prompt) {
		writeErr = io.ErrShortWrite
	}
	closeErr := stdin.Close()
	return errors.Join(writeErr, closeErr)
}
