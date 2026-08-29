package taskrail

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/repolock"
)

func TestReviewPublishTaskPreviewAndApplyBindExactBytes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports directory durability as unsupported")
	}
	repo := realGitRepo(t)
	seedFixtureTree(t, repo)
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n")
	writeFile(t, filepath.Join(repo, ".gitignore"), "planning/artifacts/\n")
	writeTask(t, repo, "T-215-review", "Review", "todo", "high", "specs/v0.1.0.md#summary", nil)
	taskPath := "planning/tasks/T-215-review.md"
	taskBytes, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(taskPath)))
	if err != nil {
		t.Fatal(err)
	}
	specPath := "specs/v0.1.0.md"
	specBytes, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(specPath)))
	if err != nil {
		t.Fatal(err)
	}
	proposal := "planning/artifacts/review-proposals/task/session-1"
	review := `{"schema_version":1,"prompt_id":"task-review","prompt_contract_version":"v1","prompt_template_sha256":"` + builtinPromptDigest(t, "task-review") + `","prompt_source":"builtin","session_id":"session-1","task_id":"T-215-review","task_path":"` + taskPath + `","task_sha256":"` + digestRaw(taskBytes) + `","spec_path":"` + specPath + `","spec_sha256":"` + digestRaw(specBytes) + `","context_mode":"fresh","generated_at":"2026-08-12T10:00:00Z","findings":[]}`
	writeFile(t, filepath.Join(repo, filepath.FromSlash(proposal), "review.json"), review)
	svc, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	input := ReviewPublishInput{Type: "task", Proposal: proposal, Destination: "planning/reviews/task/T-215-review/session-1", TaskID: "T-215-review", ExpectTaskSHA256: digestRaw(taskBytes), ExpectSpecSHA256: digestRaw(specBytes)}
	preview, err := svc.ReviewPublish(ReviewPublishInput{Type: input.Type, Proposal: input.Proposal, Destination: input.Destination, TaskID: input.TaskID, ExpectTaskSHA256: input.ExpectTaskSHA256, ExpectSpecSHA256: input.ExpectSpecSHA256, DryRun: true})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Applied || preview.Files[0].SHA256 != digestRaw([]byte(review)) {
		t.Fatalf("preview = %+v", preview)
	}
	if _, err := os.Stat(filepath.Join(repo, "planning", "reviews", "task", "T-215-review", "session-1")); !os.IsNotExist(err) {
		t.Fatalf("preview created destination: %v", err)
	}
	assertDirectReviewLockHeld(t, svc, input)
	applied, err := svc.ReviewPublish(input)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !applied.Applied || applied.Files[0] != preview.Files[0] || len(applied.Subjects) != 2 {
		t.Fatalf("apply = %+v, preview = %+v", applied, preview)
	}
	published, err := os.ReadFile(filepath.Join(repo, "planning", "reviews", "task", "T-215-review", "session-1", "review.json"))
	if err != nil || string(published) != review {
		t.Fatalf("published bytes = %q, err=%v", published, err)
	}
}

func TestReviewPublishSpecPreviewAndApplyBindExactBytes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports directory durability as unsupported")
	}
	repo, svc, input, files := specReviewPublishFixture(t)
	preview, err := svc.ReviewPublish(ReviewPublishInput{
		Type: input.Type, Proposal: input.Proposal, Destination: input.Destination,
		Spec: input.Spec, ExpectSpecSHA256: input.ExpectSpecSHA256, DryRun: true,
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Applied || len(preview.Files) != 5 || len(preview.Subjects) != 1 {
		t.Fatalf("preview = %+v", preview)
	}
	assertDirectReviewLockHeld(t, svc, input)
	if _, err := svc.ReviewPublish(input); err != nil {
		t.Fatalf("apply: %v", err)
	}
	for _, name := range []string{"consistency.json", "gaps.json", "additions.json", "adversarial.json", "manifest.json"} {
		published, err := os.ReadFile(filepath.Join(repo, "planning", "reviews", "spec", "v0.1.0", "session-1", name))
		if err != nil || string(published) != string(files[name]) {
			t.Fatalf("published %s = %q, err=%v", name, published, err)
		}
	}
}

func TestReviewPublishDecompositionPreviewAndApplyBindExactBytes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports directory durability as unsupported")
	}
	repo, svc, input, files := decompositionReviewPublishFixture(t)
	preview, err := svc.ReviewPublish(ReviewPublishInput{
		Type:                   input.Type,
		Proposal:               input.Proposal,
		Destination:            input.Destination,
		Spec:                   input.Spec,
		ExpectSpecSHA256:       input.ExpectSpecSHA256,
		SpecReview:             input.SpecReview,
		ExpectSpecReviewSHA256: input.ExpectSpecReviewSHA256,
		DryRun:                 true,
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Applied || len(preview.Files) != len(files) {
		t.Fatalf("preview = %+v", preview)
	}
	if _, err := os.Stat(filepath.Join(repo, "planning", "reviews", "decomposition", "v0.5.0", "decomposition-1")); !os.IsNotExist(err) {
		t.Fatalf("preview created destination: %v", err)
	}
	assertDirectReviewLockHeld(t, svc, input)
	applied, err := svc.ReviewPublish(input)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !applied.Applied || len(applied.Subjects) != 2 {
		t.Fatalf("apply = %+v", applied)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(repo, "planning", "reviews", "decomposition", "v0.5.0", "decomposition-1", name))
		if err != nil || string(got) != string(want) {
			t.Fatalf("published %s = %q, err=%v", name, got, err)
		}
	}
}

func TestReviewPublishRefusesDelegatedInvocationBeforePublication(t *testing.T) {
	storages := []struct {
		name  string
		setup func(t *testing.T) (*Service, string)
	}{
		{"committed", func(t *testing.T) (*Service, string) {
			repo := realGitRepo(t)
			seedFixtureTree(t, repo)
			svc, err := NewService(repo)
			if err != nil {
				t.Fatal(err)
			}
			return svc, repo
		}},
		{"local", localWriterFixture},
	}
	delegations := []struct {
		name string
		set  func(t *testing.T, svc *Service)
	}{
		{"invalid", func(t *testing.T, _ *Service) { t.Setenv("TASKRAIL_DELEGATION_TOKEN", "child-token") }},
		{"valid", func(t *testing.T, svc *Service) { setValidLoopDelegation(t, svc, "T-215-review") }},
	}
	for _, storage := range storages {
		for _, delegation := range delegations {
			for _, reviewType := range []string{"task", "spec", "decomposition", "workflow"} {
				for _, dryRun := range []bool{true, false} {
					t.Run(storage.name+"/"+delegation.name+"/"+reviewType+"/"+map[bool]string{true: "preview", false: "apply"}[dryRun], func(t *testing.T) {
						svc, repo := storage.setup(t)
						delegation.set(t, svc)
						before := snapshotTree(t, repo)
						lockBefore := snapshotLockFile(t, svc)

						_, err := svc.ReviewPublish(ReviewPublishInput{Type: reviewType, DryRun: dryRun})
						if err == nil || MachineFailureFor(err).Code != MachineCodeDelegatedRefused {
							t.Fatalf("delegated %s review publish = %v, want delegated_write_refused", reviewType, err)
						}
						if got := snapshotTree(t, repo); !mapEqual(got, before) {
							t.Fatalf("delegated %s review publish changed repository bytes", reviewType)
						}
						if got := snapshotLockFile(t, svc); got != lockBefore {
							t.Fatalf("delegated %s review publish changed lock bytes", reviewType)
						}
					})
				}
			}
		}
	}
}

func assertDirectReviewLockHeld(t *testing.T, svc *Service, input ReviewPublishInput) {
	t.Helper()
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: svc.paths.LockRepository(),
		Command:    "test",
		Capability: repolock.Capability{Commands: []string{"test"}},
	})
	if err != nil {
		t.Fatalf("acquire direct test lock: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })
	before := snapshotLockFile(t, svc)
	_, err = svc.ReviewPublish(input)
	if err == nil || MachineFailureFor(err).Code != MachineCodeLockHeld {
		t.Fatalf("contended %s review publish = %v, want lock_held", input.Type, err)
	}
	if got := snapshotLockFile(t, svc); got != before {
		t.Fatalf("contended %s review publish changed lock bytes", input.Type)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("release direct lock: %v", err)
	}
}

func TestReviewPublishDecompositionRequiresCompleteSpecReviewBundle(t *testing.T) {
	repo, svc, input, _ := decompositionReviewPublishFixture(t)
	if err := os.Remove(filepath.Join(repo, "planning", "reviews", "spec", "v0.5.0", "spec-review-1", "gaps.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReviewPublish(input); err == nil {
		t.Fatal("ReviewPublish accepted an incomplete post-spec review bundle")
	}
	assertDecompositionReviewDestinationAbsent(t, repo)
}

func TestReviewPublishDecompositionRejectsUnknownExternalDependency(t *testing.T) {
	repo, svc, input, files := decompositionReviewPublishFixture(t)
	replaceDecomposition(files, "draft.json", "T-240-implement-the-normative-review-schema-decoders", "T-999-missing")
	refreshDecompositionFinalDigests(files)
	writeDecompositionProposalFiles(t, repo, input.Proposal, files, "draft.json", "review-1.json", "manifest.json")

	if _, err := svc.ReviewPublish(input); err == nil {
		t.Fatal("ReviewPublish accepted an unknown external dependency")
	}
	assertDecompositionReviewDestinationAbsent(t, repo)
}

func TestReviewPublishDecompositionRequiresFreshReviewContext(t *testing.T) {
	repo, svc, input, files := decompositionReviewPublishFixture(t)
	files["review-1.json"] = []byte(strings.Replace(string(files["review-1.json"]), `"context_mode":"fresh"`, `"context_mode":"same-context"`, 1))
	replaceManifestDigest(files, "sha256", digestRaw(files["review-1.json"]), 1)
	files["manifest.json"] = []byte(strings.Replace(string(files["manifest.json"]), `"context_mode":"fresh"`, `"context_mode":"same-context"`, 1))
	writeDecompositionProposalFiles(t, repo, input.Proposal, files, "review-1.json", "manifest.json")

	if _, err := svc.ReviewPublish(input); err == nil {
		t.Fatal("ReviewPublish accepted a non-fresh decomposition review")
	}
	assertDecompositionReviewDestinationAbsent(t, repo)
}

func TestReviewPublishDecompositionRejectsDeferredHighSpecReviewFinding(t *testing.T) {
	repo, svc, input, files := decompositionReviewPublishFixture(t)
	manifestPath := filepath.Join(repo, filepath.FromSlash(input.SpecReview))
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	deferred := strings.Replace(string(data),
		`"disposition":"accepted","rationale":"fixed","resulting_spec_ref":"specs/v0.5.0.md#safe-review-artifact-publication"`,
		`"disposition":"deferred","rationale":"later","target_version":"v0.6.0"`, 1)
	writeFile(t, manifestPath, deferred)
	input.ExpectSpecReviewSHA256 = digestRaw([]byte(deferred))
	replaceDecomposition(files, "manifest.json", digestRaw(data), input.ExpectSpecReviewSHA256)
	writeDecompositionProposalFiles(t, repo, input.Proposal, files, "manifest.json")

	if _, err := svc.ReviewPublish(input); err == nil {
		t.Fatal("ReviewPublish accepted a deferred high post-spec review finding")
	}
	assertDecompositionReviewDestinationAbsent(t, repo)
}

func TestReviewPublishSpecRefusesInvalidInputsWithoutPublication(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(string, *ReviewPublishInput)
		code   string
	}{
		{"task flag", func(_ string, input *ReviewPublishInput) { input.TaskID = "T-215-review" }, MachineCodeInvalidArguments},
		{"wrong destination version", func(_ string, input *ReviewPublishInput) {
			input.Destination = "planning/reviews/spec/v0.2.0/session-1"
		}, MachineCodeInvalidProposal},
		{"stale spec", func(repo string, _ *ReviewPublishInput) {
			writeFile(t, filepath.Join(repo, "specs", "v0.1.0.md"), "# changed\n")
		}, MachineCodeSourceChanged},
		{"extra proposal member", func(repo string, _ *ReviewPublishInput) {
			writeFile(t, filepath.Join(repo, "planning", "artifacts", "review-proposals", "spec", "session-1", "extra.json"), "{}")
		}, MachineCodeInvalidProposal},
		{"cross-lens finding namespace", func(repo string, input *ReviewPublishInput) {
			proposal := filepath.Join(repo, filepath.FromSlash(input.Proposal))
			lensPath := filepath.Join(proposal, "gaps.json")
			lens, err := os.ReadFile(lensPath)
			if err != nil {
				t.Fatal(err)
			}
			lens = []byte(strings.Replace(string(lens), `"finding_id":"GAPS-001"`, `"finding_id":"CONS-999"`, 1))
			manifestPath := filepath.Join(proposal, "manifest.json")
			manifest, err := os.ReadFile(manifestPath)
			if err != nil {
				t.Fatal(err)
			}
			manifest = []byte(strings.Replace(string(manifest), `"finding_id":"GAPS-001"`, `"finding_id":"CONS-999"`, 1))
			files := map[string][]byte{"gaps.json": lens, "manifest.json": manifest}
			refreshManifestDigest(files, "gaps.json")
			writeFile(t, lensPath, string(files["gaps.json"]))
			writeFile(t, manifestPath, string(files["manifest.json"]))
		}, MachineCodeInvalidProposal},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, svc, input, _ := specReviewPublishFixture(t)
			test.mutate(repo, &input)
			_, err := svc.ReviewPublish(input)
			if err == nil || MachineFailureFor(err).Code != test.code {
				t.Fatalf("ReviewPublish error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, test.code)
			}
			if _, err := os.Stat(filepath.Join(repo, "planning", "reviews", "spec", "v0.1.0", "session-1")); !os.IsNotExist(err) {
				t.Fatalf("refused input created destination: %v", err)
			}
		})
	}
}

func TestReviewPublishSpecRequiresAcceptedReferencesInSelectedSpec(t *testing.T) {
	for _, test := range []struct {
		name string
		ref  string
	}{
		{"non-spec path", "README.md#summary"},
		{"missing anchor", "specs/v0.1.0.md#missing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, svc, input, _ := specReviewPublishFixture(t)
			manifest := filepath.Join(repo, "planning", "artifacts", "review-proposals", "spec", "session-1", "manifest.json")
			data, err := os.ReadFile(manifest)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, manifest, strings.Replace(string(data), `"resulting_spec_ref":"specs/v0.1.0.md#summary"`, `"resulting_spec_ref":"`+test.ref+`"`, 1))
			_, err = svc.ReviewPublish(input)
			if err == nil || MachineFailureFor(err).Code != MachineCodeInvalidProposal {
				t.Fatalf("ReviewPublish error = %v, code = %q", err, MachineFailureFor(err).Code)
			}
		})
	}
}

func TestReviewPublishSpecPromptBindingPrecedence(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(t *testing.T, repo, proposal string)
		code  string
	}{
		{
			name: "malformed lens binding before invalid replacement",
			setup: func(t *testing.T, repo, proposal string) {
				lensPath := filepath.Join(repo, filepath.FromSlash(proposal), "gaps.json")
				data, err := os.ReadFile(lensPath)
				if err != nil {
					t.Fatal(err)
				}
				data = []byte(strings.Replace(string(data), `"prompt_id":"spec-gaps"`, `"prompt_id":"spec-consistency"`, 1))
				writeFile(t, lensPath, string(data))
				manifestPath := filepath.Join(repo, filepath.FromSlash(proposal), "manifest.json")
				manifest, err := os.ReadFile(manifestPath)
				if err != nil {
					t.Fatal(err)
				}
				files := map[string][]byte{"gaps.json": data, "manifest.json": manifest}
				refreshManifestDigest(files, "gaps.json")
				writeFile(t, manifestPath, string(files["manifest.json"]))
				writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "spec-gaps.md"), "")
			},
			code: MachineCodeInvalidProposal,
		},
		{
			name: "invalid replacement",
			setup: func(t *testing.T, repo, _ string) {
				writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "spec-adversarial.md"), "")
			},
			code: MachineCodePromptInvalid,
		},
		{
			name: "equal byte replacement changes source",
			setup: func(t *testing.T, repo, _ string) {
				writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "spec-consistency.md"), string(builtinPromptTemplate(t, "spec-consistency")))
			},
			code: MachineCodeSourceChanged,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, svc, input, _ := specReviewPublishFixture(t)
			test.setup(t, repo, input.Proposal)
			_, err := svc.ReviewPublish(input)
			if err == nil || MachineFailureFor(err).Code != test.code {
				t.Fatalf("ReviewPublish error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, test.code)
			}
			assertSpecReviewDestinationAbsent(t, repo)
		})
	}
}

func TestReviewPublishSpecRechecksEveryPromptSnapshotBeforeCommit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports directory durability as unsupported")
	}
	for _, lens := range specReviewLensOrder {
		t.Run(lens, func(t *testing.T) {
			repo, svc, input, _ := specReviewPublishFixture(t)
			testHookAfterReviewParent = func() {
				writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "spec-"+lens+".md"), string(builtinPromptTemplate(t, "spec-"+lens)))
			}
			t.Cleanup(func() { testHookAfterReviewParent = nil })
			_, err := svc.ReviewPublish(input)
			if err == nil || MachineFailureFor(err).Code != MachineCodeSourceChanged {
				t.Fatalf("ReviewPublish error = %v, code = %q", err, MachineFailureFor(err).Code)
			}
			assertSpecReviewDestinationAbsent(t, repo)
		})
	}
}

func TestReviewPublishSpecPreservesMixedPromptBindingsInLensFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports directory durability as unsupported")
	}
	repo, svc, input, files := specReviewPublishFixture(t)
	for _, lens := range []string{"gaps", "adversarial"} {
		promptID := "spec-" + lens
		replacement := []byte("replacement " + lens + "\n")
		writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", promptID+".md"), string(replacement))
		name := lens + ".json"
		files[name] = []byte(strings.ReplaceAll(string(files[name]), `"prompt_source":"builtin"`, `"prompt_source":"replacement"`))
		files[name] = []byte(strings.ReplaceAll(string(files[name]), builtinPromptDigest(t, promptID), promptDigest(replacement)))
		refreshManifestDigest(files, name)
		writeFile(t, filepath.Join(repo, filepath.FromSlash(input.Proposal), name), string(files[name]))
	}
	writeFile(t, filepath.Join(repo, filepath.FromSlash(input.Proposal), "manifest.json"), string(files["manifest.json"]))

	if _, err := svc.ReviewPublish(input); err != nil {
		t.Fatalf("ReviewPublish: %v", err)
	}
	for _, name := range []string{"consistency.json", "gaps.json", "additions.json", "adversarial.json"} {
		published, err := os.ReadFile(filepath.Join(repo, "planning", "reviews", "spec", "v0.1.0", "session-1", name))
		if err != nil || string(published) != string(files[name]) {
			t.Fatalf("published %s = %q, err=%v", name, published, err)
		}
	}
	manifest, err := os.ReadFile(filepath.Join(repo, "planning", "reviews", "spec", "v0.1.0", "session-1", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(manifest), "prompt_") {
		t.Fatalf("manifest duplicated prompt binding: %s", manifest)
	}
}

func TestReviewPublishSpecCleansNewParentsAfterLateSnapshotConflict(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports directory durability as unsupported")
	}
	repo, svc, input, _ := specReviewPublishFixture(t)
	testHookAfterReviewParent = func() {
		writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n# changed\n")
	}
	t.Cleanup(func() { testHookAfterReviewParent = nil })
	_, err := svc.ReviewPublish(input)
	if err == nil || MachineFailureFor(err).Code != MachineCodeWriteConflict {
		t.Fatalf("ReviewPublish error = %v, code = %q", err, MachineFailureFor(err).Code)
	}
	if _, err := os.Stat(filepath.Join(repo, "planning", "reviews")); !os.IsNotExist(err) {
		t.Fatalf("late conflict left review parent: %v", err)
	}
}

func TestReviewPublishDecompositionAcceptsTwoConsecutivePasses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports directory durability as unsupported")
	}
	repo, svc, input, files := decompositionReviewPublishFixture(t)
	spec, err := os.ReadFile(filepath.Join(repo, "specs", "v0.5.0.md"))
	if err != nil {
		t.Fatal(err)
	}
	addSecondDecompositionPass(files, DecompositionSubjects{Spec: spec})
	for _, name := range []string{"review-2.json", "manifest.json"} {
		writeFile(t, filepath.Join(repo, filepath.FromSlash(input.Proposal), name), string(files[name]))
	}
	result, err := svc.ReviewPublish(input)
	if err != nil {
		t.Fatalf("apply two passes: %v", err)
	}
	if len(result.Files) != 5 {
		t.Fatalf("published files = %+v", result.Files)
	}
	published, err := os.ReadFile(filepath.Join(repo, "planning", "reviews", "decomposition", "v0.5.0", "decomposition-1", "review-2.json"))
	if err != nil || string(published) != string(files["review-2.json"]) {
		t.Fatalf("published second pass = %q, err=%v", published, err)
	}
}

func TestReviewPublishDecompositionRequiresExactPromptBindingFields(t *testing.T) {
	for _, test := range []struct {
		name     string
		oldValue string
		newValue string
		code     string
		pass     int
	}{
		{"role", `"prompt_id":"task-decomposition-adversarial"`, `"prompt_id":"task-review"`, MachineCodeInvalidProposal, 0},
		{"contract", `"prompt_contract_version":"v1"`, `"prompt_contract_version":"v2"`, MachineCodeInvalidProposal, 0},
		{"template digest", builtinPromptDigest(t, "task-decomposition-adversarial"), reviewDigestA, MachineCodeSourceChanged, 0},
		{"source", `"prompt_source":"builtin"`, `"prompt_source":"replacement"`, MachineCodeSourceChanged, 0},
		{"second pass source", `"prompt_source":"builtin"`, `"prompt_source":"replacement"`, MachineCodeSourceChanged, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, svc, input, files := decompositionReviewPublishFixture(t)
			pass := test.pass
			if pass == 0 {
				pass = 1
			}
			if pass == 2 {
				addSecondDecompositionPublishPass(t, repo, input, files)
			}
			name := fmt.Sprintf("review-%d.json", pass)
			files[name] = []byte(strings.Replace(string(files[name]), test.oldValue, test.newValue, 1))
			replaceManifestDigest(files, "sha256", digestRaw(files[name]), pass)
			writeDecompositionProposalFiles(t, repo, input.Proposal, files, name, "manifest.json")

			_, err := svc.ReviewPublish(input)
			if err == nil || MachineFailureFor(err).Code != test.code {
				t.Fatalf("ReviewPublish error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, test.code)
			}
			assertDecompositionReviewDestinationAbsent(t, repo)
		})
	}
}

func TestReviewPublishDecompositionPromptBindingPrecedence(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(t *testing.T, repo string, input ReviewPublishInput, files map[string][]byte)
		code  string
	}{
		{
			name: "malformed binding before invalid replacement",
			setup: func(t *testing.T, repo string, input ReviewPublishInput, files map[string][]byte) {
				files["review-1.json"] = []byte(strings.Replace(string(files["review-1.json"]), `"prompt_id":"task-decomposition-adversarial"`, `"prompt_id":"task-review"`, 1))
				replaceManifestDigest(files, "sha256", digestRaw(files["review-1.json"]), 1)
				writeDecompositionProposalFiles(t, repo, input.Proposal, files, "review-1.json", "manifest.json")
				writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-decomposition-adversarial.md"), "")
			},
			code: MachineCodeInvalidProposal,
		},
		{
			name: "invalid replacement",
			setup: func(t *testing.T, repo string, _ ReviewPublishInput, _ map[string][]byte) {
				writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-decomposition-adversarial.md"), "")
			},
			code: MachineCodePromptInvalid,
		},
		{
			name: "equal byte replacement changes source",
			setup: func(t *testing.T, repo string, _ ReviewPublishInput, _ map[string][]byte) {
				writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-decomposition-adversarial.md"), string(builtinPromptTemplate(t, "task-decomposition-adversarial")))
			},
			code: MachineCodeSourceChanged,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, svc, input, files := decompositionReviewPublishFixture(t)
			test.setup(t, repo, input, files)
			_, err := svc.ReviewPublish(input)
			if err == nil || MachineFailureFor(err).Code != test.code {
				t.Fatalf("ReviewPublish error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, test.code)
			}
			assertDecompositionReviewDestinationAbsent(t, repo)
		})
	}
}

func TestReviewPublishDecompositionPreservesReplacementPromptBindings(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports directory durability as unsupported")
	}
	for _, twoPasses := range []bool{false, true} {
		name := "one pass"
		if twoPasses {
			name = "two passes"
		}
		t.Run(name, func(t *testing.T) {
			repo, svc, input, files := decompositionReviewPublishFixture(t)
			if twoPasses {
				addSecondDecompositionPublishPass(t, repo, input, files)
			}
			replacement := []byte("replacement decomposition review prompt\n")
			writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-decomposition-adversarial.md"), string(replacement))
			reviewFiles := []string{"review-1.json"}
			if twoPasses {
				reviewFiles = append(reviewFiles, "review-2.json")
			}
			for pass, name := range reviewFiles {
				files[name] = []byte(strings.ReplaceAll(string(files[name]), `"prompt_source":"builtin"`, `"prompt_source":"replacement"`))
				files[name] = []byte(strings.ReplaceAll(string(files[name]), builtinPromptDigest(t, "task-decomposition-adversarial"), promptDigest(replacement)))
				replaceManifestDigest(files, "sha256", digestRaw(files[name]), pass+1)
			}
			writeDecompositionProposalFiles(t, repo, input.Proposal, files, append(reviewFiles, "manifest.json")...)

			if _, err := svc.ReviewPublish(input); err != nil {
				t.Fatalf("ReviewPublish: %v", err)
			}
			for _, name := range reviewFiles {
				published, err := os.ReadFile(filepath.Join(repo, "planning", "reviews", "decomposition", "v0.5.0", "decomposition-1", name))
				if err != nil || string(published) != string(files[name]) {
					t.Fatalf("published %s = %q, err=%v", name, published, err)
				}
			}
			manifest, err := os.ReadFile(filepath.Join(repo, "planning", "reviews", "decomposition", "v0.5.0", "decomposition-1", "manifest.json"))
			if err != nil || strings.Contains(string(manifest), "prompt_") {
				t.Fatalf("published manifest = %q, err=%v", manifest, err)
			}
		})
	}
}

func TestReviewShowDecompositionDoesNotRevalidateHistoricalPrompt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports directory durability as unsupported")
	}
	repo, svc, input, files := decompositionReviewPublishFixture(t)
	if _, err := svc.ReviewPublish(input); err != nil {
		t.Fatalf("ReviewPublish: %v", err)
	}
	writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-decomposition-adversarial.md"), "changed prompt bytes\n")

	result, err := svc.ReviewShow(input.Destination + "/review-1.json")
	if err != nil {
		t.Fatalf("ReviewShow: %v", err)
	}
	if result.Content != string(files["review-1.json"]) {
		t.Fatalf("historical content = %q", result.Content)
	}
}

func TestReviewPublishDecompositionRechecksPromptAndConfigBeforeCommit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports directory durability as unsupported")
	}
	for _, test := range []struct {
		name  string
		setup func(t *testing.T, repo string)
		code  string
	}{
		{
			name: "prompt source transition",
			setup: func(t *testing.T, repo string) {
				writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-decomposition-adversarial.md"), string(builtinPromptTemplate(t, "task-decomposition-adversarial")))
			},
			code: MachineCodeSourceChanged,
		},
		{
			name: "configuration bytes",
			setup: func(t *testing.T, repo string) {
				writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n# changed\n")
			},
			code: MachineCodeWriteConflict,
		},
	} {
		for _, twoPasses := range []bool{false, true} {
			name := "one pass"
			if twoPasses {
				name = "two passes"
			}
			t.Run(test.name+"/"+name, func(t *testing.T) {
				repo, svc, input, files := decompositionReviewPublishFixture(t)
				if twoPasses {
					addSecondDecompositionPublishPass(t, repo, input, files)
				}
				testHookBeforeDecompositionReviewCommit = func() { test.setup(t, repo) }
				t.Cleanup(func() { testHookBeforeDecompositionReviewCommit = nil })
				_, err := svc.ReviewPublish(input)
				if err == nil || MachineFailureFor(err).Code != test.code {
					t.Fatalf("ReviewPublish error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, test.code)
				}
				assertDecompositionReviewDestinationAbsent(t, repo)
			})
		}
	}
}

func TestReviewPublishRejectsCrossTypeFlags(t *testing.T) {
	for _, input := range []ReviewPublishInput{
		{Type: "task", Spec: "v0.5.0"},
		{Type: "decomposition", TaskID: "T-215-review"},
		{Type: "decomposition", TaskFlagsProvided: true},
		{Type: "task", DecompositionFlagsProvided: true},
	} {
		if err := validateReviewPublishInput(input); MachineFailureFor(err).Code != MachineCodeInvalidArguments {
			t.Fatalf("validateReviewPublishInput(%+v) error = %v", input, err)
		}
	}
}

func TestReviewPublishTaskRefusesChangedSubjectAndLeavesDestinationAbsent(t *testing.T) {
	repo, svc, input := reviewPublishFixture(t)
	writeFile(t, filepath.Join(repo, "specs", "v0.1.0.md"), "# changed\n")
	_, err := svc.ReviewPublish(input)
	if err == nil || MachineFailureFor(err).Code != MachineCodeSourceChanged {
		t.Fatalf("ReviewPublish error = %v, code = %q", err, MachineFailureFor(err).Code)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "planning", "reviews", "task", "T-215-review", "session-1")); !os.IsNotExist(statErr) {
		t.Fatalf("changed subject created destination: %v", statErr)
	}
}

func TestReviewPublishTaskRefusesChangedPromptTemplateWithoutPublication(t *testing.T) {
	repo, svc, input := reviewPublishFixture(t)
	proposalFile := filepath.Join(repo, filepath.FromSlash(input.Proposal), "review.json")
	data, err := os.ReadFile(proposalFile)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, proposalFile, strings.Replace(string(data), builtinPromptDigest(t, "task-review"), reviewDigestA, 1))
	_, err = svc.ReviewPublish(input)
	if err == nil || MachineFailureFor(err).Code != MachineCodeSourceChanged {
		t.Fatalf("ReviewPublish error = %v, code = %q", err, MachineFailureFor(err).Code)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "planning", "reviews", "task", "T-215-review", "session-1")); !os.IsNotExist(statErr) {
		t.Fatalf("changed prompt binding created destination: %v", statErr)
	}
}

func TestReviewPublishTaskRequiresExactPromptBindingFields(t *testing.T) {
	for _, test := range []struct {
		name     string
		oldValue string
		newValue string
		code     string
	}{
		{"role", `"prompt_id":"task-review"`, `"prompt_id":"spec-gaps"`, MachineCodeInvalidProposal},
		{"contract", `"prompt_contract_version":"v1"`, `"prompt_contract_version":"v2"`, MachineCodeInvalidProposal},
		{"template digest", builtinPromptDigest(t, "task-review"), reviewDigestA, MachineCodeSourceChanged},
		{"source", `"prompt_source":"builtin"`, `"prompt_source":"replacement"`, MachineCodeSourceChanged},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, svc, input := reviewPublishFixture(t)
			proposalFile := filepath.Join(repo, filepath.FromSlash(input.Proposal), "review.json")
			data, err := os.ReadFile(proposalFile)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, proposalFile, strings.Replace(string(data), test.oldValue, test.newValue, 1))

			_, err = svc.ReviewPublish(input)
			if err == nil || MachineFailureFor(err).Code != test.code {
				t.Fatalf("ReviewPublish error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, test.code)
			}
			assertTaskReviewDestinationAbsent(t, repo)
		})
	}
}

func TestReviewPublishTaskPromptBindingPrecedence(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(t *testing.T, repo, proposal string)
		code  string
	}{
		{
			name: "malformed binding before invalid replacement",
			setup: func(t *testing.T, repo, proposal string) {
				proposalFile := filepath.Join(repo, filepath.FromSlash(proposal), "review.json")
				data, err := os.ReadFile(proposalFile)
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, proposalFile, strings.Replace(string(data), `"prompt_id":"task-review"`, `"prompt_id":"spec-gaps"`, 1))
				writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-review.md"), "")
			},
			code: MachineCodeInvalidProposal,
		},
		{
			name: "invalid replacement",
			setup: func(t *testing.T, repo, _ string) {
				writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-review.md"), "")
			},
			code: MachineCodePromptInvalid,
		},
		{
			name: "equal byte replacement changes source",
			setup: func(t *testing.T, repo, _ string) {
				writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-review.md"), string(builtinPromptTemplate(t, "task-review")))
			},
			code: MachineCodeSourceChanged,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, svc, input := reviewPublishFixture(t)
			test.setup(t, repo, input.Proposal)
			_, err := svc.ReviewPublish(input)
			if err == nil || MachineFailureFor(err).Code != test.code {
				t.Fatalf("ReviewPublish error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, test.code)
			}
			assertTaskReviewDestinationAbsent(t, repo)
		})
	}
}

func TestReviewPublishTaskRechecksBoundSnapshotsBeforeCommit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports directory durability as unsupported")
	}
	for _, test := range []struct {
		name  string
		setup func(t *testing.T, repo, proposal string)
		race  func(t *testing.T, repo string)
		code  string
	}{
		{
			name: "source transition",
			race: func(t *testing.T, repo string) {
				writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-review.md"), string(builtinPromptTemplate(t, "task-review")))
			},
			code: MachineCodeSourceChanged,
		},
		{
			name: "replacement bytes",
			setup: func(t *testing.T, repo, proposal string) {
				first := []byte("first replacement\n")
				writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-review.md"), string(first))
				proposalFile := filepath.Join(repo, filepath.FromSlash(proposal), "review.json")
				data, err := os.ReadFile(proposalFile)
				if err != nil {
					t.Fatal(err)
				}
				data = []byte(strings.Replace(string(data), `"prompt_source":"builtin"`, `"prompt_source":"replacement"`, 1))
				writeFile(t, proposalFile, strings.Replace(string(data), builtinPromptDigest(t, "task-review"), promptDigest(first), 1))
			},
			race: func(t *testing.T, repo string) {
				writeFile(t, filepath.Join(repo, ".taskrail", "prompts", "v1", "task-review.md"), "second replacement\n")
			},
			code: MachineCodeSourceChanged,
		},
		{
			name: "configuration bytes",
			race: func(t *testing.T, repo string) {
				writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n# changed\n")
			},
			code: MachineCodeWriteConflict,
		},
		{
			name: "proposal bytes",
			race: func(t *testing.T, repo string) {
				proposalFile := filepath.Join(repo, "planning", "artifacts", "review-proposals", "task", "session-1", "review.json")
				data, err := os.ReadFile(proposalFile)
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, proposalFile, strings.Replace(string(data), `"context_mode":"fresh"`, `"context_mode":"same-context"`, 1))
			},
			code: MachineCodeWriteConflict,
		},
		{
			name: "task bytes",
			race: func(t *testing.T, repo string) {
				path := filepath.Join(repo, "planning", "tasks", "T-215-review.md")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, path, string(data)+"\nchanged task bytes\n")
			},
			code: MachineCodeSourceChanged,
		},
		{
			name: "spec bytes",
			race: func(t *testing.T, repo string) {
				writeFile(t, filepath.Join(repo, "specs", "v0.1.0.md"), "# changed\n")
			},
			code: MachineCodeSourceChanged,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, svc, input := reviewPublishFixture(t)
			if test.setup != nil {
				test.setup(t, repo, input.Proposal)
			}
			testHookBeforeTaskReviewCommit = func() { test.race(t, repo) }
			t.Cleanup(func() { testHookBeforeTaskReviewCommit = nil })
			_, err := svc.ReviewPublish(input)
			if err == nil || MachineFailureFor(err).Code != test.code {
				t.Fatalf("ReviewPublish error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, test.code)
			}
			assertTaskReviewDestinationAbsent(t, repo)
		})
	}
}

func TestResolveReviewPromptBindingRejectsUnregisteredRole(t *testing.T) {
	_, svc, _ := reviewPublishFixture(t)
	_, err := svc.resolveReviewPromptBinding(ReviewPromptBinding{
		PromptID:              "task-implementation",
		PromptContractVersion: "v1",
		PromptTemplateSHA256:  builtinPromptDigest(t, "task-implementation"),
		PromptSource:          "builtin",
	})
	if err == nil || MachineFailureFor(err).Code != MachineCodeInvalidProposal {
		t.Fatalf("resolveReviewPromptBinding error = %v, code = %q", err, MachineFailureFor(err).Code)
	}
}

func TestReviewPublishTaskRefusesExistingDestinationWithoutClobbering(t *testing.T) {
	repo, svc, input := reviewPublishFixture(t)
	destination := filepath.Join(repo, "planning", "reviews", "task", "T-215-review", "session-1")
	writeFile(t, filepath.Join(destination, "sentinel"), "existing")
	_, err := svc.ReviewPublish(input)
	if err == nil || MachineFailureFor(err).Code != MachineCodeDestinationExists {
		t.Fatalf("ReviewPublish error = %v, code = %q", err, MachineFailureFor(err).Code)
	}
	got, readErr := os.ReadFile(filepath.Join(destination, "sentinel"))
	if readErr != nil || string(got) != "existing" {
		t.Fatalf("existing destination = %q, err=%v", got, readErr)
	}
}

func TestReviewPublishTaskPreviewRefusesExistingDestination(t *testing.T) {
	repo, svc, input := reviewPublishFixture(t)
	writeFile(t, filepath.Join(repo, "planning", "reviews", "task", "T-215-review", "session-1", "sentinel"), "existing")
	input.DryRun = true
	_, err := svc.ReviewPublish(input)
	if err == nil || MachineFailureFor(err).Code != MachineCodeDestinationExists {
		t.Fatalf("ReviewPublish preview error = %v, code = %q", err, MachineFailureFor(err).Code)
	}
}

func TestReviewPublishTaskRejectsProposalIdentityBeforeOccupiedDestination(t *testing.T) {
	repo, svc, input := reviewPublishFixture(t)
	proposalFile := filepath.Join(repo, filepath.FromSlash(input.Proposal), "review.json")
	data, err := os.ReadFile(proposalFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proposalFile, []byte(strings.Replace(string(data), `"session_id":"session-1"`, `"session_id":"other-session"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "planning", "reviews", "task", "T-215-review", "session-1", "sentinel"), "existing")
	_, err = svc.ReviewPublish(input)
	if err == nil || MachineFailureFor(err).Code != MachineCodeInvalidProposal {
		t.Fatalf("ReviewPublish error = %v, code = %q", err, MachineFailureFor(err).Code)
	}
}

func reviewPublishFixture(t *testing.T) (string, *Service, ReviewPublishInput) {
	t.Helper()
	repo := realGitRepo(t)
	seedFixtureTree(t, repo)
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n")
	writeFile(t, filepath.Join(repo, ".gitignore"), "planning/artifacts/\n")
	writeTask(t, repo, "T-215-review", "Review", "todo", "high", "specs/v0.1.0.md#summary", nil)
	taskPath := "planning/tasks/T-215-review.md"
	taskBytes, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(taskPath)))
	if err != nil {
		t.Fatal(err)
	}
	specPath := "specs/v0.1.0.md"
	specBytes, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(specPath)))
	if err != nil {
		t.Fatal(err)
	}
	proposal := "planning/artifacts/review-proposals/task/session-1"
	review := `{"schema_version":1,"prompt_id":"task-review","prompt_contract_version":"v1","prompt_template_sha256":"` + builtinPromptDigest(t, "task-review") + `","prompt_source":"builtin","session_id":"session-1","task_id":"T-215-review","task_path":"` + taskPath + `","task_sha256":"` + digestRaw(taskBytes) + `","spec_path":"` + specPath + `","spec_sha256":"` + digestRaw(specBytes) + `","context_mode":"fresh","generated_at":"2026-08-12T10:00:00Z","findings":[]}`
	writeFile(t, filepath.Join(repo, filepath.FromSlash(proposal), "review.json"), review)
	svc, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	return repo, svc, ReviewPublishInput{Type: "task", Proposal: proposal, Destination: "planning/reviews/task/T-215-review/session-1", TaskID: "T-215-review", ExpectTaskSHA256: digestRaw(taskBytes), ExpectSpecSHA256: digestRaw(specBytes)}
}

func specReviewPublishFixture(t *testing.T) (string, *Service, ReviewPublishInput, map[string][]byte) {
	t.Helper()
	repo := realGitRepo(t)
	seedFixtureTree(t, repo)
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n")
	writeFile(t, filepath.Join(repo, ".gitignore"), "planning/artifacts/\n")
	specPath := "specs/v0.1.0.md"
	specBytes, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(specPath)))
	if err != nil {
		t.Fatal(err)
	}
	files := specReviewGolden()
	for _, lens := range specReviewLensOrder {
		files[lens+".json"] = replacePromptBindingDigest(t, files[lens+".json"], "spec-"+lens)
	}
	for name, content := range files {
		content = []byte(strings.ReplaceAll(string(content), reviewDigestA, digestRaw(specBytes)))
		content = []byte(strings.ReplaceAll(string(content), "specs/v0.5.0.md", specPath))
		content = []byte(strings.ReplaceAll(string(content), "#safe-review-artifact-publication", "#summary"))
		content = []byte(strings.ReplaceAll(string(content), "spec-review-1", "session-1"))
		files[name] = content
	}
	for _, name := range []string{"consistency.json", "gaps.json", "additions.json", "adversarial.json"} {
		refreshManifestDigest(files, name)
		writeFile(t, filepath.Join(repo, "planning", "artifacts", "review-proposals", "spec", "session-1", name), string(files[name]))
	}
	writeFile(t, filepath.Join(repo, "planning", "artifacts", "review-proposals", "spec", "session-1", "manifest.json"), string(files["manifest.json"]))
	svc, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	return repo, svc, ReviewPublishInput{Type: "spec", Proposal: "planning/artifacts/review-proposals/spec/session-1", Destination: "planning/reviews/spec/v0.1.0/session-1", Spec: "v0.1.0", ExpectSpecSHA256: digestRaw(specBytes)}, files
}

func decompositionReviewPublishFixture(t *testing.T) (string, *Service, ReviewPublishInput, map[string][]byte) {
	t.Helper()
	repo := realGitRepo(t)
	seedFixtureTree(t, repo)
	writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n")
	writeFile(t, filepath.Join(repo, ".gitignore"), "planning/artifacts/\n")
	files, subjects := decompositionGolden()
	files["review-1.json"] = replacePromptBindingDigest(t, files["review-1.json"], "task-decomposition-adversarial")
	replaceManifestDigest(files, "sha256", digestRaw(files["review-1.json"]), 1)
	writeFile(t, filepath.Join(repo, "specs", "v0.5.0.md"), string(subjects.Spec))
	for name, content := range subjects.SpecReviewFiles {
		writeFile(t, filepath.Join(repo, filepath.Dir(filepath.FromSlash(subjects.SpecReviewManifestPath)), filepath.FromSlash(name)), string(content))
	}
	writeTask(t, repo, "T-240-implement-the-normative-review-schema-decoders", "Review schema decoders", "completed", "high", "specs/v0.5.0.md#first-area", nil)
	proposal := "planning/artifacts/review-proposals/decomposition/decomposition-1"
	for name, content := range files {
		writeFile(t, filepath.Join(repo, filepath.FromSlash(proposal), name), string(content))
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	input := ReviewPublishInput{
		Type:                   "decomposition",
		Proposal:               proposal,
		Destination:            "planning/reviews/decomposition/v0.5.0/decomposition-1",
		Spec:                   "v0.5.0",
		ExpectSpecSHA256:       digestRaw(subjects.Spec),
		SpecReview:             subjects.SpecReviewManifestPath,
		ExpectSpecReviewSHA256: digestRaw(subjects.SpecReviewFiles["manifest.json"]),
	}
	return repo, svc, input, files
}

func addSecondDecompositionPublishPass(t *testing.T, repo string, input ReviewPublishInput, files map[string][]byte) {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join(repo, "specs", "v0.5.0.md"))
	if err != nil {
		t.Fatal(err)
	}
	addSecondDecompositionPass(files, DecompositionSubjects{Spec: spec})
	writeDecompositionProposalFiles(t, repo, input.Proposal, files, "review-2.json", "manifest.json")
}

func writeDecompositionProposalFiles(t *testing.T, repo, proposal string, files map[string][]byte, names ...string) {
	t.Helper()
	for _, name := range names {
		writeFile(t, filepath.Join(repo, filepath.FromSlash(proposal), name), string(files[name]))
	}
}

func builtinPromptDigest(t *testing.T, id string) string {
	t.Helper()
	definition, err := promptDefinitionFor(id, "v1")
	if err != nil {
		t.Fatal(err)
	}
	data, err := builtinPrompts.ReadFile(definition.asset)
	if err != nil {
		t.Fatal(err)
	}
	return promptDigest(data)
}

func replacePromptBindingDigest(t *testing.T, data []byte, id string) []byte {
	t.Helper()
	return []byte(strings.Replace(string(data), reviewDigestB, builtinPromptDigest(t, id), 1))
}

func builtinPromptTemplate(t *testing.T, id string) []byte {
	t.Helper()
	definition, err := promptDefinitionFor(id, "v1")
	if err != nil {
		t.Fatal(err)
	}
	data, err := builtinPrompts.ReadFile(definition.asset)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertTaskReviewDestinationAbsent(t *testing.T, repo string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(repo, "planning", "reviews", "task", "T-215-review", "session-1")); !os.IsNotExist(err) {
		t.Fatalf("prompt binding rejection created destination: %v", err)
	}
}

func assertSpecReviewDestinationAbsent(t *testing.T, repo string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(repo, "planning", "reviews", "spec", "v0.1.0", "session-1")); !os.IsNotExist(err) {
		t.Fatalf("prompt binding rejection created destination: %v", err)
	}
}

func assertDecompositionReviewDestinationAbsent(t *testing.T, repo string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(repo, "planning", "reviews", "decomposition", "v0.5.0", "decomposition-1")); !os.IsNotExist(err) {
		t.Fatalf("prompt binding rejection created destination: %v", err)
	}
}
