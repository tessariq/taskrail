package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// restampSkill rewrites one installed skill's marker so a test can stage skew
// without reinstalling the whole package. An empty version leaves the skill
// unmarked, the way an install from before the marker existed reads.
func restampSkill(t *testing.T, repo, target, skill, version string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(target), skill, "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	var unmarked []string
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, skillVersionKey+":") {
			unmarked = append(unmarked, line)
		}
	}
	restamped := []byte(strings.Join(unmarked, "\n"))
	if version != "" {
		restamped = stampSkillVersion(restamped, version)
	}
	if err := os.WriteFile(path, restamped, 0o644); err != nil {
		t.Fatalf("restamp skill: %v", err)
	}
}

// divergeSkill leaves a skill unmarked *and* no longer byte-identical to the
// embedded package — a genuinely unknown install, as opposed to the marker-free
// parity copy the exemption covers.
func divergeSkill(t *testing.T, repo, target, skill string) {
	t.Helper()
	restampSkill(t, repo, target, skill, "")
	path := filepath.Join(repo, filepath.FromSlash(target), skill, "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if err := os.WriteFile(path, append(data, []byte("\nlocal edit\n")...), 0o644); err != nil {
		t.Fatalf("diverge skill: %v", err)
	}
}

// Skills written by the running binary are not skew, so the check stays silent.
func TestSkillSkewWarningsSilentWhenVersionsMatch(t *testing.T) {
	t.Parallel()

	_, svc := installedSkillsRepo(t, "v0.4.0")

	warnings, err := svc.SkillSkewWarnings("v0.4.0")
	if err != nil {
		t.Fatalf("skill skew warnings: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("matching versions warned: %+v", warnings)
	}
}

// A repository with no materialized skills has nothing to say.
func TestSkillSkewWarningsSilentWithoutInstalledSkills(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(false); err != nil {
		t.Fatalf("init: %v", err)
	}

	warnings, err := svc.SkillSkewWarnings("v0.4.0")
	if err != nil {
		t.Fatalf("skill skew warnings: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("repo without skills warned: %+v", warnings)
	}
}

// Both skew directions report identically: one warning naming both versions, the
// affected skills, and the resolving command. The direction is readable from the
// two versions rather than from separate wording.
func TestSkillSkewWarningsReportsBothDirections(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		installed string
		running   string
	}{
		{"older skills than binary", "v0.3.0", "v0.4.0"},
		{"newer skills than binary", "v0.5.0", "v0.4.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, svc := installedSkillsRepo(t, tc.installed)

			warnings, err := svc.SkillSkewWarnings(tc.running)
			if err != nil {
				t.Fatalf("skill skew warnings: %v", err)
			}
			if len(warnings) != 1 {
				t.Fatalf("got %d warnings, want 1: %+v", len(warnings), warnings)
			}
			got := warnings[0]
			if got.Code != "skill_version_skew" {
				t.Errorf("code = %q, want skill_version_skew", got.Code)
			}
			for _, want := range []string{
				"warning:",
				tc.installed,
				tc.running,
				"taskrail init --with-skills --force",
				shippableSkills[0],
			} {
				if !strings.Contains(got.Message, want) {
					t.Errorf("message %q missing %q", got.Message, want)
				}
			}
		})
	}
}

// A marker-free copy that is byte-identical to the embedded package is provably
// not a stale install: nothing was recorded because nothing installed it — it was
// copied from the package the running binary carries. Reporting it as unknown is a
// standing line no contributor can clear, so the check exempts it
// (specs/v0.4.0.md#version-skew-detection, T-124). The running version is
// irrelevant to that judgement: parity is with the binary's own copy.
func TestSkillSkewWarningsSilentForUnmarkedParityCopies(t *testing.T) {
	t.Parallel()

	repo, svc := installedSkillsRepo(t, "v0.4.0")
	for _, target := range shippableSkillTargets {
		for _, skill := range shippableSkills {
			restampSkill(t, repo, target, skill, "")
		}
	}

	for _, running := range []string{"v0.4.0", "v9.9.9"} {
		warnings, err := svc.SkillSkewWarnings(running)
		if err != nil {
			t.Fatalf("skill skew warnings: %v", err)
		}
		if len(warnings) != 0 {
			t.Errorf("running %s: parity copies warned: %+v", running, warnings)
		}
	}
}

// Skills installed before the marker existed are unknown, not stale: they report
// under their own code and must not claim a version they never recorded. Content
// that diverges from the embedded package is what separates this from the parity
// copy above — the exemption must not swallow a genuinely unknown install.
func TestSkillSkewWarningsReportsUnmarkedSkillsOnceAsUnknown(t *testing.T) {
	t.Parallel()

	repo, svc := installedSkillsRepo(t, "v0.4.0")
	for _, target := range shippableSkillTargets {
		for _, skill := range shippableSkills {
			divergeSkill(t, repo, target, skill)
		}
	}

	warnings, err := svc.SkillSkewWarnings("v0.4.0")
	if err != nil {
		t.Fatalf("skill skew warnings: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want 1: %+v", len(warnings), warnings)
	}
	got := warnings[0]
	if got.Code != "unknown_skill_version" {
		t.Errorf("code = %q, want unknown_skill_version", got.Code)
	}
	if !strings.Contains(got.Message, "no version marker") {
		t.Errorf("message %q does not report the marker as absent", got.Message)
	}
	if !strings.Contains(got.Message, "v0.4.0") {
		t.Errorf("message %q omits the running version", got.Message)
	}
	// An unmarked skill may be current, older, or newer — nothing was recorded to
	// compare. Prescribing a --force reinstall would be the false upgrade prompt the
	// unknown case exists to avoid, and in a repository whose committed skills are
	// deliberately unmarked parity copies of the package
	// (docs/workflow/skills-productization.md) following it breaks `task check:skills`.
	if strings.Contains(got.Message, skillSkewRemedy) {
		t.Errorf("unknown-version message prescribes %q: %s", skillSkewRemedy, got.Message)
	}
}

// Each skill is materialized into both target trees, but an adopter fixes both
// with one command, so the report names a skewed skill once rather than per tree.
func TestSkillSkewWarningsNamesEachSkillOnce(t *testing.T) {
	t.Parallel()

	_, svc := installedSkillsRepo(t, "v0.3.0")

	warnings, err := svc.SkillSkewWarnings("v0.4.0")
	if err != nil {
		t.Fatalf("skill skew warnings: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want 1: %+v", len(warnings), warnings)
	}
	if n := strings.Count(warnings[0].Message, shippableSkills[0]); n != 1 {
		t.Errorf("skill %s named %d times, want 1: %s", shippableSkills[0], n, warnings[0].Message)
	}
}

// Mixed markers group by recorded version in a stable order, so repeated runs
// produce byte-identical output.
func TestSkillSkewWarningsGroupsByRecordedVersionDeterministically(t *testing.T) {
	t.Parallel()

	repo, svc := installedSkillsRepo(t, "v0.4.0")
	for _, target := range shippableSkillTargets {
		restampSkill(t, repo, target, shippableSkills[0], "v0.1.0")
		restampSkill(t, repo, target, shippableSkills[1], "v0.2.0")
		divergeSkill(t, repo, target, shippableSkills[2])
	}

	var first []string
	for i := 0; i < 3; i++ {
		warnings, err := svc.SkillSkewWarnings("v0.4.0")
		if err != nil {
			t.Fatalf("skill skew warnings: %v", err)
		}
		messages := make([]string, 0, len(warnings))
		for _, w := range warnings {
			messages = append(messages, w.Message)
		}
		if i == 0 {
			if len(messages) != 3 {
				t.Fatalf("got %d warnings, want 3 (v0.1.0, v0.2.0, unknown): %v", len(messages), messages)
			}
			first = messages
			continue
		}
		if strings.Join(messages, "\n") != strings.Join(first, "\n") {
			t.Fatalf("unstable order:\n%v\nvs\n%v", first, messages)
		}
	}
}

// Detection is read-only: the warning never rewrites an adopter's skill files.
func TestSkillSkewWarningsDoNotWrite(t *testing.T) {
	t.Parallel()

	repo, svc := installedSkillsRepo(t, "v0.3.0")
	before := snapshotTree(t, repo)

	if _, err := svc.SkillSkewWarnings("v0.4.0"); err != nil {
		t.Fatalf("skill skew warnings: %v", err)
	}

	after := snapshotTree(t, repo)
	if len(before) != len(after) {
		t.Fatalf("check changed the file set: %d -> %d", len(before), len(after))
	}
	for path, want := range before {
		if after[path] != want {
			t.Errorf("check modified %s", path)
		}
	}
}
