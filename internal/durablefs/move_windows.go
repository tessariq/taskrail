//go:build windows

package durablefs

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func moveDirectoryNoReplace(source *os.Root, sourceName string, destination *os.Root, destinationName string) error {
	sourceDir, err := source.Open(".")
	if err != nil {
		return err
	}
	defer sourceDir.Close()
	destinationDir, err := destination.Open(".")
	if err != nil {
		return err
	}
	defer destinationDir.Close()
	objectName, err := windows.NewNTUnicodeString(sourceName)
	if err != nil {
		return err
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: windows.Handle(sourceDir.Fd()),
		ObjectName:    objectName,
	}
	var handle windows.Handle
	err = windows.NtCreateFile(&handle, windows.SYNCHRONIZE|windows.DELETE, attributes, &windows.IO_STATUS_BLOCK{}, nil,
		windows.FILE_ATTRIBUTE_NORMAL, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN, windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_DIRECTORY_FILE, 0, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	name, err := windows.UTF16FromString(destinationName)
	if err != nil {
		return err
	}
	name = name[:len(name)-1]
	var layout fileRenameInfo
	offset := unsafe.Offsetof(layout.FileName)
	buffer := make([]byte, int(offset)+len(name)*2)
	info := (*fileRenameInfo)(unsafe.Pointer(&buffer[0]))
	info.RootDirectory = windows.Handle(destinationDir.Fd())
	info.FileNameLength = uint32(len(name) * 2)
	copy(unsafe.Slice(&info.FileName[0], len(name)), name)
	var status windows.IO_STATUS_BLOCK
	return windows.NtSetInformationFile(handle, &status, &buffer[0], uint32(len(buffer)), windows.FileRenameInformation)
}
