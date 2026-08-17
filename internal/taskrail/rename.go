package taskrail

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/tessariq/taskrail/internal/repotx"
)

// RenameTaskInput drives an atomic re-slug of a task id. Exactly one of Slug or
// Title selects the new slug segment (Title is a slug source only — it never
// rewrites the frontmatter title). The numeric `T-<n>` prefix is preserved; only
// the slug segment changes. DryRun reports the planned change set without writing.
type RenameTaskInput struct {
	OldID         string
	Slug          string
	SlugExplicit  bool
	Title         string
	TitleExplicit bool
	DryRun        bool
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
// Validation reflects the state the rename would produce — a dry run previews the
// full change set in memory, an apply validates the exact candidate it publishes
// — so a reviewer always sees the resulting validity.
type RenameTaskResult struct {
	OldID      string            `json:"old_id"`
	NewID      string            `json:"new_id"`
	Applied    bool              `json:"applied"`
	Changes    []RenameChange    `json:"changes"`
	Validation *ValidationResult `json:"validation,omitempty"`
	// Warnings reports the empty-derived-slug de-slug, the one non-fatal signal a
	// rename can raise.
	Warnings []Warning `json:"-"`
}

// RenameTask atomically re-slugs a task: it rewrites the `id:` frontmatter,
// renames the file to `<new-id>.md`, rewrites every inbound `dependencies:`
// reference (and the STATE.md current_task pointer when it names the task), then
// re-projects STATE.md and re-runs validation. The rename publishes through one
// normal transaction as filesystem operations — no Git staging — so a target id
// colliding with an existing task, a concurrent edit to any written file, or a
// handled publication failure leaves the tree either fully renamed or untouched.
// It only re-encodes an identifier and the edges that name it — it never
// advances a status or fabricates work.
func (s *Service) RenameTask(input RenameTaskInput) (result RenameTaskResult, err error) {
	oldID := input.OldID
	if oldID == "" {
		return RenameTaskResult{}, WithMachineErrorCode(MachineCodeInvalidArguments,
			errors.New("task id is required"))
	}
	slug, slugSource, err := renameSlug(input)
	if err != nil {
		return RenameTaskResult{}, err
	}

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return RenameTaskResult{}, err
	}
	target, err := exactTaskByID(tasks, oldID)
	if err != nil {
		return RenameTaskResult{}, err
	}
	prefix, ok := taskIDPrefix(oldID)
	if !ok {
		return RenameTaskResult{}, invalidArgumentsf("task id %s has no T-<n> numeric prefix to preserve", oldID)
	}
	newID := prefix
	if slug != "" {
		newID += "-" + slug
	}
	if newID == oldID {
		if slug == "" {
			return RenameTaskResult{}, invalidArgumentsf("task %s is already bare and %q produces no slug segment", oldID, slugSource)
		}
		return RenameTaskResult{}, invalidArgumentsf("task %s already carries slug %q", oldID, slug)
	}
	if _, exists := taskByID(tasks, newID); exists {
		return RenameTaskResult{}, WithMachineErrorCode(MachineCodeDestinationExists,
			fmt.Errorf("target id %s already exists", newID))
	}

	oldPath := target.Filename
	newPath := filepath.Join(s.paths.TasksDir, newID+".md")
	// Guard the physical destination too, not just the in-memory id index: a stray
	// file whose id disagrees with its name (a filename!=id drift repair heals)
	// escapes the taskByID check, and the transaction's no-clobber publication
	// would refuse it only after the snapshot. Refuse before any write, so the
	// tree is never partially renamed. Renaming onto the file's own drifted name
	// is the heal and stays allowed.
	if filepath.Clean(oldPath) != filepath.Clean(newPath) && fileExists(newPath) {
		return RenameTaskResult{}, WithMachineErrorCode(MachineCodeDestinationExists,
			fmt.Errorf("target file %s already exists", s.paths.logicalManagedPath(newPath)))
	}
	inbound := inboundDependents(tasks, oldID)
	changes := renameChanges(oldID, newID, s.paths.logicalManagedPath(oldPath), s.paths.logicalManagedPath(newPath), target.Body, inbound)

	// Both branches report the state the rename *would* leave behind. The dry
	// run is a read-only preview, so it takes no mutation lock — it publishes
	// nothing to protect. Apply mode locks and validates the same candidate
	// ledger it publishes, so a validation failure cannot leave the coupled
	// rename applied. An operator re-slugging to heal a `filename must be
	// <id>.md` drift is asking "would this fix it?", so validate the preview
	// rather than the state being replaced.
	previewState, previewTasks := renamePreview(state, tasks, target, inbound, oldID, newID, newPath)
	if input.DryRun {
		validation := s.validateInMemory(previewState, previewTasks)
		return renameTaskResult(oldID, newID, false, changes, validation, slug, slugSource), nil
	}

	own, release, err := s.beginTaskWriterWrite(taskRenameWriter)
	if err != nil {
		return RenameTaskResult{}, err
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil && err == nil {
			err = releaseErr
		}
	}()
	// The corpus and baseline are observed under the lock: the transaction's
	// recheck then refuses any candidate built from reads that predate it.
	corpus, err := snapshotTaskCorpus(tasks)
	if err != nil {
		return RenameTaskResult{}, err
	}
	baseline := s.validateInMemory(state, tasks)

	now := timestamp(s.now())
	published, err := s.renameCandidates(target, inbound, oldID, newID, newPath, now)
	if err != nil {
		return RenameTaskResult{}, err
	}
	previewState.Frontmatter.UpdatedAt = now
	previewState.Body = renderStateBody(previewState.Frontmatter, previewTasks)
	written := append([]*Task{target}, inbound...)
	validation, err := s.commitTaskWriter(own, taskRenameWriter, taskWriterLedger{
		state:     previewState,
		preview:   previewTasks,
		published: published,
		selected:  oldID,
		tasks:     tasks,
		written:   written,
		corpus:    corpus,
		baseline:  baseline,
		strict:    true,
	})
	if err != nil {
		return RenameTaskResult{}, err
	}
	return renameTaskResult(oldID, newID, true, changes, validation, slug, slugSource), nil
}

func renameTaskResult(oldID, newID string, applied bool, changes []RenameChange, validation ValidationResult, slug, slugSource string) RenameTaskResult {
	var warnings []Warning
	if slug == "" {
		warnings = emptySlugWarnings(slugSource, newID)
	}
	return RenameTaskResult{
		OldID:      oldID,
		NewID:      newID,
		Applied:    applied,
		Changes:    changes,
		Validation: &validation,
		Warnings:   warnings,
	}
}

// renameSlug resolves the new slug from exactly one selector, sharing slugify
// and capSlug with task creation (T-095, T-126) so one source yields one slug
// whichever command wrote it. It returns the normalized slug alongside the raw
// source: a source that normalizes to "" de-slugs the task to its bare id
// (symmetric with creation's bare-id fallback), and the caller warns using the
// source the operator supplied. Only the derived `--title` is capped; an
// explicit `--slug` is the operator's curation and is written verbatim.
func renameSlug(input RenameTaskInput) (slug, source string, err error) {
	hasSlug := input.SlugExplicit || strings.TrimSpace(input.Slug) != ""
	hasTitle := input.TitleExplicit || strings.TrimSpace(input.Title) != ""
	if hasSlug == hasTitle {
		return "", "", WithMachineErrorCode(MachineCodeInvalidArguments,
			errors.New("exactly one of --slug or --title is required"))
	}
	if hasSlug {
		return slugify(input.Slug), input.Slug, nil
	}
	return capSlug(slugify(input.Title)), input.Title, nil
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
	// follows), a carriage return or newline (the line ends), or the end of the
	// body ends the id token.
	if rest != "" && !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, "\n") && !strings.HasPrefix(rest, "\r") {
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
func renameChanges(oldID, newID, oldPath, newPath, body string, inbound []*Task) []RenameChange {
	changes := []RenameChange{
		{Kind: "frontmatter_id", TaskID: oldID, From: oldID, To: newID},
		{Kind: "file_rename", TaskID: oldID, From: oldPath, To: newPath},
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

// renamePreview returns copies of state and the task set with the rename applied
// in memory, so a dry run can validate the state the rename would produce while
// writing nothing. It mirrors the coupled edits renameCandidates makes — the
// target's id/filename/body heading, each inbound dependency edge, and the
// current_task pointer when it names the task — but on copies: the caller still
// reports the old id, and validateInMemory must see the pending change set, not a
// mutation of the loaded tasks it previews.
func renamePreview(state *State, tasks []*Task, target *Task, inbound []*Task, oldID, newID, newPath string) (*State, []*Task) {
	preview := make([]*Task, len(tasks))
	for i, task := range tasks {
		switch {
		case task == target:
			edited := *task
			edited.Frontmatter.ID = newID
			edited.Filename = newPath
			preview[i] = &edited
		case slices.Contains(inbound, task):
			edited := *task
			edited.Frontmatter.Dependencies = slices.Clone(task.Frontmatter.Dependencies)
			for j, dep := range edited.Frontmatter.Dependencies {
				if dep == oldID {
					edited.Frontmatter.Dependencies[j] = newID
				}
			}
			preview[i] = &edited
		default:
			preview[i] = task
		}
	}
	previewState := *state
	if previewState.Frontmatter.CurrentTask == oldID {
		previewState.Frontmatter.CurrentTask = newID
	}
	return &previewState, preview
}

// renameCandidates builds the rename's exact published set as bytes: the removal
// of the old path, the renamed task (its id and updated_at frontmatter lines
// patched and a leading H1 naming the old id repointed), and each inbound
// dependency reference rewritten with its timestamp. Every other byte of every
// written file — including fields no Taskrail struct models — survives exactly.
func (s *Service) renameCandidates(target *Task, inbound []*Task, oldID, newID, newPath, now string) ([]repotx.Candidate, error) {
	data, err := os.ReadFile(target.Filename)
	if err != nil {
		return nil, fmt.Errorf("read task %s: %w", target.Path, fsCause(err))
	}
	frontmatter, body, newline, err := splitTaskDocument(data, oldID)
	if err != nil {
		return nil, err
	}
	if frontmatter, err = replaceTaskField(frontmatter, newline, oldID, "id", newID); err != nil {
		return nil, err
	}
	if frontmatter, err = replaceTaskField(frontmatter, newline, oldID, "updated_at", strconv.Quote(now)); err != nil {
		return nil, err
	}
	// The raw body keeps whatever blank lines follow the frontmatter fence, so
	// the heading rewrite runs past them and they survive in the published bytes.
	trimmed := strings.TrimLeft(body, "\r\n")
	leading := body[:len(body)-len(trimmed)]
	rewrittenBody, _ := renameBodyHeading(trimmed, oldID, newID)

	published := []repotx.Candidate{
		managedCandidate(s.paths.logicalManagedPath(newPath), newPath, []byte(frontmatter+leading+rewrittenBody)),
	}
	// Healing a filename/id drift can rename the task onto the very file it
	// already occupies; that case publishes one replacement, not a removal
	// paired with a creation of the same path.
	if filepath.Clean(target.Filename) != filepath.Clean(newPath) {
		published = append(published, managedRemoval(s.paths.logicalManagedPath(target.Filename), target.Filename))
	}
	for _, task := range inbound {
		candidate, err := s.renamedDependentCandidate(task, oldID, newID, now)
		if err != nil {
			return nil, err
		}
		published = append(published, candidate)
	}
	return published, nil
}

// renamedDependentCandidate patches one inbound task's dependencies references
// and timestamp in place, preserving the field's layout byte for byte.
func (s *Service) renamedDependentCandidate(task *Task, oldID, newID, now string) (repotx.Candidate, error) {
	data, err := os.ReadFile(task.Filename)
	if err != nil {
		return repotx.Candidate{}, fmt.Errorf("read task %s: %w", task.Path, fsCause(err))
	}
	frontmatter, body, newline, err := splitTaskDocument(data, task.Frontmatter.ID)
	if err != nil {
		return repotx.Candidate{}, err
	}
	frontmatter, err = rewriteDependencyRefs(frontmatter, newline, task.Frontmatter.ID, oldID, newID)
	if err != nil {
		return repotx.Candidate{}, err
	}
	frontmatter, err = replaceTaskField(frontmatter, newline, task.Frontmatter.ID, "updated_at", strconv.Quote(now))
	if err != nil {
		return repotx.Candidate{}, err
	}
	return managedCandidate(task.Path, task.Filename, []byte(frontmatter+body)), nil
}

// rewriteDependencyRefs repoints every reference to oldID inside the
// dependencies field — a `- <oldID>` block entry or an exact token in an inline
// list — preserving the field's existing layout byte for byte so the rename
// never reformats a dependent's frontmatter.
func rewriteDependencyRefs(frontmatter, newline, taskID, oldID, newID string) (string, error) {
	lines := strings.Split(frontmatter, newline)
	start, end := -1, -1
	inFrontmatter := false
	for i, line := range lines {
		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			if start >= 0 {
				end = i
			}
			break
		}
		if !inFrontmatter {
			continue
		}
		if start < 0 && (line == "dependencies:" || strings.HasPrefix(line, "dependencies: ") || strings.HasPrefix(line, "dependencies:\t")) {
			start = i
			continue
		}
		if start >= 0 && line != "" && line[0] != ' ' && line[0] != '\t' {
			end = i
			break
		}
	}
	if start < 0 || end < 0 {
		return "", fmt.Errorf("task %s frontmatter has no bounded dependencies field", taskID)
	}
	headerValue := strings.TrimSpace(strings.TrimPrefix(lines[start], "dependencies:"))
	if headerValue != "" && !strings.HasPrefix(headerValue, "#") {
		lines[start] = replaceIDTokens(lines[start], oldID, newID)
		return strings.Join(lines, newline), nil
	}
	for i := start + 1; i < end; i++ {
		if strings.TrimSpace(lines[i]) == "- "+oldID {
			line := lines[i]
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "- " + newID
		}
	}
	return strings.Join(lines, newline), nil
}

// isIDByte reports whether b can appear inside a task id token: letters, digits,
// and the hyphen a slug is built from. Everything else — brackets, commas,
// spaces — ends a token, so an exact token match can never rewrite a longer id
// that merely starts with the old one.
func isIDByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9', b == '-':
		return true
	default:
		return false
	}
}

// replaceIDTokens replaces every exact oldID token in text with newID, leaving
// every separator and non-matching byte untouched. It is the inline-list
// counterpart of the block-entry rewrite above.
func replaceIDTokens(text, oldID, newID string) string {
	var out strings.Builder
	for i := 0; i < len(text); {
		if !isIDByte(text[i]) {
			out.WriteByte(text[i])
			i++
			continue
		}
		j := i
		for j < len(text) && isIDByte(text[j]) {
			j++
		}
		if text[i:j] == oldID {
			out.WriteString(newID)
		} else {
			out.WriteString(text[i:j])
		}
		i = j
	}
	return out.String()
}
