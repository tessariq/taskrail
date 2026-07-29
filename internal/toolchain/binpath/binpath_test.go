package binpath_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/toolchain/binpath"
)

// exeName appends the Windows executable extension so a seeded fixture is
// resolvable by a PATH lookup (which honours PATHEXT) on every OS.
func exeName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func seed(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o755); err != nil {
		t.Fatalf("seed %s: %v", p, err)
	}
	return p
}

// Resolve must report the taskrail a bare command finds on PATH, never a
// shadowing binary in the working directory (the Windows ErrDot trap).
func TestResolveIgnoresWorkingDirectory(t *testing.T) {
	pathDir := t.TempDir()
	want := seed(t, pathDir, exeName("taskrail"), "on-path")
	cwd := t.TempDir()
	seed(t, cwd, exeName("taskrail"), "cwd-decoy")

	t.Chdir(cwd)
	t.Setenv("PATH", pathDir)

	got, err := binpath.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	same, err := binpath.SameFile(got, want)
	if err != nil {
		t.Fatalf("SameFile: %v", err)
	}
	if !same {
		t.Errorf("Resolve() = %q, want the on-PATH binary %q", got, want)
	}
}

func TestResolveErrorsWhenAbsent(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := binpath.Resolve(); err == nil {
		t.Error("Resolve must fail when no taskrail is on PATH")
	}
}

// SameFile compares file identity, not path spelling: the install guard must not
// report a mismatch merely because one side is relative and the other absolute.
func TestSameFileIgnoresPathSpelling(t *testing.T) {
	dir := t.TempDir()
	abs := seed(t, dir, "taskrail", "bytes")
	other := seed(t, dir, "other", "bytes")

	t.Chdir(dir)
	same, err := binpath.SameFile("taskrail", abs)
	if err != nil {
		t.Fatalf("SameFile: %v", err)
	}
	if !same {
		t.Error("SameFile must treat a relative and absolute path to one file as the same file")
	}

	same, err = binpath.SameFile(abs, other)
	if err != nil {
		t.Fatalf("SameFile: %v", err)
	}
	if same {
		t.Error("SameFile must distinguish two byte-identical but distinct files")
	}
}

func TestSameFileMissingPath(t *testing.T) {
	dir := t.TempDir()
	a := seed(t, dir, "a", "x")
	if _, err := binpath.SameFile(a, filepath.Join(dir, "gone")); err == nil {
		t.Error("SameFile must return an error for a missing path")
	}
}

// GoVersion reads the toolchain recorded in a Go binary's build info. The test
// binary itself is the cheapest available real Go binary.
func TestGoVersionReadsBuildInfo(t *testing.T) {
	got, err := binpath.GoVersion(os.Args[0])
	if err != nil {
		t.Fatalf("GoVersion: %v", err)
	}
	if !strings.HasPrefix(got, "go1.") {
		t.Errorf("GoVersion(%s) = %q, want a go1.x toolchain version", os.Args[0], got)
	}
}

func TestGoVersionRejectsNonGoFile(t *testing.T) {
	f := seed(t, t.TempDir(), "notgo", "plain text")
	if _, err := binpath.GoVersion(f); err == nil {
		t.Error("GoVersion must fail on a file carrying no Go build info")
	}
}

// The shadow message is the AC-2 distinction: it must say the binary that would
// run is not the one just built, and name the two fixes that actually resolve it
// (mise run setup, or a TASKRAIL override) rather than a rebuild.
func TestShadowedErrorNamesWorkingFixes(t *testing.T) {
	// The message renders the build path absolute in OS-native form, so the
	// expectation is derived the same way rather than written as a literal — a
	// literal POSIX path is not what the message contains on Windows. `resolved`
	// deliberately shares no suffix with `built`, so neither can satisfy the
	// other's assertion by accident.
	built := filepath.Join("bin", "taskrail")
	wantBuilt := absPath(t, built)
	resolved := filepath.Join(string(filepath.Separator)+"opt", "release", "shipped-cli")

	err := binpath.ShadowedError(built, resolved)
	if err == nil {
		t.Fatal("ShadowedError must return an error")
	}
	msg := err.Error()
	for _, want := range []string{wantBuilt, resolved, "mise run setup", "TASKRAIL=" + wantBuilt} {
		if !strings.Contains(msg, want) {
			t.Errorf("ShadowedError message %q must mention %q", msg, want)
		}
	}
	if strings.Contains(msg, "stale") {
		t.Errorf("ShadowedError must not read as staleness; got %q", msg)
	}
}

// absPath resolves path the way the guards' messages do, so an expectation stays
// correct on a platform whose absolute form is not a POSIX path.
func absPath(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("abs %s: %v", path, err)
	}
	return abs
}

// A misdirected override is the wrong binary for a different reason than a PATH
// shadow, and repointing TASKRAIL is what fixes it — naming PATH there would send
// the contributor somewhere that cannot resolve it.
func TestOverrideErrorPointsAtTheOverride(t *testing.T) {
	built := filepath.Join("bin", "taskrail")
	wantBuilt := absPath(t, built)
	target := filepath.Join(string(filepath.Separator)+"opt", "old", "shipped-cli")

	err := binpath.OverrideError(built, target)
	if err == nil {
		t.Fatal("OverrideError must return an error")
	}
	msg := err.Error()
	for _, want := range []string{"TASKRAIL", target, wantBuilt, "repoint"} {
		if !strings.Contains(msg, want) {
			t.Errorf("OverrideError message %q must mention %q", msg, want)
		}
	}
	// An override wins over PATH, so framing this as "a bare `taskrail` runs X"
	// would describe a resolution that is not what happens.
	if strings.Contains(msg, "a bare `taskrail` runs") {
		t.Errorf("an override problem must not be framed as PATH shadowing; got %q", msg)
	}
}

// The shadowed and override verdicts are both stated *against* the working-tree
// build, so an absent one is a distinct cause with a distinct first step: build
// it. Naming a resolution fix there points the reader past what is missing.
func TestMissingBuildErrorNamesTheAbsentBuild(t *testing.T) {
	built := filepath.Join("bin", "taskrail")
	wantBuilt := absPath(t, built)

	err := binpath.MissingBuildError(built, os.ErrNotExist)
	if err == nil {
		t.Fatal("MissingBuildError must return an error")
	}
	msg := err.Error()
	for _, want := range []string{wantBuilt, "task taskrail:install"} {
		if !strings.Contains(msg, want) {
			t.Errorf("MissingBuildError message %q must mention %q", msg, want)
		}
	}
	// The stat failure is the evidence for the verdict; discarding it leaves the
	// reader unable to tell "never built" from "built but unreadable".
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("MissingBuildError must wrap the stat cause; got %q", msg)
	}
	for _, unwanted := range []string{"stale", "a bare `taskrail` runs"} {
		if strings.Contains(msg, unwanted) {
			t.Errorf("an absent build must not be framed as %q; got %q", unwanted, msg)
		}
	}
}

// The TASKRAIL remedy is copy-pasteable guidance that outlives the shell it is
// read in, so a relative build path (which is what the Taskfile passes) must be
// reported absolute — a relative override breaks the moment the caller cd's.
func TestShadowedErrorReportsAnAbsoluteOverride(t *testing.T) {
	dir := t.TempDir()
	built := seed(t, dir, "taskrail", "working-tree build")
	t.Chdir(dir)

	msg := binpath.ShadowedError("taskrail", filepath.Join(string(filepath.Separator)+"opt", "release", "shipped-cli")).Error()
	if !strings.Contains(msg, "TASKRAIL="+built) {
		t.Errorf("message %q must suggest the absolute override TASKRAIL=%s", msg, built)
	}
}
