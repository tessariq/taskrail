package taskrail

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const currentLayoutVersion = 1

const (
	defaultSpecsDir    = "specs"
	defaultPlanningDir = "planning"

	taskrailConfigDir  = ".taskrail"
	taskrailConfigFile = "config.yml"
)

func DiscoverPaths(start string) (Paths, error) {
	root, err := findRepoRoot(start)
	if err != nil {
		return Paths{}, err
	}

	cfg, err := loadLayoutConfig(root)
	if err != nil {
		return Paths{}, err
	}

	return pathsFromLayout(root, cfg), nil
}

// defaultLayoutConfig is the hardcoded v0.1.0 layout used when no marker exists.
func defaultLayoutConfig() LayoutConfig {
	return LayoutConfig{
		LayoutVersion: currentLayoutVersion,
		SpecsDir:      defaultSpecsDir,
		PlanningDir:   defaultPlanningDir,
	}
}

// loadLayoutConfig reads `.taskrail/config.yml` if present, falling back to the
// default layout when it is absent so discovery stays purely additive. Fields
// omitted from an existing marker default to the v0.1.0 locations.
func loadLayoutConfig(root string) (LayoutConfig, error) {
	path := filepath.Join(root, taskrailConfigDir, taskrailConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultLayoutConfig(), nil
		}
		return LayoutConfig{}, fmt.Errorf("read layout config %s: %w", relPath(root, path), fsCause(err))
	}

	cfg := defaultLayoutConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return LayoutConfig{}, fmt.Errorf("parse layout config %s: %w", path, err)
	}
	if cfg.SpecsDir == "" {
		cfg.SpecsDir = defaultSpecsDir
	}
	if cfg.PlanningDir == "" {
		cfg.PlanningDir = defaultPlanningDir
	}
	if err := ensureWithinRoot(root, "specs_dir", cfg.SpecsDir); err != nil {
		return LayoutConfig{}, err
	}
	if err := ensureWithinRoot(root, "planning_dir", cfg.PlanningDir); err != nil {
		return LayoutConfig{}, err
	}
	return cfg, nil
}

func markerPath(root string) string {
	return filepath.Join(root, taskrailConfigDir, taskrailConfigFile)
}

// readMarker reads the `.taskrail/config.yml` marker and reports whether it
// exists. Unlike loadLayoutConfig it does not synthesize a default when the
// marker is absent, so callers can distinguish an unmarked repository (needing
// fresh init or legacy adoption) from a marked one.
func readMarker(root string) (LayoutConfig, bool, error) {
	data, err := os.ReadFile(markerPath(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return LayoutConfig{}, false, nil
		}
		return LayoutConfig{}, false, fmt.Errorf("read layout marker %s: %w", relPath(root, markerPath(root)), fsCause(err))
	}
	cfg := defaultLayoutConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return LayoutConfig{}, false, fmt.Errorf("parse layout marker %s: %w", markerPath(root), err)
	}
	return cfg, true, nil
}

// writeMarker persists the layout marker, creating `.taskrail/` if needed.
func writeMarker(root string, cfg LayoutConfig) error {
	if err := os.MkdirAll(filepath.Join(root, taskrailConfigDir), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", taskrailConfigDir, fsCause(err))
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal layout marker: %w", err)
	}
	if err := os.WriteFile(markerPath(root), data, 0o644); err != nil {
		return fmt.Errorf("write layout marker %s: %w", relPath(root, markerPath(root)), fsCause(err))
	}
	return nil
}

// ensureWithinRoot rejects marker locations that resolve outside the repository
// root (e.g. `../../etc`), so an untrusted config cannot redirect discovery to
// arbitrary filesystem paths.
func ensureWithinRoot(root, field, rel string) error {
	within, err := filepath.Rel(root, filepath.Join(root, rel))
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
		return fmt.Errorf("layout config %s %q escapes repository root", field, rel)
	}
	return nil
}

func pathsFromLayout(root string, cfg LayoutConfig) Paths {
	planningDir := filepath.Join(root, cfg.PlanningDir)
	artifactsDir := filepath.Join(planningDir, "artifacts")

	return Paths{
		RepoRoot:     root,
		SpecsDir:     filepath.Join(root, cfg.SpecsDir),
		PlanningDir:  planningDir,
		TasksDir:     filepath.Join(planningDir, "tasks"),
		ArtifactsDir: artifactsDir,
		VerifyDir:    filepath.Join(artifactsDir, "verify"),
		StateFile:    filepath.Join(planningDir, "STATE.md"),
	}
}

func findRepoRoot(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	current := abs
	for {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("repository root not found from %s", start)
		}
		current = parent
	}
}
