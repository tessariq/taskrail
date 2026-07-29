package main

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSpecFile drops a versioned spec under specs/ in the current repo.
func writeSpecFile(t *testing.T, root, version, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "specs", version+".md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write spec %s: %v", version, err)
	}
}

// TestSpecDiffHumanOutput exercises the human path end to end and asserts the
// command leaves the entire repository byte-identical.
func TestSpecDiffHumanOutput(t *testing.T) {
	root := setupRepo(t)
	writeSpecFile(t, root, "v0.2.0", "# Taskrail\n\n## Alpha Area\n\n## Spec Coverage Report\n\n## Old Area\n")
	writeSpecFile(t, root, "v0.3.0", "# Taskrail\n\n## Alpha Area\n\n## Spec Coverage Summary\n\n## Brand New Area\n")

	before := snapshotTree(t, root)

	out, err := runRoot(t, "spec", "diff", "v0.2.0", "v0.3.0")
	if err != nil {
		t.Fatalf("spec diff: %v (output %q)", err, out)
	}
	addedCandidate := strings.Index(out, "#spec-coverage-summary Spec Coverage Summary")
	addedUnrelated := strings.Index(out, "#brand-new-area Brand New Area")
	if addedCandidate < 0 || addedUnrelated < addedCandidate {
		t.Fatalf("expected added anchors in document order, got %q", out)
	}
	removedCandidate := strings.Index(out, "#spec-coverage-report Spec Coverage Report")
	removedUnrelated := strings.Index(out, "#old-area Old Area")
	if removedCandidate < 0 || removedUnrelated < removedCandidate {
		t.Fatalf("expected removed anchors in document order, got %q", out)
	}

	if after := snapshotTree(t, root); !maps.Equal(before, after) {
		t.Fatal("spec diff must leave the repository byte-identical")
	}
}

// TestSpecDiffJSONOutput mirrors the human path over --json with structured
// added/removed/renamed lists.
func TestSpecDiffJSONOutput(t *testing.T) {
	root := setupRepo(t)
	writeSpecFile(t, root, "v0.2.0", "# Taskrail\n\n## Alpha Area\n\n## Spec Coverage Report\n")
	writeSpecFile(t, root, "v0.3.0", "# Taskrail\n\n## Alpha Area\n\n## Spec Coverage Summary\n\n## Fresh Area\n")

	out, err := runRoot(t, "spec", "diff", "v0.2.0", "v0.3.0", "--json")
	if err != nil {
		t.Fatalf("spec diff --json: %v (output %q)", err, out)
	}
	var payload struct {
		FromVersion string `json:"from_version"`
		ToVersion   string `json:"to_version"`
		Added       []struct {
			Anchor string `json:"anchor"`
		} `json:"added"`
		Removed []struct {
			Anchor string `json:"anchor"`
		} `json:"removed"`
		Renamed []struct {
			From struct {
				Anchor string `json:"anchor"`
			} `json:"from"`
			To struct {
				Anchor string `json:"anchor"`
			} `json:"to"`
		} `json:"renamed"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode diff json: %v (output %q)", err, out)
	}
	if payload.FromVersion != "v0.2.0" || payload.ToVersion != "v0.3.0" {
		t.Fatalf("version fields wrong: %+v", payload)
	}
	if len(payload.Added) != 2 || payload.Added[0].Anchor != "spec-coverage-summary" || payload.Added[1].Anchor != "fresh-area" {
		t.Fatalf("expected candidate target and fresh-area added in document order, got %+v", payload.Added)
	}
	if len(payload.Renamed) != 1 || payload.Renamed[0].From.Anchor != "spec-coverage-report" || payload.Renamed[0].To.Anchor != "spec-coverage-summary" {
		t.Fatalf("expected the coverage rename candidate, got %+v", payload.Renamed)
	}
	if len(payload.Removed) != 1 || payload.Removed[0].Anchor != "spec-coverage-report" {
		t.Fatalf("expected candidate source retained as removed, got %+v", payload.Removed)
	}
}

// TestSpecDiffCompletesBothVersionArgs verifies both positional slots complete to
// versioned specs — unlike single-positional `spec show`/`activate`, `spec diff`
// takes two versions, so completion must fire after the first argument too.
func TestSpecDiffCompletesBothVersionArgs(t *testing.T) {
	root := setupRepo(t)
	writeSpecFile(t, root, "v0.2.0", "# Taskrail\n\n## Summary\n")

	// Second positional (a from-version already supplied): must still suggest.
	out, err := runRoot(t, "__complete", "spec", "diff", "v0.1.0", "")
	if err != nil {
		t.Fatalf("__complete spec diff <from> \"\": %v (output %q)", err, out)
	}
	if !strings.Contains(out, "v0.1.0") || !strings.Contains(out, "v0.2.0") {
		t.Fatalf("second version slot omits a version: %q", out)
	}
	if !strings.Contains(out, "ShellCompDirectiveNoFileComp") {
		t.Fatalf("diff completion should suppress file completion: %q", out)
	}
}

// TestSpecDiffRejectsUnknownVersion confirms an unknown version fails through the
// CLI, resolved the same way as the rest of the spec family.
func TestSpecDiffRejectsUnknownVersion(t *testing.T) {
	setupRepo(t)
	if _, err := runRoot(t, "spec", "diff", "v0.1.0", "v9.9.9"); err == nil {
		t.Fatal("expected error for unknown to-version")
	}
}
