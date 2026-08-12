package taskrail

import "path"

// The active storage context: which physical root the layout's logical
// directories resolve beneath. Committed repositories keep them at the
// repository root; local mode (specs/v0.5.0.md#local-planning-mode) maps the same
// logical namespace beneath one fixed ignored overlay root.
//
// Only the reporting half lives here. Discovering a local marker, bootstrapping
// the overlay, and proving its paths ignored belong to the local-mode tasks; a
// context is supplied explicitly until then, which is why every consumer takes it
// as data rather than re-deriving it from a marker.

// StorageMode is the companion's `committed | local` enum.
type StorageMode string

const (
	StorageCommitted StorageMode = "committed"
	StorageLocal     StorageMode = "local"
)

// localStorageRoot is the one repository-root-relative physical root local mode
// uses. It is fixed by the feature spec, not configurable.
const localStorageRoot = ".taskrail/local"

// committedStorageRoot is `.` rather than the empty string: the companion fixes
// that exact value for `StatusResult.storage.root` in committed mode.
const committedStorageRoot = "."

// StorageContext is one active storage snapshot. Root is the repository-root
// relative physical directory the logical layout resolves beneath, so a reporter
// never reconstructs it from the mode and a future mode cannot be inferred wrong.
type StorageContext struct {
	Mode StorageMode
	Root string
}

func committedStorage() StorageContext {
	return StorageContext{Mode: StorageCommitted, Root: committedStorageRoot}
}

func localStorage() StorageContext {
	return StorageContext{Mode: StorageLocal, Root: localStorageRoot}
}

// physical maps one logical repository-relative directory onto its physical
// repository-relative location in this context. Callers hold logical paths; this
// is the only place the overlay prefix is applied. Committed mode prefixes
// nothing, so physical and logical paths coincide there and existing
// repositories are unaffected.
func (c StorageContext) physical(logical string) string {
	if c.Mode == StorageLocal {
		return path.Join(localStorageRoot, logical)
	}
	return logical
}

// storageSnapshot reports the context the service's paths were resolved through.
// The artifacts directory is read back from those paths rather than recomputed,
// so the reported location is the one verify and manual testing actually use.
func (s *Service) storageSnapshot() StatusStorage {
	return StatusStorage{
		Mode:         string(s.paths.Storage.Mode),
		Root:         s.paths.Storage.Root,
		ArtifactsDir: relPath(s.paths.RepoRoot, s.paths.ArtifactsDir),
	}
}
