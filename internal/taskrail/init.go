package taskrail

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Init makes a repository Taskrail-managed in a version-aware, non-destructive
// way: it writes the `.taskrail/config.yml` marker and, when the marker records
// an older layout_version, migrates to the current layout. Migration defaults to
// a dry run reporting the diff; callers must pass apply=true to write it. Content
// created for the layout uses writeFileIfMissing/saveState-if-missing semantics,
// so human-authored content under specs/ and planning/ is never rewritten.
func (s *Service) Init(apply bool) (InitResult, error) {
	cfg, hasMarker, err := readMarker(s.paths.RepoRoot)
	if err != nil {
		return InitResult{}, err
	}
	if hasMarker {
		return s.initWithMarker(cfg, apply)
	}
	return s.initWithoutMarker(apply)
}

// initWithoutMarker handles the unmarked cases: a pre-existing v0.1.0 layout is
// adopted (marker written, nothing else touched); a non-standard layout with
// candidate directories triggers a guided retrofit that proposes a mapping and
// defaults to a dry run; and an empty repository gets a fresh layout plus marker.
func (s *Service) initWithoutMarker(apply bool) (InitResult, error) {
	if s.layoutExists() {
		if err := writeMarker(s.paths.RepoRoot, defaultLayoutConfig()); err != nil {
			return InitResult{}, err
		}
		return InitResult{
			Outcome:     InitAdopted,
			FromVersion: currentLayoutVersion,
			ToVersion:   currentLayoutVersion,
			Applied:     true,
			Changes:     []string{markerWriteChange()},
		}, nil
	}

	if mapping := s.detectRetrofit(); len(mapping) > 0 {
		return s.retrofit(mapping, apply)
	}

	if err := s.ensureLayout(); err != nil {
		return InitResult{}, err
	}
	if err := writeMarker(s.paths.RepoRoot, defaultLayoutConfig()); err != nil {
		return InitResult{}, err
	}
	return InitResult{
		Outcome:     InitCreated,
		FromVersion: currentLayoutVersion,
		ToVersion:   currentLayoutVersion,
		Applied:     true,
	}, nil
}

// initWithMarker dispatches on the recorded layout version: current is an
// idempotent no-op, older triggers migration, and newer is refused so an older
// CLI never mangles a layout it does not understand.
func (s *Service) initWithMarker(cfg LayoutConfig, apply bool) (InitResult, error) {
	switch {
	case cfg.LayoutVersion == currentLayoutVersion:
		if err := s.ensureLayout(); err != nil {
			return InitResult{}, err
		}
		return InitResult{
			Outcome:     InitCurrent,
			FromVersion: cfg.LayoutVersion,
			ToVersion:   currentLayoutVersion,
			Applied:     true,
		}, nil
	case cfg.LayoutVersion > currentLayoutVersion:
		return InitResult{}, fmt.Errorf(
			"repository layout_version %d is newer than supported %d; upgrade taskrail",
			cfg.LayoutVersion, currentLayoutVersion)
	default:
		return s.migrate(cfg, apply)
	}
}

// migrate upgrades an older-version repository to the current layout. It defaults
// to a dry run that only reports the diff; apply writes the missing layout,
// bumps the marker, and re-runs validation. It only ever creates missing content
// and rewrites the machine marker, so human-authored files are left intact.
func (s *Service) migrate(cfg LayoutConfig, apply bool) (InitResult, error) {
	changes := append(s.pendingLayoutChanges(),
		fmt.Sprintf("update %s layout_version %d -> %d", markerRelPath(), cfg.LayoutVersion, currentLayoutVersion))

	if !apply {
		return InitResult{
			Outcome:     InitMigrationPreview,
			FromVersion: cfg.LayoutVersion,
			ToVersion:   currentLayoutVersion,
			Changes:     changes,
		}, nil
	}

	migrated := cfg
	migrated.LayoutVersion = currentLayoutVersion
	if migrated.SpecsDir == "" {
		migrated.SpecsDir = defaultSpecsDir
	}
	if migrated.PlanningDir == "" {
		migrated.PlanningDir = defaultPlanningDir
	}
	validation, err := s.applyLayout(migrated)
	if err != nil {
		return InitResult{}, err
	}
	return InitResult{
		Outcome:     InitMigrated,
		FromVersion: cfg.LayoutVersion,
		ToVersion:   currentLayoutVersion,
		Applied:     true,
		Changes:     changes,
		Validation:  &validation,
	}, nil
}

// retrofitCandidates lists the source directory names a non-standard repository
// might already use, in priority order, and the Taskrail directory (target) and
// role each would fill. Detection is deliberately conservative: it only
// recognizes this small, well-known set rather than guessing from arbitrary
// directory names.
var retrofitCandidates = []struct {
	dir    string
	role   string
	target string
}{
	{defaultSpecsDir, "specs", defaultSpecsDir},
	{defaultPlanningDir, "planning", defaultPlanningDir},
	{"notes", "planning", defaultPlanningDir},
}

// detectRetrofit scans an unmarked, non-standard repository for candidate
// directories that suggest an existing layout to adopt, returning the proposed
// mapping onto the Taskrail layout. It returns nil for an empty repository (which
// should be fresh-initialized) and never proposes the same role twice, so a
// human confirms one clear mapping rather than a redundant one.
func (s *Service) detectRetrofit() []RetrofitMapping {
	var mapping []RetrofitMapping
	claimed := map[string]bool{}
	for _, c := range retrofitCandidates {
		if claimed[c.role] {
			continue
		}
		if !dirExists(filepath.Join(s.paths.RepoRoot, c.dir)) {
			continue
		}
		mapping = append(mapping, RetrofitMapping{Source: c.dir, Target: c.target, Role: c.role})
		claimed[c.role] = true
	}
	return mapping
}

// retrofit adopts a detected non-standard layout into the current Taskrail
// layout. It defaults to a dry run that only reports the proposed mapping and the
// changes applying it would make; apply creates the missing layout with
// writeFileIfMissing semantics (never clobbering existing content), writes the
// marker, and re-runs validation.
func (s *Service) retrofit(mapping []RetrofitMapping, apply bool) (InitResult, error) {
	changes := append(s.pendingLayoutChanges(), markerWriteChange())

	if !apply {
		return InitResult{
			Outcome:     InitRetrofitPreview,
			FromVersion: currentLayoutVersion,
			ToVersion:   currentLayoutVersion,
			Changes:     changes,
			Mapping:     mapping,
		}, nil
	}

	validation, err := s.applyLayout(defaultLayoutConfig())
	if err != nil {
		return InitResult{}, err
	}
	return InitResult{
		Outcome:     InitRetrofitApplied,
		FromVersion: currentLayoutVersion,
		ToVersion:   currentLayoutVersion,
		Applied:     true,
		Changes:     changes,
		Mapping:     mapping,
		Validation:  &validation,
	}, nil
}

// applyLayout is the shared apply tail for migrate and retrofit: create the
// current layout idempotently, persist the given marker, and re-run validation.
// Both callers only ever add missing content, so human-authored files survive.
func (s *Service) applyLayout(marker LayoutConfig) (ValidationResult, error) {
	if err := s.ensureLayout(); err != nil {
		return ValidationResult{}, err
	}
	if err := writeMarker(s.paths.RepoRoot, marker); err != nil {
		return ValidationResult{}, err
	}
	return s.Validate()
}

// markerWriteChange describes writing the current marker, used in dry-run diffs.
func markerWriteChange() string {
	return fmt.Sprintf("write %s (layout_version %d)", markerRelPath(), currentLayoutVersion)
}

// layoutExists reports whether the repository already carries a v0.1.0 layout,
// used to tell legacy adoption apart from a fresh empty-repo init.
func (s *Service) layoutExists() bool {
	return fileExists(s.paths.StateFile) || dirExists(s.paths.TasksDir)
}

// ensureLayout creates the current layout idempotently: directories via ensureDir
// and content via writeFileIfMissing, with the state file written only when
// absent. Re-running it never overwrites existing files.
func (s *Service) ensureLayout() error {
	// Only tracked, committed directories are provisioned. Gitignored artifact
	// output (verify/, runs/, manual-test/) is created on demand by verify and
	// manual testing; a clean checkout drops it, so pre-creating it here would
	// leave init and validate inconsistent (T-024/T-025).
	for _, dir := range []string{s.paths.SpecsDir, s.paths.TasksDir} {
		if err := ensureDir(dir); err != nil {
			return err
		}
	}
	if err := writeFileIfMissing(filepath.Join(s.paths.SpecsDir, "README.md"), []byte(starterSpecsReadme())); err != nil {
		return err
	}
	if err := writeFileIfMissing(filepath.Join(s.paths.SpecsDir, "v0.1.0.md"), []byte(starterSpecV010())); err != nil {
		return err
	}
	if _, err := os.Stat(s.paths.StateFile); errors.Is(err, os.ErrNotExist) {
		if err := s.saveState(starterState(s.now())); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("stat state file: %w", err)
	}
	return nil
}

// pendingLayoutChanges lists the layout directories and files ensureLayout would
// create, so a migration dry run can report exactly what applying it would add.
func (s *Service) pendingLayoutChanges() []string {
	var changes []string
	for _, dir := range []string{s.paths.SpecsDir, s.paths.TasksDir} {
		if !dirExists(dir) {
			changes = append(changes, "create dir "+relPath(s.paths.RepoRoot, dir))
		}
	}
	for _, file := range []string{
		filepath.Join(s.paths.SpecsDir, "README.md"),
		filepath.Join(s.paths.SpecsDir, "v0.1.0.md"),
		s.paths.StateFile,
	} {
		if !fileExists(file) {
			changes = append(changes, "create "+relPath(s.paths.RepoRoot, file))
		}
	}
	return changes
}

func markerRelPath() string {
	return filepath.Join(taskrailConfigDir, taskrailConfigFile)
}
