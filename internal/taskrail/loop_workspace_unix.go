//go:build !windows

package taskrail

import (
	"fmt"
	"os"
	"syscall"
)

func validateParallelWorkspaceOwnership(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Getuid() {
		return fmt.Errorf("--workspace-root must be owned by the invoking user")
	}
	return nil
}
