package taskrail

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/tessariq/taskrail/internal/repotx"
)

// validateInitTransactionCandidate materializes only the managed candidate in a
// private sandbox and runs the ordinary repository validator before publication.
// The transaction's expected digests bind every copied original to its later
// snapshot, so validation and publication describe the same bytes.
func (s *Service) validateInitTransactionCandidate(tx *initTransaction, marker LayoutConfig) error {
	temp, err := os.MkdirTemp("", "taskrail-init-candidate-*")
	if err != nil {
		return fmt.Errorf("create init validation sandbox: %w", err)
	}
	defer os.RemoveAll(temp)

	for _, source := range []struct {
		physical string
		logical  string
	}{{s.paths.SpecsDir, s.paths.LogicalSpecsDir}, {s.paths.PlanningDir, s.paths.LogicalPlanningDir}} {
		if err := copyInitCandidateTree(source.physical, filepath.Join(temp, filepath.FromSlash(source.logical))); err != nil {
			return err
		}
	}
	for _, candidate := range tx.published {
		var destination string
		switch {
		case candidate.Kind == repotx.Managed:
			destination = filepath.Join(temp, filepath.FromSlash(candidate.Reported))
		case candidate.Kind == repotx.Worktree && candidate.Reported == markerRelPath():
			destination = filepath.Join(temp, filepath.FromSlash(candidate.Reported))
		default:
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(destination, candidate.Content, 0o644); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(temp, filepath.FromSlash(s.paths.LogicalPlanningDir), "tasks"), 0o755); err != nil {
		return err
	}
	paths := pathsFromLayout(temp, marker, committedStorage())
	validation, err := (&Service{paths: paths, now: s.now}).Validate()
	if err != nil {
		return WithMachineErrorCode(MachineCodeValidationFailed,
			fmt.Errorf("validate complete init candidate: %w", err))
	}
	tx.validation = validation
	return nil
}

func copyInitCandidateTree(source, destination string) error {
	return filepath.WalkDir(source, func(name string, entry fs.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, name)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("validate init candidate: %s is not a regular file", name)
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
