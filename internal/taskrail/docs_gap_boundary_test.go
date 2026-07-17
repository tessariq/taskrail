package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gapBoundaryHeading is the README subsection T-102 requires. Markers are asserted
// within this section (see readmeSection), so deleting the section fails the test
// even if a marker phrase survives elsewhere in the README.
const gapBoundaryHeading = "### Coverage vs gap analysis"

// TestReadmeDocumentsGapBoundary guards the T-102 requirement that the boundary
// between `coverage` and `coverage --gaps`, and the hard mechanical-only limit of
// `--gaps`, are documented where operators meet the commands — so nobody folds gap
// into coverage or expects semantic "this needs a test" inference from the binary
// (specs/v0.4.0.md#gap-analysis, #explicitly-excluded). It checks presence of the
// load-bearing phrases, not exact prose.
func TestReadmeDocumentsGapBoundary(t *testing.T) {
	section := readmeSection(readReadme(t), gapBoundaryHeading)
	if section == "" {
		t.Fatalf("README.md missing %q section", gapBoundaryHeading)
	}

	markers := []struct {
		phrase string
		why    string
	}{
		{"linked to any task", "the coverage question: is this spec area linked to any task"},
		{"coverage --gaps", "the gaps command surface"},
		{"under-decomposed", "one of the structural gap signal families"},
		{"candidates, not violations", "gap signals are candidates to promote, not failures"},
		{"false positives are expected", "the mechanical-only limit tolerates false positives"},
		{"semantic", "the hard limit: no semantic inference from the binary"},
		{"read-only", "--gaps is read-only"},
		{"advisory", "--gaps is advisory by default"},
		{"--fail-on", "the opt-in gate (T-100)"},
		{"taskrail-gap", "the skill that supplies the semantic half (T-101)"},
	}
	for _, m := range markers {
		if !strings.Contains(section, m.phrase) {
			t.Errorf("%q section missing %q (%s)", gapBoundaryHeading, m.phrase, m.why)
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
