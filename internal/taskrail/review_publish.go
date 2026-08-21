package taskrail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/tessariq/taskrail/internal/durablefs"
	"github.com/tessariq/taskrail/internal/repolock"
	"github.com/tessariq/taskrail/internal/reviewdir"
)

// ReviewPublishInput selects one review proposal and binds it to its reviewed
// subjects. Each review type supplies a type-specific adapter to the shared
// directory publisher.
type ReviewPublishInput struct {
	Type                   string
	Proposal               string
	Destination            string
	Spec                   string
	TaskID                 string
	ExpectTaskSHA256       string
	ExpectSpecSHA256       string
	SpecReview             string
	ExpectSpecReviewSHA256 string
	DryRun                 bool

	// These retain Cobra's selected-type flag boundary when an explicitly empty
	// flag would otherwise be indistinguishable from an omitted one.
	SpecFlagSet                   bool
	TaskFlagSet                   bool
	ExpectTaskSHA256FlagSet       bool
	SpecReviewFlagSet             bool
	ExpectSpecReviewSHA256FlagSet bool
	TaskFlagsProvided             bool
	DecompositionFlagsProvided    bool
}

type ReviewPublishFile struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	SHA256      string `json:"sha256"`
}

type ReviewPublishSubject struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type ReviewPublishResult struct {
	Type        string                 `json:"type"`
	Applied     bool                   `json:"applied"`
	Proposal    string                 `json:"proposal"`
	Destination string                 `json:"destination"`
	Files       []ReviewPublishFile    `json:"files"`
	Subjects    []ReviewPublishSubject `json:"subjects"`
	Validation  ValidationResult       `json:"validation"`
	Transaction any                    `json:"transaction"`
}

type taskReviewPublication struct {
	proposal     string
	destination  string
	proposalFile []byte
	config       []byte
	task         []byte
	spec         []byte
	taskPath     string
	specPath     string
	taskSHA256   string
	specSHA256   string
	review       TaskReview
}

type specReviewPublication struct {
	proposal    string
	destination string
	config      []byte
	spec        []byte
	specPath    string
	specSHA256  string
	bundle      SpecReviewBundle
}

type reviewParent struct {
	path     string
	identity durablefs.Identity
}

var testHookAfterReviewParent func()

type decompositionReviewPublication struct {
	proposal, destination, specPath, specReviewPath string
	config, spec, specReview                        []byte
	specSHA256, specReviewSHA256                    string
	bundle                                          DecompositionBundle
}

// ReviewPublish validates a proposal without writing in preview mode. Apply
// repeats the complete observation after taking the writer lock, then delegates
// the single no-clobber directory commit to reviewdir.
func (s *Service) ReviewPublish(input ReviewPublishInput) (ReviewPublishResult, error) {
	if err := s.paths.ensureStorageCapability(); err != nil {
		return ReviewPublishResult{}, err
	}
	if err := validateReviewPublishInput(input); err != nil {
		return ReviewPublishResult{}, err
	}
	switch reviewdir.Type(input.Type) {
	case reviewdir.TypeTask:
		return s.reviewPublishTask(input)
	case reviewdir.TypeSpec:
		return s.reviewPublishSpec(input)
	case reviewdir.TypeDecomposition:
		return s.publishDecompositionReview(input)
	default:
		return ReviewPublishResult{}, invalidArgumentsf("unsupported review type %q", input.Type)
	}
}

func (s *Service) reviewPublishTask(input ReviewPublishInput) (ReviewPublishResult, error) {
	candidate, err := s.taskReviewPublication(input)
	if err != nil {
		return ReviewPublishResult{}, err
	}
	result := candidate.result(false)
	if input.DryRun {
		return result, nil
	}
	own, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: s.paths.LockRepository(),
		Command:    "review publish",
		Capability: repolock.Capability{Commands: []string{"review publish"}},
	})
	if err != nil {
		return ReviewPublishResult{}, writerLockError(err)
	}
	defer func() { _ = own.Release() }()
	if err := s.validateWriterStorage(); err != nil {
		return ReviewPublishResult{}, err
	}
	// Re-read after lock acquisition so preview evidence is never reused for an
	// apply against changed proposal or reviewed-subject bytes.
	candidate, err = s.taskReviewPublication(input)
	if err != nil {
		return ReviewPublishResult{}, err
	}
	parents, err := s.ensureReviewParent(own, candidate.destination)
	if err != nil {
		return ReviewPublishResult{}, reviewPublishError(err)
	}
	if testHookAfterReviewParent != nil {
		testHookAfterReviewParent()
	}
	request := reviewdir.Request{
		Type:        reviewdir.TypeTask,
		ReviewsRoot: path.Join(s.paths.LogicalPlanningDir, "reviews"),
		Destination: candidate.destination,
		Files:       []reviewdir.File{{Name: "review.json", Content: candidate.proposalFile}},
		Validate: func(_ reviewdir.Type, files []reviewdir.File) error {
			current, err := s.taskReviewPublication(input)
			if err != nil {
				return err
			}
			if !sameTaskReviewPublication(candidate, current) || len(files) != 1 || string(files[0].Content) != string(candidate.proposalFile) {
				return WithMachineErrorCode(MachineCodeWriteConflict, fmt.Errorf("review publication inputs changed before commit"))
			}
			return nil
		},
		ValidateCommit: func() error {
			current, err := s.taskReviewPublication(input)
			if err != nil {
				return err
			}
			if !sameTaskReviewPublication(candidate, current) {
				return WithMachineErrorCode(MachineCodeWriteConflict, fmt.Errorf("review publication inputs changed before commit"))
			}
			return nil
		},
	}
	if _, err := reviewdir.Publish(context.Background(), own, request); err != nil {
		err = errors.Join(err, s.removeReviewParents(own, parents))
		return ReviewPublishResult{}, reviewPublishError(err)
	}
	result = candidate.result(true)
	return result, nil
}

func (s *Service) reviewPublishSpec(input ReviewPublishInput) (ReviewPublishResult, error) {
	candidate, err := s.specReviewPublication(input)
	if err != nil {
		return ReviewPublishResult{}, err
	}
	result := candidate.result(false)
	if input.DryRun {
		return result, nil
	}
	own, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: s.paths.LockRepository(),
		Command:    "review publish",
		Capability: repolock.Capability{Commands: []string{"review publish"}},
	})
	if err != nil {
		return ReviewPublishResult{}, writerLockError(err)
	}
	defer func() { _ = own.Release() }()
	if err := s.validateWriterStorage(); err != nil {
		return ReviewPublishResult{}, err
	}
	candidate, err = s.specReviewPublication(input)
	if err != nil {
		return ReviewPublishResult{}, err
	}
	parents, err := s.ensureReviewParent(own, candidate.destination)
	if err != nil {
		return ReviewPublishResult{}, reviewPublishError(err)
	}
	if testHookAfterReviewParent != nil {
		testHookAfterReviewParent()
	}
	request := reviewdir.Request{
		Type:        reviewdir.TypeSpec,
		ReviewsRoot: path.Join(s.paths.LogicalPlanningDir, "reviews"),
		Destination: candidate.destination,
		Files:       candidate.files(),
		Validate: func(_ reviewdir.Type, files []reviewdir.File) error {
			current, err := s.specReviewPublication(input)
			if err != nil {
				return err
			}
			if !sameSpecReviewPublication(candidate, current) || !sameReviewFiles(files, candidate.files()) {
				return WithMachineErrorCode(MachineCodeWriteConflict, fmt.Errorf("review publication inputs changed before commit"))
			}
			return nil
		},
		ValidateCommit: func() error {
			current, err := s.specReviewPublication(input)
			if err != nil {
				return err
			}
			if !sameSpecReviewPublication(candidate, current) {
				return WithMachineErrorCode(MachineCodeWriteConflict, fmt.Errorf("review publication inputs changed before commit"))
			}
			return nil
		},
	}
	if _, err := reviewdir.Publish(context.Background(), own, request); err != nil {
		err = errors.Join(err, s.removeReviewParents(own, parents))
		return ReviewPublishResult{}, reviewPublishError(err)
	}
	return candidate.result(true), nil
}

func validateReviewPublishInput(input ReviewPublishInput) error {
	switch input.Type {
	case string(reviewdir.TypeTask):
		if input.DecompositionFlagsProvided || input.Spec != "" || input.SpecReview != "" || input.ExpectSpecReviewSHA256 != "" {
			return invalidArgumentsf("task review publication does not accept decomposition flags")
		}
	case string(reviewdir.TypeSpec):
		if input.TaskFlagsProvided || input.TaskID != "" || input.ExpectTaskSHA256 != "" || input.SpecReviewFlagSet || input.ExpectSpecReviewSHA256FlagSet || input.SpecReview != "" || input.ExpectSpecReviewSHA256 != "" {
			return invalidArgumentsf("spec review publication does not accept task or decomposition-only flags")
		}
	case string(reviewdir.TypeDecomposition):
		if input.TaskFlagsProvided || input.TaskID != "" || input.ExpectTaskSHA256 != "" {
			return invalidArgumentsf("decomposition review publication does not accept task flags")
		}
	default:
		return invalidArgumentsf("unsupported review type %q", input.Type)
	}
	return nil
}

// ensureReviewParent creates only the namespace leading to an absent session.
// The session directory itself remains the publisher's sole commit point.
func (s *Service) ensureReviewParent(own *repolock.Lock, destination string) ([]reviewParent, error) {
	root, err := durablefs.OpenAt(s.paths.StorageRoot, s.paths.LockRepository(), own)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	var created []reviewParent
	cleanup := func(cause error) ([]reviewParent, error) {
		for i := len(created) - 1; i >= 0; i-- {
			cause = errors.Join(cause, root.RemoveDirExpected(created[i].path, created[i].identity))
		}
		return nil, cause
	}
	parts := strings.Split(path.Dir(destination), "/")
	for i := range parts {
		current := strings.Join(parts[:i+1], "/")
		tree, err := durablefs.ObserveTree(s.paths.StorageRoot, current)
		if err != nil {
			return cleanup(err)
		}
		if tree.Present {
			continue
		}
		dir, err := root.Mkdir(current, 0o755)
		if dir != nil {
			created = append(created, reviewParent{path: current, identity: dir.Identity})
		}
		if err != nil {
			return cleanup(err)
		}
		if err := root.SyncDir(dir.Path); err != nil {
			return cleanup(err)
		}
	}
	return created, nil
}

func (s *Service) removeReviewParents(own *repolock.Lock, parents []reviewParent) error {
	root, err := durablefs.OpenAt(s.paths.StorageRoot, s.paths.LockRepository(), own)
	if err != nil {
		return err
	}
	defer root.Close()
	var problems []error
	for i := len(parents) - 1; i >= 0; i-- {
		if err := root.RemoveDirExpected(parents[i].path, parents[i].identity); err != nil && !errors.Is(err, os.ErrNotExist) {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}

func (s *Service) taskReviewPublication(input ReviewPublishInput) (taskReviewPublication, error) {
	var out taskReviewPublication
	if input.TaskID == "" || input.Proposal == "" || input.Destination == "" {
		return out, invalidArgumentsf("task review publication requires task, proposal, and destination")
	}
	if input.Spec != "" || input.SpecFlagSet {
		return out, invalidArgumentsf("task review publication does not accept spec flags")
	}
	if !reviewDigest.MatchString(input.ExpectTaskSHA256) || !reviewDigest.MatchString(input.ExpectSpecSHA256) {
		return out, WithMachineErrorCode(MachineCodeInvalidDigest, fmt.Errorf("expected task and spec digests must be lower-case 64-hex"))
	}
	proposal, err := s.reviewProposalPath(input.Proposal, reviewdir.TypeTask)
	if err != nil {
		return out, err
	}
	if err := s.requireTransientProposal(proposal); err != nil {
		return out, err
	}
	tree, err := durablefs.ObserveTree(s.paths.RepoRoot, proposal)
	if err != nil || !tree.Present || len(tree.Entries) != 1 || tree.Entries[0].Directory || tree.Entries[0].Path != "review.json" {
		return out, reviewInputError("inspect task review proposal", err)
	}
	proposalFile, _, err := durablefs.ReadFile(s.paths.RepoRoot, path.Join(proposal, "review.json"), reviewFileLimit)
	if err != nil {
		return out, reviewInputError("read task review proposal", err)
	}
	review, err := DecodeTaskReview(proposalFile)
	if err != nil {
		return out, WithMachineErrorCode(MachineCodeInvalidProposal, err)
	}
	destination := path.Clean(input.Destination)
	if destination != input.Destination || destination != path.Join(s.paths.LogicalPlanningDir, "reviews", "task", input.TaskID, review.SessionID) {
		return out, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("task review destination does not match task and session identity"))
	}
	if path.Base(proposal) != review.SessionID {
		return out, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("task review proposal does not match session identity"))
	}
	destinationTree, err := durablefs.ObserveTree(s.paths.StorageRoot, destination)
	if err != nil {
		return out, reviewInputError("inspect task review destination", err)
	}
	if destinationTree.Present {
		return out, WithMachineErrorCode(MachineCodeDestinationExists, fmt.Errorf("task review destination already exists"))
	}
	tasks, err := s.loadTasks()
	if err != nil {
		return out, err
	}
	var task *Task
	for _, candidate := range tasks {
		if candidate.Frontmatter.ID == input.TaskID {
			task = candidate
			break
		}
	}
	if task == nil {
		return out, WithMachineErrorCode(MachineCodeTaskNotFound, fmt.Errorf("task %q not found", input.TaskID))
	}
	taskRel, err := s.storageRelative(task.Filename)
	if err != nil {
		return out, err
	}
	taskBytes, _, err := durablefs.ReadFile(s.paths.StorageRoot, taskRel, 1<<30)
	if err != nil {
		return out, reviewInputError("read reviewed task", err)
	}
	taskFrontmatter, _, err := parseFrontmatter[TaskFrontmatter](taskBytes)
	if err != nil {
		return out, WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("parse reviewed task: %w", err))
	}
	if taskFrontmatter.ID != input.TaskID {
		return out, WithMachineErrorCode(MachineCodeWriteConflict, fmt.Errorf("reviewed task identity changed while reading"))
	}
	specPath, _, err := parseSpecRef(taskFrontmatter.SpecRef)
	if err != nil {
		return out, WithMachineErrorCode(MachineCodeRepositoryInvalid, err)
	}
	specFile, err := s.paths.physicalSpecPath(specPath)
	if err != nil {
		return out, err
	}
	specRel, err := s.storageRelative(specFile)
	if err != nil {
		return out, err
	}
	specBytes, _, err := durablefs.ReadFile(s.paths.StorageRoot, specRel, 1<<30)
	if err != nil {
		return out, reviewInputError("read reviewed spec", err)
	}
	configRel, err := filepath.Rel(s.paths.RepoRoot, s.paths.ConfigFile)
	if err != nil {
		return out, err
	}
	config, _, err := durablefs.ReadFile(s.paths.RepoRoot, filepath.ToSlash(configRel), 1<<20)
	if err != nil {
		return out, reviewInputError("read Taskrail configuration", err)
	}
	out = taskReviewPublication{proposal: proposal, destination: destination, proposalFile: proposalFile, config: config, task: taskBytes, spec: specBytes, taskPath: task.Path, specPath: s.paths.logicalManagedPath(specFile), taskSHA256: digestRaw(taskBytes), specSHA256: digestRaw(specBytes), review: review}
	if out.taskSHA256 != input.ExpectTaskSHA256 || out.specSHA256 != input.ExpectSpecSHA256 || review.TaskID != input.TaskID || review.TaskPath != task.Path || review.TaskSHA256 != out.taskSHA256 || review.SpecPath != out.specPath || review.SpecSHA256 != out.specSHA256 {
		return taskReviewPublication{}, WithMachineErrorCode(MachineCodeSourceChanged, fmt.Errorf("task review bindings do not match current task and spec snapshots"))
	}
	return out, nil
}

func (s *Service) specReviewPublication(input ReviewPublishInput) (specReviewPublication, error) {
	var out specReviewPublication
	if input.Spec == "" || input.Proposal == "" || input.Destination == "" {
		return out, invalidArgumentsf("spec review publication requires spec, proposal, and destination")
	}
	if input.TaskID != "" || input.ExpectTaskSHA256 != "" || input.TaskFlagSet || input.ExpectTaskSHA256FlagSet {
		return out, invalidArgumentsf("spec review publication does not accept task flags")
	}
	if !reviewDigest.MatchString(input.ExpectSpecSHA256) {
		return out, WithMachineErrorCode(MachineCodeInvalidDigest, fmt.Errorf("expected spec digest must be lower-case 64-hex"))
	}
	version, specPath, specFile, err := s.reviewSpecPath(input.Spec)
	if err != nil {
		return out, err
	}
	proposal, err := s.reviewProposalPath(input.Proposal, reviewdir.TypeSpec)
	if err != nil {
		return out, err
	}
	if err := s.requireTransientProposal(proposal); err != nil {
		return out, err
	}
	files, err := s.specReviewProposalFiles(proposal)
	if err != nil {
		return out, err
	}
	bundle, err := DecodeSpecReviewBundle(files)
	if err != nil {
		return out, WithMachineErrorCode(MachineCodeInvalidProposal, err)
	}
	destination := path.Clean(input.Destination)
	if destination != input.Destination || destination != path.Join(s.paths.LogicalPlanningDir, "reviews", "spec", version, bundle.Manifest.SessionID) {
		return out, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("spec review destination does not match spec version and session identity"))
	}
	if path.Base(proposal) != bundle.Manifest.SessionID {
		return out, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("spec review proposal does not match session identity"))
	}
	destinationTree, err := durablefs.ObserveTree(s.paths.StorageRoot, destination)
	if err != nil {
		return out, reviewInputError("inspect spec review destination", err)
	}
	if destinationTree.Present {
		return out, WithMachineErrorCode(MachineCodeDestinationExists, fmt.Errorf("spec review destination already exists"))
	}
	specRel, err := s.storageRelative(specFile)
	if err != nil {
		return out, err
	}
	spec, _, err := durablefs.ReadFile(s.paths.StorageRoot, specRel, 1<<30)
	if err != nil {
		return out, reviewInputError("read reviewed spec", err)
	}
	configRel, err := filepath.Rel(s.paths.RepoRoot, s.paths.ConfigFile)
	if err != nil {
		return out, err
	}
	config, _, err := durablefs.ReadFile(s.paths.RepoRoot, filepath.ToSlash(configRel), 1<<20)
	if err != nil {
		return out, reviewInputError("read Taskrail configuration", err)
	}
	out = specReviewPublication{proposal: proposal, destination: destination, config: config, spec: spec, specPath: specPath, specSHA256: digestRaw(spec), bundle: bundle}
	if out.specSHA256 != input.ExpectSpecSHA256 || bundle.Manifest.SpecPath != specPath || bundle.Manifest.SpecSHA256 != out.specSHA256 {
		return specReviewPublication{}, WithMachineErrorCode(MachineCodeSourceChanged, fmt.Errorf("spec review bindings do not match current spec snapshot"))
	}
	if err := validateSpecReviewReferences(bundle, specPath, spec); err != nil {
		return specReviewPublication{}, WithMachineErrorCode(MachineCodeInvalidProposal, err)
	}
	return out, nil
}

func validateSpecReviewReferences(bundle SpecReviewBundle, specPath string, spec []byte) error {
	anchors := collectHeadingAnchors(string(spec))
	for _, disposition := range bundle.Manifest.Dispositions {
		if disposition.Disposition != "accepted" {
			continue
		}
		pathPart, anchor, err := parseSpecRef(*disposition.ResultingSpecRef)
		if err != nil || filepath.ToSlash(pathPart) != specPath {
			return fmt.Errorf("accepted finding %q has no live resulting_spec_ref in the selected spec", disposition.FindingID)
		}
		if _, ok := anchors[anchor]; !ok {
			return fmt.Errorf("accepted finding %q resulting_spec_ref names no selected spec heading", disposition.FindingID)
		}
	}
	return nil
}

func (s *Service) reviewSpecPath(value string) (version, logical, physical string, err error) {
	if specVersionPattern.MatchString(value) {
		version = value
		logical = path.Join(s.paths.LogicalSpecsDir, version+".md")
	} else {
		if filepath.IsAbs(value) || filepath.ToSlash(value) != value || path.Clean(value) != value || path.Dir(value) != s.paths.LogicalSpecsDir {
			return "", "", "", invalidArgumentsf("spec must be a version or canonical path directly beneath %q", s.paths.LogicalSpecsDir)
		}
		version = strings.TrimSuffix(path.Base(value), ".md")
		if !specVersionPattern.MatchString(version) {
			return "", "", "", invalidArgumentsf("spec must be a version or canonical versioned spec path")
		}
		logical = value
	}
	physical, err = s.paths.physicalSpecPath(logical)
	if err != nil {
		return "", "", "", err
	}
	return version, logical, physical, nil
}

func (s *Service) specReviewProposalFiles(proposal string) (map[string][]byte, error) {
	tree, err := durablefs.ObserveTree(s.paths.RepoRoot, proposal)
	if err != nil || !tree.Present || len(tree.Entries) != 5 {
		return nil, reviewInputError("inspect spec review proposal", err)
	}
	files := make(map[string][]byte, len(tree.Entries))
	for _, entry := range tree.Entries {
		if entry.Directory {
			return nil, reviewInputError("inspect spec review proposal", nil)
		}
		content, _, err := durablefs.ReadFile(s.paths.RepoRoot, path.Join(proposal, entry.Path), reviewFileLimit)
		if err != nil {
			return nil, reviewInputError("read spec review proposal", err)
		}
		files[entry.Path] = content
	}
	return files, nil
}

func (s *Service) publishDecompositionReview(input ReviewPublishInput) (ReviewPublishResult, error) {
	candidate, err := s.decompositionReviewPublication(input)
	if err != nil {
		return ReviewPublishResult{}, err
	}
	if input.DryRun {
		return candidate.result(false), nil
	}
	own, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: s.paths.LockRepository(),
		Command:    "review publish",
		Capability: repolock.Capability{Commands: []string{"review publish"}},
	})
	if err != nil {
		return ReviewPublishResult{}, writerLockError(err)
	}
	defer func() { _ = own.Release() }()
	if err := s.validateWriterStorage(); err != nil {
		return ReviewPublishResult{}, err
	}
	candidate, err = s.decompositionReviewPublication(input)
	if err != nil {
		return ReviewPublishResult{}, err
	}
	parents, err := s.ensureReviewParent(own, candidate.destination)
	if err != nil {
		return ReviewPublishResult{}, reviewPublishError(err)
	}
	if testHookAfterReviewParent != nil {
		testHookAfterReviewParent()
	}
	request := reviewdir.Request{
		Type:        reviewdir.TypeDecomposition,
		ReviewsRoot: path.Join(s.paths.LogicalPlanningDir, "reviews"),
		Destination: candidate.destination,
		Files:       candidate.files(),
		Validate: func(_ reviewdir.Type, files []reviewdir.File) error {
			current, err := s.decompositionReviewPublication(input)
			if err != nil {
				return err
			}
			if !sameDecompositionReviewPublication(candidate, current) || !sameReviewFiles(files, candidate.files()) {
				return WithMachineErrorCode(MachineCodeWriteConflict, fmt.Errorf("review publication inputs changed before commit"))
			}
			return nil
		},
		ValidateCommit: func() error {
			current, err := s.decompositionReviewPublication(input)
			if err != nil {
				return err
			}
			if !sameDecompositionReviewPublication(candidate, current) {
				return WithMachineErrorCode(MachineCodeWriteConflict, fmt.Errorf("review publication inputs changed before commit"))
			}
			return nil
		},
	}
	if _, err := reviewdir.Publish(context.Background(), own, request); err != nil {
		err = errors.Join(err, s.removeReviewParents(own, parents))
		return ReviewPublishResult{}, reviewPublishError(err)
	}
	return candidate.result(true), nil
}

func (s *Service) decompositionReviewPublication(input ReviewPublishInput) (decompositionReviewPublication, error) {
	var out decompositionReviewPublication
	if input.Spec == "" || input.Proposal == "" || input.Destination == "" || input.SpecReview == "" {
		return out, invalidArgumentsf("decomposition review publication requires spec, spec-review, proposal, and destination")
	}
	if !reviewDigest.MatchString(input.ExpectSpecSHA256) || !reviewDigest.MatchString(input.ExpectSpecReviewSHA256) {
		return out, WithMachineErrorCode(MachineCodeInvalidDigest, fmt.Errorf("expected spec and spec-review digests must be lower-case 64-hex"))
	}
	proposal, err := s.reviewProposalPath(input.Proposal, reviewdir.TypeDecomposition)
	if err != nil {
		return out, err
	}
	if err := s.requireTransientProposal(proposal); err != nil {
		return out, err
	}
	tree, err := durablefs.ObserveTree(s.paths.RepoRoot, proposal)
	if err != nil || !tree.Present {
		return out, reviewInputError("inspect decomposition review proposal", err)
	}
	files := make(map[string][]byte, len(tree.Entries))
	for _, entry := range tree.Entries {
		if entry.Directory {
			return out, reviewInputError("inspect decomposition review proposal", nil)
		}
		data, _, err := durablefs.ReadFile(s.paths.RepoRoot, path.Join(proposal, entry.Path), reviewFileLimit)
		if err != nil {
			return out, reviewInputError("read decomposition review proposal", err)
		}
		files[entry.Path] = data
	}
	version, specPath, specFile, err := s.reviewSpecPath(input.Spec)
	if err != nil {
		return out, err
	}
	specRel, err := s.storageRelative(specFile)
	if err != nil {
		return out, err
	}
	spec, _, err := durablefs.ReadFile(s.paths.StorageRoot, specRel, 1<<30)
	if err != nil {
		return out, reviewInputError("read selected spec", err)
	}
	specReview, err := s.readDecompositionSpecReview(input.SpecReview, version)
	if err != nil {
		return out, err
	}
	bundle, err := DecodeDecompositionBundle(files, DecompositionSubjects{
		SpecPath: specPath, Spec: spec, SpecReviewManifestPath: input.SpecReview, SpecReviewManifest: specReview,
	})
	if err != nil {
		return out, WithMachineErrorCode(MachineCodeInvalidProposal, err)
	}
	destination := path.Clean(input.Destination)
	if destination != input.Destination || destination != path.Join(s.paths.LogicalPlanningDir, "reviews", "decomposition", version, bundle.Manifest.SessionID) {
		return out, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("decomposition review destination does not match spec version and session identity"))
	}
	if path.Base(proposal) != bundle.Manifest.SessionID {
		return out, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("decomposition review proposal does not match session identity"))
	}
	destinationTree, err := durablefs.ObserveTree(s.paths.StorageRoot, destination)
	if err != nil {
		return out, reviewInputError("inspect decomposition review destination", err)
	}
	if destinationTree.Present {
		return out, WithMachineErrorCode(MachineCodeDestinationExists, fmt.Errorf("decomposition review destination already exists"))
	}
	configRel, err := filepath.Rel(s.paths.RepoRoot, s.paths.ConfigFile)
	if err != nil {
		return out, err
	}
	config, _, err := durablefs.ReadFile(s.paths.RepoRoot, filepath.ToSlash(configRel), 1<<20)
	if err != nil {
		return out, reviewInputError("read Taskrail configuration", err)
	}
	out = decompositionReviewPublication{
		proposal: proposal, destination: destination, specPath: specPath, specReviewPath: input.SpecReview,
		config: config, spec: spec, specReview: specReview, specSHA256: digestRaw(spec), specReviewSHA256: digestRaw(specReview), bundle: bundle,
	}
	if out.specSHA256 != input.ExpectSpecSHA256 || out.specReviewSHA256 != input.ExpectSpecReviewSHA256 {
		return decompositionReviewPublication{}, WithMachineErrorCode(MachineCodeSourceChanged, fmt.Errorf("decomposition review bindings do not match current spec and spec-review snapshots"))
	}
	return out, nil
}

func (s *Service) readDecompositionSpecReview(specReview, version string) ([]byte, error) {
	if filepath.IsAbs(specReview) || filepath.ToSlash(specReview) != specReview || path.Clean(specReview) != specReview {
		return nil, invalidArgumentsf("spec-review must be a canonical repository-relative path")
	}
	prefix := path.Join(s.paths.LogicalPlanningDir, "reviews", "spec", version) + "/"
	rel := strings.TrimPrefix(specReview, prefix)
	if rel == specReview || path.Base(specReview) != "manifest.json" || strings.Count(rel, "/") != 1 || !isPortableReviewKey(path.Dir(rel)) {
		return nil, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("spec-review is not a final manifest for selected spec version"))
	}
	physical, err := s.paths.physicalManagedPath(specReview)
	if err != nil {
		return nil, err
	}
	storageRel, err := s.storageRelative(physical)
	if err != nil {
		return nil, err
	}
	data, _, err := durablefs.ReadFile(s.paths.StorageRoot, storageRel, reviewFileLimit)
	if err != nil {
		return nil, reviewInputError("read post-spec review manifest", err)
	}
	return data, nil
}

func (p decompositionReviewPublication) files() []reviewdir.File {
	names := []string{"draft.json", "trace.json", "review-1.json"}
	if _, ok := p.bundle.Raw["review-2.json"]; ok {
		names = append(names, "review-2.json")
	}
	names = append(names, "manifest.json")
	files := make([]reviewdir.File, 0, len(names))
	for _, name := range names {
		files = append(files, reviewdir.File{Name: name, Content: p.bundle.Raw[name]})
	}
	return files
}

func (p decompositionReviewPublication) result(applied bool) ReviewPublishResult {
	files := p.files()
	resultFiles := make([]ReviewPublishFile, len(files))
	for i, file := range files {
		resultFiles[i] = ReviewPublishFile{Source: path.Join(p.proposal, file.Name), Destination: path.Join(p.destination, file.Name), SHA256: digestRaw(file.Content)}
	}
	return ReviewPublishResult{Type: string(reviewdir.TypeDecomposition), Applied: applied, Proposal: p.proposal, Destination: p.destination,
		Files: resultFiles, Subjects: []ReviewPublishSubject{{Path: p.specPath, SHA256: p.specSHA256}, {Path: p.specReviewPath, SHA256: p.specReviewSHA256}},
		Validation: ValidationResult{Valid: true, Violations: []string{}}, Transaction: nil}
}

func sameDecompositionReviewPublication(a, b decompositionReviewPublication) bool {
	if a.proposal != b.proposal || a.destination != b.destination || a.specPath != b.specPath || a.specReviewPath != b.specReviewPath || a.specSHA256 != b.specSHA256 || a.specReviewSHA256 != b.specReviewSHA256 || !bytes.Equal(a.config, b.config) || !bytes.Equal(a.spec, b.spec) || !bytes.Equal(a.specReview, b.specReview) || len(a.bundle.Raw) != len(b.bundle.Raw) {
		return false
	}
	for name, raw := range a.bundle.Raw {
		if !bytes.Equal(raw, b.bundle.Raw[name]) {
			return false
		}
	}
	return true
}

func (s *Service) reviewProposalPath(proposal string, reviewType reviewdir.Type) (string, error) {
	if filepath.IsAbs(proposal) || filepath.ToSlash(proposal) != proposal || path.Clean(proposal) != proposal {
		return "", invalidArgumentsf("proposal must be a canonical repository-relative path")
	}
	artifacts := filepath.ToSlash(relPath(s.paths.RepoRoot, s.paths.ArtifactsDir))
	prefix := path.Join(artifacts, "review-proposals", string(reviewType)) + "/"
	if !strings.HasPrefix(proposal, prefix) || !isPortableReviewKey(path.Base(proposal)) || strings.Count(strings.TrimPrefix(proposal, prefix), "/") != 0 {
		return "", WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("proposal is outside the %s review proposal root", reviewType))
	}
	return proposal, nil
}

func (s *Service) requireTransientProposal(proposal string) error {
	if s.paths.WorktreeRoot == "" {
		return nil
	}
	if _, err := gitCommand(s.paths.WorktreeRoot, "check-ignore", "-q", "--no-index", proposal); err != nil {
		return WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("review proposal %q is not effectively ignored", proposal))
	}
	if output, err := gitCommand(s.paths.WorktreeRoot, "ls-files", "--error-unmatch", "--", proposal); err == nil && strings.TrimSpace(output) != "" {
		return WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("Git tracks review proposal %q", proposal))
	}
	if _, err := gitCommand(s.paths.WorktreeRoot, "diff", "--cached", "--quiet", "--", proposal); err != nil {
		return WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("Git index contains review proposal %q", proposal))
	}
	return nil
}

func (s *Service) storageRelative(physical string) (string, error) {
	rel, err := filepath.Rel(s.paths.StorageRoot, physical)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", WithMachineErrorCode(MachineCodeRepositoryInvalid, fmt.Errorf("managed path is outside active storage"))
	}
	return filepath.ToSlash(rel), nil
}

func (p taskReviewPublication) result(applied bool) ReviewPublishResult {
	return ReviewPublishResult{Type: "task", Applied: applied, Proposal: p.proposal, Destination: p.destination,
		Files:      []ReviewPublishFile{{Source: path.Join(p.proposal, "review.json"), Destination: path.Join(p.destination, "review.json"), SHA256: digestRaw(p.proposalFile)}},
		Subjects:   []ReviewPublishSubject{{Path: p.taskPath, SHA256: p.taskSHA256}, {Path: p.specPath, SHA256: p.specSHA256}},
		Validation: ValidationResult{Valid: true, Violations: []string{}}, Transaction: nil}
}

func sameTaskReviewPublication(a, b taskReviewPublication) bool {
	return a.proposal == b.proposal && a.destination == b.destination && a.taskPath == b.taskPath && a.specPath == b.specPath && a.taskSHA256 == b.taskSHA256 && a.specSHA256 == b.specSHA256 && string(a.proposalFile) == string(b.proposalFile) && string(a.config) == string(b.config) && string(a.task) == string(b.task) && string(a.spec) == string(b.spec)
}

func (p specReviewPublication) files() []reviewdir.File {
	files := make([]reviewdir.File, 0, 5)
	for _, name := range []string{"consistency.json", "gaps.json", "additions.json", "adversarial.json", "manifest.json"} {
		files = append(files, reviewdir.File{Name: name, Content: p.bundle.Raw[name]})
	}
	return files
}

func (p specReviewPublication) result(applied bool) ReviewPublishResult {
	files := p.files()
	result := ReviewPublishResult{Type: "spec", Applied: applied, Proposal: p.proposal, Destination: p.destination,
		Files: make([]ReviewPublishFile, len(files)), Subjects: []ReviewPublishSubject{{Path: p.specPath, SHA256: p.specSHA256}},
		Validation: ValidationResult{Valid: true, Violations: []string{}}, Transaction: nil}
	for i, file := range files {
		result.Files[i] = ReviewPublishFile{Source: path.Join(p.proposal, file.Name), Destination: path.Join(p.destination, file.Name), SHA256: digestRaw(file.Content)}
	}
	return result
}

func sameSpecReviewPublication(a, b specReviewPublication) bool {
	if a.proposal != b.proposal || a.destination != b.destination || a.specPath != b.specPath || a.specSHA256 != b.specSHA256 || string(a.config) != string(b.config) || string(a.spec) != string(b.spec) {
		return false
	}
	return sameReviewFiles(a.files(), b.files())
}

func sameReviewFiles(a, b []reviewdir.File) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || string(a[i].Content) != string(b[i].Content) {
			return false
		}
	}
	return true
}

func reviewInputError(action string, err error) error {
	code := MachineCodeInvalidProposal
	if errors.Is(err, durablefs.ErrAlias) || errors.Is(err, durablefs.ErrInvalidPath) || errors.Is(err, durablefs.ErrNotRegular) {
		code = MachineCodePathBlocked
	}
	if err == nil {
		return WithMachineErrorCode(code, fmt.Errorf("%s: proposal inventory is invalid", action))
	}
	return WithMachineErrorCode(code, fmt.Errorf("%s: %w", action, fsCause(err)))
}

func reviewPublishError(err error) error {
	if failure := MachineFailureFor(err); failure.Code != MachineCodeRepositoryInvalid {
		return err
	}
	code := MachineCodeRepositoryInvalid
	switch {
	case errors.Is(err, os.ErrExist):
		code = MachineCodeDestinationExists
	case errors.Is(err, durablefs.ErrAlias), errors.Is(err, durablefs.ErrInvalidPath), errors.Is(err, durablefs.ErrNotRegular):
		code = MachineCodePathBlocked
	case errors.Is(err, durablefs.ErrConflict):
		code = MachineCodeWriteConflict
	}
	return WithMachineErrorCode(code, err)
}
