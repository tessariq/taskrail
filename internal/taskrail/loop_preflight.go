package taskrail

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tessariq/taskrail/internal/durablefs"
	"github.com/tessariq/taskrail/internal/repolock"
)

// LoopInvocation is the repository-independent portion of one loop request.
// ParseLoopInvocation validates it before a caller discovers a repository.
type LoopInvocation struct {
	DryRun                    bool
	MaxIterations             int
	MaxReviewRounds           *int
	Timeout                   *time.Duration
	AllowPromptOverrideSHA256 string
	ResultFile                string
	Child                     []string
}

// LoopPreflightSnapshot freezes the byte and Git inputs later loop stages use.
// It deliberately excludes selection, prompt authorization, and child launch.
type LoopPreflightSnapshot struct {
	invocation LoopInvocation
	inputs     map[string][]byte
	git        LoopGitSnapshot
	storage    LoopStorageSnapshot
	review     LoopReviewSnapshot
	lock       LoopLockSnapshot
	rootRefs   map[string][]byte
}

type LoopGitSnapshot struct {
	Head     string
	Ref      string
	Branch   string
	Clean    bool
	Detached bool
	Status   string
	Refs     map[string]string
	Index    []byte
}

type LoopStorageSnapshot struct {
	Mode string
	Root string
}

type LoopReviewSnapshot struct {
	ConfiguredMaxRounds             int
	EffectiveMaxRounds              int
	Source                          string
	MaxReviewersPerRound            int
	FinalDiffReviewRequiredOnChange bool
}

type LoopLockSnapshot struct {
	Available bool
	Owner     string
}

func (s LoopPreflightSnapshot) Invocation() LoopInvocation { return cloneLoopInvocation(s.invocation) }

func (s LoopPreflightSnapshot) Inputs() map[string][]byte { return cloneLoopBytes(s.inputs) }

func (s LoopPreflightSnapshot) Git() LoopGitSnapshot {
	out := s.git
	out.Refs = mapsClone(s.git.Refs)
	out.Index = append([]byte{}, s.git.Index...)
	return out
}

func (s LoopPreflightSnapshot) Storage() LoopStorageSnapshot { return s.storage }

func (s LoopPreflightSnapshot) Review() LoopReviewSnapshot { return s.review }

func (s LoopPreflightSnapshot) Lock() LoopLockSnapshot { return s.lock }

func (s LoopPreflightSnapshot) RootRefs() map[string][]byte { return cloneLoopBytes(s.rootRefs) }

var loopRootRefName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// ParseLoopInvocation accepts only the base loop flags. Later stages add their
// own options around this parser rather than weakening its delimiter boundary.
func ParseLoopInvocation(args []string) (LoopInvocation, error) {
	invocation := LoopInvocation{MaxIterations: 1}
	dash := -1
	for i, arg := range args {
		if arg == "--" {
			if dash != -1 {
				return LoopInvocation{}, invalidArgumentsf("loop accepts exactly one -- delimiter")
			}
			dash = i
		}
	}
	if dash >= 0 {
		invocation.Child = append([]string{}, args[dash+1:]...)
		args = args[:dash]
	}

	seen := make(map[string]bool)
	for i := 0; i < len(args); i++ {
		name, value, inline := strings.Cut(args[i], "=")
		if !strings.HasPrefix(name, "--") {
			return LoopInvocation{}, invalidArgumentsf("loop argument %q must follow --", args[i])
		}
		if seen[name] {
			return LoopInvocation{}, invalidArgumentsf("loop flag %s is repeated", name)
		}
		seen[name] = true
		switch name {
		case "--dry-run", "--json":
			if inline {
				return LoopInvocation{}, invalidArgumentsf("loop flag %s does not take a value", name)
			}
			if name == "--dry-run" {
				invocation.DryRun = true
			}
		case "--max-iterations", "--max-review-rounds", "--timeout", "--allow-prompt-override-sha256", "--result-file":
			if !inline {
				i++
				if i == len(args) {
					return LoopInvocation{}, invalidArgumentsf("loop flag %s requires a value", name)
				}
				value = args[i]
			}
			switch name {
			case "--max-iterations":
				n, err := positiveLoopInt(name, value)
				if err != nil {
					return LoopInvocation{}, err
				}
				invocation.MaxIterations = n
			case "--max-review-rounds":
				n, err := positiveLoopInt(name, value)
				if err != nil {
					return LoopInvocation{}, err
				}
				if n > 2 {
					return LoopInvocation{}, invalidArgumentsf("--max-review-rounds must be between 1 and 2")
				}
				invocation.MaxReviewRounds = &n
			case "--timeout":
				duration, err := time.ParseDuration(value)
				if err != nil || duration <= 0 {
					return LoopInvocation{}, invalidArgumentsf("--timeout must be a positive Go duration")
				}
				invocation.Timeout = &duration
			case "--allow-prompt-override-sha256":
				invocation.AllowPromptOverrideSHA256 = value
			case "--result-file":
				if value == "" {
					return LoopInvocation{}, invalidArgumentsf("--result-file requires a non-empty path")
				}
				invocation.ResultFile = value
			}
		default:
			return LoopInvocation{}, invalidArgumentsf("unsupported loop flag %s", name)
		}
	}
	if invocation.DryRun {
		if invocation.ResultFile != "" {
			return LoopInvocation{}, invalidArgumentsf("loop --dry-run does not support --result-file")
		}
		if dash >= 0 {
			return LoopInvocation{}, invalidArgumentsf("loop --dry-run does not accept a child command")
		}
		return invocation, nil
	}
	if dash < 0 || len(invocation.Child) == 0 {
		return LoopInvocation{}, invalidArgumentsf("loop execution requires a child command after --")
	}
	if seen["--json"] {
		return LoopInvocation{}, invalidArgumentsf("loop execution does not support --json")
	}
	return invocation, nil
}

func positiveLoopInt(name, value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, invalidArgumentsf("%s must be a positive integer", name)
	}
	return n, nil
}

// LoopPreflight captures the repository inputs common to dry-run and execution.
// It takes no lock and does not select work, so it cannot change managed state.
func (s *Service) LoopPreflight(invocation LoopInvocation) (LoopPreflightSnapshot, error) {
	if err := validateLoopInvocation(invocation); err != nil {
		return LoopPreflightSnapshot{}, err
	}
	if s.paths.WorktreeRoot == "" || s.paths.GitDir == "" || s.paths.GitCommonDir == "" {
		return LoopPreflightSnapshot{}, WithMachineErrorCode(MachineCodeGitState, errors.New("loop requires a Git worktree"))
	}
	if sourceCheckout(s.paths.ManagedRoot) {
		return LoopPreflightSnapshot{}, WithMachineErrorCode(MachineCodeUnsupported, errors.New("loop execution in the Taskrail source checkout is unsupported"))
	}
	git, err := loopGitSnapshot(s.paths.WorktreeRoot, s.paths.GitDir)
	if err != nil {
		return LoopPreflightSnapshot{}, err
	}
	if !git.Clean {
		return LoopPreflightSnapshot{}, WithMachineErrorCode(MachineCodeGitState, errors.New("loop requires a clean Git worktree"))
	}
	if git.Detached || git.Branch == "" {
		return LoopPreflightSnapshot{}, WithMachineErrorCode(MachineCodeGitState, errors.New("loop requires an attached HEAD"))
	}
	if validation, err := s.Validate(); err != nil {
		return LoopPreflightSnapshot{}, err
	} else if !validation.Valid {
		return LoopPreflightSnapshot{}, WithMachineErrorCode(MachineCodeValidationFailed, errors.New("loop requires a valid Taskrail repository"))
	}
	tasks, err := s.loadTasks()
	if err != nil {
		return LoopPreflightSnapshot{}, err
	}
	if len(inProgressTasks(tasks)) != 0 {
		return LoopPreflightSnapshot{}, WithMachineErrorCode(MachineCodeValidationFailed, errors.New("loop requires no in_progress task"))
	}
	if s.paths.Storage.Mode == StorageLocal {
		if err := s.verifyLocalIgnored(); err != nil {
			return LoopPreflightSnapshot{}, WithMachineErrorCode(MachineCodeRepositoryInvalid, err)
		}
	}
	lock, err := repolock.Inspect(s.paths.LockRepository())
	if err != nil {
		return LoopPreflightSnapshot{}, WithMachineErrorCode(MachineCodeLockHeld, err)
	}
	if lock.Held {
		return LoopPreflightSnapshot{}, WithMachineErrorCode(MachineCodeLockHeld, errors.New("loop requires an available repository mutation lock"))
	}
	inputs, err := loopInputBytes(s.paths)
	if err != nil {
		return LoopPreflightSnapshot{}, err
	}
	rootRefs, err := loopRootRefCandidates(s.paths.GitDir, s.paths.GitCommonDir)
	if err != nil {
		return LoopPreflightSnapshot{}, err
	}
	configured, err := loopConfiguredReviewRounds(inputs[filepath.ToSlash(relPath(s.paths.RepoRoot, s.paths.ConfigFile))])
	if err != nil {
		return LoopPreflightSnapshot{}, err
	}
	review := LoopReviewSnapshot{ConfiguredMaxRounds: configured, EffectiveMaxRounds: configured, Source: "config", MaxReviewersPerRound: 3, FinalDiffReviewRequiredOnChange: true}
	if invocation.MaxReviewRounds != nil {
		review.EffectiveMaxRounds = *invocation.MaxReviewRounds
		review.Source = "flag"
	}
	return LoopPreflightSnapshot{invocation: cloneLoopInvocation(invocation), inputs: cloneLoopBytes(inputs), git: cloneLoopGit(git),
		storage: LoopStorageSnapshot{Mode: string(s.paths.Storage.Mode), Root: s.paths.Storage.Root},
		review:  review, lock: LoopLockSnapshot{Available: true}, rootRefs: cloneLoopBytes(rootRefs)}, nil
}

func validateLoopInvocation(invocation LoopInvocation) error {
	if invocation.MaxIterations <= 0 {
		return invalidArgumentsf("--max-iterations must be a positive integer")
	}
	if invocation.MaxReviewRounds != nil && (*invocation.MaxReviewRounds < 1 || *invocation.MaxReviewRounds > 2) {
		return invalidArgumentsf("--max-review-rounds must be between 1 and 2")
	}
	if invocation.Timeout != nil && *invocation.Timeout <= 0 {
		return invalidArgumentsf("--timeout must be a positive Go duration")
	}
	if invocation.DryRun && invocation.ResultFile != "" {
		return invalidArgumentsf("loop --dry-run does not support --result-file")
	}
	if !invocation.DryRun && len(invocation.Child) == 0 {
		return invalidArgumentsf("loop execution requires a child command after --")
	}
	return nil
}

func cloneLoopInvocation(in LoopInvocation) LoopInvocation {
	out := in
	out.Child = append([]string{}, in.Child...)
	if in.MaxReviewRounds != nil {
		n := *in.MaxReviewRounds
		out.MaxReviewRounds = &n
	}
	if in.Timeout != nil {
		duration := *in.Timeout
		out.Timeout = &duration
	}
	return out
}

func cloneLoopGit(in LoopGitSnapshot) LoopGitSnapshot {
	out := in
	out.Refs = mapsClone(in.Refs)
	out.Index = append([]byte{}, in.Index...)
	return out
}

func cloneLoopBytes(in map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(in))
	for key, value := range in {
		out[key] = append([]byte{}, value...)
	}
	return out
}

func mapsClone(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func loopGitSnapshot(root, gitDir string) (LoopGitSnapshot, error) {
	bare, err := gitCommand(root, "rev-parse", "--is-bare-repository")
	if err != nil || strings.TrimSpace(bare) != "false" {
		return LoopGitSnapshot{}, WithMachineErrorCode(MachineCodeGitState, errors.New("loop requires a non-bare Git worktree"))
	}
	head, err := gitCommand(root, "rev-parse", "--verify", "HEAD")
	if err != nil || strings.TrimSpace(head) == "" {
		return LoopGitSnapshot{}, WithMachineErrorCode(MachineCodeGitState, errors.New("loop requires a non-unborn HEAD"))
	}
	ref := gitNullable(root, "symbolic-ref", "--quiet", "HEAD")
	status, err := gitCommand(root, "status", "--porcelain=v1", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return LoopGitSnapshot{}, WithMachineErrorCode(MachineCodeGitState, fmt.Errorf("read Git status: %w", err))
	}
	refs, err := loopRefs(root)
	if err != nil {
		return LoopGitSnapshot{}, err
	}
	index, err := loopReadFile(gitDir, "index")
	if err != nil {
		return LoopGitSnapshot{}, WithMachineErrorCode(MachineCodeGitState, fmt.Errorf("snapshot Git index: %w", err))
	}
	snapshot := LoopGitSnapshot{Head: strings.TrimSpace(head), Clean: status == "", Status: status, Refs: refs, Index: index}
	if ref == nil {
		snapshot.Detached = true
	} else {
		snapshot.Ref = *ref
		snapshot.Branch = strings.TrimPrefix(*ref, "refs/heads/")
	}
	return snapshot, nil
}

func loopRefs(root string) (map[string]string, error) {
	output, err := gitCommand(root, "for-each-ref", "--format=%(refname)%00%(objectname)")
	if err != nil {
		return nil, WithMachineErrorCode(MachineCodeGitState, fmt.Errorf("enumerate Git refs: %w", err))
	}
	refs := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if line == "" {
			continue
		}
		name, object, ok := strings.Cut(line, "\x00")
		if !ok || name == "" || object == "" {
			return nil, WithMachineErrorCode(MachineCodeGitState, errors.New("Git returned a malformed ref"))
		}
		refs[name] = object
	}
	return refs, nil
}

func loopInputBytes(paths Paths) (map[string][]byte, error) {
	inputs := make(map[string][]byte)
	config := filepath.ToSlash(relPath(paths.RepoRoot, paths.ConfigFile))
	data, err := loopReadFile(paths.RepoRoot, config)
	if err != nil {
		return nil, WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("read loop input %s: %w", config, fsCause(err)))
	}
	inputs[config] = append([]byte{}, data...)
	for _, root := range []string{paths.SpecsDir, paths.PlanningDir, paths.PromptsDir} {
		rel := filepath.ToSlash(relPath(paths.RepoRoot, root))
		if root == paths.PromptsDir {
			if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
				continue
			}
		}
		if err := collectLoopInputTree(paths.RepoRoot, rel, inputs); err != nil {
			return nil, err
		}
	}
	return inputs, nil
}

func collectLoopInputTree(repo, root string, inputs map[string][]byte) error {
	tree, err := durablefs.ObserveTree(repo, root)
	if err != nil || !tree.Present {
		return WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("inspect loop input directory %s: %w", root, err))
	}
	for _, entry := range tree.Entries {
		if entry.Directory {
			continue
		}
		file := path.Join(root, entry.Path)
		data, err := loopReadFile(repo, file)
		if err != nil {
			return WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("read loop input %s: %w", file, err))
		}
		inputs[file] = data
	}
	return nil
}

func loopConfiguredReviewRounds(data []byte) (int, error) {
	marker, err := decodeLayoutMarkerStrict(data)
	if err != nil {
		return 0, WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("read loop layout: %w", err))
	}
	if marker.LayoutVersion != layout2Version {
		return 0, WithMachineErrorCode(MachineCodeUnsupported, errors.New("loop requires layout_version 2"))
	}
	return marker.ImplementationReviewMaxRounds, nil
}

func loopRootRefCandidates(gitDirs ...string) (map[string][]byte, error) {
	candidates := make(map[string][]byte)
	seen := make(map[string]bool)
	for _, dir := range gitDirs {
		if seen[dir] {
			continue
		}
		seen[dir] = true
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, WithMachineErrorCode(MachineCodeGitState, fmt.Errorf("read Git directory %s: %w", dir, fsCause(err)))
		}
		for _, entry := range entries {
			name := entry.Name()
			if name == "COMMIT_EDITMSG" || !loopRootRefName.MatchString(name) {
				continue
			}
			candidate := filepath.Join(dir, name)
			info, err := os.Lstat(candidate)
			if err != nil || !info.Mode().IsRegular() {
				return nil, WithMachineErrorCode(MachineCodeGitState, fmt.Errorf("Git root candidate %s is not regular", candidate))
			}
			data, err := loopReadFile(dir, name)
			if err != nil {
				return nil, WithMachineErrorCode(MachineCodeGitState, fmt.Errorf("read Git root candidate %s: %w", candidate, err))
			}
			candidates[candidate] = data
		}
	}
	return candidates, nil
}

func loopReadFile(root, file string) ([]byte, error) {
	data, _, err := durablefs.ReadFile(root, file, math.MaxInt64-1)
	if err != nil {
		return nil, err
	}
	return append([]byte{}, data...), nil
}

func sourceCheckout(root string) bool {
	for _, path := range []string{filepath.Join(root, "Taskfile.yml"), filepath.Join(root, "internal", "toolchain", "cmd", "freshcheck")} {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		if path == filepath.Join(root, "Taskfile.yml") && !info.Mode().IsRegular() {
			return false
		}
		if strings.HasSuffix(path, "freshcheck") && !info.IsDir() {
			return false
		}
	}
	return true
}
