package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCorpusTasksHaveOneImplementationNotesSection holds the invariant T-119 healed
// at zero. The writer that duplicated the heading is fixed (T-117), but a task body
// is also hand-editable, so the only thing that keeps the corpus clean is checking
// it. Asserting over the real `planning/tasks/` rather than a fixture is the point:
// a fixture cannot regress, and this corpus is where the damage accumulated.
func TestCorpusTasksHaveOneImplementationNotesSection(t *testing.T) {
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
		if got := countImplementationNotesHeadings(string(data)); got > 1 {
			t.Errorf("%s has %d %q headings, want at most 1", filepath.Base(path), got, implementationNotesHeading)
		}
	}
}

// countImplementationNotesHeadings mirrors hasImplementationNotesHeading's whole-line
// match, so it counts what the note writer treats as the section and never a prose
// mention of the same words.
func countImplementationNotesHeadings(body string) int {
	var count int
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimRight(line, " \t") == implementationNotesHeading {
			count++
		}
	}
	return count
}
