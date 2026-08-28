package taskrail

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/tessariq/taskrail/internal/durabletx"
	"github.com/tessariq/taskrail/internal/repolock"
	"gopkg.in/yaml.v3"
)

const localPromoteCommand = "local promote"

// LocalPromoteInput selects a write-free preview or the explicit promotion
// publication. Skill visibility stays a separate operator decision.
type LocalPromoteInput struct {
	Apply      bool
	WithSkills bool
}

type LocalPromoteEntry struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

type LocalPromoteSkill struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

// LocalPromoteResult is the stable promotion inventory. Managed semantic paths
// remain logical while private operational paths remain repository-relative.
type LocalPromoteResult struct {
	Applied           bool                `json:"applied"`
	SourceMode        string              `json:"source_mode"`
	TargetMode        string              `json:"target_mode"`
	Writes            []LocalPromoteEntry `json:"writes"`
	Preserved         []LocalPromoteEntry `json:"preserved"`
	Excluded          []LocalPromoteEntry `json:"excluded"`
	RemovedExclusions []string            `json:"removed_exclusions"`
	Skills            []LocalPromoteSkill `json:"skills"`
	Validation        ValidationResult    `json:"validation"`
}

type localPromotionFile struct {
	Source      string
	Destination string
	Content     []byte
	Kind        string
}

type localPromotionCandidate struct {
	marker       []byte
	fence        []byte
	exclude      []byte
	excludeFinal []byte
	files        []localPromotionFile
	skills       localSkillPlan
	result       LocalPromoteResult
	semantic     bool
	withSkills   bool
}

// LocalPromote publishes local semantic state or, after semantic publication,
// makes the sole valid pending managed skill installation visible without a
// Git commit.
func (s *Service) LocalPromote(in LocalPromoteInput) (LocalPromoteResult, error) {
	switch s.paths.Storage.Mode {
	case StorageLocal:
		return s.localPromoteSemantic(in)
	case StorageCommitted:
		if !in.WithSkills {
			return LocalPromoteResult{}, WithMachineErrorCode(MachineCodeUnsupported,
				fmt.Errorf("local promote requires local storage unless --with-skills completes pending skill visibility"))
		}
		return s.localPromotePendingSkills(in)
	default:
		return LocalPromoteResult{}, WithMachineErrorCode(MachineCodeUnsupported, fmt.Errorf("local promote requires supported storage"))
	}
}

func (s *Service) localPromoteSemantic(in LocalPromoteInput) (LocalPromoteResult, error) {
	if !in.Apply {
		candidate, err := s.buildLocalPromotionCandidate("", in.WithSkills)
		if err != nil {
			return LocalPromoteResult{}, err
		}
		return candidate.result, nil
	}

	transactionID, err := newMigrationTransactionID()
	if err != nil {
		return LocalPromoteResult{}, err
	}
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: s.paths.LockRepository(), Command: localPromoteCommand, TransactionID: transactionID,
		Capability: repolock.Capability{Commands: []string{localPromoteCommand}},
	})
	if err != nil {
		return LocalPromoteResult{}, migrationLockError(err)
	}
	defer func() { _ = lock.Release() }()

	candidate, err := s.buildLocalPromotionCandidate(transactionID, in.WithSkills)
	if err != nil {
		return LocalPromoteResult{}, err
	}
	members, consumed, err := s.localPromotionMembers(candidate)
	if err != nil {
		return LocalPromoteResult{}, err
	}
	validate := func(evidence []durabletx.Evidence) error {
		if localPromotionPublished(evidence) {
			return s.validateLocalPromotionCandidate(candidate)
		}
		return s.validateLocalPromotionSource(candidate)
	}
	if _, err := durabletx.Run(context.Background(), lock, s.paths.LockRepository(), durabletx.Request{
		Command: localPromoteCommand, Members: members, Consumed: consumed, Validate: validate,
	}); err != nil {
		return LocalPromoteResult{}, s.mapMigrationFailure(transactionID, err)
	}
	if err := s.validateLocalPromotionCandidate(candidate); err != nil {
		return LocalPromoteResult{}, WithMachineFailure(MachineFailure{Code: MachineCodeRepositoryInvalid, Applied: true}, err)
	}
	candidate.result.Applied = true
	return candidate.result, nil
}

func (s *Service) localPromotePendingSkills(in LocalPromoteInput) (LocalPromoteResult, error) {
	if !in.Apply {
		candidate, err := s.buildPendingSkillPromotionCandidate("")
		if err != nil {
			return LocalPromoteResult{}, err
		}
		return candidate.result, nil
	}

	transactionID, err := newMigrationTransactionID()
	if err != nil {
		return LocalPromoteResult{}, err
	}
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: s.paths.LockRepository(), Command: localPromoteCommand, TransactionID: transactionID,
		Capability: repolock.Capability{Commands: []string{localPromoteCommand}},
	})
	if err != nil {
		return LocalPromoteResult{}, migrationLockError(err)
	}
	defer func() { _ = lock.Release() }()

	candidate, err := s.buildPendingSkillPromotionCandidate(transactionID)
	if err != nil {
		return LocalPromoteResult{}, err
	}
	members, consumed, err := s.localPromotionMembers(candidate)
	if err != nil {
		return LocalPromoteResult{}, err
	}
	validate := func(evidence []durabletx.Evidence) error {
		if localPendingSkillPromotionPublished(evidence) {
			return s.validatePendingSkillPromotionCandidate(candidate)
		}
		return s.validatePendingSkillPromotionSource(candidate)
	}
	if _, err := durabletx.Run(context.Background(), lock, s.paths.LockRepository(), durabletx.Request{
		Command: localPromoteCommand, Members: members, Consumed: consumed, Validate: validate,
	}); err != nil {
		return LocalPromoteResult{}, s.mapMigrationFailure(transactionID, err)
	}
	if err := s.validatePendingSkillPromotionCandidate(candidate); err != nil {
		return LocalPromoteResult{}, WithMachineFailure(MachineFailure{Code: MachineCodeRepositoryInvalid, Applied: true}, err)
	}
	candidate.result.Applied = true
	return candidate.result, nil
}

func (s *Service) buildLocalPromotionCandidate(transactionID string, withSkills bool) (localPromotionCandidate, error) {
	marker, err := os.ReadFile(s.paths.ConfigFile)
	if err != nil {
		return localPromotionCandidate{}, fmt.Errorf("read local layout marker: %w", err)
	}
	cfg, err := decodeLayoutMarkerStrict(marker)
	if err != nil {
		return localPromotionCandidate{}, WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("decode local layout marker: %w", err))
	}
	if cfg.StorageMode != StorageLocal || cfg.MigrationFence != nil {
		return localPromotionCandidate{}, WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("local promotion requires an unfenced local layout marker"))
	}
	if validation, err := s.Validate(); err != nil || !validation.Valid {
		return localPromotionCandidate{}, WithMachineErrorCode(MachineCodeValidationFailed, fmt.Errorf("local semantic state is not valid: %v %v", validation.Violations, err))
	}
	if shared, err := localSkillSharedGitScope(s.paths); err != nil {
		return localPromotionCandidate{}, err
	} else if shared {
		return localPromotionCandidate{}, WithMachineErrorCode(MachineCodeWriteConflict,
			fmt.Errorf("local promotion refuses a shared Git exclusion scope with linked worktrees"))
	}
	skills, err := s.planLocalSkills()
	if err != nil {
		return localPromotionCandidate{}, err
	}
	if err := s.validatePromotionSkills(skills, withSkills); err != nil {
		return localPromotionCandidate{}, WithMachineErrorCode(MachineCodeWriteConflict, err)
	}
	excludePath, err := localExcludePath(s.paths)
	if err != nil {
		return localPromotionCandidate{}, err
	}
	exclude, err := os.ReadFile(excludePath)
	if err != nil {
		return localPromotionCandidate{}, fmt.Errorf("read Git exclusion: %w", err)
	}
	excluded := s.localPromotionExcluded()
	excludeFinal, removed := localPromotionExcludeCandidate(exclude, excluded, localPromotionSkillExclusions(skills, withSkills), true)
	files, err := s.localPromotionFiles()
	if err != nil {
		return localPromotionCandidate{}, err
	}
	if err := s.validateLocalPromotionStore(files); err != nil {
		return localPromotionCandidate{}, err
	}
	if err := s.validatePromotionDestinations(files); err != nil {
		return localPromotionCandidate{}, err
	}
	if err := s.validatePromotionMarkerDestination(); err != nil {
		return localPromotionCandidate{}, err
	}

	cfg.StorageMode = StorageCommitted
	finalMarker, err := yaml.Marshal(cfg)
	if err != nil {
		return localPromotionCandidate{}, fmt.Errorf("marshal committed promotion marker: %w", err)
	}
	if _, err := decodeLayoutMarkerStrict(finalMarker); err != nil {
		return localPromotionCandidate{}, fmt.Errorf("validate committed promotion marker: %w", err)
	}
	var fence []byte
	if transactionID != "" {
		fenced := cfg
		fenced.MigrationFence = &Layout2MigrationFence{FromLayoutVersion: layout2Version, FromStorageMode: StorageLocal, TransactionID: transactionID}
		fence, err = yaml.Marshal(fenced)
		if err != nil {
			return localPromotionCandidate{}, fmt.Errorf("marshal local promotion fence: %w", err)
		}
		if decoded, decodeErr := decodeLayoutMarkerStrict(fence); decodeErr != nil || decoded.MigrationFence == nil {
			return localPromotionCandidate{}, fmt.Errorf("validate local promotion fence: %w", decodeErr)
		}
	}
	result := localPromotionResult(files, skills, excluded, removed, withSkills)
	return localPromotionCandidate{marker: finalMarker, fence: fence, exclude: exclude, excludeFinal: excludeFinal, files: files, skills: skills, result: result, semantic: true, withSkills: withSkills}, nil
}

func (s *Service) buildPendingSkillPromotionCandidate(_ string) (localPromotionCandidate, error) {
	if validation, err := s.Validate(); err != nil || !validation.Valid {
		return localPromotionCandidate{}, WithMachineErrorCode(MachineCodeValidationFailed, fmt.Errorf("committed pending skill state is not valid: %v %v", validation.Violations, err))
	}
	if shared, err := localSkillSharedGitScope(s.paths); err != nil {
		return localPromotionCandidate{}, err
	} else if shared {
		return localPromotionCandidate{}, WithMachineErrorCode(MachineCodeWriteConflict,
			fmt.Errorf("local promotion refuses a shared Git exclusion scope with linked worktrees"))
	}
	skills, err := s.planPromotionSkills()
	if err != nil {
		return localPromotionCandidate{}, err
	}
	if err := s.validatePendingSkillPromotion(skills); err != nil {
		return localPromotionCandidate{}, WithMachineErrorCode(MachineCodeUnsupported, err)
	}
	excludePath, err := localExcludePath(s.paths)
	if err != nil {
		return localPromotionCandidate{}, err
	}
	exclude, err := os.ReadFile(excludePath)
	if err != nil {
		return localPromotionCandidate{}, fmt.Errorf("read Git exclusion: %w", err)
	}
	removed := localPromotionSkillExclusions(skills, true)
	excludeFinal, removed := localPromotionExcludeCandidate(exclude, nil, removed, false)
	result := LocalPromoteResult{
		SourceMode: string(StorageCommitted), TargetMode: string(StorageCommitted),
		Writes: []LocalPromoteEntry{}, Preserved: []LocalPromoteEntry{}, Excluded: []LocalPromoteEntry{},
		RemovedExclusions: removed, Skills: localPromotionSkills(skills, true),
		Validation: ValidationResult{Valid: true, Violations: []string{}},
	}
	return localPromotionCandidate{exclude: exclude, excludeFinal: excludeFinal, skills: skills, result: result, withSkills: true}, nil
}

func (s *Service) localPromotionFiles() ([]localPromotionFile, error) {
	var files []localPromotionFile
	for _, root := range []struct {
		physical string
		logical  string
		source   string
		kind     func(string) (string, error)
	}{
		{s.paths.SpecsDir, s.paths.LogicalSpecsDir, path.Join(localStorageRoot, s.paths.LogicalSpecsDir), func(string) (string, error) { return writeKindSpec, nil }},
		{s.paths.PlanningDir, s.paths.LogicalPlanningDir, path.Join(localStorageRoot, s.paths.LogicalPlanningDir), promotionPlanningKind},
		{s.paths.PromptsDir, s.paths.LogicalPromptsDir, path.Join(localStorageRoot, "prompts"), func(string) (string, error) { return "prompt", nil }},
	} {
		entries, err := localPromotionTree(root.physical, root.logical, root.source, root.kind, root.physical == s.paths.PlanningDir)
		if err != nil {
			return nil, err
		}
		files = append(files, entries...)
	}
	slices.SortFunc(files, func(a, b localPromotionFile) int { return strings.Compare(a.Destination, b.Destination) })
	if len(files) == 0 {
		return nil, WithMachineErrorCode(MachineCodeValidationFailed, fmt.Errorf("local semantic store has no files to promote"))
	}
	return files, nil
}

func localPromotionTree(root, logicalRoot, sourceRoot string, classify func(string) (string, error), skipArtifacts bool) ([]localPromotionFile, error) {
	var files []localPromotionFile
	err := filepath.WalkDir(root, func(physical string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, physical)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("local semantic path %s is a symlink", path.Join(logicalRoot, rel))
		}
		if entry.IsDir() {
			if skipArtifacts && (rel == "artifacts" || strings.HasPrefix(rel, "artifacts/")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("local semantic path %s is not a regular file", path.Join(logicalRoot, rel))
		}
		kind, err := classify(rel)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(physical)
		if err != nil {
			return err
		}
		files = append(files, localPromotionFile{Source: path.Join(sourceRoot, rel), Destination: path.Join(logicalRoot, rel), Content: data, Kind: kind})
		return nil
	})
	if os.IsNotExist(err) {
		return []localPromotionFile{}, nil
	}
	return files, err
}

func (s *Service) validateLocalPromotionStore(files []localPromotionFile) error {
	knownFiles := make(map[string]bool, len(files))
	for _, file := range files {
		knownFiles[file.Source] = true
	}
	relative := func(physical string) string { return filepath.ToSlash(relPath(s.paths.StorageRoot, physical)) }
	specs, planning, prompts := relative(s.paths.SpecsDir), relative(s.paths.PlanningDir), relative(s.paths.PromptsDir)
	artifacts, runtime := relative(s.paths.ArtifactsDir), relative(s.paths.RuntimeDir)
	allowedDirectory := func(rel string) bool {
		for _, root := range []string{specs, planning, prompts, artifacts, runtime} {
			if rel == root || strings.HasPrefix(root, rel+"/") || strings.HasPrefix(rel, root+"/") {
				return true
			}
		}
		return false
	}
	return filepath.WalkDir(s.paths.StorageRoot, func(physical string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := relative(physical)
		if rel == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("local storage contains symlink %s", rel)
		}
		if entry.IsDir() {
			if allowedDirectory(rel) {
				return nil
			}
			return fmt.Errorf("local storage contains unknown durable directory %s", rel)
		}
		physicalPath := path.Join(localStorageRoot, rel)
		if knownFiles[physicalPath] || strings.HasPrefix(rel, artifacts+"/") || strings.HasPrefix(rel, runtime+"/") {
			return nil
		}
		return fmt.Errorf("local storage contains unknown durable file %s", rel)
	})
}

func promotionPlanningKind(rel string) (string, error) {
	switch {
	case rel == "STATE.md":
		return writeKindState, nil
	case rel == notesFileName:
		return writeKindNote, nil
	case strings.HasPrefix(rel, "tasks/") && strings.HasSuffix(rel, ".md"):
		return writeKindTask, nil
	case strings.HasPrefix(rel, "reviews/"):
		return "review", nil
	default:
		return "", fmt.Errorf("local planning entry %s is not a promotable durable semantic file", rel)
	}
}

func (s *Service) validatePromotionDestinations(files []localPromotionFile) error {
	for _, file := range files {
		target := filepath.Join(s.paths.RepoRoot, filepath.FromSlash(file.Destination))
		if err := validateTargetPath(target); err != nil {
			return WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("inspect promotion destination %s: %w", file.Destination, err))
		}
		if _, err := os.Lstat(target); !os.IsNotExist(err) {
			if err == nil {
				return WithMachineErrorCode(MachineCodeDestinationExists, fmt.Errorf("promotion destination %s already exists", file.Destination))
			}
			return err
		}
		tracked, err := gitTracks(s.paths.WorktreeRoot, file.Destination)
		if err != nil {
			return err
		}
		staged, err := gitStaged(s.paths.WorktreeRoot, file.Destination)
		if err != nil {
			return err
		}
		if tracked || staged {
			return WithMachineErrorCode(MachineCodeWriteConflict, fmt.Errorf("Git index contains promotion destination %s", file.Destination))
		}
	}
	return nil
}

func (s *Service) validatePromotionMarkerDestination() error {
	target := s.paths.ConfigFile
	if err := validateTargetPath(target); err != nil {
		return WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("inspect promotion destination %s: %w", markerRelPath(), err))
	}
	for _, indexed := range []func(string, string) (bool, error){gitTracks, gitStaged} {
		present, err := indexed(s.paths.WorktreeRoot, markerRelPath())
		if err != nil {
			return err
		}
		if present {
			return WithMachineErrorCode(MachineCodeWriteConflict, fmt.Errorf("Git index contains promotion destination %s", markerRelPath()))
		}
	}
	return nil
}

func (s *Service) validatePromotionSkills(plan localSkillPlan, withSkills bool) error {
	if len(plan.Unexpected) != 0 {
		return fmt.Errorf("local skill destination %s contains adopter-owned content", plan.Unexpected[0].Path)
	}
	for _, destination := range plan.Destinations {
		if destination.Present && destination.Action == localSkillRefuse {
			return fmt.Errorf("local skill destination %s cannot be preserved exactly", destination.Path)
		}
	}
	for _, exclusion := range plan.Exclusions {
		if exclusion.Ownership == localSkillExclusionAmbiguous {
			return fmt.Errorf("local skill exclusion %s is ambiguous", exclusion.Path)
		}
	}
	if !withSkills {
		return nil
	}
	present := 0
	for _, destination := range plan.Destinations {
		if !destination.Present {
			continue
		}
		present++
		if destination.Action != localSkillPreserve && destination.Action != localSkillRefresh {
			return fmt.Errorf("local skill destination %s cannot be promoted", destination.Path)
		}
		if destination.Action == localSkillRefresh {
			data, err := os.ReadFile(filepath.Join(s.paths.RepoRoot, filepath.FromSlash(destination.Path)))
			if err != nil {
				return fmt.Errorf("read local skill destination %s: %w", destination.Path, err)
			}
			version, err := skillVersionOf(data)
			if err != nil {
				return fmt.Errorf("read local skill version %s: %w", destination.Path, err)
			}
			packaged, err := shippableSkillsFS.ReadFile(path.Join(shippableSkillsRoot, destination.PackagePath))
			if err != nil {
				return fmt.Errorf("read embedded skill %s: %w", destination.PackagePath, err)
			}
			expected, err := stampSkillVersion(packaged, version)
			if err != nil || !reflect.DeepEqual(data, expected) {
				return fmt.Errorf("local skill destination %s is not an unchanged packaged skill", destination.Path)
			}
		}
	}
	if present == 0 {
		return nil
	}
	if present != len(plan.Destinations) {
		return fmt.Errorf("local skill installation is incomplete")
	}
	for _, exclusion := range plan.Exclusions {
		if exclusion.Ownership != localSkillExclusionManaged || !exclusion.Exact || !exclusion.Effective || exclusion.Shadowed {
			return fmt.Errorf("local skill exclusion %s is not an effective managed exclusion", exclusion.Path)
		}
	}
	return nil
}

func (s *Service) validatePendingSkillPromotion(plan localSkillPlan) error {
	if err := s.validatePromotionSkills(plan, true); err != nil {
		return err
	}
	for _, destination := range plan.Destinations {
		if !destination.Present {
			return fmt.Errorf("committed storage has no pending managed skill exclusion")
		}
	}
	return nil
}

func localPromotionSkillExclusions(plan localSkillPlan, withSkills bool) []string {
	if !withSkills {
		return nil
	}
	removed := make([]string, 0, len(plan.Exclusions))
	for _, exclusion := range plan.Exclusions {
		if exclusion.Ownership == localSkillExclusionManaged && exclusion.Exact {
			removed = append(removed, exclusion.Path)
		}
	}
	slices.Sort(removed)
	return removed
}

func localPromotionExcludeCandidate(original []byte, excluded []LocalPromoteEntry, skillExclusions []string, removeLocal bool) ([]byte, []string) {
	removed := []string{}
	remove := map[string]bool{}
	if removeLocal {
		remove[markerRelPath()] = true
		remove[localStorageRoot+"/"] = true
	}
	for _, exclusion := range skillExclusions {
		remove[exclusion] = true
	}
	var out []string
	inManagedBlock := false
	for _, line := range strings.Split(string(original), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == localExcludeBegin {
			inManagedBlock = true
		}
		if inManagedBlock && remove[trimmed] {
			removed = append(removed, trimmed)
			continue
		}
		if trimmed == localExcludeEnd {
			for _, entry := range excluded {
				out = append(out, entry.Path+"/")
			}
			inManagedBlock = false
		}
		out = append(out, line)
	}
	return []byte(strings.Join(out, "\n")), removed
}

func (s *Service) localPromotionExcluded() []LocalPromoteEntry {
	return []LocalPromoteEntry{
		{Path: path.Join(localStorageRoot, s.paths.LogicalPlanningDir, "artifacts"), Kind: "artifact"},
		{Path: path.Join(localStorageRoot, "runtime"), Kind: "runtime"},
	}
}

func localPromotionResult(files []localPromotionFile, skills localSkillPlan, excluded []LocalPromoteEntry, removed []string, withSkills bool) LocalPromoteResult {
	writes := make([]LocalPromoteEntry, 0, len(files)+1)
	writes = append(writes, LocalPromoteEntry{Path: markerRelPath(), Kind: writeKindConfig})
	for _, file := range files {
		writes = append(writes, LocalPromoteEntry{Path: file.Destination, Kind: file.Kind})
	}
	skillsResult := localPromotionSkills(skills, withSkills)
	slices.SortFunc(writes, func(a, b LocalPromoteEntry) int { return strings.Compare(a.Path, b.Path) })
	slices.SortFunc(excluded, func(a, b LocalPromoteEntry) int { return strings.Compare(a.Path, b.Path) })
	slices.SortFunc(skillsResult, func(a, b LocalPromoteSkill) int { return strings.Compare(a.Path, b.Path) })
	slices.Sort(removed)
	return LocalPromoteResult{SourceMode: string(StorageLocal), TargetMode: string(StorageCommitted), Writes: writes,
		Preserved: []LocalPromoteEntry{}, Excluded: excluded, RemovedExclusions: removed, Skills: skillsResult,
		Validation: ValidationResult{Valid: true, Violations: []string{}}}
}

func localPromotionSkills(skills localSkillPlan, withSkills bool) []LocalPromoteSkill {
	result := make([]LocalPromoteSkill, 0, len(skills.Destinations))
	for _, destination := range skills.Destinations {
		action := "absent"
		if destination.Present {
			action = "preserve_local"
			if withSkills {
				action = "promote"
			}
		}
		result = append(result, LocalPromoteSkill{Path: destination.Path, Action: action})
	}
	slices.SortFunc(result, func(a, b LocalPromoteSkill) int { return strings.Compare(a.Path, b.Path) })
	return result
}

func (s *Service) localPromotionMembers(candidate localPromotionCandidate) ([]durabletx.Member, []durabletx.Path, error) {
	excludePath, err := localExcludePath(s.paths)
	if err != nil {
		return nil, nil, err
	}
	members := []durabletx.Member{{
		Kind: durabletx.Git, Reported: filepath.ToSlash(excludePath),
		Path: gitRelativePath(s.paths.GitCommonDir, excludePath), Content: candidate.excludeFinal,
	}}
	if !candidate.withSkills {
		members[0].Fence = candidate.exclude
	}
	if candidate.semantic {
		members = append(members, durabletx.Member{Kind: durabletx.Worktree, Reported: markerRelPath(), Path: markerRelPath(), Content: candidate.marker, Fence: candidate.fence})
	}
	for _, file := range candidate.files {
		members = append(members,
			durabletx.Member{Kind: durabletx.Worktree, Reported: file.Source, Path: file.Source, Delete: true},
			durabletx.Member{Kind: durabletx.Worktree, Reported: file.Destination, Path: file.Destination, Content: file.Content},
		)
	}
	consumed := []durabletx.Path{}
	for _, destination := range candidate.skills.Destinations {
		if destination.Present {
			consumed = append(consumed, durabletx.Path{Kind: durabletx.Worktree, Reported: destination.Path, Path: destination.Path})
		}
	}
	return members, consumed, nil
}

func (s *Service) validateLocalPromotionSource(candidate localPromotionCandidate) error {
	if validation, err := s.Validate(); err != nil || !validation.Valid {
		return fmt.Errorf("local promotion source changed or became invalid: %v %v", validation.Violations, err)
	}
	if err := s.validatePromotionDestinations(candidate.files); err != nil {
		return err
	}
	if err := s.validatePromotionMarkerDestination(); err != nil {
		return err
	}
	plan, err := s.planLocalSkills()
	if err != nil {
		return err
	}
	if err := s.validatePromotionSkills(plan, candidate.withSkills); err != nil {
		return err
	}
	if !reflect.DeepEqual(plan, candidate.skills) {
		return fmt.Errorf("local skill state changed while promotion candidate was built")
	}
	return nil
}

func (s *Service) validateLocalPromotionCandidate(candidate localPromotionCandidate) error {
	committed := s.localPromotionCommittedService()
	validation, err := committed.Validate()
	if err != nil || !validation.Valid {
		return fmt.Errorf("promoted candidate is not valid: %v %v", validation.Violations, err)
	}
	if candidate.withSkills {
		if err := s.validatePromotionSkillVisibility(candidate.skills); err != nil {
			return err
		}
	}
	for _, file := range candidate.files {
		if _, err := os.Lstat(filepath.Join(s.paths.RepoRoot, filepath.FromSlash(file.Source))); !os.IsNotExist(err) {
			return fmt.Errorf("local semantic source %s remains after promotion", file.Source)
		}
	}
	return nil
}

func (s *Service) validatePendingSkillPromotionSource(candidate localPromotionCandidate) error {
	if validation, err := s.Validate(); err != nil || !validation.Valid {
		return fmt.Errorf("pending skill source changed or became invalid: %v %v", validation.Violations, err)
	}
	plan, err := s.planPromotionSkills()
	if err != nil {
		return err
	}
	if err := s.validatePendingSkillPromotion(plan); err != nil {
		return err
	}
	if !reflect.DeepEqual(plan, candidate.skills) {
		return fmt.Errorf("pending skill state changed while promotion candidate was built")
	}
	return nil
}

func (s *Service) validatePendingSkillPromotionCandidate(candidate localPromotionCandidate) error {
	if validation, err := s.Validate(); err != nil || !validation.Valid {
		return fmt.Errorf("promoted pending skill state is not valid: %v %v", validation.Violations, err)
	}
	return s.validatePromotionSkillVisibility(candidate.skills)
}

func (s *Service) validatePendingSkillPromotionRecovery(snapshots []durabletx.Evidence) error {
	if s.paths.Storage.Mode != StorageCommitted {
		return fmt.Errorf("pending skill recovery requires committed storage")
	}
	if validation, err := s.Validate(); err != nil || !validation.Valid {
		return fmt.Errorf("recovered pending skill state is not valid: %v %v", validation.Violations, err)
	}
	byPath := make(map[string]durabletx.Evidence, len(snapshots))
	for _, snapshot := range snapshots {
		byPath[string(snapshot.Kind)+"\x00"+snapshot.Reported] = snapshot
	}
	plan, err := s.planPromotionSkills()
	if err != nil {
		return err
	}
	excludePath, err := localExcludePath(s.paths)
	if err != nil {
		return err
	}
	exclude, found := byPath[string(durabletx.Git)+"\x00"+filepath.ToSlash(excludePath)]
	if !found || exclude.CandidateSHA256 == "" {
		return fmt.Errorf("pending skill recovery does not publish the Git exclusion store")
	}
	if exclude.CurrentSHA256 != exclude.CandidateSHA256 {
		return s.validatePendingSkillPromotion(plan)
	}
	for _, destination := range plan.Destinations {
		if !destination.Present {
			return fmt.Errorf("recovered pending skill installation is incomplete")
		}
		snapshot, found := byPath[string(durabletx.Worktree)+"\x00"+destination.Path]
		if !found || snapshot.CandidateSHA256 != "" || snapshot.CurrentSHA256 != destination.Digest {
			return fmt.Errorf("pending skill recovery does not preserve %s", destination.Path)
		}
	}
	return s.validatePromotionSkillVisibility(plan)
}

func (s *Service) validatePromotionSkillVisibility(plan localSkillPlan) error {
	if err := s.validatePromotionSkillBytes(plan); err != nil {
		return err
	}
	for _, destination := range plan.Destinations {
		if !destination.Present {
			continue
		}
		ignored, err := gitIgnoredExact(s.paths.WorktreeRoot, destination.Path)
		if err != nil || ignored {
			return fmt.Errorf("local skill destination %s remains excluded after promotion", destination.Path)
		}
	}
	return nil
}

func (s *Service) validatePromotionSkillBytes(plan localSkillPlan) error {
	for _, destination := range plan.Destinations {
		entryType, alias, err := localSkillEntryType(s.paths.RepoRoot, destination.Path)
		if err != nil {
			return err
		}
		if !destination.Present {
			if entryType != "absent" || alias {
				return fmt.Errorf("local skill destination %s appeared during promotion", destination.Path)
			}
			continue
		}
		if entryType != "regular" || alias {
			return fmt.Errorf("local skill destination %s changed during promotion", destination.Path)
		}
		data, err := os.ReadFile(filepath.Join(s.paths.RepoRoot, filepath.FromSlash(destination.Path)))
		if err != nil || digestBytes(data) != destination.Digest {
			return fmt.Errorf("local skill destination %s changed during promotion", destination.Path)
		}
		for _, indexed := range []func(string, string) (bool, error){gitTracks, gitStaged} {
			present, err := indexed(s.paths.WorktreeRoot, destination.Path)
			if err != nil || present {
				return fmt.Errorf("local skill destination %s changed Git visibility during promotion", destination.Path)
			}
		}
	}
	return nil
}

func (s *Service) localPromotionCommittedService() *Service {
	paths := pathsFromDiscovery(s.paths.RepoRoot, LayoutConfig{LayoutVersion: layout2Version, SpecsDir: s.paths.LogicalSpecsDir, PlanningDir: s.paths.LogicalPlanningDir}, committedStorage(),
		gitContext{WorktreeRoot: s.paths.WorktreeRoot, GitDir: s.paths.GitDir, GitCommonDir: s.paths.GitCommonDir})
	return &Service{paths: paths, now: s.now}
}

func localPromotionPublished(evidence []durabletx.Evidence) bool {
	for _, item := range evidence {
		if item.Reported == markerRelPath() {
			continue
		}
		if item.CandidateSHA256 != "" && item.CurrentSHA256 == item.CandidateSHA256 {
			return true
		}
		if item.CandidateSHA256 == "" && strings.HasPrefix(item.Reported, localStorageRoot+"/") && item.CurrentSHA256 == "" {
			return true
		}
	}
	return false
}

func localPendingSkillPromotionPublished(evidence []durabletx.Evidence) bool {
	for _, item := range evidence {
		if item.Kind == durabletx.Git && item.CandidateSHA256 != "" && item.CurrentSHA256 == item.CandidateSHA256 {
			return true
		}
	}
	return false
}
