package repolock

import "testing"

// The selected-task and write-set bounds exist so a delegate stays on the work
// its parent chose. Their asymmetry is the part worth pinning: an unbounded
// outer capability grants everything, but dropping a bound the outer capability
// set is widening, not narrowing.
func TestSelectedTaskAndWriteBoundsOnlyNarrow(t *testing.T) {
	bound := Capability{
		Commands:     []string{"complete"},
		TaskFields:   []string{"status"},
		SelectedTask: "T-1",
		Writes:       []string{"planning/STATE.md", "planning/tasks/T-1.md"},
	}
	unbounded := Capability{Commands: []string{"complete"}, TaskFields: []string{"status"}}

	for _, tc := range []struct {
		name  string
		outer Capability
		inner Capability
		want  bool
	}{
		{name: "identical bounds", outer: bound, inner: bound, want: true},
		{
			name:  "fewer write paths",
			outer: bound,
			inner: Capability{
				Commands: []string{"complete"}, TaskFields: []string{"status"},
				SelectedTask: "T-1", Writes: []string{"planning/tasks/T-1.md"},
			},
			want: true,
		},
		{name: "dropping the selected task", outer: bound, inner: Capability{
			Commands: []string{"complete"}, TaskFields: []string{"status"},
			Writes: []string{"planning/tasks/T-1.md"},
		}},
		{name: "another selected task", outer: bound, inner: Capability{
			Commands: []string{"complete"}, TaskFields: []string{"status"},
			SelectedTask: "T-2", Writes: []string{"planning/tasks/T-1.md"},
		}},
		{name: "dropping the write set", outer: bound, inner: Capability{
			Commands: []string{"complete"}, TaskFields: []string{"status"}, SelectedTask: "T-1",
		}},
		{name: "an added write path", outer: bound, inner: Capability{
			Commands: []string{"complete"}, TaskFields: []string{"status"},
			SelectedTask: "T-1", Writes: []string{"planning/tasks/T-2.md"},
		}},
		{name: "any bound inside an unbounded outer", outer: unbounded, inner: bound, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.outer.Includes(tc.inner); got != tc.want {
				t.Fatalf("Includes = %v, want %v", got, tc.want)
			}
			_, err := tc.outer.Narrow(tc.inner)
			if (err == nil) != tc.want {
				t.Fatalf("Narrow error = %v, want refusal = %v", err, !tc.want)
			}
		})
	}
}

func TestAllowsTaskAndWritesRefuseOutsideTheBound(t *testing.T) {
	bound := Capability{
		Commands:     []string{"complete"},
		SelectedTask: "T-1",
		Writes:       []string{"planning/STATE.md"},
	}
	unbounded := Capability{Commands: []string{"complete"}}

	if err := bound.AllowsTask("T-1"); err != nil {
		t.Fatalf("AllowsTask on the bound task: %v", err)
	}
	if err := bound.AllowsTask("T-2"); err == nil {
		t.Fatal("AllowsTask accepted a task outside the bound")
	}
	if err := unbounded.AllowsTask("T-9"); err != nil {
		t.Fatalf("AllowsTask on an unbounded capability: %v", err)
	}
	if err := bound.AllowsWrites([]string{"planning/STATE.md"}); err != nil {
		t.Fatalf("AllowsWrites on the bound path: %v", err)
	}
	if err := bound.AllowsWrites([]string{"planning/STATE.md", "planning/tasks/T-2.md"}); err == nil {
		t.Fatal("AllowsWrites accepted a path outside the bound")
	}
	if err := unbounded.AllowsWrites([]string{"anything.md"}); err != nil {
		t.Fatalf("AllowsWrites on an unbounded capability: %v", err)
	}
}
