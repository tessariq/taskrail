package taskrail

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// gitStatusPorcelain lists every path Git would show, the exact surface the
// committed-mode artifact-ignore contract must keep clean.
func gitStatusPorcelain(t *testing.T, repo string) []string {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "status", "--porcelain", "--untracked-files=all").Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	var paths []string
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line != "" {
			paths = append(paths, strings.TrimSpace(line[3:]))
		}
	}
	return paths
}

func gitignoreBytes(t *testing.T, repo string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	return string(data)
}

// A fresh committed-mode init must keep artifact output out of Git status
// without hand configuration: the worktree-root .gitignore carries Taskrail's
// ignore rule from the first run (T-398).
func TestInitIgnoresArtifactOutputInFreshGitRepo(t *testing.T) {
	repo := realGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	result, err := svc.Init(InitInput{})
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	var entry *WriteEntry
	for i, write := range result.Writes {
		if write.Path == ".gitignore" {
			entry = &result.Writes[i]
		}
	}
	if entry == nil {
		t.Fatal("init writes do not report .gitignore")
	}
	if entry.Kind != writeKindConfig || entry.Action != writeActionCreate {
		t.Fatalf(".gitignore entry = %+v, want kind %q action %q", *entry, writeKindConfig, writeActionCreate)
	}
	want := "# taskrail artifacts begin\n/planning/artifacts/\n# taskrail artifacts end\n"
	if got := gitignoreBytes(t, repo); got != want {
		t.Fatalf(".gitignore = %q, want %q", got, want)
	}

	// What verify would write lands under the artifacts tree and must never
	// reach Git status.
	writeFile(t, filepath.Join(repo, "planning", "artifacts", "verify", "T-001", "20260331T120000Z-id", "report.json"), "{}\n")
	for _, path := range gitStatusPorcelain(t, repo) {
		if strings.HasPrefix(path, "planning/artifacts/") {
			t.Fatalf("git status lists artifact path %s", path)
		}
	}
}

// Re-running init never duplicates the block or touches the file again: the
// second run reports the .gitignore as preserved and leaves its bytes alone.
func TestInitArtifactIgnoreIsIdempotent(t *testing.T) {
	repo := realGitRepo(t)
	at := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)

	if _, err := newTestService(t, repo, at).Init(InitInput{}); err != nil {
		t.Fatalf("first init: %v", err)
	}
	afterFirst := gitignoreBytes(t, repo)

	result, err := newTestService(t, repo, at).Init(InitInput{})
	if err != nil {
		t.Fatalf("second init: %v", err)
	}
	if afterSecond := gitignoreBytes(t, repo); afterSecond != afterFirst {
		t.Fatalf("re-run changed .gitignore:\n%q\n%q", afterFirst, afterSecond)
	}
	if strings.Count(gitignoreBytes(t, repo), "# taskrail artifacts begin") != 1 {
		t.Fatalf("re-run duplicated the ignore block:\n%s", gitignoreBytes(t, repo))
	}
	for _, write := range result.Writes {
		if write.Path == ".gitignore" && write.Action != writeActionPreserve {
			t.Fatalf("re-run reports .gitignore action %q, want %q", write.Action, writeActionPreserve)
		}
	}
}

// Appending to user-authored content must not merge into the last user line:
// the block lands after a separating newline and every user byte survives.
func TestInitArtifactIgnorePreservesUserGitignoreContent(t *testing.T) {
	repo := realGitRepo(t)
	writeFile(t, filepath.Join(repo, ".gitignore"), "node_modules/\n/dist")
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	result, err := svc.Init(InitInput{})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	want := "node_modules/\n/dist\n# taskrail artifacts begin\n/planning/artifacts/\n# taskrail artifacts end\n"
	if got := gitignoreBytes(t, repo); got != want {
		t.Fatalf(".gitignore = %q, want %q", got, want)
	}
	for _, write := range result.Writes {
		if write.Path == ".gitignore" && write.Action != writeActionRefresh {
			t.Fatalf(".gitignore action = %q, want %q", write.Action, writeActionRefresh)
		}
	}
}

// A .gitignore that already ignores the artifacts directory through an
// equivalent user rule is left byte-for-byte unchanged.
func TestInitArtifactIgnoreSkipsEquivalentUserRule(t *testing.T) {
	for _, rule := range []string{"planning/artifacts/", "/planning/artifacts/", "planning/artifacts", "/planning/artifacts"} {
		t.Run(rule, func(t *testing.T) {
			repo := realGitRepo(t)
			seeded := "node_modules/\n" + rule + "\nbuild/\n"
			writeFile(t, filepath.Join(repo, ".gitignore"), seeded)
			svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

			result, err := svc.Init(InitInput{})
			if err != nil {
				t.Fatalf("init: %v", err)
			}
			if got := gitignoreBytes(t, repo); got != seeded {
				t.Fatalf(".gitignore = %q, want unchanged %q", got, seeded)
			}
			for _, write := range result.Writes {
				if write.Path == ".gitignore" && write.Action != writeActionPreserve {
					t.Fatalf(".gitignore action = %q, want %q", write.Action, writeActionPreserve)
				}
			}
		})
	}
}

// Adoption marks an existing v0.1.0 tree without touching a byte of it, so it
// gains no ignore rule; a later current-layout init run adds it.
func TestInitAdoptionLeavesIgnoreManagementToLaterRuns(t *testing.T) {
	repo := seedLegacyFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	result, err := svc.Init(InitInput{})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	for _, write := range result.Writes {
		if write.Path == ".gitignore" {
			t.Fatalf("adoption reported .gitignore %+v", write)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf("adoption wrote .gitignore: %v", err)
	}
}

// A marked repository outside any Git worktree has no ignore state to manage,
// so init reports no .gitignore entry and writes nothing.
func TestInitOutsideGitManagesNoIgnoreEntry(t *testing.T) {
	repo := seedFixtureRepo(t)
	if err := os.RemoveAll(filepath.Join(repo, ".git")); err != nil {
		t.Fatalf("remove stub .git: %v", err)
	}
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	result, err := svc.Init(InitInput{})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	for _, write := range result.Writes {
		if write.Path == ".gitignore" {
			t.Fatalf("non-Git init reported .gitignore %+v", write)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf("non-Git init wrote .gitignore: %v", err)
	}
}

// Plain init in a local-mode repository must stay a write-free no-op: local
// planning keeps the worktree clean through .git/info/exclude, so neither the
// plain current-layout path nor the local skills refresh writes or reports a
// worktree .gitignore.
func TestPlainInitInLocalRepositoryManagesNoGitignore(t *testing.T) {
	skipDurableSkillPublication(t)

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	at := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)

	if _, err := newTestService(t, repo, at).Init(InitInput{Local: true, WithSkills: true, SkillVersion: "v9.9.9"}); err != nil {
		t.Fatalf("local init: %v", err)
	}
	for _, tc := range []struct {
		name  string
		input InitInput
	}{
		{name: "plain init", input: InitInput{}},
		{name: "local skills refresh", input: InitInput{WithSkills: true, ForceSkills: true, SkillVersion: "v9.9.9"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := newTestService(t, repo, at).Init(tc.input)
			if err != nil {
				t.Fatalf("init: %v", err)
			}
			for _, write := range result.Writes {
				if write.Path == gitignoreFile {
					t.Fatalf("local repository init reported %s entry %+v", gitignoreFile, write)
				}
			}
			if _, err := os.Stat(filepath.Join(repo, gitignoreFile)); !os.IsNotExist(err) {
				t.Fatalf("local repository init wrote %s: %v", gitignoreFile, err)
			}
		})
	}
}

// Retrofit preview and apply must agree about the worktree .gitignore the
// apply writes; both Changes inventories name it.
func TestRetrofitPreviewReportsArtifactsIgnore(t *testing.T) {
	t.Parallel()

	repo, _ := seedRetrofitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	preview, err := svc.Retrofit(RetrofitInput{})
	if err != nil {
		t.Fatalf("retrofit preview: %v", err)
	}
	if !changesMention(preview.Changes, "create "+gitignoreFile) {
		t.Fatalf("preview changes must name the .gitignore write, got %v", preview.Changes)
	}

	applied, err := svc.Retrofit(RetrofitInput{Apply: true})
	if err != nil {
		t.Fatalf("retrofit apply: %v", err)
	}
	if !changesMention(applied.Changes, "create "+gitignoreFile) {
		t.Fatalf("applied changes must name the .gitignore write, got %v", applied.Changes)
	}
	if data, err := os.ReadFile(filepath.Join(repo, gitignoreFile)); err != nil || !strings.Contains(string(data), artifactsIgnoreBegin) {
		t.Fatalf("retrofit apply did not write the ignore block: %q %v", data, err)
	}
}

// A user negation deliberately re-includes the artifacts directory, and the
// appended block would override it (last matching pattern wins), so init
// appends nothing.
func TestInitArtifactIgnoreRespectsNegation(t *testing.T) {
	repo := realGitRepo(t)
	seeded := "planning/*\n!planning/artifacts/\n"
	writeFile(t, filepath.Join(repo, ".gitignore"), seeded)
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	result, err := svc.Init(InitInput{})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if got := gitignoreBytes(t, repo); got != seeded {
		t.Fatalf(".gitignore = %q, want unchanged %q", got, seeded)
	}
	for _, write := range result.Writes {
		if write.Path == gitignoreFile && write.Action != writeActionPreserve {
			t.Fatalf(".gitignore action = %q, want %q", write.Action, writeActionPreserve)
		}
	}
}

// Leading whitespace is significant to Git, so an indented lookalike rule
// ignores nothing and init still appends the working block.
func TestInitArtifactIgnoreAppendsWhenUserRuleIsIndented(t *testing.T) {
	repo := realGitRepo(t)
	writeFile(t, filepath.Join(repo, ".gitignore"), "  /planning/artifacts/\n")
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	want := "  /planning/artifacts/\n# taskrail artifacts begin\n/planning/artifacts/\n# taskrail artifacts end\n"
	if got := gitignoreBytes(t, repo); got != want {
		t.Fatalf(".gitignore = %q, want %q", got, want)
	}
	writeFile(t, filepath.Join(repo, "planning", "artifacts", "verify", "T-001", "r", "report.json"), "{}\n")
	for _, path := range gitStatusPorcelain(t, repo) {
		if strings.HasPrefix(path, "planning/artifacts/") {
			t.Fatalf("git status lists artifact path %s", path)
		}
	}
}

// A .gitignore made only of a blank line keeps that byte; the block is
// appended below it, never written over it.
func TestInitArtifactIgnoreKeepsSoleBlankLine(t *testing.T) {
	repo := realGitRepo(t)
	writeFile(t, filepath.Join(repo, ".gitignore"), "\n")
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	want := "\n# taskrail artifacts begin\n/planning/artifacts/\n# taskrail artifacts end\n"
	if got := gitignoreBytes(t, repo); got != want {
		t.Fatalf(".gitignore = %q, want %q", got, want)
	}
}

// A symlinked .gitignore is never written through: a target that already
// manages the rule reports preserve, and a target needing a write refuses
// with path_blocked and publishes nothing.
func TestInitRefusesToWriteThroughLinkedGitignore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlinks require privileges this test cannot assume")
	}
	t.Run("covered target reports preserve", func(t *testing.T) {
		repo := realGitRepo(t)
		writeFile(t, filepath.Join(repo, "ignore-actual"), "/planning/artifacts/\n")
		if err := os.Symlink("ignore-actual", filepath.Join(repo, gitignoreFile)); err != nil {
			t.Fatal(err)
		}
		svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

		result, err := svc.Init(InitInput{})
		if err != nil {
			t.Fatalf("init: %v", err)
		}
		for _, write := range result.Writes {
			if write.Path == gitignoreFile && write.Action != writeActionPreserve {
				t.Fatalf(".gitignore action = %q, want %q", write.Action, writeActionPreserve)
			}
		}
		if target, err := os.ReadFile(filepath.Join(repo, "ignore-actual")); err != nil || string(target) != "/planning/artifacts/\n" {
			t.Fatalf("linked target changed: %q %v", target, err)
		}
	})
	t.Run("uncovered target refuses", func(t *testing.T) {
		repo := realGitRepo(t)
		writeFile(t, filepath.Join(repo, "ignore-actual"), "node_modules/\n")
		if err := os.Symlink("ignore-actual", filepath.Join(repo, gitignoreFile)); err != nil {
			t.Fatal(err)
		}
		svc := newTestService(t, repo, time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC))

		_, err := svc.Init(InitInput{})
		if err == nil {
			t.Fatal("init must refuse to append through a linked .gitignore")
		}
		if code := MachineFailureFor(err).Code; code != MachineCodePathBlocked {
			t.Fatalf("error code = %q, want %q", code, MachineCodePathBlocked)
		}
		if _, statErr := os.Stat(filepath.Join(repo, ".taskrail", "config.yml")); !os.IsNotExist(statErr) {
			t.Fatalf("refused init published the marker: %v", statErr)
		}
		if target, readErr := os.ReadFile(filepath.Join(repo, "ignore-actual")); readErr != nil || string(target) != "node_modules/\n" {
			t.Fatalf("linked target changed: %q %v", target, readErr)
		}
	})
}
