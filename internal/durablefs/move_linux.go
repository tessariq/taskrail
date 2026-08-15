//go:build linux

package durablefs

import (
	"os"

	"golang.org/x/sys/unix"
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
	return unix.Renameat2(int(sourceDir.Fd()), sourceName, int(destinationDir.Fd()), destinationName, unix.RENAME_NOREPLACE)
}
