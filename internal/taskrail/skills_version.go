package taskrail

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// skillVersionKey records, in a materialized skill's frontmatter, the Taskrail
// version that wrote it. Agent tools read `name`/`description`; an extra key is
// inert at the point of use, so the marker makes version skew detectable
// (specs/v0.4.0.md#version-skew-detection) without changing how a skill is
// interpreted.
const skillVersionKey = "taskrail_version"

// skillFileName is the only file the shippable package materializes per skill.
// Reading just this name keeps the version report to installed skills and skips
// the timestamped `.bak.` siblings a --force reinstall leaves behind.
const skillFileName = "SKILL.md"

// skillVersionMarker reads back what stampSkillVersion writes. A struct tag cannot
// reference a constant, so the tag must stay in sync with skillVersionKey by hand;
// the install/read round-trip test fails if the two ever drift apart.
type skillVersionMarker struct {
	Version string `yaml:"taskrail_version"`
}

// InstalledSkill reports the Taskrail version recorded in one materialized skill
// file. Version is empty when the file carries no marker — a skill installed
// before the marker existed is unknown, not an error.
type InstalledSkill struct {
	Path    string `json:"path"`    // repo-relative path to the skill file
	Skill   string `json:"skill"`   // skill directory name
	Version string `json:"version"` // writing Taskrail version, empty when unknown

	// MatchesPackage reports that the file is byte-identical to the copy this
	// binary embeds, which is what separates a marker-free parity copy — one
	// produced by copying the package source rather than installing it — from a
	// genuinely unknown install (see isPackageParityCopy).
	MatchesPackage bool `json:"matches_package"`
}

// stampSkillVersion appends the writing-version marker to a skill's frontmatter.
// It edits the frontmatter block textually rather than re-marshaling it, so the
// skill's own keys and body stay byte-identical to the embedded package. Content
// without a frontmatter block is returned unchanged: there is nowhere inert to put
// the marker, and corrupting a packaged file is worse than not recording it.
//
// The split is LF-only, unlike parseFrontmatter's CRLF-normalizing read: the input
// is always the repo-owned embedded package, which .gitattributes pins to LF.
// CRLF there would fall through the guards above and leave the file unmarked
// rather than corrupt it.
func stampSkillVersion(data []byte, version string) []byte {
	const delim = "---\n"
	text := string(data)
	if !strings.HasPrefix(text, delim) {
		return data
	}
	frontmatter, body, ok := strings.Cut(text[len(delim):], "\n"+delim)
	if !ok {
		return data
	}
	// %q yields a double-quoted YAML scalar, so an unusual version string cannot
	// break the block.
	marker := fmt.Sprintf("%s: %q\n", skillVersionKey, version)
	return []byte(delim + frontmatter + "\n" + marker + delim + body)
}

// InstalledSkillVersions reports which version wrote each materialized skill in
// this repository, in deterministic path order. It is read-only, and stays cheap
// enough for ordinary commands: it derives the expected paths from the embedded
// package and reads only those paths in each install target. From each installed
// file it takes both the recorded marker and an equality check against the embedded
// copy. It never compares one install against another and never inspects a
// difference beyond "identical or not". A repository with no materialized skills
// reports nothing.
func (s *Service) InstalledSkillVersions() ([]InstalledSkill, error) {
	var installed []InstalledSkill
	for _, target := range shippableSkillTargets {
		err := fs.WalkDir(shippableSkillsFS, shippableSkillsRoot, func(packagePath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || d.Name() != skillFileName {
				return nil
			}
			packaged, err := shippableSkillsFS.ReadFile(packagePath)
			if err != nil {
				return fmt.Errorf("read embedded skill %s: %w", packagePath, err)
			}
			rel := strings.TrimPrefix(packagePath, shippableSkillsRoot+"/")
			diskPath := filepath.Join(s.paths.RepoRoot, target, filepath.FromSlash(rel))
			data, err := os.ReadFile(diskPath)
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("read %s: %w", relPath(s.paths.RepoRoot, diskPath), fsCause(err))
			}
			installed = append(installed, InstalledSkill{
				Path:           relPath(s.paths.RepoRoot, diskPath),
				Skill:          filepath.Base(filepath.Dir(diskPath)),
				Version:        skillVersionOf(data),
				MatchesPackage: bytes.Equal(packaged, data),
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return installed, nil
}

// skillVersionOf returns the marker recorded in a skill file's frontmatter, or an
// empty string when it carries none. Unparseable frontmatter is unknown rather
// than an error: an adopter's hand-edited skill must not fail an advisory read.
func skillVersionOf(data []byte) string {
	marker, _, err := parseFrontmatter[skillVersionMarker](data)
	if err != nil {
		return ""
	}
	return marker.Version
}
