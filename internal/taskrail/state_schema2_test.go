package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// upgradeToLayout2 publishes the layout-2 migration over a seeded layout-1
// fixture and returns a service discovered against the upgraded marker, which is
// how every ordinary command reaches an upgraded repository.
func upgradeToLayout2(t *testing.T, repo string) *Service {
	t.Helper()
	if _, err := layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true, DropContinuationNotes: true}); err != nil {
		t.Fatalf("upgrade apply: %v", err)
	}
	return newTestService(t, repo, time.Date(2026, 8, 17, 12, 30, 0, 0, time.UTC))
}

func readStateFile(t *testing.T, repo string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, "planning", "STATE.md"))
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	return string(data)
}

func TestStateSchemaFollowsLayout(t *testing.T) {
	t.Parallel()

	t.Run("an upgraded repository validates at schema 2", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		svc := upgradeToLayout2(t, repo)
		state := readStateFile(t, repo)
		if !strings.Contains(state, "schema_version: 2") {
			t.Fatalf("upgraded state is not schema 2:\n%s", state)
		}
		validation, err := svc.Validate()
		if err != nil {
			t.Fatalf("validate: %v", err)
		}
		if !validation.Valid {
			t.Fatalf("upgraded repository is invalid: %v", validation.Violations)
		}
	})

	t.Run("layout-2 lifecycle writers keep schema 2 and stay valid", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		writeTask(t, repo, "T-001-schema", "Schema task", "todo", "high", "specs/v0.1.0.md#summary", nil)
		svc := upgradeToLayout2(t, repo)

		if _, err := svc.Start("T-001-schema"); err != nil {
			t.Fatalf("start: %v", err)
		}
		completed, err := svc.Complete("T-001-schema", "done")
		if err != nil {
			t.Fatalf("complete: %v", err)
		}
		if !completed.Validation.Valid {
			t.Fatalf("complete reported invalid state: %v", completed.Validation.Violations)
		}
		if _, err := svc.Verify(VerifyInput{TaskID: "T-001-schema", Result: "pass", Summary: "verified"}); err != nil {
			t.Fatalf("verify: %v", err)
		}
		state := readStateFile(t, repo)
		if !strings.Contains(state, "schema_version: 2") {
			t.Fatalf("writer rewrote state away from schema 2:\n%s", state)
		}
		if strings.Contains(state, "continuation_notes") || strings.Contains(state, "## Notes") {
			t.Fatalf("schema 2 state carries removed continuation prose:\n%s", state)
		}
		validation, err := svc.Validate()
		if err != nil {
			t.Fatalf("validate: %v", err)
		}
		if !validation.Valid {
			t.Fatalf("post-lifecycle state is invalid: %v", validation.Violations)
		}
	})

	// Layout 1 still reads and validates as schema 1, but no semantic writer may
	// touch it: an older binary would rewrite task frontmatter from its own typed
	// struct and erase whatever v0.5 fields a layout-1 write had introduced
	// (specs/v0.5.0.md#layout-compatibility-and-upgrade).
	t.Run("a layout-1 repository still reads at schema 1 and refuses every writer", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		writeTask(t, repo, "T-001-legacy", "Legacy task", "todo", "high", "specs/v0.1.0.md#summary", nil)
		svc := layout1Service(t, repo)
		before := readStateFile(t, repo)

		_, err := svc.Start("T-001-legacy")
		if failure := MachineFailureFor(err); failure.Code != MachineCodeIncompatibleLayout {
			t.Fatalf("start at layout 1 = %v (%s), want incompatible_layout", err, failure.Code)
		}
		if !strings.Contains(err.Error(), "init --apply --confirm-quiescent") {
			t.Fatalf("refusal does not name the upgrade remedy: %v", err)
		}
		if state := readStateFile(t, repo); state != before {
			t.Fatalf("refused writer changed state:\n%s", state)
		}
		if !strings.Contains(before, "schema_version: 1") || !strings.Contains(before, "continuation_notes") {
			t.Fatalf("layout-1 state lost its schema-1 shape:\n%s", before)
		}

		validation, err := svc.Validate()
		if err != nil {
			t.Fatalf("validate: %v", err)
		}
		if !validation.Valid {
			t.Fatalf("layout-1 repository is invalid: %v", validation.Violations)
		}
	})

	t.Run("schema 2 rejects reintroduced continuation prose", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		svc := upgradeToLayout2(t, repo)
		state := readStateFile(t, repo)
		reintroduced := strings.Replace(state, "relevant_artifacts: []\n", "relevant_artifacts: []\ncontinuation_notes:\n  - Reintroduced.\n", 1)
		reintroduced += "\n## Notes\n\n- Reintroduced.\n"
		writeFile(t, filepath.Join(repo, "planning", "STATE.md"), reintroduced)

		validation, err := svc.Validate()
		if err != nil {
			t.Fatalf("validate: %v", err)
		}
		if validation.Valid {
			t.Fatal("schema 2 accepted the removed continuation_notes field and Notes section")
		}
		joined := strings.Join(validation.Violations, "\n")
		if !strings.Contains(joined, "continuation_notes") || !strings.Contains(joined, "Notes") {
			t.Fatalf("violations do not name the removed field and section: %v", validation.Violations)
		}
	})

	t.Run("a layout-2 repository at schema 1 is reported invalid", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		svc := upgradeToLayout2(t, repo)
		state := readStateFile(t, repo)
		writeFile(t, filepath.Join(repo, "planning", "STATE.md"), strings.Replace(state, "schema_version: 2", "schema_version: 1", 1))
		validation, err := svc.Validate()
		if err != nil {
			t.Fatalf("validate: %v", err)
		}
		if validation.Valid {
			t.Fatal("layout 2 accepted state schema 1")
		}
		if !strings.Contains(strings.Join(validation.Violations, "\n"), "state schema_version must be 2") {
			t.Fatalf("violations = %v, want the layout-implied schema", validation.Violations)
		}
	})
}
