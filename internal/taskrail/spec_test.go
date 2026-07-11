package taskrail

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSpecListMarksActiveAndIgnoresNonVersioned locks the discovery contract:
// SpecList enumerates only versioned specs (vN.N.N.md) in version order, marks
// the active one, and never lists specs/README.md or other non-conforming files.
func TestSpecListMarksActiveAndIgnoresNonVersioned(t *testing.T) {
	repo := seedFixtureRepo(t) // active v0.1.0, plus specs/README.md
	writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"), "# Taskrail v0.2.0\n\n## Summary\n\nNext.\n")
	writeFile(t, filepath.Join(repo, "specs", "notes.md"), "# scratch\n")

	svc := newTestService(t, repo, time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC))
	result, err := svc.SpecList()
	if err != nil {
		t.Fatalf("SpecList: %v", err)
	}

	if result.ActiveSpecVersion != "v0.1.0" {
		t.Fatalf("active spec version = %q, want v0.1.0", result.ActiveSpecVersion)
	}
	if len(result.Specs) != 2 {
		t.Fatalf("expected 2 versioned specs, got %d: %+v", len(result.Specs), result.Specs)
	}
	if result.Specs[0].Version != "v0.1.0" || result.Specs[1].Version != "v0.2.0" {
		t.Fatalf("specs not in version order: %+v", result.Specs)
	}
	if !result.Specs[0].Active || result.Specs[1].Active {
		t.Fatalf("active flag misassigned: %+v", result.Specs)
	}
	if result.Specs[0].Path != "specs/v0.1.0.md" {
		t.Fatalf("path = %q, want specs/v0.1.0.md", result.Specs[0].Path)
	}
}

// TestSpecShowReturnsContent covers the plain (non-anchor) mode: SpecShow returns
// the spec body, marks whether it is active, and leaves Anchors empty.
func TestSpecShowReturnsContent(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC))

	result, err := svc.SpecShow("v0.1.0", false)
	if err != nil {
		t.Fatalf("SpecShow: %v", err)
	}
	if !result.Active {
		t.Fatalf("expected v0.1.0 to be marked active: %+v", result)
	}
	if result.Path != "specs/v0.1.0.md" {
		t.Fatalf("path = %q", result.Path)
	}
	if len(result.Anchors) != 0 {
		t.Fatalf("expected no anchors in content mode, got %+v", result.Anchors)
	}
	if want := "## Summary"; !strings.Contains(result.Content, want) {
		t.Fatalf("content missing %q:\n%s", want, result.Content)
	}
}

// TestSpecShowAnchorsMatchValidation is the core invariant: the anchors SpecShow
// lists are exactly the set collectHeadingAnchors (spec_ref validation) accepts —
// same membership, deduped, no re-implementation drift.
func TestSpecShowAnchorsMatchValidation(t *testing.T) {
	repo := seedFixtureRepo(t)
	// A spec with duplicate-slugging headings and punctuation to stress dedup.
	md := "# Taskrail v0.2.0\n\n## Spec Inspection\n\n### Spec Inspection\n\n## Authoring & Commands\n"
	writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"), md)

	svc := newTestService(t, repo, time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC))
	result, err := svc.SpecShow("v0.2.0", true)
	if err != nil {
		t.Fatalf("SpecShow --anchors: %v", err)
	}
	if len(result.Content) != 0 {
		t.Fatalf("expected empty content in anchors mode, got %q", result.Content)
	}

	accepted := collectHeadingAnchors(md)
	if len(result.Anchors) != len(accepted) {
		t.Fatalf("anchor count %d != validation set size %d: %+v", len(result.Anchors), len(accepted), result.Anchors)
	}
	seen := make(map[string]struct{}, len(result.Anchors))
	for _, a := range result.Anchors {
		if _, ok := accepted[a.Anchor]; !ok {
			t.Fatalf("listed anchor %q is not accepted by spec_ref validation", a.Anchor)
		}
		if _, dup := seen[a.Anchor]; dup {
			t.Fatalf("anchor %q listed more than once", a.Anchor)
		}
		seen[a.Anchor] = struct{}{}
	}
}

// TestSpecShowRejectsNonConformingVersion mirrors activate: a malformed version
// is rejected before any read, even if a matching raw file exists.
func TestSpecShowRejectsNonConformingVersion(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeFile(t, filepath.Join(repo, "specs", "garbage.md"), "# nope\n")
	svc := newTestService(t, repo, time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC))
	if _, err := svc.SpecShow("garbage", false); err == nil {
		t.Fatal("expected error for non-conforming version")
	}
}

// TestSpecShowRejectsMissingVersion errors when the versioned file is absent.
func TestSpecShowRejectsMissingVersion(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC))
	if _, err := svc.SpecShow("v9.9.9", false); err == nil {
		t.Fatal("expected error for missing spec version")
	}
}

// TestSpecShowErrorOmitsAbsolutePath locks the portable-error contract: a
// missing-spec error names the repo-relative path but never leaks the user's
// absolute repository location (the raw *fs.PathError tail).
func TestSpecShowErrorOmitsAbsolutePath(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC))

	_, err := svc.SpecShow("v9.9.9", false)
	if err == nil {
		t.Fatal("expected error for missing spec version")
	}
	if strings.Contains(err.Error(), repo) {
		t.Fatalf("error leaks absolute repo path %q: %v", repo, err)
	}
	if !strings.Contains(err.Error(), "specs/v9.9.9.md") {
		t.Fatalf("error should name the repo-relative spec path: %v", err)
	}
}
