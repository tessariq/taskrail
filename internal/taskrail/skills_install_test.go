package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// hasSkillTree reports whether any file exists under the given agent-tool skill
// directory in the repo snapshot.
func hasSkillTree(tree map[string]string, dir string) bool {
	prefix := filepath.ToSlash(dir) + "/"
	for rel := range tree {
		if strings.HasPrefix(filepath.ToSlash(rel), prefix) {
			return true
		}
	}
	return false
}

// Default init must never provision agent-tool skill directories; writing them
// is opt-in via --with-skills (skills-productization.md Decision 2).
func TestInitDefaultWritesNoSkillDirs(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.Init(false); err != nil {
		t.Fatalf("init: %v", err)
	}

	tree := snapshotTree(t, repo)
	for _, dir := range []string{".agents/skills", ".claude/skills"} {
		if hasSkillTree(tree, dir) {
			t.Errorf("default init wrote skill directory %s; must be opt-in", dir)
		}
	}
}

func TestWriteShippableSkillsInstallsToTargets(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(false); err != nil {
		t.Fatalf("init: %v", err)
	}

	written, err := svc.WriteShippableSkills()
	if err != nil {
		t.Fatalf("write shippable skills: %v", err)
	}
	if len(written) == 0 {
		t.Fatal("write shippable skills reported no files written")
	}

	for _, target := range []string{".agents/skills", ".claude/skills"} {
		for _, name := range shippableSkills {
			path := filepath.Join(repo, target, name, "SKILL.md")
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("expected installed skill %s/%s: %v", target, name, readErr)
			}
			if strings.TrimSpace(string(data)) == "" {
				t.Errorf("installed skill %s/%s is empty", target, name)
			}
			if strings.Contains(string(data), "go run") {
				t.Errorf("installed skill %s/%s references 'go run'", target, name)
			}
		}
	}

	// Dogfooding-only skills must never be installed.
	for _, target := range []string{".agents/skills", ".claude/skills"} {
		for _, name := range dogfoodingOnlySkills {
			if _, statErr := os.Stat(filepath.Join(repo, target, name, "SKILL.md")); statErr == nil {
				t.Errorf("dogfooding-only skill %s must not be installed under %s", name, target)
			}
		}
	}
}

// A re-run is non-destructive: it never clobbers a user-edited skill and reports
// nothing newly written (writeFileIfMissing semantics, consistent with T-019).
func TestWriteShippableSkillsIdempotent(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(false); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := svc.WriteShippableSkills(); err != nil {
		t.Fatalf("first write: %v", err)
	}

	edited := filepath.Join(repo, ".claude", "skills", shippableSkills[0], "SKILL.md")
	const userMark = "USER EDIT — do not clobber"
	if err := os.WriteFile(edited, []byte(userMark), 0o644); err != nil {
		t.Fatalf("edit skill: %v", err)
	}

	written, err := svc.WriteShippableSkills()
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if len(written) != 0 {
		t.Errorf("re-run wrote %v; want no files", written)
	}

	data, err := os.ReadFile(edited)
	if err != nil {
		t.Fatalf("read edited skill: %v", err)
	}
	if string(data) != userMark {
		t.Errorf("re-run clobbered user-edited skill; content = %q", string(data))
	}
}
