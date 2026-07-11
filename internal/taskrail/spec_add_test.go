package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestAddSpecScaffoldsFileAndReadme covers the happy path: the standard section
// skeleton is written under the versioned name and the version is added to the
// specs/README.md reading order, while STATE.md is left byte-for-byte unchanged
// (add never activates).
func TestAddSpecScaffoldsFileAndReadme(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))

	statePath := filepath.Join(repo, "planning", "STATE.md")
	stateBefore := readFileString(t, statePath)

	result, err := svc.AddSpec("v0.4.0")
	if err != nil {
		t.Fatalf("AddSpec: %v", err)
	}
	if result.Version != "v0.4.0" || result.SpecPath != "specs/v0.4.0.md" {
		t.Fatalf("unexpected result: %+v", result)
	}

	body := readFileString(t, filepath.Join(repo, "specs", "v0.4.0.md"))
	if !strings.Contains(body, "# Taskrail v0.4.0") {
		t.Fatalf("scaffold missing title heading:\n%s", body)
	}
	for _, section := range []string{
		"## Summary", "## Goals", "## Potential Features", "## Caution",
		"## Recommendation About LLM Support", "## Explicitly Excluded",
	} {
		if !strings.Contains(body, section) {
			t.Fatalf("scaffold missing section %q:\n%s", section, body)
		}
	}

	readme := readFileString(t, filepath.Join(repo, "specs", "README.md"))
	if !strings.Contains(readme, "`specs/v0.4.0.md`") {
		t.Fatalf("README reading order not updated:\n%s", readme)
	}

	if got := readFileString(t, statePath); got != stateBefore {
		t.Fatal("AddSpec must not write STATE.md")
	}
}

// TestAddSpecScaffoldHasNoCoverableAreas locks the T-059 alignment: a fresh
// scaffold carries no `###` feature areas, so it resolves to zero coverable
// areas and coverage reports N/A rather than inventing false gaps or a hollow 0%.
func TestAddSpecScaffoldHasNoCoverableAreas(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))

	if _, err := svc.AddSpec("v0.4.0"); err != nil {
		t.Fatalf("AddSpec: %v", err)
	}
	body := readFileString(t, filepath.Join(repo, "specs", "v0.4.0.md"))
	if areas := parseCoverableAreas(body); len(areas) != 0 {
		t.Fatalf("scaffold must have zero coverable areas, got %d: %+v", len(areas), areas)
	}
}

// TestAddSpecInsertsInVersionOrder verifies a middle version is spliced into the
// existing numbered reading order in version order and the list is renumbered,
// leaving surrounding sections intact.
func TestAddSpecInsertsInVersionOrder(t *testing.T) {
	repo := seedFixtureRepo(t)
	readmePath := filepath.Join(repo, "specs", "README.md")
	writeFile(t, readmePath, "# Specs\n\n## Reading Order\n\n1. `specs/v0.1.0.md`\n2. `specs/v0.3.0.md`\n\n## Rules\n\n- Specs are normative.\n")
	writeFile(t, filepath.Join(repo, "specs", "v0.3.0.md"), "# Taskrail v0.3.0\n\n## Summary\n\nThird.\n")
	svc := newTestService(t, repo, time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))

	if _, err := svc.AddSpec("v0.2.0"); err != nil {
		t.Fatalf("AddSpec: %v", err)
	}

	readme := readFileString(t, readmePath)
	want := "1. `specs/v0.1.0.md`\n2. `specs/v0.2.0.md`\n3. `specs/v0.3.0.md`"
	if !strings.Contains(readme, want) {
		t.Fatalf("reading order not renumbered in version order:\n%s", readme)
	}
	if !strings.Contains(readme, "## Rules") || !strings.Contains(readme, "- Specs are normative.") {
		t.Fatalf("surrounding sections not preserved:\n%s", readme)
	}
}

// TestAddSpecRejectsExistingVersion locks the no-clobber, no-write contract: an
// existing version is rejected and neither the spec file nor the README changes.
func TestAddSpecRejectsExistingVersion(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))

	specPath := filepath.Join(repo, "specs", "v0.1.0.md")
	readmePath := filepath.Join(repo, "specs", "README.md")
	specBefore := readFileString(t, specPath)
	readmeBefore := readFileString(t, readmePath)

	if _, err := svc.AddSpec("v0.1.0"); err == nil {
		t.Fatal("expected error adding an existing spec version")
	}
	if readFileString(t, specPath) != specBefore {
		t.Fatal("rejected add must not overwrite the existing spec")
	}
	if readFileString(t, readmePath) != readmeBefore {
		t.Fatal("rejected add must not write README")
	}
}

// TestAddSpecRejectsNonConformingVersion rejects a version that violates the
// versioned-specs convention, writing nothing.
func TestAddSpecRejectsNonConformingVersion(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))
	readmePath := filepath.Join(repo, "specs", "README.md")
	readmeBefore := readFileString(t, readmePath)

	if _, err := svc.AddSpec("garbage"); err == nil {
		t.Fatal("expected error for a non-conforming version name")
	}
	if fileExists(filepath.Join(repo, "specs", "garbage.md")) {
		t.Fatal("rejected add must not create a spec file")
	}
	if readFileString(t, readmePath) != readmeBefore {
		t.Fatal("rejected add must not write README")
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
