package taskrail

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// RenameTaskInput drives an atomic re-slug of a task id. Exactly one of Slug or
// Title selects the new slug segment (Title is a slug source only — it never
// rewrites the frontmatter title). The numeric `T-<n>` prefix is preserved; only
// the slug segment changes. DryRun reports the planned change set without writing.
type RenameTaskInput struct {
	OldID  string
	Slug   string
	Title  string
	DryRun bool
}

// RenameChange records one coupled edit a rename performs (or would perform on a
// dry run), named so a reviewer can inspect the change set before it lands. Kind
// is "frontmatter_id", "file_rename", or "dependency_ref"; TaskID names the task
// file the edit touches (the inbound task for a dependency_ref).
type RenameChange struct {
	Kind   string `json:"kind"`
	TaskID string `json:"task_id,omitempty"`
	From   string `json:"from"`
	To     string `json:"to"`
}

// RenameTaskResult reports the re-slug the command planned (dry run) or applied.
// Validation reflects the current state on a dry run and the post-apply state
// otherwise, so a reviewer always sees the resulting validity.
type RenameTaskResult struct {
	OldID      string            `json:"old_id"`
	NewID      string            `json:"new_id"`
	Applied    bool              `json:"applied"`
	Changes    []RenameChange    `json:"changes"`
	Validation *ValidationResult `json:"validation,omitempty"`
}

// RenameTask atomically re-slugs a task: it rewrites the `id:` frontmatter,
// renames the file to `<new-id>.md`, rewrites every inbound `dependencies:`
// reference (and the STATE.md current_task pointer when it names the task), then
// re-projects STATE.md and re-runs validation. A target id colliding with an
// existing task fails before any write, so the tree is never left partially
// renamed. It only re-encodes an identifier and the edges that name it — it never
// advances a status or fabricates work.
func (s *Service) RenameTask(input RenameTaskInput) (RenameTaskResult, error) {
	oldID := strings.TrimSpace(input.OldID)
	if oldID == "" {
		return RenameTaskResult{}, errors.New("task id is required")
	}
	slug, err := renameSlug(input)
	if err != nil {
		return RenameTaskResult{}, err
	}

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return RenameTaskResult{}, err
	}
	target, ok := taskByID(tasks, oldID)
	if !ok {
		return RenameTaskResult{}, fmt.Errorf("task %s not found", oldID)
	}
	prefix, ok := taskIDPrefix(oldID)
	if !ok {
		return RenameTaskResult{}, fmt.Errorf("task id %s has no T-<n> numeric prefix to preserve", oldID)
	}
	newID := prefix
	if slug != "" {
		newID += "-" + slug
	}
	if newID == oldID {
		return RenameTaskResult{}, fmt.Errorf("task %s already carries slug %q", oldID, slug)
	}
	if _, exists := taskByID(tasks, newID); exists {
		return RenameTaskResult{}, fmt.Errorf("target id %s already exists", newID)
	}

	oldPath := target.Filename
	newPath := filepath.Join(s.paths.TasksDir, newID+".md")
	// Guard the physical destination too, not just the in-memory id index: a stray
	// file whose id disagrees with its name (a filename!=id drift repair heals)
	// escapes the taskByID check, and the plain-rename fallback would silently
	// clobber it. Refuse before any write, so the tree is never partially renamed.
	if fileExists(newPath) {
		return RenameTaskResult{}, fmt.Errorf("target file %s already exists", relPath(s.paths.RepoRoot, newPath))
	}
	inbound := inboundDependents(tasks, oldID)
	changes := renameChanges(s.paths.RepoRoot, oldID, newID, oldPath, newPath, inbound)

	if !input.DryRun {
		if err := s.applyRename(state, tasks, target, inbound, oldID, newID, oldPath, newPath); err != nil {
			return RenameTaskResult{}, err
		}
	}
	// Validation reflects current state on a dry run and post-apply state otherwise.
	validation, err := s.Validate()
	if err != nil {
		return RenameTaskResult{}, err
	}
	return RenameTaskResult{OldID: oldID, NewID: newID, Applied: !input.DryRun, Changes: changes, Validation: &validation}, nil
}

// renameSlug resolves the new slug from exactly one selector, sharing slugify
// with task creation (T-095) so slugs are normalized identically on both paths.
func renameSlug(input RenameTaskInput) (string, error) {
	hasSlug := strings.TrimSpace(input.Slug) != ""
	hasTitle := strings.TrimSpace(input.Title) != ""
	if hasSlug == hasTitle {
		return "", errors.New("exactly one of --slug or --title is required")
	}
	source := input.Title
	if hasSlug {
		source = input.Slug
	}
	// A selector was given, so it must yield a real slug. Unlike `task new` (where a
	// bare id is a valid no-selector outcome), rename always changes the slug
	// segment, so a value that normalizes to "" is a mistake, not a request to
	// strip the slug — reject it rather than silently drop to a bare id.
	slug := slugify(source)
	if slug == "" {
		return "", fmt.Errorf("slug %q normalizes to empty", source)
	}
	return slug, nil
}

// inboundDependents returns the tasks (other than id itself) whose dependencies
// name id, in load order, so the rename can rewrite every edge that points at it.
func inboundDependents(tasks []*Task, id string) []*Task {
	dependents := make([]*Task, 0)
	for _, task := range tasks {
		if task.Frontmatter.ID == id {
			continue
		}
		if slices.Contains(task.Frontmatter.Dependencies, id) {
			dependents = append(dependents, task)
		}
	}
	return dependents
}

// renameChanges builds the reviewable change set: the frontmatter id rewrite, the
// file rename, and one dependency_ref edit per inbound task.
func renameChanges(root, oldID, newID, oldPath, newPath string, inbound []*Task) []RenameChange {
	changes := []RenameChange{
		{Kind: "frontmatter_id", TaskID: oldID, From: oldID, To: newID},
		{Kind: "file_rename", TaskID: oldID, From: relPath(root, oldPath), To: relPath(root, newPath)},
	}
	for _, task := range inbound {
		changes = append(changes, RenameChange{Kind: "dependency_ref", TaskID: task.Frontmatter.ID, From: oldID, To: newID})
	}
	return changes
}

// applyRename performs the coupled writes in an order that keeps the tree as
// consistent as possible: move the file first (preserving git rename tracking),
// rewrite the renamed task's id, rewrite each inbound dependency edge, then
// re-project STATE.md. state and the task pointers are mutated in place so the
// STATE.md projection reflects the new ids.
func (s *Service) applyRename(state *State, tasks []*Task, target *Task, inbound []*Task, oldID, newID, oldPath, newPath string) error {
	if err := s.moveTaskFile(oldPath, newPath); err != nil {
		return err
	}
	now := timestamp(s.now())

	target.Frontmatter.ID = newID
	target.Frontmatter.UpdatedAt = now
	target.Filename = newPath
	if err := s.saveTask(target); err != nil {
		return err
	}

	for _, task := range inbound {
		for i, dep := range task.Frontmatter.Dependencies {
			if dep == oldID {
				task.Frontmatter.Dependencies[i] = newID
			}
		}
		task.Frontmatter.UpdatedAt = now
		if err := s.saveTask(task); err != nil {
			return err
		}
	}

	// The current_task pointer names the task by id, so a rename of the active task
	// must repoint it or validate would flag a current_task/in_progress mismatch.
	if state.Frontmatter.CurrentTask == oldID {
		state.Frontmatter.CurrentTask = newID
	}
	state.Frontmatter.UpdatedAt = now
	// Re-project the rendered body from the (in-place mutated) task set so the
	// Current Focus section and counts stay consistent with the new ids, matching
	// every other state-writing path.
	state.Body = renderStateBody(state.Frontmatter, tasks)
	return s.saveState(state)
}

// moveTaskFile renames the task file, preferring `git mv` when the repository is
// under version control so the rename is staged and tracked. It falls back to a
// plain rename when git is absent, the tree is not a real repository, or the file
// is untracked (any of which makes `git mv` fail) so the re-slug still completes.
func (s *Service) moveTaskFile(oldPath, newPath string) error {
	if s.underVersionControl() {
		if err := gitMove(s.paths.RepoRoot, oldPath, newPath); err == nil {
			return nil
		}
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("rename task file %s to %s: %w",
			relPath(s.paths.RepoRoot, oldPath), relPath(s.paths.RepoRoot, newPath), fsCause(err))
	}
	return nil
}

func (s *Service) underVersionControl() bool {
	_, err := os.Stat(filepath.Join(s.paths.RepoRoot, ".git"))
	return err == nil
}

func gitMove(root, oldPath, newPath string) error {
	cmd := exec.Command("git", "-C", root, "mv", oldPath, newPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git mv: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
