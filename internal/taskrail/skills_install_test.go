package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// hasSkillTree reports whether any file exists under the given agent-tool skill
// directory in the repo snapshot.
func hasSkillTree(tree map[string]string, dir string) bool {
	prefix := filepath.ToSlash(dir) + "/"
	for rel := range tree {
		if strings.HasPrefix(filepath.ToSlash(rel), prefix) {
			return true
		}
	}
	return false
}

// TestInstallSkillFileReadErrorOmitsAbsolutePath locks the portable-error
// contract on the non-ErrNotExist read branch: reading a dest that is a directory
// (EISDIR, not ErrNotExist) must not leak the absolute repo path.
func TestInstallSkillFileReadErrorOmitsAbsolutePath(t *testing.T) {
	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	// A directory at dest makes os.ReadFile fail with EISDIR, hitting the default
	// (non-ErrNotExist) branch.
	dest := filepath.Join(repo, ".claude", "skills", "probe")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}

	var res SkillInstallResult
	err := svc.installSkillFile(dest, []byte("x"), false, &res)
	if err == nil {
		t.Fatal("expected a read error for a directory dest")
	}
	if strings.Contains(err.Error(), repo) {
		t.Fatalf("error leaks absolute repo path %q: %v", repo, err)
	}
}

// TestBackupPathStatErrorOmitsAbsolutePath locks the portable-error contract on
// backupPath's non-ErrNotExist stat branch, the last error site in the package
// that named a file absolutely (T-137).
func TestBackupPathStatErrorOmitsAbsolutePath(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	// A NUL byte in the name makes os.Stat fail with EINVAL on every supported
	// GOOS, hitting the branch that ErrNotExist skips. The obvious alternative —
	// statting through a regular file, for ENOTDIR — is not portable: Windows
	// aliases ENOTDIR to ERROR_PATH_NOT_FOUND, which errors.Is reports as
	// ErrNotExist, so backupPath would return no error at all there.
	_, err := svc.backupPath(filepath.Join(repo, ".claude", "skills", "pro\x00be.md"))
	if err == nil {
		t.Fatal("expected a stat error for an unstattable backup candidate")
	}
	const wantPath = ".claude/skills/pro\x00be.md.bak.20260331T120000Z"
	if !strings.Contains(err.Error(), wantPath) {
		t.Fatalf("error = %q, want it to name %q", err.Error(), wantPath)
	}
	// An absolute path ends in wantPath too, so the check above passes either
	// way; the repo root's absence is what pins the form.
	if strings.Contains(err.Error(), repo) {
		t.Fatalf("error = %q, want no absolute repo path %q", err.Error(), repo)
	}
}

// Default init must never provision agent-tool skill directories; writing them
// is opt-in via --with-skills (skills-productization.md Decision 2).
func TestInitDefaultWritesNoSkillDirs(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	tree := snapshotTree(t, repo)
	for _, dir := range []string{".agents/skills", ".claude/skills"} {
		if hasSkillTree(tree, dir) {
			t.Errorf("default init wrote skill directory %s; must be opt-in", dir)
		}
	}
}

func TestWriteShippableSkillsInstallsToTargets(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	res, err := svc.WriteShippableSkills("v0.0.0-test", false)
	if err != nil {
		t.Fatalf("write shippable skills: %v", err)
	}
	if len(res.Written) == 0 {
		t.Fatal("write shippable skills reported no files written")
	}

	for _, target := range []string{".agents/skills", ".claude/skills"} {
		for _, name := range shippableSkills {
			path := filepath.Join(repo, target, name, "SKILL.md")
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("expected installed skill %s/%s: %v", target, name, readErr)
			}
			if strings.TrimSpace(string(data)) == "" {
				t.Errorf("installed skill %s/%s is empty", target, name)
			}
			if strings.Contains(string(data), "go run") {
				t.Errorf("installed skill %s/%s references 'go run'", target, name)
			}
		}
	}

	// Dogfooding-only skills must never be installed.
	for _, target := range []string{".agents/skills", ".claude/skills"} {
		for _, name := range dogfoodingOnlySkills {
			if _, statErr := os.Stat(filepath.Join(repo, target, name, "SKILL.md")); statErr == nil {
				t.Errorf("dogfooding-only skill %s must not be installed under %s", name, target)
			}
		}
	}
}

// A re-run is non-destructive: it never clobbers a user-edited skill and reports
// nothing newly written (writeFileIfMissing semantics, consistent with T-019).
func TestWriteShippableSkillsIdempotent(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := svc.WriteShippableSkills("v0.0.0-test", false); err != nil {
		t.Fatalf("first write: %v", err)
	}

	edited := filepath.Join(repo, ".claude", "skills", shippableSkills[0], "SKILL.md")
	const userMark = "USER EDIT — do not clobber"
	if err := os.WriteFile(edited, []byte(userMark), 0o644); err != nil {
		t.Fatalf("edit skill: %v", err)
	}

	res, err := svc.WriteShippableSkills("v0.0.0-test", false)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if len(res.Written) != 0 || len(res.Overwritten) != 0 || len(res.BackedUp) != 0 {
		t.Errorf("re-run changed files: %+v; want no changes", res)
	}

	data, err := os.ReadFile(edited)
	if err != nil {
		t.Fatalf("read edited skill: %v", err)
	}
	if string(data) != userMark {
		t.Errorf("re-run clobbered user-edited skill; content = %q", string(data))
	}
}

// backupsFor returns the timestamped backup files sitting next to a skill file.
func backupsFor(t *testing.T, skillPath string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(skillPath))
	if err != nil {
		t.Fatalf("read skill dir: %v", err)
	}
	base := filepath.Base(skillPath)
	var backups []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), base+".bak.") {
			backups = append(backups, filepath.Join(filepath.Dir(skillPath), e.Name()))
		}
	}
	return backups
}

// --force reinstalls the embedded skill over a locally-modified copy, backing up
// the user's version first and reporting both the overwrite and the backup.
func TestWriteShippableSkillsForceOverwritesWithBackup(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := svc.WriteShippableSkills("v0.0.0-test", false); err != nil {
		t.Fatalf("first write: %v", err)
	}

	skill := filepath.Join(repo, ".claude", "skills", shippableSkills[0], "SKILL.md")
	embedded, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	const userMark = "USER EDIT — recover me"
	if err := os.WriteFile(skill, []byte(userMark), 0o644); err != nil {
		t.Fatalf("edit skill: %v", err)
	}

	res, err := svc.WriteShippableSkills("v0.0.0-test", true)
	if err != nil {
		t.Fatalf("force write: %v", err)
	}
	if len(res.Overwritten) == 0 {
		t.Fatal("force write reported nothing overwritten")
	}
	if len(res.BackedUp) == 0 {
		t.Fatal("force write reported nothing backed up")
	}

	// Embedded content is restored over the user edit.
	got, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("read skill after force: %v", err)
	}
	if string(got) != string(embedded) {
		t.Errorf("force did not restore embedded content; got %q", string(got))
	}

	// The user's edit is recoverable from exactly one timestamped backup.
	backups := backupsFor(t, skill)
	if len(backups) != 1 {
		t.Fatalf("want 1 backup, got %d: %v", len(backups), backups)
	}
	bak, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(bak) != userMark {
		t.Errorf("backup did not preserve user edit; got %q", string(bak))
	}
}

// Two successive --force runs must each produce a distinct backup so the first is
// never clobbered, even when the clock reports the same timestamp for both.
func TestWriteShippableSkillsForceKeepsDistinctBackups(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := svc.WriteShippableSkills("v0.0.0-test", false); err != nil {
		t.Fatalf("first write: %v", err)
	}

	skill := filepath.Join(repo, ".claude", "skills", shippableSkills[0], "SKILL.md")

	if err := os.WriteFile(skill, []byte("EDIT ONE"), 0o644); err != nil {
		t.Fatalf("edit one: %v", err)
	}
	if _, err := svc.WriteShippableSkills("v0.0.0-test", true); err != nil {
		t.Fatalf("first force: %v", err)
	}
	if err := os.WriteFile(skill, []byte("EDIT TWO"), 0o644); err != nil {
		t.Fatalf("edit two: %v", err)
	}
	if _, err := svc.WriteShippableSkills("v0.0.0-test", true); err != nil {
		t.Fatalf("second force: %v", err)
	}

	backups := backupsFor(t, skill)
	if len(backups) != 2 {
		t.Fatalf("want 2 distinct backups, got %d: %v", len(backups), backups)
	}
	contents := map[string]bool{}
	for _, b := range backups {
		data, err := os.ReadFile(b)
		if err != nil {
			t.Fatalf("read backup %s: %v", b, err)
		}
		contents[string(data)] = true
	}
	if !contents["EDIT ONE"] || !contents["EDIT TWO"] {
		t.Errorf("backups lost an edit; contents = %v", contents)
	}
}

// A --force run over an unmodified install is a no-op: content already matches the
// embedded set, so nothing is overwritten and no backups accumulate.
func TestWriteShippableSkillsForceSkipsIdentical(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := svc.WriteShippableSkills("v0.0.0-test", false); err != nil {
		t.Fatalf("first write: %v", err)
	}

	res, err := svc.WriteShippableSkills("v0.0.0-test", true)
	if err != nil {
		t.Fatalf("force write: %v", err)
	}
	if len(res.Written) != 0 || len(res.Overwritten) != 0 || len(res.BackedUp) != 0 {
		t.Errorf("force over identical install changed files: %+v", res)
	}
}

func TestWriteShippableSkillsRejectsInvalidMarkerBeforeWriting(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		marker  string
		wantErr string
	}{
		{"conflicting dual", "taskrail_version: old\nmetadata:\n  taskrail_version: new\n", "conflicting"},
		{"duplicate legacy", "taskrail_version: old\ntaskrail_version: new\n", "duplicate"},
		{"empty nested", "metadata:\n  taskrail_version: ''\n", "non-empty"},
		{"non-string nested", "metadata:\n  taskrail_version: [old]\n", "string"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := initGitRepo(t)
			svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))
			if _, err := svc.Init(InitInput{}); err != nil {
				t.Fatalf("init: %v", err)
			}

			invalid := filepath.Join(repo, shippableSkillTargets[1], shippableSkills[0], skillFileName)
			writeFile(t, invalid, string(skillDocument("name: probe\ndescription: useful\n"+tc.marker)))
			before := snapshotTree(t, repo)

			if _, err := svc.WriteShippableSkills("v0.5.0", true); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("WriteShippableSkills error = %v, want %q refusal", err, tc.wantErr)
			}
			after := snapshotTree(t, repo)
			if len(after) != len(before) {
				t.Fatalf("refusal changed file count: %d -> %d", len(before), len(after))
			}
			for path, want := range before {
				if after[path] != want {
					t.Errorf("refusal changed %s", path)
				}
			}
		})
	}
}

func TestWriteShippableSkillsForceNormalizesMatchingDualMarker(t *testing.T) {
	t.Parallel()

	repo, svc := installedSkillsRepo(t, "v0.4.0")
	dual := filepath.Join(repo, shippableSkillTargets[0], shippableSkills[0], skillFileName)
	data, err := os.ReadFile(dual)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	data = []byte(strings.Replace(string(data), "description:", "taskrail_version: v0.4.0\ndescription:", 1))
	if err := os.WriteFile(dual, data, 0o644); err != nil {
		t.Fatalf("write dual marker: %v", err)
	}

	if _, err := svc.WriteShippableSkills("v0.5.0", true); err != nil {
		t.Fatalf("refresh dual install: %v", err)
	}
	got, err := os.ReadFile(dual)
	if err != nil {
		t.Fatalf("read refreshed skill: %v", err)
	}
	if strings.Contains(string(got), "\ntaskrail_version:") || !strings.Contains(string(got), "metadata:\n    taskrail_version: v0.5.0") {
		t.Fatalf("refresh did not normalize matching dual marker:\n%s", got)
	}
}

func TestWriteShippableSkillsForceNormalizesLegacyMarker(t *testing.T) {
	t.Parallel()

	repo, svc := installedSkillsRepo(t, "v0.4.0")
	legacy := filepath.Join(repo, shippableSkillTargets[0], shippableSkills[0], skillFileName)
	data, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	data = []byte(strings.Replace(string(data), "metadata:\n    taskrail_version: v0.4.0", "taskrail_version: v0.4.0", 1))
	if err := os.WriteFile(legacy, data, 0o644); err != nil {
		t.Fatalf("write legacy marker: %v", err)
	}

	if _, err := svc.WriteShippableSkills("v0.5.0", true); err != nil {
		t.Fatalf("refresh legacy install: %v", err)
	}
	got, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatalf("read refreshed skill: %v", err)
	}
	if strings.Contains(string(got), "\ntaskrail_version:") || !strings.Contains(string(got), "metadata:\n    taskrail_version: v0.5.0") {
		t.Fatalf("refresh did not normalize marker:\n%s", got)
	}
}
