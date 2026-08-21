package taskrail

import (
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
// subject. Only task reviews are registered here; other bundle meanings stay in
// their type-specific adapters.
type ReviewPublishInput struct {
	Type             string
	Proposal         string
	Destination      string
	TaskID           string
	ExpectTaskSHA256 string
	ExpectSpecSHA256 string
	DryRun           bool
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

// ReviewPublish validates a proposal without writing in preview mode. Apply
// repeats the complete observation after taking the writer lock, then delegates
// the single no-clobber directory commit to reviewdir.
func (s *Service) ReviewPublish(input ReviewPublishInput) (ReviewPublishResult, error) {
	if err := s.paths.ensureStorageCapability(); err != nil {
		return ReviewPublishResult{}, err
	}
	if input.Type != string(reviewdir.TypeTask) {
		return ReviewPublishResult{}, invalidArgumentsf("unsupported review type %q", input.Type)
	}
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
	if err := s.ensureReviewParent(own, candidate.destination); err != nil {
		return ReviewPublishResult{}, reviewPublishError(err)
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
		return ReviewPublishResult{}, reviewPublishError(err)
	}
	result = candidate.result(true)
	return result, nil
}

// ensureReviewParent creates only the namespace leading to an absent session.
// The session directory itself remains the publisher's sole commit point.
func (s *Service) ensureReviewParent(own *repolock.Lock, destination string) error {
	root, err := durablefs.OpenAt(s.paths.StorageRoot, s.paths.LockRepository(), own)
	if err != nil {
		return err
	}
	defer root.Close()
	parts := strings.Split(path.Dir(destination), "/")
	for i := range parts {
		current := strings.Join(parts[:i+1], "/")
		tree, err := durablefs.ObserveTree(s.paths.StorageRoot, current)
		if err != nil {
			return err
		}
		if tree.Present {
			continue
		}
		dir, err := root.Mkdir(current, 0o755)
		if err != nil {
			return err
		}
		if err := root.SyncDir(dir.Path); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) taskReviewPublication(input ReviewPublishInput) (taskReviewPublication, error) {
	var out taskReviewPublication
	if input.TaskID == "" || input.Proposal == "" || input.Destination == "" {
		return out, invalidArgumentsf("task review publication requires task, proposal, and destination")
	}
	if !reviewDigest.MatchString(input.ExpectTaskSHA256) || !reviewDigest.MatchString(input.ExpectSpecSHA256) {
		return out, WithMachineErrorCode(MachineCodeInvalidDigest, fmt.Errorf("expected task and spec digests must be lower-case 64-hex"))
	}
	proposal, err := s.reviewProposalPath(input.Proposal)
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

func (s *Service) reviewProposalPath(proposal string) (string, error) {
	if filepath.IsAbs(proposal) || filepath.ToSlash(proposal) != proposal || path.Clean(proposal) != proposal {
		return "", invalidArgumentsf("proposal must be a canonical repository-relative path")
	}
	artifacts := filepath.ToSlash(relPath(s.paths.RepoRoot, s.paths.ArtifactsDir))
	prefix := path.Join(artifacts, "review-proposals", "task") + "/"
	if !strings.HasPrefix(proposal, prefix) || !isPortableReviewKey(path.Base(proposal)) || strings.Count(strings.TrimPrefix(proposal, prefix), "/") != 0 {
		return "", WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("proposal is outside the task review proposal root"))
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
