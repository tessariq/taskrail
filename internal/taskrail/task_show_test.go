package taskrail

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"strings"
	"testing"
)

func TestTaskShowReadsExactTaskBytesThroughActiveStorage(t *testing.T) {
	committed, committedBefore := storageNeutralService(t, committedStorage(), false)
	local, localBefore := storageNeutralService(t, localStorage(), true)

	committedResult, err := committed.TaskShow("T-001-local")
	if err != nil {
		t.Fatalf("show committed task: %v", err)
	}
	localResult, err := local.TaskShow("T-001-local")
	if err != nil {
		t.Fatalf("show local task: %v", err)
	}
	if committedResult != localResult {
		t.Fatalf("committed/local show differs:\ncommitted: %+v\nlocal: %+v", committedResult, localResult)
	}
	if committedResult.TaskPath != "work/planning/tasks/T-001-local.md" {
		t.Fatalf("task path = %q, want logical path", committedResult.TaskPath)
	}
	if strings.Contains(committedResult.Content, localStorageRoot) || strings.Contains(committedResult.TaskPath, localStorageRoot) {
		t.Fatalf("show leaked physical storage: %+v", committedResult)
	}
	if strings.Contains(localResult.Content, "decoy task") {
		t.Fatalf("show leaked unrelated task content: %+v", localResult)
	}
	wantDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(committedResult.Content)))
	if committedResult.SHA256 != wantDigest {
		t.Fatalf("sha256 = %q, want %q", committedResult.SHA256, wantDigest)
	}
	if got := snapshotTree(t, committed.paths.RepoRoot); !maps.Equal(got, committedBefore) {
		t.Fatal("committed task show changed fixture bytes")
	}
	if got := snapshotTree(t, local.paths.RepoRoot); !maps.Equal(got, localBefore) {
		t.Fatal("local task show changed fixture bytes")
	}
}

func TestTaskShowRequiresExactPersistedID(t *testing.T) {
	svc, _ := storageNeutralService(t, committedStorage(), false)
	for _, id := range []string{"T-001", "T-001-", " T-001-local", "T-001-local "} {
		t.Run(id, func(t *testing.T) {
			_, err := svc.TaskShow(id)
			if err == nil || MachineFailureFor(err).Code != MachineCodeTaskNotFound {
				t.Fatalf("show %q error = %v, want task_not_found", id, err)
			}
		})
	}
}
