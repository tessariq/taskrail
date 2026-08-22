//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package taskrail

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

const unixContainmentGracePeriod = 10 * time.Second

const unixContainmentLimitation = "privileged or undetectable session escape is not certified as contained"

var unixGetpgid = syscall.Getpgid

type unixLoopChildContainment struct {
	processGroup int
	processID    int
	leaderReaped bool
	leaderExit   int
	leaderSignal string
}

func newLoopChildContainment() (loopChildContainment, error) {
	return &unixLoopChildContainment{}, nil
}

func (c *unixLoopChildContainment) configure(child *exec.Cmd) {
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func (c *unixLoopChildContainment) verify(pid int) (loopChildContainmentEvidence, error) {
	c.processGroup = pid
	c.processID = pid
	group, err := unixGetpgid(pid)
	if err != nil {
		return c.evidence(), fmt.Errorf("verify child process group: %w", err)
	}
	if group != pid {
		return c.evidence(), fmt.Errorf("verify child process group: got %d, want %d", group, pid)
	}
	if _, err := unixProcessGroupExists(group); err != nil {
		return c.evidence(), fmt.Errorf("verify child process group %d: %w", group, err)
	}
	return c.evidence(), nil
}

func (c *unixLoopChildContainment) exited(pid int) (bool, int, string, error) {
	var status syscall.WaitStatus
	waited, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
	if err != nil {
		return false, 0, "", err
	}
	if waited == 0 {
		return false, 0, "", nil
	}
	c.leaderReaped = true
	if status.Exited() {
		c.leaderExit = status.ExitStatus()
		return true, c.leaderExit, "", nil
	}
	c.leaderExit = -1
	if status.Signaled() {
		c.leaderSignal = status.Signal().String()
	}
	return true, c.leaderExit, c.leaderSignal, nil
}

func (c *unixLoopChildContainment) signal(err error) string {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return ""
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		return ""
	}
	return status.Signal().String()
}

func (c *unixLoopChildContainment) cleanup() (loopChildContainmentEvidence, error) {
	if c.processGroup == 0 {
		return c.evidence(), nil
	}
	alive, err := unixProcessGroupExists(c.processGroup)
	if err != nil {
		return c.evidence(), fmt.Errorf("inspect child process group %d: %w", c.processGroup, err)
	}
	if !alive {
		evidence := c.evidence()
		evidence.NormalDrain = true
		return evidence, nil
	}
	if err := syscall.Kill(-c.processGroup, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		return c.evidence(), fmt.Errorf("terminate child process group %d: %w", c.processGroup, err)
	}
	if !c.waitForGroupExit(unixContainmentGracePeriod) {
		if err := syscall.Kill(-c.processGroup, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
			return c.evidence(), fmt.Errorf("force child process group %d: %w", c.processGroup, err)
		}
		if !c.waitForGroupExit(unixContainmentGracePeriod) {
			evidence := c.evidence()
			evidence.Survivors = true
			return evidence, fmt.Errorf("child process group %d has survivors", c.processGroup)
		}
		evidence := c.evidence()
		evidence.TerminationRequested = true
		evidence.ForcedTermination = true
		return evidence, nil
	}
	evidence := c.evidence()
	evidence.TerminationRequested = true
	return evidence, nil
}

func (c *unixLoopChildContainment) waitForGroupExit(limit time.Duration) bool {
	deadline := time.Now().Add(limit)
	for {
		c.reapLeader()
		alive, err := unixProcessGroupExists(c.processGroup)
		if err != nil || !alive {
			return !alive
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (c *unixLoopChildContainment) reapLeader() {
	if c.leaderReaped {
		return
	}
	_, _, _, _ = c.exited(c.processID)
}

func (c *unixLoopChildContainment) evidence() loopChildContainmentEvidence {
	return loopChildContainmentEvidence{
		ProcessGroup:          c.processGroup,
		ObservationLimitation: unixContainmentLimitation,
		leaderReaped:          c.leaderReaped,
		leaderExitCode:        c.leaderExit,
		leaderSignal:          c.leaderSignal,
	}
}

func unixProcessGroupExists(group int) (bool, error) {
	err := syscall.Kill(-group, 0)
	if err == nil || err == syscall.EPERM {
		return true, nil
	}
	if err == syscall.ESRCH {
		return false, nil
	}
	return false, err
}
