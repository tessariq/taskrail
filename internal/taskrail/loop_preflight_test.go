package taskrail

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tessariq/taskrail/internal/repolock"
)

func TestParseLoopInvocationRejectsInvalidFormsWithoutPreflight(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"missing delimiter", []string{"agent"}},
		{"empty child", []string{"--"}},
		{"dry run child", []string{"--dry-run", "--", "agent"}},
		{"execution json", []string{"--json", "--", "agent"}},
		{"zero iterations", []string{"--max-iterations", "0", "--", "agent"}},
		{"too many review rounds", []string{"--max-review-rounds=3", "--", "agent"}},
		{"zero timeout", []string{"--timeout", "0s", "--", "agent"}},
		{"retry", []string{"--retry", "--", "agent"}},
		{"background", []string{"--background", "--", "agent"}},
		{"duplicate delimiter", []string{"--", "agent", "--", "arg"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseLoopInvocation(test.args); err == nil {
				t.Fatal("ParseLoopInvocation succeeded")
			}
		})
	}
}

func TestParseLoopInvocationAcceptsExecutionAndDryRun(t *testing.T) {
	rounds := 2
	timeout := 90 * time.Second
	tests := []struct {
		name string
		args []string
		want LoopInvocation
	}{
		{
			name: "execution",
			args: []string{"--max-iterations=2", "--max-review-rounds", "2", "--timeout", "90s", "--result-file", "/tmp/result.json", "--", "agent", "--model", "fast"},
			want: LoopInvocation{MaxIterations: 2, Parallel: 1, CloneDepth: "1", KeepWorkspaces: "failure", Delivery: "local", MaxReviewRounds: &rounds, Timeout: &timeout, ResultFile: "/tmp/result.json", Child: []string{"agent", "--model", "fast"}},
		},
		{
			name: "dry run",
			args: []string{"--dry-run", "--json"},
			want: LoopInvocation{DryRun: true, MaxIterations: 1, Parallel: 1, CloneDepth: "1", KeepWorkspaces: "failure", Delivery: "local"},
		},
		{
			name: "parallel dry run",
			args: []string{"--dry-run", "--max-iterations", "3", "--parallel", "3", "--clone-depth", "full", "--keep-workspaces", "always", "--delivery", "review", "--review-adapter", "/tmp/adapter"},
			want: LoopInvocation{DryRun: true, MaxIterations: 3, Parallel: 3, CloneDepth: "full", CloneDepthSet: true, KeepWorkspaces: "always", KeepWorkspacesSet: true, Delivery: "review", DeliverySet: true, ReviewAdapter: "/tmp/adapter", ReviewAdapterSet: true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseLoopInvocation(test.args)
			if err != nil {
				t.Fatalf("ParseLoopInvocation: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("invocation = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestParseLoopInvocationRejectsStaleParallelIntent(t *testing.T) {
	for _, args := range [][]string{
		{"--dry-run", "--clone-depth", "1"},
		{"--dry-run", "--parallel", "2", "--max-iterations", "1", "--keep-workspaces", "always"},
		{"--dry-run", "--delivery", "review", "--review-adapter", "/tmp/adapter"},
	} {
		if _, err := ParseLoopInvocation(args); err == nil {
			t.Fatalf("ParseLoopInvocation(%q) succeeded", args)
		}
	}
}

func TestLoopPreflightCapturesCleanRepositoryWithoutMutation(t *testing.T) {
	repo := realGitRepo(t)
	svc := newTestService(t, repo, time.Now())
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	useLoopLayout2(t, repo)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "taskrail")

	before := snapshotTree(t, repo)
	rounds := 2
	snapshot, err := svc.LoopPreflight(LoopInvocation{MaxIterations: 1, MaxReviewRounds: &rounds, Child: []string{"agent"}})
	if err != nil {
		t.Fatalf("LoopPreflight: %v", err)
	}
	git := snapshot.Git()
	if !git.Clean || git.Branch == "" || git.Ref == "" || git.Head == "" || len(git.Index) == 0 {
		t.Fatalf("git snapshot = %#v", git)
	}
	review := snapshot.Review()
	if review.ConfiguredMaxRounds != 1 || review.EffectiveMaxRounds != 2 || review.Source != "flag" || review.MaxReviewersPerRound != 3 {
		t.Fatalf("review snapshot = %#v", review)
	}
	if got := snapshot.Inputs()["planning/STATE.md"]; len(got) == 0 {
		t.Fatalf("state was not captured: %#v", snapshot.Inputs())
	}
	if after := snapshotTree(t, repo); !reflect.DeepEqual(after, before) {
		t.Fatal("LoopPreflight changed repository bytes")
	}
}

func TestLoopConfiguredReviewRounds(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    int
		wantErr bool
	}{
		{"layout one refused", "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n", 0, true},
		{"layout two preserves configuration", "layout_version: 2\nspecs_dir: specs\nplanning_dir: planning\nstorage_mode: committed\nimplementation_review_max_rounds: 2\n", 2, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := loopConfiguredReviewRounds([]byte(test.data))
			if test.wantErr {
				if err == nil || MachineFailureFor(err).Code != MachineCodeUnsupported {
					t.Fatalf("loopConfiguredReviewRounds error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("loopConfiguredReviewRounds: %v", err)
			}
			if got != test.want {
				t.Fatalf("rounds = %d, want %d", got, test.want)
			}
		})
	}
}

func TestLoopPreflightRefusesSourceCheckoutBeforeUse(t *testing.T) {
	repo := realGitRepo(t)
	svc := newTestService(t, repo, time.Now())
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	useLoopLayout2(t, repo)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "taskrail")
	writeFile(t, repo+"/Taskfile.yml", "version: '3'\n")
	writeFile(t, repo+"/internal/toolchain/cmd/freshcheck/main.go", "package main\n")

	if _, err := svc.LoopPreflight(LoopInvocation{MaxIterations: 1, Child: []string{"agent"}}); err == nil || MachineFailureFor(err).Code != MachineCodeUnsupported {
		t.Fatalf("LoopPreflight error = %v", err)
	}
}

func TestLoopPreflightRefusesDirtyActiveAndLockedRepositories(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, repo string, svc *Service) func()
		code   string
	}{
		{
			name: "dirty", code: MachineCodeGitState,
			mutate: func(t *testing.T, repo string, _ *Service) func() {
				writeFile(t, filepath.Join(repo, "untracked.txt"), "dirty\n")
				return func() {}
			},
		},
		{
			name: "active task", code: MachineCodeValidationFailed,
			mutate: func(t *testing.T, repo string, svc *Service) func() {
				writeTask(t, repo, "T-001-active", "Active", "todo", "high", "specs/v0.1.0.md#summary", nil)
				runGit(t, repo, "add", ".")
				runGit(t, repo, "commit", "-m", "task")
				if _, err := svc.Start("T-001-active"); err != nil {
					t.Fatalf("start task: %v", err)
				}
				runGit(t, repo, "add", ".")
				runGit(t, repo, "commit", "-m", "active")
				return func() {}
			},
		},
		{
			name: "lock held", code: MachineCodeLockHeld,
			mutate: func(t *testing.T, _ string, svc *Service) func() {
				lock, err := repolock.Acquire(context.Background(), repolock.Request{
					Repository: svc.paths.LockRepository(), Command: "start",
					Capability: repolock.Capability{Commands: []string{"start"}},
				})
				if err != nil {
					t.Fatalf("acquire lock: %v", err)
				}
				return func() {
					if err := lock.Release(); err != nil {
						t.Fatalf("release lock: %v", err)
					}
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := realGitRepo(t)
			svc := newTestService(t, repo, time.Now())
			if _, err := svc.Init(InitInput{}); err != nil {
				t.Fatalf("init: %v", err)
			}
			useLoopLayout2(t, repo)
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-m", "taskrail")
			cleanup := test.mutate(t, repo, svc)
			defer cleanup()

			_, err := svc.LoopPreflight(LoopInvocation{MaxIterations: 1, Child: []string{"agent"}})
			if err == nil || MachineFailureFor(err).Code != test.code {
				t.Fatalf("LoopPreflight error = %v, want code %q", err, test.code)
			}
		})
	}
}

func TestLoopPreflightCapturesUppercaseGitRootCandidates(t *testing.T) {
	repo := realGitRepo(t)
	svc := newTestService(t, repo, time.Now())
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	useLoopLayout2(t, repo)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "taskrail")
	path := filepath.Join(repo, ".git", "EVIL_REV")
	if err := os.WriteFile(path, []byte("candidate\n"), 0o644); err != nil {
		t.Fatalf("write root candidate: %v", err)
	}

	snapshot, err := svc.LoopPreflight(LoopInvocation{MaxIterations: 1, Child: []string{"agent"}})
	if err != nil {
		t.Fatalf("LoopPreflight: %v", err)
	}
	if got := string(snapshot.RootRefs()[path]); got != "candidate\n" {
		t.Fatalf("EVIL_REV = %q", got)
	}
	if _, found := snapshot.RootRefs()[filepath.Join(repo, ".git", "COMMIT_EDITMSG")]; found {
		t.Fatal("COMMIT_EDITMSG must not be a root-ref candidate")
	}
}

func TestLoopPreflightRefusesAliasedOrLinkedInputs(t *testing.T) {
	t.Run("Git root alias", func(t *testing.T) {
		repo, svc := loopFixture(t)
		if err := os.WriteFile(filepath.Join(repo, ".git", "EVIL_REV"), []byte("one\n"), 0o644); err != nil {
			t.Fatalf("write uppercase candidate: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repo, ".git", "evil_rev"), []byte("two\n"), 0o644); err != nil {
			t.Skipf("filesystem cannot create portable alias: %v", err)
		}
		upper, upperErr := os.Stat(filepath.Join(repo, ".git", "EVIL_REV"))
		lower, lowerErr := os.Stat(filepath.Join(repo, ".git", "evil_rev"))
		if upperErr == nil && lowerErr == nil && os.SameFile(upper, lower) {
			t.Skip("case-insensitive filesystem cannot create distinct aliases")
		}
		if _, err := svc.LoopPreflight(LoopInvocation{MaxIterations: 1, Child: []string{"agent"}}); err == nil || MachineFailureFor(err).Code != MachineCodeGitState {
			t.Fatalf("LoopPreflight error = %v", err)
		}
	})

	t.Run("managed symlink", func(t *testing.T) {
		repo, svc := loopFixture(t)
		if err := os.Symlink("STATE.md", filepath.Join(repo, "planning", "state-alias.md")); err != nil {
			t.Skipf("create symlink: %v", err)
		}
		runGit(t, repo, "add", ".")
		runGit(t, repo, "commit", "-m", "symlink")
		if _, err := svc.LoopPreflight(LoopInvocation{MaxIterations: 1, Child: []string{"agent"}}); err == nil || MachineFailureFor(err).Code != MachineCodeRepositoryInvalid {
			t.Fatalf("LoopPreflight error = %v", err)
		}
	})
}

func TestLoopPreflightSnapshotAccessorsDefendFrozenBytes(t *testing.T) {
	_, svc := loopFixture(t)
	snapshot, err := svc.LoopPreflight(LoopInvocation{MaxIterations: 1, Child: []string{"agent"}})
	if err != nil {
		t.Fatalf("LoopPreflight: %v", err)
	}
	inputs := snapshot.Inputs()
	inputs["planning/STATE.md"][0] ^= 0xff
	git := snapshot.Git()
	git.Index[0] ^= 0xff
	git.Refs[git.Ref] = "changed"
	refs := snapshot.RootRefs()
	for path, data := range refs {
		data[0] ^= 0xff
		refs[path] = data
		break
	}
	configs := snapshot.GitConfig()
	for path, config := range configs {
		if !config.Present {
			continue
		}
		config.Bytes[0] ^= 0xff
		configs[path] = config
		break
	}
	if got := snapshot.Inputs()["planning/STATE.md"]; reflect.DeepEqual(got, inputs["planning/STATE.md"]) {
		t.Fatal("inputs accessor exposed frozen bytes")
	}
	if got := snapshot.Git(); reflect.DeepEqual(got.Index, git.Index) || got.Refs[got.Ref] == "changed" {
		t.Fatal("Git accessor exposed frozen bytes")
	}
	if reflect.DeepEqual(snapshot.GitConfig(), configs) {
		t.Fatal("Git configuration accessor exposed frozen bytes")
	}
}

func TestLoopPreflightProvesLocalManagedPathsIgnored(t *testing.T) {
	repo := realGitRepo(t)
	requireRecoveryDirectoryDurability(t, repo)
	svc := newTestService(t, repo, time.Now())
	if _, err := svc.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	local, err := NewService(repo)
	if err != nil {
		t.Fatalf("discover local service: %v", err)
	}
	exclude := filepath.Join(repo, ".git", "info", "exclude")
	if err := os.WriteFile(exclude, []byte(""), 0o644); err != nil {
		t.Fatalf("remove local exclusion: %v", err)
	}
	if _, err := local.LoopPreflight(LoopInvocation{MaxIterations: 1, Child: []string{"agent"}}); err == nil {
		t.Fatal("LoopPreflight accepted local paths without ignore proof")
	}
}

func TestLoopDryRunSelectsAllowedTaskAndRendersFrozenPrompt(t *testing.T) {
	repo, svc := loopFixture(t)
	writeTask(t, repo, "T-001-held", "Held", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002-ready", "Ready", "todo", "high", "specs/v0.1.0.md#summary", nil)
	setLoopPolicy(t, repo, "T-002-ready", "allow", "independent work")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "loop tasks")

	before := snapshotTree(t, repo)
	report, err := svc.LoopDryRun(LoopInvocation{DryRun: true, MaxIterations: 1})
	if err != nil {
		t.Fatalf("LoopDryRun: %v", err)
	}
	if report.Action != "run" || report.SelectedTask == nil || report.SelectedTask.TaskID != "T-002-ready" {
		t.Fatalf("report selection = %+v", report)
	}
	if report.Prompt == nil || report.Prompt.Source != "builtin" || !report.Prompt.OverrideAuthorized {
		t.Fatalf("report prompt = %+v", report.Prompt)
	}
	if got, want := report.Prompt.RenderedSHA256, promptDigest([]byte(report.Prompt.Content)); got != want {
		t.Fatalf("rendered hash = %q, want %q", got, want)
	}
	if after := snapshotTree(t, repo); !reflect.DeepEqual(after, before) {
		t.Fatal("LoopDryRun changed repository bytes")
	}
}

func TestLoopDryRunBuildsParallelFrontierWithoutMutation(t *testing.T) {
	repo, svc := loopFixture(t)
	writeTask(t, repo, "T-001-ready", "Ready", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002-ready", "Ready", "todo", "medium", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-003-waiting", "Waiting", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-001-ready"})
	setLoopPolicy(t, repo, "T-001-ready", "allow", "independent work")
	setLoopPolicy(t, repo, "T-002-ready", "allow", "independent work")
	setLoopPolicy(t, repo, "T-003-waiting", "allow", "waiting work")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "parallel loop tasks")

	before := snapshotTreeWithoutGitHousekeeping(t, repo)
	beforeGit, err := loopGitSnapshot(repo, svc.paths.GitDir)
	if err != nil {
		t.Fatalf("snapshot Git before dry run: %v", err)
	}
	beforeRootRefs, err := loopRootRefCandidates(svc.paths.GitDir, svc.paths.GitCommonDir)
	if err != nil {
		t.Fatalf("snapshot Git root refs before dry run: %v", err)
	}
	beforeConfigs, err := loopGitConfigSnapshot(svc.paths.GitDir, svc.paths.GitCommonDir)
	if err != nil {
		t.Fatalf("snapshot Git config before dry run: %v", err)
	}
	report, err := svc.LoopDryRun(LoopInvocation{DryRun: true, MaxIterations: 3, Parallel: 2})
	if err != nil {
		t.Fatalf("LoopDryRun: %v", err)
	}
	if report.Action != "run" || report.SelectedTask != nil || report.Prompt != nil || report.Parallel == nil {
		t.Fatalf("parallel report = %+v", report)
	}
	if report.Parallel.RequestedWidth != 2 || report.Parallel.EffectiveWidth != 2 || report.Parallel.Delivery != "local" || report.Parallel.Workspace.CloneDepth == nil || *report.Parallel.Workspace.CloneDepth != 1 || len(report.Parallel.Frontier) != 2 {
		t.Fatalf("parallel plan = %+v", report.Parallel)
	}
	if report.Parallel.Frontier[0].Task.TaskID != "T-001-ready" || report.Parallel.Frontier[1].Task.TaskID != "T-002-ready" {
		t.Fatalf("frontier = %+v", report.Parallel.Frontier)
	}
	if after := snapshotTreeWithoutGitHousekeeping(t, repo); !reflect.DeepEqual(after, before) {
		t.Fatalf("parallel dry run changed repository bytes: %v", changedPaths(t, before, after))
	}
	afterGit, err := loopGitSnapshot(repo, svc.paths.GitDir)
	if err != nil {
		t.Fatalf("snapshot Git after dry run: %v", err)
	}
	if !reflect.DeepEqual(afterGit, beforeGit) {
		t.Fatalf("parallel dry run changed semantic Git state: before=%+v after=%+v", beforeGit, afterGit)
	}
	afterRootRefs, err := loopRootRefCandidates(svc.paths.GitDir, svc.paths.GitCommonDir)
	if err != nil {
		t.Fatalf("snapshot Git root refs after dry run: %v", err)
	}
	if !reflect.DeepEqual(afterRootRefs, beforeRootRefs) {
		t.Fatalf("parallel dry run changed Git root refs: before=%+v after=%+v", beforeRootRefs, afterRootRefs)
	}
	afterConfigs, err := loopGitConfigSnapshot(svc.paths.GitDir, svc.paths.GitCommonDir)
	if err != nil {
		t.Fatalf("snapshot Git config after dry run: %v", err)
	}
	if !reflect.DeepEqual(afterConfigs, beforeConfigs) {
		t.Fatalf("parallel dry run changed Git configuration: before=%+v after=%+v", beforeConfigs, afterConfigs)
	}
	runGit(t, repo, "fsck", "--no-dangling")
}

func snapshotTreeWithoutGitHousekeeping(t *testing.T, repo string) map[string]string {
	t.Helper()
	tree := snapshotTree(t, repo)
	// Read-only Git commands may repack objects or refresh info/refs; the test
	// separately compares semantic Git state and verifies object integrity.
	objects := filepath.Join(".git", "objects") + string(filepath.Separator)
	infoRefs := filepath.Join(".git", "info", "refs")
	for name := range tree {
		if strings.HasPrefix(name, objects) || name == infoRefs {
			delete(tree, name)
		}
	}
	return tree
}

func TestLoopDryRunRejectsWorkspaceInsideRepository(t *testing.T) {
	repo, svc := loopFixture(t)
	writeTask(t, repo, "T-001-ready", "Ready", "todo", "high", "specs/v0.1.0.md#summary", nil)
	setLoopPolicy(t, repo, "T-001-ready", "allow", "independent work")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "parallel loop task")

	_, err := svc.LoopDryRun(LoopInvocation{DryRun: true, MaxIterations: 2, Parallel: 2, WorkspaceRoot: repo, WorkspaceRootSet: true})
	if err == nil || MachineFailureFor(err).Code != MachineCodeInvalidArguments {
		t.Fatalf("LoopDryRun error = %v", err)
	}
}

func TestLoopDryRunRequiresExactReplacementTemplateAuthorization(t *testing.T) {
	repo, svc := loopFixture(t)
	writeTask(t, repo, "T-001-ready", "Ready", "todo", "high", "specs/v0.1.0.md#summary", nil)
	setLoopPolicy(t, repo, "T-001-ready", "allow", "independent work")
	template := []byte("Implement {{TASK_ID}} in {{TASK_PATH}} with {{IMPLEMENTATION_REVIEW_MAX_ROUNDS}} review round(s).\n")
	writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-implementation.md"), string(template))
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "replacement prompt")

	unauthorized, err := svc.LoopDryRun(LoopInvocation{DryRun: true, MaxIterations: 1})
	if err != nil {
		t.Fatalf("unauthorized dry run: %v", err)
	}
	if unauthorized.Action != "invalid" || unauthorized.Prompt != nil {
		t.Fatalf("unauthorized report = %+v", unauthorized)
	}
	mismatched, err := svc.LoopDryRun(LoopInvocation{DryRun: true, MaxIterations: 1, AllowPromptOverrideSHA256: strings.Repeat("0", 64)})
	if err != nil {
		t.Fatalf("mismatched dry run: %v", err)
	}
	if mismatched.Action != "invalid" || mismatched.Prompt != nil {
		t.Fatalf("mismatched report = %+v", mismatched)
	}

	authorized, err := svc.LoopDryRun(LoopInvocation{DryRun: true, MaxIterations: 1, AllowPromptOverrideSHA256: promptDigest(template)})
	if err != nil {
		t.Fatalf("authorized dry run: %v", err)
	}
	if authorized.Action != "run" || authorized.Prompt == nil || authorized.Prompt.Source != "replacement" || !authorized.Prompt.OverrideAuthorized {
		t.Fatalf("authorized report = %+v", authorized)
	}
	if got, want := authorized.Prompt.TemplateSHA256, promptDigest(template); got != want {
		t.Fatalf("template hash = %q, want %q", got, want)
	}
}

func TestLoopDryRunChecksReplacementAuthorizationWithoutSelectedWork(t *testing.T) {
	repo, svc := loopFixture(t)
	template := []byte("Implement {{TASK_ID}} in {{TASK_PATH}} with {{IMPLEMENTATION_REVIEW_MAX_ROUNDS}} review round(s).\n")
	writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-implementation.md"), string(template))
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "replacement prompt")

	report, err := svc.LoopDryRun(LoopInvocation{DryRun: true, MaxIterations: 1})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if report.Action != "invalid" || report.SelectedTask != nil || report.Prompt != nil {
		t.Fatalf("report = %+v", report)
	}
}

func TestLoopFrozenSelectionUsesPreflightTaskBytes(t *testing.T) {
	repo, svc := loopFixture(t)
	writeTask(t, repo, "T-001-ready", "Ready", "todo", "high", "specs/v0.1.0.md#summary", nil)
	setLoopPolicy(t, repo, "T-001-ready", "allow", "independent work")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "allowed task")
	snapshot, err := svc.LoopPreflight(LoopInvocation{DryRun: true, MaxIterations: 1})
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	setLoopPolicy(t, repo, "T-001-ready", "hold", "changed after preflight")

	selection, err := svc.loopFrozenSelection(snapshot)
	if err != nil {
		t.Fatalf("frozen selection: %v", err)
	}
	if selection.Action != "run" || selection.SelectedTask == nil || selection.SelectedTask.TaskID != "T-001-ready" {
		t.Fatalf("selection = %+v", selection)
	}
}

func TestLoopDryRunReportsNoneWithoutPromptAndRejectsStaleBuiltinAuthorization(t *testing.T) {
	_, svc := loopFixture(t)

	none, err := svc.LoopDryRun(LoopInvocation{DryRun: true, MaxIterations: 1})
	if err != nil {
		t.Fatalf("none dry run: %v", err)
	}
	if none.Action != "none" || none.SelectedTask != nil || none.Prompt != nil {
		t.Fatalf("none report = %+v", none)
	}

	if _, err := svc.LoopDryRun(LoopInvocation{DryRun: true, MaxIterations: 1, AllowPromptOverrideSHA256: strings.Repeat("0", 64)}); MachineFailureFor(err).Code != MachineCodeInvalidArguments {
		t.Fatalf("stale authorization error = %v", err)
	}
}

func loopFixture(t *testing.T) (string, *Service) {
	t.Helper()
	repo := realGitRepo(t)
	svc := newTestService(t, repo, time.Now())
	if _, err := svc.Init(InitInput{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	useLoopLayout2(t, repo)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "taskrail")
	return repo, svc
}

func useLoopLayout2(t *testing.T, repo string) {
	t.Helper()
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: 2\nspecs_dir: specs\nplanning_dir: planning\nstorage_mode: committed\nimplementation_review_max_rounds: 1\n")
}
