package taskrail

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// shippableSkillsFS embeds the repo-agnostic skill set so it ships inside the
// binary and stays versioned with the commands the skills call
// (docs/workflow/skills-productization.md Decision 2). The embedded tree mirrors
// internal/taskrail/skills/; keep the two in sync.
//
//go:embed skills
var shippableSkillsFS embed.FS

// shippableSkillsRoot is the embed root; paths from fs.WalkDir are prefixed with it.
const shippableSkillsRoot = "skills"

// shippableSkillTargets are the agent-tool skill directories that
// `taskrail init --with-skills` provisions. Writing them is opt-in only; default
// init never creates agent-tool directories.
var shippableSkillTargets = []string{
	filepath.Join(".agents", "skills"),
	filepath.Join(".claude", "skills"),
}

// SkillInstallResult reports what WriteShippableSkills changed on disk so callers
// can show the user exactly which skill files were created, replaced from the
// embedded set, and backed up before replacement. Paths are repo-relative and in
// deterministic walk order.
type SkillInstallResult struct {
	Written     []string // newly created files (no prior copy existed)
	Overwritten []string // existing files replaced from the embedded set (force only)
	BackedUp    []string // timestamped backups written before an overwrite (force only)
}

// packagedSkillFiles lists every embedded skill file, in walk order, as a
// package-relative slash path. It is the one enumeration behind both the
// installer and the reported inventory, so the two cannot disagree about what
// the package contains.
func packagedSkillFiles() ([]string, error) {
	var files []string
	err := fs.WalkDir(shippableSkillsFS, shippableSkillsRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, strings.TrimPrefix(p, shippableSkillsRoot+"/"))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// packagedSkillNames lists the packaged skills' own directory names, which are
// the subtrees an installation owns in each assistant root.
func packagedSkillNames() ([]string, error) {
	entries, err := fs.ReadDir(shippableSkillsFS, shippableSkillsRoot)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	slices.Sort(names)
	return names, nil
}

// skillReport is the machine inventory of one `--with-skills` installation: every
// packaged file at its normal assistant discovery path, plus the skill-subtree
// exclusions the active storage context owns. Committed storage owns none, so a
// committed install reports an empty exclusion list; the local overlay never
// changes a skill's discovery path, only whether Taskrail excludes it.
//
// The installer reports what it created and what it rewrote, so anything else on
// the packaged list was already in place and is preserved.
func (s *Service) skillReport(installed SkillInstallResult) ([]InitSkill, []InitSkillExclusion, error) {
	files, err := packagedSkillFiles()
	if err != nil {
		return nil, nil, err
	}
	created := pathSet(installed.Written)
	refreshed := pathSet(installed.Overwritten)

	skills := make([]InitSkill, 0, len(files)*len(shippableSkillTargets))
	for _, target := range shippableSkillTargets {
		for _, file := range files {
			dest := path.Join(filepath.ToSlash(target), file)
			skills = append(skills, InitSkill{Path: dest, Action: skillAction(dest, created, refreshed)})
		}
	}
	slices.SortFunc(skills, func(a, b InitSkill) int { return strings.Compare(a.Path, b.Path) })

	exclusions, err := s.skillExclusions()
	if err != nil {
		return nil, nil, err
	}
	return skills, exclusions, nil
}

func skillAction(dest string, created, refreshed map[string]struct{}) string {
	if _, ok := created[dest]; ok {
		return writeActionCreate
	}
	if _, ok := refreshed[dest]; ok {
		return writeActionRefresh
	}
	return writeActionPreserve
}

// skillExclusions reports one exact exclusion per packaged-skill subtree in each
// supported assistant root, in path order. Only local storage owns them:
// `.agents/`, `.claude/`, and either shared `skills/` directory are never
// excluded, and a committed installation excludes nothing at all
// (specs/v0.5.0.md#local-planning-mode).
func (s *Service) skillExclusions() ([]InitSkillExclusion, error) {
	if s.paths.Storage.Mode != StorageLocal {
		return []InitSkillExclusion{}, nil
	}
	names, err := packagedSkillNames()
	if err != nil {
		return nil, err
	}
	exclusions := make([]InitSkillExclusion, 0, len(names)*len(shippableSkillTargets))
	for _, target := range shippableSkillTargets {
		for _, name := range names {
			subtree := path.Join(filepath.ToSlash(target), name)
			action := writeActionCreate
			if dirExists(filepath.Join(s.paths.RepoRoot, filepath.FromSlash(subtree))) {
				action = writeActionPreserve
			}
			exclusions = append(exclusions, InitSkillExclusion{Path: subtree, Action: action})
		}
	}
	slices.SortFunc(exclusions, func(a, b InitSkillExclusion) int { return strings.Compare(a.Path, b.Path) })
	return exclusions, nil
}

func pathSet(paths []string) map[string]struct{} {
	set := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		set[filepath.ToSlash(p)] = struct{}{}
	}
	return set
}

// WriteShippableSkills materializes the embedded skill set into the agent-tool
// skill directories, stamping each file with the writing binary's version.
//
// Without force it is non-destructive (writeFileIfMissing semantics, consistent
// with the T-019 Init): an existing skill is left untouched, so upgrading the
// binary never refreshes materialized copies. With force it reinstalls the
// embedded copy over an existing file whose content differs, first backing up the
// on-disk version to a timestamped sibling so a local edit stays recoverable. A
// file already identical to the stamped embedded copy is skipped in both modes, so
// a force run of the same version over an unmodified install writes nothing and
// accumulates no backups; a force run from a different version restamps.
func (s *Service) WriteShippableSkills(version string, force bool) (SkillInstallResult, error) {
	type skillWrite struct {
		dest string
		data []byte
	}
	files, err := packagedSkillFiles()
	if err != nil {
		return SkillInstallResult{}, err
	}
	var writes []skillWrite
	for _, rel := range files {
		p := path.Join(shippableSkillsRoot, rel)
		data, err := shippableSkillsFS.ReadFile(p)
		if err != nil {
			return SkillInstallResult{}, fmt.Errorf("read embedded skill %s: %w", p, err)
		}
		if err := validateAgentSkill(data); err != nil {
			return SkillInstallResult{}, fmt.Errorf("validate embedded skill %s: %w", p, err)
		}
		data, err = stampSkillVersion(data, version)
		if err != nil {
			return SkillInstallResult{}, fmt.Errorf("stamp embedded skill %s: %w", p, err)
		}
		for _, target := range shippableSkillTargets {
			writes = append(writes, skillWrite{dest: filepath.Join(s.paths.RepoRoot, target, filepath.FromSlash(rel)), data: data})
		}
	}

	// Validate every existing marker before the first write so conflicting or
	// malformed version evidence cannot leave a partially refreshed skill set.
	for _, write := range writes {
		existing, err := os.ReadFile(write.dest)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return SkillInstallResult{}, fmt.Errorf("read %s: %w", relPath(s.paths.RepoRoot, write.dest), fsCause(err))
		}
		if _, err := skillVersionOf(existing); err != nil {
			return SkillInstallResult{}, fmt.Errorf("read skill marker %s: %w", relPath(s.paths.RepoRoot, write.dest), err)
		}
	}

	var res SkillInstallResult
	for _, write := range writes {
		if err := s.installSkillFile(write.dest, write.data, force, &res); err != nil {
			return res, err
		}
	}
	// Return the partial result even on error so callers can report what was
	// installed before a mid-walk failure rather than hiding the partial state.
	return res, nil
}

// installSkillFile writes a single embedded skill file to dest, honoring the
// force/backup contract described on WriteShippableSkills.
func (s *Service) installSkillFile(dest string, data []byte, force bool, res *SkillInstallResult) error {
	existing, statErr := os.ReadFile(dest)
	switch {
	case statErr == nil:
		if bytes.Equal(existing, data) {
			return nil // already current; nothing to do in either mode
		}
		if !force {
			return nil // non-destructive: never clobber a user-edited skill
		}
		backup, err := s.backupPath(dest)
		if err != nil {
			return err
		}
		if err := os.WriteFile(backup, existing, 0o644); err != nil {
			return fmt.Errorf("write backup %s: %w", relPath(s.paths.RepoRoot, backup), fsCause(err))
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", relPath(s.paths.RepoRoot, dest), fsCause(err))
		}
		res.BackedUp = append(res.BackedUp, relPath(s.paths.RepoRoot, backup))
		res.Overwritten = append(res.Overwritten, relPath(s.paths.RepoRoot, dest))
		return nil
	case errors.Is(statErr, os.ErrNotExist):
		if err := ensureDir(s.paths.RepoRoot, filepath.Dir(dest)); err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", relPath(s.paths.RepoRoot, dest), fsCause(err))
		}
		res.Written = append(res.Written, relPath(s.paths.RepoRoot, dest))
		return nil
	default:
		return fmt.Errorf("read %s: %w", relPath(s.paths.RepoRoot, dest), fsCause(statErr))
	}
}

// backupPath returns a timestamped sibling of dest that does not yet exist. The
// timestamp gives an upgrade-ordered name; the numeric suffix disambiguates
// backups minted within the same second so a second force run never clobbers an
// earlier backup.
func (s *Service) backupPath(dest string) (string, error) {
	base := dest + ".bak." + s.now().UTC().Format("20060102T150405Z")
	candidate := base
	for i := 1; ; i++ {
		_, err := os.Stat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
		if err != nil {
			return "", fmt.Errorf("stat backup %s: %w", relPath(s.paths.RepoRoot, candidate), fsCause(err))
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
}
