//go:build windows

package taskrail

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const windowsContainmentGraceMilliseconds = 10_000

const windowsContainmentLimitation = "allowed, privileged, or undetectable Windows breakaway processes may evade containment"

var ntResumeProcess = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess")

type windowsLoopChildContainment struct {
	job                  windows.Handle
	process              windows.Handle
	assigned             bool
	terminationRequested bool
	leaderReaped         bool
	leaderExit           int
}

func newLoopChildContainment() (loopChildContainment, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	return &windowsLoopChildContainment{job: job}, nil
}

func (c *windowsLoopChildContainment) configure(child *exec.Cmd) {
	attrs := child.SysProcAttr
	if attrs == nil {
		attrs = &syscall.SysProcAttr{}
	}
	attrs.CreationFlags |= windows.CREATE_SUSPENDED
	child.SysProcAttr = attrs
}

func (c *windowsLoopChildContainment) verify(pid int) (loopChildContainmentEvidence, error) {
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_SUSPEND_RESUME|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return c.evidence(), fmt.Errorf("open suspended child: %w", err)
	}
	c.process = process
	if err := windows.AssignProcessToJobObject(c.job, process); err != nil {
		return c.evidence(), fmt.Errorf("assign child to kill-on-close job: %w", err)
	}
	c.assigned = true
	if err := resumeWindowsProcess(process); err != nil {
		return c.evidence(), err
	}
	return c.evidence(), nil
}

func (c *windowsLoopChildContainment) exited(int) (bool, int, string, error) {
	if c.process == 0 {
		return false, 0, "", nil
	}
	status, err := windows.WaitForSingleObject(c.process, 0)
	if err != nil {
		return false, 0, "", err
	}
	if status == uint32(windows.WAIT_TIMEOUT) {
		return false, 0, "", nil
	}
	if status != windows.WAIT_OBJECT_0 {
		return false, 0, "", fmt.Errorf("observe child: unexpected status %#x", status)
	}
	var exitCode uint32
	if err := windows.GetExitCodeProcess(c.process, &exitCode); err != nil {
		return false, 0, "", err
	}
	c.leaderReaped = true
	c.leaderExit = int(exitCode)
	return true, c.leaderExit, "", nil
}

func (*windowsLoopChildContainment) signal(error) string { return "" }

func (c *windowsLoopChildContainment) cleanup() (evidence loopChildContainmentEvidence, result error) {
	evidence = c.evidence()
	defer func() {
		if c.process != 0 {
			result = errors.Join(result, windows.CloseHandle(c.process))
			c.process = 0
		}
		if c.job != 0 {
			result = errors.Join(result, windows.CloseHandle(c.job))
			c.job = 0
		}
	}()
	if !c.assigned {
		return evidence, nil
	}

	members, inspectionErr := windowsJobMembers(c.job)
	if inspectionErr != nil {
		evidence.InspectionError = inspectionErr.Error()
	}
	if inspectionErr == nil && len(members) == 0 {
		evidence.NormalDrain = true
		return evidence, nil
	}
	if err := windows.TerminateJobObject(c.job, 1); err != nil {
		return evidence, errors.Join(inspectionErr, fmt.Errorf("request job termination: %w", err))
	}
	c.terminationRequested = true
	evidence.TerminationRequested = true

	members, drainErr := waitForWindowsJobDrain(c.job, windowsContainmentGraceMilliseconds*time.Millisecond)
	if drainErr != nil {
		return evidence, errors.Join(inspectionErr, fmt.Errorf("wait for job drain: %w", drainErr))
	}
	if len(members) == 0 {
		return evidence, inspectionErr
	}

	evidence.ForcedTermination = true
	if err := windows.TerminateJobObject(c.job, 1); err != nil {
		return evidence, errors.Join(inspectionErr, fmt.Errorf("force job termination: %w", err))
	}
	members, err := windowsJobMembers(c.job)
	if err != nil {
		if evidence.InspectionError == "" {
			evidence.InspectionError = err.Error()
		}
		return evidence, errors.Join(inspectionErr, fmt.Errorf("inspect forced job members: %w", err))
	}
	evidence.SurvivorPIDs = members
	evidence.Survivors = len(members) > 0
	if evidence.Survivors {
		return evidence, errors.Join(inspectionErr, fmt.Errorf("known job survivors: %v", members))
	}
	return evidence, inspectionErr
}

func waitForWindowsJobDrain(job windows.Handle, limit time.Duration) ([]int, error) {
	deadline := time.Now().Add(limit)
	for {
		members, err := windowsJobMembers(job)
		if err != nil || len(members) == 0 || time.Now().After(deadline) {
			return members, err
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (c *windowsLoopChildContainment) evidence() loopChildContainmentEvidence {
	return loopChildContainmentEvidence{
		Platform:              "windows",
		TerminationRequested:  c.terminationRequested,
		ObservationLimitation: windowsContainmentLimitation,
		leaderReaped:          c.leaderReaped,
		leaderExitCode:        c.leaderExit,
	}
}

func resumeWindowsProcess(process windows.Handle) error {
	status, _, _ := ntResumeProcess.Call(uintptr(process))
	if status != 0 {
		return fmt.Errorf("resume contained child: NTSTATUS %#x", status)
	}
	return nil
}

type windowsJobProcessListHeader struct {
	Assigned uint32
	Listed   uint32
}

func windowsJobMembers(job windows.Handle) ([]int, error) {
	assigned, _, err := queryWindowsJobMembers(job, 1)
	if err != nil && !errors.Is(err, windows.ERROR_MORE_DATA) {
		return nil, err
	}
	if assigned == 0 {
		return nil, nil
	}
	_, members, err := queryWindowsJobMembers(job, int(assigned))
	return members, err
}

func queryWindowsJobMembers(job windows.Handle, capacity int) (uint32, []int, error) {
	if capacity < 1 {
		capacity = 1
	}
	size := unsafe.Sizeof(windowsJobProcessListHeader{}) + uintptr(capacity)*unsafe.Sizeof(uintptr(0))
	data := make([]byte, int(size))
	header := (*windowsJobProcessListHeader)(unsafe.Pointer(&data[0]))
	err := windows.QueryInformationJobObject(job, windows.JobObjectBasicProcessIdList,
		uintptr(unsafe.Pointer(&data[0])), uint32(size), nil)
	if err != nil {
		return header.Assigned, nil, err
	}
	if header.Listed > uint32(capacity) || header.Assigned < header.Listed {
		return header.Assigned, nil, fmt.Errorf("invalid job process list: assigned=%d listed=%d", header.Assigned, header.Listed)
	}
	pids := unsafe.Slice((*uintptr)(unsafe.Add(unsafe.Pointer(&data[0]), unsafe.Sizeof(*header))), int(header.Listed))
	members := make([]int, 0, len(pids))
	for _, pid := range pids {
		members = append(members, int(pid))
	}
	return header.Assigned, members, nil
}
