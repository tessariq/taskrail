package taskrail

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Agent-driven apply (T-034): `taskrail import --apply <draft.json>` ingests an
// ImportDraft an agent produced from the emit-prompt output. It validates the
// draft against the T-032 schema, writes any spec sections to a new spec file,
// and scaffolds each task through CreateTask (T-027) so drafts and hand-created
// tasks share one validation and id-allocation path. The binary makes no LLM
// call; the semantic work already happened in the agent.

// ApplyDraftInput names the draft file to ingest (repo-relative or absolute).
type ApplyDraftInput struct {
	DraftPath string
}

// CreatedTaskRef records one task the apply scaffolded, pairing the draft-local
// key with the allocated real task id and file path.
type CreatedTaskRef struct {
	Key    string `json:"key,omitempty"`
	TaskID string `json:"task_id"`
	Path   string `json:"path"`
}

// ApplyDraftResult reports what apply wrote: an optional spec file and the tasks
// it created, in dependency order.
type ApplyDraftResult struct {
	Target   string           `json:"target"`
	SpecPath string           `json:"spec_path,omitempty"`
	Tasks    []CreatedTaskRef `json:"tasks,omitempty"`
}

// ApplyImportDraft validates a draft and writes real spec/task files. Structural
// validation rejects malformed drafts, then a live pre-flight (T-041) runs every
// task's repo checks — spec heading existence and external dependency existence —
// resolving spec_ref anchors against both existing spec files and the draft's own
// pending spec sections. Because that pre-flight writes nothing, a draft that
// would fail any live check leaves the repository unchanged: no orphan spec, no
// partial tasks. describeWrittenArtifacts still guards the residual I/O-failure
// path so a mid-write disk error is never silent.
func (s *Service) ApplyImportDraft(input ApplyDraftInput) (ApplyDraftResult, error) {
	draft, err := s.readImportDraft(input.DraftPath)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	if violations := ValidateImportDraft(draft); len(violations) > 0 {
		return ApplyDraftResult{}, fmt.Errorf("import draft is invalid: %s", strings.Join(violations, "; "))
	}
	if err := s.preflightImportDraft(draft); err != nil {
		return ApplyDraftResult{}, err
	}

	result := ApplyDraftResult{Target: draft.Target}
	if len(draft.SpecSections) > 0 {
		specPath, err := s.writeImportedSpec(draft)
		if err != nil {
			return ApplyDraftResult{}, err
		}
		result.SpecPath = specPath
	}

	created, err := s.createDraftTasks(draft.Tasks)
	result.Tasks = created
	if err != nil {
		if written := describeWrittenArtifacts(result); written != "" {
			return result, fmt.Errorf("%w; partial apply already wrote %s — review before retrying", err, written)
		}
		return result, err
	}
	return result, nil
}

// pendingSpec captures the spec an apply is about to write: its repo-relative
// path and the heading anchors it will expose. Pre-flight consults it so a task
// may legitimately reference a heading in the not-yet-written imported spec.
type pendingSpec struct {
	path    string
	anchors map[string]struct{}
}

// importedSpecPath is the absolute path `import --apply` writes a draft's spec
// to. Pre-flight and writeImportedSpec both derive it here so they can never
// disagree on where the imported spec lands.
func (s *Service) importedSpecPath(draft ImportDraft) string {
	return filepath.Join(s.paths.SpecsDir, specStemFromSource(draft.Source)+".md")
}

// buildPendingSpec derives the pending imported spec from a draft, or nil when
// the draft writes no spec. Anchors are collected from the exact markdown
// writeImportedSpec will render, so pre-flight and apply agree on what exists.
func (s *Service) buildPendingSpec(draft ImportDraft) *pendingSpec {
	if len(draft.SpecSections) == 0 {
		return nil
	}
	return &pendingSpec{
		path:    relPath(s.paths.RepoRoot, s.importedSpecPath(draft)),
		anchors: collectHeadingAnchors(renderImportedSpec(draft)),
	}
}

// preflightImportDraft runs every live-repo check apply would otherwise hit only
// after writing: the shared validateTaskCreatable per task (spec heading resolved
// against the pending imported spec, priority, dependency existence) plus the
// dependency-cycle check task ordering performs. In-draft key dependencies are
// accepted here because a sibling task will create them. Nothing is written, so
// any failure leaves the repository unchanged — no orphan spec, no partial tasks.
func (s *Service) preflightImportDraft(draft ImportDraft) error {
	_, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return err
	}
	pending := s.buildPendingSpec(draft)
	keys, _ := draftTaskKeys(draft.Tasks)
	opts := taskValidationOpts{pending: pending, draftKeys: keys}
	for i, task := range draft.Tasks {
		if _, err := s.validateTaskCreatable(tasks, task.SpecRef, task.Priority, task.Dependencies, opts); err != nil {
			return fmt.Errorf("%s: %w", taskDraftLabel(task, i), err)
		}
	}
	// Ordering detects a dependency cycle among draft keys; run it before any
	// write so a cyclic draft with spec sections cannot leave an orphan spec.
	if _, err := orderTaskDraftsByDeps(draft.Tasks); err != nil {
		return err
	}
	return nil
}

// validateSpecRefWithPending is the live spec_ref check CreateTask performs,
// extended to resolve a reference to the pending imported spec against that
// spec's about-to-be-written headings instead of the on-disk file (which may not
// exist yet, or may be a stale orphan the apply will overwrite).
func (s *Service) validateSpecRefWithPending(specRef string, pending *pendingSpec) error {
	if strings.TrimSpace(specRef) == "" {
		return errors.New("task spec_ref must not be empty")
	}
	pathPart, anchor, err := parseSpecRef(specRef)
	if err != nil {
		return err
	}
	// pending.path is slash-normalized (relPath applies filepath.ToSlash); pathPart
	// is OS-native from parseSpecRef, so normalize it before comparing on Windows.
	if pending != nil && filepath.ToSlash(pathPart) == pending.path {
		if _, ok := pending.anchors[anchor]; !ok {
			return fmt.Errorf("heading #%s not found in %s (pending import)", anchor, pathPart)
		}
		return nil
	}
	return s.validateSpecRef(specRef)
}

// describeWrittenArtifacts summarizes what an apply landed on disk, for surfacing
// partial state in a failure message. It returns "" when nothing was written.
func describeWrittenArtifacts(result ApplyDraftResult) string {
	parts := make([]string, 0, 2)
	if result.SpecPath != "" {
		parts = append(parts, result.SpecPath)
	}
	if n := len(result.Tasks); n > 0 {
		ids := make([]string, 0, n)
		for _, task := range result.Tasks {
			ids = append(ids, task.TaskID)
		}
		parts = append(parts, "tasks "+strings.Join(ids, ", "))
	}
	return strings.Join(parts, " and ")
}

// readImportDraft loads and parses a draft file, resolving a relative path
// against the repo root. This is a read; an absolute path is honored as given.
func (s *Service) readImportDraft(path string) (ImportDraft, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return ImportDraft{}, errors.New("import draft path must not be empty")
	}
	data, err := os.ReadFile(s.resolveRepoPath(p))
	if err != nil {
		return ImportDraft{}, fmt.Errorf("read import draft: %w", err)
	}
	return ParseImportDraft(data)
}

// createDraftTasks scaffolds each task draft through CreateTask in dependency
// order, translating in-draft key dependencies to the real ids CreateTask
// allocates as it goes.
func (s *Service) createDraftTasks(tasks []TaskDraft) ([]CreatedTaskRef, error) {
	if len(tasks) == 0 {
		return nil, nil
	}
	order, err := orderTaskDraftsByDeps(tasks)
	if err != nil {
		return nil, err
	}

	keyToID := make(map[string]string, len(tasks))
	created := make([]CreatedTaskRef, 0, len(tasks))
	for _, idx := range order {
		draft := tasks[idx]
		res, err := s.CreateTask(CreateTaskInput{
			Title:        draft.Title,
			SpecRef:      draft.SpecRef,
			Priority:     draft.Priority,
			Dependencies: translateDeps(draft.Dependencies, keyToID),
		})
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", taskDraftLabel(draft, idx), err)
		}
		if draft.Key != "" {
			keyToID[draft.Key] = res.TaskID
		}
		created = append(created, CreatedTaskRef{Key: draft.Key, TaskID: res.TaskID, Path: res.Path})
	}
	return created, nil
}

// translateDeps rewrites in-draft key dependencies to their allocated task ids.
// A dependency that is not a known key is an existing task id and passes through;
// CreateTask confirms it exists.
func translateDeps(deps []string, keyToID map[string]string) []string {
	if len(deps) == 0 {
		return nil
	}
	out := make([]string, 0, len(deps))
	for _, dep := range deps {
		if id, ok := keyToID[dep]; ok {
			dep = id
		}
		out = append(out, dep)
	}
	return out
}

// orderTaskDraftsByDeps returns task indices in an order where every in-draft
// dependency precedes its dependent. Only in-draft key edges constrain order;
// external task ids are already-created and impose none. A cycle is an error.
func orderTaskDraftsByDeps(tasks []TaskDraft) ([]int, error) {
	// Precondition: keys are unique (ValidateImportDraft enforces this before any
	// apply reaches here). With unique keys this map has one entry per keyed task.
	keyToIdx := make(map[string]int, len(tasks))
	for i, task := range tasks {
		if task.Key != "" {
			keyToIdx[task.Key] = i
		}
	}

	indegree := make([]int, len(tasks))
	dependents := make([][]int, len(tasks))
	for i, task := range tasks {
		for _, dep := range task.Dependencies {
			j, ok := keyToIdx[dep]
			if !ok {
				continue // external task id: no ordering constraint
			}
			dependents[j] = append(dependents[j], i)
			indegree[i]++
		}
	}

	order := make([]int, 0, len(tasks))
	done := make([]bool, len(tasks))
	for len(order) < len(tasks) {
		// Pick the lowest-index ready task each round to keep the order stable.
		next := -1
		for i := 0; i < len(tasks); i++ {
			if !done[i] && indegree[i] == 0 {
				next = i
				break
			}
		}
		if next == -1 {
			return nil, errors.New("import draft has a dependency cycle among draft keys")
		}
		done[next] = true
		order = append(order, next)
		for _, d := range dependents[next] {
			indegree[d]--
		}
	}
	return order, nil
}

// importedSpecMarker tags every spec file `import --apply` writes. Its presence
// distinguishes an orphan left by a prior import (safe to overwrite on retry)
// from an authored spec (never clobbered).
const importedSpecMarker = "Imported by `taskrail import --apply`. Review before adopting."

// writeImportedSpec assembles the draft's spec sections into a new spec file. It
// never clobbers an authored spec, mirroring the non-destructive Init contract,
// but overwrites an orphan a prior import left at the same path so a corrected
// re-apply can succeed (T-041).
func (s *Service) writeImportedSpec(draft ImportDraft) (string, error) {
	specPath := s.importedSpecPath(draft)
	if fileExists(specPath) && !isImportedSpec(specPath) {
		return "", fmt.Errorf("spec file %s already exists; refusing to overwrite", relPath(s.paths.RepoRoot, specPath))
	}
	if err := ensureDir(filepath.Dir(specPath)); err != nil {
		return "", err
	}
	if err := os.WriteFile(specPath, []byte(renderImportedSpec(draft)), 0o644); err != nil {
		return "", fmt.Errorf("write imported spec: %w", err)
	}
	return relPath(s.paths.RepoRoot, specPath), nil
}

// isImportedSpec reports whether the file at path was written by a prior
// `import --apply` (carries importedSpecMarker). An unreadable file is treated as
// not-imported so writeImportedSpec falls back to its refuse-to-overwrite guard.
func isImportedSpec(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), importedSpecMarker)
}

// specStemFromSource derives a safe spec filename stem from the draft source,
// preserving dots so version-like names stay intact. It falls back to a fixed
// name when the source yields nothing usable.
func specStemFromSource(source string) string {
	base := filepath.Base(strings.TrimSpace(source))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	stem := strings.Trim(b.String(), "-.")
	if stem == "" {
		return "imported-spec"
	}
	return stem
}

// renderImportedSpec renders a reviewable spec markdown from the draft sections.
func renderImportedSpec(draft ImportDraft) string {
	title := strings.TrimSpace(draft.Source)
	if title == "" {
		title = "Imported Spec"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	b.WriteString(importedSpecMarker + "\n\n")
	for _, section := range draft.SpecSections {
		fmt.Fprintf(&b, "## %s\n\n", strings.TrimSpace(section.Heading))
		if body := strings.TrimSpace(section.Body); body != "" {
			fmt.Fprintf(&b, "%s\n\n", body)
		}
	}
	return b.String()
}
