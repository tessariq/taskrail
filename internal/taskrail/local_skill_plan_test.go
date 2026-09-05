package taskrail

import (
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPlanLocalSkillsInventoriesFreshDestinationsWithoutMutation(t *testing.T) {
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
		t.Fatalf("new local service: %v", err)
	}
	before := snapshotTree(t, repo)

	plan, err := svc.planLocalSkills()
	if err != nil {
		t.Fatalf("plan local skills: %v", err)
	}

	files, err := packagedSkillFiles()
	if err != nil {
		t.Fatalf("packaged skill files: %v", err)
	}
	if got, want := len(plan.Destinations), len(files)*len(shippableSkillTargets); got != want {
		t.Fatalf("destinations = %d, want %d", got, want)
	}
	for _, destination := range plan.Destinations {
		if destination.Action != localSkillCreate || destination.Present || destination.Digest != "" || destination.PackageDigest == "" {
			t.Fatalf("fresh destination = %+v", destination)
		}
		if filepath.IsAbs(destination.Path) || !isSkillDestination(destination.Path) {
			t.Fatalf("destination path = %q, want normal assistant discovery path", destination.Path)
		}
	}

	names, err := packagedSkillNames()
	if err != nil {
		t.Fatalf("packaged skill names: %v", err)
	}
	if got, want := len(plan.Exclusions), len(names)*len(shippableSkillTargets); got != want {
		t.Fatalf("exclusions = %d, want %d", got, want)
	}
	for _, exclusion := range plan.Exclusions {
		if !isSkillDestination(exclusion.Path+"/SKILL.md") || exclusion.Ownership != localSkillExclusionCreate {
			t.Fatalf("fresh exclusion = %+v", exclusion)
		}
	}
	if got := snapshotTree(t, repo); !reflect.DeepEqual(got, before) {
		t.Fatal("local skill plan changed repository bytes")
	}
}

func TestPlanLocalSkillsClassifiesDestinationSafety(t *testing.T) {
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
		t.Fatalf("new local service: %v", err)
	}
	files, err := packagedSkillFiles()
	if err != nil {
		t.Fatalf("packaged skill files: %v", err)
	}
	rel := files[0]
	packaged, err := shippableSkillsFS.ReadFile(path.Join(shippableSkillsRoot, rel))
	if err != nil {
		t.Fatalf("read packaged skill: %v", err)
	}
	destination := path.Join(".agents/skills", rel)
	physical := filepath.Join(repo, filepath.FromSlash(destination))

	for _, tc := range []struct {
		name       string
		prepare    func(t *testing.T)
		wantAction string
		wantMarker string
		wantAlias  bool
	}{
		{
			name: "marker-free parity mirror",
			prepare: func(t *testing.T) {
				writeFile(t, physical, string(packaged))
			},
			wantAction: localSkillPreserve,
			wantMarker: "none",
		},
		{
			name: "stamped copy can refresh",
			prepare: func(t *testing.T) {
				stamped, err := stampSkillVersion(packaged, "v9.9.9")
				if err != nil {
					t.Fatalf("stamp skill: %v", err)
				}
				writeFile(t, physical, string(stamped))
				addLocalSkillExclusion(t, repo, filepath.ToSlash(filepath.Dir(destination)))
			},
			wantAction: localSkillRefresh,
			wantMarker: "nested",
		},
		{
			name: "conflicting marker refuses",
			prepare: func(t *testing.T) {
				stamped, err := stampSkillVersion(packaged, "v9.9.9")
				if err != nil {
					t.Fatalf("stamp skill: %v", err)
				}
				conflicting := strings.Replace(string(stamped), "description:", "taskrail_version: old\ndescription:", 1)
				writeFile(t, physical, conflicting)
				addLocalSkillExclusion(t, repo, filepath.ToSlash(filepath.Dir(destination)))
			},
			wantAction: localSkillRefuse,
			wantMarker: "conflicting",
		},
		{
			name: "case alias refuses",
			prepare: func(t *testing.T) {
				alias := filepath.Join(repo, ".agents", "skills", strings.ToUpper(filepath.Base(filepath.Dir(rel))), skillFileName)
				writeFile(t, alias, string(packaged))
			},
			wantAction: localSkillRefuse,
			wantMarker: "",
			wantAlias:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, target := range shippableSkillTargets {
				_ = os.RemoveAll(filepath.Join(repo, target))
			}
			tc.prepare(t)
			before := snapshotTree(t, repo)
			plan, err := svc.planLocalSkills()
			if err != nil {
				t.Fatalf("plan local skills: %v", err)
			}
			got := localSkillPlanDestination(t, plan, destination)
			if got.Action != tc.wantAction || got.Marker != tc.wantMarker || got.Alias != tc.wantAlias {
				t.Fatalf("destination = %+v, want action=%q marker=%q alias=%t", got, tc.wantAction, tc.wantMarker, tc.wantAlias)
			}
			if after := snapshotTree(t, repo); !reflect.DeepEqual(after, before) {
				t.Fatal("local skill plan changed classified bytes")
			}
		})
	}
}

func TestPlanLocalSkillsRefusesStagedAbsentDestination(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Now())
	if _, err := setup.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("new local service: %v", err)
	}
	files, err := packagedSkillFiles()
	if err != nil {
		t.Fatalf("packaged skill files: %v", err)
	}
	destination := path.Join(".agents/skills", files[0])
	physical := filepath.Join(repo, filepath.FromSlash(destination))
	writeFile(t, physical, "staged only\n")
	runLocalGit(t, repo, "add", "--", destination)
	if err := os.Remove(physical); err != nil {
		t.Fatalf("remove staged destination: %v", err)
	}

	plan, err := svc.planLocalSkills()
	if err != nil {
		t.Fatalf("plan local skills: %v", err)
	}
	got := localSkillPlanDestination(t, plan, destination)
	if got.EntryType != "absent" || !got.Staged || got.Action != localSkillRefuse {
		t.Fatalf("staged absent destination = %+v", got)
	}
}

func TestPlanLocalSkillsRefusesUnexpectedSubtreeContent(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Now())
	if _, err := setup.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("new local service: %v", err)
	}
	files, err := packagedSkillFiles()
	if err != nil {
		t.Fatalf("packaged skill files: %v", err)
	}
	destination := path.Join(".agents/skills", files[0])
	writeFile(t, filepath.Join(repo, ".agents", "skills", filepath.Base(filepath.Dir(files[0])), "adopter.md"), "keep me\n")

	plan, err := svc.planLocalSkills()
	if err != nil {
		t.Fatalf("plan local skills: %v", err)
	}
	if got := localSkillPlanDestination(t, plan, destination); got.Action != localSkillRefuse {
		t.Fatalf("destination with adopter-owned sibling = %+v", got)
	}
	if len(plan.Unexpected) != 1 || plan.Unexpected[0].Path != path.Join(".agents/skills", filepath.Base(filepath.Dir(files[0])), "adopter.md") {
		t.Fatalf("unexpected subtree inventory = %+v", plan.Unexpected)
	}
}

func TestPlanLocalSkillsRejectsSymlinkedExclusionStore(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Now())
	if _, err := setup.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	excludePath := filepath.Join(repo, ".git", "info", "exclude")
	target := filepath.Join(repo, "exclude-target")
	if err := os.Rename(excludePath, target); err != nil {
		t.Fatalf("move exclusion store: %v", err)
	}
	if err := os.Symlink(target, excludePath); err != nil {
		t.Fatalf("symlink exclusion store: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("new local service: %v", err)
	}
	if _, err := svc.planLocalSkills(); err == nil {
		t.Fatal("plan accepted a symlinked exclusion store")
	}
}

func TestPlanLocalSkillsRejectsSymlinkedExclusionParent(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Now())
	if _, err := setup.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	info := filepath.Join(repo, ".git", "info")
	target := filepath.Join(repo, "info-target")
	if err := os.Rename(info, target); err != nil {
		t.Fatalf("move exclusion parent: %v", err)
	}
	if err := os.Symlink(target, info); err != nil {
		t.Fatalf("symlink exclusion parent: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("new local service: %v", err)
	}
	if _, err := svc.planLocalSkills(); err == nil {
		t.Fatal("plan accepted a symlinked exclusion parent")
	}
}

func TestGitIgnoredExactRefusesFailedInspection(t *testing.T) {
	t.Parallel()

	if _, err := gitIgnoredExact(t.TempDir(), ".agents/skills/probe"); err == nil {
		t.Fatal("failed Git ignore inspection was treated as not ignored")
	}
}

func TestPlanLocalSkillsClassifiesExclusionOwnership(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Now())
	if _, err := setup.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("new local service: %v", err)
	}
	names, err := packagedSkillNames()
	if err != nil {
		t.Fatalf("packaged skill names: %v", err)
	}
	subtree := path.Join(".agents/skills", names[0])

	addLocalSkillExclusion(t, repo, subtree)
	plan, err := svc.planLocalSkills()
	if err != nil {
		t.Fatalf("plan exact exclusion: %v", err)
	}
	if got := localSkillPlanExclusion(t, plan, subtree); got.Ownership != localSkillExclusionManaged || !got.Exact || !got.Effective {
		t.Fatalf("exact exclusion = %+v", got)
	}

	excludePath := filepath.Join(repo, ".git", "info", "exclude")
	exclude, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("read exclusion: %v", err)
	}
	writeFile(t, excludePath, strings.Replace(string(exclude), subtree+"\n", "", 1)+"/"+subtree+"/\n")
	if err := os.MkdirAll(filepath.Join(repo, filepath.FromSlash(subtree)), 0o755); err != nil {
		t.Fatalf("create externally excluded subtree: %v", err)
	}
	plan, err = svc.planLocalSkills()
	if err != nil {
		t.Fatalf("plan external exclusion: %v", err)
	}
	if got := localSkillPlanExclusion(t, plan, subtree); got.Ownership != localSkillExclusionExternal || got.Exact || !got.Effective {
		t.Fatalf("external exclusion = %+v", got)
	}

	withoutExact := strings.Replace(string(exclude), subtree+"\n", "", 1)
	writeFile(t, excludePath, strings.Replace(withoutExact, localExcludeEnd, ".agents/\n"+localExcludeEnd, 1))
	plan, err = svc.planLocalSkills()
	if err != nil {
		t.Fatalf("plan broad managed exclusion: %v", err)
	}
	if got := localSkillPlanExclusion(t, plan, subtree); got.Ownership != localSkillExclusionAmbiguous || got.Exact || !got.Effective {
		t.Fatalf("broad managed exclusion = %+v", got)
	}
}

func TestPlanLocalSkillsReportsLinkedWorktreeExclusionScope(t *testing.T) {
	t.Parallel()

	source := t.TempDir()
	initLocalGitRepo(t, source)
	writeFile(t, filepath.Join(source, "README.md"), "# source\n")
	runLocalGit(t, source, "add", "README.md")
	runLocalGit(t, source, "-c", "user.name=Taskrail", "-c", "user.email=taskrail@example.test", "commit", "-qm", "initial")
	linked := t.TempDir()
	if err := os.Remove(linked); err != nil {
		t.Fatalf("remove linked-worktree target: %v", err)
	}
	runLocalGit(t, source, "worktree", "add", "--detach", linked)
	requireRecoveryDirectoryDurability(t, linked)
	setup := newTestService(t, linked, time.Now())
	if _, err := setup.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init linked local: %v", err)
	}
	svc, err := NewService(linked)
	if err != nil {
		t.Fatalf("new linked service: %v", err)
	}
	plan, err := svc.planLocalSkills()
	if err != nil {
		t.Fatalf("plan linked local skills: %v", err)
	}
	for _, exclusion := range plan.Exclusions {
		if !exclusion.SharedGitScope {
			t.Fatalf("linked exclusion does not report shared Git scope: %+v", exclusion)
		}
	}
}

func TestPlanLocalSkillsReportsPrimaryWorktreeExclusionScope(t *testing.T) {
	t.Parallel()

	source := t.TempDir()
	initLocalGitRepo(t, source)
	writeFile(t, filepath.Join(source, "README.md"), "# source\n")
	runLocalGit(t, source, "add", "README.md")
	runLocalGit(t, source, "-c", "user.name=Taskrail", "-c", "user.email=taskrail@example.test", "commit", "-qm", "initial")
	linked := t.TempDir()
	if err := os.Remove(linked); err != nil {
		t.Fatalf("remove linked-worktree target: %v", err)
	}
	runLocalGit(t, source, "worktree", "add", "--detach", linked)
	requireRecoveryDirectoryDurability(t, source)
	setup := newTestService(t, source, time.Now())
	if _, err := setup.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init primary local: %v", err)
	}
	svc, err := NewService(source)
	if err != nil {
		t.Fatalf("new primary service: %v", err)
	}
	plan, err := svc.planLocalSkills()
	if err != nil {
		t.Fatalf("plan primary local skills: %v", err)
	}
	for _, exclusion := range plan.Exclusions {
		if !exclusion.SharedGitScope {
			t.Fatalf("primary exclusion does not report linked-worktree scope: %+v", exclusion)
		}
	}
}

func addLocalSkillExclusion(t *testing.T, repo, subtree string) {
	t.Helper()
	excludePath := filepath.Join(repo, ".git", "info", "exclude")
	exclude, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("read exclusion: %v", err)
	}
	writeFile(t, excludePath, strings.Replace(string(exclude), localExcludeEnd, subtree+"\n"+localExcludeEnd, 1))
}

func localSkillPlanDestination(t *testing.T, plan localSkillPlan, path string) localSkillDestination {
	t.Helper()
	for _, destination := range plan.Destinations {
		if destination.Path == path {
			return destination
		}
	}
	t.Fatalf("plan has no destination %s", path)
	return localSkillDestination{}
}

func localSkillPlanExclusion(t *testing.T, plan localSkillPlan, path string) localSkillExclusion {
	t.Helper()
	for _, exclusion := range plan.Exclusions {
		if exclusion.Path == path {
			return exclusion
		}
	}
	t.Fatalf("plan has no exclusion %s", path)
	return localSkillExclusion{}
}
