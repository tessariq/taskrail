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
	excludePath, err := localExcludePath(s.paths)
	if err != nil {
		return InitResult{}, err
	}
	exclude, err := readInitFile(excludePath)
	if err != nil {
		return InitResult{}, fmt.Errorf("inspect Git exclusion: %w", err)
	}
	if exclude == nil {
		return InitResult{}, fmt.Errorf("inspect Git exclusion: file is absent")
	}
	members := []durabletx.Member{{
		Kind: durabletx.Git, Reported: filepath.ToSlash(excludePath),
		Path: gitRelativePath(s.paths.GitCommonDir, excludePath), Content: exclude,
	}}
	consumed := make([]durabletx.Path, 0, len(plan.Destinations))
	installed := SkillInstallResult{}
	for _, destination := range plan.Destinations {
		if destination.Action == localSkillPreserve {
			consumed = append(consumed, durabletx.Path{
				Kind: durabletx.Worktree, Reported: destination.Path, Path: destination.Path,
			})
			continue
		}
		packaged, readErr := shippableSkillsFS.ReadFile(path.Join(shippableSkillsRoot, destination.PackagePath))
		if readErr != nil {
			return InitResult{}, fmt.Errorf("read embedded skill %s: %w", destination.PackagePath, readErr)
		}
		if validateErr := validateAgentSkill(packaged); validateErr != nil {
			return InitResult{}, fmt.Errorf("validate embedded skill %s: %w", destination.PackagePath, validateErr)
		}
		stamped, stampErr := stampSkillVersion(packaged, in.SkillVersion)
		if stampErr != nil {
			return InitResult{}, fmt.Errorf("stamp embedded skill %s: %w", destination.PackagePath, stampErr)
		}
		existing, readErr := os.ReadFile(filepath.Join(s.paths.RepoRoot, filepath.FromSlash(destination.Path)))
		if readErr != nil {
			return InitResult{}, fmt.Errorf("read skill destination %s: %w", destination.Path, readErr)
		}
		if bytes.Equal(existing, stamped) {
			consumed = append(consumed, durabletx.Path{
				Kind: durabletx.Worktree, Reported: destination.Path, Path: destination.Path,
			})
			continue
		}
		members = append(members, durabletx.Member{
			Kind: durabletx.Worktree, Reported: destination.Path, Path: destination.Path, Content: stamped,
		})
		installed.Overwritten = append(installed.Overwritten, destination.Path)
	}
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

	marker, found, err := readMarker(s.paths.RepoRoot)
	if err != nil {
		return InitResult{}, err
	}
	result, err = s.reportInit(s.planInit(marker, found, true))
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
