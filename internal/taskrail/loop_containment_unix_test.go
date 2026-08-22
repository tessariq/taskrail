//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package taskrail

import (
	"context"
	"errors"
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestLoopLaunchChildTerminatesDescendantsAfterLeaderExit(t *testing.T) {
	input := loopHelperLaunch(t, "spawn-descendant", nil)
	started := time.Now()
	execution, err := launchLoopChild(input)
	if err != nil || execution.Failed() {
		t.Fatalf("launchLoopChild = %+v, %v", execution, err)
	}
	if !execution.Containment.TerminationRequested {
		t.Fatalf("containment = %+v, want requested termination", execution.Containment)
	}
	pid, err := strconv.Atoi(string(readBytes(t, input.Command[len(input.Command)-1]+".descendant")))
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

func TestLoopLaunchChildForcesProcessGroupTermination(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	input := loopChildLaunch{
		Command:        []string{"/bin/sh", "-c", "trap '' TERM; while :; do :; done"},
		Context:        ctx,
		RepositoryRoot: t.TempDir(),
	}
	started := time.Now()
	execution, err := launchLoopChild(input)
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
