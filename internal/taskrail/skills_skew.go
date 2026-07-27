package taskrail

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// skillSkewRemedy is the one sanctioned resolution. Detection stays read-only —
// Taskrail never rewrites an adopter's skill files to close the skew — so the
// warning names the explicit, backup-taking opt-in instead
// (specs/v0.4.0.md#version-skew-detection).
const skillSkewRemedy = "taskrail init --with-skills --force"

// SkillSkewWarnings reports materialized skills whose recorded writing version is
// not the running binary's, one warning per distinct recorded version. It is
// advisory: callers print it to stderr and never let it gate, because a stale
// skill is a missed improvement rather than invalid state.
//
// It stays cheap enough for ordinary commands by reading only the recorded marker
// (see InstalledSkillVersions); skill contents are never diffed. A repository with
// no materialized skills, or one whose skills all match, reports nothing.
func (s *Service) SkillSkewWarnings(runningVersion string) ([]Warning, error) {
	installed, err := s.InstalledSkillVersions()
	if err != nil {
		return nil, err
	}

	// A skill is materialized into every target tree, and one --force run fixes
	// all of them, so group by recorded version and name each skill once.
	skills := map[string][]string{}
	for _, sk := range installed {
		if sk.Version == runningVersion || slices.Contains(skills[sk.Version], sk.Skill) {
			continue
		}
		skills[sk.Version] = append(skills[sk.Version], sk.Skill)
	}

	versions := make([]string, 0, len(skills))
	for version := range skills {
		versions = append(versions, version)
	}
	sort.Strings(versions)

	var warnings []Warning
	for _, version := range versions {
		warnings = append(warnings, skewWarning(version, runningVersion, skills[version]))
	}
	return warnings, nil
}

// skewWarning renders one recorded-version group. Skills installed before the
// marker existed record nothing, so they are reported as unknown rather than as a
// version mismatch, and without a remedy: an unmarked skill may be current, older,
// or newer, so prescribing a --force reinstall would be exactly the false upgrade
// prompt the unknown case exists to avoid. It also matters concretely — this
// repository's committed copies are deliberately unmarked parity copies of the
// package (docs/workflow/skills-productization.md), and reinstalling them breaks
// `task check:skills`. The remedy returns as soon as a real version is readable.
func skewWarning(recorded, running string, skills []string) Warning {
	names := strings.Join(skills, ", ")
	if recorded == "" {
		return Warning{
			Code: "unknown_skill_version",
			Message: fmt.Sprintf("warning: installed skills carry no version marker, so skew with running taskrail %s cannot be determined (%s)",
				running, names),
		}
	}
	return Warning{
		Code: "skill_version_skew",
		Message: fmt.Sprintf("warning: installed skills were written by taskrail %s, running taskrail %s (%s); run %s to refresh",
			recorded, running, names, skillSkewRemedy),
	}
}
