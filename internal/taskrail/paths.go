package taskrail

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/text/unicode/norm"
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
	return discoverPaths(start, false)
}

// DiscoverRecoveryPaths resolves the repository for the shared recovery
// boundary: the one reader a fenced migration marker must admit, because the
// retained transaction it names is handed to exactly that command
// (specs/v0.5.0.md#layout-compatibility-and-upgrade).
func DiscoverRecoveryPaths(start string) (Paths, error) {
	return discoverPaths(start, true)
}

func discoverPaths(start string, admitFence bool) (Paths, error) {
	start, err := canonicalStart(start)
	if err != nil {
		return Paths{}, err
	}
	git, err := discoverGitWorktree(start)
	if err != nil {
		return Paths{}, err
	}
	root, markerFound, err := discoverManagedRoot(start)
	if err != nil {
		return Paths{}, err
	}
	if !markerFound {
		if git.WorktreeRoot == "" {
			return Paths{}, WithMachineErrorCode(MachineCodeNotInitialized,
				fmt.Errorf("managed repository root not found from %s", start))
		}
		root = git.WorktreeRoot
	}
	if git.WorktreeRoot != "" && root != git.WorktreeRoot {
		return Paths{}, WithMachineErrorCode(MachineCodeRepositoryInvalid,
			fmt.Errorf("managed root %s does not match Git worktree root %s", root, git.WorktreeRoot))
	}
	cfg, storage, err := discoverLayout(root, markerFound, admitFence)
	if err != nil {
		return Paths{}, err
	}
	if admitFence && !markerFound && exists(filepath.Join(root, filepath.FromSlash(localStorageRoot))) {
		// An interrupted fresh local init can retain its transaction before the
		// marker lands. Recovery is the only reader allowed to treat that exact
		// overlay shape as local while it derives the journal's safe action.
		storage = localStorage()
	}
	if storage.Mode == StorageLocal && git.WorktreeRoot == "" {
		return Paths{}, WithMachineErrorCode(MachineCodeRepositoryInvalid,
			fmt.Errorf("local storage mode requires a Git worktree"))
	}
	paths := pathsFromDiscovery(root, cfg, storage, git)
	if err := validateDiscoveredPaths(paths); err != nil {
		return Paths{}, WithMachineErrorCode(MachineCodeRepositoryInvalid, err)
	}
	return paths, nil
}

type gitContext struct {
	WorktreeRoot string
	GitDir       string
	GitCommonDir string
}

func canonicalStart(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	abs = filepath.Clean(abs)
	if err := validateExistingPath(abs, true); err != nil {
		return "", WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("unsafe invocation path: %w", err))
	}
	return abs, nil
}

func discoverManagedRoot(start string) (string, bool, error) {
	for current := start; ; current = filepath.Dir(current) {
		taskrailDir, found, err := exactChild(current, taskrailConfigDir)
		if err != nil {
			return "", false, err
		}
		if found {
			config, configFound, err := exactChild(taskrailDir, taskrailConfigFile)
			if err != nil {
				return "", false, err
			}
			if configFound {
				if err := validateRegularFile(config); err != nil {
					return "", false, fmt.Errorf("invalid layout marker: %w", err)
				}
				return current, true, nil
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false, nil
		}
	}
}

func discoverGitWorktree(start string) (gitContext, error) {
	for current := start; ; current = filepath.Dir(current) {
		gitEntry, found, err := exactChild(current, ".git")
		if err != nil {
			return gitContext{}, err
		}
		if found {
			gitDir, err := resolveGitDir(current, gitEntry)
			if err != nil {
				return gitContext{}, err
			}
			common, err := resolveGitCommonDir(gitDir)
			if err != nil {
				return gitContext{}, err
			}
			return gitContext{WorktreeRoot: current, GitDir: gitDir, GitCommonDir: common}, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return gitContext{}, nil
		}
	}
}

func resolveGitDir(worktree, entry string) (string, error) {
	info, err := os.Lstat(entry)
	if err != nil {
		return "", fmt.Errorf("inspect Git directory: %w", fsCause(err))
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("Git directory traversal contains symlink %s", entry)
	}
	if info.IsDir() {
		return entry, nil
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("Git directory marker %s is not a regular file or directory", entry)
	}
	data, err := os.ReadFile(entry)
	if err != nil {
		return "", fmt.Errorf("read Git directory marker: %w", fsCause(err))
	}
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, "gitdir: ") || strings.Contains(line, "\n") {
		return "", fmt.Errorf("Git directory marker is malformed")
	}
	dir := strings.TrimPrefix(line, "gitdir: ")
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(worktree, dir)
	}
	dir = filepath.Clean(dir)
	if err := validateExistingPath(dir, true); err != nil {
		return "", fmt.Errorf("unsafe Git directory: %w", err)
	}
	return dir, nil
}

func resolveGitCommonDir(gitDir string) (string, error) {
	commonFile := filepath.Join(gitDir, "commondir")
	info, err := os.Lstat(commonFile)
	if errors.Is(err, os.ErrNotExist) {
		return gitDir, nil
	}
	if err != nil {
		return "", fmt.Errorf("inspect Git common directory marker: %w", fsCause(err))
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("Git common directory marker is not a regular file")
	}
	data, err := os.ReadFile(commonFile)
	if err != nil {
		return "", fmt.Errorf("read Git common directory marker: %w", fsCause(err))
	}
	value := strings.TrimSpace(string(data))
	if value == "" || strings.Contains(value, "\n") {
		return "", fmt.Errorf("Git common directory marker is malformed")
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(gitDir, value)
	}
	value = filepath.Clean(value)
	if err := validateExistingPath(value, true); err != nil {
		return "", fmt.Errorf("unsafe Git common directory: %w", err)
	}
	return value, nil
}

func discoverLayout(root string, markerFound, admitFence bool) (LayoutConfig, StorageContext, error) {
	if !markerFound {
		return defaultLayoutConfig(), committedStorage(), nil
	}
	data, err := os.ReadFile(markerPath(root))
	if err != nil {
		return LayoutConfig{}, StorageContext{}, fmt.Errorf("read layout marker: %w", fsCause(err))
	}
	var header struct {
		LayoutVersion int `yaml:"layout_version"`
	}
	if err := yaml.Unmarshal(data, &header); err != nil {
		return LayoutConfig{}, StorageContext{}, WithMachineErrorCode(MachineCodeRepositoryInvalid,
			fmt.Errorf("parse layout marker: %w", err))
	}
	if header.LayoutVersion <= currentLayoutVersion {
		cfg, err := loadLayoutConfig(root)
		return cfg, committedStorage(), err
	}
	if header.LayoutVersion > layout2Version {
		return LayoutConfig{}, StorageContext{}, ensureSupportedLayoutVersion(LayoutConfig{LayoutVersion: header.LayoutVersion})
	}
	cfg, err := decodeLayoutMarkerStrict(data)
	if err != nil {
		return LayoutConfig{}, StorageContext{}, WithMachineErrorCode(MachineCodeRepositoryInvalid, err)
	}
	if cfg.MigrationFence != nil {
		if !admitFence {
			transaction := cfg.MigrationFence.TransactionID
			return LayoutConfig{}, StorageContext{}, WithMachineErrorCode(MachineCodeMigrationInProgress,
				fmt.Errorf("layout migration is in progress (transaction %s); run taskrail recover %s to finish or undo it, or revert the upgrade through Git",
					transaction, transaction))
		}
		// The recovery boundary reads the fenced marker as the layout-2
		// committed context it pins, because the retained transaction beneath
		// it is exactly what that boundary acts on.
		return LayoutConfig{LayoutVersion: cfg.LayoutVersion, SpecsDir: cfg.SpecsDir, PlanningDir: cfg.PlanningDir}, committedStorage(), nil
	}
	storage := committedStorage()
	if cfg.LayoutVersion == layout2Version && cfg.StorageMode == StorageLocal {
		storage = localStorage()
	}
	return LayoutConfig{LayoutVersion: cfg.LayoutVersion, SpecsDir: cfg.SpecsDir, PlanningDir: cfg.PlanningDir}, storage, nil
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
	cfg, found, err := readLayoutFile(root, "layout config")
	if err != nil {
		return LayoutConfig{}, err
	}
	if !found {
		return defaultLayoutConfig(), nil
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
	return readLayoutFile(root, "layout marker")
}

// readLayoutFile is the one read/unmarshal/version-guard sequence behind both
// marker readers, so a future marker-level rule has a single place to land. The
// label carries each caller's adopter-facing wording for the same file, which
// differs on purpose and is contract (T-131).
func readLayoutFile(root, label string) (LayoutConfig, bool, error) {
	path := markerPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return LayoutConfig{}, false, nil
		}
		return LayoutConfig{}, false, fmt.Errorf("read %s %s: %w", label, relPath(root, path), fsCause(err))
	}

	cfg := defaultLayoutConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return LayoutConfig{}, false, fmt.Errorf("parse %s %s: %w", label, relPath(root, path), err)
	}
	if err := ensureSupportedLayoutVersion(cfg); err != nil {
		return LayoutConfig{}, false, err
	}
	return cfg, true, nil
}

// ensureSupportedLayoutVersion refuses a marker recording a layout newer than
// this binary models. It guards both marker reads, so every command reaches it
// through the normal layout load — before any read-modify-write, and for
// read-only reporters too, since a plausible-looking report against a layout the
// binary cannot model is worse than a refusal
// (specs/v0.4.0.md#layout-compatibility-beyond-init).
func ensureSupportedLayoutVersion(cfg LayoutConfig) error {
	if cfg.LayoutVersion <= layout2Version {
		return nil
	}
	return WithMachineErrorCode(MachineCodeIncompatibleLayout,
		fmt.Errorf("repository layout_version %d is newer than supported %d; upgrade taskrail",
			cfg.LayoutVersion, currentLayoutVersion))
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

// pathsFromLayout resolves the layout's logical directories through one storage
// context. The marker keeps recording logical `specs`/`planning`; the context
// decides where those resolve physically, so every reader and writer below stays
// storage-neutral by construction.
func pathsFromLayout(root string, cfg LayoutConfig, storage StorageContext) Paths {
	return pathsFromDiscovery(root, cfg, storage, gitContext{})
}

func pathsFromDiscovery(root string, cfg LayoutConfig, storage StorageContext, git gitContext) Paths {
	planningDir := filepath.Join(root, filepath.FromSlash(storage.physical(cfg.PlanningDir)))
	artifactsDir := filepath.Join(planningDir, "artifacts")
	storageRoot := root
	promptsDir := filepath.Join(root, taskrailConfigDir, "prompts")
	runtimeDir := filepath.Join(root, taskrailConfigDir, "runtime")
	if storage.Mode == StorageLocal {
		storageRoot = filepath.Join(root, filepath.FromSlash(localStorageRoot))
		promptsDir = filepath.Join(storageRoot, "prompts")
		runtimeDir = filepath.Join(storageRoot, "runtime")
	}
	lockRoot := runtimeDir
	if git.GitCommonDir != "" {
		lockRoot = filepath.Join(git.GitCommonDir, "taskrail")
	}

	return Paths{
		RepoRoot:           root,
		ManagedRoot:        root,
		WorktreeRoot:       git.WorktreeRoot,
		GitDir:             git.GitDir,
		GitCommonDir:       git.GitCommonDir,
		ConfigFile:         markerPath(root),
		StorageRoot:        storageRoot,
		LockRoot:           lockRoot,
		Storage:            storage,
		LogicalSpecsDir:    filepath.ToSlash(cfg.SpecsDir),
		LogicalPlanningDir: filepath.ToSlash(cfg.PlanningDir),
		LogicalPromptsDir:  path.Join(taskrailConfigDir, "prompts"),

		SpecsDir:     filepath.Join(root, filepath.FromSlash(storage.physical(cfg.SpecsDir))),
		PlanningDir:  planningDir,
		PromptsDir:   promptsDir,
		TasksDir:     filepath.Join(planningDir, "tasks"),
		ArtifactsDir: artifactsDir,
		VerifyDir:    filepath.Join(artifactsDir, "verify"),
		RuntimeDir:   runtimeDir,
		StateFile:    filepath.Join(planningDir, "STATE.md"),
	}
}

func validateDiscoveredPaths(paths Paths) error {
	if paths.LogicalSpecsDir == paths.LogicalPlanningDir || pathContains(paths.LogicalSpecsDir, paths.LogicalPlanningDir) || pathContains(paths.LogicalPlanningDir, paths.LogicalSpecsDir) {
		return fmt.Errorf("layout specs_dir and planning_dir overlap")
	}
	for _, field := range []struct{ name, logical string }{
		{name: "specs_dir", logical: paths.LogicalSpecsDir},
		{name: "planning_dir", logical: paths.LogicalPlanningDir},
	} {
		if err := validateLogicalDir(field.name, field.logical); err != nil {
			return err
		}
	}
	if err := validateOptionalRegularFile(paths.ConfigFile); err != nil {
		return err
	}
	for _, physical := range []string{paths.StorageRoot, paths.SpecsDir, paths.PlanningDir, paths.PromptsDir, paths.RuntimeDir} {
		if err := validateDirectoryTarget(physical); err != nil {
			return err
		}
	}
	if paths.Storage.Mode == StorageLocal {
		for _, committed := range []string{
			filepath.Join(paths.ManagedRoot, filepath.FromSlash(paths.LogicalSpecsDir)),
			filepath.Join(paths.ManagedRoot, filepath.FromSlash(paths.LogicalPlanningDir)),
			filepath.Join(paths.ManagedRoot, taskrailConfigDir, "prompts"),
		} {
			if err := validateTargetPath(committed); err != nil {
				return err
			}
			if exists(committed) {
				return fmt.Errorf("mixed committed/local Taskrail state at %s", committed)
			}
		}
	} else {
		localRoot := filepath.Join(paths.ManagedRoot, filepath.FromSlash(localStorageRoot))
		if err := validateTargetPath(localRoot); err != nil {
			return err
		}
		if exists(localRoot) && !isEmptyLocalInitScaffold(localRoot) {
			return fmt.Errorf("mixed committed/local Taskrail state at %s", localRoot)
		}
		for _, local := range []string{
			filepath.Join(paths.ManagedRoot, filepath.FromSlash(localStorageRoot), filepath.FromSlash(paths.LogicalSpecsDir)),
			filepath.Join(paths.ManagedRoot, filepath.FromSlash(localStorageRoot), filepath.FromSlash(paths.LogicalPlanningDir)),
			filepath.Join(paths.ManagedRoot, filepath.FromSlash(localStorageRoot), "prompts"),
		} {
			if err := validateTargetPath(local); err != nil {
				return err
			}
			if exists(local) && !isEmptyLocalInitScaffold(filepath.Join(paths.ManagedRoot, filepath.FromSlash(localStorageRoot))) {
				return fmt.Errorf("mixed committed/local Taskrail state at %s", local)
			}
		}
	}
	return nil
}

func isEmptyLocalInitScaffold(root string) bool {
	allowed := map[string]bool{
		".":              true,
		"planning":       true,
		"planning/tasks": true,
		"specs":          true,
		"runtime":        true,
	}
	return filepath.WalkDir(root, func(physical string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, physical)
		if err != nil {
			return err
		}
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !allowed[filepath.ToSlash(rel)] {
			return fmt.Errorf("not an empty local initialization scaffold")
		}
		return nil
	}) == nil
}

func validateLogicalDir(field, value string) error {
	if value == "" || filepath.IsAbs(filepath.FromSlash(value)) || path.Clean(value) != value || value == "." || strings.HasPrefix(value, "../") || strings.Contains(value, "\\") {
		return fmt.Errorf("layout config %s %q is not a canonical repository-relative path", field, value)
	}
	return nil
}

func pathContains(parent, child string) bool {
	return strings.HasPrefix(child, strings.TrimSuffix(parent, "/")+"/")
}

func exactChild(parent, wanted string) (string, bool, error) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return "", false, fmt.Errorf("read directory %s: %w", parent, fsCause(err))
	}
	found := false
	for _, entry := range entries {
		if !samePortableName(entry.Name(), wanted) {
			continue
		}
		if entry.Name() != wanted || found {
			return "", false, fmt.Errorf("path alias for %s exists beneath %s", wanted, parent)
		}
		found = true
	}
	return filepath.Join(parent, wanted), found, nil
}

func samePortableName(a, b string) bool {
	return strings.EqualFold(norm.NFC.String(a), norm.NFC.String(b))
}

func validateTargetPath(target string) error {
	current := filepath.VolumeName(target) + string(filepath.Separator)
	for _, component := range strings.Split(strings.TrimPrefix(target, current), string(filepath.Separator)) {
		if component == "" {
			continue
		}
		parent := current
		candidate, found, err := exactChild(parent, component)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if !found {
			// Windows temporary paths may contain an 8.3 component (for example
			// RUNNER~1) that is not returned by ReadDir. Keep validating through
			// that existing spelling rather than treating the rest as absent.
			candidate = filepath.Join(parent, component)
			if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
				return nil
			} else if err != nil {
				return err
			}
		}
		info, err := os.Lstat(candidate)
		if err != nil {
			return err
		}
		macOSVarAlias := runtime.GOOS == "darwin" && candidate == "/var" && info.Mode()&os.ModeSymlink != 0
		if info.Mode()&os.ModeSymlink != 0 && !macOSVarAlias {
			return fmt.Errorf("path traversal contains symlink %s", candidate)
		}
		if candidate != target && !info.IsDir() && !macOSVarAlias {
			return fmt.Errorf("path traversal contains special or non-directory entry %s", candidate)
		}
		current = candidate
	}
	return nil
}

func validateExistingPath(target string, requireDir bool) error {
	if err := validateTargetPath(target); err != nil {
		return err
	}
	info, err := os.Lstat(target)
	if err != nil {
		return err
	}
	if requireDir && !info.IsDir() {
		return fmt.Errorf("%s is not a directory", target)
	}
	return nil
}

func validateDirectoryTarget(target string) error {
	if err := validateTargetPath(target); err != nil {
		return err
	}
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("managed directory %s is a special or non-directory entry", target)
	}
	return nil
}

func validateRegularFile(target string) error {
	if err := validateTargetPath(target); err != nil {
		return err
	}
	info, err := os.Lstat(target)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", target)
	}
	return nil
}

func validateOptionalRegularFile(target string) error {
	if err := validateTargetPath(target); err != nil {
		return err
	}
	_, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return validateRegularFile(target)
}

func exists(target string) bool {
	_, err := os.Lstat(target)
	return err == nil
}
