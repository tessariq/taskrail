package taskrail

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (s *Service) loadStateAndTasks() (*State, []*Task, error) {
	state, err := s.loadState()
	if err != nil {
		return nil, nil, err
	}
	tasks, err := s.loadTasks()
	if err != nil {
		return nil, nil, err
	}
	return state, tasks, nil
}

func (s *Service) loadState() (*State, error) {
	if err := s.paths.ensureStorageCapability(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(s.paths.StateFile)
	if err != nil {
		return nil, WithMachineErrorCode(missingOrInvalidCode(err, MachineCodeNotInitialized),
			fmt.Errorf("read state file %s: %w", s.paths.logicalManagedPath(s.paths.StateFile), fsCause(err)))
	}
	frontmatter, body, err := parseFrontmatter[StateFrontmatter](data)
	if err != nil {
		return nil, fmt.Errorf("parse state file: %w", err)
	}
	return &State{Frontmatter: frontmatter, Body: body}, nil
}

func (s *Service) loadTasks() ([]*Task, error) {
	if err := s.paths.ensureStorageCapability(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.paths.TasksDir)
	if errors.Is(err, os.ErrNotExist) {
		// A promoted empty local ledger has no task bytes to create the directory.
		// Treat that durable absence as an empty ledger; the first task writer
		// creates its parent through the transactional publication path.
		return []*Task{}, nil
	}
	if err != nil {
		return nil, WithMachineErrorCode(missingOrInvalidCode(err, MachineCodeNotInitialized),
			fmt.Errorf("read tasks dir %s: %w", s.paths.logicalManagedPath(s.paths.TasksDir), fsCause(err)))
	}
	tasks := make([]*Task, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		filename := filepath.Join(s.paths.TasksDir, entry.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("read task %s: %w", entry.Name(), fsCause(err))
		}
		frontmatter, body, err := parseFrontmatter[TaskFrontmatter](data)
		if err != nil {
			return nil, fmt.Errorf("parse task %s: %w", entry.Name(), err)
		}
		tasks = append(tasks, &Task{Frontmatter: frontmatter, Body: body, raw: data, Path: s.paths.logicalManagedPath(filename), Filename: filename})
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Frontmatter.ID < tasks[j].Frontmatter.ID
	})
	return tasks, nil
}

func (s *Service) saveState(state *State) error {
	if strings.TrimSpace(state.Body) == "" {
		state.Body = renderStateBody(state.Frontmatter, nil)
	}
	data, err := marshalFrontmatter(state.Frontmatter, state.Body)
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.paths.StateFile, data, 0o644); err != nil {
		return s.managedWriteFailure(s.paths.StateFile,
			fmt.Errorf("write state file %s: %w", relPath(s.paths.RepoRoot, s.paths.StateFile), fsCause(err)))
	}
	return nil
}

// managedWriteFailure classifies a failed write to one managed file. Taskrail
// has no transaction protocol yet, so a semantic operation that writes more than
// one file can stop here with earlier files already on disk: the operation never
// committed (`applied` stays false) but the repository may be partly updated,
// which is exactly what partial_write names. The failing path is reported
// because it now holds content the rest of the operation disagrees with; a
// caller that already landed other files adds them with withWrittenPaths.
func (s *Service) managedWriteFailure(path string, cause error) error {
	return WithMachineFailure(MachineFailure{
		Code:  MachineCodePartialWrite,
		Paths: []string{relPath(s.paths.RepoRoot, path)},
	}, cause)
}

// withWrittenPaths adds files a failed operation had already written to its
// report, so `paths` names everything an agent has to review rather than only
// the write that failed. Only the writer knows what it landed before the
// failure, so it adds them here rather than the boundary guessing.
func (s *Service) withWrittenPaths(cause error, written ...string) error {
	failure := MachineFailureFor(cause)
	for _, path := range written {
		failure.Paths = append(failure.Paths, relPath(s.paths.RepoRoot, path))
	}
	return WithMachineFailure(failure, cause)
}

func (s *Service) saveTask(task *Task) error {
	data, err := marshalFrontmatter(task.Frontmatter, task.Body)
	if err != nil {
		return fmt.Errorf("marshal task file %s: %w", filepath.Base(task.Filename), err)
	}
	if err := os.WriteFile(task.Filename, data, 0o644); err != nil {
		return s.managedWriteFailure(task.Filename,
			fmt.Errorf("write task file %s: %w", filepath.Base(task.Filename), fsCause(err)))
	}
	return nil
}

// ensureDir creates path and parents. root anchors the repo-relative path named
// on failure so the error stays portable (T-088); it carries no repo root of its
// own, so callers thread theirs.
func ensureDir(root, path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", relPath(root, path), fsCause(err))
	}
	return nil
}

// writeFileIfMissing writes data at path only when it does not already exist.
// root anchors the repo-relative path named on failure (T-088).
func writeFileIfMissing(root, path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", relPath(root, path), fsCause(err))
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", relPath(root, path), fsCause(err))
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func relPath(root, target string) string {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return target
	}
	return filepath.ToSlash(rel)
}

// fsCause unwraps a filesystem error to its underlying cause (e.g. "no such file
// or directory") without the *fs.PathError's absolute path. Read and write callers
// name the repo-relative path themselves, so wrapping the raw error would only
// append the user's absolute repository location — noise that makes emitted error
// text non-portable. The unwrapped cause still satisfies errors.Is(err, fs.ErrNotExist)
// and friends, so callers' error classification is unaffected.
func fsCause(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}
