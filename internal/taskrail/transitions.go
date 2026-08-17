package taskrail

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tessariq/taskrail/internal/repotx"
)

func (s *Service) Next() (NextResult, error) {
	return s.next(false)
}

// NextIncludingOffSpec runs the --include-off-spec recovery selection: it ranks
// eligible todo tasks across all specs with the original ranking and flags an
// off-spec pick, giving agents a deterministic path to older unfinished work. It
// writes the same next_action/updated_at probe as Next and never bypasses
// start's transition rules.
func (s *Service) NextIncludingOffSpec() (NextResult, error) {
	return s.next(true)
}

func (s *Service) next(includeOffSpec bool) (result NextResult, err error) {
	own, release, err := s.beginWriterWrite(lifecycleNext, "", []string{s.reportedStatePath()})
	if err != nil {
		return NextResult{}, err
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil && err == nil {
			err = releaseErr
		}
	}()

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return NextResult{}, err
	}
	corpus, err := snapshotTaskCorpus(tasks)
	if err != nil {
		return NextResult{}, err
	}
	baseline := s.validateInMemory(state, tasks)

	result = computeNext(state, tasks, includeOffSpec)
	state.Frontmatter.UpdatedAt = timestamp(s.now())
	state.Frontmatter.NextAction = nextAction(result)
	state.Body = renderStateBody(state.Frontmatter, tasks)
	if _, err := s.commitLifecycle(own, lifecycleNext, lifecycleLedger{state: state, preview: tasks, corpus: corpus, baseline: baseline}); err != nil {
		return NextResult{}, err
	}
	return result, nil
}

// computeNext resolves the next-task selection without persisting anything.
// Next() wraps it to also record next_action/updated_at; status reuses it to
// report the selection read-only, so the selection logic lives in one place.
func computeNext(state *State, tasks []*Task, includeOffSpec bool) NextResult {
	if state.Frontmatter.CurrentTask != "" {
		if task, ok := taskByID(tasks, state.Frontmatter.CurrentTask); ok && task.Frontmatter.Status == "in_progress" {
			return NextResult{
				TaskID:     task.Frontmatter.ID,
				Title:      task.Frontmatter.Title,
				Priority:   task.Frontmatter.Priority,
				Reason:     "active task already in progress",
				Candidates: []string{task.Frontmatter.ID},
				Warnings:   nextSelectionWarnings(state, task),
			}
		}
	}

	activeSpecPath := strings.TrimSpace(state.Frontmatter.ActiveSpecPath)

	// --include-off-spec is a one-shot opt-out of the active-spec filter: rank
	// every eligible todo across specs and flag a pick that points away from the
	// active spec, so agents can recover older unfinished work (T-110).
	if includeOffSpec {
		return computeNextAcrossSpecs(eligibleTasks(tasks), activeSpecPath)
	}

	// Idle selection is anchored to the active spec: filter eligible candidates to
	// the active spec before ranking, so higher-priority older-spec work is skipped
	// rather than selected. Skipped runnable work still surfaces as a structured
	// warning so agents can distinguish it from an empty backlog (T-108).
	inScope, skipped := partitionByActiveSpec(eligibleTasks(tasks), activeSpecPath)
	ids := taskIDs(inScope)
	if len(inScope) == 0 {
		if len(skipped) > 0 {
			return NextResult{
				Reason:     "no active-spec eligible task",
				Candidates: ids,
				Warnings:   skippedNonActiveSpecWarnings(skipped, activeSpecPath),
			}
		}
		return NextResult{Reason: "no eligible task", Candidates: ids}
	}

	return selectedTodoResult(inScope[0], ids)
}

// selectedTodoResult builds the NextResult for the top-ranked eligible todo,
// shared by the active-spec and across-specs selection paths so their pick shape
// and reason stay identical.
func selectedTodoResult(selected *Task, candidates []string) NextResult {
	return NextResult{
		TaskID:     selected.Frontmatter.ID,
		Title:      selected.Frontmatter.Title,
		Priority:   selected.Frontmatter.Priority,
		Reason:     "next eligible todo by priority and stable task id",
		Candidates: candidates,
	}
}

// computeNextAcrossSpecs ranks eligible todo tasks over all specs (the
// --include-off-spec recovery path) and marks a pick whose spec_ref points away
// from the active spec, so the relaxed selection still reports when it steps
// outside the active spec. An empty activeSpecPath means nothing to anchor
// against, so no pick is treated as off-spec.
func computeNextAcrossSpecs(eligible []*Task, activeSpecPath string) NextResult {
	ids := taskIDs(eligible)
	if len(eligible) == 0 {
		return NextResult{Reason: "no eligible task", Candidates: ids}
	}
	selected := eligible[0]
	result := selectedTodoResult(selected, ids)
	if activeSpecPath != "" && !taskMatchesActiveSpec(selected, activeSpecPath) {
		result.OffSpec = true
		result.Warnings = []Warning{{
			Code:           "selected_off_spec",
			Message:        fmt.Sprintf("off-spec: selected task %s points at %s while active spec is %s", selected.Frontmatter.ID, selected.Frontmatter.SpecRef, activeSpecPath),
			TaskID:         selected.Frontmatter.ID,
			SpecRef:        selected.Frontmatter.SpecRef,
			ActiveSpecPath: activeSpecPath,
		}}
	}
	return result
}

// partitionByActiveSpec splits eligible tasks into those linked to the active spec
// and those pointing elsewhere. An empty activeSpecPath disables filtering, so
// every eligible task stays in scope (nothing to anchor against).
func partitionByActiveSpec(eligible []*Task, activeSpecPath string) (inScope, skipped []*Task) {
	if activeSpecPath == "" {
		return eligible, nil
	}
	for _, task := range eligible {
		if taskMatchesActiveSpec(task, activeSpecPath) {
			inScope = append(inScope, task)
		} else {
			skipped = append(skipped, task)
		}
	}
	return inScope, skipped
}

// taskMatchesActiveSpec reports whether task's spec_ref path resolves to the
// active spec. An unparseable spec_ref is treated as matching so filtering never
// silently hides a task that validation would otherwise surface.
func taskMatchesActiveSpec(task *Task, activeSpecPath string) bool {
	specPath, _, err := parseSpecRef(task.Frontmatter.SpecRef)
	if err != nil {
		return true
	}
	return normalizeSpecPath(specPath) == normalizeSpecPath(activeSpecPath)
}

func taskIDs(tasks []*Task) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.Frontmatter.ID)
	}
	return ids
}

// nextSelectionWarnings surfaces the drift warning for an already-active task
// whose spec_ref points outside the active spec. Idle selection no longer warns —
// it filters instead (see computeNext) — so this only fires for the continuation
// case, where the active task owns the workflow slot and must be returned as-is.
func nextSelectionWarnings(state *State, task *Task) []Warning {
	activeSpecPath := strings.TrimSpace(state.Frontmatter.ActiveSpecPath)
	if activeSpecPath == "" || task == nil || taskMatchesActiveSpec(task, activeSpecPath) {
		return nil
	}
	return []Warning{{
		Code:           "selected_non_active_spec",
		Message:        fmt.Sprintf("warning: selected task %s points at %s while active spec is %s", task.Frontmatter.ID, task.Frontmatter.SpecRef, activeSpecPath),
		TaskID:         task.Frontmatter.ID,
		SpecRef:        task.Frontmatter.SpecRef,
		ActiveSpecPath: activeSpecPath,
	}}
}

// skippedNonActiveSpecWarnings reports each eligible task that idle selection
// skipped because its spec_ref points outside the active spec, giving agents a
// structured signal that runnable older-spec work exists.
func skippedNonActiveSpecWarnings(skipped []*Task, activeSpecPath string) []Warning {
	warnings := make([]Warning, 0, len(skipped))
	for _, task := range skipped {
		warnings = append(warnings, Warning{
			Code:           "skipped_non_active_spec",
			Message:        fmt.Sprintf("warning: skipped eligible task %s at %s; active spec is %s", task.Frontmatter.ID, task.Frontmatter.SpecRef, activeSpecPath),
			TaskID:         task.Frontmatter.ID,
			SpecRef:        task.Frontmatter.SpecRef,
			ActiveSpecPath: activeSpecPath,
		})
	}
	return warnings
}

func normalizeSpecPath(path string) string {
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
}

// nextAction renders the STATE.md next_action string that `next` persists for a
// given selection. It is the write-side counterpart to computeNext.
func nextAction(result NextResult) string {
	switch result.Reason {
	case "active task already in progress":
		return fmt.Sprintf("Continue task %s", result.TaskID)
	case "no eligible task":
		return "No eligible task is ready"
	case "no active-spec eligible task":
		return "No active-spec task is ready"
	default:
		return fmt.Sprintf("Start task %s: %s", result.TaskID, result.Title)
	}
}

// recordVerification stamps the selected task and the re-projected state with
// one verification outcome. Task files are committed, so the note and the state
// summary stay portable: they record result and timestamp without a path into
// gitignored artifacts; local evidence still lives under
// planning/artifacts/verify/ for the producer (see VerifyResult).
func recordVerification(state *State, task *Task, input VerifyInput, followupTaskID, nowText string, preview []*Task) {
	appendTaskNote(task, verificationNoteLine(nowText, input.Result))
	task.Frontmatter.UpdatedAt = nowText

	state.Frontmatter.UpdatedAt = nowText
	state.Frontmatter.LastVerificationResult = fmt.Sprintf("%s for %s at %s", input.Result, task.Frontmatter.ID, nowText)
	state.Frontmatter.RelevantArtifacts = nil
	if input.Result == "fail" && followupTaskID != "" {
		state.Frontmatter.NextAction = fmt.Sprintf("Review follow-up task %s", followupTaskID)
	} else if input.Result == "fail" {
		state.Frontmatter.NextAction = fmt.Sprintf("Resolve verification findings for %s", task.Frontmatter.ID)
	} else {
		state.Frontmatter.NextAction = nextActionSelectEligible
	}
	state.Body = renderStateBody(state.Frontmatter, preview)
}

func (s *Service) Verify(input VerifyInput) (result VerifyResult, err error) {
	if input.Result != "pass" && input.Result != "fail" {
		return VerifyResult{}, invalidArgumentsf("invalid verify result %q", input.Result)
	}
	if strings.TrimSpace(input.Summary) == "" {
		return VerifyResult{}, WithMachineErrorCode(MachineCodeInvalidArguments,
			errors.New("verify summary must not be empty"))
	}

	// The timestamp fixes the artifact destination before the lock is taken, so
	// the write set a delegated child claims is the one it publishes.
	now := s.now().UTC()
	ts := now.Format("20060102T150405Z")
	writes, err := s.verifyWriteClaim(input, ts)
	if err != nil {
		return VerifyResult{}, err
	}
	own, release, err := s.beginWriterWrite(lifecycleVerify, input.TaskID, writes)
	if err != nil {
		return VerifyResult{}, err
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil && err == nil {
			err = releaseErr
		}
	}()

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return VerifyResult{}, err
	}
	corpus, err := snapshotTaskCorpus(tasks)
	if err != nil {
		return VerifyResult{}, err
	}
	baseline := s.validateInMemory(state, tasks)
	task, err := exactTaskByID(tasks, input.TaskID)
	if err != nil {
		return VerifyResult{}, err
	}

	artifactDir := filepath.Join(s.paths.VerifyDir, task.Frontmatter.ID, ts)
	planPath := filepath.Join(artifactDir, "plan.md")
	reportPath := filepath.Join(artifactDir, "report.json")
	reportMarkdownPath := filepath.Join(artifactDir, "report.md")
	relPlan := relPath(s.paths.RepoRoot, planPath)
	relReport := relPath(s.paths.RepoRoot, reportPath)
	relReportMarkdown := relPath(s.paths.RepoRoot, reportMarkdownPath)

	var followups []*Task
	followupTaskID := ""
	var warnings []Warning
	if input.CreateFollowup {
		newTask, taskWarnings, err := s.createFollowupTask(tasks, task, input)
		if err != nil {
			return VerifyResult{}, err
		}
		followups = append(followups, newTask)
		followupTaskID = newTask.Frontmatter.ID
		warnings = taskWarnings
	}
	preview := append(slices.Clone(tasks), followups...)

	plan := renderVerificationPlan(task, input, followupTaskID)
	report := VerificationArtifact{
		SchemaVersion:  stateSchemaVersion,
		TaskID:         task.Frontmatter.ID,
		TaskTitle:      task.Frontmatter.Title,
		Result:         input.Result,
		Summary:        strings.TrimSpace(input.Summary),
		Details:        strings.TrimSpace(input.Details),
		GeneratedAt:    timestamp(now),
		SpecRef:        task.Frontmatter.SpecRef,
		Artifacts:      []string{relPlan, relReportMarkdown},
		FollowupTaskID: followupTaskID,
	}
	reportBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return VerifyResult{}, fmt.Errorf("marshal verification report: %w", err)
	}
	reportMarkdown := renderVerificationReportMarkdown(report)

	nowText := timestamp(now)
	recordVerification(state, task, input, followupTaskID, nowText, preview)

	ledger := verifyLedger{
		state:     state,
		task:      task,
		followups: followups,
		artifacts: []repotx.Candidate{
			worktreeCandidate(relPlan, planPath, []byte(plan)),
			worktreeCandidate(relReport, reportPath, reportBytes),
			worktreeCandidate(relReportMarkdown, reportMarkdownPath, []byte(reportMarkdown)),
		},
		original: tasks,
		preview:  preview,
		corpus:   corpus,
		baseline: baseline,
	}
	if err := s.commitVerify(own, ledger); err != nil {
		return VerifyResult{}, err
	}

	return VerifyResult{
		TaskID:         task.Frontmatter.ID,
		Result:         input.Result,
		ArtifactDir:    relPath(s.paths.RepoRoot, artifactDir),
		PlanPath:       relPlan,
		ReportPath:     relReport,
		ReportMarkdown: relReportMarkdown,
		FollowupTaskID: followupTaskID,
		Warnings:       warnings,
	}, nil
}

// taskValidationOpts carries the import-specific relaxations validateTaskCreatable
// honors: resolving a spec_ref against a not-yet-written imported spec, and
// accepting a dependency that names an in-draft key a sibling task will create.
// The zero value is the strict, on-disk mode CreateTask uses.
type taskValidationOpts struct {
	pending   *pendingSpec
	draftKeys map[string]struct{}
}

// validateTaskCreatable runs the spec-and-dependency live-repo checks CreateTask
// enforces before it writes — non-empty spec_ref with a resolvable heading, a
// valid priority, and existing dependencies — and returns the normalized spec_ref
// and priority. Writing nothing, it is the shared pre-write validator import
// pre-flight (T-041) reuses to reject a whole draft before any file lands, so
// any check added *here* is enforced on both paths. Title emptiness is
// deliberately not checked on either path: CreateTask allows a bare, title-less
// scaffold (T-095), while the import path independently requires a non-empty
// title via ValidateImportDraft.
func (s *Service) validateTaskCreatable(tasks []*Task, specRef, priority string, deps []string, opts taskValidationOpts) (normalizedSpecRef, normalizedPriority string, err error) {
	specRef, err = s.validateSpecRefWithPending(specRef, opts.pending)
	if err != nil {
		return "", "", invalidArgumentsf("invalid spec_ref: %w", err)
	}
	priority = strings.TrimSpace(priority)
	if priority == "" {
		priority = "medium"
	}
	if _, ok := validPriorites[priority]; !ok {
		return "", "", invalidArgumentsf("invalid priority %q", priority)
	}
	for _, dep := range deps {
		if _, ok := opts.draftKeys[dep]; ok {
			continue // an in-draft key: a sibling draft task will create it
		}
		if _, ok := taskByID(tasks, dep); !ok {
			return "", "", invalidArgumentsf("dependency %s does not exist", dep)
		}
	}
	return specRef, priority, nil
}

// resolveAreaSpecRef turns a `--area <anchor>` shorthand into the full, canonical
// `<active_spec_path>#<anchor>` spec_ref, validating the anchor through the same
// path as an explicit `--spec-ref` so the set of accepted anchors never diverges.
// On an unknown anchor it points the operator at the active spec's real anchors.
func (s *Service) resolveAreaSpecRef(state *State, area string) (string, error) {
	activePath := strings.TrimSpace(state.Frontmatter.ActiveSpecPath)
	if activePath == "" {
		return "", WithMachineErrorCode(MachineCodeInvalidArguments,
			errors.New("--area requires an active spec, but planning/STATE.md has none set"))
	}
	specRef, err := s.validateSpecRef(activePath + "#" + area)
	if err != nil {
		return "", invalidArgumentsf("unknown active-spec area %q: %w; run `taskrail spec show %s --anchors` to list valid anchors", area, err, state.Frontmatter.ActiveSpecVersion)
	}
	return specRef, nil
}

// CreateTask scaffolds a well-formed task file with the next free id. It mirrors
// the validation `validate` would apply (spec anchor, dependency existence,
// priority) at creation time so an invalid task never lands on disk.
func (s *Service) CreateTask(input CreateTaskInput) (CreateTaskResult, error) {
	// Title is optional: a scaffold with neither a title nor a slug is a legitimate
	// bare `T-<n>` task, matching the id form validate already accepts.
	title := strings.TrimSpace(input.Title)

	// The title lands verbatim in the committed frontmatter and scaffolded heading,
	// both of which validate scans (taskArtifactRefs), so a gitignored path here
	// would make the very next validate fail. It is also the only free-text operator
	// input that reaches the file: the slug is normalized to [a-z0-9-] and the
	// description/provenance lines are generated.
	if err := ensurePortableNote("title", title); err != nil {
		return CreateTaskResult{}, WithMachineErrorCode(MachineCodeInvalidArguments, err)
	}

	// A task has exactly one resolved spec reference: --area is the active-spec
	// shorthand for --spec-ref, so the two cannot both be given. Reject before any
	// load or write so a conflicting request lands nothing on disk.
	area := strings.TrimSpace(input.Area)
	specRef := strings.TrimSpace(input.SpecRef)
	if area != "" && specRef != "" {
		return CreateTaskResult{}, WithMachineErrorCode(MachineCodeInvalidArguments,
			errors.New("--area and --spec-ref are mutually exclusive"))
	}

	// Load first: a follow-up needs the parent task to inherit spec_ref and wire
	// the dependency before the shared validation below runs.
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return CreateTaskResult{}, err
	}

	// Resolve --area against STATE.md's active spec before follow-up inheritance so
	// an explicit area overrides a parent's inherited ref.
	if area != "" {
		specRef, err = s.resolveAreaSpecRef(state, area)
		if err != nil {
			return CreateTaskResult{}, err
		}
	}

	deps := append([]string(nil), input.Dependencies...)
	followUpOf := input.FollowUpOf
	if followUpOf != "" {
		parent, err := exactTaskByID(tasks, followUpOf)
		if err != nil {
			// `task new` creates a task rather than acting on one, so a --follow-up
			// that names nothing is an invalid argument; its contract does not admit
			// task_not_found.
			return CreateTaskResult{}, WithMachineErrorCode(MachineCodeInvalidArguments, err)
		}
		if specRef == "" {
			specRef = parent.Frontmatter.SpecRef
		}
		if !slices.Contains(deps, followUpOf) {
			deps = append(deps, followUpOf)
		}
	}

	specRef, priority, err := s.validateTaskCreatable(tasks, specRef, input.Priority, deps, taskValidationOpts{})
	if err != nil {
		return CreateTaskResult{}, err
	}

	// The id and filename are two encodings of one identifier: bake the slug (if
	// any) into the id so `filename == "<id>.md"` holds. nextTaskID keys on the
	// numeric prefix, so a slug suffix never affects id allocation or collision.
	nextID, warnings := nextTaskIDWithSlug(tasks, input.Slug, input.SlugExplicit, input.SlugSourceSupplied)
	now := timestamp(s.now())
	var provenance string
	if followUpOf != "" {
		provenance = fmt.Sprintf("Follow-up derived from %s's verification or discovery.", followUpOf)
	}
	body := renderNewTaskBody(nextID, title, provenance)
	newTask := &Task{
		Frontmatter: TaskFrontmatter{
			ID:           nextID,
			Title:        title,
			Status:       "todo",
			Priority:     priority,
			SpecRef:      specRef,
			Dependencies: deps,
			UpdatedAt:    now,
		},
		Body:     body,
		Filename: filepath.Join(s.paths.TasksDir, nextID+".md"),
	}

	// Write the durable task file first, then re-render STATE.md counts from the
	// full set (existing task files are left untouched). Ordering the task write
	// first means a failed state write leaves a real task with a stale count that
	// the next state-writing command heals, never a counted-but-absent task.
	if err := s.saveTask(newTask); err != nil {
		return CreateTaskResult{}, err
	}
	state.Frontmatter.UpdatedAt = now
	state.Body = renderStateBody(state.Frontmatter, append(tasks, newTask))
	if err := s.saveState(state); err != nil {
		return CreateTaskResult{}, s.withWrittenPaths(err, newTask.Filename)
	}

	return CreateTaskResult{
		TaskID:   nextID,
		Title:    title,
		Priority: priority,
		SpecRef:  specRef,
		Path:     relPath(s.paths.RepoRoot, newTask.Filename),
		Warnings: warnings,
	}, nil
}

// reconcileIdlePointers sets status_summary/next_action for the no-active-task
// state from the blockers ledger, the single reconciliation Unblock and both
// finishTask branches share. Callers must upsert/drop the ledger for the current
// transition first, then invoke this only when current_task is empty (an active
// task owns those pointers). While any blocker remains, stay "blocked" pointing at
// the most-recently recorded one — for the block branch that is the just-blocked
// task (upsertBlocker appends it last); for complete/unblock it is a still-blocked
// sibling. Only once the ledger is empty do the neutral idle pointers apply.
func reconcileIdlePointers(fm *StateFrontmatter) {
	if remaining := fm.Blockers; len(remaining) > 0 {
		fm.StatusSummary = statusSummaryBlocked
		fm.NextAction = fmt.Sprintf("Resolve blocker on %s", blockerID(remaining[len(remaining)-1]))
		return
	}
	fm.StatusSummary = statusSummaryIdle
	fm.NextAction = nextActionSelectEligible
}

// ensurePortableNote rejects a transition note that embeds a concrete gitignored
// artifact path. Complete/block/unblock notes land in the committed task body (and
// block reasons in the validated blockers ledger), so such a path would make
// validate fail right after the transition wrote it. Failing at the boundary keeps
// a transition from producing state validate would reject
// (specs/v0.2.0.md#no-local-paths-in-task-notes). It reuses the same
// danglingArtifactPaths detector validate uses, so it accepts exactly what
// validate accepts — no second path rule.
func ensurePortableNote(field, note string) error {
	if paths := danglingArtifactPaths(note); len(paths) > 0 {
		return fmt.Errorf("%s references gitignored artifact path %s: record a path-free summary instead (paths under planning/artifacts/ are gitignored and never resolve for anyone who clones the repo)", field, paths[0])
	}
	return nil
}

func (s *Service) createFollowupTask(tasks []*Task, source *Task, input VerifyInput) (*Task, []Warning, error) {
	priority := strings.TrimSpace(input.FollowupPriority)
	if priority == "" {
		priority = "medium"
	}
	if _, ok := validPriorites[priority]; !ok {
		return nil, nil, invalidArgumentsf("invalid follow-up priority %q", priority)
	}

	title := strings.TrimSpace(input.FollowupTitle)
	if title == "" {
		title = fmt.Sprintf("Follow-up for %s: %s", source.Frontmatter.ID, input.Summary)
	}
	description := strings.TrimSpace(input.FollowupDescription)
	if description == "" {
		description = strings.TrimSpace(input.Details)
	}
	if description == "" {
		description = "Investigate and resolve the verification finding recorded for this task."
	}
	// The follow-up task is committed, so its title and body must stay portable —
	// the same contract finishTask/Unblock enforce for notes. The verified task's
	// own note is fixed-format, but a follow-up's title falls back to --summary and
	// its body to --details, either of which an operator may paste a gitignored
	// evidence path into. Guard before Verify writes any artifact or task file.
	if err := ensurePortableNote("follow-up title", title); err != nil {
		return nil, nil, WithMachineErrorCode(MachineCodeInvalidArguments, err)
	}
	if err := ensurePortableNote("follow-up description", description); err != nil {
		return nil, nil, WithMachineErrorCode(MachineCodeInvalidArguments, err)
	}
	nextID, warnings := nextTaskIDWithSlug(tasks, title, false, true)

	body := renderFollowupTaskBody(nextID, title, description)
	filename := filepath.Join(s.paths.TasksDir, nextID+".md")
	task := &Task{
		Frontmatter: TaskFrontmatter{
			ID:           nextID,
			Title:        title,
			Status:       "todo",
			Priority:     priority,
			SpecRef:      source.Frontmatter.SpecRef,
			Dependencies: []string{source.Frontmatter.ID},
			UpdatedAt:    timestamp(s.now()),
		},
		Body:     body,
		Path:     s.paths.logicalManagedPath(filename),
		Filename: filename,
	}
	return task, warnings, nil
}

func nextTaskIDWithSlug(tasks []*Task, source string, explicit, supplied bool) (string, []Warning) {
	nextID := nextTaskID(tasks)
	slug := slugify(source)
	if slug == "" {
		if !supplied && !explicit && strings.TrimSpace(source) == "" {
			return nextID, nil
		}
		return nextID, emptySlugWarnings(source, nextID)
	}
	if !explicit {
		slug = capSlug(slug)
		if slug == "" {
			return nextID, emptySlugWarnings(source, nextID)
		}
	}
	return nextID + "-" + slug, nil
}
