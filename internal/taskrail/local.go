package taskrail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// LocalGitSnapshot is the nullable branch and commit pair captured from one Git
// worktree. It intentionally has no storage or lock identity fields.
type LocalGitSnapshot struct {
	Branch *string `json:"branch"`
	Head   *string `json:"head"`
}

type LocalExclusion struct {
	Path      string `json:"path"`
	Source    string `json:"source"`
	Effective bool   `json:"effective"`
}

type LocalViolation struct {
	Code    string  `json:"code"`
	Message string  `json:"message"`
	Path    *string `json:"path"`
}

// LocalStatusResult is the local storage inspection contract. Managed semantic
// paths remain logical; only physical operational roots are reported here.
type LocalStatusResult struct {
	Mode           string           `json:"mode"`
	StorageRoot    string           `json:"storage_root"`
	LogicalRoot    string           `json:"logical_root"`
	WorktreeRoot   string           `json:"worktree_root"`
	GitCommonDir   string           `json:"git_common_dir"`
	Origin         LocalGitSnapshot `json:"origin"`
	Current        LocalGitSnapshot `json:"current"`
	Drift          string           `json:"drift"`
	Exclusions     []LocalExclusion `json:"exclusions"`
	PromotionReady bool             `json:"promotion_ready"`
	Violations     []LocalViolation `json:"violations"`
}

// LocalPathResult reports the physical paths external producers may use beside
// the logical namespaces Taskrail continues to persist.
type LocalPathResult struct {
	Mode         string `json:"mode"`
	ConfigPath   string `json:"config_path"`
	StorageRoot  string `json:"storage_root"`
	SpecsDir     string `json:"specs_dir"`
	PlanningDir  string `json:"planning_dir"`
	PromptsDir   string `json:"prompts_dir"`
	ArtifactsDir string `json:"artifacts_dir"`
	RuntimeDir   string `json:"runtime_dir"`
}

type localInspectionSnapshot struct {
	marker  []byte
	origin  []byte
	exclude []byte
	current LocalGitSnapshot
}

// LocalStatus observes the initialized local context without creating any
// operational state. It rechecks every input before returning so callers never
// receive a report assembled from two repository snapshots.
func (s *Service) LocalStatus() (LocalStatusResult, error) {
	before, origin, excludePath, err := s.localInspectionSnapshot()
	if err != nil {
		return LocalStatusResult{}, err
	}
	exclusions, violations, err := s.localExclusions(string(before.exclude))
	if err != nil {
		return LocalStatusResult{}, err
	}
	result := LocalStatusResult{
		Mode: string(StorageLocal), StorageRoot: s.paths.Storage.Root,
		LogicalRoot: s.paths.ManagedRoot, WorktreeRoot: s.paths.WorktreeRoot,
		GitCommonDir: s.paths.GitCommonDir, Origin: LocalGitSnapshot{Branch: origin.Branch, Head: origin.Head},
		Current: before.current, Drift: s.localDrift(origin, before.current),
		Exclusions: exclusions, PromotionReady: len(violations) == 0, Violations: violations,
	}
	after, _, afterExcludePath, err := s.localInspectionSnapshot()
	if err != nil {
		return LocalStatusResult{}, err
	}
	if excludePath != afterExcludePath || !sameLocalInspection(before, after) {
		return LocalStatusResult{}, WithMachineErrorCode(MachineCodeRepositoryInvalid,
			fmt.Errorf("local inspection inputs changed before output"))
	}
	return result, nil
}

// LocalPath reports the active local mapping. Its physical values are read from
// the discovered context, never reconstructed from the logical namespaces.
func (s *Service) LocalPath() (LocalPathResult, error) {
	if err := s.requireLocalStorage(); err != nil {
		return LocalPathResult{}, err
	}
	before, err := os.ReadFile(s.paths.ConfigFile)
	if err != nil {
		return LocalPathResult{}, WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("read local layout marker: %w", err))
	}
	if err := s.validateLocalMarker(before); err != nil {
		return LocalPathResult{}, err
	}
	result := LocalPathResult{
		Mode: string(StorageLocal), ConfigPath: relPath(s.paths.RepoRoot, s.paths.ConfigFile),
		StorageRoot: relPath(s.paths.RepoRoot, s.paths.StorageRoot),
		SpecsDir:    s.paths.LogicalSpecsDir, PlanningDir: s.paths.LogicalPlanningDir,
		PromptsDir:   relPath(s.paths.RepoRoot, s.paths.PromptsDir),
		ArtifactsDir: relPath(s.paths.RepoRoot, s.paths.ArtifactsDir),
		RuntimeDir:   relPath(s.paths.RepoRoot, s.paths.RuntimeDir),
	}
	after, err := os.ReadFile(s.paths.ConfigFile)
	if err != nil {
		return LocalPathResult{}, WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("recheck local layout marker: %w", err))
	}
	if err := s.validateLocalMarker(after); err != nil {
		return LocalPathResult{}, err
	}
	if !bytes.Equal(before, after) {
		return LocalPathResult{}, WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("local layout marker changed before output"))
	}
	return result, nil
}

func (s *Service) requireLocalStorage() error {
	if err := s.paths.ensureStorageCapability(); err != nil {
		return err
	}
	if s.paths.Storage.Mode != StorageLocal {
		return WithMachineErrorCode(MachineCodeUnsupported, fmt.Errorf("local inspection requires initialized local storage"))
	}
	return nil
}

func (s *Service) localInspectionSnapshot() (localInspectionSnapshot, localOrigin, string, error) {
	if err := s.requireLocalStorage(); err != nil {
		return localInspectionSnapshot{}, localOrigin{}, "", err
	}
	marker, err := os.ReadFile(s.paths.ConfigFile)
	if err != nil {
		return localInspectionSnapshot{}, localOrigin{}, "", WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("read local layout marker: %w", err))
	}
	if err := s.validateLocalMarker(marker); err != nil {
		return localInspectionSnapshot{}, localOrigin{}, "", err
	}
	originPath := filepath.Join(s.paths.RuntimeDir, "origin.json")
	originBytes, err := os.ReadFile(originPath)
	if err != nil {
		return localInspectionSnapshot{}, localOrigin{}, "", WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("read local origin: %w", err))
	}
	origin, err := decodeLocalOrigin(originBytes)
	if err != nil {
		return localInspectionSnapshot{}, localOrigin{}, "", WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("decode local origin: %w", err))
	}
	if origin.WorktreeRoot != s.paths.WorktreeRoot || origin.GitCommonDir != s.paths.GitCommonDir {
		return localInspectionSnapshot{}, localOrigin{}, "", WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("local origin does not match the effective Git worktree"))
	}
	excludePath, err := localExcludePath(s.paths)
	if err != nil {
		return localInspectionSnapshot{}, localOrigin{}, "", WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("resolve local exclusion: %w", err))
	}
	exclude, err := os.ReadFile(excludePath)
	if err != nil {
		return localInspectionSnapshot{}, localOrigin{}, "", WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("read local exclusion: %w", err))
	}
	current := LocalGitSnapshot{
		Branch: gitNullable(s.paths.WorktreeRoot, "symbolic-ref", "--quiet", "--short", "HEAD"),
		Head:   gitNullable(s.paths.WorktreeRoot, "rev-parse", "--verify", "HEAD"),
	}
	return localInspectionSnapshot{marker: marker, origin: originBytes, exclude: exclude, current: current}, origin, excludePath, nil
}

func (s *Service) validateLocalMarker(data []byte) error {
	marker, err := decodeLayoutMarkerStrict(data)
	if err != nil {
		return WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("decode local layout marker: %w", err))
	}
	if marker.StorageMode != StorageLocal || marker.SpecsDir != s.paths.LogicalSpecsDir || marker.PlanningDir != s.paths.LogicalPlanningDir {
		return WithMachineErrorCode(MachineCodeUnsupported, fmt.Errorf("local inspection no longer matches the active storage context"))
	}
	return nil
}

func decodeLocalOrigin(data []byte) (localOrigin, error) {
	if err := checkDocumentFraming(data); err != nil {
		return localOrigin{}, err
	}
	var origin localOrigin
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&origin); err != nil {
		return localOrigin{}, err
	}
	if origin.SchemaVersion != 1 || origin.WorktreeRoot == "" || origin.GitCommonDir == "" || origin.InitializedAt == "" {
		return localOrigin{}, fmt.Errorf("origin must contain the schema-1 local origin fields")
	}
	if _, err := time.Parse(time.RFC3339, origin.InitializedAt); err != nil {
		return localOrigin{}, fmt.Errorf("invalid initialized_at: %w", err)
	}
	return origin, nil
}

func (s *Service) localExclusions(exclude string) ([]LocalExclusion, []LocalViolation, error) {
	managed := []string{markerRelPath(), localStorageRoot + "/"}
	installed, err := s.InstalledSkillVersions()
	if err != nil {
		return nil, nil, WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("inspect installed local skills: %w", err))
	}
	for _, skill := range installed {
		if skill.Version != "" || skill.MatchesPackage {
			managed = append(managed, filepath.ToSlash(filepath.Dir(skill.Path)))
		}
	}
	slices.Sort(managed)
	managed = slices.Compact(managed)
	exclusions := make([]LocalExclusion, 0, len(managed)+4)
	violations := []LocalViolation{}
	block, external := splitLocalExclusions(exclude)
	for _, item := range managed {
		effective := gitIgnored(s.paths.WorktreeRoot, item)
		exclusions = append(exclusions, LocalExclusion{Path: item, Source: "managed", Effective: effective})
		if !block[item] || !effective {
			path := item
			violations = append(violations, LocalViolation{Code: "local_exclusion_invalid", Message: "managed local path is not effectively excluded", Path: &path})
		}
	}
	for _, item := range external {
		exclusions = append(exclusions, LocalExclusion{Path: item, Source: "external", Effective: gitIgnored(s.paths.WorktreeRoot, item)})
	}
	slices.SortFunc(exclusions, func(a, b LocalExclusion) int { return strings.Compare(a.Path, b.Path) })
	return exclusions, violations, nil
}

func splitLocalExclusions(exclude string) (map[string]bool, []string) {
	managed := map[string]bool{}
	external := []string{}
	inManagedBlock := false
	for _, line := range strings.Split(exclude, "\n") {
		line = strings.TrimSpace(line)
		switch line {
		case localExcludeBegin:
			inManagedBlock = true
			continue
		case localExcludeEnd:
			inManagedBlock = false
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if inManagedBlock {
			managed[line] = true
			continue
		}
		external = append(external, line)
	}
	return managed, external
}

func gitIgnored(root, path string) bool {
	_, err := gitCommand(root, "check-ignore", "-q", "--no-index", path)
	return err == nil
}

func sameLocalInspection(a, b localInspectionSnapshot) bool {
	return bytes.Equal(a.marker, b.marker) && bytes.Equal(a.origin, b.origin) && bytes.Equal(a.exclude, b.exclude) &&
		nullableEqual(a.current.Branch, b.current.Branch) && nullableEqual(a.current.Head, b.current.Head)
}

func nullableEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func (s *Service) localDrift(origin localOrigin, current LocalGitSnapshot) string {
	if current.Head == nil {
		return "unborn"
	}
	if origin.Head == nil {
		return "unavailable"
	}
	if *origin.Head == *current.Head && nullableEqual(origin.Branch, current.Branch) {
		return "same"
	}
	if !nullableEqual(origin.Branch, current.Branch) {
		return "branch_changed"
	}
	if _, err := gitCommand(s.paths.WorktreeRoot, "merge-base", "--is-ancestor", *origin.Head, *current.Head); err == nil {
		return "descendant"
	}
	return "diverged"
}
