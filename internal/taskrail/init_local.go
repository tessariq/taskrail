package taskrail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/tessariq/taskrail/internal/durabletx"
	"github.com/tessariq/taskrail/internal/repolock"
	"gopkg.in/yaml.v3"
)

const localInitCommand = "init"

const localExcludeBegin = "# taskrail local planning begin"
const localExcludeEnd = "# taskrail local planning end"

var testHookLocalTasksCreated func() error

type localOrigin struct {
	SchemaVersion int     `json:"schema_version"`
	WorktreeRoot  string  `json:"worktree_root"`
	GitCommonDir  string  `json:"git_common_dir"`
	Branch        *string `json:"branch"`
	Head          *string `json:"head"`
	InitializedAt string  `json:"initialized_at"`
}

// initLocal is intentionally a fresh-layout operation. A repository that has
// already chosen a marker must use its discovered mode rather than converting
// committed planning in place.
func (s *Service) initLocal(in InitInput) (InitResult, error) {
	if err := rejectUpgradeOnlyInputs(in); err != nil {
		return InitResult{}, err
	}
	if in.WithSkills || in.ForceSkills {
		return InitResult{}, invalidArgumentsf("init --local does not install skills; run init --with-skills after local initialization")
	}
	if _, found, err := readMarker(s.paths.RepoRoot); err != nil {
		return InitResult{}, err
	} else if found {
		return InitResult{}, WithMachineErrorCode(MachineCodeDestinationExists,
			fmt.Errorf("repository already has %s", markerRelPath()))
	}
	git, err := discoverGitWorktree(s.paths.RepoRoot)
	if err != nil {
		return InitResult{}, err
	}
	if git.WorktreeRoot == "" {
		return InitResult{}, WithMachineErrorCode(MachineCodeUnsupported, fmt.Errorf("init --local requires a Git worktree"))
	}
	paths := pathsFromDiscovery(s.paths.RepoRoot, defaultLayoutConfig(), localStorage(), git)
	if err := validateDiscoveredPaths(paths); err != nil {
		return InitResult{}, WithMachineErrorCode(MachineCodePathBlocked, err)
	}
	local := *s
	local.paths = paths
	return local.applyLocalInit()
}

func (s *Service) applyLocalInit() (result InitResult, err error) {
	excludePath, err := localExcludePath(s.paths)
	if err != nil {
		return InitResult{}, err
	}
	excludeOriginal, err := readInitFile(excludePath)
	if err != nil {
		return InitResult{}, fmt.Errorf("inspect Git exclusion: %w", err)
	}
	exclude, err := localExcludeCandidate(excludeOriginal)
	if err != nil {
		return InitResult{}, err
	}
	if err := s.preflightLocalPaths(); err != nil {
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
	// Rebuild the mutable Git and filesystem observations under the writer lock.
	// The first pass above rejects obvious unsafe repositories before lock work;
	// this pass is the candidate that publication is allowed to trust.
	excludePath, err = localExcludePath(s.paths)
	if err != nil {
		return InitResult{}, err
	}
	excludeOriginal, err = readInitFile(excludePath)
	if err != nil {
		return InitResult{}, fmt.Errorf("inspect Git exclusion: %w", err)
	}
	exclude, err = localExcludeCandidate(excludeOriginal)
	if err != nil {
		return InitResult{}, err
	}
	if err := s.preflightLocalPaths(); err != nil {
		return InitResult{}, err
	}
	// durabletx binds managed members at the storage root. Create the empty root
	// only after all refusal checks; any handled publication failure removes it
	// again so callers never retain a partial local scaffold.
	if err := os.MkdirAll(s.paths.StorageRoot, 0o755); err != nil {
		return InitResult{}, fmt.Errorf("create local storage root: %w", err)
	}
	committed := false
	defer func() {
		if !committed && err != nil {
			s.cleanupEmptyLocalInitDirectories()
		}
	}()
	// The state and notes publications below durably sync PlanningDir, which also
	// makes this pre-created empty child's namespace entry durable. Creating it
	// before durabletx keeps every normal reader and writer usable immediately;
	// the failure cleanup removes only known-empty directories and never recurses.
	if err := os.MkdirAll(s.paths.TasksDir, 0o755); err != nil {
		return InitResult{}, fmt.Errorf("create local tasks directory: %w", err)
	}
	if testHookLocalTasksCreated != nil {
		if err := testHookLocalTasksCreated(); err != nil {
			return InitResult{}, err
		}
	}

	marker := Layout2Config{
		LayoutVersion: layout2Version, SpecsDir: defaultSpecsDir, PlanningDir: defaultPlanningDir,
		StorageMode: StorageLocal, ImplementationReviewMaxRounds: 1,
	}
	markerBytes, err := yaml.Marshal(marker)
	if err != nil {
		return InitResult{}, err
	}
	origin, err := localOriginBytes(s.paths, s.now)
	if err != nil {
		return InitResult{}, err
	}
	members := []durabletx.Member{
		{Kind: durabletx.Git, Reported: filepath.ToSlash(excludePath), Path: gitRelativePath(s.paths.GitCommonDir, excludePath), Content: exclude, PreSemantic: true},
		{Kind: durabletx.Worktree, Reported: markerRelPath(), Path: markerRelPath(), Content: markerBytes},
		{Kind: durabletx.Worktree, Reported: filepath.ToSlash(relPath(s.paths.RepoRoot, filepath.Join(s.paths.RuntimeDir, "origin.json"))), Path: filepath.ToSlash(relPath(s.paths.RepoRoot, filepath.Join(s.paths.RuntimeDir, "origin.json"))), Content: origin},
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
				return InitResult{}, err
			}
		}
		members = append(members, durabletx.Member{Kind: durabletx.Managed, Reported: file.logical, Path: file.logical, Content: content})
	}
	expected := map[string]string{string(durabletx.Git) + "\x00" + filepath.ToSlash(excludePath): digestBytes(excludeOriginal)}
	for _, member := range members[1:] {
		expected[string(member.Kind)+"\x00"+member.Reported] = ""
	}
	validate := func(evidence []durabletx.Evidence) error {
		for _, item := range evidence {
			if want, ok := expected[string(item.Kind)+"\x00"+item.Reported]; ok && item.OriginalSHA256 != want {
				return fmt.Errorf("%s changed after local-init preflight", item.Reported)
			}
		}
		return s.verifyLocalIgnored()
	}
	if _, err := durabletx.Run(context.Background(), lock, s.paths.LockRepository(), durabletx.Request{
		Command:                  localInitCommand,
		Members:                  members,
		Validate:                 validate,
		ValidateBeforeCandidates: validate,
	}); err != nil {
		return InitResult{}, s.mapMigrationFailure(transactionID, err)
	}
	committed = true

	digest, err := markerDigestBytes(markerBytes)
	if err != nil {
		return InitResult{}, err
	}
	result = InitResult{
		Outcome: InitCreated, ToVersion: layout2Version, Applied: true, StorageMode: string(StorageLocal),
		Config: InitConfig{Path: markerRelPath(), Action: configActionCreate, CandidateSHA256: digest},
		Writes: s.localInitWrites(), Notes: s.initNotes(true, false, nil), Skills: []InitSkill{}, SkillExclusions: []InitSkillExclusion{}, ContinuationNotes: []string{},
		Validation: &ValidationResult{Valid: true, Violations: []string{}},
	}
	return result, nil
}

func (s *Service) cleanupEmptyLocalInitDirectories() {
	for _, directory := range []string{
		s.paths.TasksDir,
		s.paths.PlanningDir,
		s.paths.SpecsDir,
		s.paths.RuntimeDir,
		s.paths.StorageRoot,
		filepath.Join(s.paths.RepoRoot, taskrailConfigDir),
	} {
		_ = os.Remove(directory)
	}
}

func (s *Service) localInitWrites() []WriteEntry {
	writes := []WriteEntry{{Path: markerRelPath(), Kind: writeKindConfig, Action: writeActionCreate}}
	for _, file := range s.layoutFiles() {
		writes = append(writes, WriteEntry{Path: file.logical, Kind: file.kind, Action: writeActionCreate})
	}
	slices.SortFunc(writes, func(a, b WriteEntry) int { return strings.Compare(a.Path, b.Path) })
	return writes
}

func markerDigestBytes(data []byte) (string, error) {
	if _, err := decodeLayoutMarkerStrict(data); err != nil {
		return "", err
	}
	return digestBytes(data), nil
}

func localExcludePath(paths Paths) (string, error) {
	output, err := gitCommand(paths.WorktreeRoot, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(output)
	if value == "" {
		return "", fmt.Errorf("Git returned an empty exclusion path")
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(paths.WorktreeRoot, value)
	}
	value = filepath.Clean(value)
	rel, err := filepath.Rel(paths.GitCommonDir, value)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("effective Git exclusion %s is outside the shared Git directory", value))
	}
	return value, nil
}

func gitRelativePath(root, target string) string {
	rel, _ := filepath.Rel(root, target)
	return filepath.ToSlash(rel)
}

func localExcludeCandidate(original []byte) ([]byte, error) {
	text := strings.TrimSuffix(string(original), "\n")
	if strings.Contains(text, localExcludeBegin) || strings.Contains(text, localExcludeEnd) || strings.Contains(text, ".taskrail/local") {
		return nil, WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("Git exclusion already contains a Taskrail local-storage entry"))
	}
	if text != "" {
		text += "\n"
	}
	return []byte(text + localExcludeBegin + "\n.taskrail/config.yml\n.taskrail/local/\n" + localExcludeEnd + "\n"), nil
}

func (s *Service) preflightLocalPaths() error {
	if entries, err := os.ReadDir(s.paths.StorageRoot); err == nil {
		if len(entries) != 0 {
			return WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("local storage root %s already exists", localStorageRoot))
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	for _, physical := range []string{s.paths.SpecsDir, s.paths.PlanningDir, filepath.Join(s.paths.RuntimeDir, "origin.json")} {
		if _, err := os.Lstat(physical); !os.IsNotExist(err) {
			if err == nil {
				return WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("local destination %s already exists", relPath(s.paths.RepoRoot, physical)))
			}
			return err
		}
	}
	for _, path := range []string{localStorageRoot, markerRelPath()} {
		if output, err := gitCommand(s.paths.WorktreeRoot, "ls-files", "--error-unmatch", "--", path); err == nil && strings.TrimSpace(output) != "" {
			return WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("Git tracks local destination %s", path))
		}
		if _, err := gitCommand(s.paths.WorktreeRoot, "diff", "--cached", "--quiet", "--", path); err != nil {
			return WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("Git index contains local destination %s", path))
		}
	}
	return nil
}

func (s *Service) verifyLocalIgnored() error {
	excludePath, err := localExcludePath(s.paths)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(excludePath)
	if err != nil {
		return err
	}
	if !strings.Contains(string(data), localExcludeBegin) {
		if _, err := os.Lstat(s.paths.ConfigFile); errors.Is(err, os.ErrNotExist) {
			// Local initialization validates its unpublished candidate before the
			// exclusion member lands. A discovered local repository never has this
			// exemption: it must prove its managed paths are ignored.
			return nil
		}
		return fmt.Errorf("Git exclusion is missing the Taskrail local-storage entry")
	}
	for _, path := range []string{markerRelPath(), localStorageRoot, filepath.ToSlash(relPath(s.paths.RepoRoot, s.paths.RuntimeDir))} {
		if _, err := gitCommand(s.paths.WorktreeRoot, "check-ignore", "-q", "--no-index", path); err != nil {
			return fmt.Errorf("local destination %s is not effectively ignored", path)
		}
		if output, err := gitCommand(s.paths.WorktreeRoot, "ls-files", "--error-unmatch", "--", path); err == nil && strings.TrimSpace(output) != "" {
			return fmt.Errorf("local destination %s is tracked", path)
		}
		if _, err := gitCommand(s.paths.WorktreeRoot, "diff", "--cached", "--quiet", "--", path); err != nil {
			return fmt.Errorf("local destination %s is staged", path)
		}
	}
	return nil
}

func localOriginBytes(paths Paths, now func() time.Time) ([]byte, error) {
	branch := gitNullable(paths.WorktreeRoot, "symbolic-ref", "--quiet", "--short", "HEAD")
	head := gitNullable(paths.WorktreeRoot, "rev-parse", "--verify", "HEAD")
	return json.Marshal(localOrigin{SchemaVersion: 1, WorktreeRoot: paths.WorktreeRoot, GitCommonDir: paths.GitCommonDir, Branch: branch, Head: head, InitializedAt: now().UTC().Format(time.RFC3339)})
}

func gitNullable(root string, args ...string) *string {
	output, err := gitCommand(root, args...)
	value := strings.TrimSpace(output)
	if err != nil || value == "" {
		return nil
	}
	return &value
}

func gitCommand(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
