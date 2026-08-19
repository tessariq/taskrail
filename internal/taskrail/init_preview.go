package taskrail

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

type initPreviewSnapshot map[string]string

// snapshotInitPreview binds every file/directory that init and retrofit inspect
// while constructing a preview. A second snapshot before return makes the
// reported candidate one stable observation without taking the writer lock or
// creating runtime artifacts.
func (s *Service) snapshotInitPreview(notesSource string) (initPreviewSnapshot, error) {
	roots := []string{
		filepath.Join(s.paths.RepoRoot, taskrailConfigDir),
		s.paths.SpecsDir,
		s.paths.PlanningDir,
		filepath.Join(s.paths.RepoRoot, "notes"),
		filepath.Join(s.paths.RepoRoot, shippableSkillTargets[0]),
		filepath.Join(s.paths.RepoRoot, shippableSkillTargets[1]),
	}
	if notesSource != "" {
		if filepath.IsAbs(notesSource) {
			roots = append(roots, notesSource)
		} else {
			roots = append(roots, filepath.Join(s.paths.RepoRoot, notesSource))
		}
	}
	slices.Sort(roots)
	snapshot := make(initPreviewSnapshot)
	for _, root := range roots {
		if err := snapshotInitRoot(snapshot, s.paths.RepoRoot, root); err != nil {
			return nil, err
		}
	}
	return snapshot, nil
}

func snapshotInitRoot(snapshot initPreviewSnapshot, repo, root string) error {
	root = filepath.Clean(root)
	return filepath.WalkDir(root, func(name string, entry os.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			snapshot[filepath.ToSlash(relPath(repo, root))] = "absent"
			return filepath.SkipDir
		}
		if err != nil {
			return err
		}
		reported := filepath.ToSlash(relPath(repo, name))
		if entry.IsDir() {
			snapshot[reported+"/"] = "directory"
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(name)
			if err != nil {
				return err
			}
			snapshot[reported] = "symlink:" + digestBytes([]byte(target))
			return nil
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		snapshot[reported] = digestBytes(data)
		return nil
	})
}

func (s *Service) recheckInitPreview(before initPreviewSnapshot, notesSource string) error {
	after, err := s.snapshotInitPreview(notesSource)
	if err != nil {
		return err
	}
	if len(before) != len(after) {
		return WithMachineErrorCode(MachineCodeSourceChanged, fmt.Errorf("repository changed while the preview candidate was built"))
	}
	for name, digest := range before {
		if after[name] != digest {
			return WithMachineErrorCode(MachineCodeSourceChanged, fmt.Errorf("%s changed while the preview candidate was built", name))
		}
	}
	return nil
}
