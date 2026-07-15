package taskrail

const stateSchemaVersion = 1

// status_summary values the transition, reconcile, and repair paths write into
// STATE.md. This axis (idle | in_progress | blocked) is distinct from a task's own
// status field (validStatuses) even where the strings coincide; centralizing it
// keeps the writers from drifting apart.
const (
	statusSummaryIdle       = "idle"
	statusSummaryInProgress = "in_progress"
	statusSummaryBlocked    = "blocked"
)

// nextActionSelectEligible is the neutral next_action pointer shared by the idle
// reconciliation and a passing verification, held identical so the two never drift.
const nextActionSelectEligible = "Select the next eligible task"

var (
	validStatuses  = map[string]struct{}{"todo": {}, "in_progress": {}, "completed": {}, "blocked": {}, "cancelled": {}}
	validPriorites = map[string]struct{}{"high": {}, "medium": {}, "low": {}}
	priorityRank   = map[string]int{"high": 0, "medium": 1, "low": 2}
)

type Paths struct {
	RepoRoot     string
	SpecsDir     string
	PlanningDir  string
	TasksDir     string
	ArtifactsDir string
	VerifyDir    string
	StateFile    string
}

// LayoutConfig is the machine-owned `.taskrail/config.yml` marker. It signals
// that a repository is Taskrail-managed, pins the layout version for upgrades,
// and records where the human-facing directories live.
type LayoutConfig struct {
	LayoutVersion int    `yaml:"layout_version" json:"layout_version"`
	SpecsDir      string `yaml:"specs_dir" json:"specs_dir"`
	PlanningDir   string `yaml:"planning_dir" json:"planning_dir"`
}

// InitOutcome classifies what version-aware init did to a repository.
type InitOutcome string

const (
	InitCreated          InitOutcome = "created"           // fresh layout written in an empty repo
	InitAdopted          InitOutcome = "adopted"           // legacy layout marked, nothing else changed
	InitCurrent          InitOutcome = "current"           // marker already at the current version
	InitMigrationPreview InitOutcome = "migration_preview" // older version, dry-run diff only
	InitMigrated         InitOutcome = "migrated"          // older version, migration applied
	InitRetrofitPreview  InitOutcome = "retrofit_preview"  // non-standard layout, dry-run proposal only
	InitRetrofitApplied  InitOutcome = "retrofit_applied"  // non-standard layout adopted after confirmation
)

// RetrofitMapping proposes how one detected candidate directory in a
// non-standard repository relates to a Taskrail layout role. It is a detection
// proposal the human confirms before the standard layout is scaffolded; applying
// a retrofit never moves or rewrites the source directory's content (content
// migration is a later flow), it only creates the missing Taskrail structure.
type RetrofitMapping struct {
	Source string `json:"source"` // detected candidate directory (repo-relative)
	Target string `json:"target"` // Taskrail directory it maps onto (repo-relative)
	Role   string `json:"role"`   // Taskrail role the target fills ("specs" | "planning")
}

// InitResult reports what version-aware init observed and did. Changes is the
// human-readable diff (populated for migration outcomes); Validation is set only
// after an applied migration re-runs validation.
type InitResult struct {
	Outcome     InitOutcome       `json:"outcome"`
	FromVersion int               `json:"from_version"`
	ToVersion   int               `json:"to_version"`
	Applied     bool              `json:"applied"`
	Changes     []string          `json:"changes,omitempty"`
	Mapping     []RetrofitMapping `json:"mapping,omitempty"`
	Validation  *ValidationResult `json:"validation,omitempty"`
}

type StateFrontmatter struct {
	SchemaVersion          int      `yaml:"schema_version" json:"schema_version"`
	UpdatedAt              string   `yaml:"updated_at" json:"updated_at"`
	ActiveSpecVersion      string   `yaml:"active_spec_version" json:"active_spec_version"`
	ActiveSpecPath         string   `yaml:"active_spec_path" json:"active_spec_path"`
	CurrentTask            string   `yaml:"current_task" json:"current_task"`
	CurrentTaskTitle       string   `yaml:"current_task_title" json:"current_task_title"`
	StatusSummary          string   `yaml:"status_summary" json:"status_summary"`
	Blockers               []string `yaml:"blockers" json:"blockers"`
	NextAction             string   `yaml:"next_action" json:"next_action"`
	LastVerificationResult string   `yaml:"last_verification_result" json:"last_verification_result"`
	RelevantArtifacts      []string `yaml:"relevant_artifacts" json:"relevant_artifacts"`
	ContinuationNotes      []string `yaml:"continuation_notes" json:"continuation_notes"`
}

type State struct {
	Frontmatter StateFrontmatter
	Body        string
}

type TaskFrontmatter struct {
	ID           string   `yaml:"id" json:"id"`
	Title        string   `yaml:"title" json:"title"`
	Status       string   `yaml:"status" json:"status"`
	Priority     string   `yaml:"priority" json:"priority"`
	SpecRef      string   `yaml:"spec_ref" json:"spec_ref"`
	Dependencies []string `yaml:"dependencies" json:"dependencies"`
	UpdatedAt    string   `yaml:"updated_at" json:"updated_at"`
}

type Task struct {
	Frontmatter TaskFrontmatter
	Body        string
	Filename    string
}

type ValidationResult struct {
	Valid      bool     `json:"valid"`
	Violations []string `json:"violations"`
}

type NextResult struct {
	TaskID     string    `json:"task_id,omitempty"`
	Title      string    `json:"title,omitempty"`
	Priority   string    `json:"priority,omitempty"`
	Reason     string    `json:"reason"`
	Candidates []string  `json:"candidates"`
	Warnings   []Warning `json:"warnings,omitempty"`
}

type Warning struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	TaskID         string `json:"task_id,omitempty"`
	SpecRef        string `json:"spec_ref,omitempty"`
	ActiveSpecPath string `json:"active_spec_path,omitempty"`
}

type CreateTaskInput struct {
	Title string
	// Slug is the raw source for the id's human-scannable suffix. When it slugifies
	// to a non-empty value the id becomes `T-<n>-<slug>` with a matching filename;
	// when empty (or all non-alphanumeric) the id stays the bare `T-<n>` form. The
	// `task new` command passes `--slug` if given, else the title, so CLI-authored
	// tasks are slugged by default while other callers (import) stay bare.
	Slug         string
	SpecRef      string
	Priority     string
	Dependencies []string
	// FollowUpOf names a parent task id. When set, the new task inherits the
	// parent's spec_ref (unless SpecRef overrides it), lists the parent in its
	// dependencies, and records the follow-up provenance in its body.
	FollowUpOf string
}

type CreateTaskResult struct {
	TaskID   string `json:"task_id"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	SpecRef  string `json:"spec_ref"`
	Path     string `json:"path"`
}

type TransitionResult struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
}

// UnblockResult reports the blocked->todo transition Unblock performed plus the
// validation it re-ran afterward, mirroring SpecActivateResult's shape so the
// spec's "re-runs validation, reporting the result" contract is machine-readable.
type UnblockResult struct {
	TaskID     string           `json:"task_id"`
	Status     string           `json:"status"`
	UpdatedAt  string           `json:"updated_at"`
	Validation ValidationResult `json:"validation"`
}

type VerifyInput struct {
	TaskID              string
	Result              string
	Summary             string
	Details             string
	CreateFollowup      bool
	FollowupTitle       string
	FollowupDescription string
	FollowupPriority    string
}

type VerifyResult struct {
	TaskID         string `json:"task_id"`
	Result         string `json:"result"`
	ArtifactDir    string `json:"artifact_dir"`
	PlanPath       string `json:"plan_path"`
	ReportPath     string `json:"report_path"`
	ReportMarkdown string `json:"report_markdown"`
	FollowupTaskID string `json:"followup_task_id,omitempty"`
}

type VerificationArtifact struct {
	SchemaVersion  int      `json:"schema_version"`
	TaskID         string   `json:"task_id"`
	TaskTitle      string   `json:"task_title"`
	Result         string   `json:"result"`
	Summary        string   `json:"summary"`
	Details        string   `json:"details,omitempty"`
	GeneratedAt    string   `json:"generated_at"`
	SpecRef        string   `json:"spec_ref"`
	Artifacts      []string `json:"artifacts"`
	FollowupTaskID string   `json:"followup_task_id,omitempty"`
}
