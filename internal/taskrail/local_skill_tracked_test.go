package taskrail

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// initAdoptedSkillFixture initializes a local repository whose packaged skill
// copies are installed, stamped, and excluded, then returns the discovered
// service, the repo root, the first skill subtree, and its on-disk SKILL.md
// bytes.
func initAdoptedSkillFixture(t *testing.T) (*Service, string, string, string) {
	t.Helper()
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Now())
	installed, err := setup.Init(InitInput{Local: true, WithSkills: true, SkillVersion: "v9.9.9"})
	if err != nil {
		t.Fatalf("init local with skills: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("discover local storage: %v", err)
	}
	skillFile := installed.Skills[0].Path
	data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(skillFile)))
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	return svc, repo, filepath.ToSlash(filepath.Dir(skillFile)), string(data)
}

func commitSkillSubtree(t *testing.T, repo, subtree string) {
	t.Helper()
	runLocalGit(t, repo, "add", "-f", "--", subtree)
	runLocalGit(t, repo, "-c", "user.name=Taskrail", "-c", "user.email=taskrail@example.test", "commit", "-qm", "adopt skill copy")
}

func TestLocalStatusClassifiesSkillCopiesByGitOwnership(t *testing.T) {
	skipDurableSkillPublication(t)

	t.Parallel()

	for _, tc := range []struct {
		name          string
		prepare       func(t *testing.T, repo, subtree string)
		wantManaged   bool
		wantEffective bool
	}{
		{
			name:          "untracked excluded copy stays managed",
			prepare:       func(t *testing.T, repo, subtree string) {},
			wantManaged:   true,
			wantEffective: true,
		},
		{
			name: "committed copy is adopter-owned",
			prepare: func(t *testing.T, repo, subtree string) {
				commitSkillSubtree(t, repo, subtree)
			},
			wantManaged: false,
		},
		{
			name: "staged copy is adopter-owned",
			prepare: func(t *testing.T, repo, subtree string) {
				runLocalGit(t, repo, "add", "-f", "--", subtree)
			},
			wantManaged: false,
		},
		{
			name: "git rm --cached returns the copy to managed",
			prepare: func(t *testing.T, repo, subtree string) {
				commitSkillSubtree(t, repo, subtree)
				runLocalGit(t, repo, "rm", "--cached", "-r", "-q", "--", subtree)
			},
			wantManaged:   true,
			wantEffective: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo, subtree, _ := initAdoptedSkillFixture(t)
			tc.prepare(t, repo, subtree)

			status, err := svc.LocalStatus()
			if err != nil {
				t.Fatalf("local status: %v", err)
			}

			var row *LocalExclusion
			for i := range status.Exclusions {
				if status.Exclusions[i].Path == subtree {
					row = &status.Exclusions[i]
				}
			}
			if tc.wantManaged {
				if row == nil || row.Source != "managed" || row.Effective != tc.wantEffective {
					t.Fatalf("managed row for %s = %+v", subtree, row)
				}
			} else if row != nil {
				t.Fatalf("adopter-owned %s is still a managed exclusion row: %+v", subtree, *row)
			}
			for _, violation := range status.Violations {
				if violation.Path != nil && *violation.Path == subtree {
					t.Fatalf("adopter-owned %s reported violation %+v", subtree, violation)
				}
			}
			if !status.PromotionReady {
				t.Fatalf("promotion ready = false, violations = %+v", status.Violations)
			}
		})
	}
}

func TestLocalStatusIgnoresCommittedSkillCopyWithoutExclusion(t *testing.T) {
	skipDurableSkillPublication(t)

	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Now())
	if _, err := setup.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	installed, err := setup.WriteShippableSkills("v9.9.9", false)
	if err != nil {
		t.Fatalf("install local skills: %v", err)
	}
	committed := filepath.ToSlash(filepath.Dir(installed.Written[0]))
	for _, written := range installed.Written {
		if subtree := filepath.ToSlash(filepath.Dir(written)); subtree != committed {
			addLocalSkillExclusion(t, repo, subtree)
		}
	}
	commitSkillSubtree(t, repo, committed)

	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("discover local storage: %v", err)
	}
	status, err := svc.LocalStatus()
	if err != nil {
		t.Fatalf("local status: %v", err)
	}
	for _, exclusion := range status.Exclusions {
		if exclusion.Path == committed {
			t.Fatalf("committed copy %s is still a managed exclusion row: %+v", committed, exclusion)
		}
	}
	for _, violation := range status.Violations {
		if violation.Path != nil && *violation.Path == committed {
			t.Fatalf("committed copy %s reported violation %+v", committed, violation)
		}
	}
	if !status.PromotionReady {
		t.Fatalf("promotion ready = false, violations = %+v", status.Violations)
	}
}

func TestLocalPromoteWithSkillsLeavesTrackedSkillCopiesToAdopter(t *testing.T) {
	skipDurableSkillPublication(t)

	t.Parallel()

	svc, repo, subtree, skillBefore := initAdoptedSkillFixture(t)
	commitSkillSubtree(t, repo, subtree)

	plan, err := svc.planLocalSkills()
	if err != nil {
		t.Fatalf("plan local skills: %v", err)
	}
	trackedFile := subtree + "/SKILL.md"
	if got := localSkillPlanDestination(t, plan, trackedFile); !got.Tracked || got.Action != localSkillRefuse {
		t.Fatalf("installation plan must keep refusing the tracked destination, got %+v", got)
	}

	counterpart := strings.Replace(subtree, ".agents/skills/", ".claude/skills/", 1)
	result, err := svc.LocalPromote(LocalPromoteInput{Apply: true, WithSkills: true})
	if err != nil {
		t.Fatalf("promote with skills over a tracked copy: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(trackedFile))); err != nil || string(got) != skillBefore {
		t.Fatalf("tracked skill bytes changed: %v", err)
	}
	for _, skill := range result.Skills {
		if skill.Path == trackedFile {
			t.Fatalf("tracked copy %s reported as %q", trackedFile, skill.Action)
		}
	}
	found := false
	for _, skill := range result.Skills {
		if skill.Path == counterpart+"/SKILL.md" {
			found = skill.Action == "promote"
		}
	}
	if !found {
		t.Fatalf("untracked counterpart was not promoted: %+v", result.Skills)
	}
	for _, removed := range result.RemovedExclusions {
		if removed == subtree {
			t.Fatalf("tracked copy exclusion %s was removed", subtree)
		}
	}
	if !slices.Contains(result.RemovedExclusions, counterpart) {
		t.Fatalf("untracked counterpart exclusion was not removed: %+v", result.RemovedExclusions)
	}
	exclude, err := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(exclude), subtree) || strings.Contains(string(exclude), counterpart) {
		t.Fatalf("promotion exclusion result:\n%s", exclude)
	}

	committedService, err := NewService(repo)
	if err != nil {
		t.Fatalf("discover committed storage: %v", err)
	}
	if validation, err := committedService.Validate(); err != nil || !validation.Valid {
		t.Fatalf("validate promoted state: %+v, %v", validation, err)
	}
}

// TestLocalPromoteWithSkillsDropsNestedFilesOfTrackedSubtrees covers packaged
// skills that carry nested files (taskrail-sdd-handoff/references/*): the
// whole subtree is the adopter's as soon as Git tracks any file inside it, so
// promotion must drop every nested destination, not only the top-level file.
func TestLocalPromoteWithSkillsDropsNestedFilesOfTrackedSubtrees(t *testing.T) {
	skipDurableSkillPublication(t)

	t.Parallel()

	nestedSubtree := ".agents/skills/taskrail-sdd-handoff"
	nestedFiles := []string{
		nestedSubtree + "/SKILL.md",
		nestedSubtree + "/references/openspec.md",
		nestedSubtree + "/references/spec-kit.md",
	}
	counterpart := ".claude/skills/taskrail-sdd-handoff"

	for _, tc := range []struct {
		name      string
		commit    func(t *testing.T, repo string)
		untouched []string
	}{
		{
			name:      "whole subtree committed",
			commit:    func(t *testing.T, repo string) { commitSkillSubtree(t, repo, nestedSubtree) },
			untouched: nestedFiles,
		},
		{
			name: "top-level SKILL.md only committed",
			commit: func(t *testing.T, repo string) {
				commitSkillSubtree(t, repo, nestedSubtree+"/SKILL.md")
			},
			untouched: nestedFiles,
		},
		{
			name: "nested references file only committed",
			commit: func(t *testing.T, repo string) {
				commitSkillSubtree(t, repo, nestedSubtree+"/references/openspec.md")
			},
			untouched: nestedFiles,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo, _, _ := initAdoptedSkillFixture(t)
			before := map[string]string{}
			for _, file := range nestedFiles {
				data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(file)))
				if err != nil {
					t.Fatalf("read nested skill file %s: %v", file, err)
				}
				before[file] = string(data)
			}
			tc.commit(t, repo)

			status, err := svc.LocalStatus()
			if err != nil {
				t.Fatalf("local status: %v", err)
			}
			if !status.PromotionReady {
				t.Fatalf("status not promotion ready over a tracked nested subtree: %+v", status.Violations)
			}
			for _, exclusion := range status.Exclusions {
				if exclusion.Path == nestedSubtree {
					t.Fatalf("tracked nested subtree is still a managed row: %+v", exclusion)
				}
			}

			preview, err := svc.LocalPromote(LocalPromoteInput{WithSkills: true})
			if err != nil {
				t.Fatalf("preview promotion over a tracked nested subtree: %v", err)
			}
			for _, skill := range preview.Skills {
				if strings.HasPrefix(skill.Path, nestedSubtree+"/") {
					t.Fatalf("preview reports tracked nested destination %s as %q", skill.Path, skill.Action)
				}
			}

			result, err := svc.LocalPromote(LocalPromoteInput{Apply: true, WithSkills: true})
			if err != nil {
				t.Fatalf("apply promotion over a tracked nested subtree: %v", err)
			}
			for _, file := range tc.untouched {
				got, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(file)))
				if err != nil || string(got) != before[file] {
					t.Fatalf("tracked nested file %s changed: %v", file, err)
				}
			}
			for _, skill := range result.Skills {
				if strings.HasPrefix(skill.Path, nestedSubtree+"/") {
					t.Fatalf("tracked nested destination %s reported as %q", skill.Path, skill.Action)
				}
			}
			if !slices.Contains(result.RemovedExclusions, counterpart) || slices.Contains(result.RemovedExclusions, nestedSubtree) {
				t.Fatalf("removed exclusions = %+v", result.RemovedExclusions)
			}
			exclude, err := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(exclude), nestedSubtree) || strings.Contains(string(exclude), counterpart) {
				t.Fatalf("promotion exclusion result:\n%s", exclude)
			}

			committedService, err := NewService(repo)
			if err != nil {
				t.Fatalf("discover committed storage: %v", err)
			}
			if validation, err := committedService.Validate(); err != nil || !validation.Valid {
				t.Fatalf("validate promoted state: %+v, %v", validation, err)
			}
		})
	}
}
