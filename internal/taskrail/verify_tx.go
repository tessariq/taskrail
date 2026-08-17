package taskrail

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tessariq/taskrail/internal/repotx"
)

// The verification transaction substrate
// (specs/v0.5.0.md#repository-discovery-locking-and-recovery): `verify`,
// including `--create-followup`, publishes its artifacts, selected task,
// fresh follow-up tasks, and re-projected state through one normal
// transaction. Verification identity and completion-binding semantics are
// deliberately not touched here; this file owns publication only.

// verifyLedger is the complete candidate set one verification validated and
// published. original is the under-lock corpus before any follow-up joined it;
// preview is the complete candidate ledger state is re-projected from.
type verifyLedger struct {
	state     *State
	task      *Task
	followups []*Task
	artifacts []repotx.Candidate
	original  []*Task
	preview   []*Task
	corpus    []string
	baseline  ValidationResult
}

// verifyWriteClaim derives the exact reported write set one verify will
// publish, before the lock is taken: a delegated child joins its parent's lock
// presenting exactly these paths, so the set must be reconstructible from the
// request and the repository alone. The artifact timestamp is fixed up front,
// and a follow-up's path is derived from a pre-lock read of the task corpus —
// the transaction re-derives the follow-up under the lock and refuses (rather
// than publish outside the claimed set) if the corpus moved in between.
func (s *Service) verifyWriteClaim(input VerifyInput, ts string) ([]string, error) {
	artifactDir := filepath.Join(s.paths.VerifyDir, input.TaskID, ts)
	writes := []string{
		s.reportedStatePath(),
		s.reportedTaskPath(input.TaskID),
		relPath(s.paths.RepoRoot, filepath.Join(artifactDir, "plan.md")),
		relPath(s.paths.RepoRoot, filepath.Join(artifactDir, "report.json")),
		relPath(s.paths.RepoRoot, filepath.Join(artifactDir, "report.md")),
	}
	if !input.CreateFollowup {
		return writes, nil
	}
	tasks, err := s.loadTasks()
	if err != nil {
		return nil, err
	}
	task, err := exactTaskByID(tasks, input.TaskID)
	if err != nil {
		return nil, err
	}
	followup, _, err := s.createFollowupTask(tasks, task, input)
	if err != nil {
		return nil, err
	}
	return append(writes, followup.Path), nil
}

// commitVerify validates the complete candidate ledger and publishes it as one
// normal transaction: the re-projected state, the selected task (patched to
// preserve every unmodeled frontmatter byte), each fresh follow-up task, and
// the verification artifacts.
func (s *Service) commitVerify(own repotx.Ownership, ledger verifyLedger) error {
	validation := s.validateInMemory(ledger.state, ledger.preview)
	if candidateIntroducesViolations(validation, ledger.baseline) {
		return WithMachineErrorCode(MachineCodeValidationFailed,
			fmt.Errorf("verify candidate failed validation: %s", strings.Join(validation.Violations, "; ")))
	}

	stateBytes, err := marshalFrontmatter(ledger.state.Frontmatter, ledger.state.Body)
	if err != nil {
		return err
	}
	taskBytes, err := patchLifecycleTask(ledger.task, map[string]string{
		"updated_at": strconv.Quote(ledger.task.Frontmatter.UpdatedAt),
	})
	if err != nil {
		return err
	}
	published := []repotx.Candidate{
		managedCandidate(s.reportedStatePath(), s.paths.StateFile, stateBytes),
		managedCandidate(ledger.task.Path, ledger.task.Filename, taskBytes),
	}
	for _, followup := range ledger.followups {
		followupBytes, err := marshalFrontmatter(followup.Frontmatter, followup.Body)
		if err != nil {
			return err
		}
		published = append(published, managedCandidate(followup.Path, followup.Filename, followupBytes))
	}
	published = append(published, ledger.artifacts...)

	consumed, err := writerConsumedPaths(s.paths, ledger.original, ledger.task)
	if err != nil {
		return err
	}
	request := repotx.Request{
		Command:      lifecycleVerify.command,
		SelectedTask: ledger.task.Frontmatter.ID,
		TaskFields:   lifecycleVerify.taskFields,
		Consumed:     consumed,
		Published:    published,
		Validate: func([]repotx.Snapshot) error {
			if testHookWriterValidated != nil {
				testHookWriterValidated()
			}
			currentTasks, err := s.loadTasks()
			if err != nil {
				return err
			}
			if !sameTaskCorpus(ledger.corpus, currentTasks) {
				return fmt.Errorf("verify task corpus changed during candidate validation")
			}
			if got := s.validateInMemory(ledger.state, ledger.preview); candidateIntroducesViolations(got, ledger.baseline) {
				return fmt.Errorf("verify candidate failed validation: %s", strings.Join(got.Violations, "; "))
			}
			return nil
		},
	}
	if _, err := repotx.Commit(context.Background(), own, request); err != nil {
		return writerTransactionError(err)
	}
	return nil
}

// worktreeCandidate is one repository-root physical publication, the class the
// machine contract assigns verification artifact paths. The transaction's
// directory creation and rollback own the artifact tree; nothing lands before
// the candidate ledger validates.
func worktreeCandidate(reported, physical string, content []byte) repotx.Candidate {
	return repotx.Candidate{
		Path:    repotx.Path{Kind: repotx.Worktree, Reported: filepath.ToSlash(reported), Physical: physical},
		Content: content,
	}
}
