package durablefs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func countDirectoryReads(t *testing.T, run func()) int {
	t.Helper()
	count := 0
	testHookDirectoryRead = func() { count++ }
	t.Cleanup(func() { testHookDirectoryRead = nil })
	run()
	return count
}

func writeCorpus(t *testing.T, dir string, files int) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range files {
		name := filepath.Join(dir, fmt.Sprintf("T-%04d.md", i))
		if err := os.WriteFile(name, []byte("body\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Observing one directory must cost a bounded number of directory reads, not one
// full listing per leaf: the per-leaf scan is what makes a transaction over a few
// hundred managed files quadratic.
func TestObserveTreeReadsDirectoryBoundedTimes(t *testing.T) {
	base := t.TempDir()
	const files = 300
	writeCorpus(t, filepath.Join(base, "tasks"), files)

	var snapshot TreeSnapshot
	reads := countDirectoryReads(t, func() {
		var err error
		snapshot, err = ObserveTree(base, "tasks")
		if err != nil {
			t.Fatal(err)
		}
	})
	if len(snapshot.Entries) != files {
		t.Fatalf("observed %d entries, want %d", len(snapshot.Entries), files)
	}
	if reads > 16 {
		t.Fatalf("observing %d files performed %d directory reads, want a bounded count", files, reads)
	}
}

// The bounded listing must not weaken alias strictness, including for a folded
// pair that is not adjacent in byte order.
func TestInspectDirectoryDetectsNonAdjacentFoldedCollision(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "tasks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"A", "AB", "a"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Skipf("case-sensitive filesystem required: %v", err)
		}
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 3 {
		t.Skipf("case-sensitive filesystem required (%d entries, %v)", len(entries), err)
	}
	if _, err := ObserveTree(base, "tasks"); err == nil {
		t.Fatal("expected an alias refusal for a folded collision")
	}
}
