package taskrail

import "github.com/tessariq/taskrail/internal/repolock"

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
	// RepoRoot remains the logical repository root used by existing callers.
	// ManagedRoot names the same identity explicitly beside the independent Git
	// and physical-storage identities discovery now preserves.
	RepoRoot     string
	ManagedRoot  string
	WorktreeRoot string
	GitDir       string
	GitCommonDir string
	ConfigFile   string
	StorageRoot  string
	LockRoot     string
	// Storage is the active storage context every physical directory below was
	// resolved through, so a reporter states the mode and root it actually used
	// rather than re-deriving one.
	Storage StorageContext
	// LogicalSpecsDir and LogicalPlanningDir are the directory names the marker
	// records. They stay logical in every storage mode, so a reported managed
	// path never carries a physical overlay prefix.
	LogicalSpecsDir    string
	LogicalPlanningDir string
	LogicalPromptsDir  string
	SpecsDir           string
	PlanningDir        string
	PromptsDir         string
	TasksDir           string
	ArtifactsDir       string
	VerifyDir          string
	RuntimeDir         string
	StateFile          string
}

// LockRepository projects the discovered identity into the existing lock
// contract without asking a caller to reconstruct roots or translate modes.
func (p Paths) LockRepository() repolock.Repository {
	mode := repolock.ModeCommitted
	if p.Storage.Mode == StorageLocal {
		mode = repolock.ModeLocal
	}
	return repolock.Repository{Root: p.ManagedRoot, GitCommonDir: p.GitCommonDir, Mode: mode}
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

// WriteEntry is one path an operation creates, leaves alone, rewrites, or
// removes. Paths are managed logical repository paths except where the entry's
// kind names a physical destination (`skill`, `runtime`), which keeps its
// discovery location in every storage mode.
type WriteEntry struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Action string `json:"action"`
}

// The WriteEntry kinds and actions this version emits. Init writes layout
// content only, so `task`, `review`, `runtime`, and `remove` belong to other
// producers; installed skills are reported through `skills`, which is its own
// array with its own action vocabulary.
const (
	writeKindConfig = "config"
	writeKindSpec   = "spec"
	writeKindState  = "state"
	writeKindNote   = "note"

	writeActionCreate   = "create"
	writeActionPreserve = "preserve"
	writeActionRefresh  = "refresh"
)

// InitConfig reports the layout marker init wrote or kept. CandidateSHA256
// digests the exact marker bytes the outcome publishes, so preview and apply
// name the same candidate and a caller can tell an unchanged marker from a
// rewritten one without reading the file.
type InitConfig struct {
	Path            string `json:"path"`
	Action          string `json:"action"`
	CandidateSHA256 string `json:"candidate_sha256"`
}

// The config actions: a fresh marker, an existing one left as it is, and one
// rewritten to the current layout version.
const (
	configActionCreate   = "create"
	configActionPreserve = "preserve"
	configActionMigrate  = "migrate"
)

// InitNote reports the human-owned notes sidecar: what happened to the file, and
// which continuation-note dispositions the repository's state makes available.
// ContinuationAction is null until an operator selects one; the choices are what
// they may select from, in the contract's `extract|drop` order.
type InitNote struct {
	Path                string   `json:"path"`
	FileAction          string   `json:"file_action"`
	ContinuationAction  *string  `json:"continuation_action"`
	ContinuationChoices []string `json:"continuation_choices"`
}

// The notes file actions: a template written into an absent destination, an
// existing human-owned sidecar left untouched, and an outcome that does not
// scaffold layout content at all.
const (
	noteActionCreateTemplate = "create_template"
	noteActionPreserve       = "preserve"
	noteActionNone           = "none"

	continuationChoiceExtract = "extract"
	continuationChoiceDrop    = "drop"
)

// InitSkill is one installed packaged-skill file at its normal assistant
// discovery path, which is the same in committed and local storage.
type InitSkill struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

// InitSkillExclusion is one packaged-skill destination subtree Taskrail manages
// an exclusion for. Only a local installation owns exclusions; a committed one
// reports none.
type InitSkillExclusion struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

// InitInput selects what one init invocation does. Skills are opt-in: without
// WithSkills no assistant directory is touched and both skill inventories are
// empty.
type InitInput struct {
	Apply        bool
	WithSkills   bool
	ForceSkills  bool
	SkillVersion string
}

// InitResult reports what version-aware init observed and did, in the exact
// shape specs/contracts/v0.5.0-machine-api.md fixes for it. Preview and apply
// expose the same candidate paths and choices, so an agent can decide from a dry
// run exactly what applying would do. Validation is set only where the outcome
// re-runs it.
type InitResult struct {
	Outcome           InitOutcome          `json:"outcome"`
	FromVersion       int                  `json:"from_version"`
	ToVersion         int                  `json:"to_version"`
	Applied           bool                 `json:"applied"`
	StorageMode       string               `json:"storage_mode"`
	Config            InitConfig           `json:"config"`
	Writes            []WriteEntry         `json:"writes"`
	Notes             []InitNote           `json:"notes"`
	Skills            []InitSkill          `json:"skills"`
	SkillExclusions   []InitSkillExclusion `json:"skill_exclusions"`
	ContinuationNotes []string             `json:"continuation_notes"`
	Validation        *ValidationResult    `json:"validation"`

	// Mapping and SkillInstall are the retrofit proposal and the installer's own
	// created/overwritten/backed-up lists. Both exist for the human summary:
	// `RetrofitResult` owns the published mapping, and the skill inventory above
	// owns the published installation, so neither belongs on init's wire shape.
	Mapping      []RetrofitMapping  `json:"-"`
	SkillInstall SkillInstallResult `json:"-"`
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
	ID                             string   `yaml:"id" json:"id"`
	Title                          string   `yaml:"title" json:"title"`
	Status                         string   `yaml:"status" json:"status"`
	Priority                       string   `yaml:"priority" json:"priority"`
	SpecRef                        string   `yaml:"spec_ref" json:"spec_ref"`
	Dependencies                   []string `yaml:"dependencies" json:"dependencies"`
	UpdatedAt                      string   `yaml:"updated_at" json:"updated_at"`
	LoopPolicyMetadata             `yaml:",inline"`
	CompletionVerificationMetadata `yaml:",inline"`
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
	TaskID     string   `json:"task_id,omitempty"`
	Title      string   `json:"title,omitempty"`
	Priority   string   `json:"priority,omitempty"`
	Reason     string   `json:"reason"`
	Candidates []string `json:"candidates"`
	// OffSpec reports whether the selected task's spec_ref points away from the
	// active spec. It can only be true on the --include-off-spec recovery path
	// (default idle selection filters off-spec work out); it is always emitted so
	// agents can distinguish an active-spec pick from an off-spec pick.
	OffSpec  bool      `json:"off_spec"`
	Warnings []Warning `json:"-"`
}

// Warning is one advisory a command raised in process. It is never a published
// member of a result: the envelope's `warnings` array is the single wire
// location for every advisory, so these values only travel from the service to
// the boundary that projects them onto the closed union
// (specs/v0.5.0.md#uniform-agent-machine-results).
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
	// `task new` and import pass the title when no curated slug exists, so titled
	// CLI-authored tasks are slugged by default.
	Slug string
	// SlugExplicit reports that Slug is an operator-curated `--slug`, not the
	// title fallback. A title-derived slug is length-capped (see capSlug); an
	// explicit one is written verbatim after normalization, since the operator
	// owns that choice. It also preserves an explicitly empty --slug so it can
	// override a title-derived slug and trigger the warned bare-id fallback.
	SlugExplicit bool
	// SlugSourceSupplied distinguishes an explicitly empty title or slug source
	// from the intentional no-selector case, without changing title-derived capping.
	SlugSourceSupplied bool
	// SpecRef is the explicit `path#anchor` spec reference. Area is its active-spec
	// shorthand: when set, CreateTask resolves SpecRef to
	// `<active_spec_path>#<Area>` from STATE.md. The two are mutually exclusive — a
	// task has exactly one resolved spec reference.
	SpecRef      string
	Area         string
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
	// Warnings reports non-fatal signals about the created task — currently only
	// the empty-derived-slug fallback.
	Warnings []Warning `json:"-"`
}

// The three lifecycle writers each report their own transition plus the
// validation they re-ran afterward, so an agent learns from one document what
// the transition did and whether the repository is still valid. Their field sets
// are fixed by specs/v0.5.0.md#uniform-agent-machine-results.
type StartResult struct {
	TaskID     string           `json:"task_id"`
	Status     string           `json:"status"`
	UpdatedAt  string           `json:"updated_at"`
	Validation ValidationResult `json:"validation"`
}

type CompleteResult struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
	// CompletionID is the completed task's persisted completion identity. It is
	// empty until T-158 creates one; complete reports whatever the task carries
	// rather than inventing an identity here.
	CompletionID string           `json:"completion_id"`
	Validation   ValidationResult `json:"validation"`
}

type BlockResult struct {
	TaskID     string           `json:"task_id"`
	Status     string           `json:"status"`
	Reason     string           `json:"reason"`
	UpdatedAt  string           `json:"updated_at"`
	Validation ValidationResult `json:"validation"`
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
	TaskID         string    `json:"task_id"`
	Result         string    `json:"result"`
	ArtifactDir    string    `json:"artifact_dir"`
	PlanPath       string    `json:"plan_path"`
	ReportPath     string    `json:"report_path"`
	ReportMarkdown string    `json:"report_markdown"`
	FollowupTaskID string    `json:"followup_task_id,omitempty"`
	Warnings       []Warning `json:"-"`
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
