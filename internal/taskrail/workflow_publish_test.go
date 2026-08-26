package taskrail

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/tessariq/taskrail/internal/durabletx"
)

func TestReviewPublishWorkflowPreviewAndApplyBindReportAndMemory(t *testing.T) {
	repo, runGit := workflowEvidenceGitRepo(t)
	seedFixtureTree(t, repo)
	writeFixtureState(t, repo, "v0.5.0", "", "", "idle")
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n")
	writeFile(t, filepath.Join(repo, ".gitignore"), "planning/artifacts/\n")
	writeFile(t, filepath.Join(repo, "specs", "v0.5.0.md"), workflowSpecText)
	writeFile(t, filepath.Join(repo, "product.txt"), "product\n")
	runGit("add", ".")
	runGit("commit", "-q", "-m", "workflow fixture")

	svc, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	subjects, err := CaptureWorkflowSubjects(WorkflowSnapshotContext{
		RepoRoot: repo, SpecPath: "specs/v0.5.0.md", ReviewsRoot: "planning/reviews", ArtifactsDir: "planning/artifacts",
	})
	if err != nil {
		t.Fatal(err)
	}
	report := workflowPublicationReport(t, subjects, "workflow-1")
	review := "planning/artifacts/review-proposals/workflow-adversarial/workflow-1/report.json"
	writeFile(t, filepath.Join(repo, filepath.FromSlash(review)), string(report))
	input := ReviewPublishInput{
		Type: "workflow", Review: review, Memory: "planning/reviews/workflow-adversarial/INDEX.json",
		Destination: "planning/reviews/workflow-adversarial/runs/v0.5.0/workflow-1.json", Spec: "v0.5.0",
		ExpectSpecSHA256: digestRaw(subjects.Spec), ExpectHead: subjects.TestedHead, ExpectProductSHA256: subjects.ProductSHA256,
		ExpectMemoryAbsent: true,
	}
	preview, err := svc.ReviewPublish(ReviewPublishInput{
		Type: input.Type, Review: input.Review, Memory: input.Memory, Destination: input.Destination, Spec: input.Spec,
		ExpectSpecSHA256: input.ExpectSpecSHA256, ExpectHead: input.ExpectHead, ExpectProductSHA256: input.ExpectProductSHA256,
		ExpectMemoryAbsent: true, DryRun: true,
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Applied || len(preview.Files) != 2 {
		t.Fatalf("preview = %+v", preview)
	}
	if _, err := os.Stat(filepath.Join(repo, "planning", "reviews", "workflow-adversarial", "INDEX.json")); !os.IsNotExist(err) {
		t.Fatalf("preview created memory: %v", err)
	}
	requireRecoveryDirectoryDurability(t, repo)
	if _, err := svc.ReviewPublish(input); err != nil {
		t.Fatalf("apply: %v", err)
	}
	published, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(input.Destination)))
	if err != nil || string(published) != string(report) {
		t.Fatalf("published report = %q, err=%v", published, err)
	}
	memory, err := os.ReadFile(filepath.Join(repo, "planning", "reviews", "workflow-adversarial", "INDEX.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeWorkflowIndex(memory); err != nil {
		t.Fatalf("published memory: %v", err)
	}
}

func TestReviewPublishWorkflowRejectsExistingMemoryWhenAbsenceExpected(t *testing.T) {
	repo, runGit := workflowEvidenceGitRepo(t)
	seedFixtureTree(t, repo)
	writeFixtureState(t, repo, "v0.5.0", "", "", "idle")
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n")
	writeFile(t, filepath.Join(repo, ".gitignore"), "planning/artifacts/\n")
	writeFile(t, filepath.Join(repo, "specs", "v0.5.0.md"), workflowSpecText)
	prior, err := EncodeWorkflowIndex(emptyWorkflowIndex())
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "planning", "reviews", "workflow-adversarial", "INDEX.json"), string(prior))
	runGit("add", ".")
	runGit("commit", "-q", "-m", "workflow fixture")
	svc, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	subjects, err := CaptureWorkflowSubjects(WorkflowSnapshotContext{RepoRoot: repo, SpecPath: "specs/v0.5.0.md", ReviewsRoot: "planning/reviews", ArtifactsDir: "planning/artifacts"})
	if err != nil {
		t.Fatal(err)
	}
	review := "planning/artifacts/review-proposals/workflow-adversarial/workflow-1/report.json"
	writeFile(t, filepath.Join(repo, filepath.FromSlash(review)), string(workflowPublicationReportWithPrior(t, subjects, "workflow-1", prior)))
	_, err = svc.ReviewPublish(ReviewPublishInput{
		Type: "workflow", Review: review, Memory: "planning/reviews/workflow-adversarial/INDEX.json",
		Destination: "planning/reviews/workflow-adversarial/runs/v0.5.0/workflow-1.json", Spec: "v0.5.0",
		ExpectSpecSHA256: digestRaw(subjects.Spec), ExpectHead: subjects.TestedHead, ExpectProductSHA256: subjects.ProductSHA256,
		ExpectMemoryAbsent: true,
	})
	if err == nil || MachineFailureFor(err).Code != MachineCodeSourceChanged {
		t.Fatalf("ReviewPublish error = %v, code = %q", err, MachineFailureFor(err).Code)
	}
}

func TestReviewPublishWorkflowRejectsChangedMemorySnapshot(t *testing.T) {
	repo, runGit := workflowEvidenceGitRepo(t)
	seedFixtureTree(t, repo)
	writeFixtureState(t, repo, "v0.5.0", "", "", "idle")
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n")
	writeFile(t, filepath.Join(repo, ".gitignore"), "planning/artifacts/\n")
	writeFile(t, filepath.Join(repo, "specs", "v0.5.0.md"), workflowSpecText)
	prior, err := EncodeWorkflowIndex(emptyWorkflowIndex())
	if err != nil {
		t.Fatal(err)
	}
	memory := "planning/reviews/workflow-adversarial/INDEX.json"
	writeFile(t, filepath.Join(repo, filepath.FromSlash(memory)), string(prior))
	runGit("add", ".")
	runGit("commit", "-q", "-m", "workflow fixture")
	svc, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	subjects, err := CaptureWorkflowSubjects(WorkflowSnapshotContext{RepoRoot: repo, SpecPath: "specs/v0.5.0.md", ReviewsRoot: "planning/reviews", ArtifactsDir: "planning/artifacts"})
	if err != nil {
		t.Fatal(err)
	}
	review := "planning/artifacts/review-proposals/workflow-adversarial/workflow-1/report.json"
	writeFile(t, filepath.Join(repo, filepath.FromSlash(review)), string(workflowPublicationReportWithPrior(t, subjects, "workflow-1", prior)))
	changed := emptyWorkflowIndex()
	changed.NextFindingNumber = 2
	changedBytes, err := EncodeWorkflowIndex(changed)
	if err != nil {
		t.Fatal(err)
	}
	testHookAfterWorkflowSnapshot = func() {
		writeFile(t, filepath.Join(repo, filepath.FromSlash(memory)), string(changedBytes))
	}
	t.Cleanup(func() { testHookAfterWorkflowSnapshot = nil })
	_, err = svc.ReviewPublish(ReviewPublishInput{
		Type: "workflow", Review: review, Memory: memory,
		Destination: "planning/reviews/workflow-adversarial/runs/v0.5.0/workflow-1.json", Spec: "v0.5.0",
		ExpectSpecSHA256: digestRaw(subjects.Spec), ExpectHead: subjects.TestedHead, ExpectProductSHA256: subjects.ProductSHA256,
		ExpectMemorySHA256: digestRaw(prior),
	})
	if err == nil || MachineFailureFor(err).Code != MachineCodeSourceChanged {
		t.Fatalf("ReviewPublish error = %v, code = %q", err, MachineFailureFor(err).Code)
	}
}

func TestRecoverWorkflowPublicationRestoresOrAcceptsTheRetainedPair(t *testing.T) {
	for _, test := range []struct {
		name, phase, action string
	}{
		{"interrupted publish restores", "publishing", "restore_original"},
		{"validated pair accepts", "candidate_published", "accept_candidate"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, runGit := workflowEvidenceGitRepo(t)
			seedFixtureTree(t, repo)
			writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n")
			writeFixtureState(t, repo, "v0.5.0", "", "", "idle")
			writeFile(t, filepath.Join(repo, "specs", "v0.5.0.md"), workflowSpecText)
			writeFile(t, filepath.Join(repo, "product.txt"), "product\n")
			runGit("add", ".")
			runGit("commit", "-q", "-m", "workflow recovery fixture")
			svc, err := NewService(repo)
			if err != nil {
				t.Fatal(err)
			}
			memory := "planning/reviews/workflow-adversarial/INDEX.json"
			report := "planning/reviews/workflow-adversarial/runs/v0.5.0/workflow-1.json"
			subjects, err := CaptureWorkflowSubjects(WorkflowSnapshotContext{RepoRoot: repo, SpecPath: "specs/v0.5.0.md", ReviewsRoot: "planning/reviews", ArtifactsDir: "planning/artifacts"})
			if err != nil {
				t.Fatal(err)
			}
			stagedReport := workflowPublicationReport(t, subjects, "workflow-1")
			decoded, err := DecodeWorkflowReport(stagedReport, subjects)
			if err != nil {
				t.Fatal(err)
			}
			candidate, err := DeriveWorkflowIndex(nil, decoded)
			if err != nil {
				t.Fatal(err)
			}
			index := candidate.Index.Raw
			fabricateRetained(t, svc.paths.LockRepository(), recoverFixtureID, "review publish", test.phase, []recoverMember{
				{kind: durabletx.Managed, reported: memory, path: memory, candidate: index, fence: []byte(workflowPublicationFence), present: true, onDisk: []byte(workflowPublicationFence)},
				{kind: durabletx.Managed, reported: report, path: report, candidate: stagedReport, present: true, onDisk: stagedReport},
			}, "")

			requireRecoveryDirectoryDurability(t, repo)
			result, err := newRecoverService(t, svc.paths).RecoverTransaction(context.Background(), recoverFixtureID, true)
			if err != nil {
				t.Fatalf("RecoverTransaction: %v", err)
			}
			if result.Action != test.action || !result.Applied {
				t.Fatalf("result = %+v", result)
			}
			memoryBytes, memoryErr := os.ReadFile(filepath.Join(repo, filepath.FromSlash(memory)))
			publishedReport, reportErr := os.ReadFile(filepath.Join(repo, filepath.FromSlash(report)))
			if test.action == "restore_original" {
				if !os.IsNotExist(memoryErr) || !os.IsNotExist(reportErr) {
					t.Fatalf("restore retained a partial pair: memory=%v report=%v", memoryErr, reportErr)
				}
				return
			}
			if string(memoryBytes) != string(index) || string(publishedReport) != string(stagedReport) {
				t.Fatalf("accepted pair = memory %q/%v report %q/%v", memoryBytes, memoryErr, publishedReport, reportErr)
			}
		})
	}
}

func workflowPublicationReport(t *testing.T, subjects WorkflowSubjects, id string) []byte {
	return workflowPublicationReportWithPrior(t, subjects, id, nil)
}

func workflowPublicationReportWithPrior(t *testing.T, subjects WorkflowSubjects, id string, prior []byte) []byte {
	t.Helper()
	format := func(after string) []byte {
		before := "absent"
		if prior != nil {
			before = digestRaw(prior)
		}
		return []byte(fmt.Sprintf(`{"schema_version":1,"prompt_id":"workflow-adversarial","prompt_contract_version":"v1","prompt_template_sha256":"%s","prompt_source":"builtin","review_id":"%s","spec_path":"%s","spec_sha256":"%s","tested_head":"%s","product_sha256":"%s","context_mode":"fresh","generated_at":"2026-08-12T10:00:00Z","scope":{"summary":"lifecycle probe","surfaces":[{"surface_key":"lifecycle/transitions","angle":"repeat completion","rationale":"first run","outcome":"finding","evidence_refs":[{"review_id":"%s","probe_id":"probe-1","observation_id":"obs-1"}],"finding_ids":["WF-000001"],"next_angle":"retry after fix"}],"freshness_assessments":[]},"probes":[{"probe_id":"probe-1","surface_keys":["lifecycle/transitions"],"action":"repeat completion","executed":true,"outcome":"fail","observation_ids":["obs-1"],"evidence_refs":[{"review_id":"%s","probe_id":"probe-1","observation_id":"obs-1"}] }],"observations":[{"observation_id":"obs-1","probe_id":"probe-1","expected":"one completion","observed":"two completions","outcome":"supports-finding","evidence":[{"kind":"command","summary":"repeat completion succeeded","path":null,"sha256":null,"command":"taskrail complete T-1","exit_code":0}]}],"findings":[{"finding_id":"WF-000001","severity":"high","evidence_refs":[{"review_id":"%s","probe_id":"probe-1","observation_id":"obs-1"}],"impact":"terminal transition repeats","status":"open","rationale":"observed directly"}],"index_sha256_before":"%s","index_sha256_after":"%s"}`,
			builtinPromptDigest(t, "workflow-adversarial"), id, subjects.SpecPath, digestRaw(subjects.Spec), subjects.TestedHead, subjects.ProductSHA256, id, id, id, before, after))
	}
	report, err := DecodeWorkflowReport(format(reviewDigestA), subjects)
	if err != nil {
		t.Fatal(err)
	}
	index := emptyWorkflowIndex()
	if prior != nil {
		index, err = DecodeWorkflowIndex(prior)
		if err != nil {
			t.Fatal(err)
		}
	}
	candidate, err := buildWorkflowIndexCandidate(index, report)
	if err != nil {
		t.Fatal(err)
	}
	return format(candidate.SHA256)
}
