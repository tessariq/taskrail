package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// exeName appends the Windows executable extension so a seeded fixture is
// resolvable by exec.LookPath (which honours PATHEXT) on every OS.
func exeName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

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

	if err := run([]string{fresh}, io.Discard); err == nil {
		t.Error("run must fail when taskrail is not on PATH")
	}
	if _, err := os.Stat(fresh); !os.IsNotExist(err) {
		t.Errorf("run must remove the fresh build; stat err = %v", err)
	}
}

func TestRunRejectsBadArgs(t *testing.T) {
	if err := run(nil, io.Discard); err == nil {
		t.Error("run must reject a missing path argument")
	}
}

// run must route cleanup through the warning path, not swallow the removal
// error: when the throwaway build cannot be removed, the warning must reach
// run's writer. Guards against a regression back to a bare `defer os.Remove`.
func TestRunWarnsWhenCleanupFails(t *testing.T) {
	dir := t.TempDir()
	nonEmpty := filepath.Join(dir, "sub")
	if err := os.Mkdir(nonEmpty, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, nonEmpty, "child", []byte("x")) // makes os.Remove(nonEmpty) fail
	t.Setenv("PATH", dir)                        // no taskrail: run returns an error, still cleans up

	var warn bytes.Buffer
	_ = run([]string{nonEmpty}, &warn)
	if !strings.Contains(warn.String(), "warning") {
		t.Errorf("run must surface the cleanup warning; wrote %q", warn.String())
	}
}

// run must resolve the taskrail on PATH, never a shadowing binary in the working
// directory. On Windows this reproduces the ErrDot failure that broke the CI
// freshness leg (a cwd taskrail.exe from `task build` differing from the PATH
// one): without the NoDefaultCurrentDirectoryInExePath opt-out, exec.LookPath
// returns "cannot run executable found relative to current directory" and run
// errors. On POSIX LookPath ignores cwd, so this pins the PATH-not-cwd contract.
func TestRunResolvesPathNotCwd(t *testing.T) {
	pathDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pathDir, exeName("taskrail")), []byte("on-path"), 0o755); err != nil {
		t.Fatalf("seed on-PATH taskrail: %v", err)
	}
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, exeName("taskrail")), []byte("cwd-decoy-differs"), 0o755); err != nil {
		t.Fatalf("seed cwd decoy: %v", err)
	}
	fresh := writeFile(t, t.TempDir(), "fresh", []byte("on-path")) // matches the on-PATH bytes

	t.Chdir(cwd)
	t.Setenv("PATH", pathDir)
	if err := run([]string{fresh}, io.Discard); err != nil {
		t.Errorf("run must resolve and match the on-PATH taskrail, ignoring the cwd decoy; got %v", err)
	}
}

// cleanup must delete the throwaway build and stay silent on success.
func TestCleanupRemovesAndIsSilentOnSuccess(t *testing.T) {
	dir := t.TempDir()
	fresh := writeFile(t, dir, "fresh", []byte("bytes"))
	var warn bytes.Buffer
	cleanup(fresh, &warn)
	if _, err := os.Stat(fresh); !os.IsNotExist(err) {
		t.Errorf("cleanup must remove the fresh build; stat err = %v", err)
	}
	if warn.Len() != 0 {
		t.Errorf("cleanup must stay silent on success; wrote %q", warn.String())
	}
}

// An already-removed build is not a failure: cleanup must not warn about it.
func TestCleanupSilentWhenAlreadyGone(t *testing.T) {
	dir := t.TempDir()
	var warn bytes.Buffer
	cleanup(filepath.Join(dir, "gone"), &warn)
	if warn.Len() != 0 {
		t.Errorf("cleanup must not warn on a missing file; wrote %q", warn.String())
	}
}

// A removal that genuinely fails (here: a non-empty directory, which os.Remove
// refuses on POSIX and Windows alike) must warn to the writer rather than fail
// the check — a leftover file cannot flip a fresh binary to stale.
func TestCleanupWarnsOnRemoveFailure(t *testing.T) {
	dir := t.TempDir()
	nonEmpty := filepath.Join(dir, "sub")
	if err := os.Mkdir(nonEmpty, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, nonEmpty, "child", []byte("x")) // makes os.Remove(nonEmpty) fail
	var warn bytes.Buffer
	cleanup(nonEmpty, &warn)
	if !strings.Contains(warn.String(), "warning") {
		t.Errorf("cleanup must warn on a failed removal; wrote %q", warn.String())
	}
}
