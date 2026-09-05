package taskrail

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tessariq/taskrail/internal/durabletx"
)

func TestInitLocalSkillsRefreshRejectsPostPlanDestinationChange(t *testing.T) {
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
	parityPath := installed.Skills[0].Path
	packagePath := strings.TrimPrefix(parityPath, ".agents/skills/")
	packaged, err := shippableSkillsFS.ReadFile(path.Join(shippableSkillsRoot, packagePath))
	if err != nil {
		t.Fatalf("read embedded skill: %v", err)
	}
	writeFile(t, filepath.Join(repo, filepath.FromSlash(parityPath)), string(packaged))

	testHookLocalSkillsPlanned = func() error {
		stamped, stampErr := stampSkillVersion(packaged, "v9.9.8")
		if stampErr != nil {
			return stampErr
		}
		return os.WriteFile(filepath.Join(repo, filepath.FromSlash(parityPath)), stamped, 0o644)
	}
	t.Cleanup(func() { testHookLocalSkillsPlanned = nil })

	if _, err := svc.Init(InitInput{WithSkills: true, ForceSkills: true, SkillVersion: "v9.9.9"}); err == nil || !strings.Contains(err.Error(), "changed after refresh preflight") {
		t.Fatalf("refresh after parity drift = %v, want preflight conflict", err)
	}
}

func TestInitLocalSkillsRefreshRejectsPostPlanAdopterContent(t *testing.T) {
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
	adopterPath := filepath.Join(repo, filepath.FromSlash(path.Dir(installed.Skills[0].Path)), "adopter.md")
	testHookLocalSkillsPlanned = func() error {
		return os.WriteFile(adopterPath, []byte("keep me\n"), 0o644)
	}
	t.Cleanup(func() { testHookLocalSkillsPlanned = nil })

	if _, err := svc.Init(InitInput{WithSkills: true, ForceSkills: true, SkillVersion: "v9.9.9"}); err == nil || !strings.Contains(err.Error(), "adopter-owned content") {
		t.Fatalf("refresh after adopter-content drift = %v, want refusal", err)
	}
}

func TestInitLocalSkillsRefreshRejectsPostPlanExclusionChange(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))
	if _, err := setup.Init(InitInput{Local: true, WithSkills: true, SkillVersion: "v9.9.8"}); err != nil {
		t.Fatalf("init local with skills: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("rediscover local service: %v", err)
	}
	excludePath, err := localExcludePath(svc.paths)
	if err != nil {
		t.Fatalf("local exclusion path: %v", err)
	}
	exclude, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("read local exclusion: %v", err)
	}
	testHookLocalSkillsPlanned = func() error {
		return os.WriteFile(excludePath, append(exclude, []byte("# concurrent change\n")...), 0o644)
	}
	t.Cleanup(func() { testHookLocalSkillsPlanned = nil })

	if _, err := svc.Init(InitInput{WithSkills: true, ForceSkills: true, SkillVersion: "v9.9.9"}); err == nil || !strings.Contains(err.Error(), "Git exclusion changed") {
		t.Fatalf("refresh after exclusion drift = %v, want preflight conflict", err)
	}
}

func TestLocalSkillRefreshRecoveryShapeExcludesMigrationMembers(t *testing.T) {
	t.Parallel()

	snapshots := []durabletx.Evidence{
		{Kind: durabletx.Worktree, Reported: ".agents/skills/taskrail-repair/SKILL.md"},
		{Kind: durabletx.Managed, Reported: ".taskrail/config.yml"},
	}
	if hasLocalSkillRefreshMembers(snapshots) {
		t.Fatal("combined migration members were misclassified as a local skill refresh")
	}
}

func TestValidateInitRecoveryAcceptsLocalSkillRefreshCandidate(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))
	if _, err := setup.Init(InitInput{Local: true, WithSkills: true, SkillVersion: "v9.9.8"}); err != nil {
		t.Fatalf("init local with skills: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("rediscover local service: %v", err)
	}
	plan, err := svc.planLocalSkills()
	if err != nil {
		t.Fatalf("plan local skills: %v", err)
	}
	if err := validateLocalSkillRefreshPlan(plan); err != nil {
		t.Fatalf("validate refresh plan: %v", err)
	}
	evidence := make([]durabletx.Evidence, 0, len(plan.Destinations))
	for _, destination := range plan.Destinations {
		packaged, readErr := shippableSkillsFS.ReadFile(path.Join(shippableSkillsRoot, destination.PackagePath))
		if readErr != nil {
			t.Fatalf("read embedded %s: %v", destination.PackagePath, readErr)
		}
		candidate, stampErr := stampSkillVersion(packaged, "v9.9.9")
		if stampErr != nil {
			t.Fatalf("stamp %s: %v", destination.PackagePath, stampErr)
		}
		writeFile(t, filepath.Join(repo, filepath.FromSlash(destination.Path)), string(candidate))
		evidence = append(evidence, durabletx.Evidence{
			Kind: durabletx.Worktree, Reported: destination.Path,
			CandidateSHA256: digestBytes(candidate), CurrentSHA256: digestBytes(candidate),
		})
	}
	if err := svc.validateInitRecovery(recoverFixtureID, evidence); err != nil {
		t.Fatalf("validate complete local skill refresh candidate: %v", err)
	}
	for _, destination := range plan.Destinations {
		data, readErr := os.ReadFile(filepath.Join(repo, filepath.FromSlash(destination.Path)))
		if readErr != nil {
			t.Fatalf("read recovered %s: %v", destination.Path, readErr)
		}
		if version, versionErr := skillVersionOf(data); versionErr != nil || version != "v9.9.9" {
			t.Fatalf("recovered %s version = %q, %v", destination.Path, version, versionErr)
		}
	}
	if validation, validationErr := svc.Validate(); validationErr != nil || !validation.Valid {
		t.Fatalf("validate recovered refresh = %+v, %v", validation, validationErr)
	}
	if status := gitOutput(t, repo, "status", "--porcelain"); status != "" {
		t.Fatalf("recovered refresh exposed local bytes to Git: %q", status)
	}
}
