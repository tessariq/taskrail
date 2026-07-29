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

// seedTaskrail writes a `taskrail` fixture into dir. The exec bit and the
// Windows .exe suffix are what let a PATH lookup resolve it when dir is on PATH.
func seedTaskrail(t *testing.T, dir string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, exeName("taskrail"))
	if err := os.WriteFile(p, data, 0o755); err != nil {
		t.Fatalf("seed taskrail in %s: %v", dir, err)
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
	t.Setenv("TASKRAIL", "")
	t.Setenv("PATH", dir) // no taskrail here

	if err := run([]string{fresh, filepath.Join(dir, exeName("taskrail"))}, io.Discard); err == nil {
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
	// The working-tree build path is what separates "stale" from "shadowed", so
	// the old single-argument form must not silently keep working.
	if err := run([]string{"fresh"}, io.Discard); err == nil {
		t.Error("run must reject a missing working-tree build path argument")
	}
}

// AC-2: a byte difference against a taskrail resolved from somewhere other than
// the working-tree build is a PATH problem, not staleness. Reporting it as stale
// sends the contributor to a rebuild that leaves the same binary in front of them.
func TestRunReportsShadowingRatherThanStale(t *testing.T) {
	pathDir := t.TempDir()
	seedTaskrail(t, pathDir, []byte("installed release"))
	installed := seedTaskrail(t, t.TempDir(), []byte("working-tree build"))
	fresh := writeFile(t, t.TempDir(), "fresh", []byte("working-tree build"))

	t.Setenv("TASKRAIL", "")
	t.Setenv("PATH", pathDir)
	err := run([]string{fresh, installed}, io.Discard)
	if err == nil {
		t.Fatal("run must fail when the on-PATH taskrail is not the working-tree build")
	}
	if strings.Contains(err.Error(), "stale") {
		t.Errorf("a shadowed binary must not be reported as stale; got %q", err)
	}
	if !strings.Contains(err.Error(), "mise run setup") {
		t.Errorf("a shadowed binary must name the PATH remedy; got %q", err)
	}
}

// The complement: once the on-PATH taskrail *is* the working-tree build, a byte
// difference from equally-built sources is genuine staleness and a rebuild fixes it.
func TestRunReportsStaleForTheWorkingTreeBuild(t *testing.T) {
	pathDir := t.TempDir()
	installed := seedTaskrail(t, pathDir, []byte("old build"))
	fresh := writeFile(t, t.TempDir(), "fresh", []byte("new build"))

	t.Setenv("TASKRAIL", "")
	t.Setenv("PATH", pathDir)
	err := run([]string{fresh, installed}, io.Discard)
	if err == nil {
		t.Fatal("run must fail when the working-tree build is stale")
	}
	if !strings.Contains(err.Error(), "stale") || !strings.Contains(err.Error(), "task taskrail:install") {
		t.Errorf("a stale working-tree build must name the rebuild remedy; got %q", err)
	}
}

// The guards prescribe `export TASKRAIL=<build>` as one of two working fixes, so
// the freshness guard has to honour it: ${TASKRAIL:-taskrail} is what the skills
// execute, and checking PATH instead would fail a contributor who did exactly
// what the message told them to.
func TestRunChecksTheOverrideRatherThanPath(t *testing.T) {
	pathDir := t.TempDir()
	seedTaskrail(t, pathDir, []byte("installed release"))
	installed := seedTaskrail(t, t.TempDir(), []byte("working-tree build"))
	fresh := writeFile(t, t.TempDir(), "fresh", []byte("working-tree build"))

	t.Setenv("PATH", pathDir)
	t.Setenv("TASKRAIL", installed)
	if err := run([]string{fresh, installed}, io.Discard); err != nil {
		t.Errorf("run must check the TASKRAIL override, not PATH; got %v", err)
	}
}

// An override left pointing at some other binary is still the wrong binary — but
// the remedy is to repoint TASKRAIL, not to fix PATH, so it must not be reported
// as a PATH problem.
func TestRunReportsAStaleOverride(t *testing.T) {
	pathDir := t.TempDir()
	current := seedTaskrail(t, pathDir, []byte("working-tree build"))
	override := writeFile(t, t.TempDir(), "taskrail-old", []byte("installed release"))
	fresh := writeFile(t, t.TempDir(), "fresh", []byte("working-tree build"))

	t.Setenv("PATH", pathDir)
	t.Setenv("TASKRAIL", override)
	err := run([]string{fresh, current}, io.Discard)
	if err == nil {
		t.Fatal("run must fail when TASKRAIL points at something other than the working-tree build")
	}
	if !strings.Contains(err.Error(), "TASKRAIL") {
		t.Errorf("a misdirected override must name TASKRAIL as the thing to fix; got %q", err)
	}
	if strings.Contains(err.Error(), "stale") {
		t.Errorf("a misdirected override is not staleness; got %q", err)
	}
}

// An override that points nowhere is the same class of contributor mistake as
// one pointing at the wrong binary, so it must name TASKRAIL rather than surface
// a bare open error the reader cannot act on.
func TestRunReportsAnUnreadableOverride(t *testing.T) {
	pathDir := t.TempDir()
	current := seedTaskrail(t, pathDir, []byte("working-tree build"))
	fresh := writeFile(t, t.TempDir(), "fresh", []byte("working-tree build"))

	t.Setenv("PATH", pathDir)
	t.Setenv("TASKRAIL", filepath.Join(t.TempDir(), "gone"))
	err := run([]string{fresh, current}, io.Discard)
	if err == nil {
		t.Fatal("run must fail when TASKRAIL points at a path it cannot read")
	}
	if !strings.Contains(err.Error(), "TASKRAIL") {
		t.Errorf("an unreadable override must name TASKRAIL as the cause; got %q", err)
	}
}

// A build that was never produced is not a PATH shadow: the first step is
// taskrail:install, and reporting a resolution fix skips it.
func TestRunReportsAMissingWorkingTreeBuild(t *testing.T) {
	pathDir := t.TempDir()
	seedTaskrail(t, pathDir, []byte("installed release"))
	installed := filepath.Join(t.TempDir(), exeName("taskrail")) // taskrail:install never ran
	fresh := writeFile(t, t.TempDir(), "fresh", []byte("working-tree build"))

	t.Setenv("TASKRAIL", "")
	t.Setenv("PATH", pathDir)
	err := run([]string{fresh, installed}, io.Discard)
	if err == nil {
		t.Fatal("run must fail when the working-tree build does not exist")
	}
	if !strings.Contains(err.Error(), "task taskrail:install") {
		t.Errorf("an absent working-tree build must name the build remedy; got %q", err)
	}
	if strings.Contains(err.Error(), "a bare `taskrail` runs") {
		t.Errorf("an absent working-tree build is not a PATH shadow; got %q", err)
	}
}

// The same holds under an override: repointing TASKRAIL at a build that was
// never produced is not a remedy, so the absent build outranks that verdict.
func TestRunReportsAMissingWorkingTreeBuildUnderAnOverride(t *testing.T) {
	pathDir := t.TempDir()
	seedTaskrail(t, pathDir, []byte("on-path build"))
	override := writeFile(t, t.TempDir(), "taskrail-old", []byte("installed release"))
	installed := filepath.Join(t.TempDir(), exeName("taskrail")) // taskrail:install never ran
	fresh := writeFile(t, t.TempDir(), "fresh", []byte("working-tree build"))

	t.Setenv("PATH", pathDir)
	t.Setenv("TASKRAIL", override)
	err := run([]string{fresh, installed}, io.Discard)
	if err == nil {
		t.Fatal("run must fail when the working-tree build does not exist")
	}
	if !strings.Contains(err.Error(), "task taskrail:install") {
		t.Errorf("an absent working-tree build must name the build remedy; got %q", err)
	}
	if strings.Contains(err.Error(), "repoint") {
		t.Errorf("repointing at a build that does not exist is not a remedy; got %q", err)
	}
}

// AC-3: identical source built by two Go toolchains differs in bytes without
// being stale, and rerunning the rebuild in the same shell never converges. The
// message must name the toolchain as the variable.
func TestDifferenceErrorNamesToolchainWhenBuildersDiffer(t *testing.T) {
	err := differenceError("/repo/bin/taskrail", "go1.26.5", "go1.26.0")
	if err == nil {
		t.Fatal("differenceError must return an error")
	}
	msg := err.Error()
	for _, want := range []string{"go1.26.5", "go1.26.0", "toolchain", "mise exec"} {
		if !strings.Contains(msg, want) {
			t.Errorf("toolchain-difference message %q must mention %q", msg, want)
		}
	}
	if strings.Contains(msg, "stale") {
		t.Errorf("a toolchain difference must not be reported as staleness; got %q", msg)
	}
}

// An unknown toolchain on either side is not evidence of a toolchain difference,
// so the guard must fall back to the staleness verdict rather than invent one.
func TestDifferenceErrorFallsBackToStale(t *testing.T) {
	cases := map[string][2]string{
		"same toolchain":   {"go1.26.5", "go1.26.5"},
		"fresh unknown":    {"", "go1.26.0"},
		"resolved unknown": {"go1.26.5", ""},
		"both unknown":     {"", ""},
	}
	for name, versions := range cases {
		t.Run(name, func(t *testing.T) {
			err := differenceError("/repo/bin/taskrail", versions[0], versions[1])
			if err == nil || !strings.Contains(err.Error(), "stale") {
				t.Errorf("want a staleness verdict, got %v", err)
			}
		})
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
	t.Setenv("TASKRAIL", "")
	t.Setenv("PATH", dir) // no taskrail: run returns an error, still cleans up

	var warn bytes.Buffer
	_ = run([]string{nonEmpty, filepath.Join(dir, exeName("taskrail"))}, &warn)
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
	installed := seedTaskrail(t, pathDir, []byte("on-path"))
	cwd := t.TempDir()
	seedTaskrail(t, cwd, []byte("cwd-decoy-differs"))
	fresh := writeFile(t, t.TempDir(), "fresh", []byte("on-path")) // matches the on-PATH bytes

	t.Chdir(cwd)
	t.Setenv("TASKRAIL", "")
	t.Setenv("PATH", pathDir)
	if err := run([]string{fresh, installed}, io.Discard); err != nil {
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
