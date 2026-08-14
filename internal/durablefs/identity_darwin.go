//go:build darwin

package durablefs

import (
	"os"

	"golang.org/x/sys/unix"
)

func unixIdentity(stat *unix.Stat_t) (Identity, uint64) {
	return Identity{Volume: uint64(stat.Dev), File: stat.Ino, Mount: uint64(stat.Dev)}, uint64(stat.Nlink)
}

func augmentIdentity(_ *os.File, _ *Identity) error { return nil }

func nativeSync(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return unix.Fsync(int(file.Fd()))
	}
	_, err = unix.FcntlInt(file.Fd(), unix.F_FULLFSYNC, 0)
	return err
}
