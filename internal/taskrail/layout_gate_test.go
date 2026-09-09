package taskrail

import (
	"path/filepath"
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
		"validate":  func() error { _, err := svc.Validate(); return err },
		"status":    func() error { _, err := svc.Status(); return err },
		"stats":     func() error { _, err := svc.Stats(); return err },
		"coverage":  func() error { _, err := svc.Coverage(); return err },
		"spec list": func() error { _, err := svc.SpecList(); return err },
		"task show": func() error { _, err := svc.TaskShow("T-001-gate"); return err },
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
