package taskrail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// importDraftSchemaVersion versions the structured draft contract that the
// agent-driven import path (T-034) emits and `taskrail import --apply` ingests.
// Bump it only on an incompatible change to the draft shape so older drafts fail
// loudly rather than ingesting under the wrong assumptions.
const importDraftSchemaVersion = 1

// Target is a validated `taskrail import --to` destination. The binary performs
// no LLM calls; an agent produces the draft and these targets describe only where
// an applied draft lands. parseTarget is the single gate that admits a target, so
// callers route on the constants instead of re-checking a raw string.
type Target string

const (
	TargetTasks    Target = "tasks"
	TargetSpec     Target = "spec"
	TargetPlanning Target = "planning"
)

// importTargets is the canonical, ordered set of valid import targets. Membership
// checks and the error-message enumeration both derive from it, so a new target is
// added in exactly one place and no message can drift out of sync with the set.
var importTargets = []Target{TargetTasks, TargetSpec, TargetPlanning}

func (t Target) valid() bool {
	return slices.Contains(importTargets, t)
}

// importTargetList renders the valid targets for error messages so the message
// and the accepted set stay in lockstep.
func importTargetList() string {
	names := make([]string, len(importTargets))
	for i, t := range importTargets {
		names[i] = string(t)
	}
	return strings.Join(names, ", ")
}

// parseTarget trims and validates a raw import target, returning the canonical
// Target. It is the one place that decides which targets exist; Import and
// EmitImportPrompt delegate here rather than re-checking membership themselves.
func parseTarget(raw string) (Target, error) {
	t := Target(strings.TrimSpace(raw))
	if !t.valid() {
		return "", fmt.Errorf("import target must be one of %s; got %q", importTargetList(), string(t))
	}
	return t, nil
}

// taskIDPattern matches an already-tracked task id (for example `T-027`). A draft
// dependency may reference such an id; its existence is a deferred apply-time
// check, not a draft-structural one, since the referenced task lives in the repo.
var taskIDPattern = regexp.MustCompile(`^T-\d+$`)

// ImportDraft is the versioned envelope an agent emits from source material. It
// round-trips through `--emit-prompt` / `--apply` (T-034); `--apply` validates it
// structurally here, then ingests each task draft through CreateTask so drafts and
// real tasks share one validation path (T-027) rather than diverging.
type ImportDraft struct {
	SchemaVersion int                `json:"schema_version"`
	Target        string             `json:"target"`
	Source        string             `json:"source,omitempty"`
	Tasks         []TaskDraft        `json:"tasks,omitempty"`
	SpecSections  []SpecSectionDraft `json:"spec_sections,omitempty"`
}

// TaskDraft maps onto the T-027 task fields CreateTask consumes. Key is a draft-
// local handle so one draft task can depend on another before either has a real
// task id; it is not persisted on the scaffolded task.
type TaskDraft struct {
	Key          string   `json:"key,omitempty"`
	Title        string   `json:"title"`
	SpecRef      string   `json:"spec_ref,omitempty"`
	Priority     string   `json:"priority,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Body         string   `json:"body,omitempty"`
}

// SpecSectionDraft is a proposed spec heading and body for `--to spec|planning`.
type SpecSectionDraft struct {
	Heading string `json:"heading"`
	Body    string `json:"body,omitempty"`
}

// ParseImportDraft decodes a structured import draft, rejecting unknown fields so
// a malformed agent emission fails fast instead of silently dropping data.
func ParseImportDraft(data []byte) (ImportDraft, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var draft ImportDraft
	if err := dec.Decode(&draft); err != nil {
		return ImportDraft{}, fmt.Errorf("parse import draft: %w", err)
	}
	if dec.More() {
		return ImportDraft{}, fmt.Errorf("parse import draft: unexpected trailing content after the draft object")
	}
	return draft, nil
}

// ValidateImportDraft applies the draft-structural rules: schema version, target,
// non-empty payload, and per-task field shape. It deliberately does not touch the
// filesystem; spec-file existence and cross-repo dependency existence are reused
// from T-027's CreateTask at apply time, when the spec and tasks actually exist.
func ValidateImportDraft(draft ImportDraft) []string {
	violations := make([]string, 0)
	if draft.SchemaVersion != importDraftSchemaVersion {
		violations = append(violations, fmt.Sprintf("import draft schema_version must be %d", importDraftSchemaVersion))
	}
	if !Target(draft.Target).valid() {
		violations = append(violations, fmt.Sprintf("import draft target must be one of %s; got %q", importTargetList(), draft.Target))
	}
	if len(draft.Tasks) == 0 && len(draft.SpecSections) == 0 {
		violations = append(violations, "import draft must contain at least one task or spec section")
	}

	keys, keyViolations := draftTaskKeys(draft.Tasks)
	violations = append(violations, keyViolations...)
	for i, task := range draft.Tasks {
		violations = append(violations, validateTaskDraft(task, i, keys)...)
	}
	for i, section := range draft.SpecSections {
		if strings.TrimSpace(section.Heading) == "" {
			violations = append(violations, fmt.Sprintf("spec section #%d missing heading", i+1))
		}
	}
	return violations
}

// draftTaskKeys collects the set of unique draft-local keys, returning any
// duplicate-key violations so an intra-draft dependency reference can be resolved
// against a unique handle.
func draftTaskKeys(tasks []TaskDraft) (map[string]struct{}, []string) {
	keys := make(map[string]struct{}, len(tasks))
	violations := make([]string, 0)
	for _, task := range tasks {
		if strings.TrimSpace(task.Key) == "" {
			continue
		}
		if _, dup := keys[task.Key]; dup {
			violations = append(violations, fmt.Sprintf("duplicate task draft key %q", task.Key))
			continue
		}
		keys[task.Key] = struct{}{}
	}
	return keys, violations
}

func validateTaskDraft(task TaskDraft, index int, keys map[string]struct{}) []string {
	violations := make([]string, 0)
	label := taskDraftLabel(task, index)
	if strings.TrimSpace(task.Title) == "" {
		violations = append(violations, fmt.Sprintf("%s missing title", label))
	}
	if task.Priority != "" {
		if _, ok := validPriorites[task.Priority]; !ok {
			violations = append(violations, fmt.Sprintf("%s has invalid priority %q", label, task.Priority))
		}
	}
	if task.SpecRef != "" {
		if _, _, err := parseSpecRef(task.SpecRef); err != nil {
			violations = append(violations, fmt.Sprintf("%s invalid spec_ref: %v", label, err))
		}
	}
	for _, dep := range task.Dependencies {
		if strings.TrimSpace(task.Key) != "" && dep == task.Key {
			violations = append(violations, fmt.Sprintf("%s cannot depend on itself", label))
			continue
		}
		if !dependencyResolvable(dep, keys) {
			violations = append(violations, fmt.Sprintf("%s has unresolved dependency %q (expect an in-draft key or existing task id)", label, dep))
		}
	}
	return violations
}

// dependencyResolvable accepts an in-draft key or an existing task id pattern. The
// latter is confirmed against the repo at apply time, not here.
func dependencyResolvable(dep string, keys map[string]struct{}) bool {
	if dep == "" {
		return false
	}
	if _, ok := keys[dep]; ok {
		return true
	}
	return taskIDPattern.MatchString(dep)
}

func taskDraftLabel(task TaskDraft, index int) string {
	if task.Key != "" {
		return fmt.Sprintf("task draft %q", task.Key)
	}
	if title := strings.TrimSpace(task.Title); title != "" {
		return fmt.Sprintf("task draft %q", title)
	}
	return fmt.Sprintf("task draft #%d", index+1)
}
