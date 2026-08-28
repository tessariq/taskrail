package taskrail

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	localSkillCreate   = "create"
	localSkillRefresh  = "refresh"
	localSkillPreserve = "preserve_parity"
	localSkillRefuse   = "refuse"

	localSkillExclusionCreate    = "create"
	localSkillExclusionManaged   = "managed"
	localSkillExclusionExternal  = "external"
	localSkillExclusionAmbiguous = "ambiguous"
)

// localSkillPlan is a read-only snapshot that later local-skill transactions
// consume and recheck. Skill paths stay at assistant discovery roots even when
// planning content itself is stored in the local overlay.
type localSkillPlan struct {
	Destinations []localSkillDestination
	Exclusions   []localSkillExclusion
	Unexpected   []localSkillUnexpected
}

type localSkillDestination struct {
	Path            string
	PackagePath     string
	PackageDigest   string
	Digest          string
	Marker          string
	EntryType       string
	Action          string
	Present         bool
	Tracked         bool
	Staged          bool
	Ignored         bool
	Alias           bool
	SharedExclusion bool
}

type localSkillExclusion struct {
	Path           string
	Ownership      string
	Exact          bool
	Effective      bool
	Shadowed       bool
	SharedGitScope bool
}

// localSkillUnexpected is an existing non-package file in a subtree Taskrail
// would otherwise claim. Its presence makes that subtree unsafe to install.
type localSkillUnexpected struct {
	Path      string
	Digest    string
	EntryType string
}

// planLocalSkills never creates an exclusion or a destination. It is deliberately
// separate from skill installation so the fresh-install and refresh transactions
// can bind this complete preflight snapshot to their own publication boundary.
func (s *Service) planLocalSkills() (localSkillPlan, error) {
	if err := s.requireLocalStorage(); err != nil {
		return localSkillPlan{}, err
	}
	return s.planPromotionSkills()
}

// planPromotionSkills classifies installed skill ownership in either storage
// mode. Deferred promotion needs the same snapshot after semantic state is
// already committed.
func (s *Service) planPromotionSkills() (localSkillPlan, error) {
	excludePath, err := localExcludePath(s.paths)
	if err != nil {
		return localSkillPlan{}, err
	}
	if err := validateTargetPath(excludePath); err != nil {
		return localSkillPlan{}, fmt.Errorf("inspect Git exclusion path: %w", err)
	}
	exclude, err := readInitFile(excludePath)
	if err != nil {
		return localSkillPlan{}, fmt.Errorf("read Git exclusion: %w", err)
	}
	if exclude == nil {
		return localSkillPlan{}, fmt.Errorf("read Git exclusion: file is absent")
	}
	managed, external := splitLocalExclusions(string(exclude))
	managedPatterns := make([]string, 0, len(managed))
	for pattern := range managed {
		managedPatterns = append(managedPatterns, pattern)
	}
	files, err := packagedSkillFiles()
	if err != nil {
		return localSkillPlan{}, err
	}
	names, err := packagedSkillNames()
	if err != nil {
		return localSkillPlan{}, err
	}

	sharedGitScope, err := localSkillSharedGitScope(s.paths)
	if err != nil {
		return localSkillPlan{}, err
	}
	expected := make(map[string]map[string]bool, len(names))
	for _, rel := range files {
		name, _, found := strings.Cut(rel, "/")
		if !found {
			return localSkillPlan{}, fmt.Errorf("embedded skill file %s is not beneath a skill subtree", rel)
		}
		if expected[name] == nil {
			expected[name] = map[string]bool{}
		}
		expected[name][rel] = true
	}
	unsafeSubtrees := map[string]bool{}
	plan := localSkillPlan{
		Destinations: make([]localSkillDestination, 0, len(files)*len(shippableSkillTargets)),
		Exclusions:   make([]localSkillExclusion, 0, len(names)*len(shippableSkillTargets)),
	}
	for _, target := range shippableSkillTargets {
		for _, name := range names {
			subtree := path.Join(filepath.ToSlash(target), name)
			unexpected, err := s.unexpectedLocalSkillFiles(subtree, expected[name])
			if err != nil {
				return localSkillPlan{}, err
			}
			if len(unexpected) > 0 {
				unsafeSubtrees[subtree] = true
				plan.Unexpected = append(plan.Unexpected, unexpected...)
			}
		}
		for _, rel := range files {
			packaged, err := shippableSkillsFS.ReadFile(path.Join(shippableSkillsRoot, rel))
			if err != nil {
				return localSkillPlan{}, fmt.Errorf("read embedded skill %s: %w", rel, err)
			}
			destination := path.Join(filepath.ToSlash(target), rel)
			entry, err := s.planLocalSkillDestination(destination, rel, packaged, sharedGitScope, unsafeSubtrees[path.Dir(destination)])
			if err != nil {
				return localSkillPlan{}, err
			}
			plan.Destinations = append(plan.Destinations, entry)
		}
		for _, name := range names {
			subtree := path.Join(filepath.ToSlash(target), name)
			effective, err := gitIgnoredExact(s.paths.WorktreeRoot, subtree)
			if err != nil {
				return localSkillPlan{}, err
			}
			shadowed, err := localSkillExclusionShadowed(s.paths.WorktreeRoot, excludePath, subtree)
			if err != nil {
				return localSkillPlan{}, err
			}
			exclusion := localSkillExclusion{
				Path: subtree, Exact: managed[subtree], Effective: effective, Shadowed: shadowed || localSkillExcludedBy(external, subtree), SharedGitScope: sharedGitScope,
			}
			switch {
			case exclusion.Exact:
				exclusion.Ownership = localSkillExclusionManaged
			case localSkillExcludedBy(managedPatterns, subtree):
				exclusion.Ownership = localSkillExclusionAmbiguous
			case localSkillExcludedBy(external, subtree) || exclusion.Effective:
				exclusion.Ownership = localSkillExclusionExternal
			default:
				exclusion.Ownership = localSkillExclusionCreate
			}
			plan.Exclusions = append(plan.Exclusions, exclusion)
		}
	}
	slices.SortFunc(plan.Destinations, func(a, b localSkillDestination) int { return strings.Compare(a.Path, b.Path) })
	slices.SortFunc(plan.Exclusions, func(a, b localSkillExclusion) int { return strings.Compare(a.Path, b.Path) })
	slices.SortFunc(plan.Unexpected, func(a, b localSkillUnexpected) int { return strings.Compare(a.Path, b.Path) })
	return plan, nil
}

func localSkillExclusionShadowed(root, excludePath, subtree string) (bool, error) {
	output, err := gitCommand(root, "check-ignore", "-v", "--no-index", "--", subtree)
	if gitExitCode(err) == 1 {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect Git exclusion source for %s: %w", subtree, err)
	}
	match, _, found := strings.Cut(output, "\t")
	if !found || match == "" {
		return false, fmt.Errorf("inspect Git exclusion source for %s: malformed output", subtree)
	}
	managedSource := filepath.ToSlash(excludePath)
	relativeSource := filepath.ToSlash(relPath(root, excludePath))
	return !strings.HasPrefix(match, managedSource+":") && !strings.HasPrefix(match, relativeSource+":"), nil
}

func (s *Service) planLocalSkillDestination(destination, packagePath string, packaged []byte, sharedGitScope, unexpected bool) (localSkillDestination, error) {
	physical := filepath.Join(s.paths.RepoRoot, filepath.FromSlash(destination))
	entryType, alias, err := localSkillEntryType(s.paths.RepoRoot, destination)
	if err != nil {
		return localSkillDestination{}, fmt.Errorf("inspect skill destination %s: %w", destination, err)
	}
	result := localSkillDestination{
		Path: destination, PackagePath: packagePath, PackageDigest: digestBytes(packaged), EntryType: entryType, Alias: alias, SharedExclusion: sharedGitScope,
	}
	result.Tracked, err = gitTracks(s.paths.WorktreeRoot, destination)
	if err != nil {
		return localSkillDestination{}, err
	}
	result.Staged, err = gitStaged(s.paths.WorktreeRoot, destination)
	if err != nil {
		return localSkillDestination{}, err
	}
	result.Ignored, err = gitIgnoredExact(s.paths.WorktreeRoot, destination)
	if err != nil {
		return localSkillDestination{}, err
	}
	if unexpected {
		result.Action = localSkillRefuse
		return result, nil
	}
	if entryType == "absent" && !alias {
		if result.Tracked || result.Staged {
			result.Action = localSkillRefuse
		} else {
			result.Action = localSkillCreate
		}
		return result, nil
	}
	result.Present = true
	if entryType != "regular" || alias || result.Tracked || result.Staged {
		result.Action = localSkillRefuse
		return result, nil
	}
	data, err := os.ReadFile(physical)
	if err != nil {
		return localSkillDestination{}, fmt.Errorf("read skill destination %s: %w", destination, err)
	}
	result.Digest = digestBytes(data)
	result.Marker = localSkillMarker(data)
	if result.Marker == "invalid" || result.Marker == "conflicting" {
		result.Action = localSkillRefuse
		return result, nil
	}
	if bytes.Equal(data, packaged) {
		result.Action = localSkillPreserve
		return result, nil
	}
	if !result.Ignored {
		result.Action = localSkillRefuse
		return result, nil
	}
	if result.Marker == "nested" || result.Marker == "legacy" || result.Marker == "dual" {
		result.Action = localSkillRefresh
		return result, nil
	}
	result.Action = localSkillRefuse
	return result, nil
}

func localSkillMarker(data []byte) string {
	root, _, err := parseSkillDocument(data)
	if err != nil {
		if _, versionErr := skillVersionOf(data); versionErr != nil {
			return "invalid"
		}
		return "none"
	}
	if _, err := skillVersionFromRoot(root); err != nil {
		if strings.Contains(err.Error(), "conflicting") {
			return "conflicting"
		}
		return "invalid"
	}
	mapping := root.Content[0]
	legacy, _ := uniqueMappingValue(mapping, legacySkillVersionKey)
	metadata, _ := uniqueMappingValue(mapping, "metadata")
	var nested *yaml.Node
	if metadata != nil && metadata.Kind == yaml.MappingNode {
		nested, _ = uniqueMappingValue(metadata, skillVersionKey)
	}
	switch {
	case legacy != nil && nested != nil:
		return "dual"
	case nested != nil:
		return "nested"
	case legacy != nil:
		return "legacy"
	default:
		return "none"
	}
}

func localSkillEntryType(root, destination string) (string, bool, error) {
	parent := root
	components := strings.Split(destination, "/")
	for i, component := range components {
		entries, err := os.ReadDir(parent)
		if errors.Is(err, fs.ErrNotExist) {
			return "absent", false, nil
		}
		if err != nil {
			return "", false, err
		}
		var match os.DirEntry
		for _, entry := range entries {
			if !samePortableName(entry.Name(), component) {
				continue
			}
			if entry.Name() != component || match != nil {
				return "alias", true, nil
			}
			match = entry
		}
		if match == nil {
			return "absent", false, nil
		}
		physical := filepath.Join(parent, component)
		info, err := os.Lstat(physical)
		if err != nil {
			return "", false, err
		}
		if i == len(components)-1 {
			switch {
			case info.Mode().IsRegular():
				return "regular", false, nil
			case info.Mode()&os.ModeSymlink != 0:
				return "symlink", false, nil
			case info.IsDir():
				return "directory", false, nil
			default:
				return "special", false, nil
			}
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "traversal", false, nil
		}
		parent = physical
	}
	return "absent", false, nil
}

func (s *Service) unexpectedLocalSkillFiles(subtree string, expected map[string]bool) ([]localSkillUnexpected, error) {
	root := filepath.Join(s.paths.RepoRoot, filepath.FromSlash(subtree))
	info, err := os.Lstat(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("inspect skill subtree %s: %w", subtree, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		item := localSkillUnexpected{Path: subtree, EntryType: localSkillObservedEntryType(info.Mode())}
		if item.EntryType == "regular" {
			data, err := os.ReadFile(root)
			if err != nil {
				return nil, fmt.Errorf("inspect skill subtree %s: %w", subtree, err)
			}
			item.Digest = digestBytes(data)
		}
		return []localSkillUnexpected{item}, nil
	}
	var unexpected []localSkillUnexpected
	err = filepath.WalkDir(root, func(physical string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if physical == root || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(s.paths.RepoRoot, physical)
		if err != nil {
			return err
		}
		reported := filepath.ToSlash(rel)
		packagePath := strings.TrimPrefix(reported, path.Dir(subtree)+"/")
		if expected[packagePath] {
			return nil
		}
		item := localSkillUnexpected{Path: reported, EntryType: localSkillObservedEntryType(entry.Type())}
		if item.EntryType == "regular" {
			data, err := os.ReadFile(physical)
			if err != nil {
				return err
			}
			item.Digest = digestBytes(data)
		}
		unexpected = append(unexpected, item)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inspect skill subtree %s: %w", subtree, err)
	}
	return unexpected, nil
}

func localSkillObservedEntryType(mode fs.FileMode) string {
	switch {
	case mode.IsRegular():
		return "regular"
	case mode&os.ModeSymlink != 0:
		return "symlink"
	default:
		return "special"
	}
}

func localSkillSharedGitScope(paths Paths) (bool, error) {
	if paths.GitDir != paths.GitCommonDir {
		return true, nil
	}
	entries, err := os.ReadDir(filepath.Join(paths.GitCommonDir, "worktrees"))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect linked worktrees: %w", err)
	}
	return len(entries) > 0, nil
}

func localSkillExcludedBy(patterns []string, subtree string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimPrefix(strings.TrimSuffix(pattern, "/"), "/")
		if pattern == subtree || strings.HasPrefix(subtree, pattern+"/") {
			return true
		}
	}
	return false
}

func gitTracks(root, path string) (bool, error) {
	output, err := gitCommand(root, "ls-files", "--error-unmatch", "--", path)
	if err == nil {
		return strings.TrimSpace(output) != "", nil
	}
	if gitExitCode(err) == 1 {
		return false, nil
	}
	return false, fmt.Errorf("inspect Git tracking for %s: %w", path, err)
}

func gitStaged(root, path string) (bool, error) {
	_, err := gitCommand(root, "diff", "--cached", "--quiet", "--", path)
	if err == nil {
		return false, nil
	}
	if gitExitCode(err) == 1 {
		return true, nil
	}
	return false, fmt.Errorf("inspect Git index for %s: %w", path, err)
}

func gitExitCode(err error) int {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return -1
}

func gitIgnoredExact(root, path string) (bool, error) {
	_, err := gitCommand(root, "check-ignore", "-q", "--no-index", path)
	if err == nil {
		return true, nil
	}
	if gitExitCode(err) == 1 {
		return false, nil
	}
	return false, fmt.Errorf("inspect Git exclusion for %s: %w", path, err)
}

func isSkillDestination(path string) bool {
	for _, root := range shippableSkillTargets {
		if strings.HasPrefix(path, filepath.ToSlash(root)+"/") {
			return true
		}
	}
	return false
}
