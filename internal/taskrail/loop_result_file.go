package taskrail

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/tessariq/taskrail/internal/durablefs"
)

// LoopResultFile retains the canonical destination and the parent identity that
// must still name the same no-follow directory at final publication.
type LoopResultFile struct {
	path           string
	parent         string
	leaf           string
	parentIdentity durablefs.Identity
}

var testHookLoopResultBeforePublish func()
var testHookLoopResultAfterPublish func()

// PrepareLoopResultFile validates the caller-owned destination before repository
// preflight, so ordinary refusals can still produce an out-of-band envelope.
func PrepareLoopResultFile(path string) (*LoopResultFile, error) {
	if path == "" {
		return nil, invalidArgumentsf("--result-file requires a non-empty path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, invalidArgumentsf("resolve --result-file: %v", err)
	}
	abs = filepath.Clean(abs)
	parent, leaf := filepath.Dir(abs), filepath.Base(abs)
	root, err := durablefs.OpenExternal(parent)
	if err != nil {
		return nil, WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("open result-file parent: %w", err))
	}
	defer root.Close()
	entry, err := root.Bind(leaf)
	if err == nil {
		entry.Close()
		return nil, WithMachineErrorCode(MachineCodeDestinationExists, fmt.Errorf("result file already exists: %s", abs))
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("inspect result file: %w", err))
	}
	return &LoopResultFile{path: abs, parent: parent, leaf: leaf, parentIdentity: root.Identity()}, nil
}

// ValidateOutside rejects destinations that would alter or alias repository
// inputs. It runs after discovery but before the loop's repository preflight.
func (s *Service) ValidateLoopResultFile(result *LoopResultFile) error {
	for _, root := range []string{s.paths.WorktreeRoot, s.paths.GitDir, s.paths.GitCommonDir, s.paths.ManagedRoot, s.paths.SpecsDir, s.paths.PlanningDir, s.paths.PromptsDir} {
		if root != "" && loopResultWithin(result.path, root) {
			return WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("result file must be outside repository inputs: %s", result.path))
		}
	}
	return nil
}

func loopResultWithin(path, root string) bool {
	root = filepath.Clean(root)
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

// Publish reopens the original parent without following substitutions and uses a
// no-clobber final link, leaving a late target or swapped parent unclaimed.
func (r *LoopResultFile) Publish(document []byte) error {
	if testHookLoopResultBeforePublish != nil {
		testHookLoopResultBeforePublish()
	}
	root, err := durablefs.OpenExternal(r.parent)
	if err != nil {
		return err
	}
	defer root.Close()
	if root.Identity() != r.parentIdentity {
		return fmt.Errorf("%w: result-file parent changed", durablefs.ErrConflict)
	}
	entry, err := root.Bind(r.leaf)
	if err == nil {
		entry.Close()
		return fs.ErrExist
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	_, err = root.Publish(r.leaf, append(document, '\n'), 0o600)
	if err != nil {
		return err
	}
	if testHookLoopResultAfterPublish != nil {
		testHookLoopResultAfterPublish()
	}
	current, err := durablefs.OpenExternal(r.parent)
	if err != nil {
		return err
	}
	defer current.Close()
	if current.Identity() != r.parentIdentity {
		return fmt.Errorf("%w: result-file parent changed after publication", durablefs.ErrConflict)
	}
	return nil
}
