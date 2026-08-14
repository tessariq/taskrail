package taskrail

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// The active storage context: which physical root the layout's logical
// directories resolve beneath. Committed repositories keep them at the
// repository root; local mode (specs/v0.5.0.md#local-planning-mode) maps the same
// logical namespace beneath one fixed ignored overlay root.
//
// Discovery supplies this context from the layout marker. Bootstrapping the
// overlay and proving its paths ignored remain separate local-mode outcomes.

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
	switch c.Mode {
	case StorageLocal:
		return path.Join(localStorageRoot, logical)
	case StorageCommitted:
		return logical
	default:
		return ""
	}
}

func (c StorageContext) validate() error {
	switch {
	case c.Mode == StorageCommitted && c.Root == committedStorageRoot:
		return nil
	case c.Mode == StorageLocal && c.Root == localStorageRoot:
		return nil
	default:
		return WithMachineErrorCode(MachineCodeUnsupported,
			fmt.Errorf("unsupported storage context mode %q root %q", c.Mode, c.Root))
	}
}

func (p Paths) ensureStorageCapability() error {
	return p.Storage.validate()
}

// physicalManagedPath resolves a durable logical identity through the active
// context. It accepts only configured semantic namespaces and never probes the
// inactive storage root.
func (p Paths) physicalManagedPath(logical string) (string, error) {
	if err := p.ensureStorageCapability(); err != nil {
		return "", err
	}
	logical = path.Clean(filepath.ToSlash(logical))
	for _, namespace := range []struct {
		logical  string
		physical string
	}{
		{p.LogicalSpecsDir, p.SpecsDir},
		{p.LogicalPlanningDir, p.PlanningDir},
		{p.LogicalPromptsDir, p.PromptsDir},
	} {
		if logical == namespace.logical {
			return namespace.physical, nil
		}
		prefix := namespace.logical + "/"
		if strings.HasPrefix(logical, prefix) {
			return filepath.Join(namespace.physical, filepath.FromSlash(strings.TrimPrefix(logical, prefix))), nil
		}
	}
	return "", WithMachineErrorCode(MachineCodeRepositoryInvalid,
		fmt.Errorf("managed path %q is outside configured semantic namespaces", logical))
}

func (p Paths) physicalSpecPath(logical string) (string, error) {
	clean := path.Clean(filepath.ToSlash(logical))
	if clean != p.LogicalSpecsDir && !strings.HasPrefix(clean, p.LogicalSpecsDir+"/") {
		return "", WithMachineErrorCode(MachineCodeRepositoryInvalid,
			fmt.Errorf("spec path %q is outside configured specs directory %q", clean, p.LogicalSpecsDir))
	}
	return p.physicalManagedPath(clean)
}

// logicalManagedPath converts a context-resolved physical semantic path back to
// its durable repository identity for models, diagnostics, and command results.
func (p Paths) logicalManagedPath(physical string) string {
	for _, namespace := range []struct {
		logical  string
		physical string
	}{
		{p.LogicalSpecsDir, p.SpecsDir},
		{p.LogicalPlanningDir, p.PlanningDir},
		{p.LogicalPromptsDir, p.PromptsDir},
	} {
		rel, err := filepath.Rel(namespace.physical, physical)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if rel == "." {
			return namespace.logical
		}
		return path.Join(namespace.logical, filepath.ToSlash(rel))
	}
	return relPath(p.RepoRoot, physical)
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
