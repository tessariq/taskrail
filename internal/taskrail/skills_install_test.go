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

// Default init must never provision agent-tool skill directories; writing them
// is opt-in via --with-skills (skills-productization.md Decision 2).
func TestInitDefaultWritesNoSkillDirs(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.Init(false); err != nil {
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
	if _, err := svc.Init(false); err != nil {
		t.Fatalf("init: %v", err)
	}

	res, err := svc.WriteShippableSkills(false)
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
	if _, err := svc.Init(false); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := svc.WriteShippableSkills(false); err != nil {
		t.Fatalf("first write: %v", err)
	}

	edited := filepath.Join(repo, ".claude", "skills", shippableSkills[0], "SKILL.md")
	const userMark = "USER EDIT — do not clobber"
	if err := os.WriteFile(edited, []byte(userMark), 0o644); err != nil {
		t.Fatalf("edit skill: %v", err)
	}

	res, err := svc.WriteShippableSkills(false)
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
	if _, err := svc.Init(false); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := svc.WriteShippableSkills(false); err != nil {
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

	res, err := svc.WriteShippableSkills(true)
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
	if _, err := svc.Init(false); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := svc.WriteShippableSkills(false); err != nil {
		t.Fatalf("first write: %v", err)
	}

	skill := filepath.Join(repo, ".claude", "skills", shippableSkills[0], "SKILL.md")

	if err := os.WriteFile(skill, []byte("EDIT ONE"), 0o644); err != nil {
		t.Fatalf("edit one: %v", err)
	}
	if _, err := svc.WriteShippableSkills(true); err != nil {
		t.Fatalf("first force: %v", err)
	}
	if err := os.WriteFile(skill, []byte("EDIT TWO"), 0o644); err != nil {
		t.Fatalf("edit two: %v", err)
	}
	if _, err := svc.WriteShippableSkills(true); err != nil {
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
	if _, err := svc.Init(false); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := svc.WriteShippableSkills(false); err != nil {
		t.Fatalf("first write: %v", err)
	}

	res, err := svc.WriteShippableSkills(true)
	if err != nil {
		t.Fatalf("force write: %v", err)
	}
	if len(res.Written) != 0 || len(res.Overwritten) != 0 || len(res.BackedUp) != 0 {
		t.Errorf("force over identical install changed files: %+v", res)
	}
}
