//go:build windows

package taskrail

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsLoopContainmentAssignsAndDrainsChild(t *testing.T) {
	input := loopHelperLaunch(t, "observe", nil)
	execution, err := launchLoopChild(input)
	if err != nil || execution.Failed() {
		t.Fatalf("launchLoopChild = %+v, %v", execution, err)
	}
	if got := execution.Containment; got.Platform != "windows" || !got.NormalDrain || got.TerminationRequested || got.ForcedTermination || got.Survivors {
		t.Fatalf("containment evidence = %+v", got)
	}
}

func TestWindowsLoopContainmentReportsRequestedStreamTermination(t *testing.T) {
	input := loopHelperLaunch(t, "linger-output", nil)
	input.Stdout = failingLoopWriter{}
	execution, err := launchLoopChild(input)
	if err != nil || execution.StdoutError == nil {
		t.Fatalf("launchLoopChild = %+v, %v", execution, err)
	}
	if got := execution.Containment; got.Platform != "windows" || got.NormalDrain || !got.TerminationRequested || got.ForcedTermination || got.Survivors {
		t.Fatalf("containment evidence = %+v", got)
	}
}

func TestWindowsLoopContainmentTerminatesDescendantsAfterLeaderExit(t *testing.T) {
	input := loopHelperLaunch(t, "spawn-descendant", nil)
	started := time.Now()
	execution, err := launchLoopChild(input)
	if err != nil || execution.Failed() {
		t.Fatalf("launchLoopChild = %+v, %v", execution, err)
	}
	if !execution.Containment.TerminationRequested || execution.Containment.Survivors {
		t.Fatalf("containment evidence = %+v", execution.Containment)
	}
	pid, err := strconv.Atoi(string(readBytes(t, input.Command[len(input.Command)-1]+".descendant")))
	if err != nil {
		t.Fatalf("read descendant pid: %v", err)
	}
	assertWindowsProcessExited(t, uint32(pid))
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cleanup took %s, want less than one second", elapsed)
	}
}

func TestWindowsLoopContainmentTerminatesOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	input := loopHelperLaunch(t, "linger-output", nil)
	input.Context = ctx
	result := make(chan loopChildExecution, 1)
	go func() {
		execution, err := launchLoopChild(input)
		if err != nil {
			execution.ContainmentError = err
		}
		result <- execution
	}()
	select {
	case execution := <-result:
		if execution.CancellationError != context.DeadlineExceeded || execution.ContainmentError != nil ||
			!execution.Containment.TerminationRequested || execution.Containment.Survivors ||
			!execution.Containment.leaderReaped || execution.ExitCode == nil || *execution.ExitCode != 1 || !execution.Failed() {
			t.Fatalf("execution = %+v, want observed contained cancellation failure", execution)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("launcher did not return after contained cancellation")
	}
}

func assertWindowsProcessExited(t *testing.T, pid uint32) {
	t.Helper()
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return
	}
	if err != nil {
		t.Fatalf("open descendant %d: %v", pid, err)
	}
	defer windows.CloseHandle(handle)
	status, err := windows.WaitForSingleObject(handle, 0)
	if err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatalf("descendant %d wait status = %#x, %v", pid, status, err)
	}
}
