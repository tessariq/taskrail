package taskrail

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tessariq/taskrail/internal/repolock"
	"github.com/tessariq/taskrail/internal/repotx"
)

// testHookTaskWriterLocked runs after rename/repoint have their mutation lock
// and before either reads its candidate. It makes the state-only race boundary
// deterministic in tests.
var testHookTaskWriterLocked func()

// The inherited task-mutation transaction substrate
// (specs/v0.5.0.md#repository-discovery-locking-and-recovery): `task new`,
// `task rename`, `task repoint`, and `task dependency add|remove` publish
// through one normal transaction. Each writer takes the discovered repository
// mutation lock, snapshots its complete consumed and collision set, validates
// the full candidate ledger before the first write, and publishes only its
// declared files. None of these commands is delegable — the delegated command
// bound excludes every one of them — so a delegated invocation is refused
// outright rather than joined.

// taskWriterCommand names one inherited task-mutation writer and the task
// fields its publication rewrites. Unlike the lifecycle writers, the field
// set never becomes a delegated join's bound (delegation is refused before
// any lock exists); it records the writer's declared reach for the lock it
// acquires directly.
type taskWriterCommand struct {
	command    string
	taskFields []string
}

var (
	// Task creation writes a fresh file; no existing task's fields change.
	taskNewWriter     = taskWriterCommand{command: "task new"}
	taskRenameWriter  = taskWriterCommand{command: "task rename", taskFields: []string{"id", "updated_at", "dependencies"}}
	taskRepointWriter = taskWriterCommand{command: "task repoint", taskFields: []string{"spec_ref", "updated_at"}}
	taskAuthorWriter  = taskWriterCommand{command: "task author", taskFields: []string{"description", "acceptance", "verification_notes"}}
)

// dependencyTaskWriter names one exact-ID dependency editor. It rewrites only
// the dependencies field, mirroring the byte patch its candidate publishes.
func dependencyTaskWriter(operation DependencyOperation) taskWriterCommand {
	return taskWriterCommand{command: "task dependency " + string(operation), taskFields: []string{"dependencies"}}
}

func loopPolicyTaskWriter(operation LoopPolicyOperation) taskWriterCommand {
	return taskWriterCommand{command: "task loop " + string(operation), taskFields: []string{"loop_policy", "loop_reason", "updated_at"}}
}

// beginTaskWriterWrite takes the repository mutation lock for one
// non-delegable task-mutation writer. A delegated loop child is refused
// before anything is read: `verify --create-followup` is the only task
// creator delegation permits, and identity, anchoring, and dependency edits
// would widen the work its parent selected.
func (s *Service) beginTaskWriterWrite(w taskWriterCommand) (repotx.Ownership, func() error, error) {
	if err := s.paths.ensureStorageCapability(); err != nil {
		return nil, nil, err
	}
	if delegatedInvocation() {
		return nil, nil, WithMachineErrorCode(MachineCodeDelegatedRefused,
			fmt.Errorf("delegated loop children cannot invoke %s", w.command))
	}
	own, release, err := s.acquireWriterLock(w.command, w.taskFields)
	if err != nil {
		return nil, nil, err
	}
	if err := s.validateWriterStorage(); err != nil {
		return nil, nil, errors.Join(err, release())
	}
	return own, release, nil
}

// validateWriterStorage rechecks the discovered layout after the mutation lock
// is held. A committed semantic tree may appear after discovery in local mode;
// writers must refuse that mixed state rather than silently continue against one
// root.
func (s *Service) validateWriterStorage() error {
	if err := validateDiscoveredPaths(s.paths); err != nil {
		return WithMachineErrorCode(MachineCodeRepositoryInvalid, err)
	}
	return nil
}

// acquireWriterLock directly acquires the mutation lock for one writer's
// declared command and task fields, returning the ownership plus its release.
// Acquiring may create the shared runtime parent; the release refreshes the
// recovered ancestor identity so the command boundary does not mistake this
// writer's own lock directory for recovery activity.
func (s *Service) acquireWriterLock(command string, taskFields []string) (repotx.Ownership, func() error, error) {
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: s.paths.LockRepository(),
		Command:    command,
		Capability: repolock.Capability{Commands: []string{command}, TaskFields: taskFields},
	})
	if err != nil {
		return nil, nil, writerLockError(err)
	}
	release := func() error {
		if releaseErr := lock.Release(); releaseErr != nil {
			return WithMachineErrorCode(MachineCodeRepositoryInvalid, releaseErr)
		}
		return s.refreshRecoveryAfterLock()
	}
	return lock, release, nil
}

// taskWriterLedger is the complete candidate set one task-mutation writer
// validated and published: the candidate state, the corpus projection the
// state body is re-rendered from, the exact published set beyond the state
// file, and the pre-write corpus and validation baseline. written names the
// loaded tasks the published set replaces, so the consumed set covers every
// other task plus the specs and config the candidate was built from.
type taskWriterLedger struct {
	state     *State
	preview   []*Task
	published []repotx.Candidate
	selected  string
	tasks     []*Task
	written   []*Task
	corpus    []string
	baseline  ValidationResult
	// taskSHA256Before binds a selected-task candidate to the bytes it was
	// derived from, so a later unmodeled-byte edit cannot be overwritten.
	taskSHA256Before string
	// strict refuses any invalid candidate. Rename and the dependency
	// editors shipped refusing a preview that leaves the repository invalid;
	// task new and repoint follow the lifecycle rule and refuse only
	// violations the candidate introduces over the baseline.
	strict bool
}

// commitTaskWriter validates the complete candidate ledger and publishes it
// as one normal transaction: the candidate state plus the writer's declared
// task files. The returned validation is the pre-publication verdict the
// writer reports in its result.
func (s *Service) commitTaskWriter(own repotx.Ownership, w taskWriterCommand, ledger taskWriterLedger) (ValidationResult, error) {
	refuses := func(validation ValidationResult) bool {
		if ledger.strict {
			return !validation.Valid
		}
		return candidateIntroducesViolations(validation, ledger.baseline)
	}
	validation := s.validateInMemory(ledger.state, ledger.preview)
	if refuses(validation) {
		return ValidationResult{}, WithMachineErrorCode(MachineCodeValidationFailed,
			fmt.Errorf("%s candidate failed validation: %s", w.command, strings.Join(validation.Violations, "; ")))
	}

	stateBytes, err := marshalFrontmatter(ledger.state.Frontmatter, ledger.state.Body)
	if err != nil {
		return ValidationResult{}, err
	}
	published := append([]repotx.Candidate{
		managedCandidate(s.reportedStatePath(), s.paths.StateFile, stateBytes),
	}, ledger.published...)
	consumed, err := writerConsumedPaths(s.paths, ledger.tasks, ledger.written...)
	if err != nil {
		return ValidationResult{}, err
	}

	request := repotx.Request{
		Command:                w.command,
		SelectedTask:           ledger.selected,
		TaskFields:             w.taskFields,
		Consumed:               consumed,
		Published:              published,
		ExpectedOriginalSHA256: expectedTaskWriterDigest(ledger),
		Validate: func([]repotx.Snapshot) error {
			if testHookWriterValidated != nil {
				testHookWriterValidated()
			}
			if err := s.validateWriterStorage(); err != nil {
				return err
			}
			currentTasks, err := s.loadTasks()
			if err != nil {
				return err
			}
			if !sameTaskCorpus(ledger.corpus, currentTasks) {
				return fmt.Errorf("%s task corpus changed during candidate validation", w.command)
			}
			// The candidate was proven valid above, but validation can take
			// arbitrary time: re-prove it under the snapshot so a repository
			// that changed in between is refused rather than published.
			if got := s.validateInMemory(ledger.state, ledger.preview); refuses(got) {
				return fmt.Errorf("%s candidate failed validation: %s", w.command, strings.Join(got.Violations, "; "))
			}
			return nil
		},
	}
	if testHookWriterCandidateBuilt != nil {
		testHookWriterCandidateBuilt()
	}
	if _, err := repotx.Commit(context.Background(), own, request); err != nil {
		return ValidationResult{}, writerTransactionError(err)
	}
	return validation, nil
}

func expectedTaskWriterDigest(ledger taskWriterLedger) map[string]string {
	if len(ledger.written) != 1 || ledger.taskSHA256Before == "" {
		return nil
	}
	return map[string]string{ledger.written[0].Path: ledger.taskSHA256Before}
}

// managedRemoval is one published absence of a managed semantic path, the
// candidate a rename pairs with its successor file.
func managedRemoval(reported, physical string) repotx.Candidate {
	return repotx.Candidate{
		Path:   repotx.Path{Kind: repotx.Managed, Reported: filepath.ToSlash(reported), Physical: physical},
		Remove: true,
	}
}

// splitTaskDocument splits one task file into its frontmatter block (both
// fences included) and body, remembering the line ending so patches preserve
// the file's own bytes rather than normalizing them.
func splitTaskDocument(data []byte, taskID string) (frontmatter, body, newline string, err error) {
	newline = "\n"
	if strings.HasPrefix(string(data), "---\r\n") {
		newline = "\r\n"
	} else if !strings.HasPrefix(string(data), "---\n") {
		return "", "", "", fmt.Errorf("task %s has no frontmatter start", taskID)
	}
	marker := newline + "---" + newline
	end := strings.Index(string(data[len("---"+newline):]), marker)
	if end < 0 {
		return "", "", "", fmt.Errorf("task %s has no frontmatter end", taskID)
	}
	end += len("---"+newline) + len(marker)
	return string(data[:end]), string(data[end:]), newline, nil
}

// replaceTaskField rewrites the frontmatter's single `field:` line to
// `field: value`, failing when the field is absent or spelled more than once.
// Every other byte — other fields, their order, and any field no Taskrail
// struct models — survives exactly.
func replaceTaskField(frontmatter, newline, taskID, field, value string) (string, error) {
	prefix := field + ":"
	lines := strings.Split(frontmatter, newline)
	matches := 0
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[i] = prefix + " " + value
			matches++
		}
	}
	if matches != 1 {
		return "", fmt.Errorf("task %s has %d %s fields", taskID, matches, field)
	}
	return strings.Join(lines, newline), nil
}
