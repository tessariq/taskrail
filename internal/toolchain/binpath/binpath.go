// Package binpath answers the two questions the contributor-facing binary guards
// ask about `taskrail`: which binary a bare command actually runs, and how that
// binary relates to the working-tree build.
//
// Keeping both in one place is what lets the install guard and the freshness
// guard prescribe remedies that resolve what they detected: a binary resolved
// from somewhere other than the working-tree build needs a PATH fix, not a
// rebuild, and two builds from different Go toolchains differ in bytes without
// either being stale (T-123, specs/v0.4.0.md#version-skew-detection).
package binpath

import (
	"debug/buildinfo"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Resolve returns the path a bare `taskrail` resolves to on PATH. It sets
// NoDefaultCurrentDirectoryInExePath in the process environment as a side effect,
// which is the only stdlib-sanctioned way to get the behavior described below.
//
// On Windows exec.LookPath probes the working directory first and, when a cwd
// binary shadows a different PATH one, refuses with ErrDot rather than returning
// the PATH match — and CI's build-test job leaves a `taskrail.exe` in the repo
// root from `task build`. These guards only care about the on-PATH binary, so
// opt out of the cwd probe via the documented
// NoDefaultCurrentDirectoryInExePath signal (honoured on Windows; inert on
// POSIX, whose LookPath never scans cwd). LookPath still honours PATHEXT, so a
// bare "taskrail" resolves taskrail.exe on Windows and plain taskrail on POSIX.
func Resolve() (string, error) {
	os.Setenv("NoDefaultCurrentDirectoryInExePath", "1")
	resolved, err := exec.LookPath("taskrail")
	if err != nil {
		return "", fmt.Errorf("taskrail is not on PATH; run 'mise run setup' (or export TASKRAIL=<path to the working-tree build>)")
	}
	return resolved, nil
}

// SameFile reports whether two paths name the same file on disk. Identity, not
// path spelling: the guards compare a relative build output against an absolute
// PATH resolution, and on Windows against a differently-cased one.
func SameFile(a, b string) (bool, error) {
	fa, err := os.Stat(a)
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", a, err)
	}
	fb, err := os.Stat(b)
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", b, err)
	}
	return os.SameFile(fa, fb), nil
}

// GoVersion returns the Go toolchain version recorded in a binary's build info.
// It fails for anything that is not a Go binary, which the callers treat as
// "toolchain unknown" rather than as a signal.
func GoVersion(path string) (string, error) {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read build info from %s: %w", path, err)
	}
	return info.GoVersion, nil
}

// ShadowedError reports that a bare `taskrail` runs something other than the
// working-tree build. The remedy is a PATH fix, never a rebuild: rebuilding
// leaves the caller running the same wrong binary.
//
// built is reported absolute because callers pass the Taskfile's repo-relative
// output path, and a relative TASKRAIL override stops working the moment the
// contributor changes directory.
func ShadowedError(built, resolved string) error {
	abs := absolute(built)
	return fmt.Errorf("a bare `taskrail` runs %s, not the working-tree build %s;\n"+
		"  fix the resolution, not the build: run 'mise run setup' (puts ./bin on PATH),\n"+
		"  or export TASKRAIL=%s for this shell", resolved, abs, abs)
}

// OverrideError reports that an explicit TASKRAIL override points at something
// other than the working-tree build. PATH is not the variable here — the override
// wins over it — so naming a PATH remedy would send the caller somewhere that
// cannot resolve what was detected.
func OverrideError(built, target string) error {
	abs := absolute(built)
	return fmt.Errorf("TASKRAIL points at %s, not the working-tree build %s;\n"+
		"  repoint it: export TASKRAIL=%s (or unset TASKRAIL and run 'mise run setup')", target, abs, abs)
}

// MissingBuildError reports that the working-tree build itself is absent (or
// cannot be stat'd). Folding this into the shadowed or override verdicts would
// prescribe a resolution fix for a binary that was never produced, skipping the
// step actually missing. The stat cause is wrapped because "never built" and
// "built but unreadable" want different next moves.
func MissingBuildError(built string, cause error) error {
	abs := absolute(built)
	return fmt.Errorf("the working-tree build %s does not exist: %w;\n"+
		"  build it first: run 'task taskrail:install' (or 'mise run setup', which also puts ./bin on PATH)", abs, cause)
}

// absolute resolves path against the working directory, falling back to the
// input when that is not possible — a message is worth printing either way.
func absolute(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
