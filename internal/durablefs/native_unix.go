//go:build linux || darwin

package durablefs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func nativeIdentity(file *os.File) (Identity, uint64, error) {
	var stat unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &stat); err != nil {
		return Identity{}, 0, fmt.Errorf("read native identity: %w", err)
	}
	identity, links := unixIdentity(&stat)
	if err := augmentIdentity(file, &identity); err != nil {
		return Identity{}, 0, err
	}
	if identity.Volume == 0 || identity.File == 0 {
		return Identity{}, 0, fmt.Errorf("%w: filesystem returned an empty file identity", ErrUnsupported)
	}
	return identity, links, nil
}

func openObserved(parent *os.Root, name string, directory bool) (*os.File, error) {
	flags := os.O_RDONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC
	if directory {
		flags |= unix.O_DIRECTORY
	}
	file, err := parent.OpenFile(name, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("open %q without following links: %w", name, err)
	}
	return file, nil
}

func openRootObserved(path string) (*os.File, error) {
	fd, err := unix.Open(string(filepath.Separator), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, string(filepath.Separator)), string(filepath.Separator)) {
		if part == "" {
			continue
		}
		next, openErr := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(fd)
		if openErr != nil {
			return nil, fmt.Errorf("open root component %q without following links: %w", part, openErr)
		}
		fd = next
	}
	return os.NewFile(uintptr(fd), path), nil
}

func replaceStaged(parent *os.Root, _ *os.File, staged, leaf string) error {
	return parent.Rename(staged, leaf)
}

func isReparsePoint(fs.FileInfo) bool { return false }

func classifySyncError(step Barrier, err error) error {
	if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.ENOSYS) {
		return fmt.Errorf("%w: %s barrier: %v", ErrUnsupported, step, err)
	}
	return fmt.Errorf("%s barrier: %w", step, err)
}
