package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// markdownSection returns the body of the Markdown section introduced by heading,
// up to the next same-or-higher-level heading. Empty if the heading is absent.
// Scoping an assertion to one section keeps it from passing on a phrase that
// drifted elsewhere in the document.
func markdownSection(doc, heading string) string {
	start := strings.Index(doc, heading)
	if start < 0 {
		return ""
	}
	body := doc[start+len(heading):]
	for _, marker := range []string{"\n## ", "\n### "} {
		if end := strings.Index(body, marker); end >= 0 {
			body = body[:end]
		}
	}
	return body
}

func TestGapGatingContractIsDocumentedAtSources(t *testing.T) {
	for _, tc := range []struct {
		path    string
		heading string
	}{
		{filepath.Join("..", "..", "specs", "v0.4.0.md"), "### Gap Analysis"},
		{filepath.Join("skills", "taskrail-gap", "SKILL.md"), "## The mechanical-vs-semantic split"},
		{filepath.Join("..", "..", "AGENTS.md"), "## Notes On Repository Behavior"},
	} {
		data, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("read %s: %v", tc.path, err)
		}
		section := markdownSection(string(data), tc.heading)
		if section == "" {
			t.Fatalf("%s missing %q section", tc.path, tc.heading)
		}
		section = strings.Join(strings.Fields(section), " ")
		for _, phrase := range []string{"advisory by default", "--fail-on", "exit code", "never make `validate` fail"} {
			if !strings.Contains(section, phrase) {
				t.Errorf("%s section %q missing %q", tc.path, tc.heading, phrase)
			}
		}
	}
}

// markdownBullet returns the body of the top-level list bullet that begins with
// marker (e.g. a "- `taskrail-gap`" entry), up to the next top-level bullet or the
// blank line that ends the list. Empty if the marker is absent. Scoping the assertion
// this way — mirroring readmeSection's discipline — keeps the check from passing on a
// phrase that drifted to an unrelated part of the doc.
func markdownBullet(doc, marker string) string {
	start := strings.Index(doc, marker)
	if start < 0 {
		return ""
	}
	body := doc[start+len(marker):]
	for _, end := range []string{"\n\n", "\n- "} {
		if i := strings.Index(body, end); i >= 0 {
			body = body[:i]
		}
	}
	return body
}

// TestSkillsOverviewStatesGapMechanicalLimit guards that the workflow docs where the
// `taskrail-gap` skill is catalogued also state the mechanical-only limit and the
// structural(binary)/semantic(skill) split, so an operator reading either the README
// or the skills doc learns the boundary (T-102 acceptance: README + relevant
// docs/workflow). The assertion is scoped to the `taskrail-gap` bullet so a phrase
// surviving elsewhere in the doc cannot mask its removal from that entry.
func TestSkillsOverviewStatesGapMechanicalLimit(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "workflow", "skills-overview.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	bullet := markdownBullet(string(data), "- `taskrail-gap`")
	if bullet == "" {
		t.Fatalf("skills-overview.md missing the `taskrail-gap` bullet")
	}

	for _, m := range []struct {
		phrase string
		why    string
	}{
		{"coverage --gaps", "the structural half runs in the binary"},
		{"mechanical", "the binary's gap signals stay mechanical"},
		{"never semantic", "the binary never infers semantic gaps"},
	} {
		if !strings.Contains(bullet, m.phrase) {
			t.Errorf("`taskrail-gap` bullet missing %q (%s)", m.phrase, m.why)
		}
	}
}
