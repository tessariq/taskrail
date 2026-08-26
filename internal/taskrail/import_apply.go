package taskrail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tessariq/taskrail/internal/durabletx"
	"github.com/tessariq/taskrail/internal/repolock"
	"github.com/tessariq/taskrail/internal/repotx"
	"gopkg.in/yaml.v3"
)

// Agent-driven apply (T-034): `taskrail import --apply <draft.json>` ingests an
// ImportDraft an agent produced from the emit-prompt output. It validates the
// draft against the T-032 schema, writes any spec sections to a new spec file,
// and scaffolds each task through CreateTask (T-027) so drafts and hand-created
// tasks share one validation and id-allocation path. The binary makes no LLM
// call; the semantic work already happened in the agent.

// ApplyDraftInput names the draft file to ingest (repo-relative or absolute).
type ApplyDraftInput struct {
	DraftPath          string
	ExpectSHA256       string
	ReviewManifestPath string
	ExpectReviewSHA256 string
}

// applyReviewedImportDraft admits only an immutable decomposition publication.
// The bundle decoder proves its review bindings before this writer stages exact
// reviewed bodies into ordinary task candidates.
func (s *Service) applyReviewedImportDraft(input ApplyDraftInput) (result ApplyDraftResult, err error) {
	transactionID, err := newMigrationTransactionID()
	if err != nil {
		return ApplyDraftResult{}, err
	}
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: s.paths.LockRepository(), Command: "import", TransactionID: transactionID,
		Capability: repolock.Capability{Commands: []string{"import"}},
	})
	if err != nil {
		return ApplyDraftResult{}, writerLockError(err)
	}
	defer func() {
		if releaseErr := lock.Release(); releaseErr != nil && err == nil {
			err = WithMachineErrorCode(MachineCodeRepositoryInvalid, releaseErr)
		}
	}()
	marker, present, err := readMarker(s.paths.RepoRoot)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	if !present || marker.LayoutVersion != layout2Version {
		return ApplyDraftResult{}, WithMachineErrorCode(MachineCodeUnsupported, errors.New("reviewed import requires layout_version 2"))
	}
	data, draftPath, err := s.readImportDraftBytes(input.DraftPath)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	return s.applyReviewedImportDraftLocked(lock, input, data, draftPath)
}

func (s *Service) applyReviewedImportDraftLocked(own durabletx.Ownership, input ApplyDraftInput, draftBytes []byte, draftPath string) (ApplyDraftResult, error) {
	files, manifestPath, err := s.readPublishedDecompositionFiles(input, draftBytes, draftPath)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return ApplyDraftResult{}, err
	}
	if path.Base(path.Dir(path.Dir(input.ReviewManifestPath))) != state.Frontmatter.ActiveSpecVersion {
		return ApplyDraftResult{}, WithMachineErrorCode(MachineCodeInvalidProposal, errors.New("published decomposition directory does not match selected spec version"))
	}
	manifest, err := decodeDecompositionManifest(files["manifest.json"])
	if err != nil {
		return ApplyDraftResult{}, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("manifest.json: %w", err))
	}
	if manifest.SpecPath != state.Frontmatter.ActiveSpecPath {
		return ApplyDraftResult{}, WithMachineErrorCode(MachineCodeInvalidProposal, errors.New("reviewed import must target the selected active spec"))
	}
	specPath, err := s.paths.physicalSpecPath(manifest.SpecPath)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	spec, err := os.ReadFile(specPath)
	if err != nil {
		return ApplyDraftResult{}, reviewInputError("read selected spec", err)
	}
	specReviewFiles, err := s.readDecompositionSpecReview(manifest.SpecReviewManifestPath, state.Frontmatter.ActiveSpecVersion)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	taskIDs := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		taskIDs[task.Frontmatter.ID] = struct{}{}
	}
	bundle, err := DecodeDecompositionBundle(files, DecompositionSubjects{
		SpecPath: manifest.SpecPath, Spec: spec, SpecReviewManifestPath: manifest.SpecReviewManifestPath,
		SpecReviewFiles: specReviewFiles, TaskIDs: taskIDs,
	})
	if err != nil {
		return ApplyDraftResult{}, WithMachineErrorCode(MachineCodeInvalidProposal, err)
	}
	if path.Base(path.Dir(input.ReviewManifestPath)) != bundle.Manifest.SessionID {
		return ApplyDraftResult{}, WithMachineErrorCode(MachineCodeInvalidProposal, errors.New("published decomposition directory does not match manifest session identity"))
	}
	draft := ImportDraft{SchemaVersion: 2, Target: "tasks", Tasks: bundle.Draft.Tasks}
	baseline := s.validateInMemory(state, tasks)
	if !baseline.Valid {
		return ApplyDraftResult{}, WithMachineErrorCode(MachineCodeValidationFailed,
			fmt.Errorf("reviewed import requires a valid repository baseline: %s", strings.Join(baseline.Violations, "; ")))
	}
	if err := s.preflightImportDraft(tasks, draft); err != nil {
		return ApplyDraftResult{}, WithMachineErrorCode(MachineCodeInvalidProposal, err)
	}
	result, published, preview, err := s.buildImportCandidate(draft, state, tasks, true)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	stateBytes, err := marshalFrontmatter(state.Frontmatter, state.Body)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	published = append([]repotx.Candidate{managedCandidate(s.reportedStatePath(), s.paths.StateFile, stateBytes)}, published...)
	consumed, err := writerConsumedPaths(s.paths, preview, preview[len(tasks):]...)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	publishedPaths := make(map[string]struct{}, len(published))
	for _, candidate := range published {
		publishedPaths[candidate.Reported] = struct{}{}
	}
	filtered := consumed[:0]
	for _, candidate := range consumed {
		if _, published := publishedPaths[candidate.Reported]; !published {
			filtered = append(filtered, candidate)
		}
	}
	consumed = filtered
	for name := range files {
		consumed = append(consumed, repotx.Path{Kind: repotx.Managed, Reported: path.Join(path.Dir(relPath(s.paths.RepoRoot, manifestPath)), name), Physical: filepath.Join(filepath.Dir(manifestPath), name)})
	}
	for name := range specReviewFiles {
		logical := path.Join(path.Dir(manifest.SpecReviewManifestPath), name)
		physical, err := s.paths.physicalManagedPath(logical)
		if err != nil {
			return ApplyDraftResult{}, err
		}
		consumed = append(consumed, repotx.Path{Kind: repotx.Managed, Reported: logical, Physical: physical})
	}
	inputs, err := snapshotImportInputs(candidatePaths(consumed, published))
	if err != nil {
		return ApplyDraftResult{}, err
	}
	request := durabletx.Request{
		Command: "import", Consumed: durableImportPaths(consumed), Members: durableImportMembers(published),
		Validate: func(evidence []durabletx.Evidence) error {
			if durableBeforeCandidate(evidence) && !sameImportInputs(inputs) {
				return errImportInputChanged
			}
			if candidateIntroducesViolations(ValidationResult{Violations: s.validateState(state)}, baseline) {
				return errors.New("import candidate failed validation")
			}
			return nil
		},
	}
	if _, err := durabletx.Run(context.Background(), own, s.paths.LockRepository(), request); err != nil {
		return ApplyDraftResult{}, writerTransactionError(err)
	}
	return result, nil
}

func (s *Service) readPublishedDecompositionFiles(input ApplyDraftInput, draftBytes []byte, draftPath string) (map[string][]byte, string, error) {
	if !reviewDigest.MatchString(input.ExpectSHA256) || !reviewDigest.MatchString(input.ExpectReviewSHA256) {
		return nil, "", WithMachineErrorCode(MachineCodeInvalidDigest, errors.New("reviewed import requires lower-case expected draft and manifest SHA-256 digests"))
	}
	if digestRaw(draftBytes) != input.ExpectSHA256 {
		return nil, "", WithMachineErrorCode(MachineCodeSourceChanged, errors.New("reviewed draft digest does not match --expect-sha256"))
	}
	draftLogical := filepath.ToSlash(relPath(s.paths.RepoRoot, draftPath))
	manifestLogical := strings.TrimSpace(input.ReviewManifestPath)
	if filepath.IsAbs(manifestLogical) || filepath.ToSlash(manifestLogical) != manifestLogical || path.Clean(manifestLogical) != manifestLogical || path.Base(draftLogical) != "draft.json" || path.Base(manifestLogical) != "manifest.json" || path.Dir(draftLogical) != path.Dir(manifestLogical) {
		return nil, "", invalidArgumentsf("reviewed import requires published draft.json and manifest.json from one canonical directory")
	}
	parts := strings.Split(path.Dir(draftLogical), "/")
	if len(parts) != 5 || parts[0] != s.paths.LogicalPlanningDir || parts[1] != "reviews" || parts[2] != "decomposition" || !isPortableReviewKey(parts[4]) {
		return nil, "", WithMachineErrorCode(MachineCodeInvalidProposal, errors.New("reviewed draft is not a published decomposition bundle"))
	}
	manifestPath, err := s.paths.physicalManagedPath(manifestLogical)
	if err != nil {
		return nil, "", err
	}
	entries, err := os.ReadDir(filepath.Dir(manifestPath))
	if err != nil {
		return nil, "", reviewInputError("inspect published decomposition bundle", err)
	}
	files := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			return nil, "", WithMachineErrorCode(MachineCodeInvalidProposal, errors.New("published decomposition bundle contains a directory"))
		}
		data, err := os.ReadFile(filepath.Join(filepath.Dir(manifestPath), entry.Name()))
		if err != nil {
			return nil, "", reviewInputError("read published decomposition bundle", err)
		}
		files[entry.Name()] = data
	}
	if !slices.Equal(files["draft.json"], draftBytes) || digestRaw(files["manifest.json"]) != input.ExpectReviewSHA256 {
		return nil, "", WithMachineErrorCode(MachineCodeSourceChanged, errors.New("published decomposition inputs changed or manifest digest does not match"))
	}
	return files, manifestPath, nil
}

func durableImportPaths(paths []repotx.Path) []durabletx.Path {
	result := make([]durabletx.Path, len(paths))
	for i, input := range paths {
		result[i] = durabletx.Path{Kind: durabletx.PathKind(input.Kind), Reported: input.Reported, Path: input.Reported}
	}
	return result
}

func durableImportMembers(candidates []repotx.Candidate) []durabletx.Member {
	result := make([]durabletx.Member, len(candidates))
	for i, candidate := range candidates {
		result[i] = durabletx.Member{Kind: durabletx.PathKind(candidate.Kind), Reported: candidate.Reported, Path: candidate.Reported, Content: candidate.Content}
	}
	return result
}

func candidatePaths(consumed []repotx.Path, published []repotx.Candidate) []string {
	paths := make([]string, 0, len(consumed)+len(published))
	for _, input := range consumed {
		paths = append(paths, input.Physical)
	}
	for _, candidate := range published {
		paths = append(paths, candidate.Physical)
	}
	return paths
}

func durableBeforeCandidate(evidence []durabletx.Evidence) bool {
	for _, entry := range evidence {
		if entry.CurrentSHA256 != entry.OriginalSHA256 {
			return false
		}
	}
	return true
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
	if data, _, readErr := s.readImportDraftBytes(input.DraftPath); readErr == nil {
		var version struct {
			SchemaVersion int `json:"schema_version"`
		}
		if json.Unmarshal(data, &version) == nil && version.SchemaVersion == 2 {
			return s.applyReviewedImportDraft(input)
		}
	}
	own, release, err := s.acquireWriterLock("import", nil)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil && err == nil {
			err = releaseErr
		}
	}()

	data, draftPath, err := s.readImportDraftBytes(input.DraftPath)
	if err != nil {
		return ApplyDraftResult{}, err
	}
	var version struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &version); err != nil {
		return ApplyDraftResult{}, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("parse import draft: %w", err))
	}
	if version.SchemaVersion == 2 {
		return ApplyDraftResult{}, WithMachineErrorCode(MachineCodeSourceChanged, errors.New("import draft changed while acquiring the writer lock"))
	}
	draft, err := ParseImportDraft(data)
	if err != nil {
		return ApplyDraftResult{}, WithMachineErrorCode(MachineCodeInvalidProposal, err)
	}
	if input.ExpectSHA256 != "" || input.ReviewManifestPath != "" || input.ExpectReviewSHA256 != "" {
		return ApplyDraftResult{}, invalidArgumentsf("reviewed import flags require an ImportDraft v2")
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

	result, published, preview, err := s.buildImportCandidate(draft, state, tasks, false)
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
func (s *Service) buildImportCandidate(draft ImportDraft, state *State, tasks []*Task, reviewed bool) (ApplyDraftResult, []repotx.Candidate, []*Task, error) {
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
		body := renderNewTaskBody(id, strings.TrimSpace(draftTask.Title), "")
		if reviewed {
			body = fmt.Sprintf("# %s %s\n\n%s", id, strings.TrimSpace(draftTask.Title), draftTask.Body)
		}
		candidate := &Task{Frontmatter: TaskFrontmatter{ID: id, Title: strings.TrimSpace(draftTask.Title), Status: "todo", Priority: priority, SpecRef: specRef, Dependencies: translateDeps(draftTask.Dependencies, keyToID), UpdatedAt: now}, Body: body, Path: s.paths.logicalManagedPath(filepath.Join(s.paths.TasksDir, id+".md")), Filename: filepath.Join(s.paths.TasksDir, id+".md")}
		bytes, err := marshalFrontmatter(candidate.Frontmatter, candidate.Body)
		if reviewed {
			bytes, err = marshalReviewedTask(candidate.Frontmatter, candidate.Body)
		}
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

func marshalReviewedTask(frontmatter TaskFrontmatter, body string) ([]byte, error) {
	data, err := yaml.Marshal(frontmatter)
	if err != nil {
		return nil, fmt.Errorf("marshal reviewed task frontmatter: %w", err)
	}
	var out bytes.Buffer
	out.WriteString("---\n")
	out.Write(data)
	out.WriteString("---\n\n")
	out.WriteString(body)
	return out.Bytes(), nil
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
	data, resolved, err := s.readImportDraftBytes(path)
	if err != nil {
		return ImportDraft{}, nil, "", err
	}
	draft, err := ParseImportDraft(data)
	if err != nil {
		return ImportDraft{}, nil, "", WithMachineErrorCode(MachineCodeInvalidProposal, err)
	}
	return draft, data, resolved, nil
}

func (s *Service) readImportDraftBytes(path string) ([]byte, string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil, "", WithMachineErrorCode(MachineCodeInvalidArguments,
			errors.New("import draft path must not be empty"))
	}
	resolved := s.resolveRepoPath(p)
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, "", invalidArgumentsf("read import draft %s: %w", relPath(s.paths.RepoRoot, resolved), fsCause(err))
	}
	return data, resolved, nil
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
