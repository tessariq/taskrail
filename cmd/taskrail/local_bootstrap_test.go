package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestImplicitLocalBootstrapForNext(t *testing.T) {
	repo := t.TempDir()
	if output, err := exec.Command("git", "init", "--quiet", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, output)
	}
	t.Chdir(repo)
	requireRecoveryDirectoryDurability(t, repo)

	stdout, _, err := runRootSplit(t, "next", "--json")
	if err != nil {
		t.Fatalf("next: %v (stdout %q)", err, stdout)
	}
	envelope := decodeEnvelope(t, stdout)
	if len(envelope.Warnings) != 1 || envelope.Warnings[0].Code != "local_initialized" {
		t.Fatalf("warnings = %+v, want one local_initialized warning", envelope.Warnings)
	}
	if _, err := os.Stat(filepath.Join(repo, ".taskrail", "config.yml")); err != nil {
		t.Fatalf("implicit bootstrap did not create local marker: %v", err)
	}
}

func TestImplicitLocalBootstrapCommandMatrix(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	writers := []struct {
		name      string
		args      []string
		wantError bool
	}{
		{"next", []string{"next", "--json"}, false},
		{"start", []string{"start", "T-999", "--json"}, true},
		{"complete", []string{"complete", "T-999", "--json"}, true},
		{"block", []string{"block", "T-999", "--reason", "blocked", "--json"}, true},
		{"unblock", []string{"unblock", "T-999", "--json"}, true},
		{"verify", []string{"verify", "T-999", "--result", "fail", "--summary", "missing", "--json"}, true},
		{"repair apply", []string{"repair", "--apply", "--json"}, false},
		{"task new", []string{"task", "new", "--title", "New", "--spec-ref", "specs/v0.1.0.md#summary", "--json"}, false},
		{"task rename", []string{"task", "rename", "T-999", "--title", "New", "--json"}, true},
		{"task repoint", []string{"task", "repoint", "T-999", "--area", "summary", "--json"}, true},
		{"task release", []string{"task", "release", "T-999", "--reason", "releasing", "--json"}, true},
		{"task author", []string{"task", "author", "T-999", "--body", missing, "--expect-sha256", strings.Repeat("0", 64), "--json"}, true},
		{"task loop allow", []string{"task", "loop", "allow", "T-999", "--reason", "allowed", "--json"}, true},
		{"task loop hold", []string{"task", "loop", "hold", "T-999", "--reason", "held", "--json"}, true},
		{"task loop clear", []string{"task", "loop", "clear", "T-999", "--json"}, true},
		{"spec add", []string{"spec", "add", "v0.2.0", "--json"}, false},
		{"spec activate", []string{"spec", "activate", "v9.9.9", "--json"}, true},
		{"import apply", []string{"import", "--apply", missing, "--json"}, true},
	}
	for _, writer := range writers {
		t.Run(writer.name, func(t *testing.T) {
			repo := setupUninitializedGitRepo(t)
			requireRecoveryDirectoryDurability(t, repo)
			stdout, _, err := runRootSplit(t, writer.args...)
			assertLocalBootstrap(t, repo, stdout, writer.wantError)
			if writer.wantError && err == nil {
				t.Fatal("expected a semantic refusal after bootstrap")
			}
		})
	}

	t.Run("loop execution", func(t *testing.T) {
		repo := setupUninitializedGitRepo(t)
		requireRecoveryDirectoryDurability(t, repo)
		resultPath := filepath.Join(t.TempDir(), "loop-result.json")
		_, _, _ = runRootSplit(t, "loop", "--result-file", resultPath, "--", "true")
		data, err := os.ReadFile(resultPath)
		if err != nil {
			t.Fatalf("read loop result: %v", err)
		}
		assertLocalBootstrap(t, repo, string(data), true)
	})
}

func TestImplicitLocalBootstrapExclusions(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	readersAndPreviews := []struct {
		name string
		args []string
	}{
		{"validate", []string{"validate", "--json"}},
		{"coverage", []string{"coverage", "--json"}},
		{"status", []string{"status", "--json"}},
		{"stats", []string{"stats", "--json"}},
		{"task show", []string{"task", "show", "T-999", "--json"}},
		{"task dependency", []string{"task", "dependency", "add", "T-999", "T-998", "--json"}},
		{"task dependency remove", []string{"task", "dependency", "remove", "T-999", "T-998", "--json"}},
		{"task loop list", []string{"task", "loop", "list", "--json"}},
		{"spec list", []string{"spec", "list", "--json"}},
		{"spec show", []string{"spec", "show", "v0.1.0", "--json"}},
		{"spec diff", []string{"spec", "diff", "v0.1.0", "v0.2.0", "--json"}},
		{"prompt list", []string{"prompt", "list", "--json"}},
		{"prompt show", []string{"prompt", "show", "task-review", "--json"}},
		{"prompt render", []string{"prompt", "render", "task-review", "--json"}},
		{"review show", []string{"review", "show", "planning/reviews/missing", "--json"}},
		{"retrofit", []string{"retrofit", "--json"}},
		{"local status", []string{"local", "status", "--json"}},
		{"local path", []string{"local", "path", "--json"}},
		{"lock status", []string{"lock", "status", "--json"}},
		{"lock clear", []string{"lock", "clear", "missing", "--expect-sha256", strings.Repeat("0", 64), "--json"}},
		{"recover preview", []string{"recover", strings.Repeat("0", 32), "--json"}},
		{"recover apply", []string{"recover", strings.Repeat("0", 32), "--apply", "--json"}},
		{"loop dry run", []string{"loop", "--dry-run", "--json"}},
		{"repair preview", []string{"repair", "--json"}},
		{"task rename preview", []string{"task", "rename", "T-999", "--title", "New", "--dry-run", "--json"}},
		{"task repoint preview", []string{"task", "repoint", "T-999", "--area", "summary", "--dry-run", "--json"}},
		{"task release preview", []string{"task", "release", "T-999", "--reason", "releasing", "--dry-run", "--json"}},
		{"task author preview", []string{"task", "author", "T-999", "--body", missing, "--expect-sha256", strings.Repeat("0", 64), "--dry-run", "--json"}},
		{"task loop preview", []string{"task", "loop", "allow", "T-999", "--reason", "allowed", "--dry-run", "--json"}},
		{"task loop hold preview", []string{"task", "loop", "hold", "T-999", "--reason", "held", "--dry-run", "--json"}},
		{"task loop clear preview", []string{"task", "loop", "clear", "T-999", "--dry-run", "--json"}},
		{"task dependency preview", []string{"task", "dependency", "add", "T-999", "T-998", "--dry-run", "--json"}},
		{"import preview", []string{"import", missing, "--to", "tasks", "--json"}},
		{"review publish", []string{"review", "publish", "--type", "task", "--destination", missing, "--dry-run", "--json"}},
		{"review publish apply", []string{"review", "publish", "--type", "task", "--destination", missing, "--json"}},
	}
	for _, invocation := range readersAndPreviews {
		t.Run(invocation.name, func(t *testing.T) {
			repo := setupUninitializedGitRepo(t)
			_, _, _ = runRootSplit(t, invocation.args...)
			assertNoLocalBootstrap(t, repo)
		})
	}
}

func setupUninitializedGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if output, err := exec.Command("git", "init", "--quiet", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, output)
	}
	t.Chdir(repo)
	return repo
}

func assertLocalBootstrap(t *testing.T, repo, document string, wantError bool) {
	t.Helper()
	envelope := decodeEnvelope(t, document)
	if len(envelope.Warnings) == 0 || envelope.Warnings[0].Code != "local_initialized" {
		t.Fatalf("warnings = %+v, want local_initialized", envelope.Warnings)
	}
	if wantError && envelope.Error == nil {
		t.Fatal("expected local_initialized to accompany an error envelope")
	}
	if !wantError && envelope.Error != nil {
		t.Fatalf("unexpected error envelope: %s", envelope.Error.Code)
	}
	if _, err := os.Stat(filepath.Join(repo, ".taskrail", "config.yml")); err != nil {
		t.Fatalf("implicit bootstrap did not create local marker: %v", err)
	}
	assertLocalSkillRootsAndGitStatus(t, repo, true)
}

func assertNoLocalBootstrap(t *testing.T, repo string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(repo, ".taskrail")); !os.IsNotExist(err) {
		t.Fatalf("reader or preview created local storage: %v", err)
	}
	assertLocalSkillRootsAndGitStatus(t, repo, false)
}

func assertLocalSkillRootsAndGitStatus(t *testing.T, repo string, initialized bool) {
	t.Helper()
	for _, root := range []string{".agents", ".claude"} {
		if _, err := os.Stat(filepath.Join(repo, root)); !os.IsNotExist(err) {
			t.Fatalf("command changed assistant root %s: %v", root, err)
		}
	}
	exclude, err := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("read Git exclusion: %v", err)
	}
	if strings.Contains(string(exclude), ".agents") || strings.Contains(string(exclude), ".claude") {
		t.Fatalf("command changed skill exclusions:\n%s", exclude)
	}
	if initialized && !strings.Contains(string(exclude), ".taskrail/local/") {
		t.Fatalf("implicit bootstrap did not install local exclusion:\n%s", exclude)
	}
	if !initialized && strings.Contains(string(exclude), ".taskrail/local/") {
		t.Fatalf("reader or preview installed local exclusion:\n%s", exclude)
	}
	output, err := exec.Command("git", "-C", repo, "status", "--porcelain").Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	if got := strings.TrimSpace(string(output)); got != "" {
		t.Fatalf("Git status = %q, want clean", got)
	}
}
