package taskrail

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestCorpusTasksHaveOneOfEachScaffoldSection holds the invariant T-119 healed at
// zero, widened in T-135 from Implementation Notes to every scaffold section. The
// writer that duplicated Implementation Notes is fixed (T-117), but a task body is
// also hand-editable — T-130 shipped with two `## Verification Notes` headings —
// and that reason covers every section, not one. Asserting over the real
// `planning/tasks/` rather than a fixture is the point: a fixture cannot regress,
// and this corpus is where the damage accumulated.
func TestCorpusTasksHaveOneOfEachScaffoldSection(t *testing.T) {
	pattern := filepath.Join("..", "..", "planning", "tasks", "*.md")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob %s: %v", pattern, err)
	}
	// A corpus that yields no task files would make the loop below pass without
	// checking anything.
	if len(paths) == 0 {
		t.Fatalf("no task files matched %s; the invariant was not actually checked", pattern)
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, dup := range duplicateScaffoldSections(string(data)) {
			t.Errorf("%s has %d %q headings, want at most 1", filepath.Base(path), dup.count, dup.heading)
		}
	}
}

// TestDuplicateScaffoldSectionsFlagsRepeatedHeadings covers what the corpus test
// cannot: the corpus is expected to stay clean, so only a fixture can show the
// rule still fires, still ignores prose, and still leaves non-scaffold headings
// alone.
func TestDuplicateScaffoldSectionsFlagsRepeatedHeadings(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []duplicateSection
	}{
		{
			name: "clean scaffold",
			body: renderNewTaskBody("T-001", "Example", ""),
		},
		{
			name: "duplicate verification notes",
			body: "## Verification Notes\n\n- a\n\n## Verification Notes\n",
			want: []duplicateSection{{heading: "## Verification Notes", count: 2}},
		},
		{
			name: "duplicate implementation notes",
			body: "## Implementation Notes\n\n- a\n\n## Implementation Notes\n",
			want: []duplicateSection{{heading: "## Implementation Notes", count: 2}},
		},
		{
			name: "several sections duplicated",
			body: "## Description\n\n## Description\n\n## Acceptance\n\n## Acceptance\n\n## Acceptance\n",
			want: []duplicateSection{
				{heading: "## Description", count: 2},
				{heading: "## Acceptance", count: 3},
			},
		},
		{
			name: "trailing whitespace still counts as the heading",
			body: "## Acceptance\n\n## Acceptance \t\n",
			want: []duplicateSection{{heading: "## Acceptance", count: 2}},
		},
		{
			name: "CRLF line endings still count as the heading",
			body: "## Acceptance\r\n\r\n## Acceptance\r\n",
			want: []duplicateSection{{heading: "## Acceptance", count: 2}},
		},
		{
			name: "headings inside fenced code blocks are ignored",
			body: "## Acceptance\n\n```markdown\n## Acceptance\n## Description\n```\n\n## Acceptance\n",
			want: []duplicateSection{{heading: "## Acceptance", count: 2}},
		},
		{
			name: "backtick in info string is not a fence",
			body: "## Acceptance\n\n```markdown`bad\n## Acceptance\n```\n",
			want: []duplicateSection{{heading: "## Acceptance", count: 2}},
		},
		{
			name: "prose mention is not a heading",
			body: "## Acceptance\n\n- The `## Acceptance` section stays single.\n- See ## Acceptance above.\n",
		},
		{
			name: "deeper heading with the same words is out of scope",
			body: "## Acceptance\n\n### Acceptance\n\n### Acceptance\n",
		},
		{
			name: "repeated non-scaffold heading is allowed",
			body: "## Description\n\n## Notes\n\n## Notes\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := duplicateScaffoldSections(tc.body)
			if !slices.Equal(got, tc.want) {
				t.Errorf("duplicateScaffoldSections() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestScaffoldSectionHeadingsMatchScaffoldOutput ties the guarded heading list to
// the scaffold it claims to mirror: a section added to or renamed in
// renderNewTaskBody would otherwise silently fall out of the guard's scope.
func TestScaffoldSectionHeadingsMatchScaffoldOutput(t *testing.T) {
	var rendered []string
	for _, line := range strings.Split(renderNewTaskBody("T-001", "Example", ""), "\n") {
		if strings.HasPrefix(line, "## ") {
			rendered = append(rendered, strings.TrimRight(line, " \t"))
		}
	}

	want := slices.Clone(scaffoldSectionHeadings)
	slices.Sort(want)
	slices.Sort(rendered)
	if !slices.Equal(rendered, want) {
		t.Errorf("renderNewTaskBody sections = %q, guarded headings = %q", rendered, want)
	}
}

// duplicateSection reports a scaffold heading that a task body carries more than
// once, with the count so the failure can name it.
type duplicateSection struct {
	heading string
	count   int
}

// duplicateScaffoldSections returns the scaffold headings body repeats, in
// scaffoldSectionHeadings order. It mirrors hasImplementationNotesHeading's
// whole-line match, so it counts what the note writer treats as a section and
// never a prose mention of the same words. It trims `\r` on top of that, so a
// CRLF-authored task file cannot slip a duplicate past the corpus guard — a
// detector may be stricter than the writer, never laxer.
func duplicateScaffoldSections(body string) []duplicateSection {
	counts := make(map[string]int)
	for _, line := range markdownLinesWithoutFencedContent(body) {
		counts[strings.TrimRight(line, " \t\r")]++
	}

	var dups []duplicateSection
	for _, heading := range scaffoldSectionHeadings {
		if counts[heading] > 1 {
			dups = append(dups, duplicateSection{heading: heading, count: counts[heading]})
		}
	}
	return dups
}
