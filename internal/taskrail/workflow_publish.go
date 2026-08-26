package taskrail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/tessariq/taskrail/internal/durablefs"
	"github.com/tessariq/taskrail/internal/durabletx"
	"github.com/tessariq/taskrail/internal/repolock"
)

const workflowPublicationFence = "workflow review publication in progress\n"

var testHookAfterWorkflowSnapshot func()

// publishWorkflowReview keeps the two workflow outputs behind one durable fence:
// readers either see the prior memory or recover the exact report/index pair.
func (s *Service) publishWorkflowReview(input ReviewPublishInput) (ReviewPublishResult, error) {
	candidate, err := s.workflowReviewPublication(input)
	if err != nil {
		return ReviewPublishResult{}, err
	}
	if input.DryRun {
		return candidate.result(false), nil
	}
	transactionID, err := newMigrationTransactionID()
	if err != nil {
		return ReviewPublishResult{}, err
	}
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: s.paths.LockRepository(), Command: "review publish", TransactionID: transactionID,
		Capability: repolock.Capability{Commands: []string{"review publish"}},
	})
	if err != nil {
		return ReviewPublishResult{}, writerLockError(err)
	}
	defer func() { _ = lock.Release() }()
	if err := s.validateWriterStorage(); err != nil {
		return ReviewPublishResult{}, err
	}
	candidate, err = s.workflowReviewPublication(input)
	if err != nil {
		return ReviewPublishResult{}, err
	}
	request := durabletx.Request{
		Command: "review publish",
		Members: []durabletx.Member{
			{Kind: durabletx.Managed, Reported: candidate.memory, Path: candidate.memory, Content: candidate.index, Fence: []byte(workflowPublicationFence)},
			{Kind: durabletx.Managed, Reported: candidate.destination, Path: candidate.destination, Content: candidate.report},
		},
		Validate: candidate.validate,
	}
	if _, err := durabletx.Run(context.Background(), lock, s.paths.LockRepository(), request); err != nil {
		return ReviewPublishResult{}, workflowPublishError(err)
	}
	return candidate.result(true), nil
}

func (s *Service) workflowReviewPublication(input ReviewPublishInput) (workflowReviewPublication, error) {
	var out workflowReviewPublication
	if input.Review == "" || input.Memory == "" || input.Destination == "" || input.Spec == "" || input.ExpectHead == "" || input.ExpectProductSHA256 == "" {
		return out, invalidArgumentsf("workflow review publication requires review, memory, destination, spec, expected HEAD, and expected product digest")
	}
	if input.ExpectMemoryAbsent == (input.ExpectMemorySHA256 != "") {
		return out, invalidArgumentsf("workflow review publication requires exactly one expected-memory value")
	}
	if !workflowObjectID.MatchString(input.ExpectHead) || !reviewDigest.MatchString(input.ExpectProductSHA256) ||
		(!input.ExpectMemoryAbsent && !reviewDigest.MatchString(input.ExpectMemorySHA256)) || !reviewDigest.MatchString(input.ExpectSpecSHA256) {
		return out, WithMachineErrorCode(MachineCodeInvalidDigest, fmt.Errorf("workflow review publication has an invalid expected digest"))
	}
	version, specPath, _, err := s.reviewSpecPath(input.Spec)
	if err != nil {
		return out, err
	}
	review, err := s.workflowReviewPath(input.Review)
	if err != nil {
		return out, err
	}
	if err := s.requireTransientProposal(review); err != nil {
		return out, err
	}
	memory := path.Join(s.paths.LogicalPlanningDir, "reviews", "workflow-adversarial", "INDEX.json")
	if input.Memory != memory {
		return out, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("workflow memory must be %q", memory))
	}
	reportBytes, _, err := durablefs.ReadFile(s.paths.RepoRoot, review, reviewFileLimit)
	if err != nil {
		return out, reviewInputError("read workflow review proposal", err)
	}
	subjects, err := CaptureWorkflowSubjects(WorkflowSnapshotContext{
		RepoRoot: s.paths.RepoRoot, SpecPath: specPath,
		ReviewsRoot:  path.Join(s.paths.LogicalPlanningDir, "reviews"),
		ArtifactsDir: filepath.ToSlash(relPath(s.paths.RepoRoot, s.paths.ArtifactsDir)),
	})
	if err != nil {
		return out, WithMachineErrorCode(MachineCodeSourceChanged, err)
	}
	if testHookAfterWorkflowSnapshot != nil {
		testHookAfterWorkflowSnapshot()
	}
	if subjects.TestedHead != input.ExpectHead || subjects.ProductSHA256 != input.ExpectProductSHA256 || digestRaw(subjects.Spec) != input.ExpectSpecSHA256 {
		return out, WithMachineErrorCode(MachineCodeSourceChanged, fmt.Errorf("workflow review snapshots do not match expected HEAD, product, and spec values"))
	}
	report, err := DecodeWorkflowReport(reportBytes, subjects)
	if err != nil {
		return out, WithMachineErrorCode(MachineCodeInvalidProposal, err)
	}
	prompt, err := s.resolveReviewPromptBinding(report.Binding)
	if err != nil {
		return out, err
	}
	if err := ValidateWorkflowFileEvidence(report, WorkflowEvidenceContext{
		RepoRoot: s.paths.RepoRoot, Storage: s.paths.Storage, PlanningDir: s.paths.LogicalPlanningDir,
		ArtifactsDir: filepath.ToSlash(relPath(s.paths.RepoRoot, s.paths.ArtifactsDir)),
	}); err != nil {
		return out, WithMachineErrorCode(MachineCodeInvalidProposal, err)
	}
	prior, err := s.workflowMemory(memory)
	if err != nil {
		return out, err
	}
	if input.ExpectMemoryAbsent {
		if prior != nil {
			return out, WithMachineErrorCode(MachineCodeSourceChanged, fmt.Errorf("workflow memory exists but absence was expected"))
		}
	} else if prior == nil || digestRaw(prior) != input.ExpectMemorySHA256 {
		return out, WithMachineErrorCode(MachineCodeSourceChanged, fmt.Errorf("workflow memory does not match its expected snapshot"))
	}
	candidate, err := DeriveWorkflowIndex(prior, report)
	if err != nil {
		return out, WithMachineErrorCode(MachineCodeInvalidProposal, err)
	}
	destination := path.Join(s.paths.LogicalPlanningDir, "reviews", "workflow-adversarial", "runs", version, report.ReviewID+".json")
	if input.Destination != destination {
		return out, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("workflow review destination does not match the selected spec and review identity"))
	}
	if err := s.requireAbsentWorkflowDestination(destination); err != nil {
		return out, err
	}
	if err := s.requireUniqueWorkflowReviewID(report.ReviewID); err != nil {
		return out, err
	}
	return workflowReviewPublication{review: review, memory: memory, destination: destination, report: report.Raw,
		index: candidate.Index.Raw, subjects: subjects, reviewID: report.ReviewID, prompt: prompt}, nil
}

func (s *Service) workflowReviewPath(value string) (string, error) {
	if filepath.IsAbs(value) || filepath.ToSlash(value) != value || path.Clean(value) != value {
		return "", invalidArgumentsf("review must be a canonical repository-relative path")
	}
	artifacts := filepath.ToSlash(relPath(s.paths.RepoRoot, s.paths.ArtifactsDir))
	prefix := path.Join(artifacts, "review-proposals", "workflow-adversarial") + "/"
	rel := strings.TrimPrefix(value, prefix)
	if rel == value || path.Base(value) != "report.json" || strings.Count(rel, "/") != 1 || !isPortableReviewKey(path.Dir(rel)) {
		return "", WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("workflow review is not a staged report.json"))
	}
	return value, nil
}

func (s *Service) workflowMemory(logical string) ([]byte, error) {
	data, _, err := durablefs.ReadFile(s.paths.StorageRoot, logical, workflowIndexByteLimit)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, reviewInputError("read workflow memory", err)
	}
	if _, err := DecodeWorkflowIndex(data); err != nil {
		return nil, WithMachineErrorCode(MachineCodeInvalidProposal, fmt.Errorf("decode workflow memory: %w", err))
	}
	return data, nil
}

func (s *Service) requireAbsentWorkflowDestination(destination string) error {
	tree, err := durablefs.ObserveTree(s.paths.StorageRoot, path.Dir(destination))
	if err != nil {
		return reviewInputError("inspect workflow destination", err)
	}
	if tree.Present {
		for _, entry := range tree.Entries {
			if entry.Path == path.Base(destination) {
				return WithMachineErrorCode(MachineCodeDestinationExists, fmt.Errorf("workflow review destination already exists"))
			}
		}
	}
	return nil
}

func (s *Service) requireUniqueWorkflowReviewID(id string) error {
	root := filepath.Join(s.paths.StorageRoot, filepath.FromSlash(path.Join(s.paths.LogicalPlanningDir, "reviews", "workflow-adversarial", "runs")))
	err := filepath.WalkDir(root, func(filename string, entry fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return durablefs.ErrAlias
		}
		if !entry.IsDir() && path.Base(filepath.ToSlash(filename)) == id+".json" {
			return fmt.Errorf("workflow review id %q already exists", id)
		}
		return nil
	})
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "already exists") {
		return WithMachineErrorCode(MachineCodeDestinationExists, err)
	}
	return reviewInputError("inspect workflow review IDs", err)
}

func (p workflowReviewPublication) validate(evidence []durabletx.Evidence) error {
	if len(evidence) != 2 || evidence[0].Reported != p.memory || evidence[1].Reported != p.destination ||
		evidence[0].CandidateSHA256 != digestRaw(p.index) || evidence[1].CandidateSHA256 != digestRaw(p.report) ||
		evidence[0].FenceSHA256 != digestRaw([]byte(workflowPublicationFence)) {
		return fmt.Errorf("workflow publication transaction no longer matches its validated pair")
	}
	if _, err := DecodeWorkflowIndex(p.index); err != nil {
		return err
	}
	return nil
}

func (p workflowReviewPublication) result(applied bool) ReviewPublishResult {
	return ReviewPublishResult{Type: "workflow", Applied: applied, Proposal: p.review, Destination: p.destination,
		Files:      []ReviewPublishFile{{Source: p.review, Destination: p.destination, SHA256: digestRaw(p.report)}, {Source: p.memory, Destination: p.memory, SHA256: digestRaw(p.index)}},
		Subjects:   []ReviewPublishSubject{{Path: p.subjects.SpecPath, SHA256: digestRaw(p.subjects.Spec)}},
		Validation: ValidationResult{Valid: true, Violations: []string{}}, Transaction: nil}
}

// validateWorkflowPublicationRecovery confirms that the retained durable pair
// still names one exact immutable report and one strict final index. The engine
// already proves the complete pair holds candidate bytes before this callback.
func (s *Service) validateWorkflowPublicationRecovery(transactionID string, evidence []durabletx.Evidence) error {
	if len(evidence) != 2 {
		return fmt.Errorf("workflow publication recovery has an unexpected write set")
	}
	memory := path.Join(s.paths.LogicalPlanningDir, "reviews", "workflow-adversarial", "INDEX.json")
	if evidence[0].Kind != durabletx.Managed || evidence[0].Reported != memory || evidence[0].FenceSHA256 == "" ||
		evidence[1].Kind != durabletx.Managed || !strings.HasPrefix(evidence[1].Reported, path.Join(s.paths.LogicalPlanningDir, "reviews", "workflow-adversarial", "runs")+"/") ||
		evidence[0].CandidateSHA256 == "" || evidence[1].CandidateSHA256 == "" {
		return fmt.Errorf("workflow publication recovery does not retain the report/index pair")
	}
	index, err := durabletx.RetainedCandidate(s.paths.LockRepository(), transactionID, durabletx.Managed, memory)
	if err != nil {
		return err
	}
	if digestRaw(index) != evidence[0].CandidateSHA256 {
		return fmt.Errorf("workflow publication retained index digest does not match its candidate")
	}
	if _, err := DecodeWorkflowIndex(index); err != nil {
		return fmt.Errorf("decode retained workflow index: %w", err)
	}
	report, _, err := durablefs.ReadFile(s.paths.StorageRoot, evidence[1].Reported, reviewFileLimit)
	if err != nil || digestRaw(report) != evidence[1].CandidateSHA256 {
		return fmt.Errorf("read retained workflow report: %w", err)
	}
	obj, err := decodeReviewObject(report, "retained workflow report")
	if err != nil {
		return err
	}
	specPath, err := reviewPathMember(obj, "retained workflow report", "spec_path")
	if err != nil {
		return err
	}
	head, err := workflowObjectIDMember(obj, "retained workflow report", "tested_head")
	if err != nil {
		return err
	}
	spec, err := readGitTreeBlob(s.paths.RepoRoot, head, specPath)
	if err != nil {
		return err
	}
	product, err := WorkflowProductSHA256(s.paths.RepoRoot, head, path.Join(s.paths.LogicalPlanningDir, "reviews"))
	if err != nil {
		return err
	}
	decoded, err := DecodeWorkflowReport(report, WorkflowSubjects{SpecPath: specPath, Spec: spec, TestedHead: head,
		ProductSHA256: product, ArtifactsDir: filepath.ToSlash(relPath(s.paths.RepoRoot, s.paths.ArtifactsDir))})
	if err != nil {
		return err
	}
	if decoded.IndexSHA256After != evidence[0].CandidateSHA256 ||
		(decoded.IndexSHA256Before == "absent") != (evidence[0].OriginalSHA256 == "") ||
		(decoded.IndexSHA256Before != "absent" && decoded.IndexSHA256Before != evidence[0].OriginalSHA256) {
		return fmt.Errorf("retained workflow report does not bind the retained memory snapshots")
	}
	version := strings.TrimSuffix(path.Base(specPath), ".md")
	wantReport := path.Join(s.paths.LogicalPlanningDir, "reviews", "workflow-adversarial", "runs", version, decoded.ReviewID+".json")
	if evidence[1].Reported != wantReport {
		return fmt.Errorf("retained workflow report path does not match its spec and review identity")
	}
	prior, present, err := durabletx.RetainedOriginal(s.paths.LockRepository(), transactionID, durabletx.Managed, memory)
	if err != nil {
		return err
	}
	if !present {
		prior = nil
	}
	derived, err := DeriveWorkflowIndex(prior, decoded)
	if err != nil || !bytes.Equal(derived.Index.Raw, index) {
		return fmt.Errorf("retained workflow index is not the report-derived candidate: %w", err)
	}
	return nil
}

func workflowPublishError(err error) error {
	if errors.Is(err, os.ErrExist) {
		return WithMachineErrorCode(MachineCodeDestinationExists, err)
	}
	return reviewPublishError(err)
}
