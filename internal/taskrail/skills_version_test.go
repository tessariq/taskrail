package taskrail

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// skillMetaProbe reads the frontmatter fields an agent tool actually consumes, so
// a test can assert the marker leaves them untouched.
type skillMetaProbe struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// installedSkillsRepo returns a repo with the shippable skills materialized by the
// given writing version.
func installedSkillsRepo(t *testing.T, writingVersion string) (string, *Service) {
	t.Helper()
	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := svc.WriteShippableSkills(writingVersion, false); err != nil {
		t.Fatalf("write shippable skills: %v", err)
	}
	return repo, svc
}

// A materialized skill records the version that wrote it, as frontmatter an agent
// tool ignores. The skill's own name/description and body must survive untouched,
// otherwise the marker would change how the skill is interpreted.
func TestWriteShippableSkillsStampsWritingVersion(t *testing.T) {
	t.Parallel()

	repo, _ := installedSkillsRepo(t, "v9.9.9")

	for _, target := range []string{".agents/skills", ".claude/skills"} {
		name := shippableSkills[0]
		data, err := os.ReadFile(filepath.Join(repo, target, name, "SKILL.md"))
		if err != nil {
			t.Fatalf("read installed skill: %v", err)
		}
		got := string(data)
		if !strings.Contains(got, "metadata:\n    taskrail_version: v9.9.9") {
			t.Errorf("%s/%s carries no writing-version marker:\n%s", target, name, firstLines(got, 8))
		}
		if strings.Contains(got, "\ntaskrail_version:") {
			t.Errorf("%s/%s carries a legacy top-level marker:\n%s", target, name, firstLines(got, 8))
		}

		embedded, err := shippableSkillsFS.ReadFile(shippableSkillsRoot + "/" + name + "/SKILL.md")
		if err != nil {
			t.Fatalf("read embedded skill: %v", err)
		}
		wantMeta, wantBody, err := parseFrontmatter[skillMetaProbe](embedded)
		if err != nil {
			t.Fatalf("parse embedded skill: %v", err)
		}
		gotMeta, gotBody, err := parseFrontmatter[skillMetaProbe](data)
		if err != nil {
			t.Fatalf("parse installed skill: %v", err)
		}
		if gotMeta != wantMeta {
			t.Errorf("marker changed skill metadata: got %+v, want %+v", gotMeta, wantMeta)
		}
		if gotBody != wantBody {
			t.Errorf("marker changed skill body for %s/%s", target, name)
		}
	}
}

func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// A --force reinstall from a newer binary restamps the marker so the recorded
// version tracks the binary that last wrote the file.
func TestWriteShippableSkillsForceRestampsNewVersion(t *testing.T) {
	t.Parallel()

	repo, svc := installedSkillsRepo(t, "v1.0.0")

	res, err := svc.WriteShippableSkills("v2.0.0", true)
	if err != nil {
		t.Fatalf("force write: %v", err)
	}
	if len(res.Overwritten) == 0 {
		t.Fatal("force write across versions overwrote nothing")
	}

	data, err := os.ReadFile(filepath.Join(repo, ".claude", "skills", shippableSkills[0], "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if !strings.Contains(string(data), "taskrail_version: v2.0.0") {
		t.Errorf("skill kept the old marker:\n%s", firstLines(string(data), 8))
	}
}

// The round trip T-120 consumes: install, read back, and see the writing version
// for every installed skill in both target trees.
func TestInstalledSkillVersionsRoundTrip(t *testing.T) {
	t.Parallel()

	_, svc := installedSkillsRepo(t, "v0.4.0")

	installed, err := svc.InstalledSkillVersions()
	if err != nil {
		t.Fatalf("installed skill versions: %v", err)
	}
	if len(installed) != len(shippableSkills)*len(shippableSkillTargets) {
		t.Fatalf("got %d installed skills, want %d", len(installed), len(shippableSkills)*len(shippableSkillTargets))
	}
	for _, s := range installed {
		if s.Version != "v0.4.0" {
			t.Errorf("%s: version = %q, want v0.4.0", s.Path, s.Version)
		}
		if s.Skill == "" {
			t.Errorf("%s: empty skill name", s.Path)
		}
		if filepath.IsAbs(s.Path) {
			t.Errorf("path %q is absolute; want repo-relative", s.Path)
		}
		// Reported paths are slash-separated on every OS (relPath normalizes), so
		// callers and tests may key on them without OS-conditional joining. A
		// backslash here would only show up on Windows, and did: a test keying on
		// filepath.Join reds the Windows CI leg alone (T-124).
		if strings.ContainsRune(s.Path, '\\') {
			t.Errorf("path %q is not slash-separated; reported paths must be portable", s.Path)
		}
	}
}

// Adopter-owned skills can live beside or below Taskrail's installed skills, but
// they have no Taskrail writing version. Only paths represented by the embedded
// package belong in the inventory used for skew detection.
func TestInstalledSkillVersionsIgnoreAdopterOwnedSkills(t *testing.T) {
	t.Parallel()

	repo, svc := installedSkillsRepo(t, "v0.4.0")
	custom := []string{
		filepath.Join(".agents", "skills", "custom", "SKILL.md"),
		filepath.Join(".claude", "skills", "vendor", "custom", "SKILL.md"),
	}
	for _, rel := range custom {
		full := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("create custom skill directory: %v", err)
		}
		if err := os.WriteFile(full, []byte("---\nname: custom\ndescription: adopter owned\n---\n"), 0o644); err != nil {
			t.Fatalf("write custom skill: %v", err)
		}
	}

	installed, err := svc.InstalledSkillVersions()
	if err != nil {
		t.Fatalf("installed skill versions: %v", err)
	}
	if len(installed) != len(shippableSkills)*len(shippableSkillTargets) {
		t.Fatalf("got %d installed skills, want %d packaged skills", len(installed), len(shippableSkills)*len(shippableSkillTargets))
	}
	for _, skill := range installed {
		for _, rel := range custom {
			if skill.Path == filepath.ToSlash(rel) {
				t.Errorf("adopter-owned skill included in version inventory: %+v", skill)
			}
		}
	}
}

// A skill installed before the marker existed reports as unknown rather than
// failing the read.
func TestInstalledSkillVersionsReportsUnmarkedAsUnknown(t *testing.T) {
	t.Parallel()

	repo, svc := installedSkillsRepo(t, "v0.4.0")

	unmarked := filepath.Join(repo, ".claude", "skills", shippableSkills[0], "SKILL.md")
	if err := os.WriteFile(unmarked, []byte("---\nname: x\ndescription: y\n---\n\nbody\n"), 0o644); err != nil {
		t.Fatalf("write unmarked skill: %v", err)
	}

	installed, err := svc.InstalledSkillVersions()
	if err != nil {
		t.Fatalf("installed skill versions: %v", err)
	}
	var found bool
	for _, s := range installed {
		if filepath.ToSlash(s.Path) != filepath.ToSlash(relPath(repo, unmarked)) {
			continue
		}
		found = true
		if s.Version != "" {
			t.Errorf("unmarked skill reported version %q, want unknown", s.Version)
		}
	}
	if !found {
		t.Fatalf("unmarked skill missing from report")
	}
}

// The read reports whether each materialized skill is byte-identical to the copy
// the running binary embeds, which is what lets the skew check tell a marker-free
// parity copy apart from a genuinely unknown install (T-124). The comparison is
// against the in-binary package only — never against another install.
func TestInstalledSkillVersionsFlagsPackageParity(t *testing.T) {
	t.Parallel()

	repo, svc := installedSkillsRepo(t, "v0.4.0")

	// Stripping the marker restores the embedded bytes, which is exactly what
	// copying the package source (task skills:regen) produces.
	parity := shippableSkills[0]
	restampSkill(t, repo, ".claude/skills", parity, "")

	diverged := shippableSkills[1]
	divergeSkill(t, repo, ".claude/skills", diverged)

	installed, err := svc.InstalledSkillVersions()
	if err != nil {
		t.Fatalf("installed skill versions: %v", err)
	}

	// Keyed with path.Join, not filepath.Join: InstalledSkill.Path is
	// slash-separated on every OS, so an OS-dependent joiner would build keys that
	// match nothing on Windows.
	want := map[string]bool{
		path.Join(".claude", "skills", parity, "SKILL.md"):   true,
		path.Join(".claude", "skills", diverged, "SKILL.md"): false,
		// A stamped skill differs from the package by its own marker line.
		path.Join(".claude", "skills", shippableSkills[2], "SKILL.md"): false,
	}
	got := map[string]bool{}
	for _, s := range installed {
		got[s.Path] = s.MatchesPackage
	}
	for rel, expected := range want {
		matches, reported := got[rel]
		if !reported {
			t.Errorf("%s missing from the report", rel)
			continue
		}
		if matches != expected {
			t.Errorf("%s: MatchesPackage = %v, want %v", rel, matches, expected)
		}
	}
}

// A repository with no materialized skills reports nothing and does not error, so
// the read side stays silent instead of noisy.
func TestInstalledSkillVersionsWithoutSkillsIsEmpty(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	installed, err := svc.InstalledSkillVersions()
	if err != nil {
		t.Fatalf("installed skill versions: %v", err)
	}
	if len(installed) != 0 {
		t.Errorf("got %d installed skills in a repo with none: %+v", len(installed), installed)
	}
}

// A walk-level failure must report a repo-relative path like every other Taskrail
// filesystem error, so an advisory read never leaks the absolute repo root.
func TestInstalledSkillVersionsWalkErrorOmitsAbsolutePath(t *testing.T) {
	t.Parallel()

	repo, svc := installedSkillsRepo(t, "v0.4.0")

	// An unlistable skill directory fails inside WalkDir itself rather than in the
	// per-file read, which is the branch that previously propagated the raw error.
	dir := filepath.Join(repo, ".claude", "skills", shippableSkills[0])
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatalf("chmod skill dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if _, probeErr := os.ReadDir(dir); probeErr == nil {
		t.Skip("unreadable directory does not block listing here (root or native Windows); injection cannot fire")
	}

	_, err := svc.InstalledSkillVersions()
	if err == nil {
		t.Fatal("expected an error for an unreadable skill directory")
	}
	assertPortablePermissionError(t, repo, err)
}

// Reading is read-only: it never rewrites an adopter's skill files.
func TestInstalledSkillVersionsDoesNotWrite(t *testing.T) {
	t.Parallel()

	repo, svc := installedSkillsRepo(t, "v0.4.0")
	before := snapshotTree(t, repo)

	if _, err := svc.InstalledSkillVersions(); err != nil {
		t.Fatalf("installed skill versions: %v", err)
	}

	after := snapshotTree(t, repo)
	if len(before) != len(after) {
		t.Fatalf("read changed the file set: %d -> %d", len(before), len(after))
	}
	for path, want := range before {
		if after[path] != want {
			t.Errorf("read modified %s", path)
		}
	}
}
