package repolock

import "testing"

func TestCapabilityAllowsTaskScopedArtifactGrant(t *testing.T) {
	outer := Capability{Commands: []string{"verify"}, SelectedTask: "T-001", Writes: []string{"planning/STATE.md", "planning/tasks/", "planning/artifacts/verify/T-001/"}}
	inner := Capability{Commands: []string{"verify"}, SelectedTask: "T-001", Writes: []string{"planning/STATE.md", "planning/tasks/T-002-followup.md", "planning/artifacts/verify/T-001/20260823T120000Z-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/report.json"}}
	if _, err := outer.Narrow(inner); err != nil {
		t.Fatalf("task-scoped artifact grant rejected: %v", err)
	}
	if err := outer.AllowsWrites(inner.Writes); err != nil {
		t.Fatalf("task-scoped artifact write rejected: %v", err)
	}
	other := Capability{Commands: []string{"verify"}, SelectedTask: "T-001", Writes: []string{"planning/artifacts/verify/T-002/report.json"}}
	if _, err := outer.Narrow(other); err == nil {
		t.Fatal("task-scoped artifact grant accepted another task")
	}
}
