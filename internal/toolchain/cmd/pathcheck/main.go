// Command pathcheck fails loud when a successful `task taskrail:install` left
// the caller no better off: the build landed somewhere a bare `taskrail` does
// not resolve from, so the workflows' ${TASKRAIL:-taskrail} still runs a
// different binary. Detecting that at install time is what keeps the trap from
// surfacing later as an unknown flag — or, worse, as tracked-work state written
// by an older binary. See Taskfile.yml taskrail:install and T-123.
package main

import (
	"fmt"
	"os"

	"github.com/tessariq/taskrail/internal/toolchain/binpath"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run verifies that a bare `taskrail` resolves to the binary just built at
// args[0]. An explicit TASKRAIL override is one of the two sanctioned fixes, so
// it short-circuits the PATH question entirely.
func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: pathcheck <built-binary-path>")
	}
	if os.Getenv("TASKRAIL") != "" {
		return nil
	}
	built := args[0]

	resolved, err := binpath.Resolve()
	if err != nil {
		// Resolve's message already names both sanctioned fixes.
		return err
	}
	same, err := binpath.SameFile(resolved, built)
	if err != nil {
		return err
	}
	if !same {
		return binpath.ShadowedError(built, resolved)
	}
	return nil
}
