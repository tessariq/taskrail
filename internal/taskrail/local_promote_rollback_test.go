package taskrail

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// installLocalPromotionValidatedHook installs the local promotion
// transaction's post-publication validation hook for one test and guarantees
// its removal, because the hook is package-global. The owning test must not
// run in parallel: the hook fires for every concurrently running promotion
// apply. Run invokes the validator exactly twice per apply — once over the
// preparation evidence and once after every candidate byte has published — and
// the hook fires only in that second, post-publication window.
func installLocalPromotionValidatedHook(t *testing.T, hook func() error) {
	t.Helper()
	testHookLocalPromotionValidated = hook
	t.Cleanup(func() { testHookLocalPromotionValidated = nil })
}

// dirTreeHasEntries reports whether any non-directory entry exists beneath
// root. An absent root and a tree of empty directories both count as empty.
func dirTreeHasEntries(t *testing.T, root string) bool {
	t.Helper()
	if _, err := os.Lstat(root); err != nil {
		if os.IsNotExist(err) {
			return false
		}
		t.Fatalf("inspect %s: %v", root, err)
	}
	found := false
	err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			found = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return found
}

// A deterministic post-publication failure drives the promotion transaction to
// a completed rollback after committed destination roots exist. Local storage
// stays discoverable and usable: a fresh discovery and `local status` succeed,
// the local semantic, artifact, and runtime bytes return byte-for-byte, user
// content stays untouched, and the committed roots the failed attempt left
// hold no Taskrail content.
func TestLocalPromoteRollbackKeepsLocalModeUsable(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	requireRecoveryDirectoryDurability(t, repo)

	setup := newTestService(t, repo, time.Date(2026, 9, 17, 9, 0, 0, 0, time.UTC))
	if _, err := setup.Init(InitInput{Local: true}); err != nil {
		t.Fatalf("init local: %v", err)
	}
	local, err := NewService(repo)
	if err != nil {
		t.Fatalf("discover local storage: %v", err)
	}
	created, err := local.CreateTask(CreateTaskInput{Title: "Rolled back task", SpecRef: "specs/v0.1.0.md#summary"})
	if err != nil {
		t.Fatalf("create local task: %v", err)
	}
	taskPath := filepath.Join(local.paths.TasksDir, created.TaskID+".md")
	if _, err := os.Stat(taskPath); err != nil {
		t.Fatalf("created local task file: %v", err)
	}
	writeFile(t, filepath.Join(local.paths.PromptsDir, "v1", "task.md"), "local prompt\n")
	writeFile(t, filepath.Join(local.paths.ArtifactsDir, "keep.txt"), "local artifact\n")
	markerBefore := readFileString(t, filepath.Join(repo, markerRelPath()))
	excludeBefore := readFileString(t, filepath.Join(repo, ".git", "info", "exclude"))
	userFile := filepath.Join(repo, "user-owned.txt")
	writeFile(t, userFile, "pre-existing user bytes\n")
	externalEdit := filepath.Join(repo, "docs", "external-edit.md")
	writeFile(t, externalEdit, "external edit bytes\n")
	semanticBefore := snapshotTree(t, local.paths.StorageRoot)

	installLocalPromotionValidatedHook(t, func() error {
		for _, committed := range []string{"specs", "planning", filepath.Join(".taskrail", "prompts")} {
			if _, err := os.Stat(filepath.Join(repo, committed)); err != nil {
				return fmt.Errorf("committed destination root %s was not created before validation: %w", committed, err)
			}
		}
		return errors.New("deterministic post-publication promotion failure")
	})

	_, err = local.LocalPromote(LocalPromoteInput{Apply: true})
	if err == nil || !strings.Contains(err.Error(), "deterministic post-publication promotion failure") {
		t.Fatalf("apply failure = %v, want the deterministic post-publication failure", err)
	}

	if got := snapshotTree(t, local.paths.StorageRoot); !reflect.DeepEqual(got, semanticBefore) {
		t.Fatal("rollback did not restore the local semantic, artifact, and runtime bytes")
	}
	if got := readFileString(t, filepath.Join(repo, markerRelPath())); got != markerBefore {
		t.Fatal("rollback did not restore the local layout marker")
	}
	if got := readFileString(t, filepath.Join(repo, ".git", "info", "exclude")); got != excludeBefore {
		t.Fatal("rollback did not restore the Git exclusion store")
	}
	if got := readFileString(t, userFile); got != "pre-existing user bytes\n" {
		t.Fatalf("rollback changed a pre-existing user file: %q", got)
	}
	if got := readFileString(t, externalEdit); got != "external edit bytes\n" {
		t.Fatalf("rollback changed externally edited user content: %q", got)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "taskrail", "transactions")); !os.IsNotExist(err) {
		t.Fatalf("rolled-back promotion retained transaction state: %v", err)
	}
	for _, committed := range []string{"specs", "planning", filepath.Join(".taskrail", "prompts")} {
		if dirTreeHasEntries(t, filepath.Join(repo, committed)) {
			t.Fatalf("committed root %s left by the failed attempt holds Taskrail content", committed)
		}
	}

	fresh, err := NewService(repo)
	if err != nil {
		t.Fatalf("fresh discovery after rolled-back promotion: %v", err)
	}
	if fresh.paths.Storage.Mode != StorageLocal {
		t.Fatalf("storage mode after rollback = %q, want local", fresh.paths.Storage.Mode)
	}
	status, err := fresh.LocalStatus()
	if err != nil {
		t.Fatalf("local status after rolled-back promotion: %v", err)
	}
	if status.Mode != string(StorageLocal) || !status.PromotionReady || len(status.Violations) != 0 {
		t.Fatalf("local status = %+v", status)
	}
	if _, err := fresh.LocalPath(); err != nil {
		t.Fatalf("local path after rolled-back promotion: %v", err)
	}
	if _, err := fresh.LocalPromote(LocalPromoteInput{}); err != nil {
		t.Fatalf("promotion preview after rolled-back promotion: %v", err)
	}
}

// The rollback tolerance must not broaden local discovery: actual committed
// Taskrail content — a regular file beneath a committed logical root — still
// refuses with the mixed-state error without changing repository bytes.
func TestLocalDiscoveryRefusesNonEmptyCommittedRootsAfterRollbackShape(t *testing.T) {
	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "committed spec file", path: filepath.Join("specs", "v0.1.0.md")},
		{name: "committed planning file", path: filepath.Join("planning", "STATE.md")},
		{name: "committed nested planning file", path: filepath.Join("planning", "tasks", "T-001-nested.md")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			initLocalGitRepo(t, repo)
			requireRecoveryDirectoryDurability(t, repo)
			if _, err := newTestService(t, repo, time.Now()).Init(InitInput{Local: true}); err != nil {
				t.Fatalf("init local: %v", err)
			}
			committed := filepath.Join(repo, tc.path)
			writeFile(t, committed, "committed Taskrail content\n")
			before := snapshotTree(t, repo)

			if _, err := NewService(repo); err == nil || !strings.Contains(err.Error(), "mixed committed/local") {
				t.Fatalf("discovery with %s = %v, want mixed committed/local refusal", tc.path, err)
			}
			if got := snapshotTree(t, repo); !reflect.DeepEqual(got, before) {
				t.Fatal("mixed-state refusal changed repository bytes")
			}
		})
	}
}
