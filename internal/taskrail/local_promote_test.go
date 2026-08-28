package taskrail

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tessariq/taskrail/internal/durabletx"
)

func TestLocalPromotePreviewsAndPublishesSemanticStateAtomically(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)

	setup := newTestService(t, repo, time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
	if _, err := setup.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	local, err := NewService(repo)
	if err != nil {
		t.Fatalf("discover local storage: %v", err)
	}
	created, err := local.CreateTask(CreateTaskInput{Title: "Promoted task", SpecRef: "specs/v0.1.0.md#summary"})
	if err != nil {
		t.Fatalf("create local task: %v", err)
	}
	taskBefore := readFileString(t, filepath.Join(local.paths.TasksDir, created.TaskID+".md"))
	writeFile(t, filepath.Join(local.paths.PromptsDir, "v1", "task.md"), "local prompt\n")
	writeFile(t, filepath.Join(local.paths.ArtifactsDir, "keep.txt"), "local artifact\n")

	semanticBefore := snapshotTree(t, local.paths.StorageRoot)
	preview, err := local.LocalPromote(LocalPromoteInput{})
	if err != nil {
		t.Fatalf("preview local promotion: %v", err)
	}
	if preview.Applied || preview.SourceMode != string(StorageLocal) || preview.TargetMode != string(StorageCommitted) || !preview.Validation.Valid {
		t.Fatalf("preview = %+v", preview)
	}
	if got := snapshotTree(t, local.paths.StorageRoot); !reflect.DeepEqual(got, semanticBefore) {
		t.Fatal("preview changed local storage")
	}
	if _, err := os.Stat(filepath.Join(repo, "planning", "STATE.md")); !os.IsNotExist(err) {
		t.Fatalf("preview created committed state: %v", err)
	}

	applied, err := local.LocalPromote(LocalPromoteInput{Apply: true})
	if err != nil {
		t.Fatalf("apply local promotion: %v", err)
	}
	if !applied.Applied || !applied.Validation.Valid {
		t.Fatalf("apply result = %+v", applied)
	}
	committed, err := NewService(repo)
	if err != nil {
		t.Fatalf("discover committed storage: %v", err)
	}
	if committed.paths.Storage.Mode != StorageCommitted {
		t.Fatalf("storage mode = %q, want committed", committed.paths.Storage.Mode)
	}
	if validation, err := committed.Validate(); err != nil || !validation.Valid {
		t.Fatalf("validate promoted state: %+v, %v", validation, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "planning", "tasks", created.TaskID+".md")); err != nil {
		t.Fatalf("promoted task: %v", err)
	}
	if got := readFileString(t, filepath.Join(repo, "planning", "tasks", created.TaskID+".md")); got != taskBefore {
		t.Fatal("promoted task bytes differ from local source")
	}
	if _, err := os.Stat(filepath.Join(repo, ".taskrail", "local", "planning", "artifacts", "keep.txt")); err != nil {
		t.Fatalf("local artifact was not preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".taskrail", "local", "planning", "STATE.md")); !os.IsNotExist(err) {
		t.Fatalf("local semantic state remains after promotion: %v", err)
	}
}

func TestLocalPromotePreservesInstalledSkillsAndTheirExclusions(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	setup := newTestService(t, repo, time.Now())
	installed, err := setup.Init(InitInput{Local: true, WithSkills: true, SkillVersion: "v9.9.9"})
	if err != nil {
		t.Fatalf("init local with skills: %v", err)
	}
	skillPath := installed.Skills[0].Path
	skillBefore := readFileString(t, filepath.Join(repo, filepath.FromSlash(skillPath)))
	local, err := NewService(repo)
	if err != nil {
		t.Fatalf("discover local storage: %v", err)
	}
	result, err := local.LocalPromote(LocalPromoteInput{Apply: true})
	if err != nil {
		t.Fatalf("promote local storage: %v", err)
	}
	if got := readFileString(t, filepath.Join(repo, filepath.FromSlash(skillPath))); got != skillBefore {
		t.Fatal("promotion changed installed skill bytes")
	}
	if len(result.Skills) == 0 {
		t.Fatal("promotion did not report installed skills")
	}
	for _, skill := range result.Skills {
		if skill.Action != "preserve_local" {
			t.Fatalf("skill result = %+v, want preserve_local", skill)
		}
	}
	exclude := readFileString(t, filepath.Join(repo, ".git", "info", "exclude"))
	if !strings.Contains(exclude, filepath.ToSlash(filepath.Dir(skillPath))) || !strings.Contains(exclude, ".taskrail/local/planning/artifacts/") || !strings.Contains(exclude, ".taskrail/local/runtime/") || strings.Contains(exclude, localStorageRoot+"\n") || strings.Contains(exclude, markerRelPath()) {
		t.Fatalf("promotion exclusion result:\n%s", exclude)
	}
}

func TestLocalPromoteSupportsCustomSemanticPaths(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	if _, err := newTestService(t, repo, time.Now()).Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	localRoot := filepath.Join(repo, ".taskrail", "local")
	for _, move := range [][2]string{{"specs", "product/specs"}, {"planning", "work/planning"}} {
		to := filepath.Join(localRoot, move[1])
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			t.Fatalf("make custom parent: %v", err)
		}
		if err := os.Rename(filepath.Join(localRoot, move[0]), to); err != nil {
			t.Fatalf("move local %s: %v", move[0], err)
		}
	}
	writeFile(t, filepath.Join(repo, markerRelPath()), "layout_version: 2\nspecs_dir: product/specs\nplanning_dir: work/planning\nstorage_mode: local\nimplementation_review_max_rounds: 1\n")
	statePath := filepath.Join(localRoot, "work", "planning", "STATE.md")
	writeFile(t, statePath, strings.ReplaceAll(readFileString(t, statePath), "specs/v0.1.0.md", "product/specs/v0.1.0.md"))
	local, err := NewService(repo)
	if err != nil {
		t.Fatalf("discover custom local storage: %v", err)
	}
	writeFile(t, filepath.Join(local.paths.ArtifactsDir, "keep.txt"), "keep local\n")
	if _, err := local.LocalPromote(LocalPromoteInput{Apply: true}); err != nil {
		t.Fatalf("promote custom paths: %v", err)
	}
	committed, err := NewService(repo)
	if err != nil {
		t.Fatalf("discover custom committed storage: %v", err)
	}
	if validation, err := committed.Validate(); err != nil || !validation.Valid {
		t.Fatalf("validate custom promotion: %+v, %v", validation, err)
	}
	if _, err := os.Stat(filepath.Join(localRoot, "work", "planning", "artifacts", "keep.txt")); err != nil {
		t.Fatalf("custom local artifact was not preserved: %v", err)
	}
}

func TestLocalPromoteRefusesStagedConfigMarker(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	if _, err := newTestService(t, repo, time.Now()).Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	if _, err := gitCommand(repo, "add", "-f", markerRelPath()); err != nil {
		t.Fatalf("stage local marker: %v", err)
	}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatalf("discover local storage: %v", err)
	}
	if _, err := svc.LocalPromote(LocalPromoteInput{}); err == nil {
		t.Fatal("staged local marker did not refuse promotion")
	}
}

func TestRecoveredLocalPromotionValidatesCommittedCandidate(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)
	if _, err := newTestService(t, repo, time.Now()).Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	local, err := NewService(repo)
	if err != nil {
		t.Fatalf("discover local storage: %v", err)
	}
	if _, err := local.LocalPromote(LocalPromoteInput{Apply: true}); err != nil {
		t.Fatalf("promote local storage: %v", err)
	}
	validation, err := local.recoveredValidation(durabletx.RecoveryResult{Applied: true, Command: localPromoteCommand, Action: durabletx.AcceptCandidate})
	if err != nil || !validation.Valid {
		t.Fatalf("recovered validation: %+v, %v", validation, err)
	}
}

func TestLocalPromoteRefusesCommittedCollisionAndUnknownLocalEntry(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(t *testing.T, svc *Service, repo string)
	}{
		{
			name: "committed destination collision",
			prepare: func(t *testing.T, _ *Service, repo string) {
				writeFile(t, filepath.Join(repo, "planning", "STATE.md"), "adopter state\n")
			},
		},
		{
			name: "unknown local durable entry",
			prepare: func(t *testing.T, _ *Service, repo string) {
				writeFile(t, filepath.Join(repo, ".taskrail", "local", "unexpected", "entry"), "unknown\n")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			initLocalGitRepo(t, repo)
			requireRecoveryDirectoryDurability(t, repo)
			if _, err := newTestService(t, repo, time.Now()).Init(InitInput{Local: true}); err != nil {
				t.Fatalf("init local: %v", err)
			}
			svc, err := NewService(repo)
			if err != nil {
				t.Fatalf("discover local storage: %v", err)
			}
			tc.prepare(t, svc, repo)
			before := snapshotTree(t, repo)
			if _, err := svc.LocalPromote(LocalPromoteInput{}); err == nil {
				t.Fatal("unsafe promotion preview succeeded")
			}
			if after := snapshotTree(t, repo); !reflect.DeepEqual(after, before) {
				t.Fatal("refused promotion changed repository bytes")
			}
		})
	}
}
