package taskrail

import (
	"bytes"
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
// is "frontmatter_id", "file_rename", "body_heading", or "dependency_ref"; TaskID
// names the task file the edit touches (the inbound task for a dependency_ref).
// A "body_heading" change is reported only when the body actually opens with a
// heading naming the old id, so the change set never claims an edit that does not
// happen.
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
	// Warnings reports the empty-derived-slug de-slug, the one non-fatal signal a
	// rename can raise. Omitted when empty so an ordinary re-slug's shape is
	// unchanged.
	Warnings []Warning `json:"warnings,omitempty"`
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
	slug, slugSource, err := renameSlug(input)
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
		if slug == "" {
			return RenameTaskResult{}, fmt.Errorf("task %s is already bare and %q produces no slug segment", oldID, slugSource)
		}
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
	changes := renameChanges(s.paths.RepoRoot, oldID, newID, oldPath, newPath, target.Body, inbound)

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
	var warnings []Warning
	if slug == "" {
		warnings = emptySlugWarnings(slugSource, newID)
	}
	return RenameTaskResult{
		OldID:      oldID,
		NewID:      newID,
		Applied:    !input.DryRun,
		Changes:    changes,
		Validation: &validation,
		Warnings:   warnings,
	}, nil
}

// renameSlug resolves the new slug from exactly one selector, sharing slugify
// with task creation (T-095) so slugs are normalized identically on both paths.
// It returns the normalized slug alongside the raw source: a source that
// normalizes to "" de-slugs the task to its bare id (symmetric with creation's
// bare-id fallback), and the caller warns using the source the operator supplied.
func renameSlug(input RenameTaskInput) (slug, source string, err error) {
	hasSlug := strings.TrimSpace(input.Slug) != ""
	hasTitle := strings.TrimSpace(input.Title) != ""
	if hasSlug == hasTitle {
		return "", "", errors.New("exactly one of --slug or --title is required")
	}
	source = input.Title
	if hasSlug {
		source = input.Slug
	}
	return slugify(source), source, nil
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

// renameBodyHeading repoints a task body's opening `# <id> <title>` heading at the
// new id, returning the rewritten body and whether it changed. The id and the
// heading are two places the same identifier is written, so leaving the heading
// behind makes the file name two different tasks. It is deliberately conservative:
// only a leading H1 whose first token is exactly the old id is a heading Taskrail
// wrote, and the title text after it is content the rename must not disturb.
func renameBodyHeading(body, oldID, newID string) (string, bool) {
	rest, ok := strings.CutPrefix(body, "# "+oldID)
	if !ok {
		return body, false
	}
	// Guard against a longer id sharing this one as a prefix: only a space (a title
	// follows) or the end of the line ends the id token.
	if rest != "" && !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, "\n") {
		return body, false
	}
	return "# " + newID + rest, true
}

// bodyHeadingLine returns the first line of body, which is where a task's H1 lives.
func bodyHeadingLine(body string) string {
	line, _, _ := strings.Cut(body, "\n")
	return line
}

// renameChanges builds the reviewable change set: the frontmatter id rewrite, the
// file rename, the body heading rewrite when the body carries one, and one
// dependency_ref edit per inbound task.
func renameChanges(root, oldID, newID, oldPath, newPath, body string, inbound []*Task) []RenameChange {
	changes := []RenameChange{
		{Kind: "frontmatter_id", TaskID: oldID, From: oldID, To: newID},
		{Kind: "file_rename", TaskID: oldID, From: relPath(root, oldPath), To: relPath(root, newPath)},
	}
	if rewritten, ok := renameBodyHeading(body, oldID, newID); ok {
		changes = append(changes, RenameChange{
			Kind:   "body_heading",
			TaskID: oldID,
			From:   bodyHeadingLine(body),
			To:     bodyHeadingLine(rewritten),
		})
	}
	for _, task := range inbound {
		changes = append(changes, RenameChange{Kind: "dependency_ref", TaskID: task.Frontmatter.ID, From: oldID, To: newID})
	}
	return changes
}

// applyRename performs the coupled writes as one outcome: a failure partway
// through unwinds every write already made, so the tree is either fully renamed
// or untouched. Ordering keeps the tree as consistent as possible while the
// writes run: move the file first (preserving git rename tracking), rewrite the
// renamed task's id, rewrite each inbound dependency edge, then re-project
// STATE.md.
func (s *Service) applyRename(state *State, tasks []*Task, target *Task, inbound []*Task, oldID, newID, oldPath, newPath string) error {
	undo := &renameUndo{root: s.paths.RepoRoot}
	if err := s.renameWrites(undo, state, tasks, target, inbound, newID, oldPath, newPath); err != nil {
		return renameFailure(oldID, newID, err, undo.run())
	}
	return nil
}

// renameWrites applies the coupled edits, registering each compensating action
// with undo before the write it compensates. state and the task pointers are
// mutated in place so the STATE.md projection reflects the new ids; those
// in-memory edits are discarded with the loaded set when the rename fails.
func (s *Service) renameWrites(undo *renameUndo, state *State, tasks []*Task, target *Task, inbound []*Task, newID, oldPath, newPath string) error {
	oldID := target.Frontmatter.ID
	if err := s.moveTaskFile(oldPath, newPath); err != nil {
		return err
	}
	undo.push(func() error { return s.moveTaskFile(newPath, oldPath) })
	now := timestamp(s.now())

	if err := undo.snapshot(newPath); err != nil {
		return err
	}
	target.Frontmatter.ID = newID
	target.Frontmatter.UpdatedAt = now
	target.Filename = newPath
	target.Body, _ = renameBodyHeading(target.Body, oldID, newID)
	if err := s.saveTask(target); err != nil {
		return err
	}

	for _, task := range inbound {
		if err := undo.snapshot(task.Filename); err != nil {
			return err
		}
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
	if err := undo.snapshot(s.paths.StateFile); err != nil {
		return err
	}
	return s.saveState(state)
}

// renameUndo collects the compensating actions for the writes a rename has
// already made. Actions run last-in-first-out, so a file's content is restored
// before the file itself is moved back. root anchors repo-relative paths in the
// errors it emits (T-088).
type renameUndo struct {
	root    string
	actions []func() error
}

func (u *renameUndo) push(action func() error) { u.actions = append(u.actions, action) }

// snapshot captures path's current bytes and registers the action that restores
// them. Callers must snapshot *before* the write it compensates: os.WriteFile
// truncates first, so a write that fails partway still needs the original bytes.
func (u *renameUndo) snapshot(path string) error {
	before, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", relPath(u.root, path), fsCause(err))
	}
	u.push(func() error {
		// A write that failed at open (permission denied, the failure mode this
		// rollback exists for) never truncated the file, so its bytes still match
		// the snapshot and rewriting them would fail for the same reason — turning
		// a clean rollback into a spurious "may be partially renamed" report.
		if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, before) {
			return nil
		}
		if err := os.WriteFile(path, before, 0o644); err != nil {
			return fmt.Errorf("restore %s: %w", relPath(u.root, path), fsCause(err))
		}
		return nil
	})
	return nil
}

// run unwinds the recorded actions and reports the first failure, which is the
// point where the tree stopped being restorable — later actions are still
// attempted so as much of the tree as possible returns to its original shape.
func (u *renameUndo) run() error {
	var firstErr error
	for i := len(u.actions) - 1; i >= 0; i-- {
		if err := u.actions[i](); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// renameFailure composes the error a failed rename returns. A clean rollback
// leaves the tree exactly as it was, so the operator only needs the cause; a
// failed rollback may leave the tree half renamed, and the error is the only
// signal they get — so it names the reconcile commands rather than letting the
// drift be discovered by a later validate.
func renameFailure(oldID, newID string, cause, undoErr error) error {
	if undoErr == nil {
		return fmt.Errorf("rename %s to %s failed, no changes were applied: %w", oldID, newID, cause)
	}
	return fmt.Errorf("rename %s to %s failed and the rollback failed (%v); the tree may be partially renamed — run `taskrail validate` then `taskrail repair --apply` to reconcile: %w",
		oldID, newID, undoErr, cause)
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
