// Command freshcheck fails loud when the taskrail binary on PATH is not the
// current working-tree build, without relying on external coreutils
// (mktemp/cmp/trap) that are absent on a stock native Windows install. The
// Taskfile builds a fresh binary with the reproducible flags and passes its path
// here alongside the path taskrail:install writes to; this helper resolves the
// on-PATH taskrail, works out why it differs, and removes the throwaway build.
//
// Working out why is the point. A byte difference has three causes with three
// different remedies: the on-PATH binary is a different file (fix PATH), the two
// binaries came from different Go toolchains (build both under one toolchain), or
// the source moved on (rebuild). Reporting all three as "stale" sends the
// contributor to a remedy that cannot resolve what was detected. See Taskfile.yml
// taskrail:check, T-082 and T-123.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/tessariq/taskrail/internal/toolchain/binpath"
)

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run compares the freshly built binary at args[0] against the taskrail resolved
// on PATH, where args[1] is the path taskrail:install writes the working-tree
// build to. It always removes the fresh build (the Taskfile's throwaway) before
// returning so no cleanup trap is needed; a failed removal is reported to warn
// rather than failing the check.
func run(args []string, warn io.Writer) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: freshcheck <fresh-build-path> <working-tree-build-path>")
	}
	fresh, installed := args[0], args[1]
	defer cleanup(fresh, warn)

	// Check the binary the workflows would actually run. They invoke
	// ${TASKRAIL:-taskrail}, and exporting TASKRAIL is one of the two fixes these
	// guards prescribe — checking PATH regardless would fail a contributor for
	// having done exactly what the message told them to.
	target, override := os.LookupEnv("TASKRAIL")
	override = override && target != ""
	if !override {
		var err error
		if target, err = binpath.Resolve(); err != nil {
			return err
		}
	}

	same, err := sameBytes(fresh, target)
	if err != nil {
		if override {
			return fmt.Errorf("TASKRAIL=%s cannot be read: %w;\n"+
				"  repoint it at the working-tree build, or unset it and run 'mise run setup'", target, err)
		}
		return err
	}
	if same {
		return nil
	}

	// Ordered ahead of the verdicts below: both are stated against the working-tree
	// build, so an absent one leaves their remedies pointing at a file that does
	// not exist.
	if _, err := os.Stat(installed); err != nil {
		return binpath.MissingBuildError(installed, err)
	}

	// A byte difference only means "stale" once the binary that would run is the
	// working-tree build; anything else is pointing elsewhere.
	isWorkingTreeBuild, err := binpath.SameFile(target, installed)
	if err != nil {
		return err
	}
	if !isWorkingTreeBuild {
		if override {
			return binpath.OverrideError(installed, target)
		}
		return binpath.ShadowedError(installed, target)
	}
	return differenceError(target, goVersion(fresh), goVersion(target))
}

// goVersion reports the Go toolchain recorded in a binary, or "" when it cannot
// be determined. An unknown toolchain is not evidence of a toolchain difference,
// so callers must compare only two known versions.
func goVersion(path string) string {
	v, err := binpath.GoVersion(path)
	if err != nil {
		return ""
	}
	return v
}

// differenceError explains a byte difference between the fresh build and an
// on-PATH binary already established to be the working-tree build. Identical
// source built by two Go toolchains differs in bytes, so prescribing a rebuild
// there yields an install/check loop that never converges — name the toolchain as
// the variable instead.
func differenceError(resolved, freshGo, resolvedGo string) error {
	if freshGo != "" && resolvedGo != "" && freshGo != resolvedGo {
		return fmt.Errorf("on-PATH taskrail (%s) was built by %s but this check built with %s;\n"+
			"  the Go toolchain is the difference, not the source — rebuilding in this shell will not converge.\n"+
			"  Run both halves under one toolchain: 'mise exec -- task taskrail:install && mise exec -- task taskrail:check'",
			resolved, resolvedGo, freshGo)
	}
	return fmt.Errorf("on-PATH taskrail (%s) is stale versus the working tree; run 'task taskrail:install'", resolved)
}

// cleanup removes the throwaway fresh build. A failed removal (e.g. a Windows
// file lock on the just-built exe) is a hygiene problem, not a freshness signal:
// it warns to warn but never fails the check, so a leftover file cannot flip a
// genuinely fresh binary to "stale". An already-absent file is not a failure.
// The next build overwrites any leftover, so it self-heals.
func cleanup(path string, warn io.Writer) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(warn, "warning: could not remove throwaway build %s: %v\n", path, err)
	}
}

// sameBytes reports whether the two files have identical contents. It reads both
// fully; the taskrail binary is small enough that streaming buys nothing.
func sameBytes(a, b string) (bool, error) {
	da, err := os.ReadFile(a)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", a, err)
	}
	db, err := os.ReadFile(b)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", b, err)
	}
	return bytes.Equal(da, db), nil
}
