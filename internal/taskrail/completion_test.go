package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSpecVersionCompletionsListsVersions verifies version completion offers the
// versioned specs under specs/ in version order, so `spec show`/`spec activate`
// argument completion draws on the same discovery as `spec list`.
func TestSpecVersionCompletionsListsVersions(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"), "# v0.2.0\n\n## Summary\n\nNext.\n")
	writeFile(t, filepath.Join(repo, "specs", "notes.md"), "# scratch\n")

	svc := newTestService(t, repo, time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))
	got, err := svc.SpecVersionCompletions()
	if err != nil {
		t.Fatalf("SpecVersionCompletions: %v", err)
	}
	want := []string{"v0.1.0", "v0.2.0"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v (version order)", got, want)
		}
	}
}

// TestSpecRefCompletionsPathPhase verifies that with no anchor typed yet, each
// candidate is a spec path with a trailing '#', ready for the anchor phase.
func TestSpecRefCompletionsPathPhase(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"), "# v0.2.0\n\n## Summary\n\nNext.\n")

	svc := newTestService(t, repo, time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))
	got, err := svc.SpecRefCompletions("")
	if err != nil {
		t.Fatalf("SpecRefCompletions: %v", err)
	}
	want := []string{"specs/v0.1.0.md#", "specs/v0.2.0.md#"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestSpecRefCompletionsAnchorPhaseMatchesValidation is the core invariant: once
// a spec path is fixed, the anchor candidates are exactly the anchors spec_ref
// validation accepts for that spec — reused from the T-062 source, no re-slug.
func TestSpecRefCompletionsAnchorPhaseMatchesValidation(t *testing.T) {
	repo := seedFixtureRepo(t)
	md := "# Taskrail v0.2.0\n\n## Spec Inspection\n\n### Spec Inspection\n\n## Authoring & Commands\n"
	writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"), md)

	svc := newTestService(t, repo, time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))
	got, err := svc.SpecRefCompletions("specs/v0.2.0.md#")
	if err != nil {
		t.Fatalf("SpecRefCompletions: %v", err)
	}
	accepted := collectHeadingAnchors(md)
	if len(got) != len(accepted) {
		t.Fatalf("got %d candidates, want %d (validation set): %v", len(got), len(accepted), got)
	}
	for _, cand := range got {
		if !strings.HasPrefix(cand, "specs/v0.2.0.md#") {
			t.Fatalf("candidate %q is not prefixed with the fixed path", cand)
		}
		anchor := strings.TrimPrefix(cand, "specs/v0.2.0.md#")
		if _, ok := accepted[anchor]; !ok {
			t.Fatalf("candidate anchor %q is not accepted by spec_ref validation", anchor)
		}
	}
}

// TestCompletionsWithNoVersionedSpecs guards the empty-specs edge: with no
// versioned spec present, both completion sources return nothing (and no error),
// so completion stays quiet rather than panicking or offering junk.
func TestCompletionsWithNoVersionedSpecs(t *testing.T) {
	repo := seedFixtureRepo(t) // seeds specs/v0.1.0.md + specs/README.md
	if err := os.Remove(filepath.Join(repo, "specs", "v0.1.0.md")); err != nil {
		t.Fatalf("remove seeded spec: %v", err)
	}

	svc := newTestService(t, repo, time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))
	versions, err := svc.SpecVersionCompletions()
	if err != nil {
		t.Fatalf("SpecVersionCompletions: %v", err)
	}
	if len(versions) != 0 {
		t.Fatalf("expected no version candidates, got %v", versions)
	}
	refs, err := svc.SpecRefCompletions("")
	if err != nil {
		t.Fatalf("SpecRefCompletions: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("expected no spec-ref candidates, got %v", refs)
	}
}

// TestSpecRefCompletionsAnchorPhaseNoAnchors guards the zero-anchor edge: a spec
// whose body carries no headings yields no anchor candidates for its fixed path.
func TestSpecRefCompletionsAnchorPhaseNoAnchors(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"), "just prose, no headings at all\n")

	svc := newTestService(t, repo, time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))
	got, err := svc.SpecRefCompletions("specs/v0.2.0.md#")
	if err != nil {
		t.Fatalf("SpecRefCompletions: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no anchor candidates for a heading-less spec, got %v", got)
	}
}

// TestSpecRefCompletionsUnknownPathYieldsNothing verifies a path that is not a
// discovered versioned spec yields no anchor candidates rather than erroring, so
// completion stays quiet for arbitrary paths.
func TestSpecRefCompletionsUnknownPathYieldsNothing(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))
	got, err := svc.SpecRefCompletions("docs/other.md#")
	if err != nil {
		t.Fatalf("SpecRefCompletions: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no candidates for unknown path, got %v", got)
	}
}
