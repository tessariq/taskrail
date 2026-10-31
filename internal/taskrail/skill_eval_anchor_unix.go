//go:build !windows

package taskrail

import (
	"fmt"
	"os"
	"syscall"
)

func skillEvalPathIdentity(path string) (uint64, uint64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, 0, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, fmt.Errorf("filesystem identity unavailable")
	}
	return uint64(stat.Dev), uint64(stat.Ino), nil
}
