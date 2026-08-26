package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/durablefs"
)

func TestLoopGitConfigSnapshotCapturesCommonAndWorktreeConfiguration(t *testing.T) {
	worktree, common := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(worktree, "config.worktree"), []byte("[core]\nworktree = true\n"), 0o600); err != nil {
		t.Fatalf("write worktree configuration: %v", err)
	}
	if err := os.WriteFile(filepath.Join(common, "config"), []byte("[core]\ncommon = true\n"), 0o600); err != nil {
		t.Fatalf("write common configuration: %v", err)
	}

	configs, err := loopGitConfigSnapshot(worktree, common)
	if err != nil {
		t.Fatalf("loopGitConfigSnapshot: %v", err)
	}
	for configPath, want := range map[string]string{
		filepath.Join(worktree, "config.worktree"): "[core]\nworktree = true\n",
		filepath.Join(common, "config"):            "[core]\ncommon = true\n",
	} {
		config, ok := configs[configPath]
		if !ok || !config.Present || string(config.Bytes) != want {
			t.Fatalf("configuration %s = %+v, want present bytes %q", configPath, config, want)
		}
	}
	if config := configs[filepath.Join(worktree, "config")]; config.Present {
		t.Fatalf("absent worktree configuration = %+v", config)
	}
}

func TestLoopGitConfigSnapshotCapturesLinkedWorktreeConfiguration(t *testing.T) {
	repo := realGitRepo(t)
	worktree := filepath.Join(t.TempDir(), "linked")
	runGit(t, repo, "worktree", "add", "-b", "linked", worktree)
	git, err := discoverGitWorktree(worktree)
	if err != nil {
		t.Fatalf("discover linked worktree: %v", err)
	}
	if git.GitDir == git.GitCommonDir {
		t.Fatalf("linked worktree Git directory = common directory = %q", git.GitDir)
	}
	if err := os.WriteFile(filepath.Join(git.GitDir, "config.worktree"), []byte("[core]\nworktree = true\n"), 0o600); err != nil {
		t.Fatalf("write worktree configuration: %v", err)
	}
	configs, err := loopGitConfigSnapshot(git.GitDir, git.GitCommonDir)
	if err != nil {
		t.Fatalf("loopGitConfigSnapshot: %v", err)
	}
	if common := configs[filepath.Join(git.GitCommonDir, "config")]; !common.Present || len(common.Bytes) == 0 {
		t.Fatalf("common configuration = %+v, want present bytes", common)
	}
	if worktree := configs[filepath.Join(git.GitDir, "config.worktree")]; !worktree.Present || string(worktree.Bytes) != "[core]\nworktree = true\n" {
		t.Fatalf("worktree configuration = %+v", worktree)
	}
}

func TestLoopIntegrityRejectsGitConfigurationChanges(t *testing.T) {
	before := loopIntegrityFixtureInputs(t)
	preflight := loopIntegrityPreflight(before)
	preflight.gitConfig = map[string]loopGitConfigFile{
		"/git/worktree/config.worktree": {Present: true, Bytes: []byte("worktree\n"), Snapshot: durablefs.Snapshot{SHA256: "worktree", Identity: durablefs.Identity{File: 1}}},
		"/git/common/config":            {Present: true, Bytes: []byte("common\n"), Snapshot: durablefs.Snapshot{SHA256: "common", Identity: durablefs.Identity{File: 2}}},
		"/git/common/config.worktree":   {},
	}
	for _, test := range []struct {
		name   string
		mutate func(map[string]loopGitConfigFile)
		path   string
	}{
		{
			name: "worktree bytes", path: "/git/worktree/config.worktree",
			mutate: func(configs map[string]loopGitConfigFile) {
				config := configs["/git/worktree/config.worktree"]
				config.Bytes = []byte("changed\n")
				configs["/git/worktree/config.worktree"] = config
			},
		},
		{
			name: "common replacement", path: "/git/common/config",
			mutate: func(configs map[string]loopGitConfigFile) {
				config := configs["/git/common/config"]
				config.Snapshot.Identity.File = 3
				configs["/git/common/config"] = config
			},
		},
		{
			name: "absent creation", path: "/git/common/config.worktree",
			mutate: func(configs map[string]loopGitConfigFile) {
				configs["/git/common/config.worktree"] = loopGitConfigFile{Present: true, Bytes: []byte("created\n"), Snapshot: durablefs.Snapshot{SHA256: "created", Identity: durablefs.Identity{File: 4}}}
			},
		},
		{
			name: "worktree deletion", path: "/git/worktree/config.worktree",
			mutate: func(configs map[string]loopGitConfigFile) {
				configs["/git/worktree/config.worktree"] = loopGitConfigFile{}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			after := preflight.GitConfig()
			test.mutate(after)
			evidence := loopIntegrityEvidenceFor(preflight, "T-001-selected", before)
			evidence.GitConfig = after
			violations := checkLoopIntegrity(evidence)
			found := false
			for _, violation := range violations {
				if violation.Code == "git_config_changed" && violation.Path != nil && *violation.Path == test.path {
					found = true
				}
			}
			if !found {
				t.Fatalf("violations = %+v, want Git configuration mutation at %s", violations, test.path)
			}
		})
	}
}

func TestLoopGitConfigSnapshotRejectsAliasedConfiguration(t *testing.T) {
	dir := t.TempDir()
	config, alias := filepath.Join(dir, "config"), filepath.Join(dir, "config.worktree")
	if err := os.WriteFile(config, []byte("[core]\n"), 0o600); err != nil {
		t.Fatalf("write configuration: %v", err)
	}
	if err := os.Symlink(config, alias); err != nil {
		t.Skipf("create configuration alias: %v", err)
	}
	if _, err := loopGitConfigSnapshot(dir); err == nil || !strings.Contains(err.Error(), "Git configuration") {
		t.Fatalf("loopGitConfigSnapshot alias error = %v", err)
	}
}
