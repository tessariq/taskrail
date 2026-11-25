//go:build windows

package durablefs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestClassifyDirectorySyncAccessDeniedAsUnsupported(t *testing.T) {
	err := classifySyncError(BarrierDirectory, windows.ERROR_ACCESS_DENIED)
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("access-denied directory sync = %v, want ErrUnsupported", err)
	}
	if err := classifySyncError(BarrierDirectory, windows.ERROR_WRITE_FAULT); errors.Is(err, ErrUnsupported) {
		t.Fatalf("write fault classified unsupported: %v", err)
	}
}

// The directory barrier must really flush on NTFS, where MS-FSA 2.1.5.7 note
// <80> says a directory flush persists directory structure, and must refuse
// elsewhere because other file systems report success without flushing.
func TestDirectoryBarrierFlushesOnlyNTFS(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	err = barrier(BarrierDirectory, func() error { return syncDirectory(root) })
	if volumeFileSystem(t, dir) == "NTFS" {
		if err != nil {
			t.Fatalf("NTFS directory barrier = %v, want success", err)
		}
		return
	}
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("non-NTFS directory barrier = %v, want ErrUnsupported", err)
	}
}

// A share can report "NTFS" while acknowledging flushes it never syncs, so a
// directory reached over SMB must never pass the barrier, even when it is local disk.
func TestDirectoryBarrierRefusesNetworkVolume(t *testing.T) {
	local := t.TempDir()
	share := `\\localhost\` + strings.TrimSuffix(filepath.VolumeName(local), ":") + `$` + local[len(filepath.VolumeName(local)):]
	root, err := os.OpenRoot(share)
	if err != nil {
		t.Skipf("administrative loopback share unavailable: %v", err)
	}
	defer root.Close()
	err = barrier(BarrierDirectory, func() error { return syncDirectory(root) })
	if err == nil {
		t.Fatal("network directory barrier succeeded, want refusal")
	}
}

// A caller that may not add entries to the directory gets no flushable handle;
// the barrier must refuse rather than succeed through privilege or by skipping.
func TestDirectoryBarrierRefusesWithoutAddRights(t *testing.T) {
	dir := t.TempDir()
	descriptor, err := windows.SecurityDescriptorFromString("D:P(D;;0x6;;;WD)(A;OICI;FA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	err = barrier(BarrierDirectory, func() error { return syncDirectory(root) })
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("directory barrier without add rights = %v, want ErrUnsupported", err)
	}
}

func volumeFileSystem(t *testing.T, path string) string {
	t.Helper()
	volume, err := windows.UTF16PtrFromString(filepath.VolumeName(path) + `\`)
	if err != nil {
		t.Fatal(err)
	}
	name := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumeInformation(volume, nil, 0, nil, nil, nil, &name[0], uint32(len(name))); err != nil {
		t.Fatal(err)
	}
	return windows.UTF16ToString(name)
}

func TestWindowsSnapshotsCanonicalizeWritableAndReadOnlyModes(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct {
		name string
		mode os.FileMode
		want os.FileMode
	}{
		{name: "writable", mode: 0o600, want: 0o666},
		{name: "read-only", mode: 0o400, want: 0o444},
	} {
		path := filepath.Join(root, test.name)
		if err := os.WriteFile(path, []byte(test.name), test.mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, test.mode); err != nil {
			t.Fatal(err)
		}
		_, snapshot, err := ReadFile(root, test.name, 1024)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Mode != test.want {
			t.Errorf("%s snapshot mode = %o, want %o", test.name, snapshot.Mode, test.want)
		}
	}
}
