package taskrail

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestReviewShowReadsExactDurableBytesThroughActiveStorage(t *testing.T) {
	committed, committedBefore := storageNeutralService(t, committedStorage(), false)
	local, localBefore := storageNeutralService(t, localStorage(), true)
	logicalPath := "work/planning/reviews/spec/v0.5.0/session/report.json"

	committedResult, err := committed.ReviewShow(logicalPath)
	if err != nil {
		t.Fatalf("show committed review: %v", err)
	}
	localResult, err := local.ReviewShow(logicalPath)
	if err != nil {
		t.Fatalf("show local review: %v", err)
	}
	if committedResult != localResult {
		t.Fatalf("committed/local show differs:\ncommitted: %+v\nlocal: %+v", committedResult, localResult)
	}
	if committedResult.Path != logicalPath || committedResult.Content != "review bytes\n" {
		t.Fatalf("review result = %+v", committedResult)
	}
	if committedResult.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(committedResult.Content))) {
		t.Fatalf("sha256 = %q does not match content", committedResult.SHA256)
	}
	if strings.Contains(localResult.Path, localStorageRoot) || strings.Contains(localResult.Content, "decoy review") {
		t.Fatalf("show leaked physical storage: %+v", localResult)
	}
	if got := snapshotTree(t, committed.paths.RepoRoot); !maps.Equal(got, committedBefore) {
		t.Fatal("committed review show changed fixture bytes")
	}
	if got := snapshotTree(t, local.paths.RepoRoot); !maps.Equal(got, localBefore) {
		t.Fatal("local review show changed fixture bytes")
	}
}

func TestReviewShowRejectsNonDurableAndInvalidEntries(t *testing.T) {
	svc, _ := storageNeutralService(t, committedStorage(), false)
	for _, logicalPath := range []string{
		"work/planning/STATE.md",
		"work/planning/artifacts/review-proposals/spec/p-1/report.json",
		"work/planning/tasks/T-001-local.md",
		"work/planning/prompts/v1/task-review.md",
		"product/specs/v0.5.0.md",
		"README.md",
		".taskrail/runtime/transactions/report.json",
		"work/planning/reviews/spec/../task/T-001/session/report.json",
		".taskrail/local/work/planning/reviews/spec/v0.5.0/session/report.json",
		"work/planning/reviews/spec/v0.5.0/session/missing.json",
	} {
		t.Run(logicalPath, func(t *testing.T) {
			_, err := svc.ReviewShow(logicalPath)
			if err == nil {
				t.Fatal("ReviewShow succeeded")
			}
			want := MachineCodePathBlocked
			if strings.HasSuffix(logicalPath, "missing.json") {
				want = MachineCodeReviewNotFound
			}
			if got := MachineFailureFor(err).Code; got != want {
				t.Fatalf("code = %q, want %q (%v)", got, want, err)
			}
		})
	}

	physical := filepath.Join(svc.paths.PlanningDir, "reviews", "task", "T-001", "session", "alias.json")
	if err := os.MkdirAll(filepath.Dir(physical), 0o755); err != nil {
		t.Fatalf("create alias directory: %v", err)
	}
	if err := os.Symlink("../../../../reviews/spec/v0.5.0/session/report.json", physical); err != nil {
		t.Fatalf("create alias: %v", err)
	}
	if _, err := svc.ReviewShow("work/planning/reviews/task/T-001/session/alias.json"); MachineFailureFor(err).Code != MachineCodePathBlocked {
		t.Fatalf("alias error = %v, want path_blocked", err)
	}
	if err := os.Mkdir(filepath.Join(svc.paths.PlanningDir, "reviews", "task", "T-001", "session", "directory.json"), 0o755); err != nil {
		t.Fatalf("create directory entry: %v", err)
	}
	if _, err := svc.ReviewShow("work/planning/reviews/task/T-001/session/directory.json"); MachineFailureFor(err).Code != MachineCodePathBlocked {
		t.Fatalf("directory error = %v, want path_blocked", err)
	}
}

func TestReviewShowRefusesFileReplacedByAliasBetweenStableReads(t *testing.T) {
	svc, _ := storageNeutralService(t, committedStorage(), false)
	logicalPath := "work/planning/reviews/spec/v0.5.0/session/report.json"
	physicalPath := filepath.Join(svc.paths.PlanningDir, "reviews", "spec", "v0.5.0", "session", "report.json")
	outsidePath := filepath.Join(svc.paths.RepoRoot, "outside-review.json")
	writeFile(t, outsidePath, "outside bytes\n")
	testHookReadOnlyRecheck = func() {
		if err := os.Remove(physicalPath); err != nil {
			t.Fatalf("remove review: %v", err)
		}
		if err := os.Symlink(outsidePath, physicalPath); err != nil {
			t.Fatalf("replace review with alias: %v", err)
		}
	}
	t.Cleanup(func() { testHookReadOnlyRecheck = nil })

	if _, err := svc.ReviewShow(logicalPath); MachineFailureFor(err).Code != MachineCodePathBlocked {
		t.Fatalf("replaced review error = %v, want path_blocked", err)
	}
}

func TestReviewShowReportsWriteConflictForChangedBytes(t *testing.T) {
	svc, _ := storageNeutralService(t, committedStorage(), false)
	logicalPath := "work/planning/reviews/spec/v0.5.0/session/report.json"
	physicalPath := filepath.Join(svc.paths.PlanningDir, "reviews", "spec", "v0.5.0", "session", "report.json")
	testHookReadOnlyRecheck = func() { writeFile(t, physicalPath, "changed review bytes\n") }
	t.Cleanup(func() { testHookReadOnlyRecheck = nil })

	if _, err := svc.ReviewShow(logicalPath); MachineFailureFor(err).Code != MachineCodeWriteConflict {
		t.Fatalf("changed review error = %v, want write_conflict", err)
	}
	entry, ok := MachineCommandEntryFor("review show", MachineSurfaceStdout)
	if !ok || !slices.Contains(entry.Errors, MachineCodeWriteConflict) {
		t.Fatalf("review show errors = %+v, want write_conflict", entry.Errors)
	}
}

func TestReviewShowDoesNotRevalidateHistoricalContentAgainstCurrentStateOrPrompt(t *testing.T) {
	svc, _ := storageNeutralService(t, committedStorage(), false)
	state := readFileString(t, svc.paths.StateFile)
	state = strings.ReplaceAll(state, "v0.5.0", "v0.6.0")
	writeFile(t, svc.paths.StateFile, state)
	writeFile(t, filepath.Join(svc.paths.PromptsDir, "v1", "task-review.md"), "changed prompt bytes\n")
	before := snapshotTree(t, svc.paths.RepoRoot)

	result, err := svc.ReviewShow("work/planning/reviews/spec/v0.5.0/session/report.json")
	if err != nil {
		t.Fatalf("show historical review: %v", err)
	}
	if result.Content != "review bytes\n" {
		t.Fatalf("historical content = %q", result.Content)
	}
	if got := snapshotTree(t, svc.paths.RepoRoot); !maps.Equal(got, before) {
		t.Fatal("review show changed repository bytes")
	}
}
