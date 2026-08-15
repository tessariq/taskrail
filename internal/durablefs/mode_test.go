package durablefs

import (
	"io/fs"
	"runtime"
	"testing"
)

func TestPortableModeMatchesNativeFilesystemSemantics(t *testing.T) {
	wantWritable, wantReadOnly := fs.FileMode(0o640), fs.FileMode(0o440)
	if runtime.GOOS == "windows" {
		wantWritable, wantReadOnly = 0o666, 0o444
	}
	if got := PortableMode(0o640); got != wantWritable {
		t.Fatalf("writable mode = %o, want %o", got, wantWritable)
	}
	if got := PortableMode(0o440); got != wantReadOnly {
		t.Fatalf("read-only mode = %o, want %o", got, wantReadOnly)
	}
}
