//go:build linux

package durablefs

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func unixIdentity(stat *unix.Stat_t) (Identity, uint64) {
	return Identity{Volume: uint64(stat.Dev), File: stat.Ino, Mount: uint64(stat.Dev)}, uint64(stat.Nlink)
}

func augmentIdentity(file *os.File, identity *Identity) error {
	var stat unix.Statx_t
	err := unix.Statx(int(file.Fd()), "", unix.AT_EMPTY_PATH|unix.AT_SYMLINK_NOFOLLOW, unix.STATX_MNT_ID, &stat)
	if err == nil && stat.Mask&unix.STATX_MNT_ID != 0 {
		identity.Mount = stat.Mnt_id
		return nil
	}
	if err == nil || errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EOPNOTSUPP) {
		return nil
	}
	return fmt.Errorf("read mount identity: %w", err)
}

func nativeSync(file *os.File) error { return unix.Fsync(int(file.Fd())) }
