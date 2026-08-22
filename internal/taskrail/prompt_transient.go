package taskrail

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tessariq/taskrail/internal/durablefs"
)

const (
	PromptContextReviewPath = "REVIEW_PATH"
	PromptContextDraftPath  = "DRAFT_PATH"
	PromptContextTracePath  = "TRACE_PATH"
)

// TransientPromptPath is one physical output or staging path a prompt may name.
// ProposalType selects the one review-proposal subtree that owns the path.
type TransientPromptPath struct {
	Role         string
	ProposalType string
	Path         string
}

// TransientPromptPathAuthorization retains a read-only no-follow snapshot for
// publication to recheck after an external producer writes its proposal.
type TransientPromptPathAuthorization struct {
	Paths     []TransientPromptPath
	snapshots []transientPromptPathSnapshot
}

type transientPromptPathSnapshot struct {
	path      TransientPromptPath
	ancestors []durablefs.Identity
	identity  durablefs.Identity
}

// AuthorizeTransientPromptPaths admits only the three physical-path exceptions
// in the prompt contract. Managed prompt context stays logical and never enters
// this boundary.
func (s *Service) AuthorizeTransientPromptPaths(input []TransientPromptPath) (TransientPromptPathAuthorization, error) {
	if len(input) == 0 {
		return TransientPromptPathAuthorization{}, invalidArgumentsf("at least one transient prompt path is required")
	}
	paths := slices.Clone(input)
	slices.SortFunc(paths, func(a, b TransientPromptPath) int {
		if a.Role != b.Role {
			return strings.Compare(a.Role, b.Role)
		}
		return strings.Compare(a.Path, b.Path)
	})
	seenRoles := make(map[string]struct{}, len(paths))
	seenPaths := make(map[string]struct{}, len(paths))
	snapshots := make([]transientPromptPathSnapshot, 0, len(paths))
	for _, candidate := range paths {
		if _, exists := seenRoles[candidate.Role]; exists {
			return TransientPromptPathAuthorization{}, invalidArgumentsf("duplicate transient prompt context %q", candidate.Role)
		}
		if _, exists := seenPaths[candidate.Path]; exists {
			return TransientPromptPathAuthorization{}, invalidArgumentsf("transient prompt paths must be distinct")
		}
		for _, prior := range paths[:len(snapshots)] {
			if path.Dir(candidate.Path) == path.Dir(prior.Path) && samePortableName(path.Base(candidate.Path), path.Base(prior.Path)) {
				return TransientPromptPathAuthorization{}, WithMachineErrorCode(MachineCodePathBlocked,
					fmt.Errorf("transient prompt path %q aliases requested path %q", candidate.Path, prior.Path))
			}
		}
		seenRoles[candidate.Role] = struct{}{}
		seenPaths[candidate.Path] = struct{}{}

		tree, err := s.authorizeTransientPromptPath(candidate)
		if err != nil {
			return TransientPromptPathAuthorization{}, err
		}
		snapshots = append(snapshots, transientPromptPathSnapshot{path: candidate, ancestors: slices.Clone(tree.Ancestors), identity: tree.Identity})
	}
	return TransientPromptPathAuthorization{Paths: paths, snapshots: snapshots}, nil
}

// RecheckTransientPromptPaths proves the proposal directories still have the
// exact identities observed before prompt rendering. It intentionally repeats
// Git admission because staging or ignore rules can change after authorization.
func (s *Service) RecheckTransientPromptPaths(authorization TransientPromptPathAuthorization) error {
	if len(authorization.Paths) == 0 || len(authorization.Paths) != len(authorization.snapshots) {
		return invalidArgumentsf("invalid transient prompt path authorization")
	}
	for i, snapshot := range authorization.snapshots {
		if authorization.Paths[i] != snapshot.path {
			return invalidArgumentsf("transient prompt path authorization changed")
		}
		tree, err := s.authorizeTransientPromptPath(snapshot.path)
		if err != nil {
			return err
		}
		if !sameTransientPromptAncestors(snapshot, tree) {
			return WithMachineErrorCode(MachineCodePathBlocked,
				fmt.Errorf("transient prompt proposal path %q changed after authorization", snapshot.path.Path))
		}
	}
	return nil
}

func sameTransientPromptAncestors(snapshot transientPromptPathSnapshot, current durablefs.TreeSnapshot) bool {
	return current.Present && snapshot.identity == current.Identity && slices.Equal(snapshot.ancestors, current.Ancestors)
}

func (s *Service) authorizeTransientPromptPath(candidate TransientPromptPath) (durablefs.TreeSnapshot, error) {
	if !validTransientPromptProposalType(candidate.Role, candidate.ProposalType) {
		return durablefs.TreeSnapshot{}, invalidArgumentsf("transient prompt context %q does not permit proposal type %q", candidate.Role, candidate.ProposalType)
	}
	artifacts, err := transientArtifactsPath(s.paths)
	if err != nil {
		return durablefs.TreeSnapshot{}, WithMachineErrorCode(MachineCodePathBlocked, err)
	}
	if !canonicalTransientPromptPath(candidate.Path) {
		return durablefs.TreeSnapshot{}, WithMachineErrorCode(MachineCodePathBlocked,
			fmt.Errorf("transient prompt path %q is not a canonical repository-relative path", candidate.Path))
	}
	prefix := path.Join(artifacts, "review-proposals", candidate.ProposalType) + "/"
	relative := strings.TrimPrefix(candidate.Path, prefix)
	parts := strings.Split(relative, "/")
	if relative == candidate.Path || len(parts) != 2 || !isPortableReviewKey(parts[0]) || parts[1] == "" || (candidate.Role == PromptContextDraftPath && parts[1] != "draft.json") || (candidate.Role == PromptContextTracePath && parts[1] != "trace.json") {
		return durablefs.TreeSnapshot{}, WithMachineErrorCode(MachineCodePathBlocked,
			fmt.Errorf("transient prompt path %q is outside its %s proposal subtree", candidate.Path, candidate.Role))
	}
	if err := s.requireIgnoredUntrackedPromptPath(candidate.Path); err != nil {
		return durablefs.TreeSnapshot{}, err
	}

	proposal := path.Dir(candidate.Path)
	tree, err := durablefs.ObserveTree(s.paths.RepoRoot, proposal)
	if err != nil {
		return durablefs.TreeSnapshot{}, transientPromptPathError(candidate.Path, err)
	}
	if !tree.Present {
		return durablefs.TreeSnapshot{}, WithMachineErrorCode(MachineCodePathBlocked,
			fmt.Errorf("transient prompt proposal directory %q is missing", proposal))
	}
	for _, entry := range tree.Entries {
		if path.Dir(entry.Path) == "." && samePortableName(path.Base(entry.Path), parts[1]) && path.Base(entry.Path) != parts[1] {
			return durablefs.TreeSnapshot{}, WithMachineErrorCode(MachineCodePathBlocked,
				fmt.Errorf("transient prompt path %q aliases proposal entry %q", candidate.Path, entry.Path))
		}
	}
	return tree, nil
}

func validTransientPromptProposalType(role, proposalType string) bool {
	switch role {
	case PromptContextReviewPath:
		return proposalType == "task" || proposalType == "spec" || proposalType == "decomposition" || proposalType == "workflow-adversarial"
	case PromptContextDraftPath, PromptContextTracePath:
		return proposalType == "decomposition"
	default:
		return false
	}
}

func transientArtifactsPath(paths Paths) (string, error) {
	if paths.RepoRoot == "" || paths.ArtifactsDir == "" {
		return "", fmt.Errorf("transient artifacts directory is unavailable")
	}
	relative, err := filepath.Rel(paths.RepoRoot, paths.ArtifactsDir)
	if err != nil {
		return "", fmt.Errorf("resolve transient artifacts directory: %w", err)
	}
	relative = filepath.ToSlash(relative)
	if !canonicalTransientPromptPath(relative) {
		return "", fmt.Errorf("transient artifacts directory is outside the repository")
	}
	return relative, nil
}

func canonicalTransientPromptPath(value string) bool {
	return value != "" && !filepath.IsAbs(value) && !strings.Contains(value, "\\") && !strings.HasPrefix(value, "../") && path.Clean(value) == value && value != "."
}

func (s *Service) requireIgnoredUntrackedPromptPath(candidate string) error {
	if s.paths.WorktreeRoot == "" {
		return nil
	}
	if _, err := gitCommand(s.paths.WorktreeRoot, "check-ignore", "-q", "--no-index", candidate); err != nil {
		return WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("transient prompt path %q is not effectively ignored", candidate))
	}
	if output, err := gitCommand(s.paths.WorktreeRoot, "ls-files", "--error-unmatch", "--", candidate); err == nil && strings.TrimSpace(output) != "" {
		return WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("Git tracks transient prompt path %q", candidate))
	}
	if _, err := gitCommand(s.paths.WorktreeRoot, "diff", "--cached", "--quiet", "--", candidate); err != nil {
		return WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("Git index contains transient prompt path %q", candidate))
	}
	return nil
}

func transientPromptPathError(candidate string, err error) error {
	return WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("inspect transient prompt path %q: %w", candidate, err))
}
