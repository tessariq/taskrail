package taskrail

import "testing"

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
