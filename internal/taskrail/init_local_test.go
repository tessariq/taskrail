package taskrail

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitLocalCreatesIgnoredOverlay(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	command := exec.Command("git", "init", "--quiet", repo)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, output)
	}
	requireRecoveryDirectoryDurability(t, repo)
	svc := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	result, err := svc.Init(InitInput{Local: true})
	if err != nil {
		t.Fatalf("init local: %v", err)
	}
	if result.StorageMode != string(StorageLocal) || !result.Applied {
		t.Fatalf("result = %+v, want an applied local initialization", result)
	}
	for _, write := range result.Writes {
		if write.Action != writeActionCreate {
			t.Fatalf("local init reported %s as %s, want create", write.Path, write.Action)
		}
	}
	for _, logical := range []string{"specs/README.md", "specs/v0.1.0.md", "planning/STATE.md", "planning/NOTES.md"} {
		physical := filepath.Join(repo, ".taskrail", "local", filepath.FromSlash(logical))
		if _, err := os.Stat(physical); err != nil {
			t.Fatalf("local scaffold %s: %v", logical, err)
		}
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(logical))); !os.IsNotExist(err) {
			t.Fatalf("committed scaffold %s unexpectedly exists: %v", logical, err)
		}
	}
	marker, err := os.ReadFile(filepath.Join(repo, ".taskrail", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := decodeLayoutMarkerStrict(marker); err != nil || got.StorageMode != StorageLocal {
		t.Fatalf("local marker = %+v, err = %v", got, err)
	}
	origin, err := os.ReadFile(filepath.Join(repo, ".taskrail", "local", "runtime", "origin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		SchemaVersion int    `json:"schema_version"`
		WorktreeRoot  string `json:"worktree_root"`
		GitCommonDir  string `json:"git_common_dir"`
	}
	if err := json.Unmarshal(origin, &decoded); err != nil {
		t.Fatalf("decode origin: %v", err)
	}
	if decoded.SchemaVersion != 1 || decoded.WorktreeRoot != repo || decoded.GitCommonDir == "" {
		t.Fatalf("origin = %+v", decoded)
	}
	exclude, err := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(exclude), ".taskrail/local/") {
		t.Fatalf("exclude does not own local storage:\n%s", exclude)
	}
	for _, root := range shippableSkillTargets {
		if _, err := os.Stat(filepath.Join(repo, root)); !os.IsNotExist(err) {
			t.Fatalf("plain local init changed assistant root %s: %v", root, err)
		}
	}
	runGit(t, repo, "diff", "--quiet")
	runGit(t, repo, "diff", "--cached", "--quiet")
	output := gitOutput(t, repo, "status", "--porcelain")
	if output != "" {
		t.Fatalf("git status = %q, want clean", output)
	}
}

func TestInitLocalRejectsUpgradeOnlyInputsBeforeWriting(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	command := exec.Command("git", "init", "--quiet", repo)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, output)
	}
	svc := newTestService(t, repo, time.Now())
	if _, err := svc.Init(InitInput{Local: true, ConfirmQuiescent: true}); err == nil {
		t.Fatal("init --local accepted an inapplicable upgrade flag")
	}
	if _, err := os.Stat(filepath.Join(repo, ".taskrail")); !os.IsNotExist(err) {
		t.Fatalf("rejected local init wrote Taskrail storage: %v", err)
	}
	if output := gitOutput(t, repo, "status", "--porcelain"); output != "" {
		t.Fatalf("rejected local init changed Git status: %q", output)
	}
}

func gitOutput(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(output))
}
