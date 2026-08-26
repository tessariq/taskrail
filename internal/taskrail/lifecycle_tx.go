package taskrail

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tessariq/taskrail/internal/repolock"
	"github.com/tessariq/taskrail/internal/repotx"
)

// The tracked-work transaction substrate
// (specs/v0.5.0.md#repository-discovery-locking-and-recovery): `next`, `start`,
// `complete`, `block`, `unblock`, `task release`, and `verify` publish through one normal
// transaction. Each writer takes the discovered repository mutation lock,
// snapshots its complete consumed and published set, validates the full
// candidate ledger before the first write, and publishes only its declared
// files — never the save-all rewrite that re-encodes unselected task bytes.

// testHookWriterValidated runs at the start of one writer transaction's
// candidate-validation phase: after the snapshot, before the recheck and the
// first publication. Tests use it to change the repository mid-transaction —
// the only window where a conflict or an invalid candidate is deterministically
// reachable from this package — and to observe the lock a writer must hold. It
// is nil outside tests.
var testHookWriterValidated func()

// testHookWriterCandidateBuilt runs after a transactional writer captures its
// candidate corpus and before repotx takes its transaction snapshot. Tests use
// it to exercise edits that parsed task fields alone cannot observe.
var testHookWriterCandidateBuilt func()

// writerCommand names one tracked-work writer and the task fields it writes.
// The fields double as the capability bound a delegated child is held to, so
// every declared field must stay inside repolock's delegated field bound.
type writerCommand struct {
	command    string
	taskFields []string
}

var (
	lifecycleNext = writerCommand{command: "next"}
	// A state-selection writer reads every task but writes none of them.
	lifecycleStart = writerCommand{command: "start", taskFields: []string{"status", "updated_at"}}
	// Block also records the reason in the STATE.md blockers ledger, which the
	// delegated field set names through its "blocker" member.
	lifecycleComplete = writerCommand{command: "complete", taskFields: []string{"status", "updated_at", "implementation_notes", "completion_id", "last_verification_id", "last_verification_previous_id", "last_verification_result", "last_verified_at", "last_verified_completion_id"}}
	lifecycleBlock    = writerCommand{command: "block", taskFields: []string{"status", "updated_at", "implementation_notes", "blocker"}}
	lifecycleUnblock  = writerCommand{command: "unblock", taskFields: []string{"status", "updated_at", "implementation_notes"}}
	lifecycleRelease  = writerCommand{command: "task release", taskFields: []string{"status", "updated_at", "implementation_notes"}}
	// Verify records its immutable identity tuple without changing lifecycle
	// status; its follow-up publication is owned by verify_tx.go.
	lifecycleVerify = writerCommand{command: "verify", taskFields: []string{"updated_at", "implementation_notes", "completion_id", "last_verification_id", "last_verification_previous_id", "last_verification_result", "last_verified_at", "last_verified_completion_id"}}
)

// beginWriterWrite takes the repository mutation lock for one tracked-work
// writer. A direct operator acquires it bounded by the command it typed; a
// delegated child joins its parent's already-held lock, arriving narrowed to
// the selected task and the exact write set its grant authenticated. The
// returned release retires only a directly acquired lock — a delegate never
// releases ownership it does not hold.
func (s *Service) beginWriterWrite(w writerCommand, selectedTask string, writes []string) (repotx.Ownership, func() error, error) {
	if err := s.paths.ensureStorageCapability(); err != nil {
		return nil, nil, err
	}
	if delegatedInvocation() {
		identity, err := delegatedWriterIdentity()
		if err != nil {
			return nil, nil, WithMachineErrorCode(MachineCodeDelegatedRefused,
				fmt.Errorf("delegated %s has invalid loop identity: %w", w.command, err))
		}
		joined, err := repolock.Join(repolock.JoinRequest{
			Repository:       s.paths.LockRepository(),
			Command:          w.command,
			InvocationID:     identity.invocationID,
			Token:            identity.token,
			ExecutableSHA256: identity.executableSHA256,
			Grant:            s.loopDelegationGrant(selectedTask),
			Capability:       repolock.Capability{Commands: []string{w.command}, TaskFields: w.taskFields, SelectedTask: selectedTask, Writes: writes},
		})
		if err != nil {
			return nil, nil, WithMachineErrorCode(MachineCodeDelegatedRefused,
				fmt.Errorf("delegated %s could not join its parent's repository lock: %w", w.command, err))
		}
		if err := s.validateWriterStorage(); err != nil {
			return nil, nil, err
		}
		return joined, func() error { return nil }, nil
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

// loopDelegationGrant is the stable task-scoped authority a loop authenticates
// before a child narrows to one lifecycle transaction. Verification destinations
// include a runtime-generated timestamp and ID, so its selected-task prefix is
// part of the canonical grant rather than a concrete writer claim.
func (s *Service) loopDelegationGrant(taskID string) repolock.Capability {
	return repolock.Capability{SelectedTask: taskID, Writes: []string{
		s.reportedStatePath(),
		path.Join(s.paths.LogicalPlanningDir, "tasks") + "/",
		relPath(s.paths.RepoRoot, filepath.Join(s.paths.VerifyDir, taskID)) + "/",
	}}
}

type delegatedIdentity struct {
	invocationID     string
	token            string
	executableSHA256 string
}

func delegatedWriterIdentity() (delegatedIdentity, error) {
	values := make(map[string]string, len(loopChildEnvironmentNames))
	for _, name := range loopChildEnvironmentNames {
		value, present := os.LookupEnv(name)
		if !present || value == "" {
			return delegatedIdentity{}, fmt.Errorf("%s is required", name)
		}
		values[name] = value
	}
	if !lowerHex32.MatchString(values["TASKRAIL_DELEGATION_ID"]) {
		return delegatedIdentity{}, errors.New("TASKRAIL_DELEGATION_ID must be lower-case 32-hex")
	}
	stagedPath := values["TASKRAIL"]
	if !filepath.IsAbs(stagedPath) || filepath.Clean(stagedPath) != stagedPath {
		return delegatedIdentity{}, errors.New("TASKRAIL must name a clean absolute path")
	}
	staged, err := os.Lstat(stagedPath)
	if err != nil || !staged.Mode().IsRegular() {
		return delegatedIdentity{}, errors.New("TASKRAIL must name a regular staged executable")
	}
	runningPath, err := os.Executable()
	if err != nil {
		return delegatedIdentity{}, fmt.Errorf("resolve running executable: %w", err)
	}
	running, err := os.Stat(runningPath)
	if err != nil || !running.Mode().IsRegular() || !os.SameFile(staged, running) {
		return delegatedIdentity{}, errors.New("running executable does not match TASKRAIL")
	}
	stagedDigest, err := repolock.ExecutableDigest(stagedPath)
	if err != nil || stagedDigest != values["TASKRAIL_EXECUTABLE_SHA256"] {
		return delegatedIdentity{}, errors.New("TASKRAIL digest does not match TASKRAIL_EXECUTABLE_SHA256")
	}
	runningDigest, err := repolock.ExecutableDigest(runningPath)
	if err != nil || runningDigest != stagedDigest {
		return delegatedIdentity{}, errors.New("running executable digest does not match TASKRAIL")
	}
	return delegatedIdentity{
		invocationID: values["TASKRAIL_DELEGATION_ID"],
		token:        values["TASKRAIL_DELEGATION_TOKEN"], executableSHA256: stagedDigest,
	}, nil
}

// lifecycleLedger is the complete candidate set one lifecycle writer validated
// and published: the re-projected state, the one task file it owns (nil for
// `next`), the mutated task set the render and the validation both saw, and the
// pre-transition verdict. A lifecycle transition preserves violations that
// already existed — it reports them rather than healing or refusing them — so
// the baseline is what "the candidate introduced nothing new" is measured
// against.
type lifecycleLedger struct {
	state            *State
	task             *Task
	preview          []*Task
	corpus           []string
	baseline         ValidationResult
	taskSHA256Before string
}

// candidateIntroducesViolations reports whether the candidate verdict contains
// a violation the baseline lacked: a write that would make the repository worse
// than it already was. Pre-existing violations pass through into the reported
// result, matching the shipped lifecycle contract (a transition neither heals
// nor is blocked by an already-invalid repository).
func candidateIntroducesViolations(candidate, baseline ValidationResult) bool {
	known := make(map[string]struct{}, len(baseline.Violations))
	for _, violation := range baseline.Violations {
		known[violation] = struct{}{}
	}
	for _, violation := range candidate.Violations {
		if _, ok := known[violation]; !ok {
			return true
		}
	}
	return false
}

// commitLifecycle validates the complete candidate ledger and publishes it as
// one normal transaction: the state file plus, when a task is selected, exactly
// that task's file. The returned validation is the pre-publication verdict the
// writer reports in its result.
func (s *Service) commitLifecycle(own repotx.Ownership, w writerCommand, ledger lifecycleLedger) (ValidationResult, error) {
	validation := s.validateInMemory(ledger.state, ledger.preview)
	if candidateIntroducesViolations(validation, ledger.baseline) {
		return ValidationResult{}, WithMachineErrorCode(MachineCodeValidationFailed,
			fmt.Errorf("%s candidate failed validation: %s", w.command, strings.Join(validation.Violations, "; ")))
	}

	stateBytes, err := marshalFrontmatter(ledger.state.Frontmatter, ledger.state.Body)
	if err != nil {
		return ValidationResult{}, err
	}
	published := []repotx.Candidate{
		managedCandidate(s.reportedStatePath(), s.paths.StateFile, stateBytes),
	}
	selectedTask := ""
	if ledger.task != nil {
		taskBytes, err := patchLifecycleTask(ledger.task, map[string]string{
			"status":     ledger.task.Frontmatter.Status,
			"updated_at": strconv.Quote(ledger.task.Frontmatter.UpdatedAt),
		})
		if err != nil {
			return ValidationResult{}, err
		}
		if w.command == lifecycleComplete.command {
			taskBytes, err = patchCompletionMetadata(taskBytes, ledger.task)
			if err != nil {
				return ValidationResult{}, err
			}
		}
		published = append(published, managedCandidate(ledger.task.Path, ledger.task.Filename, taskBytes))
		selectedTask = ledger.task.Frontmatter.ID
	}
	consumed, err := writerConsumedPaths(s.paths, ledger.preview, ledger.task)
	if err != nil {
		return ValidationResult{}, err
	}

	request := repotx.Request{
		Command:                w.command,
		SelectedTask:           selectedTask,
		TaskFields:             w.taskFields,
		Consumed:               consumed,
		Published:              published,
		ExpectedOriginalSHA256: expectedLifecycleTaskDigest(ledger),
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
			if got := s.validateInMemory(ledger.state, ledger.preview); candidateIntroducesViolations(got, ledger.baseline) {
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

func expectedLifecycleTaskDigest(ledger lifecycleLedger) map[string]string {
	if ledger.task == nil || ledger.taskSHA256Before == "" {
		return nil
	}
	return map[string]string{ledger.task.Path: ledger.taskSHA256Before}
}

func patchCompletionMetadata(data []byte, task *Task) ([]byte, error) {
	frontmatter, body, newline, err := splitTaskDocument(data, task.Frontmatter.ID)
	if err != nil {
		return nil, err
	}
	completion := strconv.Quote(task.Frontmatter.CompletionID)
	frontmatter, err = rewriteOptionalTaskField(frontmatter, newline, task.Frontmatter.ID, "completion_id", &completion)
	if err != nil {
		return nil, err
	}
	for _, field := range []string{"last_verification_id", "last_verification_previous_id", "last_verification_result", "last_verified_at", "last_verified_completion_id"} {
		frontmatter, err = rewriteOptionalTaskField(frontmatter, newline, task.Frontmatter.ID, field, nil)
		if err != nil {
			return nil, err
		}
	}
	return []byte(frontmatter + body), nil
}

func patchVerificationMetadata(data []byte, task *Task) ([]byte, error) {
	frontmatter, body, newline, err := splitTaskDocument(data, task.Frontmatter.ID)
	if err != nil {
		return nil, err
	}
	meta := task.Frontmatter.CompletionVerificationMetadata
	values := map[string]*string{
		"completion_id":                 optionalQuotedString(meta.CompletionID),
		"last_verification_id":          verificationFieldPointer(strconv.Quote(meta.LastVerificationID)),
		"last_verification_previous_id": optionalQuotedString(meta.LastVerificationPreviousID),
		"last_verification_result":      verificationFieldPointer(meta.LastVerificationResult),
		"last_verified_at":              verificationFieldPointer(strconv.Quote(meta.LastVerifiedAt)),
		"last_verified_completion_id":   optionalQuotedString(meta.LastVerifiedCompletionID),
	}
	for _, field := range []string{"completion_id", "last_verification_id", "last_verification_previous_id", "last_verification_result", "last_verified_at", "last_verified_completion_id"} {
		frontmatter, err = rewriteOptionalTaskField(frontmatter, newline, task.Frontmatter.ID, field, values[field])
		if err != nil {
			return nil, err
		}
	}
	return []byte(frontmatter + body), nil
}

func verificationFieldPointer(value string) *string { return &value }

func optionalQuotedString(value string) *string {
	if value == "" {
		return nil
	}
	return verificationFieldPointer(strconv.Quote(value))
}

func rewriteOptionalTaskField(frontmatter, newline, taskID, field string, value *string) (string, error) {
	prefix := field + ":"
	lines := strings.Split(frontmatter, newline)
	match := -1
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			if match >= 0 {
				return "", fmt.Errorf("task %s has multiple %s fields", taskID, field)
			}
			match = i
		}
	}
	if match >= 0 {
		if value == nil {
			lines = append(lines[:match], lines[match+1:]...)
		} else {
			lines[match] = prefix + " " + *value
		}
		return strings.Join(lines, newline), nil
	}
	if value == nil {
		return frontmatter, nil
	}
	end := len(lines) - 1
	for end >= 0 && lines[end] != "---" {
		end--
	}
	if end < 1 {
		return "", fmt.Errorf("task %s has no frontmatter end", taskID)
	}
	lines = append(lines, "")
	copy(lines[end+1:], lines[end:])
	lines[end] = prefix + " " + *value
	return strings.Join(lines, newline), nil
}

func snapshotTaskCorpus(tasks []*Task) ([]string, error) {
	result := make([]string, len(tasks))
	for i, task := range tasks {
		result[i] = task.Path + "\x00" + string(task.raw)
	}
	return result, nil
}

func sameTaskCorpus(expected []string, current []*Task) bool {
	got, err := snapshotTaskCorpus(current)
	if err != nil || len(expected) != len(got) {
		return false
	}
	for i := range expected {
		if expected[i] != got[i] {
			return false
		}
	}
	return true
}

// patchLifecycleTask changes only the fields the caller names and the body
// while preserving every unmodeled frontmatter byte on the selected task. The
// field set must match the writer's declared taskFields so a writer's byte
// reach never exceeds its capability bound — verify, for instance, never
// names "status" and so never rewrites that line.
func patchLifecycleTask(task *Task, fields map[string]string) ([]byte, error) {
	data, err := os.ReadFile(task.Filename)
	if err != nil {
		return nil, err
	}
	return patchLifecycleTaskBytes(data, task, fields)
}

func patchLifecycleTaskBytes(data []byte, task *Task, fields map[string]string) ([]byte, error) {
	frontmatter, rawBody, newline, err := splitTaskDocument(data, task.Frontmatter.ID)
	if err != nil {
		return nil, err
	}
	for field, value := range fields {
		frontmatter, err = replaceTaskField(frontmatter, newline, task.Frontmatter.ID, field, value)
		if err != nil {
			return nil, err
		}
	}
	_, originalBody, err := parseFrontmatter[TaskFrontmatter](data)
	if err != nil {
		return nil, err
	}
	if task.Body == originalBody {
		return []byte(frontmatter + rawBody), nil
	}
	base := strings.TrimRight(originalBody, "\n")
	if !strings.HasPrefix(task.Body, base) {
		return nil, fmt.Errorf("task %s lifecycle body change is not append-only", task.Frontmatter.ID)
	}
	addition := task.Body[len(base):]
	if newline == "\r\n" {
		addition = strings.ReplaceAll(addition, "\n", newline)
	}
	return []byte(frontmatter + strings.TrimRight(rawBody, "\r\n") + addition), nil
}

// reportedStatePath is the state file's durable logical identity, the spelling
// a delegated write set is granted and a transaction reports.
func (s *Service) reportedStatePath() string {
	return s.paths.logicalManagedPath(s.paths.StateFile)
}

// reportedTaskPath derives a task operand's logical path before any load proves
// the task exists, so a delegated child can assert the grant it joins with from
// exactly what the operator handed it.
func (s *Service) reportedTaskPath(taskID string) string {
	return path.Join(s.paths.LogicalPlanningDir, "tasks", taskID+".md")
}
