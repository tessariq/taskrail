//go:build windows

package repolock

import (
	"errors"

	"golang.org/x/sys/windows"
)

// processAlive probes one pid on Windows by opening a query handle. The
// platform distinguishes its answers: a recycled or exited pid fails to open
// with ERROR_INVALID_PARAMETER, while ERROR_ACCESS_DENIED names an existing
// process this user cannot query — the Windows analogue of Unix EPERM, and
// therefore a live owner `lock clear` must refuse rather than delete.
func processAlive(pid int) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return errors.Is(err, windows.ERROR_ACCESS_DENIED)
	}
	// The close is best effort: a successful open already proved liveness,
	// and a close failure must not turn a proven-live owner into one
	// `lock clear` would delete.
	_ = windows.CloseHandle(handle)
	return true
}
