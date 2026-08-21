package taskrail

import (
	"crypto/sha256"
	"fmt"
	"os"
)

// TaskShowResult is the exact persisted content of one task, identified by its
// logical path so callers never need to resolve an active storage overlay.
type TaskShowResult struct {
	TaskID   string `json:"task_id"`
	TaskPath string `json:"task_path"`
	Content  string `json:"content"`
	SHA256   string `json:"sha256"`
}

// TaskShow reads one exact persisted task ID through the active storage context.
// It keeps the selected bytes intact because prompts and skills use this command
// where reconstructing parsed frontmatter would change their workflow input.
func (s *Service) TaskShow(id string) (TaskShowResult, error) {
	if err := s.paths.ensureStorageCapability(); err != nil {
		return TaskShowResult{}, err
	}
	return stableRead(func() (TaskShowResult, error) {
		tasks, err := s.loadTasks()
		if err != nil {
			return TaskShowResult{}, err
		}
		task, err := exactTaskByID(tasks, id)
		if err != nil {
			return TaskShowResult{}, err
		}
		content, err := os.ReadFile(task.Filename)
		if err != nil {
			return TaskShowResult{}, WithMachineErrorCode(missingOrInvalidCode(err, MachineCodeTaskNotFound),
				fmt.Errorf("read task %s: %w", task.Path, fsCause(err)))
		}
		digest := sha256.Sum256(content)
		return TaskShowResult{
			TaskID:   task.Frontmatter.ID,
			TaskPath: task.Path,
			Content:  string(content),
			SHA256:   fmt.Sprintf("%x", digest),
		}, nil
	})
}
