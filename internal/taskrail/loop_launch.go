package taskrail

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// loopChildLaunch holds the frozen values a single generic child receives.
// Later loop stages own selection, containment, and lifecycle interpretation.
type loopChildLaunch struct {
	Command        []string
	Context        context.Context
	Prompt         []byte
	RepositoryRoot string
	Identity       loopChildIdentity
	Stdout         io.Writer
	Stderr         io.Writer
}

// loopChildExecution distinguishes launch and transport failures so postflight
// can preserve the process evidence without treating a failed child as success.
type loopChildExecution struct {
	PID               int
	ExitCode          *int
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
	child.Env = loopChildEnvironment(input.Identity)
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
		if killErr := child.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			execution.ContainmentError = errors.Join(execution.ContainmentError, killErr)
		}
		cleanupLoopChildContainment(&execution, containment)
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
	stdoutResult := copyLoopChildStream(childStdout, stdout)
	stderrResult := copyLoopChildStream(childStderr, stderr)
	stdinResult := writeLoopChildPromptAsync(stdin, input.Prompt)
	contextDone := input.Context
	if contextDone == nil {
		contextDone = context.Background()
	}
	checkExit := time.NewTicker(10 * time.Millisecond)
	defer checkExit.Stop()
	cleanupDone := false
	leaderExited := false
	releaseLeader := func(exitCode int, signal string) {
		if leaderExited {
			return
		}
		leaderExited = true
		execution.ExitCode = &exitCode
		execution.Signal = signal
		if exitCode != 0 {
			execution.WaitError = fmt.Errorf("child exited with code %d", exitCode)
		}
		if err := child.Process.Release(); err != nil {
			execution.ContainmentError = errors.Join(execution.ContainmentError, fmt.Errorf("release reaped child: %w", err))
		}
	}
	cleanup := func() {
		if cleanupDone {
			return
		}
		cleanupDone = true
		cleanupLoopChildContainment(&execution, containment)
		if execution.Containment.leaderReaped {
			releaseLeader(execution.Containment.leaderExitCode, execution.Containment.leaderSignal)
		}
	}
	for stdinResult != nil || stdoutResult != nil || stderrResult != nil {
		select {
		case err := <-stdinResult:
			stdinResult = nil
			execution.StdinError = err
			if err != nil {
				cleanup()
			}
		case err := <-stdoutResult:
			stdoutResult = nil
			execution.StdoutError = err
			if err != nil {
				cleanup()
			}
		case err := <-stderrResult:
			stderrResult = nil
			execution.StderrError = err
			if err != nil {
				cleanup()
			}
		case <-contextDone.Done():
			execution.CancellationError = contextDone.Err()
			cleanup()
		case <-checkExit.C:
			if leaderExited {
				continue
			}
			exited, exitCode, signal, err := containment.exited(execution.PID)
			if err != nil {
				execution.ContainmentError = errors.Join(execution.ContainmentError, fmt.Errorf("observe child exit: %w", err))
				cleanup()
				continue
			}
			if exited {
				releaseLeader(exitCode, signal)
				cleanup()
			}
		}
	}
	if !leaderExited {
		execution.WaitError = child.Wait()
		execution.Signal = containment.signal(execution.WaitError)
		cleanup()
	}
	if execution.ExitCode == nil && child.ProcessState != nil {
		exitCode := child.ProcessState.ExitCode()
		execution.ExitCode = &exitCode
	}
	return execution, nil
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
	return append(values,
		"TASKRAIL="+identity.Executable,
		"TASKRAIL_EXECUTABLE_SHA256="+identity.SHA256,
		"TASKRAIL_DELEGATION_ID="+identity.InvocationID,
		"TASKRAIL_DELEGATION_TOKEN="+identity.Token,
	)
}

func containsLoopChildEnvironmentName(name string) bool {
	for _, candidate := range loopChildEnvironmentNames {
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

func writeLoopChildPrompt(stdin io.WriteCloser, prompt []byte) error {
	written, writeErr := stdin.Write(prompt)
	if writeErr == nil && written != len(prompt) {
		writeErr = io.ErrShortWrite
	}
	closeErr := stdin.Close()
	return errors.Join(writeErr, closeErr)
}
