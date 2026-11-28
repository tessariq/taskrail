package taskrail

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/tessariq/taskrail/internal/durabletx"
	"github.com/tessariq/taskrail/internal/repolock"
)

// refreshLocalSkills updates only already-owned installed copies. Unlike fresh
// local initialization, no destination or exclusion may be created here.
func (s *Service) refreshLocalSkills(in InitInput) (result InitResult, err error) {
	if err := s.requireLocalStorage(); err != nil {
		return InitResult{}, err
	}
	if !in.WithSkills || !in.ForceSkills {
		return InitResult{}, invalidArgumentsf("initialized local storage refreshes packaged skills only with init --with-skills --force")
	}
	if plan, err := s.planLocalSkills(); err != nil {
		return InitResult{}, err
	} else if err := validateLocalSkillRefreshPlan(plan); err != nil {
		return InitResult{}, err
	}

	transactionID, err := newMigrationTransactionID()
	if err != nil {
		return InitResult{}, err
	}
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: s.paths.LockRepository(), Command: localInitCommand, TransactionID: transactionID,
		Capability: repolock.Capability{Commands: []string{localInitCommand}},
	})
	if err != nil {
		return InitResult{}, migrationLockError(err)
	}
	defer func() {
		if releaseErr := lock.Release(); releaseErr != nil && err == nil {
			err = releaseErr
		}
	}()

	plan, err := s.planLocalSkills()
	if err != nil {
		return InitResult{}, err
	}
	if err := validateLocalSkillRefreshPlan(plan); err != nil {
		return InitResult{}, err
	}
	excludePath, exclude, err := s.localSkillRefreshReadExclusion()
	if err != nil {
		return InitResult{}, err
	}
	members := []durabletx.Member{{
		Kind: durabletx.Git, Reported: filepath.ToSlash(excludePath),
		Path: gitRelativePath(s.paths.GitCommonDir, excludePath), Content: exclude,
	}}
	refreshMembers, consumed, installed, err := s.localSkillRefreshMembers(plan, in.SkillVersion)
	if err != nil {
		return InitResult{}, err
	}
	members = append(members, refreshMembers...)
	if testHookLocalSkillsPlanned != nil {
		if err := testHookLocalSkillsPlanned(); err != nil {
			return InitResult{}, err
		}
	}
	validate := func(evidence []durabletx.Evidence) error {
		if validationErr := validateLocalSkillRefreshEvidence(plan, excludePath, exclude, evidence); validationErr != nil {
			return validationErr
		}
		current, planErr := s.planLocalSkills()
		if planErr != nil {
			return planErr
		}
		if planErr := validateLocalSkillRefreshPlan(current); planErr != nil {
			return planErr
		}
		return s.verifyLocalIgnored()
	}
	if _, err := durabletx.Run(context.Background(), lock, s.paths.LockRepository(), durabletx.Request{
		Command: localInitCommand, Consumed: consumed, Members: members, Validate: validate,
	}); err != nil {
		return InitResult{}, s.mapMigrationFailure(transactionID, err)
	}

	return s.localSkillRefreshResult(installed)
}

// localSkillRefreshReadExclusion reads the effective Git exclusion a refresh
// binds to. Refresh never creates it, so an absent file is an error.
func (s *Service) localSkillRefreshReadExclusion() (string, []byte, error) {
	excludePath, err := localExcludePath(s.paths)
	if err != nil {
		return "", nil, err
	}
	exclude, err := readInitFile(excludePath)
	if err != nil {
		return "", nil, fmt.Errorf("inspect Git exclusion: %w", err)
	}
	if exclude == nil {
		return "", nil, fmt.Errorf("inspect Git exclusion: file is absent")
	}
	return excludePath, exclude, nil
}

// localSkillRefreshResult reports the repository as init sees it after the
// refresh committed, with the refreshed skills and a fresh validation.
func (s *Service) localSkillRefreshResult(installed SkillInstallResult) (InitResult, error) {
	marker, found, err := readMarker(s.paths.RepoRoot)
	if err != nil {
		return InitResult{}, err
	}
	initialized, err := s.planInit(marker, found, true)
	if err != nil {
		return InitResult{}, err
	}
	result, err := s.reportInit(initialized)
	if err != nil {
		return InitResult{}, err
	}
	result.SkillInstall = installed
	result.Skills, result.SkillExclusions, err = s.skillReport(installed)
	if err != nil {
		return InitResult{}, err
	}
	validation, err := s.Validate()
	if err != nil {
		return InitResult{}, err
	}
	result.Validation = &validation
	return result, nil
}

// localSkillRefreshMembers splits the planned destinations into members that
// rewrite a drifted copy and consumed paths that must stay byte-identical:
// preserved destinations and copies already matching the stamped package.
func (s *Service) localSkillRefreshMembers(plan localSkillPlan, version string) ([]durabletx.Member, []durabletx.Path, SkillInstallResult, error) {
	var members []durabletx.Member
	consumed := make([]durabletx.Path, 0, len(plan.Destinations))
	installed := SkillInstallResult{}
	for _, destination := range plan.Destinations {
		unchanged := durabletx.Path{Kind: durabletx.Worktree, Reported: destination.Path, Path: destination.Path}
		if destination.Action == localSkillPreserve {
			consumed = append(consumed, unchanged)
			continue
		}
		stamped, err := localSkillStampPackaged(destination.PackagePath, version)
		if err != nil {
			return nil, nil, SkillInstallResult{}, err
		}
		existing, readErr := os.ReadFile(filepath.Join(s.paths.RepoRoot, filepath.FromSlash(destination.Path)))
		if readErr != nil {
			return nil, nil, SkillInstallResult{}, fmt.Errorf("read skill destination %s: %w", destination.Path, readErr)
		}
		if bytes.Equal(existing, stamped) {
			consumed = append(consumed, unchanged)
			continue
		}
		members = append(members, durabletx.Member{
			Kind: durabletx.Worktree, Reported: destination.Path, Path: destination.Path, Content: stamped,
		})
		installed.Overwritten = append(installed.Overwritten, destination.Path)
	}
	return members, consumed, installed, nil
}

// localSkillStampPackaged reads one embedded skill file, refuses it unless it
// validates, and stamps the installing binary's version into it.
func localSkillStampPackaged(packagePath, version string) ([]byte, error) {
	packaged, err := shippableSkillsFS.ReadFile(path.Join(shippableSkillsRoot, packagePath))
	if err != nil {
		return nil, fmt.Errorf("read embedded skill %s: %w", packagePath, err)
	}
	if err := validateAgentSkill(packaged); err != nil {
		return nil, fmt.Errorf("validate embedded skill %s: %w", packagePath, err)
	}
	stamped, err := stampSkillVersion(packaged, version)
	if err != nil {
		return nil, fmt.Errorf("stamp embedded skill %s: %w", packagePath, err)
	}
	return stamped, nil
}

func validateLocalSkillRefreshPlan(plan localSkillPlan) error {
	if len(plan.Unexpected) != 0 {
		return fmt.Errorf("local skill destination %s contains adopter-owned content", plan.Unexpected[0].Path)
	}
	for _, destination := range plan.Destinations {
		if destination.Action != localSkillRefresh && destination.Action != localSkillPreserve {
			return fmt.Errorf("local skill destination %s is not safe to refresh", destination.Path)
		}
	}
	for _, exclusion := range plan.Exclusions {
		if exclusion.Ownership != localSkillExclusionManaged || !exclusion.Exact || !exclusion.Effective {
			return fmt.Errorf("local skill exclusion %s is not an effective managed exclusion", exclusion.Path)
		}
	}
	return nil
}

// validateLocalSkillRefreshEvidence binds the preflight classification to the
// durable transaction's original snapshots. Reclassifying only after publication
// would allow a concurrent but still syntactically valid skill replacement to
// silently change the transaction's ownership decision.
func validateLocalSkillRefreshEvidence(plan localSkillPlan, excludePath string, exclude []byte, evidence []durabletx.Evidence) error {
	observed := make(map[string]durabletx.Evidence, len(evidence))
	for _, item := range evidence {
		observed[string(item.Kind)+"\x00"+item.Reported] = item
	}
	for _, destination := range plan.Destinations {
		key := string(durabletx.Worktree) + "\x00" + destination.Path
		item, found := observed[key]
		if !found || item.OriginalSHA256 != destination.Digest {
			return fmt.Errorf("local skill destination %s changed after refresh preflight", destination.Path)
		}
	}
	excludeKey := string(durabletx.Git) + "\x00" + filepath.ToSlash(excludePath)
	item, found := observed[excludeKey]
	if !found || item.OriginalSHA256 != digestBytes(exclude) {
		return fmt.Errorf("Git exclusion changed after local skill refresh preflight")
	}
	return nil
}
