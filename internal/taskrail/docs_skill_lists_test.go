package taskrail

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// docSkillListLine matches a markdown list bullet or table row whose first item
// is a backtick-wrapped skill name, e.g. "- `autonomous-backlog`" or
// "| `taskrail-import` | ...". Prose lines and other backtick spans (commands,
// paths) never lead with this shape, so the capture is exactly a listed skill.
var docSkillListLine = regexp.MustCompile("^\\s*[-|]\\s*`([a-z][a-z0-9-]*)`")

// documentedSkillSet returns the skills listed under the given "## " headings of
// a workflow doc. Extraction is scoped to those sections so backtick-leading
// bullets elsewhere in the file cannot pollute the set.
func documentedSkillSet(t *testing.T, path string, headings ...string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	set := map[string]bool{}
	inSection := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "## ") {
			inSection = slices.Contains(headings, strings.TrimSpace(line))
			continue
		}
		if strings.HasPrefix(line, "# ") {
			inSection = false
			continue
		}
		if !inSection {
			continue
		}
		if m := docSkillListLine.FindStringSubmatch(line); m != nil {
			set[m[1]] = true
		}
	}
	return set
}

// TestDocsSkillListsMatchShippableSet guards the hand-maintained skill lists in
// the workflow docs against the authoritative Go sets (shippableSkills /
// dogfoodingOnlySkills). It bites when a skill graduation updates the code but
// not both docs (or vice versa): every shippable skill must be listed, no
// dogfooding-only skill may appear, and no doc may list a skill absent from the
// authoritative set. The docs stay human-authored markdown; this only asserts
// agreement, it does not generate them.
func TestDocsSkillListsMatchShippableSet(t *testing.T) {
	docs := []struct {
		path     string
		headings []string
	}{
		{
			filepath.Join("..", "..", "docs", "workflow", "skills-overview.md"),
			[]string{"## Packaged Skills"},
		},
		{
			filepath.Join("..", "..", "docs", "workflow", "skills-productization.md"),
			[]string{"## The Packaged Skill Set", "## Onboarding Skills"},
		},
	}

	shippable := map[string]bool{}
	for _, name := range shippableSkills {
		shippable[name] = true
	}

	for _, doc := range docs {
		documented := documentedSkillSet(t, doc.path, doc.headings...)
		for _, name := range shippableSkills {
			if !documented[name] {
				t.Errorf("%s: shippable skill %q missing from its skill list; update the doc (or shippableSkills)", doc.path, name)
			}
		}
		for _, name := range dogfoodingOnlySkills {
			if documented[name] {
				t.Errorf("%s: dogfooding-only skill %q must not appear in a shippable skill list", doc.path, name)
			}
		}
		for name := range documented {
			if !shippable[name] {
				t.Errorf("%s: skill list documents %q, absent from shippableSkills; remove it or add it to the Go list", doc.path, name)
			}
		}
	}
}
