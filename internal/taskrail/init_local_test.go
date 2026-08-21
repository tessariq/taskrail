package taskrail

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
	localTasks := filepath.Join(repo, ".taskrail", "local", "planning", "tasks")
	if info, err := os.Stat(localTasks); err != nil || !info.IsDir() {
		t.Fatalf("local tasks directory: info=%v err=%v", info, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "planning", "tasks")); !os.IsNotExist(err) {
		t.Fatalf("committed tasks directory unexpectedly exists: %v", err)
	}
	local, err := NewService(repo)
	if err != nil {
		t.Fatalf("rediscover local service: %v", err)
	}
	if _, err := local.AddSpec("v0.2.0"); err != nil {
		t.Fatalf("add spec after local init: %v", err)
	}
	if _, err := local.CreateTask(CreateTaskInput{Title: "First local task", SpecRef: "specs/v0.1.0.md#summary"}); err != nil {
		t.Fatalf("create task after local init: %v", err)
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

func TestInitLocalRemovesEmptyTaskDirectoryOnPrepublicationFailure(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	testHookLocalTasksCreated = func() error { return errors.New("injected after local tasks creation") }
	t.Cleanup(func() { testHookLocalTasksCreated = nil })

	_, err := newTestService(t, repo, time.Now()).Init(InitInput{Local: true})
	if err == nil || !strings.Contains(err.Error(), "injected after local tasks creation") {
		t.Fatalf("init local error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".taskrail")); !os.IsNotExist(err) {
		t.Fatalf("failed local init retained scaffold: %v", err)
	}
	if output := gitOutput(t, repo, "status", "--porcelain"); output != "" {
		t.Fatalf("failed local init changed Git status: %q", output)
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

func TestLocalStatusAndPathReportOneReadOnlyLocalContext(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	if output, err := exec.Command("git", "init", "--quiet", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, output)
	}
	requireRecoveryDirectoryDurability(t, repo)
	if _, err := newTestService(t, repo, time.Now()).Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("new local service: %v", err)
	}
	before := snapshotTree(t, repo)

	status, err := svc.LocalStatus()
	if err != nil {
		t.Fatalf("local status: %v", err)
	}
	paths, err := svc.LocalPath()
	if err != nil {
		t.Fatalf("local path: %v", err)
	}
	if status.Mode != string(StorageLocal) || status.StorageRoot != localStorageRoot ||
		status.LogicalRoot != repo || status.WorktreeRoot != repo || status.GitCommonDir == "" {
		t.Fatalf("local status identity = %+v", status)
	}
	if !status.PromotionReady || len(status.Violations) != 0 {
		t.Fatalf("local status readiness = %+v", status)
	}
	storage := svc.storageSnapshot()
	if paths.Mode != string(StorageLocal) || paths.ConfigPath != ".taskrail/config.yml" ||
		paths.StorageRoot != localStorageRoot || paths.SpecsDir != "specs" ||
		paths.PlanningDir != "planning" || paths.PromptsDir != ".taskrail/local/prompts" ||
		paths.ArtifactsDir != storage.ArtifactsDir || paths.RuntimeDir != ".taskrail/local/runtime" {
		t.Fatalf("local paths = %+v; status storage = %+v", paths, storage)
	}
	if got := snapshotTree(t, repo); !reflect.DeepEqual(got, before) {
		t.Fatal("local inspection changed fixture bytes")
	}
}

func TestLocalInspectionRefusesMarkerChangedAfterDiscovery(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	if output, err := exec.Command("git", "init", "--quiet", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, output)
	}
	requireRecoveryDirectoryDurability(t, repo)
	if _, err := newTestService(t, repo, time.Now()).Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("new local service: %v", err)
	}
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: 2\nspecs_dir: specs\nplanning_dir: planning\nstorage_mode: committed\nimplementation_review_max_rounds: 1\n")

	if _, err := svc.LocalStatus(); err == nil || MachineFailureFor(err).Code != MachineCodeUnsupported {
		t.Fatalf("local status error = %v, want unsupported", err)
	}
	if _, err := svc.LocalPath(); err == nil || MachineFailureFor(err).Code != MachineCodeUnsupported {
		t.Fatalf("local path error = %v, want unsupported", err)
	}
}

func TestLocalStatusIncludesInstalledSkillExclusions(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	if output, err := exec.Command("git", "init", "--quiet", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, output)
	}
	requireRecoveryDirectoryDurability(t, repo)
	if _, err := newTestService(t, repo, time.Now()).Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("new local service: %v", err)
	}
	installed, err := svc.WriteShippableSkills("v9.9.9", false)
	if err != nil {
		t.Fatalf("install local skills: %v", err)
	}
	skillPath := filepath.ToSlash(filepath.Dir(installed.Written[0]))
	excludePath := filepath.Join(repo, ".git", "info", "exclude")
	exclude, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, excludePath, strings.Replace(string(exclude), localExcludeEnd, skillPath+"\n"+localExcludeEnd, 1))

	svc, err = NewService(repo)
	if err != nil {
		t.Fatalf("new local service: %v", err)
	}
	status, err := svc.LocalStatus()
	if err != nil {
		t.Fatalf("local status: %v", err)
	}
	for _, exclusion := range status.Exclusions {
		if exclusion.Path == skillPath && exclusion.Source == "managed" && exclusion.Effective {
			return
		}
	}
	t.Fatalf("local status exclusions = %+v, want installed skill %s", status.Exclusions, skillPath)
}

func TestLocalInspectionRefusesUninitializedAndMalformedOrigins(t *testing.T) {
	t.Parallel()

	uninitialized := t.TempDir()
	initLocalGitRepo(t, uninitialized)
	svc, err := NewService(uninitialized)
	if err != nil {
		t.Fatalf("new uninitialized service: %v", err)
	}
	if _, err := svc.LocalStatus(); err == nil || MachineFailureFor(err).Code != MachineCodeUnsupported {
		t.Fatalf("uninitialized local status error = %v, want unsupported", err)
	}
	if _, err := svc.LocalPath(); err == nil || MachineFailureFor(err).Code != MachineCodeUnsupported {
		t.Fatalf("uninitialized local path error = %v, want unsupported", err)
	}
	if _, err := os.Stat(filepath.Join(uninitialized, ".taskrail")); !os.IsNotExist(err) {
		t.Fatalf("uninitialized inspection created local storage: %v", err)
	}

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	if _, err := newTestService(t, repo, time.Now()).Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	svc, err = NewService(repo)
	if err != nil {
		t.Fatalf("new local service: %v", err)
	}
	writeFile(t, filepath.Join(repo, ".taskrail", "local", "runtime", "origin.json"), "{}")
	if _, err := svc.LocalStatus(); err == nil || MachineFailureFor(err).Code != MachineCodeRepositoryInvalid {
		t.Fatalf("malformed origin error = %v, want repository_invalid", err)
	}
}

func TestLocalStatusDiscoversDescendantAndLinkedWorktreeScopes(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	if _, err := newTestService(t, repo, time.Now()).Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	descendant := filepath.Join(repo, "nested", "directory")
	if err := os.MkdirAll(descendant, 0o755); err != nil {
		t.Fatalf("create descendant: %v", err)
	}
	svc, err := NewService(descendant)
	if err != nil {
		t.Fatalf("new descendant service: %v", err)
	}
	status, err := svc.LocalStatus()
	if err != nil {
		t.Fatalf("descendant local status: %v", err)
	}
	if status.LogicalRoot != repo || status.WorktreeRoot != repo || status.GitCommonDir != filepath.Join(repo, ".git") {
		t.Fatalf("descendant scope = %+v", status)
	}

	linkedSource := t.TempDir()
	initLocalGitRepo(t, linkedSource)
	writeFile(t, filepath.Join(linkedSource, "README.md"), "# source\n")
	runLocalGit(t, linkedSource, "add", "README.md")
	runLocalGit(t, linkedSource, "-c", "user.name=Taskrail", "-c", "user.email=taskrail@example.test", "commit", "-qm", "initial")
	linked := t.TempDir()
	if err := os.Remove(linked); err != nil {
		t.Fatalf("remove linked-worktree target: %v", err)
	}
	runLocalGit(t, linkedSource, "worktree", "add", "--detach", linked)
	requireRecoveryDirectoryDurability(t, linked)
	linkedSvc, err := NewService(linked)
	if err != nil {
		t.Fatalf("new linked service: %v", err)
	}
	if _, err := linkedSvc.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init linked local storage: %v", err)
	}
	linkedSvc, err = NewService(linked)
	if err != nil {
		t.Fatalf("rediscover linked service: %v", err)
	}
	linkedStatus, err := linkedSvc.LocalStatus()
	if err != nil {
		t.Fatalf("linked local status: %v", err)
	}
	expectedCommonDir, err := filepath.EvalSymlinks(filepath.Join(linkedSource, ".git"))
	if err != nil {
		t.Fatalf("canonicalize linked common directory: %v", err)
	}
	actualCommonDir, err := filepath.EvalSymlinks(linkedStatus.GitCommonDir)
	if err != nil {
		t.Fatalf("canonicalize reported common directory: %v", err)
	}
	if linkedStatus.WorktreeRoot != linked || actualCommonDir != expectedCommonDir {
		t.Fatalf("linked scope = %+v", linkedStatus)
	}
}

func initLocalGitRepo(t *testing.T, repo string) {
	t.Helper()
	if output, err := exec.Command("git", "init", "--quiet", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, output)
	}
}

func runLocalGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	if output, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, output)
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
