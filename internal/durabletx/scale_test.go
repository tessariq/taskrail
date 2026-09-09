package durabletx

import (
	"context"
	"fmt"
	"testing"

	"github.com/tessariq/taskrail/internal/durablefs"
)

// A transaction over many members in one directory must observe that directory
// a bounded number of times per pass. Observing it once per member re-read and
// re-digested every sibling, which is what made an upgrade of a few hundred
// managed files unusable.
func TestRunObservesOneDirectoryBoundedTimesPerPass(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())

	const members = 40
	requested := make([]Member, 0, members)
	for i := range members {
		reported := fmt.Sprintf("planning/tasks/T-%03d.md", i)
		seed(t, repo, reported, "before")
		requested = append(requested, member(reported, "after"))
	}

	observations := 0
	original := observeParentTree
	observeParentTree = func(base, parent string) (durablefs.TreeSnapshot, error) {
		observations++
		return original(base, parent)
	}
	t.Cleanup(func() { observeParentTree = original })

	if _, err := Run(context.Background(), lock, repo, request("init", requested...)); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for i := range members {
		reported := fmt.Sprintf("planning/tasks/T-%03d.md", i)
		if got, ok := read(t, repo, reported); !ok || got != "after" {
			t.Fatalf("%s = %q (present=%t), want the candidate", reported, got, ok)
		}
	}
	if observations > 4*members {
		t.Fatalf("publishing %d members observed the parent directory %d times, want a bounded count per pass", members, observations)
	}
}
