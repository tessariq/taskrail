package taskrail

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPromptListAndShowResolveCommittedReplacement(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))

	list, err := svc.PromptList()
	if err != nil {
		t.Fatalf("list prompts: %v", err)
	}
	gotIDs := make([]string, len(list.Prompts))
	for i, prompt := range list.Prompts {
		gotIDs[i] = prompt.ID
		if prompt.ContractVersion != "v1" || prompt.Source != "builtin" || prompt.ReplacementPath != nil {
			t.Fatalf("builtin prompt = %+v, want v1 builtin with no path", prompt)
		}
	}
	wantIDs := []string{
		"task-implementation", "task-authoring", "task-review", "spec-consistency",
		"spec-gaps", "spec-additions", "spec-adversarial", "task-decomposition",
		"task-decomposition-adversarial", "workflow-adversarial",
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("prompt order = %v, want %v", gotIDs, wantIDs)
	}

	builtin, err := svc.PromptShow(PromptShowInput{ID: "task-implementation"})
	if err != nil {
		t.Fatalf("show built-in: %v", err)
	}
	path := filepath.Join(repo, ".taskrail", "prompts", "v1", "task-implementation.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create replacement directory: %v", err)
	}
	const replacement = "Use the selected task {{TASK_ID}}.\n"
	if err := os.WriteFile(path, []byte(replacement), 0o644); err != nil {
		t.Fatalf("write replacement: %v", err)
	}

	resolved, err := svc.PromptShow(PromptShowInput{ID: "task-implementation"})
	if err != nil {
		t.Fatalf("show replacement: %v", err)
	}
	if resolved.Content != replacement || resolved.Source != "replacement" || resolved.ReplacementPath == nil || *resolved.ReplacementPath != ".taskrail/prompts/v1/task-implementation.md" {
		t.Fatalf("replacement = %+v, want committed replacement result", resolved)
	}
	if resolved.SHA256 != resolved.TemplateSHA256 {
		t.Fatalf("show digests = %q and %q, want equal", resolved.SHA256, resolved.TemplateSHA256)
	}
	if resolved.SHA256 == builtin.SHA256 {
		t.Fatal("replacement digest matched the different builtin bytes")
	}

	forced, err := svc.PromptShow(PromptShowInput{ID: "task-implementation", Builtin: true})
	if err != nil {
		t.Fatalf("show forced built-in: %v", err)
	}
	if forced != builtin {
		t.Fatalf("forced builtin = %+v, want %+v", forced, builtin)
	}
}

func TestPromptShowResolvesLocalReplacementThroughOverlay(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	svc := newLocalTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))

	builtin, err := svc.PromptShow(PromptShowInput{ID: "task-implementation", Builtin: true})
	if err != nil {
		t.Fatalf("show built-in: %v", err)
	}
	localPath := filepath.Join(repo, localStorageRoot, "prompts", "v1", "task-implementation.md")
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		t.Fatalf("create local replacement directory: %v", err)
	}
	if err := os.WriteFile(localPath, []byte(builtin.Content), 0o644); err != nil {
		t.Fatalf("write local replacement: %v", err)
	}
	excludePath := filepath.Join(repo, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		t.Fatalf("create Git exclusion directory: %v", err)
	}
	if err := os.WriteFile(excludePath, []byte(".taskrail/local/\n"), 0o644); err != nil {
		t.Fatalf("ignore local overlay: %v", err)
	}

	list, err := svc.PromptList()
	if err != nil {
		t.Fatalf("list local replacement: %v", err)
	}
	if list.Prompts[0].Source != "replacement" || list.Prompts[0].ReplacementPath == nil || *list.Prompts[0].ReplacementPath != ".taskrail/prompts/v1/task-implementation.md" {
		t.Fatalf("local replacement list metadata = %+v, want logical replacement metadata", list.Prompts[0])
	}
	resolved, err := svc.PromptShow(PromptShowInput{ID: "task-implementation"})
	if err != nil {
		t.Fatalf("show local replacement: %v", err)
	}
	if resolved.Source != "replacement" || resolved.ReplacementPath == nil || *resolved.ReplacementPath != ".taskrail/prompts/v1/task-implementation.md" {
		t.Fatalf("local replacement metadata = %+v, want logical replacement metadata", resolved)
	}
	if resolved.Content != builtin.Content || resolved.SHA256 != builtin.SHA256 || resolved.TemplateSHA256 != builtin.TemplateSHA256 {
		t.Fatalf("equal-byte local replacement = %+v, want built-in bytes and digests %+v", resolved, builtin)
	}
}

func TestPromptShowRejectsUnsafeLocalReplacementSources(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, repo string)
		code  string
	}{
		{
			name: "committed collision",
			setup: func(t *testing.T, repo string) {
				writeLocalPromptReplacement(t, repo, "replacement")
				writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-review.md"), "committed")
			},
			code: MachineCodePathBlocked,
		},
		{
			name: "contract alias",
			setup: func(t *testing.T, repo string) {
				writeFile(t, filepath.Join(repo, localStorageRoot, "prompts", "V1", "task-review.md"), "replacement")
			},
			code: MachineCodePathBlocked,
		},
		{
			name: "tracked overlay",
			setup: func(t *testing.T, repo string) {
				writeLocalPromptReplacement(t, repo, "replacement")
				runLocalGit(t, repo, "add", "-f", localStorageRoot)
			},
			code: MachineCodePathBlocked,
		},
		{
			name: "replacement not ignored",
			setup: func(t *testing.T, repo string) {
				writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), ".taskrail/local/prompts/v1/replacement.md\n")
				writeLocalPromptReplacement(t, repo, "replacement")
			},
			code: MachineCodePathBlocked,
		},
		{
			name: "invalid UTF-8",
			setup: func(t *testing.T, repo string) {
				writeLocalPromptReplacement(t, repo, string([]byte{0xff}))
			},
			code: MachineCodePromptInvalid,
		},
		{
			name: "oversize",
			setup: func(t *testing.T, repo string) {
				writeLocalPromptReplacement(t, repo, strings.Repeat("x", promptReplacementLimit+1))
			},
			code: MachineCodePromptInvalid,
		},
		{
			name: "far oversize",
			setup: func(t *testing.T, repo string) {
				writeLocalPromptReplacement(t, repo, strings.Repeat("x", promptReplacementLimit+2))
			},
			code: MachineCodePromptInvalid,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := t.TempDir()
			initLocalGitRepo(t, repo)
			writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), ".taskrail/local/\n")
			test.setup(t, repo)
			svc := newLocalTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
			before := gitOutput(t, repo, "status", "--porcelain")

			if _, err := svc.PromptShow(PromptShowInput{ID: "task-review"}); MachineFailureFor(err).Code != test.code {
				t.Fatalf("show error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, test.code)
			}
			if after := gitOutput(t, repo, "status", "--porcelain"); after != before {
				t.Fatalf("prompt show changed Git status from %q to %q", before, after)
			}
		})
	}
}

func TestPromptShowRejectsLocalReplacementPathSubstitution(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), ".taskrail/local/\n")
	target := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, localStorageRoot, "prompts"), 0o755); err != nil {
		t.Fatalf("create local prompts parent: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(repo, localStorageRoot, "prompts", "v1")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	svc := newLocalTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	before := gitOutput(t, repo, "status", "--porcelain")

	if _, err := svc.PromptShow(PromptShowInput{ID: "task-review"}); err == nil {
		t.Fatal("show accepted a substituted local replacement path")
	}
	if after := gitOutput(t, repo, "status", "--porcelain"); after != before {
		t.Fatalf("prompt show changed Git status from %q to %q", before, after)
	}
}

func writeLocalPromptReplacement(t *testing.T, repo, content string) {
	t.Helper()
	writeFile(t, filepath.Join(repo, localStorageRoot, "prompts", "v1", "task-review.md"), content)
}

func TestPromptShowRejectsUnknownAndInvalidCommittedReplacement(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))

	if _, err := svc.PromptShow(PromptShowInput{ID: "unknown"}); MachineFailureFor(err).Code != MachineCodePromptNotFound {
		t.Fatalf("unknown prompt code = %q, want %q", MachineFailureFor(err).Code, MachineCodePromptNotFound)
	}
	if _, err := svc.PromptShow(PromptShowInput{ID: "task-review", Contract: "v2"}); MachineFailureFor(err).Code != MachineCodePromptNotFound {
		t.Fatalf("unknown contract code = %q, want %q", MachineFailureFor(err).Code, MachineCodePromptNotFound)
	}

	path := filepath.Join(repo, ".taskrail", "prompts", "v1", "task-review.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create replacement directory: %v", err)
	}
	if err := os.WriteFile(path, []byte{0xff}, 0o644); err != nil {
		t.Fatalf("write invalid replacement: %v", err)
	}
	if _, err := svc.PromptShow(PromptShowInput{ID: "task-review"}); MachineFailureFor(err).Code != MachineCodePromptInvalid {
		t.Fatalf("invalid replacement code = %q, want %q", MachineFailureFor(err).Code, MachineCodePromptInvalid)
	}
	if _, err := svc.PromptList(); MachineFailureFor(err).Code != MachineCodePromptInvalid {
		t.Fatalf("invalid replacement list code = %q, want %q", MachineFailureFor(err).Code, MachineCodePromptInvalid)
	}
}

func TestPromptCatalogRejectsUnknownReplacementContractAndAlias(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))

	unknown := filepath.Join(repo, ".taskrail", "prompts", "v2")
	if err := os.MkdirAll(unknown, 0o755); err != nil {
		t.Fatalf("create unknown contract: %v", err)
	}
	if _, err := svc.PromptList(); MachineFailureFor(err).Code != MachineCodePromptInvalid {
		t.Fatalf("unknown contract code = %q, want %q", MachineFailureFor(err).Code, MachineCodePromptInvalid)
	}
	if err := os.RemoveAll(unknown); err != nil {
		t.Fatalf("remove unknown contract: %v", err)
	}

	alias := filepath.Join(repo, ".taskrail", "prompts", "V1")
	if err := os.MkdirAll(alias, 0o755); err != nil {
		t.Fatalf("create alias contract: %v", err)
	}
	if _, err := svc.PromptList(); MachineFailureFor(err).Code != MachineCodePromptInvalid {
		t.Fatalf("list alias contract code = %q, want %q", MachineFailureFor(err).Code, MachineCodePromptInvalid)
	}
	if _, err := svc.PromptShow(PromptShowInput{ID: "task-review"}); MachineFailureFor(err).Code != MachineCodePathBlocked {
		t.Fatalf("show alias contract code = %q, want %q", MachineFailureFor(err).Code, MachineCodePathBlocked)
	}
}
