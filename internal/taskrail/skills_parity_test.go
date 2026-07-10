package taskrail

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// committedSkillTargets are the agent-tool skill trees this repository keeps
// committed for a zero-setup clone. They are the production install targets
// (shippableSkillTargets) resolved relative to this package, so the list stays
// authoritative rather than a second copy. They must stay byte-identical to the
// embedded package the binary installs via `init --with-skills`; the parity check
// below replaces the retired three-way `check-skill-mirrors.sh` diff (T-055).
var committedSkillTargets = repoRelativeSkillTargets()

func repoRelativeSkillTargets() []string {
	targets := make([]string, len(shippableSkillTargets))
	for i, target := range shippableSkillTargets {
		targets[i] = filepath.Join("..", "..", target)
	}
	return targets
}

// embeddedSkillFiles returns the embedded package as a rel-path -> content map,
// the ground truth `init --with-skills` materializes.
func embeddedSkillFiles(t *testing.T) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := fs.WalkDir(shippableSkillsFS, shippableSkillsRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := shippableSkillsFS.ReadFile(p)
		if err != nil {
			return err
		}
		files[strings.TrimPrefix(p, shippableSkillsRoot+"/")] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded skills: %v", err)
	}
	return files
}

// TestCommittedSkillsMatchPackage asserts each committed copy equals the embedded
// `--with-skills` output exactly: every embedded file present and byte-identical,
// and no committed file without an embedded counterpart. This single parity
// contract is what lets this repository adopt the packaged skills rather than
// maintain a bespoke skills/ source with a three-way mirror diff.
func TestCommittedSkillsMatchPackage(t *testing.T) {
	embedded := embeddedSkillFiles(t)

	for _, target := range committedSkillTargets {
		seen := map[string]bool{}
		err := filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			// embed omits dotfiles; skip them on disk too (e.g. .DS_Store) so the
			// check tracks tracked skill content, not local editor cruft.
			if d.IsDir() || strings.HasPrefix(d.Name(), ".") {
				return nil
			}
			rel, err := filepath.Rel(target, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			seen[rel] = true
			want, ok := embedded[rel]
			if !ok {
				t.Errorf("%s: committed %s has no embedded counterpart; remove it or add it to the package", target, rel)
				return nil
			}
			got, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if string(got) != want {
				t.Errorf("%s: committed %s diverges from the embedded package; regenerate with `task skills:regen`", target, rel)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", target, err)
		}
		for rel := range embedded {
			if !seen[rel] {
				t.Errorf("%s: embedded %s is missing from the committed copy", target, rel)
			}
		}
	}
}
