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
// validation rejects malformed drafts before anything is written; runtime repo
// checks that CreateTask performs (spec heading existence, external dependency
// existence) can still fail after a spec section was written or an earlier task
// created. On such a failure the returned result reports what already landed and
// the error names it, so partial state is never silent.
func (s *Service) ApplyImportDraft(input ApplyDraftInput) (ApplyDraftResult, error) {
	draft, err := s.readImportDraft(input.DraftPath)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	if violations := ValidateImportDraft(draft); len(violations) > 0 {
		return ApplyDraftResult{}, fmt.Errorf("import draft is invalid: %s", strings.Join(violations, "; "))
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

// writeImportedSpec assembles the draft's spec sections into a new spec file. It
// refuses to overwrite an existing file so an import never clobbers authored
// specs, mirroring the non-destructive Init contract.
func (s *Service) writeImportedSpec(draft ImportDraft) (string, error) {
	specPath := filepath.Join(s.paths.SpecsDir, specStemFromSource(draft.Source)+".md")
	if fileExists(specPath) {
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
	b.WriteString("Imported by `taskrail import --apply`. Review before adopting.\n\n")
	for _, section := range draft.SpecSections {
		fmt.Fprintf(&b, "## %s\n\n", strings.TrimSpace(section.Heading))
		if body := strings.TrimSpace(section.Body); body != "" {
			fmt.Fprintf(&b, "%s\n\n", body)
		}
	}
	return b.String()
}
