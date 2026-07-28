package binpath_test

import (
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
	err := binpath.ShadowedError("bin/taskrail", "/usr/local/bin/taskrail")
	if err == nil {
		t.Fatal("ShadowedError must return an error")
	}
	msg := err.Error()
	for _, want := range []string{"bin/taskrail", "/usr/local/bin/taskrail", "mise run setup", "TASKRAIL"} {
		if !strings.Contains(msg, want) {
			t.Errorf("ShadowedError message %q must mention %q", msg, want)
		}
	}
	if strings.Contains(msg, "stale") {
		t.Errorf("ShadowedError must not read as staleness; got %q", msg)
	}
}

// A misdirected override is the wrong binary for a different reason than a PATH
// shadow, and repointing TASKRAIL is what fixes it — naming PATH there would send
// the contributor somewhere that cannot resolve it.
func TestOverrideErrorPointsAtTheOverride(t *testing.T) {
	err := binpath.OverrideError("/repo/bin/taskrail", "/opt/old/taskrail")
	if err == nil {
		t.Fatal("OverrideError must return an error")
	}
	msg := err.Error()
	for _, want := range []string{"TASKRAIL", "/opt/old/taskrail", "/repo/bin/taskrail", "repoint"} {
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

// The TASKRAIL remedy is copy-pasteable guidance that outlives the shell it is
// read in, so a relative build path (which is what the Taskfile passes) must be
// reported absolute — a relative override breaks the moment the caller cd's.
func TestShadowedErrorReportsAnAbsoluteOverride(t *testing.T) {
	dir := t.TempDir()
	built := seed(t, dir, "taskrail", "working-tree build")
	t.Chdir(dir)

	msg := binpath.ShadowedError("taskrail", "/usr/local/bin/taskrail").Error()
	if !strings.Contains(msg, "TASKRAIL="+built) {
		t.Errorf("message %q must suggest the absolute override TASKRAIL=%s", msg, built)
	}
}
