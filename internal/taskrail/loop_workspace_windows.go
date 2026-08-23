//go:build windows

package taskrail

import (
	"fmt"
	"os"
)

// Go does not expose the invoking token's SID through FileInfo. Rejecting an
// explicit root is safer than claiming ACL ownership we cannot establish.
func validateParallelWorkspaceOwnership(_ os.FileInfo) error {
	return fmt.Errorf("--workspace-root ownership cannot be verified on Windows")
}
