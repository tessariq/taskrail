// Command freshcheck fails loud when the taskrail binary on PATH is stale versus
// a freshly built one, without relying on external coreutils (mktemp/cmp/trap)
// that are absent on a stock native Windows install. The Taskfile builds a fresh
// binary with the reproducible flags and passes its path here; this helper
// resolves the on-PATH taskrail, compares the two byte-for-byte, and removes the
// throwaway build. See Taskfile.yml taskrail:check and T-082.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run compares the freshly built binary at args[0] against the taskrail resolved
// on PATH. It always removes the fresh build (the Taskfile's throwaway) before
// returning so no cleanup trap is needed.
func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: freshcheck <fresh-build-path>")
	}
	fresh := args[0]
	defer os.Remove(fresh)

	// exec.LookPath honours PATHEXT on Windows, so a bare "taskrail" resolves the
	// installed taskrail.exe there and plain taskrail on POSIX.
	resolved, err := exec.LookPath("taskrail")
	if err != nil {
		return fmt.Errorf("taskrail is not on PATH; run 'mise run setup' or 'task taskrail:install'")
	}

	same, err := sameBytes(fresh, resolved)
	if err != nil {
		return err
	}
	if !same {
		return fmt.Errorf("on-PATH taskrail (%s) is stale versus the working tree; run 'task taskrail:install'", resolved)
	}
	return nil
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
