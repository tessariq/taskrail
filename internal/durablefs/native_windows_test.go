//go:build windows

package durablefs

import (
	"errors"
	"os"
	"path/filepath"
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
