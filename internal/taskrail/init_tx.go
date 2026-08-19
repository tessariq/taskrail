package taskrail

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tessariq/taskrail/internal/repotx"
	"gopkg.in/yaml.v3"
)

// testHookInitValidated runs after an init/retrofit transaction snapshots its
// complete set and before the whole-set recheck. Tests use it to exercise lock
// ownership and concurrent edits.
var testHookInitValidated func()
var testHookInitPreviewBuilt func()
var testHookInitPathObserved func(string)

type initTransaction struct {
	published  []repotx.Candidate
	consumed   []repotx.Path
	expected   map[string]*string
	physical   map[string]bool
	installed  SkillInstallResult
	validation ValidationResult
}

func (s *Service) applyInitTransaction(in InitInput) (result InitResult, err error) {
	own, release, err := s.acquireWriterLock("init", nil)
	if err != nil {
		return InitResult{}, err
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil && err == nil {
			err = releaseErr
		}
	}()

	markerOriginal, cfg, hasMarker, err := s.readStableInitMarker()
	if err != nil {
		return InitResult{}, err
	}
	plan := s.planInit(cfg, hasMarker, true)
	result, err = s.reportInit(plan)
	if err != nil {
		return InitResult{}, err
	}
	tx, err := s.buildInitTransaction(plan, in, markerOriginal)
	if err != nil {
		return InitResult{}, err
	}
	if len(tx.published) > 0 {
		if err := s.commitInitTransaction(own, "init", tx); err != nil {
			return InitResult{}, err
		}
	}
	result.SkillInstall = tx.installed
	result.Skills, result.SkillExclusions, err = s.skillReportForInput(in, tx.installed)
	if err != nil {
		return InitResult{}, err
	}
	if plan.validates {
		result.Validation = &tx.validation
	}
	return result, nil
}

func (s *Service) applyRetrofitTransaction(input RetrofitInput) (result RetrofitResult, err error) {
	own, release, err := s.acquireWriterLock("retrofit", nil)
	if err != nil {
		return RetrofitResult{}, err
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil && err == nil {
			err = releaseErr
		}
	}()

	markerOriginal, _, hasMarker, err := s.readStableInitMarker()
	if err != nil {
		return RetrofitResult{}, err
	}
	if hasMarker {
		return RetrofitResult{}, WithMachineErrorCode(MachineCodeDestinationExists, fmt.Errorf(
			"repository is already Taskrail-managed (%s exists); use `taskrail init`", markerRelPath()))
	}
	bootstrap, err := s.retrofitBootstrap(input.NotesPath)
	if err != nil {
		return RetrofitResult{}, err
	}
	mapping := s.detectRetrofit()
	pending, err := s.pendingLayoutChanges()
	if err != nil {
		return RetrofitResult{}, err
	}
	plan := initPlan{marker: defaultLayoutConfig(), createsLayout: true, scaffolds: true, writesMarker: true, validates: true, applied: true}
	tx, err := s.buildInitTransaction(plan, InitInput{}, markerOriginal)
	if err != nil {
		return RetrofitResult{}, err
	}
	if strings.TrimSpace(input.NotesPath) != "" {
		if err := s.addInitConsumedFile(&tx, input.NotesPath); err != nil {
			return RetrofitResult{}, err
		}
	}
	if err := s.commitInitTransaction(own, "retrofit", tx); err != nil {
		return RetrofitResult{}, err
	}
	return RetrofitResult{
		Applied: true, Mapping: mapping, Bootstrap: bootstrap,
		Changes: append(pending, markerWriteChange()), Validation: &tx.validation,
	}, nil
}

func (s *Service) buildInitTransaction(plan initPlan, in InitInput, markerOriginal []byte) (initTransaction, error) {
	tx := initTransaction{expected: make(map[string]*string), physical: make(map[string]bool)}
	add := func(kind repotx.PathKind, reported, physical string, original, content []byte, publish bool, priority int) error {
		physical = filepath.Clean(physical)
		if tx.physical[physical] {
			return nil
		}
		reported = filepath.ToSlash(reported)
		key := string(kind) + "\x00" + reported
		if _, exists := tx.expected[key]; exists {
			return nil
		}
		tx.expected[key] = digestOptional(original)
		tx.physical[physical] = true
		p := repotx.Path{Kind: kind, Reported: reported, Physical: physical}
		if publish {
			tx.published = append(tx.published, repotx.Candidate{Path: p, Content: content, PublishPriority: priority})
		} else {
			tx.consumed = append(tx.consumed, p)
		}
		return nil
	}

	markerBytes, err := yaml.Marshal(plan.marker)
	if err != nil {
		return tx, fmt.Errorf("marshal layout marker: %w", err)
	}
	if err := add(repotx.Worktree, markerRelPath(), filepath.Join(s.paths.RepoRoot, taskrailConfigDir, taskrailConfigFile), markerOriginal, markerBytes, plan.writesMarker, 0); err != nil {
		return tx, err
	}
	for _, file := range s.layoutFiles() {
		var content []byte
		switch {
		case file.starter != nil:
			content = []byte(file.starter())
		case file.kind == writeKindNote:
			content = []byte(starterNotes())
		case file.kind == writeKindState:
			state := starterState(s.now())
			content, err = marshalFrontmatter(state.Frontmatter, state.Body)
			if err != nil {
				return tx, err
			}
		}
		original, readErr := readInitFile(file.physical)
		if readErr != nil {
			return tx, fmt.Errorf("inspect %s: %w", file.logical, readErr)
		}
		if testHookInitPathObserved != nil {
			testHookInitPathObserved(file.logical)
		}
		publish := plan.scaffolds && original == nil
		if err := add(repotx.Managed, file.logical, file.physical, original, content, publish, 0); err != nil {
			return tx, err
		}
	}
	if in.WithSkills {
		if err := s.addSkillCandidates(&tx, in.SkillVersion, in.ForceSkills, add); err != nil {
			return tx, err
		}
	}
	for _, mapping := range plan.mapping {
		if err := s.addInitConsumedTree(&tx, filepath.Join(s.paths.RepoRoot, mapping.Source)); err != nil {
			return tx, err
		}
	}
	if err := s.validateInitTransactionCandidate(&tx, plan.marker); err != nil {
		return tx, err
	}
	slices.SortFunc(tx.published, func(a, b repotx.Candidate) int { return strings.Compare(a.Reported, b.Reported) })
	slices.SortFunc(tx.consumed, func(a, b repotx.Path) int { return strings.Compare(a.Reported, b.Reported) })
	return tx, nil
}

func (s *Service) addSkillCandidates(tx *initTransaction, version string, force bool, add func(repotx.PathKind, string, string, []byte, []byte, bool, int) error) error {
	files, err := packagedSkillFiles()
	if err != nil {
		return err
	}
	for _, rel := range files {
		packaged, err := shippableSkillsFS.ReadFile(path.Join(shippableSkillsRoot, rel))
		if err != nil {
			return err
		}
		if err := validateAgentSkill(packaged); err != nil {
			return err
		}
		candidate, err := stampSkillVersion(packaged, version)
		if err != nil {
			return err
		}
		for _, target := range shippableSkillTargets {
			physical := filepath.Join(s.paths.RepoRoot, target, filepath.FromSlash(rel))
			reported := filepath.ToSlash(relPath(s.paths.RepoRoot, physical))
			existing, readErr := readInitFile(physical)
			switch {
			case readErr != nil:
				return fmt.Errorf("read %s: %w", reported, readErr)
			case existing == nil:
				tx.installed.Written = append(tx.installed.Written, reported)
				if err := add(repotx.Worktree, reported, physical, nil, candidate, true, 0); err != nil {
					return err
				}
			default:
				if _, err := skillVersionOf(existing); err != nil {
					return fmt.Errorf("read skill marker %s: %w", reported, err)
				}
				publish := force && string(existing) != string(candidate)
				if publish {
					tx.installed.Overwritten = append(tx.installed.Overwritten, reported)
					backup, err := s.backupPath(physical)
					if err != nil {
						return err
					}
					backupReported := filepath.ToSlash(relPath(s.paths.RepoRoot, backup))
					tx.installed.BackedUp = append(tx.installed.BackedUp, backupReported)
					if err := add(repotx.Worktree, backupReported, backup, nil, existing, true, 0); err != nil {
						return err
					}
				}
				if err := add(repotx.Worktree, reported, physical, existing, candidate, publish, 1); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *Service) addInitConsumedFile(tx *initTransaction, source string) error {
	physical := source
	if !filepath.IsAbs(physical) {
		physical = filepath.Join(s.paths.RepoRoot, physical)
	}
	physical = filepath.Clean(physical)
	relative, err := filepath.Rel(s.paths.RepoRoot, physical)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return WithMachineErrorCode(MachineCodePathBlocked,
			fmt.Errorf("retrofit notes source must be within the repository"))
	}
	if tx.physical[physical] {
		return nil
	}
	reported := filepath.ToSlash(relPath(s.paths.RepoRoot, physical))
	key := string(repotx.Worktree) + "\x00" + reported
	if _, exists := tx.expected[key]; exists {
		return nil
	}
	data, err := readInitFile(physical)
	if err != nil {
		return err
	}
	tx.expected[key] = digestOptional(data)
	tx.physical[physical] = true
	tx.consumed = append(tx.consumed, repotx.Path{Kind: repotx.Worktree, Reported: reported, Physical: physical})
	return nil
}

func (s *Service) addInitConsumedTree(tx *initTransaction, root string) error {
	return filepath.WalkDir(root, func(name string, entry os.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("inspect %s: is not a regular file", relPath(s.paths.RepoRoot, name))
		}
		return s.addInitConsumedFile(tx, name)
	})
}

func (s *Service) commitInitTransaction(own repotx.Ownership, command string, tx initTransaction) error {
	if err := os.MkdirAll(s.paths.TasksDir, 0o755); err != nil {
		return fmt.Errorf("create tasks directory: %w", err)
	}
	_, err := repotx.Commit(context.Background(), own, repotx.Request{
		Command: command, Consumed: tx.consumed, Published: tx.published,
		Validate: func(snapshots []repotx.Snapshot) error {
			for _, snapshot := range snapshots {
				key := string(snapshot.Kind) + "\x00" + snapshot.Path
				if !sameOptionalDigest(snapshot.OriginalSHA256, tx.expected[key]) {
					return fmt.Errorf("%s changed while the candidate was built", snapshot.Path)
				}
			}
			if testHookInitValidated != nil {
				testHookInitValidated()
			}
			return nil
		},
	})
	if err != nil {
		return writerTransactionError(err)
	}
	return nil
}

func (s *Service) skillReportForInput(in InitInput, installed SkillInstallResult) ([]InitSkill, []InitSkillExclusion, error) {
	if !in.WithSkills {
		return []InitSkill{}, []InitSkillExclusion{}, nil
	}
	return s.skillReport(installed)
}

func readInitFile(name string) ([]byte, error) {
	info, err := os.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("is not a regular file")
	}
	return os.ReadFile(name)
}

func digestOptional(data []byte) *string {
	if data == nil {
		return nil
	}
	digest := digestBytes(data)
	return &digest
}

func sameOptionalDigest(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (s *Service) readStableInitMarker() ([]byte, LayoutConfig, bool, error) {
	markerPath := filepath.Join(s.paths.RepoRoot, taskrailConfigDir, taskrailConfigFile)
	data, err := readInitFile(markerPath)
	if err != nil {
		return nil, LayoutConfig{}, false, err
	}
	if data == nil {
		return nil, LayoutConfig{}, false, nil
	}
	cfg := defaultLayoutConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, LayoutConfig{}, false, fmt.Errorf("parse layout marker %s: %w", markerRelPath(), err)
	}
	if err := ensureSupportedLayoutVersion(cfg); err != nil {
		return nil, LayoutConfig{}, false, err
	}
	return data, cfg, true, nil
}
