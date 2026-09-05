package taskrail

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tessariq/taskrail/internal/durabletx"
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

func TestInitLocalWithSkillsPublishesSkillsAndExclusions(t *testing.T) {
	skipDurableSkillPublication(t)

	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	svc := newTestService(t, repo, time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))

	result, err := svc.Init(InitInput{Local: true, WithSkills: true, SkillVersion: "v9.9.9"})
	if err != nil {
		t.Fatalf("init local with skills: %v", err)
	}
	files, err := packagedSkillFiles()
	if err != nil {
		t.Fatalf("packaged skill files: %v", err)
	}
	assertSkillInventory(t, result.Skills, len(files)*len(shippableSkillTargets), writeActionCreate)
	names, err := packagedSkillNames()
	if err != nil {
		t.Fatalf("packaged skill names: %v", err)
	}
	if got, want := len(result.SkillExclusions), len(names)*len(shippableSkillTargets); got != want {
		t.Fatalf("skill exclusions = %d, want %d", got, want)
	}
	for _, skill := range result.Skills {
		data, readErr := os.ReadFile(filepath.Join(repo, filepath.FromSlash(skill.Path)))
		if readErr != nil {
			t.Fatalf("read installed skill %s: %v", skill.Path, readErr)
		}
		version, versionErr := skillVersionOf(data)
		if versionErr != nil || version != "v9.9.9" {
			t.Fatalf("installed skill %s version = %q, %v", skill.Path, version, versionErr)
		}
	}
	exclude, err := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("read exclusion: %v", err)
	}
	managed, _ := splitLocalExclusions(string(exclude))
	for _, exclusion := range result.SkillExclusions {
		if exclusion.Action != writeActionCreate || !managed[exclusion.Path] {
			t.Fatalf("skill exclusion = %+v, managed = %v", exclusion, managed[exclusion.Path])
		}
	}
	if output := gitOutput(t, repo, "status", "--porcelain"); output != "" {
		t.Fatalf("git status = %q, want clean", output)
	}
}

func TestInitLocalSkillsRefreshPreservesParityAndNormalizesLegacyMarker(t *testing.T) {
	skipDurableSkillPublication(t)

	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	svc := newTestService(t, repo, time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))

	installed, err := svc.Init(InitInput{Local: true, WithSkills: true, SkillVersion: "v9.9.8"})
	if err != nil {
		t.Fatalf("init local with skills: %v", err)
	}
	svc, err = NewService(repo)
	if err != nil {
		t.Fatalf("rediscover local service: %v", err)
	}
	parityPath := installed.Skills[0].Path
	parityPackagePath := strings.TrimPrefix(parityPath, ".agents/skills/")
	if strings.HasPrefix(parityPackagePath, ".claude/") {
		parityPackagePath = strings.TrimPrefix(parityPath, ".claude/skills/")
	}
	parity, err := shippableSkillsFS.ReadFile(path.Join(shippableSkillsRoot, parityPackagePath))
	if err != nil {
		t.Fatalf("read parity package: %v", err)
	}
	writeFile(t, filepath.Join(repo, filepath.FromSlash(parityPath)), string(parity))

	legacyPath := installed.Skills[1].Path
	legacy, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(legacyPath)))
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	legacy = []byte(strings.Replace(string(legacy), "metadata:\n    taskrail_version: v9.9.8", "taskrail_version: v9.9.8", 1))
	writeFile(t, filepath.Join(repo, filepath.FromSlash(legacyPath)), string(legacy))

	refreshed, err := svc.Init(InitInput{WithSkills: true, ForceSkills: true, SkillVersion: "v9.9.9"})
	if err != nil {
		t.Fatalf("refresh local skills: %v", err)
	}
	for _, skill := range refreshed.Skills {
		switch skill.Path {
		case parityPath:
			if skill.Action != writeActionPreserve {
				t.Fatalf("parity skill = %+v, want preserve", skill)
			}
		case legacyPath:
			if skill.Action != writeActionRefresh {
				t.Fatalf("legacy skill = %+v, want refresh", skill)
			}
		}
	}
	gotParity, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(parityPath)))
	if err != nil {
		t.Fatalf("read preserved parity skill: %v", err)
	}
	if string(gotParity) != string(parity) {
		t.Fatal("refresh stamped or changed a marker-free parity mirror")
	}
	gotLegacy, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(legacyPath)))
	if err != nil {
		t.Fatalf("read refreshed legacy skill: %v", err)
	}
	if strings.Contains(string(gotLegacy), "\ntaskrail_version:") || !strings.Contains(string(gotLegacy), "metadata:\n    taskrail_version: v9.9.9") {
		t.Fatalf("refresh did not normalize legacy marker:\n%s", gotLegacy)
	}
	for _, exclusion := range refreshed.SkillExclusions {
		if exclusion.Action != writeActionPreserve {
			t.Fatalf("refresh changed managed exclusion %+v", exclusion)
		}
	}
	if output := gitOutput(t, repo, "status", "--porcelain"); output != "" {
		t.Fatalf("refresh exposed local skills to Git: %q", output)
	}
}

func TestInitLocalSkillsRefreshRefusesDivergentDestination(t *testing.T) {
	skipDurableSkillPublication(t)

	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	svc := newTestService(t, repo, time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))
	installed, err := svc.Init(InitInput{Local: true, WithSkills: true, SkillVersion: "v9.9.8"})
	if err != nil {
		t.Fatalf("init local with skills: %v", err)
	}
	svc, err = NewService(repo)
	if err != nil {
		t.Fatalf("rediscover local service: %v", err)
	}
	diverged := filepath.Join(repo, filepath.FromSlash(installed.Skills[0].Path))
	writeFile(t, diverged, "---\nname: altered\ndescription: adopter owned\n---\n")
	before := snapshotTree(t, repo)

	if _, err := svc.Init(InitInput{WithSkills: true, ForceSkills: true, SkillVersion: "v9.9.9"}); err == nil {
		t.Fatal("refresh accepted divergent local skill bytes")
	}
	if after := snapshotTree(t, repo); !reflect.DeepEqual(after, before) {
		t.Fatal("refused refresh changed local skill or exclusion bytes")
	}
}

func TestInitLocalSkillsRefreshRequiresForce(t *testing.T) {
	skipDurableSkillPublication(t)

	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))
	installed, err := setup.Init(InitInput{Local: true, WithSkills: true, SkillVersion: "v9.9.8"})
	if err != nil {
		t.Fatalf("init local with skills: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("rediscover local service: %v", err)
	}
	changed := filepath.Join(repo, filepath.FromSlash(installed.Skills[0].Path))
	data, err := os.ReadFile(changed)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	writeFile(t, changed, strings.Replace(string(data), "v9.9.8", "v0.0.1", 1))
	before := snapshotTree(t, repo)

	if _, err := svc.Init(InitInput{WithSkills: true, SkillVersion: "v9.9.9"}); err == nil {
		t.Fatal("local skill refresh without --force succeeded")
	}
	if after := snapshotTree(t, repo); !reflect.DeepEqual(after, before) {
		t.Fatal("local skill refresh without --force changed bytes")
	}
}

func TestInitLocalSkillsRefreshPreservesCurrentBytes(t *testing.T) {
	skipDurableSkillPublication(t)

	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))
	if _, err := setup.Init(InitInput{Local: true, WithSkills: true, SkillVersion: "v9.9.9"}); err != nil {
		t.Fatalf("init local with skills: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("rediscover local service: %v", err)
	}

	refreshed, err := svc.Init(InitInput{WithSkills: true, ForceSkills: true, SkillVersion: "v9.9.9"})
	if err != nil {
		t.Fatalf("refresh current local skills: %v", err)
	}
	for _, skill := range refreshed.Skills {
		if skill.Action != writeActionPreserve {
			t.Fatalf("current skill = %+v, want preserve", skill)
		}
	}
}

func TestInitLocalWithSkillsRejectsPlanDriftWithoutScaffold(t *testing.T) {
	skipDurableSkillPublication(t)

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	files, err := packagedSkillFiles()
	if err != nil {
		t.Fatalf("packaged skill files: %v", err)
	}
	drift := filepath.Join(repo, ".agents", "skills", filepath.Dir(files[0]), "adopter.md")
	testHookLocalSkillsPlanned = func() error {
		if err := os.MkdirAll(filepath.Dir(drift), 0o755); err != nil {
			return err
		}
		return os.WriteFile(drift, []byte("keep me\n"), 0o644)
	}
	t.Cleanup(func() { testHookLocalSkillsPlanned = nil })

	_, err = newTestService(t, repo, time.Now()).Init(InitInput{Local: true, WithSkills: true, SkillVersion: "v9.9.9"})
	if err == nil || !strings.Contains(err.Error(), "adopter-owned content") {
		t.Fatalf("init local with post-plan collision error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(repo, ".taskrail")); !os.IsNotExist(statErr) {
		t.Fatalf("post-plan collision retained local scaffold: %v", statErr)
	}
	if data, readErr := os.ReadFile(drift); readErr != nil || string(data) != "keep me\n" {
		t.Fatalf("post-plan collision did not preserve adopter content: %q, %v", data, readErr)
	}
	if output := gitOutput(t, repo, "status", "--porcelain"); output != "?? .agents/" {
		t.Fatalf("post-plan collision status = %q, want only adopter content", output)
	}
}

func TestInitLocalSkillRecoveryRejectsAdopterContent(t *testing.T) {
	skipDurableSkillPublication(t)

	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Now())
	result, err := setup.Init(InitInput{Local: true, WithSkills: true, SkillVersion: "v9.9.9"})
	if err != nil {
		t.Fatalf("init local with skills: %v", err)
	}
	marker, err := os.ReadFile(filepath.Join(repo, ".taskrail", "config.yml"))
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	adopterPath := filepath.Join(repo, filepath.Dir(result.Skills[0].Path), "adopter.md")
	writeFile(t, adopterPath, "keep me\n")
	local, err := NewService(repo)
	if err != nil {
		t.Fatalf("new local service: %v", err)
	}

	err = local.validateInitRecovery("0123456789abcdef0123456789abcdef", []durabletx.Evidence{
		{Kind: durabletx.Worktree, Reported: markerRelPath(), CandidateSHA256: digestBytes(marker)},
		{Kind: durabletx.Worktree, Reported: result.Skills[0].Path, CandidateSHA256: "candidate"},
	})
	if err == nil || !strings.Contains(err.Error(), "adopter-owned content") {
		t.Fatalf("local skill recovery error = %v", err)
	}
}

func TestCreateTaskInLocalStorageReportsLogicalPath(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	if _, err := setup.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}

	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("rediscover local service: %v", err)
	}
	result, err := svc.CreateTask(CreateTaskInput{
		Title:   "Logical result path",
		SpecRef: "specs/v0.1.0.md#summary",
	})
	if err != nil {
		t.Fatalf("create local task: %v", err)
	}
	if want := "planning/tasks/" + result.TaskID + ".md"; result.Path != want {
		t.Fatalf("result path = %q, want logical path %q", result.Path, want)
	}
	if _, err := os.Stat(filepath.Join(repo, ".taskrail", "local", filepath.FromSlash(result.Path))); err != nil {
		t.Fatalf("local task was not published beneath the overlay: %v", err)
	}
}

func TestLifecycleAndTaskWritersUseOnlyLocalStorage(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	if _, err := setup.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}

	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("rediscover local service: %v", err)
	}
	svc.now = func() time.Time { return time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC) }
	specPath := filepath.Join(svc.paths.SpecsDir, "v0.1.0.md")
	writeFile(t, specPath, readFileString(t, specPath)+"\n## Goals\n")
	first, err := svc.CreateTask(CreateTaskInput{Title: "First", SpecRef: "specs/v0.1.0.md#summary"})
	if err != nil {
		t.Fatalf("create first local task: %v", err)
	}
	second, err := svc.CreateTask(CreateTaskInput{Title: "Second", SpecRef: "specs/v0.1.0.md#goals"})
	if err != nil {
		t.Fatalf("create second local task: %v", err)
	}

	statusBefore := gitOutput(t, repo, "status", "--porcelain")

	if result, err := svc.Next(); err != nil || result.TaskID != first.TaskID {
		t.Fatalf("next = %+v, %v", result, err)
	}
	if _, err := svc.Start(first.TaskID); err != nil {
		t.Fatalf("start local task: %v", err)
	}
	if _, err := svc.Block(first.TaskID, "exercise local block"); err != nil {
		t.Fatalf("block local task: %v", err)
	}
	if _, err := svc.Unblock(first.TaskID, "resume local work"); err != nil {
		t.Fatalf("unblock local task: %v", err)
	}
	if _, err := svc.Start(first.TaskID); err != nil {
		t.Fatalf("restart local task: %v", err)
	}
	if _, err := svc.Complete(first.TaskID, "local implementation complete"); err != nil {
		t.Fatalf("complete local task: %v", err)
	}
	verified, err := svc.Verify(VerifyInput{
		TaskID: first.TaskID, Result: "pass", Summary: "local workflow verified",
		CreateFollowup: true, FollowupTitle: "Local follow-up",
	})
	if err != nil {
		t.Fatalf("verify local task with follow-up: %v", err)
	}
	if verified.FollowupTaskID == "" {
		t.Fatal("verify did not create a local follow-up")
	}
	if _, err := svc.RepointTask(RepointTaskInput{TaskID: second.TaskID, Area: "summary"}); err != nil {
		t.Fatalf("repoint local task: %v", err)
	}
	if _, err := svc.EditDependency(EditDependencyInput{
		TaskID: second.TaskID, DependencyID: first.TaskID, Operation: DependencyAdd,
	}); err != nil {
		t.Fatalf("edit local dependency: %v", err)
	}
	if _, err := svc.RenameTask(RenameTaskInput{OldID: second.TaskID, Slug: "renamed", SlugExplicit: true}); err != nil {
		t.Fatalf("rename local task: %v", err)
	}

	if got := gitOutput(t, repo, "status", "--porcelain"); got != statusBefore {
		t.Fatalf("local writers changed ordinary Git status:\nbefore: %q\nafter:  %q", statusBefore, got)
	}
	if got := gitOutput(t, repo, "diff", "--cached", "--name-only"); got != "" {
		t.Fatalf("local writers staged ignored files:\n%s", got)
	}
	if validation, err := svc.Validate(); err != nil || !validation.Valid {
		t.Fatalf("validate local writer result: validation=%+v err=%v", validation, err)
	}
}

func TestLocalWritersRefuseCommittedStateAddedAfterDiscovery(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	if _, err := setup.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("rediscover local service: %v", err)
	}
	decoy := filepath.Join(repo, "planning", "STATE.md")
	writeFile(t, decoy, "committed state decoy\n")
	before := snapshotTree(t, repo)

	if _, err := svc.Next(); err == nil || MachineFailureFor(err).Code != MachineCodeRepositoryInvalid {
		t.Fatalf("next with mixed local/committed state = %v, want repository_invalid", err)
	}
	if got := snapshotTree(t, repo); !reflect.DeepEqual(got, before) {
		t.Fatal("mixed-state refusal changed local or committed bytes")
	}
}

func TestLocalWriterTransactionsRejectMixedStateBeforePublication(t *testing.T) {
	for _, command := range []string{"next", "task new", "verify follow-up"} {
		t.Run(command, func(t *testing.T) {
			repo := t.TempDir()
			initLocalGitRepo(t, repo)
			requireRecoveryDirectoryDurability(t, repo)
			setup := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
			if _, err := setup.Init(InitInput{Local: true}); err != nil {
				t.Fatalf("init local: %v", err)
			}
			svc, err := NewService(repo)
			if err != nil {
				t.Fatalf("rediscover local service: %v", err)
			}
			var taskID string
			if command != "task new" {
				created, err := svc.CreateTask(CreateTaskInput{Title: "Source", SpecRef: "specs/v0.1.0.md#summary"})
				if err != nil {
					t.Fatalf("create local source task: %v", err)
				}
				taskID = created.TaskID
			}
			before := snapshotTree(t, filepath.Join(repo, ".taskrail", "local"))
			installLifecycleHook(t, func() {
				writeFile(t, filepath.Join(repo, "planning", "STATE.md"), "committed state decoy\n")
			})

			switch command {
			case "next":
				_, err = svc.Next()
			case "task new":
				_, err = svc.CreateTask(CreateTaskInput{Title: "Blocked", SpecRef: "specs/v0.1.0.md#summary"})
			case "verify follow-up":
				_, err = svc.Verify(VerifyInput{TaskID: taskID, Result: "fail", Summary: "mixed state", CreateFollowup: true, FollowupTitle: "Blocked"})
			}
			if err == nil || MachineFailureFor(err).Code != MachineCodeValidationFailed {
				t.Fatalf("%s with mixed state during validation = %v, want validation_failed", command, err)
			}
			if got := snapshotTree(t, filepath.Join(repo, ".taskrail", "local")); !reflect.DeepEqual(got, before) {
				t.Fatalf("%s published local bytes after mixed-state detection", command)
			}
		})
	}
}

func localWriterFixture(t *testing.T) (*Service, string) {
	t.Helper()
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	if _, err := setup.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("rediscover local service: %v", err)
	}
	svc.now = func() time.Time { return time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC) }
	writeFile(t, filepath.Join(svc.paths.SpecsDir, "v0.1.0.md"), "# Taskrail v0.1.0\n\n## Summary\n\n## Details\n")
	return svc, repo
}

func TestLocalLifecycleAndVerificationRollbackStayInOverlay(t *testing.T) {
	requirePermissionFaultInjection(t)
	for _, command := range []string{"start", "verify follow-up"} {
		t.Run(command, func(t *testing.T) {
			svc, repo := localWriterFixture(t)
			created, err := svc.CreateTask(CreateTaskInput{Title: "Source", SpecRef: "specs/v0.1.0.md#summary"})
			if err != nil {
				t.Fatalf("create local source task: %v", err)
			}
			if command == "verify follow-up" {
				if _, err := svc.Start(created.TaskID); err != nil {
					t.Fatalf("start local source task: %v", err)
				}
				if _, err := svc.Complete(created.TaskID, "complete before verify"); err != nil {
					t.Fatalf("complete local source task: %v", err)
				}
			}
			stateBefore := readBytes(t, svc.paths.StateFile)
			taskPath := filepath.Join(svc.paths.TasksDir, created.TaskID+".md")
			taskBefore := readBytes(t, taskPath)
			installLifecycleHook(t, func() {
				blocked := svc.paths.TasksDir
				if command == "verify follow-up" {
					blocked = svc.paths.ArtifactsDir
					if err := os.MkdirAll(blocked, 0o755); err != nil {
						t.Fatalf("create local artifacts directory: %v", err)
					}
				}
				if err := os.Chmod(blocked, 0o500); err != nil {
					t.Fatalf("block local publication: %v", err)
				}
				t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })
			})
			if command == "start" {
				_, err = svc.Start(created.TaskID)
			} else {
				_, err = svc.Verify(VerifyInput{TaskID: created.TaskID, Result: "pass", Summary: "rollback", CreateFollowup: true, FollowupTitle: "Follow-up"})
			}
			if err == nil || MachineFailureFor(err).Code != MachineCodePartialWrite {
				t.Fatalf("%s with blocked local publication = %v, want partial_write", command, err)
			}
			if got := readBytes(t, svc.paths.StateFile); got != stateBefore {
				t.Fatalf("%s left local state rolled forward", command)
			}
			if got := readBytes(t, taskPath); got != taskBefore {
				t.Fatalf("%s left local task rolled forward", command)
			}
			if got := gitOutput(t, repo, "status", "--porcelain"); got != "" {
				t.Fatalf("%s exposed ignored local state to Git: %q", command, got)
			}
		})
	}
}

func TestLocalTaskMutationRollbackStaysInOverlay(t *testing.T) {
	requirePermissionFaultInjection(t)
	for _, command := range []string{"new", "rename", "repoint", "dependency"} {
		t.Run(command, func(t *testing.T) {
			svc, repo := localWriterFixture(t)
			base, err := svc.CreateTask(CreateTaskInput{Title: "Base", SpecRef: "specs/v0.1.0.md#summary"})
			if err != nil {
				t.Fatalf("create local base task: %v", err)
			}
			if _, err := svc.Start(base.TaskID); err != nil {
				t.Fatalf("start local base task: %v", err)
			}
			if _, err := svc.Complete(base.TaskID, "complete before mutation"); err != nil {
				t.Fatalf("complete local base task: %v", err)
			}
			work, err := svc.CreateTask(CreateTaskInput{Title: "Work", SpecRef: "specs/v0.1.0.md#summary"})
			if err != nil {
				t.Fatalf("create local work task: %v", err)
			}
			extra, err := svc.CreateTask(CreateTaskInput{Title: "Extra", SpecRef: "specs/v0.1.0.md#summary"})
			if err != nil {
				t.Fatalf("create local extra task: %v", err)
			}
			stateBefore := readBytes(t, svc.paths.StateFile)
			installLifecycleHook(t, func() {
				if err := os.Chmod(svc.paths.TasksDir, 0o500); err != nil {
					t.Fatalf("block local task publication: %v", err)
				}
				t.Cleanup(func() { _ = os.Chmod(svc.paths.TasksDir, 0o755) })
			})

			switch command {
			case "new":
				_, err = svc.CreateTask(CreateTaskInput{Title: "New", SpecRef: "specs/v0.1.0.md#summary"})
			case "rename":
				_, err = svc.RenameTask(RenameTaskInput{OldID: base.TaskID, Slug: "renamed", SlugExplicit: true})
			case "repoint":
				_, err = svc.RepointTask(RepointTaskInput{TaskID: work.TaskID, Area: "details"})
			case "dependency":
				_, err = svc.EditDependency(EditDependencyInput{TaskID: work.TaskID, DependencyID: extra.TaskID, Operation: DependencyAdd})
			}
			if err == nil || MachineFailureFor(err).Code != MachineCodePartialWrite {
				t.Fatalf("%s with blocked local task publication = %v, want partial_write", command, err)
			}
			if got := readBytes(t, svc.paths.StateFile); got != stateBefore {
				t.Fatalf("%s left local state rolled forward", command)
			}
			if got := gitOutput(t, repo, "status", "--porcelain"); got != "" {
				t.Fatalf("%s exposed ignored local state to Git: %q", command, got)
			}
		})
	}
}

func TestInitLocalHasNoPretransactionScaffold(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	testHookLocalTransactionReady = func() error { return errors.New("injected before local transaction") }
	t.Cleanup(func() { testHookLocalTransactionReady = nil })

	_, err := newTestService(t, repo, time.Now()).Init(InitInput{Local: true})
	if err == nil || !strings.Contains(err.Error(), "injected before local transaction") {
		t.Fatalf("init local error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".taskrail")); !os.IsNotExist(err) {
		t.Fatalf("failed local init retained scaffold: %v", err)
	}
	if output := gitOutput(t, repo, "status", "--porcelain"); output != "" {
		t.Fatalf("failed local init changed Git status: %q", output)
	}
}

func TestInitLocalRecoversEmptyPretransactionScaffold(t *testing.T) {
	skipDurableSkillPublication(t)

	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	if err := os.MkdirAll(filepath.Join(repo, ".taskrail", "local", "planning", "tasks"), 0o755); err != nil {
		t.Fatalf("create abandoned local scaffold: %v", err)
	}

	if _, err := newTestService(t, repo, time.Now()).Init(InitInput{Local: true, WithSkills: true, SkillVersion: "v9.9.9"}); err != nil {
		t.Fatalf("recover empty local scaffold: %v", err)
	}
	if output := gitOutput(t, repo, "status", "--porcelain"); output != "" {
		t.Fatalf("recovered local scaffold changed Git status: %q", output)
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
	skipDurableSkillPublication(t)

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
