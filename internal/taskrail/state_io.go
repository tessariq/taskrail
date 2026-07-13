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
	data, err := os.ReadFile(s.paths.StateFile)
	if err != nil {
		return nil, fmt.Errorf("read state file %s: %w", relPath(s.paths.RepoRoot, s.paths.StateFile), fsCause(err))
	}
	frontmatter, body, err := parseFrontmatter[StateFrontmatter](data)
	if err != nil {
		return nil, fmt.Errorf("parse state file: %w", err)
	}
	return &State{Frontmatter: frontmatter, Body: body}, nil
}

func (s *Service) loadTasks() ([]*Task, error) {
	entries, err := os.ReadDir(s.paths.TasksDir)
	if err != nil {
		return nil, fmt.Errorf("read tasks dir %s: %w", relPath(s.paths.RepoRoot, s.paths.TasksDir), fsCause(err))
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
		tasks = append(tasks, &Task{Frontmatter: frontmatter, Body: body, Filename: filename})
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Frontmatter.ID < tasks[j].Frontmatter.ID
	})
	return tasks, nil
}

func (s *Service) saveAll(state *State, tasks []*Task) error {
	state.Body = renderStateBody(state.Frontmatter, tasks)
	if err := s.saveState(state); err != nil {
		return err
	}
	for _, task := range tasks {
		if err := s.saveTask(task); err != nil {
			return err
		}
	}
	return nil
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
		return fmt.Errorf("write state file %s: %w", relPath(s.paths.RepoRoot, s.paths.StateFile), fsCause(err))
	}
	return nil
}

func (s *Service) saveTask(task *Task) error {
	data, err := marshalFrontmatter(task.Frontmatter, task.Body)
	if err != nil {
		return fmt.Errorf("marshal task file %s: %w", filepath.Base(task.Filename), err)
	}
	if err := os.WriteFile(task.Filename, data, 0o644); err != nil {
		return fmt.Errorf("write task file %s: %w", filepath.Base(task.Filename), fsCause(err))
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
