package taskrail

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
	review := `{"schema_version":1,"prompt_id":"task-review","prompt_contract_version":"v1","prompt_template_sha256":"` + reviewDigestB + `","prompt_source":"builtin","session_id":"session-1","task_id":"T-215-review","task_path":"` + taskPath + `","task_sha256":"` + digestRaw(taskBytes) + `","spec_path":"` + specPath + `","spec_sha256":"` + digestRaw(specBytes) + `","context_mode":"fresh","generated_at":"2026-08-12T10:00:00Z","findings":[]}`
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

func TestReviewPublishSpecCleansNewParentsAfterLateSnapshotConflict(t *testing.T) {
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
	review := `{"schema_version":1,"prompt_id":"task-review","prompt_contract_version":"v1","prompt_template_sha256":"` + reviewDigestB + `","prompt_source":"builtin","session_id":"session-1","task_id":"T-215-review","task_path":"` + taskPath + `","task_sha256":"` + digestRaw(taskBytes) + `","spec_path":"` + specPath + `","spec_sha256":"` + digestRaw(specBytes) + `","context_mode":"fresh","generated_at":"2026-08-12T10:00:00Z","findings":[]}`
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
