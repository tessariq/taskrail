package taskrail

import (
	"fmt"
	"path/filepath"
	"regexp"
)

// specVersionPattern is the versioned-specs naming convention: a leading "v"
// and a three-part dotted numeric version (for example v0.3.0). ActivateSpec
// resolves such a version to specs/<version>.md; anything else is rejected
// before any write, so a typo never repoints the active spec at a malformed
// target.
var specVersionPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// SpecActivateResult reports the repoint ActivateSpec performed, the validation
// it re-ran afterward, and the coverage of the now-active spec. Coverage is
// informational only — it never affects whether activation succeeds.
type SpecActivateResult struct {
	ActiveSpecVersion string           `json:"active_spec_version"`
	ActiveSpecPath    string           `json:"active_spec_path"`
	Validation        ValidationResult `json:"validation"`
	Coverage          CoverageReport   `json:"coverage"`
}

// ActivateSpec repoints STATE.md's active spec to version. It is the sanctioned
// CLI-only writer of active_spec_version/active_spec_path: it validates the
// version against the versioned-specs convention and that specs/<version>.md
// exists, rejecting a bad target with no write. On success it rewrites only
// STATE.md (never task files or status fields), then re-runs validation and
// returns its result. Activation repoints the active spec and nothing else.
func (s *Service) ActivateSpec(version string) (SpecActivateResult, error) {
	if !specVersionPattern.MatchString(version) {
		return SpecActivateResult{}, fmt.Errorf("invalid spec version %q: expected a versioned name like v0.3.0", version)
	}
	specFile := filepath.Join(s.paths.SpecsDir, version+".md")
	if !fileExists(specFile) {
		return SpecActivateResult{}, fmt.Errorf("spec file %s does not exist", relPath(s.paths.RepoRoot, specFile))
	}

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return SpecActivateResult{}, err
	}

	state.Frontmatter.ActiveSpecVersion = version
	state.Frontmatter.ActiveSpecPath = relPath(s.paths.RepoRoot, specFile)
	state.Frontmatter.UpdatedAt = timestamp(s.now())
	state.Body = renderStateBody(state.Frontmatter, tasks)
	if err := s.saveState(state); err != nil {
		return SpecActivateResult{}, err
	}

	validation, err := s.Validate()
	if err != nil {
		return SpecActivateResult{}, err
	}
	// Coverage of the just-repointed spec, computed via the shared T-059
	// capability against the in-memory state/tasks (activation never rewrites
	// task files). Informational only: activation already succeeded above.
	coverage, err := s.coverageFor(state, tasks)
	if err != nil {
		return SpecActivateResult{}, err
	}
	return SpecActivateResult{
		ActiveSpecVersion: version,
		ActiveSpecPath:    state.Frontmatter.ActiveSpecPath,
		Validation:        validation,
		Coverage:          coverage,
	}, nil
}
