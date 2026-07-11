package taskrail

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// SpecEntry is one versioned spec discovered under specs/, with the active one
// flagged. Path is repo-relative so it matches STATE.md's active_spec_path.
type SpecEntry struct {
	Version string `json:"version"`
	Path    string `json:"path"`
	Active  bool   `json:"active"`
}

// SpecListResult is the read-only listing of versioned specs, in version order,
// with the STATE.md active spec identified.
type SpecListResult struct {
	ActiveSpecVersion string      `json:"active_spec_version"`
	Specs             []SpecEntry `json:"specs"`
}

// SpecAnchor is one spec_ref heading anchor. Anchor is the slug spec_ref
// validation accepts; Heading and Level carry the source heading text and depth
// so a human listing stays legible without re-deriving them.
type SpecAnchor struct {
	Anchor  string `json:"anchor"`
	Heading string `json:"heading"`
	Level   int    `json:"level"`
}

// SpecShowResult is a read-only view of one versioned spec. In plain mode Content
// holds the spec body and Anchors is empty; in --anchors mode Anchors holds the
// stable, deduped anchor list and Content is empty.
type SpecShowResult struct {
	Version string       `json:"version"`
	Path    string       `json:"path"`
	Active  bool         `json:"active"`
	Content string       `json:"content,omitempty"`
	Anchors []SpecAnchor `json:"anchors,omitempty"`
}

// SpecList enumerates the versioned specs under specs/ (files matching the
// vN.N.N naming convention) in version order and marks the STATE.md active spec.
// It is strictly read-only: it never writes STATE.md or task files.
func (s *Service) SpecList() (SpecListResult, error) {
	active, err := s.activeSpecVersion()
	if err != nil {
		return SpecListResult{}, err
	}

	entries, err := os.ReadDir(s.paths.SpecsDir)
	if err != nil {
		return SpecListResult{}, fmt.Errorf("read specs dir %s: %w", relPath(s.paths.RepoRoot, s.paths.SpecsDir), fsCause(err))
	}

	specs := make([]SpecEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		version := strings.TrimSuffix(entry.Name(), ".md")
		if version == entry.Name() || !specVersionPattern.MatchString(version) {
			continue
		}
		specs = append(specs, SpecEntry{
			Version: version,
			Path:    relPath(s.paths.RepoRoot, filepath.Join(s.paths.SpecsDir, entry.Name())),
			Active:  version == active,
		})
	}
	sort.Slice(specs, func(i, j int) bool {
		return lessSpecVersion(specs[i].Version, specs[j].Version)
	})

	return SpecListResult{ActiveSpecVersion: active, Specs: specs}, nil
}

// sortSpecVersions sorts versioned names in place by the numeric version
// comparison, so callers that hold a plain []string (not []SpecEntry) share one
// ordering rule with SpecList.
func sortSpecVersions(versions []string) {
	sort.Slice(versions, func(i, j int) bool {
		return lessSpecVersion(versions[i], versions[j])
	})
}

// SpecShow returns one versioned spec. With anchorsOnly it returns the stable,
// deduped spec_ref anchor list (exactly the anchors validation accepts); without
// it, the spec body. It rejects a non-conforming or missing version before any
// output and is strictly read-only.
func (s *Service) SpecShow(version string, anchorsOnly bool) (SpecShowResult, error) {
	if !specVersionPattern.MatchString(version) {
		return SpecShowResult{}, fmt.Errorf("invalid spec version %q: expected a versioned name like v0.3.0", version)
	}
	specFile := filepath.Join(s.paths.SpecsDir, version+".md")
	data, err := os.ReadFile(specFile)
	if err != nil {
		return SpecShowResult{}, fmt.Errorf("read spec file %s: %w", relPath(s.paths.RepoRoot, specFile), fsCause(err))
	}
	active, err := s.activeSpecVersion()
	if err != nil {
		return SpecShowResult{}, err
	}

	result := SpecShowResult{
		Version: version,
		Path:    relPath(s.paths.RepoRoot, specFile),
		Active:  version == active,
	}
	if anchorsOnly {
		result.Anchors = collectHeadingAnchorList(string(data))
	} else {
		result.Content = string(data)
	}
	return result, nil
}

// activeSpecVersion reads STATE.md's active_spec_version without loading task
// files, keeping the read-only spec commands cheap.
func (s *Service) activeSpecVersion() (string, error) {
	state, err := s.loadState()
	if err != nil {
		return "", err
	}
	return state.Frontmatter.ActiveSpecVersion, nil
}

// lessSpecVersion orders two vN.N.N versions numerically (so v0.10.0 sorts after
// v0.2.0). Inputs are pre-filtered by specVersionPattern, so each parses cleanly.
func lessSpecVersion(a, b string) bool {
	ap, bp := parseSpecVersion(a), parseSpecVersion(b)
	for i := 0; i < len(ap); i++ {
		if ap[i] != bp[i] {
			return ap[i] < bp[i]
		}
	}
	return false
}

// parseSpecVersion splits a "vMAJOR.MINOR.PATCH" version into its three numbers.
func parseSpecVersion(version string) [3]int {
	var parts [3]int
	for i, field := range strings.SplitN(strings.TrimPrefix(version, "v"), ".", 3) {
		parts[i], _ = strconv.Atoi(field)
	}
	return parts
}
