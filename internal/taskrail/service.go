package taskrail

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	paths Paths
	now   func() time.Time
}

func NewService(start string) (*Service, error) {
	paths, err := DiscoverPaths(start)
	if err != nil {
		return nil, err
	}
	return &Service{paths: paths, now: time.Now}, nil
}

// Init makes a repository Taskrail-managed in a version-aware, non-destructive
// way: it writes the `.taskrail/config.yml` marker and, when the marker records
// an older layout_version, migrates to the current layout. Migration defaults to
// a dry run reporting the diff; callers must pass apply=true to write it. Content
// created for the layout uses writeFileIfMissing/saveState-if-missing semantics,
// so human-authored content under specs/ and planning/ is never rewritten.
func (s *Service) Init(apply bool) (InitResult, error) {
	cfg, hasMarker, err := readMarker(s.paths.RepoRoot)
	if err != nil {
		return InitResult{}, err
	}
	if hasMarker {
		return s.initWithMarker(cfg, apply)
	}
	return s.initWithoutMarker(apply)
}

// initWithoutMarker handles the unmarked cases: a pre-existing v0.1.0 layout is
// adopted (marker written, nothing else touched); a non-standard layout with
// candidate directories triggers a guided retrofit that proposes a mapping and
// defaults to a dry run; and an empty repository gets a fresh layout plus marker.
func (s *Service) initWithoutMarker(apply bool) (InitResult, error) {
	if s.layoutExists() {
		if err := writeMarker(s.paths.RepoRoot, defaultLayoutConfig()); err != nil {
			return InitResult{}, err
		}
		return InitResult{
			Outcome:     InitAdopted,
			FromVersion: currentLayoutVersion,
			ToVersion:   currentLayoutVersion,
			Applied:     true,
			Changes:     []string{markerWriteChange()},
		}, nil
	}

	if mapping := s.detectRetrofit(); len(mapping) > 0 {
		return s.retrofit(mapping, apply)
	}

	if err := s.ensureLayout(); err != nil {
		return InitResult{}, err
	}
	if err := writeMarker(s.paths.RepoRoot, defaultLayoutConfig()); err != nil {
		return InitResult{}, err
	}
	return InitResult{
		Outcome:     InitCreated,
		FromVersion: currentLayoutVersion,
		ToVersion:   currentLayoutVersion,
		Applied:     true,
	}, nil
}

// initWithMarker dispatches on the recorded layout version: current is an
// idempotent no-op, older triggers migration, and newer is refused so an older
// CLI never mangles a layout it does not understand.
func (s *Service) initWithMarker(cfg LayoutConfig, apply bool) (InitResult, error) {
	switch {
	case cfg.LayoutVersion == currentLayoutVersion:
		if err := s.ensureLayout(); err != nil {
			return InitResult{}, err
		}
		return InitResult{
			Outcome:     InitCurrent,
			FromVersion: cfg.LayoutVersion,
			ToVersion:   currentLayoutVersion,
			Applied:     true,
		}, nil
	case cfg.LayoutVersion > currentLayoutVersion:
		return InitResult{}, fmt.Errorf(
			"repository layout_version %d is newer than supported %d; upgrade taskrail",
			cfg.LayoutVersion, currentLayoutVersion)
	default:
		return s.migrate(cfg, apply)
	}
}

// migrate upgrades an older-version repository to the current layout. It defaults
// to a dry run that only reports the diff; apply writes the missing layout,
// bumps the marker, and re-runs validation. It only ever creates missing content
// and rewrites the machine marker, so human-authored files are left intact.
func (s *Service) migrate(cfg LayoutConfig, apply bool) (InitResult, error) {
	changes := append(s.pendingLayoutChanges(),
		fmt.Sprintf("update %s layout_version %d -> %d", markerRelPath(), cfg.LayoutVersion, currentLayoutVersion))

	if !apply {
		return InitResult{
			Outcome:     InitMigrationPreview,
			FromVersion: cfg.LayoutVersion,
			ToVersion:   currentLayoutVersion,
			Changes:     changes,
		}, nil
	}

	migrated := cfg
	migrated.LayoutVersion = currentLayoutVersion
	if migrated.SpecsDir == "" {
		migrated.SpecsDir = defaultSpecsDir
	}
	if migrated.PlanningDir == "" {
		migrated.PlanningDir = defaultPlanningDir
	}
	validation, err := s.applyLayout(migrated)
	if err != nil {
		return InitResult{}, err
	}
	return InitResult{
		Outcome:     InitMigrated,
		FromVersion: cfg.LayoutVersion,
		ToVersion:   currentLayoutVersion,
		Applied:     true,
		Changes:     changes,
		Validation:  &validation,
	}, nil
}

// retrofitCandidates lists the source directory names a non-standard repository
// might already use, in priority order, and the Taskrail directory (target) and
// role each would fill. Detection is deliberately conservative: it only
// recognizes this small, well-known set rather than guessing from arbitrary
// directory names.
var retrofitCandidates = []struct {
	dir    string
	role   string
	target string
}{
	{defaultSpecsDir, "specs", defaultSpecsDir},
	{defaultPlanningDir, "planning", defaultPlanningDir},
	{"notes", "planning", defaultPlanningDir},
}

// detectRetrofit scans an unmarked, non-standard repository for candidate
// directories that suggest an existing layout to adopt, returning the proposed
// mapping onto the Taskrail layout. It returns nil for an empty repository (which
// should be fresh-initialized) and never proposes the same role twice, so a
// human confirms one clear mapping rather than a redundant one.
func (s *Service) detectRetrofit() []RetrofitMapping {
	var mapping []RetrofitMapping
	claimed := map[string]bool{}
	for _, c := range retrofitCandidates {
		if claimed[c.role] {
			continue
		}
		if !dirExists(filepath.Join(s.paths.RepoRoot, c.dir)) {
			continue
		}
		mapping = append(mapping, RetrofitMapping{Source: c.dir, Target: c.target, Role: c.role})
		claimed[c.role] = true
	}
	return mapping
}

// retrofit adopts a detected non-standard layout into the current Taskrail
// layout. It defaults to a dry run that only reports the proposed mapping and the
// changes applying it would make; apply creates the missing layout with
// writeFileIfMissing semantics (never clobbering existing content), writes the
// marker, and re-runs validation.
func (s *Service) retrofit(mapping []RetrofitMapping, apply bool) (InitResult, error) {
	changes := append(s.pendingLayoutChanges(), markerWriteChange())

	if !apply {
		return InitResult{
			Outcome:     InitRetrofitPreview,
			FromVersion: currentLayoutVersion,
			ToVersion:   currentLayoutVersion,
			Changes:     changes,
			Mapping:     mapping,
		}, nil
	}

	validation, err := s.applyLayout(defaultLayoutConfig())
	if err != nil {
		return InitResult{}, err
	}
	return InitResult{
		Outcome:     InitRetrofitApplied,
		FromVersion: currentLayoutVersion,
		ToVersion:   currentLayoutVersion,
		Applied:     true,
		Changes:     changes,
		Mapping:     mapping,
		Validation:  &validation,
	}, nil
}

// applyLayout is the shared apply tail for migrate and retrofit: create the
// current layout idempotently, persist the given marker, and re-run validation.
// Both callers only ever add missing content, so human-authored files survive.
func (s *Service) applyLayout(marker LayoutConfig) (ValidationResult, error) {
	if err := s.ensureLayout(); err != nil {
		return ValidationResult{}, err
	}
	if err := writeMarker(s.paths.RepoRoot, marker); err != nil {
		return ValidationResult{}, err
	}
	return s.Validate()
}

// markerWriteChange describes writing the current marker, used in dry-run diffs.
func markerWriteChange() string {
	return fmt.Sprintf("write %s (layout_version %d)", markerRelPath(), currentLayoutVersion)
}

// layoutExists reports whether the repository already carries a v0.1.0 layout,
// used to tell legacy adoption apart from a fresh empty-repo init.
func (s *Service) layoutExists() bool {
	return fileExists(s.paths.StateFile) || dirExists(s.paths.TasksDir)
}

// ensureLayout creates the current layout idempotently: directories via ensureDir
// and content via writeFileIfMissing, with the state file written only when
// absent. Re-running it never overwrites existing files.
func (s *Service) ensureLayout() error {
	// Only tracked, committed directories are provisioned. Gitignored artifact
	// output (verify/, runs/, manual-test/) is created on demand by verify and
	// manual testing; a clean checkout drops it, so pre-creating it here would
	// leave init and validate inconsistent (T-024/T-025).
	for _, dir := range []string{s.paths.SpecsDir, s.paths.TasksDir} {
		if err := ensureDir(dir); err != nil {
			return err
		}
	}
	if err := writeFileIfMissing(filepath.Join(s.paths.SpecsDir, "README.md"), []byte(starterSpecsReadme())); err != nil {
		return err
	}
	if err := writeFileIfMissing(filepath.Join(s.paths.SpecsDir, "v0.1.0.md"), []byte(starterSpecV010())); err != nil {
		return err
	}
	if _, err := os.Stat(s.paths.StateFile); errors.Is(err, os.ErrNotExist) {
		if err := s.saveState(starterState(s.now())); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("stat state file: %w", err)
	}
	return nil
}

// pendingLayoutChanges lists the layout directories and files ensureLayout would
// create, so a migration dry run can report exactly what applying it would add.
func (s *Service) pendingLayoutChanges() []string {
	var changes []string
	for _, dir := range []string{s.paths.SpecsDir, s.paths.TasksDir} {
		if !dirExists(dir) {
			changes = append(changes, "create dir "+relPath(s.paths.RepoRoot, dir))
		}
	}
	for _, file := range []string{
		filepath.Join(s.paths.SpecsDir, "README.md"),
		filepath.Join(s.paths.SpecsDir, "v0.1.0.md"),
		s.paths.StateFile,
	} {
		if !fileExists(file) {
			changes = append(changes, "create "+relPath(s.paths.RepoRoot, file))
		}
	}
	return changes
}

func markerRelPath() string {
	return filepath.Join(taskrailConfigDir, taskrailConfigFile)
}

func (s *Service) Validate() (ValidationResult, error) {
	violations := make([]string, 0)
	// Artifacts (ArtifactsDir/VerifyDir) are gitignored output and empty on a
	// clean checkout, so they are not required; verify creates them on demand.
	for _, requiredDir := range []string{s.paths.SpecsDir, s.paths.PlanningDir, s.paths.TasksDir} {
		if !dirExists(requiredDir) {
			violations = append(violations, fmt.Sprintf("missing required directory %s", relPath(s.paths.RepoRoot, requiredDir)))
		}
	}
	for _, requiredFile := range []string{filepath.Join(s.paths.SpecsDir, "README.md"), s.paths.StateFile} {
		if !fileExists(requiredFile) {
			violations = append(violations, fmt.Sprintf("missing required file %s", relPath(s.paths.RepoRoot, requiredFile)))
		}
	}

	state, stateErr := s.loadState()
	if stateErr != nil {
		violations = append(violations, stateErr.Error())
	}
	tasks, taskErr := s.loadTasks()
	if taskErr != nil {
		violations = append(violations, taskErr.Error())
	}
	if stateErr != nil || taskErr != nil {
		return ValidationResult{Valid: len(violations) == 0, Violations: violations}, nil
	}

	violations = append(violations, s.validateState(state)...)
	violations = append(violations, s.validateTasks(state, tasks)...)

	return ValidationResult{Valid: len(violations) == 0, Violations: violations}, nil
}

func (s *Service) Next() (NextResult, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return NextResult{}, err
	}

	if state.Frontmatter.CurrentTask != "" {
		if task, ok := taskByID(tasks, state.Frontmatter.CurrentTask); ok && task.Frontmatter.Status == "in_progress" {
			state.Frontmatter.UpdatedAt = timestamp(s.now())
			state.Frontmatter.NextAction = fmt.Sprintf("Continue task %s", task.Frontmatter.ID)
			state.Body = renderStateBody(state.Frontmatter, tasks)
			if err := s.saveState(state); err != nil {
				return NextResult{}, err
			}
			return NextResult{
				TaskID:     task.Frontmatter.ID,
				Title:      task.Frontmatter.Title,
				Priority:   task.Frontmatter.Priority,
				Reason:     "active task already in progress",
				Candidates: []string{task.Frontmatter.ID},
			}, nil
		}
	}

	candidates := eligibleTasks(tasks)
	ids := make([]string, 0, len(candidates))
	for _, task := range candidates {
		ids = append(ids, task.Frontmatter.ID)
	}

	state.Frontmatter.UpdatedAt = timestamp(s.now())
	if len(candidates) == 0 {
		state.Frontmatter.NextAction = "No eligible task is ready"
		state.Body = renderStateBody(state.Frontmatter, tasks)
		if err := s.saveState(state); err != nil {
			return NextResult{}, err
		}
		return NextResult{Reason: "no eligible task", Candidates: ids}, nil
	}

	selected := candidates[0]
	state.Frontmatter.NextAction = fmt.Sprintf("Start task %s: %s", selected.Frontmatter.ID, selected.Frontmatter.Title)
	state.Body = renderStateBody(state.Frontmatter, tasks)
	if err := s.saveState(state); err != nil {
		return NextResult{}, err
	}

	return NextResult{
		TaskID:     selected.Frontmatter.ID,
		Title:      selected.Frontmatter.Title,
		Priority:   selected.Frontmatter.Priority,
		Reason:     "next eligible todo by priority and stable task id",
		Candidates: ids,
	}, nil
}

func (s *Service) Start(taskID string) (TransitionResult, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return TransitionResult{}, err
	}
	if state.Frontmatter.CurrentTask != "" {
		return TransitionResult{}, fmt.Errorf("task %s is already active", state.Frontmatter.CurrentTask)
	}

	task, ok := taskByID(tasks, taskID)
	if !ok {
		return TransitionResult{}, fmt.Errorf("task %s not found", taskID)
	}
	if task.Frontmatter.Status != "todo" {
		return TransitionResult{}, fmt.Errorf("task %s is not todo", taskID)
	}
	if !dependenciesResolved(task, tasks) {
		return TransitionResult{}, fmt.Errorf("task %s has unresolved dependencies", taskID)
	}

	now := timestamp(s.now())
	task.Frontmatter.Status = "in_progress"
	task.Frontmatter.UpdatedAt = now

	state.Frontmatter.UpdatedAt = now
	state.Frontmatter.CurrentTask = task.Frontmatter.ID
	state.Frontmatter.CurrentTaskTitle = task.Frontmatter.Title
	state.Frontmatter.StatusSummary = "in_progress"
	state.Frontmatter.Blockers = []string{}
	state.Frontmatter.NextAction = fmt.Sprintf("Implement %s and run targeted tests", task.Frontmatter.ID)
	state.Body = renderStateBody(state.Frontmatter, tasks)

	if err := s.saveAll(state, tasks); err != nil {
		return TransitionResult{}, err
	}

	return TransitionResult{TaskID: taskID, Status: task.Frontmatter.Status, UpdatedAt: now}, nil
}

func (s *Service) Complete(taskID, note string) (TransitionResult, error) {
	return s.finishTask(taskID, "completed", strings.TrimSpace(note))
}

func (s *Service) Block(taskID, reason string) (TransitionResult, error) {
	if strings.TrimSpace(reason) == "" {
		return TransitionResult{}, errors.New("block reason must not be empty")
	}
	return s.finishTask(taskID, "blocked", strings.TrimSpace(reason))
}

func (s *Service) Verify(input VerifyInput) (VerifyResult, error) {
	if input.Result != "pass" && input.Result != "fail" {
		return VerifyResult{}, fmt.Errorf("invalid verify result %q", input.Result)
	}
	if strings.TrimSpace(input.Summary) == "" {
		return VerifyResult{}, errors.New("verify summary must not be empty")
	}

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return VerifyResult{}, err
	}
	task, ok := taskByID(tasks, input.TaskID)
	if !ok {
		return VerifyResult{}, fmt.Errorf("task %s not found", input.TaskID)
	}

	now := s.now().UTC()
	ts := now.Format("20060102T150405Z")
	artifactDir := filepath.Join(s.paths.VerifyDir, task.Frontmatter.ID, ts)
	if err := ensureDir(artifactDir); err != nil {
		return VerifyResult{}, err
	}

	planPath := filepath.Join(artifactDir, "plan.md")
	reportPath := filepath.Join(artifactDir, "report.json")
	reportMarkdownPath := filepath.Join(artifactDir, "report.md")

	followupTaskID := ""
	if input.CreateFollowup {
		newTask, err := s.createFollowupTask(tasks, task, input)
		if err != nil {
			return VerifyResult{}, err
		}
		tasks = append(tasks, newTask)
		followupTaskID = newTask.Frontmatter.ID
	}

	plan := renderVerificationPlan(task, input, followupTaskID)
	if err := os.WriteFile(planPath, []byte(plan), 0o644); err != nil {
		return VerifyResult{}, fmt.Errorf("write verification plan: %w", err)
	}

	relPlan := relPath(s.paths.RepoRoot, planPath)
	relReport := relPath(s.paths.RepoRoot, reportPath)
	relReportMarkdown := relPath(s.paths.RepoRoot, reportMarkdownPath)

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
	if err := os.WriteFile(reportPath, reportBytes, 0o644); err != nil {
		return VerifyResult{}, fmt.Errorf("write verification report: %w", err)
	}

	reportMarkdown := renderVerificationReportMarkdown(report)
	if err := os.WriteFile(reportMarkdownPath, []byte(reportMarkdown), 0o644); err != nil {
		return VerifyResult{}, fmt.Errorf("write verification markdown report: %w", err)
	}

	nowText := timestamp(now)
	// Task files are committed, so the note must stay portable: record the
	// result and timestamp without a path into gitignored artifacts (mirrors
	// the path-free state summary below).
	appendTaskNote(task, fmt.Sprintf("- %s: verification %s", nowText, input.Result))
	task.Frontmatter.UpdatedAt = nowText

	state.Frontmatter.UpdatedAt = nowText
	// Keep committed state portable: record a path-free summary and list no
	// gitignored artifact paths. Local evidence still lives under
	// planning/artifacts/verify/ for the producer (see VerifyResult).
	state.Frontmatter.LastVerificationResult = fmt.Sprintf("%s for %s at %s", input.Result, task.Frontmatter.ID, nowText)
	state.Frontmatter.RelevantArtifacts = nil
	if input.Result == "fail" && followupTaskID != "" {
		state.Frontmatter.NextAction = fmt.Sprintf("Review follow-up task %s", followupTaskID)
	} else if input.Result == "fail" {
		state.Frontmatter.NextAction = fmt.Sprintf("Resolve verification findings for %s", task.Frontmatter.ID)
	} else {
		state.Frontmatter.NextAction = "Select the next eligible task"
	}
	state.Body = renderStateBody(state.Frontmatter, tasks)

	if err := s.saveAll(state, tasks); err != nil {
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
	}, nil
}

// CreateTask scaffolds a well-formed task file with the next free id. It mirrors
// the validation `validate` would apply (spec anchor, dependency existence,
// priority) at creation time so an invalid task never lands on disk.
func (s *Service) CreateTask(input CreateTaskInput) (CreateTaskResult, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return CreateTaskResult{}, errors.New("task title must not be empty")
	}
	specRef := strings.TrimSpace(input.SpecRef)
	if specRef == "" {
		return CreateTaskResult{}, errors.New("task spec_ref must not be empty")
	}
	if err := s.validateSpecRef(specRef); err != nil {
		return CreateTaskResult{}, fmt.Errorf("invalid spec_ref: %w", err)
	}
	priority := strings.TrimSpace(input.Priority)
	if priority == "" {
		priority = "medium"
	}
	if _, ok := validPriorites[priority]; !ok {
		return CreateTaskResult{}, fmt.Errorf("invalid priority %q", priority)
	}

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return CreateTaskResult{}, err
	}

	deps := append([]string(nil), input.Dependencies...)
	for _, dep := range deps {
		if _, ok := taskByID(tasks, dep); !ok {
			return CreateTaskResult{}, fmt.Errorf("dependency %s does not exist", dep)
		}
	}

	nextID := nextTaskID(tasks)
	now := timestamp(s.now())
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
		Body:     renderNewTaskBody(nextID, title),
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
		return CreateTaskResult{}, err
	}

	return CreateTaskResult{
		TaskID:   nextID,
		Title:    title,
		Priority: priority,
		SpecRef:  specRef,
		Path:     relPath(s.paths.RepoRoot, newTask.Filename),
	}, nil
}

func (s *Service) finishTask(taskID, status, note string) (TransitionResult, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return TransitionResult{}, err
	}
	task, ok := taskByID(tasks, taskID)
	if !ok {
		return TransitionResult{}, fmt.Errorf("task %s not found", taskID)
	}
	if task.Frontmatter.Status != "in_progress" && !(status == "blocked" && task.Frontmatter.Status == "todo") {
		return TransitionResult{}, fmt.Errorf("task %s is not in a transitionable state", taskID)
	}

	now := timestamp(s.now())
	task.Frontmatter.Status = status
	task.Frontmatter.UpdatedAt = now
	if note != "" {
		appendTaskNote(task, fmt.Sprintf("- %s: %s", now, note))
	}

	if state.Frontmatter.CurrentTask == taskID {
		state.Frontmatter.CurrentTask = ""
		state.Frontmatter.CurrentTaskTitle = ""
	}
	state.Frontmatter.UpdatedAt = now
	state.Frontmatter.StatusSummary = "idle"
	if status == "blocked" {
		state.Frontmatter.Blockers = []string{fmt.Sprintf("%s: %s", taskID, note)}
		state.Frontmatter.StatusSummary = "blocked"
		state.Frontmatter.NextAction = fmt.Sprintf("Resolve blocker on %s", taskID)
	} else {
		state.Frontmatter.Blockers = []string{}
		state.Frontmatter.NextAction = "Select the next eligible task"
	}
	state.Body = renderStateBody(state.Frontmatter, tasks)

	if err := s.saveAll(state, tasks); err != nil {
		return TransitionResult{}, err
	}

	return TransitionResult{TaskID: taskID, Status: status, UpdatedAt: now}, nil
}

func (s *Service) validateState(state *State) []string {
	violations := make([]string, 0)
	if state.Frontmatter.SchemaVersion != stateSchemaVersion {
		violations = append(violations, fmt.Sprintf("state schema_version must be %d", stateSchemaVersion))
	}
	if strings.TrimSpace(state.Frontmatter.ActiveSpecPath) == "" {
		violations = append(violations, "state active_spec_path must not be empty")
	} else if !fileExists(filepath.Join(s.paths.RepoRoot, filepath.Clean(state.Frontmatter.ActiveSpecPath))) {
		violations = append(violations, fmt.Sprintf("state active_spec_path does not exist: %s", state.Frontmatter.ActiveSpecPath))
	}
	if strings.TrimSpace(state.Frontmatter.StatusSummary) == "" {
		violations = append(violations, "state status_summary must not be empty")
	}
	violations = append(violations, stateArtifactRefs(state.Frontmatter)...)
	return violations
}

// gitignoredArtifactPrefix is the repo-relative prefix of the gitignored
// planning/artifacts/ tree. A clean checkout drops that tree, so a committed
// reference to a concrete file under it always dangles. We pattern-match the
// prefix rather than shelling out to git (T-026).
const gitignoredArtifactPrefix = "planning/artifacts/"

// danglingArtifactPaths returns concrete file paths under the gitignored
// artifacts tree referenced in s. It targets producer-only pointers like
// planning/artifacts/verify/T-011/<ts>/report.json (the rot scrubbed in
// T-021/T-022/T-023), not the contract prose the scrub deliberately kept:
// bare directory prefixes (planning/artifacts/verify/) and placeholder paths
// (planning/artifacts/manual-test/T-019/<timestamp>/) are legitimate and pass.
// It scans for the prefix as a plain substring anywhere in s; committed
// planning prose does not embed the prefix inside unrelated tokens (e.g. URLs),
// so keeping the scan simple is preferred over anchoring to word boundaries.
func danglingArtifactPaths(s string) []string {
	paths := make([]string, 0)
	for rest := s; ; {
		idx := strings.Index(rest, gitignoredArtifactPrefix)
		if idx < 0 {
			break
		}
		end := idx
		for end < len(rest) && isArtifactPathByte(rest[end]) {
			end++
		}
		token := rest[idx:end]
		rest = rest[end:]
		if isConcreteArtifactFile(token) {
			paths = append(paths, token)
		}
	}
	return paths
}

func isArtifactPathByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	case b == '/' || b == '.' || b == '-' || b == '_' || b == '<' || b == '>':
		return true
	default:
		return false
	}
}

// isConcreteArtifactFile reports whether token names a specific file (not a
// directory or placeholder) inside the artifacts tree. Placeholder markers and
// bare directory references are treated as contract prose and ignored.
func isConcreteArtifactFile(token string) bool {
	if strings.ContainsAny(token, "<>") || strings.Contains(token, "...") {
		return false
	}
	base := path.Base(token)
	dot := strings.LastIndex(base, ".")
	if dot <= 0 || dot == len(base)-1 {
		return false
	}
	ext := base[dot+1:]
	// Artifact files use short extensions (report.json, report.md, .txt, .log).
	// Cap the extension length so a directory segment that merely contains a dot
	// (e.g. a versioned dir) is not mistaken for a file. Longer extensions are
	// out of scope: no producer writes them, and prose stays unflagged.
	if len(ext) > 6 {
		return false
	}
	for i := 0; i < len(ext); i++ {
		b := ext[i]
		if !(b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9') {
			return false
		}
	}
	return true
}

// stateArtifactRefs flags committed STATE.md frontmatter fields that point at a
// concrete gitignored artifact file, naming the offending field.
func stateArtifactRefs(fm StateFrontmatter) []string {
	violations := make([]string, 0)
	scan := func(field, value string) {
		for _, p := range danglingArtifactPaths(value) {
			violations = append(violations, fmt.Sprintf("state %s references gitignored artifact path %s", field, p))
		}
	}
	scan("last_verification_result", fm.LastVerificationResult)
	scan("next_action", fm.NextAction)
	scan("current_task_title", fm.CurrentTaskTitle)
	for _, v := range fm.RelevantArtifacts {
		scan("relevant_artifacts", v)
	}
	for _, v := range fm.ContinuationNotes {
		scan("continuation_notes", v)
	}
	for _, v := range fm.Blockers {
		scan("blockers", v)
	}
	return violations
}

// taskArtifactRefs flags a committed task file (frontmatter title and body
// lines) that points at a concrete gitignored artifact file.
func taskArtifactRefs(task *Task) []string {
	violations := make([]string, 0)
	id := task.Frontmatter.ID
	for _, p := range danglingArtifactPaths(task.Frontmatter.Title) {
		violations = append(violations, fmt.Sprintf("task %s title references gitignored artifact path %s", id, p))
	}
	// An artifact path never spans a newline (\n is not a path byte), so a
	// single whole-body scan yields the same tokens as a per-line scan.
	for _, p := range danglingArtifactPaths(task.Body) {
		violations = append(violations, fmt.Sprintf("task %s body references gitignored artifact path %s", id, p))
	}
	return violations
}

// detectDependencyCycles reports directed cycles among existing task
// dependencies. Missing deps and self-deps are reported by validateTasks, so
// only edges between distinct existing tasks are followed here. Each distinct
// cycle is reported once regardless of traversal entry point.
func detectDependencyCycles(tasks []*Task) []string {
	exists := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		exists[task.Frontmatter.ID] = struct{}{}
	}
	adj := make(map[string][]string, len(tasks))
	for _, task := range tasks {
		id := task.Frontmatter.ID
		for _, dep := range task.Frontmatter.Dependencies {
			if dep == id {
				continue // self-dependency handled by validateTasks
			}
			if _, ok := exists[dep]; ok {
				adj[id] = append(adj[id], dep)
			}
		}
	}

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(tasks))
	reported := make(map[string]struct{})
	violations := make([]string, 0)
	stack := make([]string, 0, len(tasks))

	// Recursion depth is bounded by the longest dependency chain; planning
	// backlogs are small, so an explicit work stack is not warranted.
	var visit func(id string)
	visit = func(id string) {
		color[id] = gray
		stack = append(stack, id)
		for _, dep := range adj[id] {
			switch color[dep] {
			case gray:
				cycle := append(append([]string{}, stack[slices.Index(stack, dep):]...), dep)
				sig := cycleSignature(cycle)
				if _, seen := reported[sig]; !seen {
					reported[sig] = struct{}{}
					violations = append(violations, "dependency cycle detected: "+strings.Join(cycle, " -> "))
				}
			case white:
				visit(dep)
			}
		}
		stack = stack[:len(stack)-1]
		color[id] = black
	}

	for _, task := range tasks {
		if color[task.Frontmatter.ID] == white {
			visit(task.Frontmatter.ID)
		}
	}
	return violations
}

// cycleSignature normalizes a directed cycle (given as [a, b, ..., a]) to a
// rotation starting at its smallest node so the same cycle discovered from
// different entry points yields one signature.
func cycleSignature(cycle []string) string {
	nodes := cycle[:len(cycle)-1] // drop the repeated closing node
	minIdx := 0
	for i, n := range nodes {
		if n < nodes[minIdx] {
			minIdx = i
		}
	}
	rotated := append(append([]string{}, nodes[minIdx:]...), nodes[:minIdx]...)
	return strings.Join(rotated, ">")
}

func (s *Service) validateTasks(state *State, tasks []*Task) []string {
	violations := make([]string, 0)
	seen := make(map[string]struct{}, len(tasks))
	inProgress := make([]*Task, 0)
	for _, task := range tasks {
		if _, ok := seen[task.Frontmatter.ID]; ok {
			violations = append(violations, fmt.Sprintf("duplicate task id %s", task.Frontmatter.ID))
			continue
		}
		seen[task.Frontmatter.ID] = struct{}{}

		if task.Frontmatter.ID == "" {
			violations = append(violations, fmt.Sprintf("task file %s missing id", relPath(s.paths.RepoRoot, task.Filename)))
		}
		if base := filepath.Base(task.Filename); base != task.Frontmatter.ID+".md" {
			violations = append(violations, fmt.Sprintf("task %s filename must be %s.md", task.Frontmatter.ID, task.Frontmatter.ID))
		}
		if _, ok := validStatuses[task.Frontmatter.Status]; !ok {
			violations = append(violations, fmt.Sprintf("task %s has invalid status %q", task.Frontmatter.ID, task.Frontmatter.Status))
		}
		if _, ok := validPriorites[task.Frontmatter.Priority]; !ok {
			violations = append(violations, fmt.Sprintf("task %s has invalid priority %q", task.Frontmatter.ID, task.Frontmatter.Priority))
		}
		if task.Frontmatter.Status == "in_progress" {
			inProgress = append(inProgress, task)
		}
		for _, dep := range task.Frontmatter.Dependencies {
			if dep == task.Frontmatter.ID {
				violations = append(violations, fmt.Sprintf("task %s cannot depend on itself", task.Frontmatter.ID))
			}
		}
		if strings.TrimSpace(task.Frontmatter.SpecRef) == "" {
			violations = append(violations, fmt.Sprintf("task %s missing spec_ref", task.Frontmatter.ID))
		} else if err := s.validateSpecRef(task.Frontmatter.SpecRef); err != nil {
			violations = append(violations, fmt.Sprintf("task %s invalid spec_ref: %v", task.Frontmatter.ID, err))
		}
		violations = append(violations, taskArtifactRefs(task)...)
	}

	for _, task := range tasks {
		for _, dep := range task.Frontmatter.Dependencies {
			if _, ok := seen[dep]; !ok {
				violations = append(violations, fmt.Sprintf("task %s depends on missing task %s", task.Frontmatter.ID, dep))
			}
		}
	}

	violations = append(violations, detectDependencyCycles(tasks)...)

	if len(inProgress) > 1 {
		ids := make([]string, 0, len(inProgress))
		for _, task := range inProgress {
			ids = append(ids, task.Frontmatter.ID)
		}
		violations = append(violations, fmt.Sprintf("multiple in_progress tasks: %s", strings.Join(ids, ", ")))
	}
	if len(inProgress) == 1 {
		if state.Frontmatter.CurrentTask != inProgress[0].Frontmatter.ID {
			violations = append(violations, fmt.Sprintf("state current_task %q does not match in_progress task %q", state.Frontmatter.CurrentTask, inProgress[0].Frontmatter.ID))
		}
	} else if state.Frontmatter.CurrentTask != "" {
		violations = append(violations, fmt.Sprintf("state current_task %q set but no task is in_progress", state.Frontmatter.CurrentTask))
	}

	return violations
}

// parseSpecRef splits a `path#anchor` spec reference into its structural parts
// without touching the filesystem. It is the shared shape check for both a real
// task's spec_ref (validateSpecRef adds the file/heading existence check) and an
// import draft's spec_ref (validated structurally only, before the spec exists).
func parseSpecRef(specRef string) (string, string, error) {
	parts := strings.SplitN(specRef, "#", 2)
	if len(parts) != 2 {
		return "", "", errors.New("spec_ref must include a path and heading anchor")
	}
	if strings.TrimSpace(parts[0]) == "" {
		return "", "", errors.New("spec_ref path must not be empty")
	}
	pathPart := filepath.Clean(parts[0])
	if pathPart == ".." || strings.HasPrefix(pathPart, ".."+string(filepath.Separator)) {
		return "", "", errors.New("spec_ref path must stay within the repository")
	}
	anchor := strings.TrimSpace(parts[1])
	if anchor == "" {
		return "", "", errors.New("spec_ref anchor must not be empty")
	}
	return pathPart, anchor, nil
}

func (s *Service) validateSpecRef(specRef string) error {
	pathPart, anchor, err := parseSpecRef(specRef)
	if err != nil {
		return err
	}

	fullPath := filepath.Join(s.paths.RepoRoot, pathPart)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("read spec file: %w", err)
	}
	anchors := collectHeadingAnchors(string(data))
	if _, ok := anchors[anchor]; !ok {
		return fmt.Errorf("heading #%s not found in %s", anchor, pathPart)
	}
	return nil
}

func (s *Service) createFollowupTask(tasks []*Task, source *Task, input VerifyInput) (*Task, error) {
	priority := strings.TrimSpace(input.FollowupPriority)
	if priority == "" {
		priority = "medium"
	}
	if _, ok := validPriorites[priority]; !ok {
		return nil, fmt.Errorf("invalid follow-up priority %q", priority)
	}

	nextID := nextTaskID(tasks)
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

	body := fmt.Sprintf("# %s %s\n\n## Description\n\n%s\n\n## Acceptance\n\n- The follow-up issue described by verification is resolved.\n- Verification evidence is updated.\n\n## Verification Notes\n\n- Re-run task-scoped verification after implementing the fix.\n", nextID, title, description)
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
		Filename: filepath.Join(s.paths.TasksDir, nextID+".md"),
	}
	return task, nil
}

func renderVerificationPlan(task *Task, input VerifyInput, followupTaskID string) string {
	var builder strings.Builder
	builder.WriteString("# Verification Plan\n\n")
	builder.WriteString(fmt.Sprintf("- Task: `%s`\n", task.Frontmatter.ID))
	builder.WriteString(fmt.Sprintf("- Title: %s\n", task.Frontmatter.Title))
	builder.WriteString(fmt.Sprintf("- Requested result: %s\n", input.Result))
	builder.WriteString(fmt.Sprintf("- Summary: %s\n", input.Summary))
	if input.Details != "" {
		builder.WriteString(fmt.Sprintf("- Details: %s\n", input.Details))
	}
	if followupTaskID != "" {
		builder.WriteString(fmt.Sprintf("- Follow-up task to create: `%s`\n", followupTaskID))
	}
	return builder.String()
}

func renderVerificationReportMarkdown(report VerificationArtifact) string {
	var builder strings.Builder
	builder.WriteString("# Verification Report\n\n")
	builder.WriteString(fmt.Sprintf("- Task: `%s`\n", report.TaskID))
	builder.WriteString(fmt.Sprintf("- Title: %s\n", report.TaskTitle))
	builder.WriteString(fmt.Sprintf("- Result: %s\n", report.Result))
	builder.WriteString(fmt.Sprintf("- Summary: %s\n", report.Summary))
	if report.Details != "" {
		builder.WriteString(fmt.Sprintf("- Details: %s\n", report.Details))
	}
	builder.WriteString(fmt.Sprintf("- Generated at: %s\n", report.GeneratedAt))
	builder.WriteString(fmt.Sprintf("- Spec ref: `%s`\n", report.SpecRef))
	for _, artifact := range report.Artifacts {
		builder.WriteString(fmt.Sprintf("- Artifact: `%s`\n", artifact))
	}
	if report.FollowupTaskID != "" {
		builder.WriteString(fmt.Sprintf("- Follow-up task: `%s`\n", report.FollowupTaskID))
	}
	return builder.String()
}

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
		return nil, fmt.Errorf("read state file: %w", err)
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
		return nil, fmt.Errorf("read tasks dir: %w", err)
	}
	tasks := make([]*Task, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		filename := filepath.Join(s.paths.TasksDir, entry.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("read task %s: %w", entry.Name(), err)
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
	return os.WriteFile(s.paths.StateFile, data, 0o644)
}

func (s *Service) saveTask(task *Task) error {
	data, err := marshalFrontmatter(task.Frontmatter, task.Body)
	if err != nil {
		return err
	}
	return os.WriteFile(task.Filename, data, 0o644)
}

func ensureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", path, err)
	}
	return nil
}

func writeFileIfMissing(path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
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

func taskByID(tasks []*Task, id string) (*Task, bool) {
	for _, task := range tasks {
		if task.Frontmatter.ID == id {
			return task, true
		}
	}
	return nil, false
}

func eligibleTasks(tasks []*Task) []*Task {
	eligible := make([]*Task, 0)
	for _, task := range tasks {
		if task.Frontmatter.Status != "todo" {
			continue
		}
		if dependenciesResolved(task, tasks) {
			eligible = append(eligible, task)
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		left := priorityRank[eligible[i].Frontmatter.Priority]
		right := priorityRank[eligible[j].Frontmatter.Priority]
		if left != right {
			return left < right
		}
		return eligible[i].Frontmatter.ID < eligible[j].Frontmatter.ID
	})
	return eligible
}

func dependenciesResolved(task *Task, tasks []*Task) bool {
	for _, dep := range task.Frontmatter.Dependencies {
		found, ok := taskByID(tasks, dep)
		if !ok || found.Frontmatter.Status != "completed" {
			return false
		}
	}
	return true
}

func appendTaskNote(task *Task, line string) {
	section := "## Implementation Notes\n\n"
	if strings.Contains(task.Body, section) {
		task.Body = strings.TrimRight(task.Body, "\n") + "\n" + line + "\n"
		return
	}
	task.Body = strings.TrimRight(task.Body, "\n") + "\n\n" + section + line + "\n"
}

func nextTaskID(tasks []*Task) string {
	max := 0
	for _, task := range tasks {
		if !strings.HasPrefix(task.Frontmatter.ID, "T-") {
			continue
		}
		num, err := strconv.Atoi(strings.TrimPrefix(task.Frontmatter.ID, "T-"))
		if err == nil && num > max {
			max = num
		}
	}
	return fmt.Sprintf("T-%03d", max+1)
}

func collectHeadingAnchors(markdown string) map[string]struct{} {
	anchors := make(map[string]struct{})
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		if heading == "" {
			continue
		}
		anchors[slugHeading(heading)] = struct{}{}
	}
	return anchors
}

func slugHeading(heading string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(heading) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-':
			if !lastDash && builder.Len() > 0 {
				builder.WriteRune('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}
