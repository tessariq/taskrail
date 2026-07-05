package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shippableSkillsDir holds the repo-agnostic skill set embedded and installed by
// `taskrail init --with-skills` (T-030). It is deliberately separate from the
// dogfooding skills under the repo-root skills/ tree, which may keep `go run`.
const shippableSkillsDir = "skills"

// shippableSkills is the exact set promoted to the product surface, per the
// portability contract in docs/workflow/skills-productization.md (T-031).
var shippableSkills = []string{
	"autonomous-backlog",
	"autonomous-task",
	"autonomous-verify",
}

// dogfoodingOnlySkills must never leak into the shippable set: recovery still
// hand-edits authoritative state and manual-test writes an internal-only
// artifact convention, both forbidden for shipped skills.
var dogfoodingOnlySkills = []string{
	"autonomous-recovery",
	"autonomous-manual-test",
}

func shippableSkillPath(name string) string {
	return filepath.Join(shippableSkillsDir, name, "SKILL.md")
}

func readShippableSkill(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(shippableSkillPath(name))
	if err != nil {
		t.Fatalf("read shippable skill %s: %v", name, err)
	}
	return string(data)
}

func TestShippableSkillsExist(t *testing.T) {
	for _, name := range shippableSkills {
		if got := readShippableSkill(t, name); strings.TrimSpace(got) == "" {
			t.Errorf("shippable skill %s is empty", name)
		}
	}
}

// The whole point of the shippable set: it invokes the installed binary, never
// `go run ./cmd/taskrail`, which only resolves inside this source tree.
func TestShippableSkillsNeverUseGoRun(t *testing.T) {
	for _, name := range shippableSkills {
		if strings.Contains(readShippableSkill(t, name), "go run") {
			t.Errorf("shippable skill %s must not reference 'go run'", name)
		}
	}
}

// Shippable skills create tasks through the real command, not hand-authored
// markdown (Decision 3 in the productization contract).
func TestShippableSkillsUseTaskNew(t *testing.T) {
	for _, name := range shippableSkills {
		if !strings.Contains(readShippableSkill(t, name), "taskrail task new") {
			t.Errorf("shippable skill %s must reference 'taskrail task new' for task creation", name)
		}
	}
}

// Dogfooding-only skills stay out of the shippable directory entirely.
func TestDogfoodingOnlySkillsAreNotShipped(t *testing.T) {
	for _, name := range dogfoodingOnlySkills {
		if _, err := os.Stat(shippableSkillPath(name)); err == nil {
			t.Errorf("dogfooding-only skill %s must not appear in the shippable set", name)
		}
	}
}
