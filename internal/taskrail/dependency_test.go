package taskrail

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func dependencyFixture(t *testing.T) (*Service, string) {
	t.Helper()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001-target", "Target", "todo", "high", "specs/v0.1.0.md#summary", []string{"T-002-first"})
	writeTask(t, repo, "T-002-first", "First", "completed", "medium", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-003-second", "Second", "todo", "medium", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-004-cancelled", "Cancelled", "cancelled", "low", "specs/v0.1.0.md#summary", nil)
	return newTestService(t, repo, time.Date(2026, 8, 14, 16, 0, 0, 0, time.UTC)), repo
}

func TestEditDependencyAddAndRemovePreserveOrderAndOtherTaskBytes(t *testing.T) {
	svc, _ := dependencyFixture(t)
	targetPath := svc.paths.TasksDir + "/T-001-target.md"
	siblingPath := svc.paths.TasksDir + "/T-003-second.md"
	targetBefore := readBytes(t, targetPath)
	siblingBefore := readBytes(t, siblingPath)
	stateBefore := readBytes(t, svc.paths.StateFile)

	preview, err := svc.EditDependency(EditDependencyInput{
		TaskID: "T-001-target", DependencyID: "T-003-second", Operation: DependencyAdd, DryRun: true,
	})
	if err != nil {
		t.Fatalf("preview add: %v", err)
	}
	if preview.Applied || !reflect.DeepEqual(preview.DependenciesBefore, []string{"T-002-first"}) ||
		!reflect.DeepEqual(preview.DependenciesAfter, []string{"T-002-first", "T-003-second"}) {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if got := readBytes(t, targetPath); got != targetBefore {
		t.Fatal("preview changed target bytes")
	}
	if got := readBytes(t, svc.paths.StateFile); got != stateBefore {
		t.Fatal("preview changed state bytes")
	}

	applied, err := svc.EditDependency(EditDependencyInput{
		TaskID: "T-001-target", DependencyID: "T-003-second", Operation: DependencyAdd,
	})
	if err != nil {
		t.Fatalf("apply add: %v", err)
	}
	if !applied.Applied || !reflect.DeepEqual(applied.DependenciesAfter, preview.DependenciesAfter) || applied.Validation == nil || !applied.Validation.Valid {
		t.Fatalf("unexpected apply: %+v", applied)
	}
	targetAfter := readBytes(t, targetPath)
	if !strings.Contains(targetAfter, "  - T-002-first\n  - T-003-second\n") {
		t.Fatalf("add did not append in order:\n%s", targetAfter)
	}
	withoutDependency := strings.Replace(targetAfter, "  - T-003-second\n", "", 1)
	if withoutDependency != targetBefore {
		t.Fatalf("add changed bytes outside the added dependency line\nbefore:\n%s\nafter without edge:\n%s", targetBefore, withoutDependency)
	}
	if got := readBytes(t, siblingPath); got != siblingBefore {
		t.Fatal("add rewrote an unrelated task")
	}
	if got := readBytes(t, svc.paths.StateFile); got == stateBefore || !strings.Contains(got, "## Task Counts") {
		t.Fatal("add did not reproject state")
	}

	removed, err := svc.EditDependency(EditDependencyInput{
		TaskID: "T-001-target", DependencyID: "T-002-first", Operation: DependencyRemove,
	})
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !reflect.DeepEqual(removed.DependenciesBefore, []string{"T-002-first", "T-003-second"}) ||
		!reflect.DeepEqual(removed.DependenciesAfter, []string{"T-003-second"}) {
		t.Fatalf("remove changed the wrong edge: %+v", removed)
	}
	removedBytes := readBytes(t, targetPath)
	if want := strings.Replace(targetAfter, "  - T-002-first\n", "", 1); removedBytes != want {
		t.Fatalf("remove changed bytes outside the removed dependency line\nwant:\n%s\ngot:\n%s", want, removedBytes)
	}
}

func TestEditDependencyHandlesInlineListsWithoutChangingOtherBytes(t *testing.T) {
	svc, _ := dependencyFixture(t)
	path := filepath.Join(svc.paths.TasksDir, "T-001-target.md")
	before := readBytes(t, path)
	block := "dependencies: \n  - T-002-first\n\n"
	inline := "dependencies: [T-002-first]\n"
	writeFile(t, path, strings.Replace(before, block, inline, 1))
	inlineBefore := readBytes(t, path)

	_, err := svc.EditDependency(EditDependencyInput{
		TaskID: "T-001-target", DependencyID: "T-003-second", Operation: DependencyAdd,
	})
	if err != nil {
		t.Fatalf("add to inline list: %v", err)
	}
	after := readBytes(t, path)
	wantField := "dependencies:\n    - T-002-first\n    - T-003-second\n"
	if !strings.Contains(after, wantField) {
		t.Fatalf("inline list did not become one valid dependency field:\n%s", after)
	}
	if strings.Replace(after, wantField, "", 1) != strings.Replace(inlineBefore, inline, "", 1) {
		t.Fatal("inline edit changed bytes outside the dependencies field")
	}
}

func TestEditDependencyRejectsInvalidEdgesWithoutWrites(t *testing.T) {
	tests := []struct {
		name       string
		operation  DependencyOperation
		taskID     string
		dependency string
		prepare    func(*testing.T, *Service, string)
		code       string
	}{
		{name: "fuzzy target", operation: DependencyAdd, taskID: "T-001", dependency: "T-003-second", code: MachineCodeTaskNotFound},
		{name: "fuzzy dependency", operation: DependencyAdd, taskID: "T-001-target", dependency: "T-003", code: MachineCodeTaskNotFound},
		{name: "missing dependency", operation: DependencyAdd, taskID: "T-001-target", dependency: "T-999-missing", code: MachineCodeTaskNotFound},
		{name: "self dependency", operation: DependencyAdd, taskID: "T-001-target", dependency: "T-001-target", code: MachineCodeDependencyCycle},
		{name: "duplicate dependency", operation: DependencyAdd, taskID: "T-001-target", dependency: "T-002-first", code: MachineCodeDependencyExists},
		{name: "cancelled dependency", operation: DependencyAdd, taskID: "T-001-target", dependency: "T-004-cancelled", code: MachineCodeCancelledDependency},
		{name: "absent dependency", operation: DependencyRemove, taskID: "T-001-target", dependency: "T-003-second", code: MachineCodeDependencyAbsent},
		{name: "cycle", operation: DependencyAdd, taskID: "T-001-target", dependency: "T-003-second", code: MachineCodeDependencyCycle, prepare: func(t *testing.T, _ *Service, repo string) {
			writeTask(t, repo, "T-003-second", "Second", "todo", "medium", "specs/v0.1.0.md#summary", []string{"T-001-target"})
		}},
		{name: "completed target", operation: DependencyAdd, taskID: "T-001-target", dependency: "T-003-second", code: MachineCodeInvalidStatus, prepare: func(t *testing.T, _ *Service, repo string) {
			writeTask(t, repo, "T-001-target", "Target", "completed", "high", "specs/v0.1.0.md#summary", []string{"T-002-first"})
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo := dependencyFixture(t)
			if tc.prepare != nil {
				tc.prepare(t, svc, repo)
			}
			before := snapshotTree(t, repo)
			_, err := svc.EditDependency(EditDependencyInput{TaskID: tc.taskID, DependencyID: tc.dependency, Operation: tc.operation})
			if err == nil {
				t.Fatal("expected refusal")
			}
			if got := MachineFailureFor(err).Code; got != tc.code {
				t.Fatalf("error code = %q, want %q: %v", got, tc.code, err)
			}
			if got := snapshotTree(t, repo); !reflect.DeepEqual(got, before) {
				t.Fatal("refusal changed repository bytes")
			}
		})
	}
}

func TestEditDependencySupportsEveryLiveOpenTarget(t *testing.T) {
	for _, status := range []string{"todo", "in_progress", "blocked"} {
		t.Run(status, func(t *testing.T) {
			svc, repo := dependencyFixture(t)
			writeTask(t, repo, "T-001-target", "Target", status, "high", "specs/v0.1.0.md#summary", []string{"T-002-first"})
			state, tasks, err := svc.loadStateAndTasks()
			if err != nil {
				t.Fatal(err)
			}
			if status == "in_progress" {
				state.Frontmatter.CurrentTask = "T-001-target"
				state.Frontmatter.CurrentTaskTitle = "Target"
				state.Frontmatter.StatusSummary = statusSummaryInProgress
			}
			if status == "blocked" {
				state.Frontmatter.Blockers = []string{"T-001-target: waiting"}
				state.Frontmatter.StatusSummary = statusSummaryBlocked
			}
			state.Body = renderStateBody(state.Frontmatter, tasks)
			if err := svc.saveState(state); err != nil {
				t.Fatal(err)
			}
			result, err := svc.EditDependency(EditDependencyInput{TaskID: "T-001-target", DependencyID: "T-003-second", Operation: DependencyAdd, DryRun: true})
			if err != nil || !reflect.DeepEqual(result.DependenciesAfter, []string{"T-002-first", "T-003-second"}) {
				t.Fatalf("%s target: result=%+v err=%v", status, result, err)
			}
		})
	}
}

func TestEditDependencyUsesActiveLocalStorageAndLeavesCommittedDecoysUntouched(t *testing.T) {
	svc, _ := storageNeutralService(t, localStorage(), true)
	svc.paths.ManagedRoot = svc.paths.RepoRoot
	svc.paths.WorktreeRoot = svc.paths.RepoRoot
	svc.paths.GitCommonDir = filepath.Join(svc.paths.RepoRoot, ".git")
	writeFile(t, filepath.Join(svc.paths.TasksDir, "T-002-dependency.md"), `---
id: T-002-dependency
title: Dependency
status: todo
priority: medium
spec_ref: product/specs/v0.5.0.md#local-planning
dependencies: []
updated_at: "2026-08-14T00:00:00Z"
---

# T-002-dependency Dependency
`)
	decoy := filepath.Join(svc.paths.RepoRoot, "work", "planning", "tasks", "T-001-renamed.md")
	decoyBefore := readBytes(t, decoy)

	result, err := svc.EditDependency(EditDependencyInput{
		TaskID: "T-001-local", DependencyID: "T-002-dependency", Operation: DependencyAdd,
	})
	if err != nil {
		t.Fatalf("local dependency add: %v", err)
	}
	if !result.Applied || !reflect.DeepEqual(result.DependenciesAfter, []string{"T-002-dependency"}) {
		t.Fatalf("local result = %+v", result)
	}
	if got := readBytes(t, decoy); got != decoyBefore {
		t.Fatal("local writer changed the committed-storage decoy")
	}
	if got := readBytes(t, filepath.Join(svc.paths.TasksDir, "T-001-local.md")); !strings.Contains(got, "T-002-dependency") {
		t.Fatal("local writer did not update the active overlay")
	}
}

func TestEditDependencyRefusesDelegatedAndUnsupportedStorageWithoutWrites(t *testing.T) {
	t.Run("delegated child", func(t *testing.T) {
		svc, repo := dependencyFixture(t)
		before := snapshotTree(t, repo)
		t.Setenv("TASKRAIL_DELEGATION_TOKEN", "child-token")
		_, err := svc.EditDependency(EditDependencyInput{TaskID: "T-001-target", DependencyID: "T-003-second", Operation: DependencyAdd})
		if err == nil || MachineFailureFor(err).Code != MachineCodeDelegatedRefused {
			t.Fatalf("delegated refusal = %v", err)
		}
		if got := snapshotTree(t, repo); !reflect.DeepEqual(got, before) {
			t.Fatal("delegated refusal changed repository bytes")
		}
	})

	t.Run("unsupported storage", func(t *testing.T) {
		svc, repo := dependencyFixture(t)
		before := snapshotTree(t, repo)
		svc.paths.Storage = StorageContext{Mode: "remote", Root: "remote"}
		_, err := svc.EditDependency(EditDependencyInput{TaskID: "T-001-target", DependencyID: "T-003-second", Operation: DependencyAdd})
		if err == nil || MachineFailureFor(err).Code != MachineCodeUnsupported {
			t.Fatalf("storage refusal = %v", err)
		}
		if got := snapshotTree(t, repo); !reflect.DeepEqual(got, before) {
			t.Fatal("storage refusal changed repository bytes")
		}
	})
}
