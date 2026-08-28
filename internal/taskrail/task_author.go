package taskrail

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/tessariq/taskrail/internal/repotx"
)

// TaskAuthorInput binds a reviewed three-section body to one todo task.
type TaskAuthorInput struct {
	TaskID       string
	BodyPath     string
	ExpectSHA256 string
	DryRun       bool
}

// TaskAuthorResult reports the exact selected-task candidate in both preview and
// apply modes. No state projection changes because task authoring changes no
// frontmatter or lifecycle field.
type TaskAuthorResult struct {
	TaskID           string           `json:"task_id"`
	TaskPath         string           `json:"task_path"`
	TaskSHA256Before string           `json:"task_sha256_before"`
	TaskSHA256After  string           `json:"task_sha256_after"`
	Applied          bool             `json:"applied"`
	Diff             string           `json:"diff"`
	Validation       ValidationResult `json:"validation"`
}

// TaskAuthor replaces only the reviewed body sections of a todo task. The
// transaction includes the task corpus and validation inputs so the selected
// byte patch cannot publish over a concurrent semantic change.
func (s *Service) TaskAuthor(input TaskAuthorInput) (result TaskAuthorResult, err error) {
	if input.TaskID == "" || input.BodyPath == "" {
		return result, invalidArgumentsf("task id and body are required")
	}
	if !reviewDigest.MatchString(input.ExpectSHA256) {
		return result, WithMachineErrorCode(MachineCodeInvalidDigest, fmt.Errorf("expected task digest must be lower-case 64-hex"))
	}
	if delegatedInvocation() {
		return result, WithMachineErrorCode(MachineCodeDelegatedRefused, fmt.Errorf("delegated loop children cannot invoke task author"))
	}
	if err := s.requireLayout2ForTaskAuthor(); err != nil {
		return result, err
	}

	var own repotx.Ownership
	var release func() error
	if !input.DryRun {
		own, release, err = s.beginTaskWriterWrite(taskAuthorWriter)
		if err != nil {
			return result, err
		}
		defer func() {
			if releaseErr := release(); releaseErr != nil && err == nil {
				err = releaseErr
			}
		}()
		if testHookTaskWriterLocked != nil {
			testHookTaskWriterLocked()
		}
	}

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return result, err
	}
	corpus, err := snapshotTaskCorpus(tasks)
	if err != nil {
		return result, err
	}
	target, err := exactTaskByID(tasks, input.TaskID)
	if err != nil {
		return result, err
	}
	before, err := os.ReadFile(target.Filename)
	if err != nil {
		return result, fmt.Errorf("read task %s: %w", target.Path, fsCause(err))
	}
	if digestRaw(before) != input.ExpectSHA256 {
		return result, WithMachineErrorCode(MachineCodeWriteConflict, fmt.Errorf("task %s does not match expected digest", input.TaskID))
	}
	exactFrontmatter, _, err := parseFrontmatter[TaskFrontmatter](before)
	if err != nil {
		return result, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("parse task %s: %w", input.TaskID, err))
	}
	if exactFrontmatter.ID != input.TaskID {
		return result, WithMachineErrorCode(MachineCodeWriteConflict, fmt.Errorf("task %s identity changed while reading", input.TaskID))
	}
	if exactFrontmatter.Status != "todo" {
		return result, WithMachineErrorCode(MachineCodeInvalidStatus, fmt.Errorf("task %s is %s: task author targets todo work", input.TaskID, exactFrontmatter.Status))
	}
	proposal, err := s.readTaskAuthorProposal(input.BodyPath)
	if err != nil {
		return result, err
	}
	after, candidateBody, err := authorTaskBytes(before, input.TaskID, proposal)
	if err != nil {
		return result, WithMachineErrorCode(MachineCodeInvalidProposal, err)
	}
	preview := withTaskBody(tasks, target, candidateBody)
	validation := s.validateInMemory(state, preview)
	if !validation.Valid {
		return result, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("task author candidate failed validation: %s", strings.Join(validation.Violations, "; ")))
	}
	result = TaskAuthorResult{
		TaskID: input.TaskID, TaskPath: target.Path, TaskSHA256Before: digestRaw(before),
		TaskSHA256After: digestRaw(after), Diff: taskAuthorDiff(target.Path, before, after), Validation: validation,
	}
	if input.DryRun {
		return result, nil
	}

	consumed, err := writerConsumedPaths(s.paths, tasks, target)
	if err != nil {
		return TaskAuthorResult{}, err
	}
	consumed = append(consumed, repotx.Path{Kind: repotx.Managed, Reported: s.reportedStatePath(), Physical: s.paths.StateFile})
	request := repotx.Request{
		Command: "task author", SelectedTask: input.TaskID, TaskFields: taskAuthorWriter.taskFields,
		Consumed: consumed, Published: []repotx.Candidate{managedCandidate(target.Path, target.Filename, after)},
		ExpectedOriginalSHA256: map[string]string{target.Path: result.TaskSHA256Before},
		Validate: func([]repotx.Snapshot) error {
			if err := s.validateWriterStorage(); err != nil {
				return err
			}
			currentTasks, err := s.loadTasks()
			if err != nil {
				return err
			}
			if !sameTaskCorpus(corpus, currentTasks) {
				return fmt.Errorf("task author task corpus changed during candidate validation")
			}
			if testHookWriterValidated != nil {
				testHookWriterValidated()
			}
			if got := s.validateInMemory(state, preview); !got.Valid {
				return fmt.Errorf("task author candidate failed validation: %s", strings.Join(got.Violations, "; "))
			}
			return nil
		},
	}
	if testHookWriterCandidateBuilt != nil {
		testHookWriterCandidateBuilt()
	}
	if _, err := repotx.Commit(context.Background(), own, request); err != nil {
		return TaskAuthorResult{}, taskAuthorTransactionError(err)
	}
	result.Applied = true
	return result, nil
}

func taskAuthorTransactionError(err error) error {
	var txErr *repotx.Error
	if errors.As(err, &txErr) && txErr.Kind == repotx.KindValidation {
		return WithMachineErrorCode(MachineCodeInvalidProposal, err)
	}
	return writerTransactionError(err)
}

func (s *Service) requireLayout2ForTaskAuthor() error {
	config, found, err := readMarker(s.paths.RepoRoot)
	if err != nil {
		return err
	}
	if !found || config.LayoutVersion != layout2Version {
		return WithMachineErrorCode(MachineCodeIncompatibleLayout, fmt.Errorf("task author requires layout_version 2"))
	}
	return nil
}

func (s *Service) readTaskAuthorProposal(bodyPath string) ([]byte, error) {
	if filepath.IsAbs(bodyPath) || filepath.ToSlash(bodyPath) != bodyPath || path.Clean(bodyPath) != bodyPath {
		return nil, invalidArgumentsf("body must be a canonical repository-relative path")
	}
	artifactPrefix := s.paths.LogicalPlanningDir + "/artifacts/"
	if strings.HasPrefix(bodyPath, artifactPrefix) {
		return nil, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("body proposal must not be under ignored artifacts"))
	}
	physical := filepath.Join(s.paths.RepoRoot, filepath.FromSlash(bodyPath))
	if !pathWithinRepository(s.paths.RepoRoot, physical) {
		return nil, invalidArgumentsf("body must remain within the repository")
	}
	info, err := os.Stat(physical)
	if err != nil {
		return nil, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("read body proposal: %w", fsCause(err)))
	}
	if !info.Mode().IsRegular() {
		return nil, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("body proposal is not a regular file"))
	}
	data, err := os.ReadFile(physical)
	if err != nil {
		return nil, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("read body proposal: %w", fsCause(err)))
	}
	if err := validateTaskAuthorProposal(data); err != nil {
		return nil, WithMachineErrorCode(MachineCodeInvalidProposal, err)
	}
	return data, nil
}

func validateTaskAuthorProposal(data []byte) error {
	if !utf8.Valid(data) || strings.HasPrefix(string(data), "\ufeff") {
		return fmt.Errorf("body proposal must be UTF-8 without a BOM")
	}
	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := markdownLinesWithoutFencedContent(normalized)
	if len(lines) > 0 && lines[0] == "---" {
		return fmt.Errorf("body proposal contains frontmatter")
	}
	expected := []string{"Description", "Acceptance", "Verification Notes"}
	seen := make([]int, 0, len(expected))
	for i, line := range lines {
		level, heading := markdownATXHeading(line)
		if level == 1 || (i > 0 && strings.TrimSpace(lines[i-1]) != "" && markdownSetextH1Underline(line)) {
			return fmt.Errorf("body proposal contains top-level heading")
		}
		if level == 2 {
			index := -1
			for j, want := range expected {
				if heading == want {
					index = j
				}
			}
			if index < 0 {
				return fmt.Errorf("body proposal contains forbidden level-2 heading %q", heading)
			}
			seen = append(seen, index)
		}
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("body proposal must contain exactly Description, Acceptance, and Verification Notes sections")
	}
	for i, index := range seen {
		if index != i {
			return fmt.Errorf("body proposal sections must be ordered Description, Acceptance, and Verification Notes")
		}
		start, end := indexMarkdownHeading(lines, "## "+expected[i])+1, len(lines)
		for j := start; j < len(lines); j++ {
			if level, _ := markdownATXHeading(lines[j]); level == 2 {
				end = j
				break
			}
		}
		if strings.TrimSpace(strings.Join(lines[start:end], "\n")) == "" {
			return fmt.Errorf("body proposal section %q is empty", expected[i])
		}
	}
	return nil
}

func authorTaskBytes(before []byte, taskID string, proposal []byte) ([]byte, string, error) {
	frontmatter, body, newline, err := splitTaskDocument(before, taskID)
	if err != nil {
		return nil, "", err
	}
	sectionStart, notesStart, err := authorableTaskSectionBounds(body, newline, taskID)
	if err != nil {
		return nil, "", err
	}
	newBody := body[:sectionStart] + string(proposal)
	if !strings.HasSuffix(newBody, newline) {
		newBody += newline
	}
	if !strings.HasSuffix(newBody, newline+newline) {
		newBody += newline
	}
	newBody += body[notesStart:]
	return []byte(frontmatter + newBody), newBody, nil
}

// authorableTaskSectionBounds finds only real level-two headings, rejecting a
// target where an unrelated heading would otherwise be silently deleted.
func authorableTaskSectionBounds(body, newline, taskID string) (int, int, error) {
	rawLines := strings.Split(body, newline)
	safeLines := markdownLinesWithoutFencedContent(strings.ReplaceAll(body, "\r\n", "\n"))
	expected := []string{"Description", "Acceptance", "Verification Notes", "Implementation Notes"}
	positions := make([]int, len(expected))
	for i := range positions {
		positions[i] = -1
	}
	offset := 0
	for i, raw := range rawLines {
		level, heading := markdownATXHeading(safeLines[i])
		if level == 2 {
			match := -1
			for j, want := range expected {
				if heading == want {
					match = j
					break
				}
			}
			if match >= 0 {
				if positions[match] >= 0 {
					return 0, 0, fmt.Errorf("task %s repeats authorable section %q", taskID, heading)
				}
				positions[match] = offset
			} else if positions[0] >= 0 && positions[3] < 0 {
				return 0, 0, fmt.Errorf("task %s has a level-two heading between Description and Implementation Notes", taskID)
			}
		}
		offset += len(raw)
		if i < len(rawLines)-1 {
			offset += len(newline)
		}
	}
	for i, position := range positions {
		if position < 0 {
			return 0, 0, fmt.Errorf("task %s does not contain authorable section %q", taskID, expected[i])
		}
		if i > 0 && positions[i-1] >= position {
			return 0, 0, fmt.Errorf("task %s authorable sections are out of order", taskID)
		}
	}
	return positions[0], positions[3], nil
}

func withTaskBody(tasks []*Task, target *Task, body string) []*Task {
	preview := make([]*Task, len(tasks))
	for i, task := range tasks {
		if task != target {
			preview[i] = task
			continue
		}
		edited := *task
		edited.Body = body
		preview[i] = &edited
	}
	return preview
}

func taskAuthorDiff(path string, before, after []byte) string {
	if string(before) == string(after) {
		return ""
	}
	oldLines := strings.Split(strings.ReplaceAll(string(before), "\r\n", "\n"), "\n")
	newLines := strings.Split(strings.ReplaceAll(string(after), "\r\n", "\n"), "\n")
	prefix := 0
	for prefix < len(oldLines) && prefix < len(newLines) && oldLines[prefix] == newLines[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(oldLines)-prefix && suffix < len(newLines)-prefix && oldLines[len(oldLines)-suffix-1] == newLines[len(newLines)-suffix-1] {
		suffix++
	}
	const contextLines = 3
	oldStart := max(0, prefix-contextLines)
	newStart := max(0, prefix-contextLines)
	oldEnd := min(len(oldLines), len(oldLines)-suffix+contextLines)
	newEnd := min(len(newLines), len(newLines)-suffix+contextLines)
	var diff strings.Builder
	fmt.Fprintf(&diff, "--- %s\n+++ %s\n@@ -%d,%d +%d,%d @@\n", path, path, oldStart+1, oldEnd-oldStart, newStart+1, newEnd-newStart)
	for _, line := range oldLines[oldStart:prefix] {
		fmt.Fprintf(&diff, " %s\n", line)
	}
	for _, line := range oldLines[prefix : len(oldLines)-suffix] {
		fmt.Fprintf(&diff, "-%s\n", line)
	}
	for _, line := range newLines[prefix : len(newLines)-suffix] {
		fmt.Fprintf(&diff, "+%s\n", line)
	}
	for _, line := range oldLines[len(oldLines)-suffix : oldEnd] {
		fmt.Fprintf(&diff, " %s\n", line)
	}
	return diff.String()
}
