package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestSameBytes(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", []byte("taskrail-binary-bytes"))
	same := writeFile(t, dir, "same", []byte("taskrail-binary-bytes"))
	diffLen := writeFile(t, dir, "difflen", []byte("taskrail-binary"))
	diffByte := writeFile(t, dir, "diffbyte", []byte("taskrail-binary-byteS"))

	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"identical", a, same, true},
		{"different length", a, diffLen, false},
		{"same length differing byte", a, diffByte, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sameBytes(tc.a, tc.b)
			if err != nil {
				t.Fatalf("sameBytes: %v", err)
			}
			if got != tc.want {
				t.Errorf("sameBytes(%s, %s) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestSameBytesMissingFile(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", []byte("x"))
	if _, err := sameBytes(a, filepath.Join(dir, "nope")); err == nil {
		t.Error("sameBytes with missing file must return an error")
	}
}

// run must remove the throwaway fresh build even when the on-PATH taskrail is
// absent, so no cleanup trap is needed in the Taskfile.
func TestRunRemovesFreshBuildWhenNotOnPath(t *testing.T) {
	dir := t.TempDir()
	fresh := writeFile(t, dir, "fresh", []byte("bytes"))
	t.Setenv("PATH", dir) // no taskrail here

	if err := run([]string{fresh}); err == nil {
		t.Error("run must fail when taskrail is not on PATH")
	}
	if _, err := os.Stat(fresh); !os.IsNotExist(err) {
		t.Errorf("run must remove the fresh build; stat err = %v", err)
	}
}

func TestRunRejectsBadArgs(t *testing.T) {
	if err := run(nil); err == nil {
		t.Error("run must reject a missing path argument")
	}
}
