package taskrail

import "fmt"

// requireCurrentLayout refuses one semantic writer on a repository that still
// records the legacy layout. Task-local loop policy, verification IDs, and the
// schema-2 state references v0.5 introduces are invisible to an older binary,
// which rewrites task frontmatter from its own typed struct and silently drops
// them. Layout 1 therefore permits read-only inspection and init migration
// only, so no v0.5 field ever reaches a repository an older writer will accept
// (specs/v0.5.0.md#layout-compatibility-and-upgrade).
//
// The check runs before the writer reads or publishes any byte. It asks the
// discovered layout rather than re-reading the marker, so a writer can never
// be admitted against a version other than the one that resolved the paths it
// is about to write. An unmarked repository resolves to the legacy default and
// is refused for the same reason a legacy marker is.
func (s *Service) requireCurrentLayout(command string) error {
	if s.paths.LayoutVersion == currentLayoutVersion {
		return nil
	}
	return WithMachineErrorCode(MachineCodeIncompatibleLayout,
		fmt.Errorf("%s requires layout_version %d; upgrade this repository with taskrail init --apply --confirm-quiescent",
			command, currentLayoutVersion))
}
