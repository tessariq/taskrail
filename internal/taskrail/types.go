package taskrail

const stateSchemaVersion = 1

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
	TaskID     string   `json:"task_id,omitempty"`
	Title      string   `json:"title,omitempty"`
	Priority   string   `json:"priority,omitempty"`
	Reason     string   `json:"reason"`
	Candidates []string `json:"candidates"`
}

type TransitionResult struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
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
