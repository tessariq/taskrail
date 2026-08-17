package taskrail

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const layout2Version = 2

type Layout2MigrationFence struct {
	FromLayoutVersion int    `yaml:"from_layout_version" json:"from_layout_version"`
	TransactionID     string `yaml:"transaction_id" json:"transaction_id"`
}

type Layout2Config struct {
	LayoutVersion                 int                    `yaml:"layout_version" json:"layout_version"`
	SpecsDir                      string                 `yaml:"specs_dir" json:"specs_dir"`
	PlanningDir                   string                 `yaml:"planning_dir" json:"planning_dir"`
	StorageMode                   StorageMode            `yaml:"storage_mode" json:"storage_mode"`
	ImplementationReviewMaxRounds int                    `yaml:"implementation_review_max_rounds" json:"implementation_review_max_rounds"`
	MigrationFence                *Layout2MigrationFence `yaml:"migration_fence,omitempty" json:"migration_fence,omitempty"`
}

type MigrationSkillOutcome string

const (
	migrationSkillParity  MigrationSkillOutcome = "parity"
	migrationSkillRefresh MigrationSkillOutcome = "refresh"
)

type MigrationSkillCandidate struct {
	Path    string
	Outcome MigrationSkillOutcome
	Marker  string
	Version string
}

type Layout2MigrationCandidate struct {
	Marker               Layout2Config
	MarkerPath           string
	MarkerBytes          []byte
	StatePath            string
	StateBytes           []byte
	TaskBytes            map[string][]byte
	ContinuationNotes    []string
	NotesPath            string
	NotesPresent         bool
	NotesTemplateBytes   []byte
	NotesExtractionBytes []byte
	Skills               []MigrationSkillCandidate
}

type stateV2Frontmatter struct {
	SchemaVersion              int      `yaml:"schema_version"`
	UpdatedAt                  string   `yaml:"updated_at"`
	ActiveSpecVersion          string   `yaml:"active_spec_version"`
	ActiveSpecPath             string   `yaml:"active_spec_path"`
	CurrentTask                string   `yaml:"current_task"`
	CurrentTaskTitle           string   `yaml:"current_task_title"`
	StatusSummary              string   `yaml:"status_summary"`
	Blockers                   []string `yaml:"blockers"`
	NextAction                 string   `yaml:"next_action"`
	LastVerificationResult     string   `yaml:"last_verification_result"`
	LastVerificationID         string   `yaml:"last_verification_id,omitempty"`
	LastVerificationPreviousID string   `yaml:"last_verification_previous_id,omitempty"`
	LastVerifiedCompletionID   string   `yaml:"last_verified_completion_id,omitempty"`
	RelevantArtifacts          []string `yaml:"relevant_artifacts"`
}

type decodedMigrationState struct {
	Frontmatter       stateV2Frontmatter
	ContinuationNotes []string
	SourceSchema      int
}

var lowerHex32 = regexp.MustCompile(`^[0-9a-f]{32}$`)
var canonicalStateVerification = regexp.MustCompile(`^(pass|fail) for (T-[^ ]+) at ([^ ]+) id ([0-9a-f]{32})$`)

func decodeLayoutMarkerStrict(data []byte) (Layout2Config, error) {
	keys, node, err := strictYAMLMapping(data)
	if err != nil {
		return Layout2Config{}, fmt.Errorf("parse layout marker: %w", err)
	}
	versionNode := mappingValue(node, "layout_version")
	if versionNode == nil {
		return Layout2Config{}, fmt.Errorf("layout marker is missing layout_version")
	}
	var version int
	if err := versionNode.Decode(&version); err != nil {
		return Layout2Config{}, fmt.Errorf("layout_version must be an integer")
	}

	switch version {
	case 1:
		if err := requireExactKeys(keys, []string{"layout_version", "specs_dir", "planning_dir"}, nil); err != nil {
			return Layout2Config{}, err
		}
		var legacy LayoutConfig
		if err := strictYAMLDecode(data, &legacy); err != nil {
			return Layout2Config{}, err
		}
		if strings.TrimSpace(legacy.SpecsDir) == "" || strings.TrimSpace(legacy.PlanningDir) == "" {
			return Layout2Config{}, fmt.Errorf("layout 1 directories must be non-empty")
		}
		return Layout2Config{LayoutVersion: 1, SpecsDir: legacy.SpecsDir, PlanningDir: legacy.PlanningDir}, nil
	case layout2Version:
		required := []string{"layout_version", "specs_dir", "planning_dir", "storage_mode", "implementation_review_max_rounds"}
		if err := requireExactKeys(keys, required, []string{"migration_fence"}); err != nil {
			return Layout2Config{}, err
		}
		var cfg Layout2Config
		if err := strictYAMLDecode(data, &cfg); err != nil {
			return Layout2Config{}, err
		}
		if strings.TrimSpace(cfg.SpecsDir) == "" || strings.TrimSpace(cfg.PlanningDir) == "" {
			return Layout2Config{}, fmt.Errorf("layout 2 directories must be non-empty")
		}
		if cfg.StorageMode != StorageCommitted && cfg.StorageMode != StorageLocal {
			return Layout2Config{}, fmt.Errorf("storage_mode must be committed or local")
		}
		if cfg.ImplementationReviewMaxRounds < 1 || cfg.ImplementationReviewMaxRounds > 2 {
			return Layout2Config{}, fmt.Errorf("implementation_review_max_rounds must be between 1 and 2")
		}
		if keys["migration_fence"] && cfg.MigrationFence == nil {
			return Layout2Config{}, fmt.Errorf("migration_fence must be omitted or an exact fence mapping")
		}
		if cfg.MigrationFence != nil {
			if cfg.StorageMode != StorageCommitted || cfg.MigrationFence.FromLayoutVersion != 1 || !lowerHex32.MatchString(cfg.MigrationFence.TransactionID) {
				return Layout2Config{}, fmt.Errorf("migration_fence must be an exact layout 1 committed migration fence")
			}
		}
		return cfg, nil
	default:
		return Layout2Config{}, fmt.Errorf("layout_version must be 1 or 2")
	}
}

func decodeStateStrict(data []byte) (decodedMigrationState, string, error) {
	frontmatter, body, err := splitFrontmatter(data)
	if err != nil {
		return decodedMigrationState{}, "", err
	}
	keys, node, err := strictYAMLMapping(frontmatter)
	if err != nil {
		return decodedMigrationState{}, "", fmt.Errorf("parse state frontmatter: %w", err)
	}
	versionNode := mappingValue(node, "schema_version")
	if versionNode == nil {
		return decodedMigrationState{}, "", fmt.Errorf("state is missing schema_version")
	}
	var version int
	if err := versionNode.Decode(&version); err != nil {
		return decodedMigrationState{}, "", fmt.Errorf("state schema_version must be an integer")
	}
	common := []string{"schema_version", "updated_at", "active_spec_version", "active_spec_path", "current_task", "current_task_title", "status_summary", "blockers", "next_action", "last_verification_result", "relevant_artifacts"}
	var decoded decodedMigrationState
	if err := validateStateNodeTypes(node); err != nil {
		return decodedMigrationState{}, "", err
	}
	switch version {
	case 1:
		if err := requireExactKeys(keys, append(slices.Clone(common), "continuation_notes"), nil); err != nil {
			return decodedMigrationState{}, "", err
		}
		var legacy StateFrontmatter
		if err := strictYAMLDecode(frontmatter, &legacy); err != nil {
			return decodedMigrationState{}, "", err
		}
		if legacy.ContinuationNotes == nil {
			return decodedMigrationState{}, "", fmt.Errorf("state continuation_notes must be non-null")
		}
		decoded = decodedMigrationState{Frontmatter: stateV2FromLegacy(legacy), ContinuationNotes: slices.Clone(legacy.ContinuationNotes), SourceSchema: 1}
	case 2:
		optional := []string{"last_verification_id", "last_verification_previous_id", "last_verified_completion_id"}
		if err := requireExactKeys(keys, common, optional); err != nil {
			return decodedMigrationState{}, "", err
		}
		if hasMarkdownHeading(body, "Notes") {
			return decodedMigrationState{}, "", fmt.Errorf("state schema 2 must not render Notes")
		}
		if err := strictYAMLDecode(frontmatter, &decoded.Frontmatter); err != nil {
			return decodedMigrationState{}, "", err
		}
		decoded.SourceSchema = 2
	default:
		return decodedMigrationState{}, "", fmt.Errorf("state schema_version must be 1 or 2")
	}
	if err := validateDecodedState(decoded.Frontmatter, keys); err != nil {
		return decodedMigrationState{}, "", err
	}
	return decoded, body, nil
}

func stateV2FromLegacy(legacy StateFrontmatter) stateV2Frontmatter {
	return stateV2Frontmatter{
		SchemaVersion: 2, UpdatedAt: legacy.UpdatedAt, ActiveSpecVersion: legacy.ActiveSpecVersion,
		ActiveSpecPath: legacy.ActiveSpecPath, CurrentTask: legacy.CurrentTask,
		CurrentTaskTitle: legacy.CurrentTaskTitle, StatusSummary: legacy.StatusSummary,
		Blockers: slices.Clone(legacy.Blockers), NextAction: legacy.NextAction,
		LastVerificationResult: legacy.LastVerificationResult,
		RelevantArtifacts:      slices.Clone(legacy.RelevantArtifacts),
	}
}

func validateDecodedState(state stateV2Frontmatter, keys map[string]bool) error {
	if _, err := time.Parse(time.RFC3339, state.UpdatedAt); err != nil || !strings.HasSuffix(state.UpdatedAt, "Z") {
		return fmt.Errorf("state updated_at must be RFC3339 UTC")
	}
	if strings.TrimSpace(state.ActiveSpecPath) == "" || strings.TrimSpace(state.StatusSummary) == "" || strings.TrimSpace(state.LastVerificationResult) == "" {
		return fmt.Errorf("state active_spec_path, status_summary, and last_verification_result must be non-empty")
	}
	if state.ActiveSpecVersion == "" || state.NextAction == "" {
		return fmt.Errorf("state active_spec_version and next_action must be non-empty")
	}
	if state.StatusSummary != statusSummaryIdle && state.StatusSummary != statusSummaryInProgress && state.StatusSummary != statusSummaryBlocked {
		return fmt.Errorf("state status_summary must be idle, in_progress, or blocked")
	}
	if (state.CurrentTask == "") != (state.CurrentTaskTitle == "") {
		return fmt.Errorf("state current_task and current_task_title must both be set or both be empty")
	}
	if state.Blockers == nil || state.RelevantArtifacts == nil || len(state.RelevantArtifacts) != 0 {
		return fmt.Errorf("state blockers and relevant_artifacts must be non-null, and relevant_artifacts must be empty")
	}
	if len(danglingArtifactPaths(state.LastVerificationResult)) > 0 {
		return fmt.Errorf("state last_verification_result must be path-free")
	}
	hasID := keys["last_verification_id"]
	hasPrevious := keys["last_verification_previous_id"]
	hasCompletion := keys["last_verified_completion_id"]
	if !hasID {
		if hasPrevious || hasCompletion {
			return fmt.Errorf("verification predecessor and completion binding require last_verification_id")
		}
		return nil
	}
	if !lowerHex32.MatchString(state.LastVerificationID) {
		return fmt.Errorf("last_verification_id must be lower-case 32-hex")
	}
	match := canonicalStateVerification.FindStringSubmatch(state.LastVerificationResult)
	if match == nil || match[4] != state.LastVerificationID {
		return fmt.Errorf("last_verification_result must be canonical and match last_verification_id")
	}
	if _, full := taskIDPrefix(match[2]); !full {
		return fmt.Errorf("last_verification_result must name a full task id")
	}
	if parsed, err := time.Parse(time.RFC3339, match[3]); err != nil || parsed.Format(time.RFC3339) != match[3] || !strings.HasSuffix(match[3], "Z") {
		return fmt.Errorf("last_verification_result timestamp must be canonical RFC3339 UTC")
	}
	if hasPrevious && !lowerHex32.MatchString(state.LastVerificationPreviousID) {
		return fmt.Errorf("last_verification_previous_id must be lower-case 32-hex")
	}
	if hasPrevious && state.LastVerificationPreviousID == state.LastVerificationID {
		return fmt.Errorf("last_verification_previous_id must differ from last_verification_id")
	}
	if hasCompletion && (!lowerHex32.MatchString(state.LastVerifiedCompletionID) || match[1] != "pass") {
		return fmt.Errorf("last_verified_completion_id requires a passing verification and lower-case 32-hex")
	}
	return nil
}

func decodeMigrationTaskStrict(data []byte) (*Task, error) {
	frontmatter, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	keys, node, err := strictYAMLMapping(frontmatter)
	if err != nil {
		return nil, fmt.Errorf("parse task frontmatter: %w", err)
	}
	required := []string{"id", "title", "status", "priority", "spec_ref", "dependencies", "updated_at"}
	optional := []string{"loop_policy", "loop_reason", "completion_id", "last_verification_id", "last_verification_previous_id", "last_verification_result", "last_verified_at", "last_verified_completion_id"}
	if err := requireExactKeys(keys, required, optional); err != nil {
		return nil, err
	}
	if err := validateTaskNodeTypes(node); err != nil {
		return nil, err
	}
	var fm TaskFrontmatter
	if err := strictYAMLDecode(frontmatter, &fm); err != nil {
		return nil, err
	}
	if fields := loopPolicyFieldsInBody(body); len(fields) > 0 {
		return nil, fmt.Errorf("task has %s outside frontmatter", strings.Join(fields, " and "))
	}
	if violations := ValidateLoopPolicyMetadata(fm.LoopPolicyMetadata); len(violations) > 0 {
		return nil, fmt.Errorf("invalid task loop policy: %s", strings.Join(violations, "; "))
	}
	if _, full := taskIDPrefix(fm.ID); !full || strings.TrimSpace(fm.Title) == "" || strings.TrimSpace(fm.SpecRef) == "" {
		return nil, fmt.Errorf("task id, title, and spec_ref must be valid and non-empty")
	}
	if _, ok := validStatuses[fm.Status]; !ok {
		return nil, fmt.Errorf("invalid task status %q", fm.Status)
	}
	if _, ok := validPriorites[fm.Priority]; !ok {
		return nil, fmt.Errorf("invalid task priority %q", fm.Priority)
	}
	if fm.Dependencies == nil {
		return nil, fmt.Errorf("task dependencies must be non-null")
	}
	if parsed, err := time.Parse(time.RFC3339, fm.UpdatedAt); err != nil || parsed.Format(time.RFC3339) != fm.UpdatedAt || !strings.HasSuffix(fm.UpdatedAt, "Z") {
		return nil, fmt.Errorf("task updated_at must be canonical RFC3339 UTC")
	}
	return &Task{Frontmatter: fm, Body: body}, nil
}

func buildLayout2MigrationCandidate(root string) (*Layout2MigrationCandidate, error) {
	markerData, err := os.ReadFile(markerPath(root))
	if err != nil {
		return nil, fmt.Errorf("read layout marker: %w", fsCause(err))
	}
	sourceMarker, err := decodeLayoutMarkerStrict(markerData)
	if err != nil {
		return nil, err
	}
	if sourceMarker.LayoutVersion != 1 {
		return nil, fmt.Errorf("layout 2 migration candidate requires layout 1 input")
	}
	for field, dir := range map[string]string{"specs_dir": sourceMarker.SpecsDir, "planning_dir": sourceMarker.PlanningDir} {
		if err := ensureWithinRoot(root, field, dir); err != nil {
			return nil, err
		}
	}
	paths := pathsFromLayout(root, LayoutConfig{LayoutVersion: 1, SpecsDir: sourceMarker.SpecsDir, PlanningDir: sourceMarker.PlanningDir}, committedStorage())
	if err := refuseLegacyPolicyPath(root, paths.PlanningDir); err != nil {
		return nil, err
	}

	stateData, err := os.ReadFile(paths.StateFile)
	if err != nil {
		return nil, fmt.Errorf("read state: %w", fsCause(err))
	}
	decodedState, _, err := decodeStateStrict(stateData)
	if err != nil {
		return nil, err
	}
	if err := ensureWithinRoot(root, "active_spec_path", filepath.FromSlash(decodedState.Frontmatter.ActiveSpecPath)); err != nil {
		return nil, err
	}
	if !fileExists(filepath.Join(root, filepath.FromSlash(decodedState.Frontmatter.ActiveSpecPath))) {
		return nil, fmt.Errorf("state active_spec_path does not exist: %s", decodedState.Frontmatter.ActiveSpecPath)
	}

	tasks, taskBytes, err := readMigrationTasks(paths)
	if err != nil {
		return nil, err
	}
	if err := validateMigrationStateTaskLinks(decodedState.Frontmatter, tasks); err != nil {
		return nil, err
	}
	service := &Service{paths: paths}
	validationState := &State{Frontmatter: legacyStateShape(decodedState.Frontmatter)}
	if violations := service.validateTasks(validationState, tasks); len(violations) > 0 {
		return nil, fmt.Errorf("invalid migration source: %s", strings.Join(violations, "; "))
	}

	notesPresent, err := classifyNotesDestination(root, paths.PlanningDir)
	if err != nil {
		return nil, err
	}
	candidate := &Layout2MigrationCandidate{
		Marker:     Layout2Config{LayoutVersion: 2, SpecsDir: sourceMarker.SpecsDir, PlanningDir: sourceMarker.PlanningDir, StorageMode: StorageCommitted, ImplementationReviewMaxRounds: 1},
		MarkerPath: markerRelPath(), StatePath: path.Join(paths.LogicalPlanningDir, "STATE.md"),
		TaskBytes: taskBytes, ContinuationNotes: slices.Clone(decodedState.ContinuationNotes),
		NotesPath: path.Join(paths.LogicalPlanningDir, notesFileName), NotesPresent: notesPresent,
	}
	if !notesPresent && len(decodedState.ContinuationNotes) > 0 {
		candidate.NotesExtractionBytes, err = notesExtractionCandidate(root, paths.PlanningDir, decodedState.ContinuationNotes)
		if err != nil {
			return nil, err
		}
	} else if !notesPresent {
		candidate.NotesTemplateBytes = []byte(starterNotes())
	}
	if candidate.Skills, err = classifyMigrationSkills(root); err != nil {
		return nil, err
	}
	if candidate.MarkerBytes, err = yaml.Marshal(candidate.Marker); err != nil {
		return nil, fmt.Errorf("marshal layout 2 marker: %w", err)
	}
	if _, err := decodeLayoutMarkerStrict(candidate.MarkerBytes); err != nil {
		return nil, fmt.Errorf("validate layout 2 marker candidate: %w", err)
	}
	body := renderStateBodyV2(decodedState.Frontmatter, tasks)
	if candidate.StateBytes, err = marshalFrontmatter(decodedState.Frontmatter, body); err != nil {
		return nil, err
	}
	if _, _, err := decodeStateStrict(candidate.StateBytes); err != nil {
		return nil, fmt.Errorf("validate state candidate: %w", err)
	}
	return candidate, nil
}

func readMigrationTasks(paths Paths) ([]*Task, map[string][]byte, error) {
	entries, err := os.ReadDir(paths.TasksDir)
	if err != nil {
		return nil, nil, fmt.Errorf("read tasks: %w", fsCause(err))
	}
	var tasks []*Task
	bytesByPath := make(map[string][]byte)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		physical := filepath.Join(paths.TasksDir, entry.Name())
		data, err := os.ReadFile(physical)
		if err != nil {
			return nil, nil, fmt.Errorf("read task %s: %w", entry.Name(), fsCause(err))
		}
		task, err := decodeMigrationTaskStrict(data)
		if err != nil {
			return nil, nil, fmt.Errorf("parse task %s: %w", entry.Name(), err)
		}
		if entry.Name() != task.Frontmatter.ID+".md" {
			return nil, nil, fmt.Errorf("task filename %s does not match id %s", entry.Name(), task.Frontmatter.ID)
		}
		task.Filename = physical
		tasks = append(tasks, task)
		logical := path.Join(paths.LogicalPlanningDir, "tasks", entry.Name())
		bytesByPath[logical] = slices.Clone(data)
	}
	slices.SortFunc(tasks, func(a, b *Task) int { return strings.Compare(a.Frontmatter.ID, b.Frontmatter.ID) })
	return tasks, bytesByPath, nil
}

func validateMigrationStateTaskLinks(state stateV2Frontmatter, tasks []*Task) error {
	seen := make(map[string]*Task, len(tasks))
	for _, task := range tasks {
		if seen[task.Frontmatter.ID] != nil {
			return fmt.Errorf("duplicate task id %s", task.Frontmatter.ID)
		}
		seen[task.Frontmatter.ID] = task
	}
	for _, task := range tasks {
		for _, dependency := range task.Frontmatter.Dependencies {
			if seen[dependency] == nil {
				return fmt.Errorf("task %s has unknown dependency %s", task.Frontmatter.ID, dependency)
			}
		}
	}
	if state.CurrentTask != "" && seen[state.CurrentTask] == nil {
		return fmt.Errorf("state current_task %s does not exist", state.CurrentTask)
	}
	if state.LastVerificationID != "" {
		match := canonicalStateVerification.FindStringSubmatch(state.LastVerificationResult)
		task := seen[match[2]]
		if task == nil {
			return fmt.Errorf("state verification task %s does not exist", match[2])
		}
		if state.LastVerifiedCompletionID != "" && (task.Frontmatter.Status != "completed" || task.Frontmatter.CompletionID != state.LastVerifiedCompletionID) {
			return fmt.Errorf("state verified completion does not match the completed task")
		}
	}
	return nil
}

func classifyMigrationSkills(root string) ([]MigrationSkillCandidate, error) {
	files, err := packagedSkillFiles()
	if err != nil {
		return nil, err
	}
	var candidates []MigrationSkillCandidate
	for _, target := range shippableSkillTargets {
		for _, rel := range files {
			packaged, err := shippableSkillsFS.ReadFile(path.Join(shippableSkillsRoot, rel))
			if err != nil {
				return nil, err
			}
			diskPath := filepath.Join(root, target, filepath.FromSlash(rel))
			info, err := os.Lstat(diskPath)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("inspect skill %s: %w", relPath(root, diskPath), fsCause(err))
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("skill destination %s is not a regular file", relPath(root, diskPath))
			}
			data, err := os.ReadFile(diskPath)
			if err != nil {
				return nil, err
			}
			version, marker, err := migrationSkillMarker(data)
			if err != nil {
				return nil, fmt.Errorf("read skill marker %s: %w", relPath(root, diskPath), err)
			}
			outcome := migrationSkillRefresh
			if version == "" {
				if !bytes.Equal(data, packaged) {
					return nil, fmt.Errorf("marker-free skill %s diverges from the packaged copy", relPath(root, diskPath))
				}
				outcome = migrationSkillParity
			}
			candidates = append(candidates, MigrationSkillCandidate{Path: relPath(root, diskPath), Outcome: outcome, Marker: marker, Version: version})
		}
	}
	slices.SortFunc(candidates, func(a, b MigrationSkillCandidate) int { return strings.Compare(a.Path, b.Path) })
	return candidates, nil
}

func migrationSkillMarker(data []byte) (string, string, error) {
	root, _, err := parseSkillDocument(data)
	if err != nil {
		first, _, ok := nextSkillLine(data, 0)
		if !ok || string(first) != "---" {
			return "", "none", nil
		}
		return "", "", err
	}
	version, err := skillVersionFromRoot(root)
	if err != nil {
		return "", "", err
	}
	mapping := root.Content[0]
	legacy, err := uniqueMappingValue(mapping, legacySkillVersionKey)
	if err != nil {
		return "", "", err
	}
	var nested *yaml.Node
	metadata, err := uniqueMappingValue(mapping, "metadata")
	if err != nil {
		return "", "", err
	}
	if metadata != nil {
		if metadata.Kind != yaml.MappingNode {
			return "", "", fmt.Errorf("metadata must be a mapping")
		}
		nested, err = uniqueMappingValue(metadata, skillVersionKey)
		if err != nil {
			return "", "", err
		}
	}
	switch {
	case legacy != nil && nested != nil:
		return version, "dual", nil
	case legacy != nil:
		return version, "legacy", nil
	case nested != nil:
		return version, "nested", nil
	default:
		return version, "none", nil
	}
}

func refuseLegacyPolicyPath(root, planningDir string) error {
	legacy := filepath.Join(planningDir, "AUTONOMY.tsv")
	_, err := os.Lstat(legacy)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect unsupported legacy input %s: %w", relPath(root, legacy), fsCause(err))
	}
	return fmt.Errorf("unsupported legacy input %s: remove it and record intended policy with taskrail task loop", relPath(root, legacy))
}

func renderStateBodyV2(state stateV2Frontmatter, tasks []*Task) string {
	body := renderStateBody(legacyStateShape(state), tasks)
	start := strings.Index(body, "## Notes\n")
	if start < 0 {
		return body
	}
	end := strings.Index(body[start:], "## Task Counts\n")
	if end < 0 {
		return strings.TrimRight(body[:start], "\n") + "\n"
	}
	return body[:start] + body[start+end:]
}

func legacyStateShape(state stateV2Frontmatter) StateFrontmatter {
	return StateFrontmatter{
		SchemaVersion: 1, UpdatedAt: state.UpdatedAt, ActiveSpecVersion: state.ActiveSpecVersion,
		ActiveSpecPath: state.ActiveSpecPath, CurrentTask: state.CurrentTask,
		CurrentTaskTitle: state.CurrentTaskTitle, StatusSummary: state.StatusSummary,
		Blockers: state.Blockers, NextAction: state.NextAction,
		LastVerificationResult: state.LastVerificationResult, RelevantArtifacts: state.RelevantArtifacts,
	}
}

func splitFrontmatter(data []byte) ([]byte, string, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, "", fmt.Errorf("missing frontmatter start")
	}
	parts := strings.SplitN(strings.TrimPrefix(text, "---\n"), "\n---\n", 2)
	if len(parts) != 2 {
		return nil, "", fmt.Errorf("missing frontmatter end")
	}
	return []byte(parts[0]), strings.TrimLeft(parts[1], "\n"), nil
}

func strictYAMLMapping(data []byte) (map[string]bool, *yaml.Node, error) {
	var document yaml.Node
	if err := strictYAMLDecode(data, &document); err != nil {
		return nil, nil, err
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("document must be a mapping")
	}
	mapping := document.Content[0]
	keys := make(map[string]bool, len(mapping.Content)/2)
	for i := 0; i < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
			return nil, nil, fmt.Errorf("field names must be strings")
		}
		if keys[key.Value] {
			return nil, nil, fmt.Errorf("duplicate field %q", key.Value)
		}
		keys[key.Value] = true
	}
	return keys, mapping, nil
}

func strictYAMLDecode(data []byte, out any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple YAML documents are not allowed")
		}
		return err
	}
	return nil
}

func validateStateNodeTypes(mapping *yaml.Node) error {
	for _, key := range []string{"updated_at", "active_spec_version", "active_spec_path", "current_task", "current_task_title", "status_summary", "next_action", "last_verification_result", "last_verification_id", "last_verification_previous_id", "last_verified_completion_id"} {
		if value := mappingValue(mapping, key); value != nil && (value.Kind != yaml.ScalarNode || value.Tag != "!!str") {
			return fmt.Errorf("state %s must be a string", key)
		}
	}
	for _, key := range []string{"blockers", "relevant_artifacts", "continuation_notes"} {
		if value := mappingValue(mapping, key); value != nil {
			if value.Kind != yaml.SequenceNode {
				return fmt.Errorf("state %s must be a sequence", key)
			}
			for _, item := range value.Content {
				if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
					return fmt.Errorf("state %s entries must be strings", key)
				}
			}
		}
	}
	return nil
}

func validateTaskNodeTypes(mapping *yaml.Node) error {
	for _, key := range []string{"id", "title", "status", "priority", "spec_ref", "updated_at", "loop_policy", "loop_reason", "completion_id", "last_verification_id", "last_verification_previous_id", "last_verification_result", "last_verified_at", "last_verified_completion_id"} {
		if value := mappingValue(mapping, key); value != nil && (value.Kind != yaml.ScalarNode || value.Tag != "!!str") {
			return fmt.Errorf("task %s must be a string", key)
		}
	}
	dependencies := mappingValue(mapping, "dependencies")
	if dependencies == nil || dependencies.Kind != yaml.SequenceNode {
		return fmt.Errorf("task dependencies must be a sequence")
	}
	for _, dependency := range dependencies.Content {
		if dependency.Kind != yaml.ScalarNode || dependency.Tag != "!!str" {
			return fmt.Errorf("task dependencies must contain strings")
		}
	}
	return nil
}

func requireExactKeys(keys map[string]bool, required, optional []string) error {
	allowed := make(map[string]bool, len(required)+len(optional))
	for _, key := range required {
		allowed[key] = true
		if !keys[key] {
			return fmt.Errorf("missing required field %q", key)
		}
	}
	for _, key := range optional {
		allowed[key] = true
	}
	for key := range keys {
		if !allowed[key] {
			return fmt.Errorf("unknown field %q", key)
		}
	}
	return nil
}

func hasMarkdownHeading(body, heading string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "## "+heading {
			return true
		}
	}
	return false
}
