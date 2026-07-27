package taskrail

import (
	"strings"
	"testing"
)

// TestAppendTaskNoteReusesExistingSection covers the shapes a task body arrives in
// when verify/block append a note. The load-bearing case is a body ending at the
// bare `## Implementation Notes` heading — exactly what renderNewTaskBody scaffolds
// — which the old heading+blank-line probe missed, so every first note stamped a
// second copy of the heading into a committed file.
func TestAppendTaskNoteReusesExistingSection(t *testing.T) {
	t.Parallel()

	const note = "- 2026-07-27T00:00:00Z: verification pass"
	cases := []struct {
		name         string
		body         string
		wantHeadings int
		wantContains []string
	}{
		{
			name:         "scaffolded body ending at the heading",
			body:         "# T-001 Task\n\n## Implementation Notes\n",
			wantHeadings: 1,
			wantContains: []string{"## Implementation Notes\n\n" + note},
		},
		{
			name:         "section with an existing note appends at the end",
			body:         "# T-001 Task\n\n## Implementation Notes\n\n- earlier note\n",
			wantHeadings: 1,
			wantContains: []string{"- earlier note\n" + note},
		},
		{
			name:         "body without the section gets one scaffolded",
			body:         "# T-001 Task\n\n## Description\n\nBody.\n",
			wantHeadings: 1,
			wantContains: []string{"## Implementation Notes\n\n" + note},
		},
		{
			name:         "a longer heading is not the section",
			body:         "# T-001 Task\n\n## Implementation Notes For Reviewers\n",
			wantHeadings: 1,
			wantContains: []string{"## Implementation Notes For Reviewers", "## Implementation Notes\n\n" + note},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			task := &Task{Body: tc.body}
			appendTaskNote(task, note)

			if got := strings.Count(task.Body, "## Implementation Notes\n"); got != tc.wantHeadings {
				t.Fatalf("got %d %q headings, want %d:\n%s", got, "## Implementation Notes", tc.wantHeadings, task.Body)
			}
			for _, want := range tc.wantContains {
				if !strings.Contains(task.Body, want) {
					t.Fatalf("body missing %q:\n%s", want, task.Body)
				}
			}
		})
	}
}

// TestAppendTaskNoteTwiceKeepsOneHeading is the regression the corpus shows: notes
// accumulate over a task's life (block, then verify), and every append after the
// first must land in the section the first one established.
func TestAppendTaskNoteTwiceKeepsOneHeading(t *testing.T) {
	t.Parallel()

	task := &Task{Body: "# T-001 Task\n\n## Implementation Notes\n"}
	appendTaskNote(task, "- first")
	appendTaskNote(task, "- second")

	if got := strings.Count(task.Body, "## Implementation Notes"); got != 1 {
		t.Fatalf("got %d headings after two notes, want 1:\n%s", got, task.Body)
	}
	if !strings.Contains(task.Body, "- first\n- second\n") {
		t.Fatalf("notes not appended in order:\n%s", task.Body)
	}
}

// nextTaskID derives the next number from the numeric prefix (^T-(\d+)) of every
// task id, so slug-suffixed ids (T-076-ingestion-commands) allocate correctly and
// bare ids keep their existing behavior.
func TestNextTaskIDNumericPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		ids  []string
		want string
	}{
		{"empty set starts at one", nil, "T-001"},
		{"bare ids stay bare", []string{"T-001", "T-084"}, "T-085"},
		{"slug-suffixed max prefix", []string{"T-001-milestone-v0.1.0", "T-102-quality-check-cli-command"}, "T-103"},
		{"mixed bare and slug", []string{"T-084", "T-076-ingestion-commands", "T-100-thing"}, "T-101"},
		{"non-task ids ignored", []string{"NOTE-9", "T-005"}, "T-006"},
		{"digits must end at id or hyphen boundary", []string{"T-1abc"}, "T-001"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tasks := make([]*Task, 0, len(tc.ids))
			for _, id := range tc.ids {
				tasks = append(tasks, &Task{Frontmatter: TaskFrontmatter{ID: id}})
			}
			if got := nextTaskID(tasks); got != tc.want {
				t.Fatalf("nextTaskID(%v) = %s, want %s", tc.ids, got, tc.want)
			}
		})
	}
}
