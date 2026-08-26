package taskrail

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tessariq/taskrail/internal/repotx"
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
	Warnings []Warning        `json:"-"`
}

// ApplyImportDraft publishes a fully validated v1 import through one normal
// transaction. The source draft, existing corpus, config, and every destination
// are snapshotted under the mutation lock before any candidate reaches disk.
func (s *Service) ApplyImportDraft(input ApplyDraftInput) (result ApplyDraftResult, err error) {
	own, release, err := s.acquireWriterLock("import", nil)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil && err == nil {
			err = releaseErr
		}
	}()

	draft, data, draftPath, err := s.readImportDraftSnapshot(input.DraftPath)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	if violations := ValidateImportDraft(draft); len(violations) > 0 {
		return ApplyDraftResult{}, WithMachineErrorCode(MachineCodeInvalidProposal,
			fmt.Errorf("import draft is invalid: %s", strings.Join(violations, "; ")))
	}
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return ApplyDraftResult{}, err
	}
	corpus, err := snapshotTaskCorpus(tasks)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	baseline := s.validateInMemory(state, tasks)
	if err := s.preflightImportDraft(tasks, draft); err != nil {
		// Pre-flight rejects the agent-supplied draft itself, so it is an invalid
		// proposal rather than an invalid CLI argument.
		return ApplyDraftResult{}, WithMachineErrorCode(MachineCodeInvalidProposal, err)
	}

	result, published, preview, err := s.buildImportCandidate(draft, state, tasks)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	stateBytes, err := marshalFrontmatter(state.Frontmatter, state.Body)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	published = append([]repotx.Candidate{managedCandidate(s.reportedStatePath(), s.paths.StateFile, stateBytes)}, published...)
	// Include specs referenced only by the incoming tasks, too. The common task
	// writer helper skips the newly proposed task files while retaining every
	// source spec the candidate consulted. A spec candidate itself is already a
	// published path, so it must not also enter the consumed set.
	consumed, err := writerConsumedPaths(s.paths, preview, preview[len(tasks):]...)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	publishedPaths := make(map[string]struct{}, len(published))
	for _, candidate := range published {
		publishedPaths[candidate.Reported] = struct{}{}
	}
	filtered := consumed[:0]
	for _, path := range consumed {
		if _, published := publishedPaths[path.Reported]; !published {
			filtered = append(filtered, path)
		}
	}
	consumed = filtered
	draftInRepository := pathWithinRepository(s.paths.RepoRoot, draftPath)
	if draftInRepository {
		consumed = append(consumed, repotx.Path{Kind: repotx.Worktree, Reported: relPath(s.paths.RepoRoot, draftPath), Physical: draftPath})
	}
	observedPaths := make([]string, 0, len(consumed)+len(published)+1)
	for _, path := range consumed {
		observedPaths = append(observedPaths, path.Physical)
	}
	for _, candidate := range published {
		observedPaths = append(observedPaths, candidate.Physical)
	}
	if !draftInRepository {
		observedPaths = append(observedPaths, draftPath)
	}
	inputs, err := snapshotImportInputs(observedPaths)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	if testHookImportCandidateBuilt != nil {
		testHookImportCandidateBuilt()
	}

	request := repotx.Request{
		Command:   "import",
		Consumed:  consumed,
		Published: published,
		Validate: func([]repotx.Snapshot) error {
			if testHookWriterValidated != nil {
				testHookWriterValidated()
			}
			if !sameImportInputs(inputs) {
				return errImportInputChanged
			}
			if !draftInRepository {
				current, readErr := os.ReadFile(draftPath)
				if readErr != nil || !slices.Equal(current, data) {
					return errImportInputChanged
				}
			}
			currentTasks, loadErr := s.loadTasks()
			if loadErr != nil {
				return loadErr
			}
			if !sameTaskCorpus(corpus, currentTasks) {
				return fmt.Errorf("import task corpus changed during candidate validation")
			}
			// preflightImportDraft validates every new task and its pending-spec
			// anchor; validateInMemory cannot see a spec candidate before publish.
			if candidateIntroducesViolations(ValidationResult{Violations: s.validateState(state)}, baseline) {
				return errors.New("import candidate failed validation")
			}
			return nil
		},
	}
	if testHookWriterCandidateBuilt != nil {
		testHookWriterCandidateBuilt()
	}
	if _, err := repotx.Commit(context.Background(), own, request); err != nil {
		if errors.Is(err, errImportInputChanged) {
			return ApplyDraftResult{}, importInputConflict(err)
		}
		return ApplyDraftResult{}, writerTransactionError(err)
	}
	return result, nil
}

func importInputConflict(err error) error {
	failure := MachineFailure{Code: MachineCodeWriteConflict}
	var txErr *repotx.Error
	if errors.As(err, &txErr) {
		for _, snapshot := range txErr.Snapshots() {
			failure.Snapshots = append(failure.Snapshots, MachineSnapshot{
				PathKind: string(snapshot.Kind), Path: snapshot.Path,
				OriginalSHA256: snapshot.OriginalSHA256, CandidateSHA256: snapshot.CandidateSHA256,
				CurrentSHA256: snapshot.CurrentSHA256,
			})
		}
	}
	return WithMachineFailure(failure, err)
}

var (
	errImportInputChanged        = errors.New("import input changed during candidate validation")
	testHookImportCandidateBuilt func()
)

// importInput records the exact pre-candidate filesystem identity. repotx takes
// its own snapshot before commit; this extra comparison closes the interval
// between candidate construction and that transaction snapshot.
type importInput struct {
	path    string
	exists  bool
	mode    fs.FileMode
	content []byte
}

func snapshotImportInputs(paths []string) ([]importInput, error) {
	seen := make(map[string]struct{}, len(paths))
	inputs := make([]importInput, 0, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		input := importInput{path: path}
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			inputs = append(inputs, input)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect import input: %w", fsCause(err))
		}
		input.exists, input.mode = true, info.Mode()
		if info.Mode().IsRegular() {
			input.content, err = os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read import input: %w", fsCause(err))
			}
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}

func sameImportInputs(inputs []importInput) bool {
	for _, expected := range inputs {
		info, err := os.Lstat(expected.path)
		if errors.Is(err, os.ErrNotExist) {
			if expected.exists {
				return false
			}
			continue
		}
		if err != nil || !expected.exists || info.Mode() != expected.mode {
			return false
		}
		if info.Mode().IsRegular() {
			content, readErr := os.ReadFile(expected.path)
			if readErr != nil || !slices.Equal(content, expected.content) {
				return false
			}
		}
	}
	return true
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
		path:    s.paths.logicalManagedPath(s.importedSpecPath(draft)),
		anchors: collectHeadingAnchors(renderImportedSpec(draft)),
	}
}

// preflightImportDraft runs every live-repo check apply would otherwise hit only
// after writing: the shared validateTaskCreatable per task (spec heading resolved
// against the pending imported spec, priority, dependency existence) plus the
// dependency-cycle check task ordering performs. In-draft key dependencies are
// accepted here because a sibling task will create them. Nothing is written, so
// any failure leaves the repository unchanged — no orphan spec, no partial tasks.
func (s *Service) preflightImportDraft(tasks []*Task, draft ImportDraft) error {
	pending := s.buildPendingSpec(draft)
	keys, _ := draftTaskKeys(draft.Tasks)
	opts := taskValidationOpts{pending: pending, draftKeys: keys}
	for i, task := range draft.Tasks {
		if _, _, err := s.validateTaskCreatable(tasks, task.SpecRef, task.Priority, task.Dependencies, opts); err != nil {
			return fmt.Errorf("%s: %w", taskDraftLabel(task, i), err)
		}
		// Mirror CreateTask's title-portability guard here too: without it a draft
		// whose later task carries an unportable title would only fail mid-loop,
		// after the earlier tasks were already written.
		if err := ensureTaskTitle(strings.TrimSpace(task.Title)); err != nil {
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

// buildImportCandidate preserves v1's task scaffolding and allocation semantics
// while keeping every proposed byte in memory until the transaction commits.
func (s *Service) buildImportCandidate(draft ImportDraft, state *State, tasks []*Task) (ApplyDraftResult, []repotx.Candidate, []*Task, error) {
	result := ApplyDraftResult{Target: draft.Target}
	published := make([]repotx.Candidate, 0, len(draft.Tasks)+1)
	if len(draft.SpecSections) > 0 {
		specPath := s.importedSpecPath(draft)
		logicalSpecPath := s.paths.logicalManagedPath(specPath)
		if fileExists(specPath) && !isImportedSpec(specPath) {
			return ApplyDraftResult{}, nil, nil, WithMachineErrorCode(MachineCodeDestinationExists,
				fmt.Errorf("spec file %s already exists; refusing to overwrite", logicalSpecPath))
		}
		result.SpecPath = logicalSpecPath
		published = append(published, managedCandidate(result.SpecPath, specPath, []byte(renderImportedSpec(draft))))
	}

	order, err := orderTaskDraftsByDeps(draft.Tasks)
	if err != nil {
		return ApplyDraftResult{}, nil, nil, err
	}
	preview := slices.Clone(tasks)
	keyToID := make(map[string]string, len(draft.Tasks))
	for _, idx := range order {
		draftTask := draft.Tasks[idx]
		specRef, priority, err := s.validateTaskCreatable(preview, draftTask.SpecRef, draftTask.Priority,
			translateDeps(draftTask.Dependencies, keyToID), taskValidationOpts{pending: s.buildPendingSpec(draft)})
		if err != nil {
			return ApplyDraftResult{}, nil, nil, fmt.Errorf("%s: %w", taskDraftLabel(draftTask, idx), err)
		}
		id, warnings := nextTaskIDWithSlug(preview, draftTask.Title, false, true)
		now := timestamp(s.now())
		candidate := &Task{Frontmatter: TaskFrontmatter{ID: id, Title: strings.TrimSpace(draftTask.Title), Status: "todo", Priority: priority, SpecRef: specRef, Dependencies: translateDeps(draftTask.Dependencies, keyToID), UpdatedAt: now}, Body: renderNewTaskBody(id, strings.TrimSpace(draftTask.Title), ""), Path: s.paths.logicalManagedPath(filepath.Join(s.paths.TasksDir, id+".md")), Filename: filepath.Join(s.paths.TasksDir, id+".md")}
		bytes, err := marshalFrontmatter(candidate.Frontmatter, candidate.Body)
		if err != nil {
			return ApplyDraftResult{}, nil, nil, err
		}
		if draftTask.Key != "" {
			keyToID[draftTask.Key] = id
		}
		preview = append(preview, candidate)
		result.Tasks = append(result.Tasks, CreatedTaskRef{Key: draftTask.Key, TaskID: id, Path: candidate.Path})
		result.Warnings = append(result.Warnings, warnings...)
		published = append(published, managedCandidate(candidate.Path, candidate.Filename, bytes))
	}
	state.Frontmatter.UpdatedAt = timestamp(s.now())
	state.Body = renderStateBody(state.Frontmatter, preview)
	return result, published, preview, nil
}

// validateSpecRefWithPending is the live spec_ref check CreateTask performs,
// extended to resolve a reference to the pending imported spec against that
// spec's about-to-be-written headings instead of the on-disk file (which may not
// exist yet, or may be a stale orphan the apply will overwrite).
func (s *Service) validateSpecRefWithPending(specRef string, pending *pendingSpec) (string, error) {
	if strings.TrimSpace(specRef) == "" {
		return "", errors.New("task spec_ref must not be empty")
	}
	pathPart, anchor, err := parseSpecRef(specRef)
	if err != nil {
		return "", err
	}
	// pending.path is slash-normalized (relPath applies filepath.ToSlash); pathPart
	// is OS-native from parseSpecRef, so normalize it before comparing on Windows.
	if pending != nil && filepath.ToSlash(pathPart) == pending.path {
		if _, ok := pending.anchors[anchor]; !ok {
			return "", fmt.Errorf("heading #%s not found in %s (pending import)", anchor, pathPart)
		}
		return normalizeSpecRef(specRef)
	}
	return s.validateSpecRef(specRef)
}

// readImportDraft loads and parses a draft file, resolving a relative path
// against the repo root. This is a read; an absolute path is honored as given.
func (s *Service) readImportDraft(path string) (ImportDraft, error) {
	draft, _, _, err := s.readImportDraftSnapshot(path)
	return draft, err
}

func (s *Service) readImportDraftSnapshot(path string) (ImportDraft, []byte, string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return ImportDraft{}, nil, "", WithMachineErrorCode(MachineCodeInvalidArguments,
			errors.New("import draft path must not be empty"))
	}
	resolved := s.resolveRepoPath(p)
	data, err := os.ReadFile(resolved)
	if err != nil {
		return ImportDraft{}, nil, "", invalidArgumentsf("read import draft %s: %w", relPath(s.paths.RepoRoot, resolved), fsCause(err))
	}
	draft, err := ParseImportDraft(data)
	if err != nil {
		return ImportDraft{}, nil, "", WithMachineErrorCode(MachineCodeInvalidProposal, err)
	}
	return draft, data, resolved, nil
}

func pathWithinRepository(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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
	logicalSpecPath := s.paths.logicalManagedPath(specPath)
	if fileExists(specPath) && !isImportedSpec(specPath) {
		return "", WithMachineErrorCode(MachineCodeDestinationExists,
			fmt.Errorf("spec file %s already exists; refusing to overwrite", logicalSpecPath))
	}
	if err := ensureDir(s.paths.RepoRoot, filepath.Dir(specPath)); err != nil {
		return "", err
	}
	if err := os.WriteFile(specPath, []byte(renderImportedSpec(draft)), 0o644); err != nil {
		return logicalSpecPath, fmt.Errorf("write imported spec %s: %w", logicalSpecPath, fsCause(err))
	}
	return logicalSpecPath, nil
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
