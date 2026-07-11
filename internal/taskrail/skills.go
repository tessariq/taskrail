package taskrail

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

// WriteShippableSkills materializes the embedded skill set into the agent-tool
// skill directories.
//
// Without force it is non-destructive (writeFileIfMissing semantics, consistent
// with the T-019 Init): an existing skill is left untouched, so upgrading the
// binary never refreshes materialized copies. With force it reinstalls the
// embedded copy over an existing file whose content differs, first backing up the
// on-disk version to a timestamped sibling so a local edit stays recoverable. A
// file already identical to the embedded copy is skipped in both modes, so a
// force run over an unmodified install writes nothing and accumulates no backups.
func (s *Service) WriteShippableSkills(force bool) (SkillInstallResult, error) {
	var res SkillInstallResult
	err := fs.WalkDir(shippableSkillsFS, shippableSkillsRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := shippableSkillsFS.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read embedded skill %s: %w", p, err)
		}
		rel := strings.TrimPrefix(p, shippableSkillsRoot+"/")
		for _, target := range shippableSkillTargets {
			dest := filepath.Join(s.paths.RepoRoot, target, filepath.FromSlash(rel))
			if err := s.installSkillFile(dest, data, force, &res); err != nil {
				return err
			}
		}
		return nil
	})
	// Return the partial result even on error so callers can report what was
	// installed before a mid-walk failure rather than hiding the partial state.
	return res, err
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
			return fmt.Errorf("write backup %s: %w", backup, err)
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		res.BackedUp = append(res.BackedUp, relPath(s.paths.RepoRoot, backup))
		res.Overwritten = append(res.Overwritten, relPath(s.paths.RepoRoot, dest))
		return nil
	case errors.Is(statErr, os.ErrNotExist):
		if err := ensureDir(filepath.Dir(dest)); err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
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
			return "", fmt.Errorf("stat backup %s: %w", candidate, err)
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
}
