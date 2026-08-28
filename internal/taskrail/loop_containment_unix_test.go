//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package taskrail

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestLoopLaunchChildTerminatesDescendantsAfterLeaderExit(t *testing.T) {
	// The race runtime delays child exit by one second unless this fixture
	// disables that unrelated post-exit sleep.
	raceOptions := os.Getenv("GORACE")
	if raceOptions != "" {
		raceOptions += " "
	}
	t.Setenv("GORACE", raceOptions+"atexit_sleep_ms=0")
	input := loopHelperLaunch(t, "spawn-descendant", nil)
	record := input.Command[len(input.Command)-1]
	result := make(chan struct {
		execution loopChildExecution
		err       error
	}, 1)
	go func() {
		execution, err := launchLoopChild(input)
		result <- struct {
			execution loopChildExecution
			err       error
		}{execution, err}
	}()

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		if _, err := os.Stat(record + ".descendant"); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("inspect descendant readiness: %v", err)
		}
		select {
		case result := <-result:
			t.Fatalf("launchLoopChild returned before descendant readiness: %+v, %v", result.execution, result.err)
		case <-deadline.C:
			t.Fatal("descendant did not become ready")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	started := time.Now()
	outcome := <-result
	execution, err := outcome.execution, outcome.err
	if err != nil || execution.Failed() {
		t.Fatalf("launchLoopChild = %+v, %v", execution, err)
	}
	if !execution.Containment.TerminationRequested {
		t.Fatalf("containment = %+v, want requested termination", execution.Containment)
	}
	pid, err := strconv.Atoi(string(readBytes(t, record+".descendant")))
	if err != nil {
		t.Fatalf("read descendant pid: %v", err)
	}
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatalf("descendant %d remains after containment cleanup: %v", pid, err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cleanup took %s, want less than one second", elapsed)
	}
}

func TestLoopLaunchChildTerminatesOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	input := loopHelperLaunch(t, "linger-output", nil)
	input.Context = ctx
	started := time.Now()
	execution, err := launchLoopChild(input)
	if err != nil {
		t.Fatalf("launchLoopChild: %v", err)
	}
	if execution.CancellationError != context.DeadlineExceeded {
		t.Fatalf("cancellation = %v, want deadline exceeded", execution.CancellationError)
	}
	if execution.TimedOut {
		t.Fatalf("execution = %+v, caller cancellation must not report a loop timeout", execution)
	}
	if execution.Signal != syscall.SIGTERM.String() {
		t.Fatalf("signal = %q, want %q", execution.Signal, syscall.SIGTERM.String())
	}
	if !execution.Containment.TerminationRequested || !execution.Failed() {
		t.Fatalf("execution = %+v, want contained cancellation failure", execution)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cleanup took %s, want less than one second", elapsed)
	}
}

func TestLoopLaunchChildCleansUpAfterContainmentVerificationFailure(t *testing.T) {
	input := loopHelperLaunch(t, "spawn-descendant", nil)
	record := input.Command[len(input.Command)-1]
	original := unixGetpgid
	unixGetpgid = func(int) (int, error) {
		deadline := time.Now().Add(time.Second)
		for {
			if _, err := os.Stat(record + ".descendant"); err == nil || time.Now().After(deadline) {
				return 0, errors.New("forced containment verification failure")
			}
			time.Sleep(time.Millisecond)
		}
	}
	t.Cleanup(func() { unixGetpgid = original })

	started := time.Now()
	execution, err := launchLoopChild(input)
	if err != nil {
		t.Fatalf("launchLoopChild: %v", err)
	}
	if execution.ContainmentError == nil || !execution.Containment.TerminationRequested || !execution.Failed() {
		t.Fatalf("execution = %+v, want containment verification failure after cleanup", execution)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cleanup took %s, want less than one second", elapsed)
	}
	pid, err := strconv.Atoi(string(readBytes(t, record+".descendant")))
	if err != nil {
		t.Fatalf("read descendant pid: %v", err)
	}
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatalf("descendant %d remains after verification cleanup: %v", pid, err)
	}
}

func TestLoopLaunchChildReturnsAfterSurvivorEvidence(t *testing.T) {
	if _, err := exec.LookPath("setsid"); err != nil {
		t.Skip("setsid is required to retain an escaped descendant stream")
	}
	record := t.TempDir() + "/descendant"
	t.Setenv("LOOP_DESCENDANT", record)
	originalAlive, originalKill, originalGrace := unixProcessGroupIsAlive, unixKill, unixContainmentGracePeriod
	unixProcessGroupIsAlive = func(int) (bool, error) { return true, nil }
	unixKill = func(int, syscall.Signal) error { return nil }
	unixContainmentGracePeriod = 0
	t.Cleanup(func() {
		unixProcessGroupIsAlive = originalAlive
		unixKill = originalKill
		unixContainmentGracePeriod = originalGrace
		if data, err := os.ReadFile(record); err == nil {
			if pid, err := strconv.Atoi(string(data)); err == nil {
				_ = syscall.Kill(-pid, syscall.SIGKILL)
			}
		}
	})

	input := loopChildLaunch{
		Command:        []string{"/bin/sh", "-c", "setsid sleep 5 & echo $! > \"$LOOP_DESCENDANT\"; exit 0"},
		RepositoryRoot: t.TempDir(),
	}
	started := time.Now()
	execution, err := launchLoopChild(input)
	if err != nil {
		t.Fatalf("launchLoopChild: %v", err)
	}
	if !execution.Containment.Survivors || execution.ContainmentError == nil || execution.StdoutError == nil {
		t.Fatalf("execution = %+v, want survivor evidence and closed stream failure", execution)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("launchLoopChild returned after %s, want survivor evidence without waiting for the escaped stream", elapsed)
	}
}

func TestLoopLaunchChildForcesProcessGroupTermination(t *testing.T) {
	originalGrace := unixContainmentGracePeriod
	unixContainmentGracePeriod = 20 * time.Millisecond
	t.Cleanup(func() { unixContainmentGracePeriod = originalGrace })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input := loopHelperLaunch(t, "ignore-termination", nil)
	input.Context = ctx
	record := input.Command[len(input.Command)-1]
	result := make(chan struct {
		execution loopChildExecution
		err       error
	}, 1)
	go func() {
		execution, err := launchLoopChild(input)
		result <- struct {
			execution loopChildExecution
			err       error
		}{execution, err}
	}()

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		if _, err := os.Stat(record + ".ready"); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("inspect helper readiness: %v", err)
		}
		select {
		case outcome := <-result:
			t.Fatalf("launchLoopChild returned before helper readiness: %+v, %v", outcome.execution, outcome.err)
		case <-deadline.C:
			t.Fatal("termination-resistant helper did not become ready")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	started := time.Now()
	cancel()
	terminationDeadline := time.NewTimer(unixContainmentGracePeriod + 2*time.Second)
	defer terminationDeadline.Stop()
	var outcome struct {
		execution loopChildExecution
		err       error
	}
	select {
	case outcome = <-result:
	case <-terminationDeadline.C:
		t.Fatal("forced process-group termination did not return")
	}
	execution, err := outcome.execution, outcome.err
	if err != nil {
		t.Fatalf("launchLoopChild: %v", err)
	}
	if !execution.Containment.TerminationRequested || !execution.Containment.ForcedTermination || !execution.Failed() {
		t.Fatalf("execution = %+v, want forced containment failure", execution)
	}
	if elapsed := time.Since(started); elapsed < unixContainmentGracePeriod || elapsed > unixContainmentGracePeriod+2*time.Second {
		t.Fatalf("cleanup took %s, want one graceful termination interval", elapsed)
	}
}
