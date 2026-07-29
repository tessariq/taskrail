package taskrail

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// specDiffAnchorSet collapses a diff anchor list to a slug set for order-free
// membership assertions.
func specDiffAnchorSet(anchors []SpecAnchor) map[string]struct{} {
	set := make(map[string]struct{}, len(anchors))
	for _, a := range anchors {
		set[a.Anchor] = struct{}{}
	}
	return set
}

// seedTwoSpecs writes v1 and v2 under specs/ and returns the service. The
// fixture repo already carries an active v0.1.0 spec; both versions here are
// distinct from it so the diff is exercised independent of the active spec.
func seedTwoSpecs(t *testing.T, v1md, v2md string) *Service {
	t.Helper()
	repo := seedFixtureRepo(t)
	writeFile(t, filepath.Join(repo, "specs", "v0.2.0.md"), v1md)
	writeFile(t, filepath.Join(repo, "specs", "v0.3.0.md"), v2md)
	return newTestService(t, repo, time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC))
}

// TestSpecDiffAddedOnly: an anchor present only in v2 is reported as added, with
// no removals or rename candidates.
func TestSpecDiffAddedOnly(t *testing.T) {
	svc := seedTwoSpecs(t,
		"# Taskrail\n\n## Alpha Area\n",
		"# Taskrail\n\n## Alpha Area\n\n## Beta Area\n",
	)
	result, err := svc.SpecDiff("v0.2.0", "v0.3.0")
	if err != nil {
		t.Fatalf("SpecDiff: %v", err)
	}
	if _, ok := specDiffAnchorSet(result.Added)["beta-area"]; !ok {
		t.Fatalf("expected beta-area added, got %+v", result.Added)
	}
	if len(result.Added) != 1 {
		t.Fatalf("expected exactly one added anchor, got %+v", result.Added)
	}
	if len(result.Removed) != 0 || len(result.Renamed) != 0 {
		t.Fatalf("expected no removals/renames, got removed=%+v renamed=%+v", result.Removed, result.Renamed)
	}
}

// TestSpecDiffRemovedOnly: an anchor present only in v1 is reported as removed.
func TestSpecDiffRemovedOnly(t *testing.T) {
	svc := seedTwoSpecs(t,
		"# Taskrail\n\n## Alpha Area\n\n## Gamma Area\n",
		"# Taskrail\n\n## Alpha Area\n",
	)
	result, err := svc.SpecDiff("v0.2.0", "v0.3.0")
	if err != nil {
		t.Fatalf("SpecDiff: %v", err)
	}
	if _, ok := specDiffAnchorSet(result.Removed)["gamma-area"]; !ok {
		t.Fatalf("expected gamma-area removed, got %+v", result.Removed)
	}
	if len(result.Removed) != 1 {
		t.Fatalf("expected exactly one removed anchor, got %+v", result.Removed)
	}
	if len(result.Added) != 0 || len(result.Renamed) != 0 {
		t.Fatalf("expected no adds/renames, got added=%+v renamed=%+v", result.Added, result.Renamed)
	}
}

// TestSpecDiffMixed: simultaneous add and remove of unrelated areas, neither a
// rename candidate (no shared stem).
func TestSpecDiffMixed(t *testing.T) {
	svc := seedTwoSpecs(t,
		"# Taskrail\n\n## Alpha Area\n\n## Gamma Area\n",
		"# Taskrail\n\n## Alpha Area\n\n## Delta Area\n",
	)
	result, err := svc.SpecDiff("v0.2.0", "v0.3.0")
	if err != nil {
		t.Fatalf("SpecDiff: %v", err)
	}
	if _, ok := specDiffAnchorSet(result.Added)["delta-area"]; !ok {
		t.Fatalf("expected delta-area added, got %+v", result.Added)
	}
	if _, ok := specDiffAnchorSet(result.Removed)["gamma-area"]; !ok {
		t.Fatalf("expected gamma-area removed, got %+v", result.Removed)
	}
	if len(result.Renamed) != 0 {
		t.Fatalf("unrelated add+remove must not be a rename candidate, got %+v", result.Renamed)
	}
}

// TestSpecDiffIdentical: identical specs produce an empty delta.
func TestSpecDiffIdentical(t *testing.T) {
	md := "# spec\n\n## Alpha Area\n\n## Beta Area\n"
	svc := seedTwoSpecs(t, md, md)
	result, err := svc.SpecDiff("v0.2.0", "v0.3.0")
	if err != nil {
		t.Fatalf("SpecDiff: %v", err)
	}
	if len(result.Added) != 0 || len(result.Removed) != 0 || len(result.Renamed) != 0 {
		t.Fatalf("identical specs must yield empty delta, got %+v", result)
	}
}

// TestSpecDiffSameVersion: diffing a version against itself yields an empty delta
// (self-comparison is a degenerate but valid input).
func TestSpecDiffSameVersion(t *testing.T) {
	svc := seedTwoSpecs(t, "# Taskrail\n\n## Alpha Area\n", "# Taskrail\n\n## Beta Area\n")
	result, err := svc.SpecDiff("v0.2.0", "v0.2.0")
	if err != nil {
		t.Fatalf("SpecDiff: %v", err)
	}
	if len(result.Added) != 0 || len(result.Removed) != 0 || len(result.Renamed) != 0 {
		t.Fatalf("self-diff must be empty, got %+v", result)
	}
}

// TestSpecDiffRenameCandidate: an added and a removed anchor sharing a normalized
// stem (same leading tokens, differing final token) surface as a supplemental
// candidate without being removed from the definitive delta.
func TestSpecDiffRenameCandidate(t *testing.T) {
	svc := seedTwoSpecs(t,
		"# Taskrail\n\n## Spec Coverage Report\n\n## Old Area\n",
		"# Taskrail\n\n## Spec Coverage Summary\n\n## Fresh Area\n",
	)
	result, err := svc.SpecDiff("v0.2.0", "v0.3.0")
	if err != nil {
		t.Fatalf("SpecDiff: %v", err)
	}
	if len(result.Renamed) != 1 {
		t.Fatalf("expected one rename candidate, got %+v", result.Renamed)
	}
	rc := result.Renamed[0]
	if rc.From.Anchor != "spec-coverage-report" || rc.To.Anchor != "spec-coverage-summary" {
		t.Fatalf("rename candidate endpoints wrong: %+v", rc)
	}
	if len(result.Added) != 2 || result.Added[0].Anchor != "spec-coverage-summary" || result.Added[1].Anchor != "fresh-area" {
		t.Fatalf("added must retain document order and candidate target, got %+v", result.Added)
	}
	if len(result.Removed) != 2 || result.Removed[0].Anchor != "spec-coverage-report" || result.Removed[1].Anchor != "old-area" {
		t.Fatalf("removed must retain document order and candidate source, got %+v", result.Removed)
	}
}

// TestSpecDiffRejectsUnknownVersion: an unknown version fails before any work,
// resolved the same way as the rest of the spec family.
func TestSpecDiffRejectsUnknownVersion(t *testing.T) {
	svc := seedTwoSpecs(t, "# a\n\n## Alpha\n", "# b\n\n## Beta\n")
	if _, err := svc.SpecDiff("v0.2.0", "v9.9.9"); err == nil {
		t.Fatal("expected error for unknown v2")
	}
	if _, err := svc.SpecDiff("garbage", "v0.3.0"); err == nil {
		t.Fatal("expected error for non-conforming v1")
	}
}

// TestSpecDiffEmptyListsMarshalAsArrays guards the --json automation contract: an
// empty delta must serialize added/removed/renamed as `[]`, never `null`, so a
// consumer iterating the lists never trips over a null. Mirrors the package
// convention (coverage orphans, etc.).
func TestSpecDiffEmptyListsMarshalAsArrays(t *testing.T) {
	md := "# Taskrail\n\n## Alpha Area\n"
	svc := seedTwoSpecs(t, md, md)
	result, err := svc.SpecDiff("v0.2.0", "v0.3.0")
	if err != nil {
		t.Fatalf("SpecDiff: %v", err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, field := range []string{`"added":[]`, `"removed":[]`, `"renamed":[]`} {
		if !strings.Contains(string(data), field) {
			t.Fatalf("expected %s in JSON, got %s", field, data)
		}
	}
	if strings.Contains(string(data), "null") {
		t.Fatalf("empty diff must not serialize null lists: %s", data)
	}
}

// TestSpecDiffAnchorsMatchValidation locks the shared-slug invariant: the diff
// operates over exactly the anchors spec_ref validation accepts.
func TestSpecDiffAnchorsMatchValidation(t *testing.T) {
	v2md := "# Taskrail\n\n## Alpha Area\n\n### Alpha Area\n\n## New & Fancy\n"
	svc := seedTwoSpecs(t, "# Taskrail\n\n## Alpha Area\n", v2md)
	result, err := svc.SpecDiff("v0.2.0", "v0.3.0")
	if err != nil {
		t.Fatalf("SpecDiff: %v", err)
	}
	accepted := collectHeadingAnchors(v2md)
	for _, a := range result.Added {
		if _, ok := accepted[a.Anchor]; !ok {
			t.Fatalf("added anchor %q not accepted by spec_ref validation", a.Anchor)
		}
	}
}
