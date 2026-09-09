package taskrail

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Every v0.5 semantic writer refuses a layout-1 repository before it writes a
// byte. The trap this closes is silent: at layout 1 these writers persisted
// loop policy, completion IDs, and the verification tuple, and an older binary
// then rewrote the same task frontmatter from its own typed struct and dropped
// them (specs/v0.5.0.md#layout-compatibility-and-upgrade).
func TestSemanticWritersRefuseLayout1(t *testing.T) {
	t.Parallel()

	writers := map[string]func(*Service) error{
		"next":     func(s *Service) error { _, err := s.Next(); return err },
		"start":    func(s *Service) error { _, err := s.Start("T-001-gate"); return err },
		"complete": func(s *Service) error { _, err := s.Complete("T-001-gate", "done"); return err },
		"block":    func(s *Service) error { _, err := s.Block("T-001-gate", "reason"); return err },
		"unblock":  func(s *Service) error { _, err := s.Unblock("T-001-gate", "reason"); return err },
		"verify": func(s *Service) error {
			_, err := s.Verify(VerifyInput{TaskID: "T-001-gate", Result: "pass", Summary: "checked"})
			return err
		},
		"task release": func(s *Service) error {
			_, err := s.ReleaseTask(ReleaseTaskInput{TaskID: "T-001-gate", Reason: "interrupted"})
			return err
		},
		"task new": func(s *Service) error {
			_, err := s.CreateTask(CreateTaskInput{Title: "New", Priority: "low", SpecRef: "specs/v0.1.0.md#summary"})
			return err
		},
		"task author": func(s *Service) error {
			_, err := s.TaskAuthor(TaskAuthorInput{
				TaskID:       "T-001-gate",
				BodyPath:     "planning/tasks/body.md",
				ExpectSHA256: strings.Repeat("a", 64),
			})
			return err
		},
		"task rename": func(s *Service) error {
			_, err := s.RenameTask(RenameTaskInput{OldID: "T-001-gate", Title: "Renamed", TitleExplicit: true})
			return err
		},
		"task repoint": func(s *Service) error {
			_, err := s.RepointTask(RepointTaskInput{TaskID: "T-001-gate", SpecRef: "specs/v0.1.0.md#summary"})
			return err
		},
		"task dependency add": func(s *Service) error {
			_, err := s.EditDependency(EditDependencyInput{TaskID: "T-001-gate", DependencyID: "T-002-gate", Operation: DependencyAdd})
			return err
		},
		"task dependency remove": func(s *Service) error {
			_, err := s.EditDependency(EditDependencyInput{TaskID: "T-001-gate", DependencyID: "T-002-gate", Operation: DependencyRemove})
			return err
		},
		"task loop allow": func(s *Service) error {
			_, err := s.MutateTaskLoopPolicy(LoopPolicyMutationInput{TaskID: "T-001-gate", Operation: LoopPolicyAllow, Reason: "eligible"})
			return err
		},
		"task loop hold": func(s *Service) error {
			_, err := s.MutateTaskLoopPolicy(LoopPolicyMutationInput{TaskID: "T-001-gate", Operation: LoopPolicyHold, Reason: "on hold"})
			return err
		},
		"spec activate": func(s *Service) error { _, err := s.ActivateSpec("v0.1.0"); return err },
		"spec add":      func(s *Service) error { _, err := s.AddSpec("v0.2.0"); return err },
		"repair":        func(s *Service) error { _, err := s.Repair(RepairInput{Apply: true}); return err },
	}
	for name, write := range writers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repo := seedLayout1Repo(t)
			writeTask(t, repo, "T-001-gate", "Gate task", "todo", "high", "specs/v0.1.0.md#summary", nil)
			writeTask(t, repo, "T-002-gate", "Other task", "todo", "low", "specs/v0.1.0.md#summary", nil)
			before := snapshotTree(t, repo)

			err := write(layout1Service(t, repo))
			if failure := MachineFailureFor(err); failure.Code != MachineCodeIncompatibleLayout {
				t.Fatalf("%s at layout 1 = %v (%s), want incompatible_layout", name, err, failure.Code)
			}
			// The refusal has to be actionable: an operator reading it must know
			// which command moves the repository forward.
			if !strings.Contains(err.Error(), "init --apply --confirm-quiescent") {
				t.Fatalf("%s refusal does not name the upgrade remedy: %v", name, err)
			}
			assertRefusalLeftTreeUnchanged(t, before, snapshotTree(t, repo))
		})
	}
}

// Layout 1 keeps every read-only reporter working, so an un-upgraded repository
// stays fully inspectable while it refuses writes.
func TestReadOnlyCommandsSucceedAtLayout1(t *testing.T) {
	t.Parallel()

	repo := seedLayout1Repo(t)
	writeTask(t, repo, "T-001-gate", "Gate task", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := layout1Service(t, repo)
	before := snapshotTree(t, repo)

	readers := map[string]func() error{
		"validate":    func() error { _, err := svc.Validate(); return err },
		"status":      func() error { _, err := svc.Status(); return err },
		"stats":       func() error { _, err := svc.Stats(); return err },
		"coverage":    func() error { _, err := svc.Coverage(); return err },
		"spec list":   func() error { _, err := svc.SpecList(); return err },
		"spec show":   func() error { _, err := svc.SpecShow("v0.1.0", false); return err },
		"spec diff":   func() error { _, err := svc.SpecDiff("v0.1.0", "v0.1.0"); return err },
		"task show":   func() error { _, err := svc.TaskShow("T-001-gate"); return err },
		"prompt list": func() error { _, err := svc.PromptList(); return err },
		"prompt show": func() error {
			_, err := svc.PromptShow(PromptShowInput{ID: "task-review"})
			return err
		},
		"lock status": func() error { _, err := svc.LockStatus(); return err },
	}
	// The local reporters answer only for an initialized local store, which this
	// committed fixture has none of. They still must not be refused for the
	// layout — the reason has to stay their own.
	for name, read := range map[string]func() error{
		"local status": func() error { _, err := svc.LocalStatus(); return err },
		"local path":   func() error { _, err := svc.LocalPath(); return err },
	} {
		if err := read(); MachineFailureFor(err).Code == MachineCodeIncompatibleLayout {
			t.Fatalf("%s at layout 1 refused for the layout: %v", name, err)
		}
	}
	for name, read := range readers {
		if err := read(); err != nil {
			t.Fatalf("%s at layout 1: %v", name, err)
		}
	}
	assertRefusalLeftTreeUnchanged(t, before, snapshotTree(t, repo))
}

// A fresh init publishes the strict final marker: exactly the five contracted
// keys, and the schema-2 state that layout implies.
func TestFreshInitPublishesTheStrictLayout2Marker(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC))
	result, err := svc.Init(InitInput{})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if result.ToVersion != layout2Version {
		t.Fatalf("to_version = %d, want %d", result.ToVersion, layout2Version)
	}

	marker := readBytes(t, filepath.Join(repo, taskrailConfigDir, taskrailConfigFile))
	want := "layout_version: 2\nspecs_dir: specs\nplanning_dir: planning\nstorage_mode: committed\nimplementation_review_max_rounds: 1\n"
	if marker != want {
		t.Fatalf("fresh marker =\n%s\nwant\n%s", marker, want)
	}
	if state := readStateFile(t, repo); !strings.Contains(state, "schema_version: 2") ||
		strings.Contains(state, "continuation_notes") || strings.Contains(state, "## Notes") {
		t.Fatalf("fresh state is not schema 2:\n%s", state)
	}
	validation, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !validation.Valid {
		t.Fatalf("fresh layout-2 repository is invalid: %v", validation.Violations)
	}
}

// Retrofit adopts a non-standard repository, so it publishes the same strict
// final marker a fresh init does. Publishing a layout below the current one
// would hand the adopter a repository every semantic writer then refuses.
func TestRetrofitPublishesTheStrictLayout2Marker(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	writeFile(t, filepath.Join(repo, "notes", "ideas.md"), "# Roadmap\n\n## Ship it\n\n- Add login\n")
	svc := newTestService(t, repo, time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Retrofit(RetrofitInput{NotesPath: "notes/ideas.md", Apply: true}); err != nil {
		t.Fatalf("retrofit: %v", err)
	}

	marker := readBytes(t, filepath.Join(repo, taskrailConfigDir, taskrailConfigFile))
	want := "layout_version: 2\nspecs_dir: specs\nplanning_dir: planning\nstorage_mode: committed\nimplementation_review_max_rounds: 1\n"
	if marker != want {
		t.Fatalf("retrofit marker =\n%q\nwant\n%q", marker, want)
	}
	if state := readStateFile(t, repo); !strings.Contains(state, "schema_version: 2") {
		t.Fatalf("retrofit state is not schema 2:\n%s", state)
	}
	// A retrofitted repository has to be usable by the writers immediately —
	// including through the same service that just retrofitted it, which must
	// judge the tree by the layout it published rather than the one it replaced.
	if _, err := svc.CreateTask(CreateTaskInput{Title: "Adopt", SpecRef: "specs/v0.1.0.md#summary"}); err != nil {
		t.Fatalf("create task on the retrofitting service: %v", err)
	}
	rediscovered, err := NewService(repo)
	if err != nil {
		t.Fatalf("rediscover retrofitted repository: %v", err)
	}
	if _, err := rediscovered.CreateTask(CreateTaskInput{Title: "Adopt again", SpecRef: "specs/v0.1.0.md#summary"}); err != nil {
		t.Fatalf("create task after rediscovery: %v", err)
	}
}

func assertRefusalLeftTreeUnchanged(t *testing.T, before, after map[string]string) {
	t.Helper()
	if len(addedPaths(before, after)) != 0 {
		t.Fatalf("refused writer added files: %v", addedPaths(before, after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Fatalf("refused writer changed %s", path)
		}
	}
}

// `import --apply` and `review publish` are named writers in the same
// requirement, but neither fits the table above: import gates before it reads
// its draft, and review publish gates only after a real proposal has been
// bound, so each needs its own fixture to reach the gate at all.
func TestImportApplyAndReviewPublishRefuseLayout1(t *testing.T) {
	t.Parallel()

	t.Run("import --apply", func(t *testing.T) {
		t.Parallel()

		repo := seedLayout1Repo(t)

		// A v1 draft takes the legacy apply path and a v2 bundle the reviewed
		// one. Both create tasks and rewrite state, so both have to refuse; the
		// v1 draft here is otherwise valid, so nothing but the layout can be
		// refusing it.
		draft := ImportDraft{
			SchemaVersion: importDraftSchemaVersion,
			Target:        "tasks",
			Source:        "notes.md",
			Tasks:         []TaskDraft{{Key: "alpha", Title: "Alpha task", SpecRef: "specs/v0.1.0.md#summary"}},
		}
		rel := writeDraftFile(t, repo, "planning/imports/draft.json", draft)
		before := snapshotTree(t, repo)

		_, err := layout1Service(t, repo).ApplyImportDraft(ApplyDraftInput{DraftPath: rel})
		if failure := MachineFailureFor(err); failure.Code != MachineCodeIncompatibleLayout {
			t.Fatalf("import --apply at layout 1 = %v (%s), want incompatible_layout", err, failure.Code)
		}
		if !strings.Contains(err.Error(), "init --apply --confirm-quiescent") {
			t.Fatalf("import --apply refusal does not name the upgrade remedy: %v", err)
		}
		assertRefusalLeftTreeUnchanged(t, before, snapshotTree(t, repo))
	})

	t.Run("import --apply reviewed v2 bundle", func(t *testing.T) {
		t.Parallel()

		repo := seedLayout1Repo(t)
		// Only the draft's schema version is needed to route to the reviewed
		// path, which proves the gate refuses before the bundle is decoded.
		writeFile(t, filepath.Join(repo, "planning", "imports", "draft.json"), `{"schema_version":2}`)
		before := snapshotTree(t, repo)

		_, err := layout1Service(t, repo).ApplyImportDraft(ApplyDraftInput{DraftPath: "planning/imports/draft.json"})
		if failure := MachineFailureFor(err); failure.Code != MachineCodeIncompatibleLayout {
			t.Fatalf("reviewed import at layout 1 = %v (%s), want incompatible_layout", err, failure.Code)
		}
		assertRefusalLeftTreeUnchanged(t, before, snapshotTree(t, repo))
	})

	t.Run("review publish workflow", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows reports directory durability as unsupported")
		}
		t.Parallel()

		repo, runGit := workflowEvidenceGitRepo(t)
		seedFixtureTree(t, repo)
		writeFixtureState(t, repo, "v0.5.0", "", "", "idle")
		writeFile(t, markerFile(repo), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n")
		writeFile(t, filepath.Join(repo, ".gitignore"), "planning/artifacts/\n")
		writeFile(t, filepath.Join(repo, "specs", "v0.5.0.md"), workflowSpecText)
		writeFile(t, filepath.Join(repo, "product.txt"), "product\n")
		runGit("add", ".")
		runGit("commit", "-q", "-m", "workflow fixture")

		svc := layout1Service(t, repo)
		subjects, err := CaptureWorkflowSubjects(WorkflowSnapshotContext{
			RepoRoot: repo, SpecPath: "specs/v0.5.0.md", ReviewsRoot: "planning/reviews", ArtifactsDir: "planning/artifacts",
		})
		if err != nil {
			t.Fatal(err)
		}
		review := "planning/artifacts/review-proposals/workflow-adversarial/workflow-1/report.json"
		writeFile(t, filepath.Join(repo, filepath.FromSlash(review)), string(workflowPublicationReport(t, subjects, "workflow-1")))
		input := ReviewPublishInput{
			Type: "workflow", Review: review, Memory: "planning/reviews/workflow-adversarial/INDEX.json",
			Destination: "planning/reviews/workflow-adversarial/runs/v0.5.0/workflow-1.json", Spec: "v0.5.0",
			ExpectSpecSHA256: digestRaw(subjects.Spec), ExpectHead: subjects.TestedHead,
			ExpectProductSHA256: subjects.ProductSHA256, ExpectMemoryAbsent: true,
		}
		if _, err := svc.ReviewPublish(previewOf(input)); err != nil {
			t.Fatalf("workflow review preview at layout 1: %v", err)
		}
		before := snapshotTree(t, repo)

		_, err = svc.ReviewPublish(input)
		if failure := MachineFailureFor(err); failure.Code != MachineCodeIncompatibleLayout {
			t.Fatalf("workflow review publish at layout 1 = %v (%s), want incompatible_layout", err, failure.Code)
		}
		assertRefusalLeftTreeUnchanged(t, before, snapshotTree(t, repo))
	})

	t.Run("review publish", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows reports directory durability as unsupported")
		}
		t.Parallel()

		repo := realGitRepo(t)
		seedFixtureTree(t, repo)
		writeFile(t, markerFile(repo), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n")
		writeFile(t, filepath.Join(repo, ".gitignore"), "planning/artifacts/\n")
		writeTask(t, repo, "T-389-gate", "Gate", "todo", "high", "specs/v0.1.0.md#summary", nil)
		input := layout1ReviewPublishInput(t, repo, "T-389-gate")
		svc := layout1Service(t, repo)

		// The same proposal previews cleanly, so the refusal below is the layout
		// gate rather than an invalid publication.
		if _, err := svc.ReviewPublish(previewOf(input)); err != nil {
			t.Fatalf("review publish preview at layout 1: %v", err)
		}
		before := snapshotTree(t, repo)

		_, err := svc.ReviewPublish(input)
		if failure := MachineFailureFor(err); failure.Code != MachineCodeIncompatibleLayout {
			t.Fatalf("review publish at layout 1 = %v (%s), want incompatible_layout", err, failure.Code)
		}
		if !strings.Contains(err.Error(), "init --apply --confirm-quiescent") {
			t.Fatalf("review publish refusal does not name the upgrade remedy: %v", err)
		}
		assertRefusalLeftTreeUnchanged(t, before, snapshotTree(t, repo))
	})
}

func previewOf(input ReviewPublishInput) ReviewPublishInput {
	preview := input
	preview.DryRun = true
	return preview
}

// layout1ReviewPublishInput writes a well-formed task-review proposal bound to
// the repository's real task and spec bytes, so publication is refused for the
// layout alone.
func layout1ReviewPublishInput(t *testing.T, repo, taskID string) ReviewPublishInput {
	t.Helper()
	taskPath := "planning/tasks/" + taskID + ".md"
	taskBytes, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(taskPath)))
	if err != nil {
		t.Fatal(err)
	}
	specPath := "specs/v0.1.0.md"
	specBytes, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(specPath)))
	if err != nil {
		t.Fatal(err)
	}
	proposal := "planning/artifacts/review-proposals/task/session-389"
	review := `{"schema_version":1,"prompt_id":"task-review","prompt_contract_version":"v1","prompt_template_sha256":"` +
		builtinPromptDigest(t, "task-review") + `","prompt_source":"builtin","session_id":"session-389","task_id":"` +
		taskID + `","task_path":"` + taskPath + `","task_sha256":"` + digestRaw(taskBytes) + `","spec_path":"` +
		specPath + `","spec_sha256":"` + digestRaw(specBytes) +
		`","context_mode":"fresh","generated_at":"2026-08-12T10:00:00Z","findings":[]}`
	writeFile(t, filepath.Join(repo, filepath.FromSlash(proposal), "review.json"), review)
	return ReviewPublishInput{
		Type: "task", Proposal: proposal,
		Destination:      "planning/reviews/task/" + taskID + "/session-389",
		TaskID:           taskID,
		ExpectTaskSHA256: digestRaw(taskBytes),
		ExpectSpecSHA256: digestRaw(specBytes),
	}
}
