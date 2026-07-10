package taskrail

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validateRepo(t *testing.T, repo string) ValidationResult {
	t.Helper()
	svc := newTestService(t, repo, time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC))
	res, err := svc.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	return res
}

func hasViolation(violations []string, substr string) bool {
	for _, v := range violations {
		if strings.Contains(v, substr) {
			return true
		}
	}
	return false
}

func TestValidateCleanLinkagePasses(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "First", "completed", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002", "Second", "todo", "medium", "specs/v0.1.0.md#summary", []string{"T-001"})

	res := validateRepo(t, repo)
	if !res.Valid {
		t.Fatalf("expected valid repo, got %v", res.Violations)
	}
}

func TestValidateFlagsDanglingArtifactInTaskBody(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	content := `---
id: T-001
title: First
status: todo
priority: medium
spec_ref: specs/v0.1.0.md#summary
dependencies: []
updated_at: "2026-03-31T00:00:00Z"
---

# T-001 First

## Verification Notes

See planning/artifacts/verify/T-001/2026/report.md for evidence.
`
	writeFile(t, filepath.Join(repo, "planning", "tasks", "T-001.md"), content)

	res := validateRepo(t, repo)
	if res.Valid {
		t.Fatal("expected invalid repo for dangling artifact reference")
	}
	if !hasViolation(res.Violations, "task T-001 body references gitignored artifact path planning/artifacts/verify/T-001/2026/report.md") {
		t.Fatalf("expected violation naming file and concrete artifact path, got %v", res.Violations)
	}
}

func TestValidateAllowsArtifactContractProse(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	// Directory prefixes and placeholder paths are the contract prose the T-023
	// scrub deliberately kept; they must not be flagged as dangling references.
	content := `---
id: T-001
title: First
status: todo
priority: medium
spec_ref: specs/v0.1.0.md#summary
dependencies: []
updated_at: "2026-03-31T00:00:00Z"
---

# T-001 First

## Verification Notes

- Artifacts live under the gitignored ` + "`planning/artifacts/verify/`" + ` tree.
- Persist manual-test evidence under ` + "`planning/artifacts/manual-test/T-001/<timestamp>/`" + `.
- Older notes referenced ` + "`planning/artifacts/verify/...`" + ` paths.
`
	writeFile(t, filepath.Join(repo, "planning", "tasks", "T-001.md"), content)

	res := validateRepo(t, repo)
	if !res.Valid {
		t.Fatalf("expected contract prose to pass, got %v", res.Violations)
	}
}

func TestValidateFlagsDanglingArtifactInStateField(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "First", "todo", "medium", "specs/v0.1.0.md#summary", nil)
	state := `---
schema_version: 1
updated_at: "2026-03-31T00:00:00Z"
active_spec_version: v0.1.0
active_spec_path: specs/v0.1.0.md
current_task: ""
current_task_title: ""
status_summary: idle
blockers: []
next_action: Start the next task
last_verification_result: pass; see planning/artifacts/verify/T-001/x/report.md
relevant_artifacts: []
continuation_notes:
  - Fixture repo.
---

# STATE
`
	writeFile(t, filepath.Join(repo, "planning", "STATE.md"), state)

	res := validateRepo(t, repo)
	if res.Valid {
		t.Fatal("expected invalid repo for dangling artifact reference in state")
	}
	if !hasViolation(res.Violations, "last_verification_result") {
		t.Fatalf("expected violation naming state field, got %v", res.Violations)
	}
}

func TestValidateFlagsDanglingArtifactInStateRelevantArtifacts(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "First", "todo", "medium", "specs/v0.1.0.md#summary", nil)
	state := `---
schema_version: 1
updated_at: "2026-03-31T00:00:00Z"
active_spec_version: v0.1.0
active_spec_path: specs/v0.1.0.md
current_task: ""
current_task_title: ""
status_summary: idle
blockers: []
next_action: Start the next task
last_verification_result: Not yet run
relevant_artifacts:
  - planning/artifacts/verify/T-001/x/report.json
continuation_notes:
  - Fixture repo.
---

# STATE
`
	writeFile(t, filepath.Join(repo, "planning", "STATE.md"), state)

	res := validateRepo(t, repo)
	if res.Valid {
		t.Fatal("expected invalid repo for dangling artifact reference in relevant_artifacts")
	}
	if !hasViolation(res.Violations, "state relevant_artifacts references gitignored artifact path planning/artifacts/verify/T-001/x/report.json") {
		t.Fatalf("expected violation naming relevant_artifacts field, got %v", res.Violations)
	}
}

func TestValidateDetectsDependencyCycle(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "One", "todo", "medium", "specs/v0.1.0.md#summary", []string{"T-002"})
	writeTask(t, repo, "T-002", "Two", "todo", "medium", "specs/v0.1.0.md#summary", []string{"T-001"})

	res := validateRepo(t, repo)
	if res.Valid {
		t.Fatal("expected invalid repo for dependency cycle")
	}
	if !hasViolation(res.Violations, "cycle") {
		t.Fatalf("expected dependency cycle violation, got %v", res.Violations)
	}
}

func TestValidateDetectsThreeNodeCycleOnce(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	// Cycle: T-001 -> T-003 -> T-002 -> T-001. Reported once regardless of the
	// DFS entry point, exercising the full detection path over three nodes.
	writeTask(t, repo, "T-001", "A", "todo", "medium", "specs/v0.1.0.md#summary", []string{"T-003"})
	writeTask(t, repo, "T-002", "B", "todo", "medium", "specs/v0.1.0.md#summary", []string{"T-001"})
	writeTask(t, repo, "T-003", "C", "todo", "medium", "specs/v0.1.0.md#summary", []string{"T-002"})

	res := validateRepo(t, repo)
	if res.Valid {
		t.Fatal("expected invalid repo for three-node dependency cycle")
	}
	count := 0
	for _, v := range res.Violations {
		if strings.Contains(v, "dependency cycle detected") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one cycle violation, got %d: %v", count, res.Violations)
	}
}

func TestCycleSignatureNormalizesRotation(t *testing.T) {
	t.Parallel()
	// The same directed cycle discovered from different entry points must yield
	// one signature (rotation-to-minimum), so dedup collapses it to a single
	// report. This directly guards the rotation slice math in cycleSignature.
	a := cycleSignature([]string{"T-002", "T-003", "T-001", "T-002"})
	b := cycleSignature([]string{"T-001", "T-002", "T-003", "T-001"})
	if a != b {
		t.Fatalf("rotation not normalized: %q vs %q", a, b)
	}
	if b != "T-001>T-002>T-003" {
		t.Fatalf("unexpected signature %q", b)
	}
}

func TestValidateDetectsDuplicateTaskID(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "One", "todo", "medium", "specs/v0.1.0.md#summary", nil)
	writeFile(t, filepath.Join(repo, "planning", "tasks", "T-001-dup.md"), `---
id: T-001
title: Dup
status: todo
priority: medium
spec_ref: specs/v0.1.0.md#summary
dependencies: []
updated_at: "2026-03-31T00:00:00Z"
---

# T-001 Dup
`)

	res := validateRepo(t, repo)
	if res.Valid {
		t.Fatal("expected invalid repo for duplicate task id")
	}
	if !hasViolation(res.Violations, "duplicate task id") {
		t.Fatalf("expected duplicate task id violation, got %v", res.Violations)
	}
}

// A duplicated id whose two files are both in_progress must count as one task for
// the current_task/in_progress reconciliation: the duplicate is reported as a
// duplicate id, not spuriously re-reported as "multiple in_progress" (which would
// also wrongly suppress repair of the single logical task).
func TestValidateCountsDuplicateInProgressIDOnce(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "One", "in_progress", "medium", "specs/v0.1.0.md#summary", nil)
	writeFile(t, filepath.Join(repo, "planning", "tasks", "T-001-dup.md"), `---
id: T-001
title: Dup
status: in_progress
priority: medium
spec_ref: specs/v0.1.0.md#summary
dependencies: []
updated_at: "2026-03-31T00:00:00Z"
---

# T-001 Dup
`)

	res := validateRepo(t, repo)
	if !hasViolation(res.Violations, "duplicate task id") {
		t.Fatalf("expected duplicate task id violation, got %v", res.Violations)
	}
	if hasViolation(res.Violations, "multiple in_progress") {
		t.Fatalf("duplicate id must not be counted as multiple in_progress, got %v", res.Violations)
	}
}

// Two task files whose ids share the same numeric prefix (T-001 and
// T-001-milestone-v0.1.0) are a collision even though their full id strings
// differ: dependency references and renumbering treat the numeric prefix as the
// task's identity, so validate must reject the pair.
func TestValidateDetectsNumericPrefixCollision(t *testing.T) {
	t.Parallel()
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001", "One", "todo", "medium", "specs/v0.1.0.md#summary", nil)
	writeFile(t, filepath.Join(repo, "planning", "tasks", "T-001-milestone-v0.1.0.md"), `---
id: T-001-milestone-v0.1.0
title: Milestone
status: todo
priority: medium
spec_ref: specs/v0.1.0.md#summary
dependencies: []
updated_at: "2026-03-31T00:00:00Z"
---

# T-001-milestone-v0.1.0 Milestone
`)

	res := validateRepo(t, repo)
	if res.Valid {
		t.Fatal("expected invalid repo for numeric-prefix collision")
	}
	if !hasViolation(res.Violations, "share numeric id prefix") {
		t.Fatalf("expected numeric-prefix collision violation, got %v", res.Violations)
	}
}
