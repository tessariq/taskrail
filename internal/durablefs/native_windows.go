//go:build windows

package durablefs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func nativeIdentity(file *os.File) (Identity, uint64, error) {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &info); err != nil {
		return Identity{}, 0, fmt.Errorf("read native identity: %w", err)
	}
	identity := Identity{
		Volume: uint64(info.VolumeSerialNumber),
		File:   uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow),
		Mount:  uint64(info.VolumeSerialNumber),
	}
	if identity.Volume == 0 || identity.File == 0 {
		return Identity{}, 0, fmt.Errorf("%w: filesystem returned an empty file identity", ErrUnsupported)
	}
	return identity, uint64(info.NumberOfLinks), nil
}

func openObserved(parent *os.Root, name string, directory bool) (*os.File, error) {
	before, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if isReparsePoint(before) || before.Mode()&fs.ModeSymlink != 0 || directory != before.IsDir() {
		return nil, fmt.Errorf("entry %q is a reparse point or has the wrong type", name)
	}
	file, err := parent.Open(name)
	if err != nil {
		return nil, err
	}
	after, err := parent.Lstat(name)
	if err != nil || isReparsePoint(after) || after.Mode()&fs.ModeSymlink != 0 || directory != after.IsDir() {
		file.Close()
		return nil, fmt.Errorf("%w: entry %q changed while opening", ErrConflict, name)
	}
	return file, nil
}

func openRootObserved(path string) (*os.File, error) {
	volume := filepath.VolumeName(path)
	current := volume + string(filepath.Separator)
	var final windows.Handle
	for _, part := range strings.Split(strings.TrimPrefix(path[len(volume):], string(filepath.Separator)), string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		name, err := windows.UTF16PtrFromString(current)
		if err != nil {
			return nil, err
		}
		handle, err := windows.CreateFile(name, windows.GENERIC_READ,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
			windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if err != nil {
			return nil, err
		}
		var info windows.ByHandleFileInformation
		if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
			windows.CloseHandle(handle)
			return nil, err
		}
		if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
			windows.CloseHandle(handle)
			return nil, fmt.Errorf("%w: root component %q is a reparse point or not a directory", ErrInvalidPath, part)
		}
		if final != 0 {
			windows.CloseHandle(final)
		}
		final = handle
	}
	if final == 0 {
		return nil, fmt.Errorf("%w: root has no component", ErrInvalidPath)
	}
	return os.NewFile(uintptr(final), path), nil
}

type fileRenameInfo struct {
	ReplaceIfExists uint32
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

func replaceStaged(parent *os.Root, _ *os.File, staged, leaf string) error {
	directory, err := parent.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	objectName, err := windows.NewNTUnicodeString(staged)
	if err != nil {
		return err
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: windows.Handle(directory.Fd()),
		ObjectName:    objectName,
	}
	var source windows.Handle
	err = windows.NtCreateFile(&source, windows.FILE_GENERIC_READ|windows.DELETE, attributes,
		&windows.IO_STATUS_BLOCK{}, nil, windows.FILE_ATTRIBUTE_NORMAL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN, windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_NON_DIRECTORY_FILE,
		0, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(source)
	name, err := windows.UTF16FromString(leaf)
	if err != nil {
		return err
	}
	name = name[:len(name)-1]
	var layout fileRenameInfo
	offset := unsafe.Offsetof(layout.FileName)
	buffer := make([]byte, int(offset)+len(name)*2)
	info := (*fileRenameInfo)(unsafe.Pointer(&buffer[0]))
	info.ReplaceIfExists = windows.FILE_RENAME_REPLACE_IF_EXISTS | windows.FILE_RENAME_POSIX_SEMANTICS
	info.RootDirectory = windows.Handle(directory.Fd())
	info.FileNameLength = uint32(len(name) * 2)
	copy(unsafe.Slice(&info.FileName[0], len(name)), name)
	var status windows.IO_STATUS_BLOCK
	if err := windows.NtSetInformationFile(source, &status, &buffer[0], uint32(len(buffer)), windows.FileRenameInformation); err != nil {
		return err
	}
	return nil
}

func isReparsePoint(info fs.FileInfo) bool {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

func classifySyncError(step Barrier, err error) error {
	if errors.Is(err, windows.ERROR_INVALID_FUNCTION) || errors.Is(err, windows.ERROR_NOT_SUPPORTED) || errors.Is(err, windows.ERROR_INVALID_HANDLE) {
		return fmt.Errorf("%w: %s barrier: %v", ErrUnsupported, step, err)
	}
	return fmt.Errorf("%s barrier: %w", step, err)
}

func nativeSync(file *os.File) error {
	return windows.FlushFileBuffers(windows.Handle(file.Fd()))
}
