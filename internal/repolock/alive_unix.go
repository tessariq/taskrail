//go:build unix

package repolock

import (
	"errors"

	"golang.org/x/sys/unix"
)

// processAlive probes one pid on a Unix-like host. Signal 0 performs no
// action, so it is a pure existence check: success and EPERM both name a live
// process (EPERM only means it belongs to another user), while ESRCH is the
// kernel reporting no such process.
func processAlive(pid int) bool {
	err := unix.Kill(pid, 0)
	return err == nil || !errors.Is(err, unix.ESRCH)
}
