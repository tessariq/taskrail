package taskrail

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// loopChildLaunch holds the frozen values a single generic child receives.
// Later loop stages own selection, containment, and lifecycle interpretation.
type loopChildLaunch struct {
	Command        []string
	Prompt         []byte
	RepositoryRoot string
	Identity       loopChildIdentity
	Stdout         io.Writer
	Stderr         io.Writer
}

// loopChildExecution distinguishes launch and transport failures so postflight
// can preserve the process evidence without treating a failed child as success.
type loopChildExecution struct {
	PID         int
	ExitCode    *int
	SpawnError  error
	StdinError  error
	StdoutError error
	StderrError error
	WaitError   error
}

func (e loopChildExecution) Failed() bool {
	return e.SpawnError != nil || e.StdinError != nil || e.StdoutError != nil || e.StderrError != nil || e.WaitError != nil
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
	stdin, err := child.StdinPipe()
	if err != nil {
		return loopChildExecution{}, fmt.Errorf("open child stdin: %w", err)
	}
	childStdout, err := child.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return loopChildExecution{}, fmt.Errorf("open child stdout: %w", err)
	}
	childStderr, err := child.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = childStdout.Close()
		return loopChildExecution{}, fmt.Errorf("open child stderr: %w", err)
	}
	if err := child.Start(); err != nil {
		_ = stdin.Close()
		_ = childStdout.Close()
		_ = childStderr.Close()
		return loopChildExecution{SpawnError: err}, nil
	}

	execution := loopChildExecution{PID: child.Process.Pid}
	stdoutResult := copyLoopChildStream(childStdout, stdout)
	stderrResult := copyLoopChildStream(childStderr, stderr)
	stdinResult := writeLoopChildPromptAsync(stdin, input.Prompt)
	terminated := false
	for stdinResult != nil || stdoutResult != nil || stderrResult != nil {
		select {
		case err := <-stdinResult:
			stdinResult = nil
			execution.StdinError = err
			terminated = stopLoopChild(child, err, terminated)
		case err := <-stdoutResult:
			stdoutResult = nil
			execution.StdoutError = err
			terminated = stopLoopChild(child, err, terminated)
		case err := <-stderrResult:
			stderrResult = nil
			execution.StderrError = err
			terminated = stopLoopChild(child, err, terminated)
		}
	}
	execution.WaitError = child.Wait()
	if child.ProcessState != nil {
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

func stopLoopChild(child *exec.Cmd, cause error, terminated bool) bool {
	if cause == nil || terminated {
		return terminated
	}
	_ = child.Process.Kill()
	return true
}

func writeLoopChildPrompt(stdin io.WriteCloser, prompt []byte) error {
	written, writeErr := stdin.Write(prompt)
	if writeErr == nil && written != len(prompt) {
		writeErr = io.ErrShortWrite
	}
	closeErr := stdin.Close()
	return errors.Join(writeErr, closeErr)
}
