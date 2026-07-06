package taskrail

import (
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

// WriteShippableSkills materializes the embedded skill set into the agent-tool
// skill directories. It uses writeFileIfMissing semantics, so a re-run never
// clobbers a user-edited skill, consistent with the non-destructive Init in
// T-019. It returns the repo-relative paths it newly wrote, in deterministic
// order, so callers can report exactly what was added.
func (s *Service) WriteShippableSkills() ([]string, error) {
	var written []string
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
			// Non-destructive skip: an existing skill (even user-edited) is
			// never overwritten. Mirror writeFileIfMissing's stat handling so a
			// non-ErrNotExist stat failure surfaces instead of being read as absent.
			if _, statErr := os.Stat(dest); statErr == nil {
				continue
			} else if !errors.Is(statErr, os.ErrNotExist) {
				return fmt.Errorf("stat %s: %w", dest, statErr)
			}
			if err := ensureDir(filepath.Dir(dest)); err != nil {
				return err
			}
			if err := os.WriteFile(dest, data, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", dest, err)
			}
			written = append(written, relPath(s.paths.RepoRoot, dest))
		}
		return nil
	})
	// Return the partial list even on error so callers can report what was
	// installed before a mid-walk failure rather than hiding the partial state.
	return written, err
}
