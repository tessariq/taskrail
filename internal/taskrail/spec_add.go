package taskrail

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// readingOrderEntryPattern matches a numbered reading-order entry naming a
// versioned spec file, e.g. "2. `specs/v0.2.0.md`". The captured version lets
// AddSpec merge a new entry into the list in version order.
var readingOrderEntryPattern = regexp.MustCompile("^\\s*\\d+\\.\\s+`specs/(v\\d+\\.\\d+\\.\\d+)\\.md`\\s*$")

// SpecAddResult reports the scaffold AddSpec wrote: the created spec file and the
// README whose reading order it updated, both repo-relative. It carries no
// STATE.md field because add never activates the new spec.
type SpecAddResult struct {
	Version    string `json:"version"`
	SpecPath   string `json:"spec_path"`
	ReadmePath string `json:"readme_path"`
}

// AddSpec scaffolds specs/<version>.md with the standard section skeleton and
// adds the version to the specs/README.md reading order in version order. It is
// the one writer in the spec family that authors a spec file, but it never
// writes STATE.md or any task file and never activates the new spec — activation
// stays a separate explicit step (ActivateSpec). The scaffolded Potential
// Features section carries no `###` feature areas, so a fresh spec resolves to
// zero coverable areas and coverage reports N/A rather than a false gap. A
// version that already exists or violates the versioned-specs convention is
// rejected before any write.
func (s *Service) AddSpec(version string) (SpecAddResult, error) {
	if !specVersionPattern.MatchString(version) {
		return SpecAddResult{}, fmt.Errorf("invalid spec version %q: expected a versioned name like v0.3.0", version)
	}
	specFile := filepath.Join(s.paths.SpecsDir, version+".md")
	if fileExists(specFile) {
		return SpecAddResult{}, fmt.Errorf("spec file %s already exists; refusing to overwrite", relPath(s.paths.RepoRoot, specFile))
	}

	// Read the README before any write so a genuinely unreadable README fails the
	// operation before it touches the filesystem; a missing README is fine (the
	// reading order is created fresh).
	readmePath := filepath.Join(s.paths.SpecsDir, "README.md")
	readme, err := os.ReadFile(readmePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return SpecAddResult{}, fmt.Errorf("read specs README %s: %w", relPath(s.paths.RepoRoot, readmePath), fsCause(err))
	}

	if err := os.WriteFile(specFile, []byte(scaffoldSpec(version)), 0o644); err != nil {
		return SpecAddResult{}, fmt.Errorf("write spec file %s: %w", relPath(s.paths.RepoRoot, specFile), err)
	}
	if err := os.WriteFile(readmePath, []byte(updateReadingOrder(string(readme), version)), 0o644); err != nil {
		return SpecAddResult{}, fmt.Errorf("update specs README %s: %w", relPath(s.paths.RepoRoot, readmePath), err)
	}

	return SpecAddResult{
		Version:    version,
		SpecPath:   relPath(s.paths.RepoRoot, specFile),
		ReadmePath: relPath(s.paths.RepoRoot, readmePath),
	}, nil
}

// updateReadingOrder returns readme with version added to its `## Reading Order`
// numbered list in version order, renumbering the list. Existing entries are
// preserved and a version already listed is left in place (idempotent). When the
// README has no reading order section, one is appended carrying the new version.
// The result always ends with exactly one trailing newline.
func updateReadingOrder(readme, version string) string {
	lines := strings.Split(readme, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "## Reading Order" {
			start = i
			break
		}
	}
	if start == -1 {
		return appendReadingOrder(readme, version)
	}

	// The section runs to the next level-1/2 heading or end of file.
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if trimmed := strings.TrimSpace(lines[i]); strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ") {
			end = i
			break
		}
	}

	versions := []string{version}
	for i := start + 1; i < end; i++ {
		if m := readingOrderEntryPattern.FindStringSubmatch(lines[i]); m != nil {
			versions = append(versions, m[1])
		}
	}
	versions = dedupSortVersions(versions)

	rebuilt := append([]string{}, lines[:start]...)
	rebuilt = append(rebuilt, "## Reading Order", "")
	for n, v := range versions {
		rebuilt = append(rebuilt, fmt.Sprintf("%d. `specs/%s.md`", n+1, v))
	}
	if end < len(lines) {
		rebuilt = append(rebuilt, "")
		rebuilt = append(rebuilt, lines[end:]...)
	}
	return strings.TrimRight(strings.Join(rebuilt, "\n"), "\n") + "\n"
}

// appendReadingOrder adds a fresh reading order section to a README that lacks
// one, listing only the new version.
func appendReadingOrder(readme, version string) string {
	section := fmt.Sprintf("## Reading Order\n\n1. `specs/%s.md`\n", version)
	trimmed := strings.TrimRight(readme, "\n")
	if trimmed == "" {
		return section
	}
	return trimmed + "\n\n" + section
}

// dedupSortVersions returns the versions with duplicates removed, ordered by the
// numeric version comparison shared with the spec listing.
func dedupSortVersions(versions []string) []string {
	seen := make(map[string]bool, len(versions))
	out := make([]string, 0, len(versions))
	for _, v := range versions {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sortSpecVersions(out)
	return out
}
