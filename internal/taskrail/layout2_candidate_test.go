package taskrail

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeLayoutMarkerStrict(t *testing.T) {
	t.Parallel()

	valid := []string{
		"layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n",
		"layout_version: 2\nspecs_dir: specs\nplanning_dir: planning\nstorage_mode: committed\nimplementation_review_max_rounds: 2\n",
		"layout_version: 2\nspecs_dir: specs\nplanning_dir: planning\nstorage_mode: committed\nimplementation_review_max_rounds: 1\nmigration_fence:\n  from_layout_version: 1\n  transaction_id: 0123456789abcdef0123456789abcdef\n",
	}
	for _, input := range valid {
		if _, err := decodeLayoutMarkerStrict([]byte(input)); err != nil {
			t.Errorf("decode valid marker %q: %v", input, err)
		}
	}

	invalid := []string{
		"layout_version: 0\nspecs_dir: specs\nplanning_dir: planning\n",
		"layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\nunknown: true\n",
		"layout_version: 2\nspecs_dir: specs\nplanning_dir: planning\nimplementation_review_max_rounds: 2\n",
		"layout_version: 2\nspecs_dir: specs\nplanning_dir: planning\nstorage_mode: remote\nimplementation_review_max_rounds: 2\n",
		"layout_version: 2\nspecs_dir: specs\nplanning_dir: planning\nstorage_mode: local\nimplementation_review_max_rounds: 0\n",
		"layout_version: 2\nspecs_dir: specs\nplanning_dir: planning\nstorage_mode: local\nimplementation_review_max_rounds: 3\n",
		"layout_version: 2\nspecs_dir: specs\nplanning_dir: planning\nstorage_mode: committed\nimplementation_review_max_rounds: 2\nmigration_fence: null\n",
		"layout_version: 2\nspecs_dir: specs\nplanning_dir: planning\nstorage_mode: local\nimplementation_review_max_rounds: 2\nmigration_fence:\n  from_layout_version: 2\n  transaction_id: 0123456789abcdef0123456789abcdef\n",
		"layout_version: 2\nspecs_dir: specs\nplanning_dir: planning\nstorage_mode: local\nimplementation_review_max_rounds: 2\nmigration_fence:\n  from_layout_version: 1\n  transaction_id: ABCDEF0123456789abcdef0123456789\n",
	}
	for _, input := range invalid {
		if _, err := decodeLayoutMarkerStrict([]byte(input)); err == nil {
			t.Errorf("decode invalid marker %q succeeded", input)
		}
	}
}

func TestDecodeStateV2Strict(t *testing.T) {
	t.Parallel()

	base := `---
schema_version: 2
updated_at: "2026-08-14T00:00:00Z"
active_spec_version: v0.5.0
active_spec_path: specs/v0.5.0.md
current_task: ""
current_task_title: ""
status_summary: idle
blockers: []
next_action: Select the next eligible task
last_verification_result: Not yet run
relevant_artifacts: []
---

# STATE
`
	if _, _, err := decodeStateStrict([]byte(base)); err != nil {
		t.Fatalf("decode fresh schema 2: %v", err)
	}
	canonical := strings.Replace(base,
		"last_verification_result: Not yet run",
		"last_verification_result: pass for T-001-example at 2026-08-14T00:00:00Z id 0123456789abcdef0123456789abcdef\nlast_verification_id: 0123456789abcdef0123456789abcdef\nlast_verification_previous_id: fedcba9876543210fedcba9876543210\nlast_verified_completion_id: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1)
	if _, _, err := decodeStateStrict([]byte(canonical)); err != nil {
		t.Fatalf("decode canonical verification tuple: %v", err)
	}

	invalid := []string{
		strings.Replace(base, "current_task: \"\"", "current_task: null", 1),
		strings.Replace(base, "blockers: []\n", "", 1),
		strings.Replace(base, "relevant_artifacts: []", "relevant_artifacts: []\nunknown: value", 1),
		strings.Replace(base, "relevant_artifacts: []", "relevant_artifacts: []\ncontinuation_notes: []", 1),
		strings.Replace(base, "# STATE", "# STATE\n\n## Notes\n\n- returned", 1),
		strings.Replace(base, "last_verification_result: Not yet run", "last_verification_result: ''", 1),
		strings.Replace(base, "relevant_artifacts: []", "relevant_artifacts: [planning/artifacts/verify/x]", 1),
		strings.Replace(base, "last_verification_result: Not yet run", "last_verification_result: pass for T-001 at 2026-08-14T00:00:00Z id 0123456789abcdef0123456789abcdef\nlast_verification_id: 0123456789abcdef0123456789abcdee", 1),
		strings.Replace(base, "last_verification_result: Not yet run", "last_verification_result: Not yet run\nlast_verification_previous_id: fedcba9876543210fedcba9876543210", 1),
		strings.Replace(base, "last_verification_result: Not yet run", "last_verification_result: pass for T-001 at 2026-08-14T00:00:00Z id 0123456789abcdef0123456789abcdef\nlast_verification_id: 0123456789abcdef0123456789abcdef\nlast_verification_previous_id: 0123456789abcdef0123456789abcdef", 1),
	}
	for _, input := range invalid {
		if _, _, err := decodeStateStrict([]byte(input)); err == nil {
			t.Errorf("decode invalid schema 2 state succeeded:\n%s", input)
		}
	}
}

func TestDecodeMigrationTaskStrict(t *testing.T) {
	t.Parallel()

	base := `---
id: T-001-example
title: Example
status: todo
priority: high
spec_ref: specs/v0.5.0.md#summary
dependencies: []
updated_at: "2026-08-14T00:00:00Z"
---

# T-001-example Example
`
	if _, err := decodeMigrationTaskStrict([]byte(base)); err != nil {
		t.Fatalf("decode implicit hold: %v", err)
	}
	explicit := strings.Replace(base, "updated_at:", "loop_policy: allow\nloop_reason: bounded unattended work\nupdated_at:", 1)
	got, err := decodeMigrationTaskStrict([]byte(explicit))
	if err != nil {
		t.Fatalf("decode explicit pair: %v", err)
	}
	if ResolveLoopPolicy(got.Frontmatter.LoopPolicyMetadata).Policy != "allow" {
		t.Fatalf("explicit policy not preserved: %+v", got.Frontmatter.LoopPolicyMetadata)
	}

	invalid := []string{
		strings.Replace(base, "updated_at:", "loop_policy: allow\nupdated_at:", 1),
		strings.Replace(base, "updated_at:", "loop_policy: ALLOW\nloop_reason: bounded\nupdated_at:", 1),
		strings.Replace(base, "updated_at:", "loop_policy: allow\nloop_reason: bounded\nunknown: value\nupdated_at:", 1),
		strings.Replace(base, "# T-001-example", "loop_policy: allow\n# T-001-example", 1),
	}
	for _, input := range invalid {
		if _, err := decodeMigrationTaskStrict([]byte(input)); err == nil {
			t.Errorf("decode invalid task succeeded:\n%s", input)
		}
	}
}

func TestBuildLayout2MigrationCandidateIsCompleteAndWriteFree(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeFile(t, markerFile(repo), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n")
	writeTask(t, repo, "T-001-example", "Example", "todo", "high", "specs/v0.1.0.md#summary", nil)
	taskPath := filepath.Join(repo, "planning", "tasks", "T-001-example.md")
	taskBytes, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	taskBytes = bytes.Replace(taskBytes, []byte("updated_at:"), []byte("loop_policy: hold\nloop_reason: operator review required\nupdated_at:"), 1)
	if err := os.WriteFile(taskPath, taskBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := packagedSkillFiles()
	if err != nil {
		t.Fatal(err)
	}
	packageBytes, err := shippableSkillsFS.ReadFile(filepath.ToSlash(filepath.Join(shippableSkillsRoot, files[0])))
	if err != nil {
		t.Fatal(err)
	}
	parityPath := filepath.Join(repo, shippableSkillTargets[0], filepath.FromSlash(files[0]))
	if err := os.MkdirAll(filepath.Dir(parityPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parityPath, packageBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	stamped, err := stampSkillVersion(packageBytes, "v0.4.0")
	if err != nil {
		t.Fatal(err)
	}
	installedPath := filepath.Join(repo, shippableSkillTargets[1], filepath.FromSlash(files[0]))
	if err := os.MkdirAll(filepath.Dir(installedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installedPath, stamped, 0o644); err != nil {
		t.Fatal(err)
	}

	before := treeDigest(t, repo)
	candidate, err := buildLayout2MigrationCandidate(repo)
	if err != nil {
		t.Fatalf("build candidate: %v", err)
	}
	after := treeDigest(t, repo)
	if before != after {
		t.Fatal("candidate construction changed repository bytes")
	}
	if candidate.Marker.StorageMode != StorageCommitted || candidate.Marker.ImplementationReviewMaxRounds != 1 {
		t.Fatalf("marker candidate = %+v", candidate.Marker)
	}
	if candidate.MarkerPath != ".taskrail/config.yml" || candidate.StatePath != "planning/STATE.md" || candidate.NotesPath != "planning/NOTES.md" {
		t.Fatalf("candidate paths = marker:%q state:%q notes:%q", candidate.MarkerPath, candidate.StatePath, candidate.NotesPath)
	}
	if !bytes.Contains(candidate.MarkerBytes, []byte("layout_version: 2\n")) {
		t.Fatalf("marker bytes = %q", candidate.MarkerBytes)
	}
	if !bytes.Contains(candidate.MarkerBytes, []byte("implementation_review_max_rounds: 1\n")) {
		t.Fatalf("marker bytes do not contain the default review maximum:\n%s", candidate.MarkerBytes)
	}
	if bytes.Contains(candidate.StateBytes, []byte("continuation_notes")) || bytes.Contains(candidate.StateBytes, []byte("## Notes")) {
		t.Fatalf("schema 2 state retained notes:\n%s", candidate.StateBytes)
	}
	if !bytes.Contains(candidate.StateBytes, []byte("schema_version: 2\n")) {
		t.Fatalf("state candidate is not schema 2:\n%s", candidate.StateBytes)
	}
	if bytes.Contains(candidate.StateBytes, []byte("last_verification_id")) {
		t.Fatal("schema 1 migration invented verification identity")
	}
	if got := candidate.TaskBytes[filepath.ToSlash("planning/tasks/T-001-example.md")]; !bytes.Equal(got, taskBytes) {
		t.Fatal("task candidate did not preserve exact task bytes")
	}
	if len(candidate.ContinuationNotes) != 1 || candidate.ContinuationNotes[0] != "Fixture repo." || len(candidate.NotesExtractionBytes) == 0 {
		t.Fatalf("note candidate = %#v, extraction bytes %d", candidate.ContinuationNotes, len(candidate.NotesExtractionBytes))
	}
	if got := skillOutcome(candidate.Skills, filepath.ToSlash(filepath.Join(shippableSkillTargets[0], files[0]))); got != migrationSkillParity {
		t.Fatalf("parity skill outcome = %q", got)
	}
	if got := skillOutcome(candidate.Skills, filepath.ToSlash(filepath.Join(shippableSkillTargets[1], files[0]))); got != migrationSkillRefresh {
		t.Fatalf("installed skill outcome = %q", got)
	}
	for _, skill := range candidate.Skills {
		if skill.Path == filepath.ToSlash(filepath.Join(shippableSkillTargets[1], files[0])) && (skill.Marker != "nested" || skill.Version != "v0.4.0") {
			t.Fatalf("installed skill marker classification = %+v", skill)
		}
	}
}

func TestBuildLayout2MigrationCandidateRefusesInvalidSources(t *testing.T) {
	tests := []struct {
		name string
		edit func(t *testing.T, repo string)
	}{
		{"marker", func(t *testing.T, repo string) {
			writeFile(t, markerFile(repo), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\nextra: true\n")
		}},
		{"state", func(t *testing.T, repo string) {
			p := filepath.Join(repo, "planning", "STATE.md")
			data, _ := os.ReadFile(p)
			writeFile(t, p, strings.Replace(string(data), "schema_version: 1", "schema_version: 1\nunknown: true", 1))
		}},
		{"task policy", func(t *testing.T, repo string) {
			writeTask(t, repo, "T-001", "Example", "todo", "high", "specs/v0.1.0.md#summary", nil)
			p := filepath.Join(repo, "planning", "tasks", "T-001.md")
			data, _ := os.ReadFile(p)
			writeFile(t, p, strings.Replace(string(data), "updated_at:", "loop_policy: allow\nupdated_at:", 1))
		}},
		{"legacy policy", func(t *testing.T, repo string) {
			writeFile(t, filepath.Join(repo, "planning", "AUTONOMY.tsv"), "T-001\tallow\n")
		}},
		{"notes destination", func(t *testing.T, repo string) {
			if err := os.Symlink("elsewhere", filepath.Join(repo, "planning", "NOTES.md")); err != nil {
				t.Fatal(err)
			}
		}},
		{"divergent skill", func(t *testing.T, repo string) {
			writeFile(t, filepath.Join(repo, ".agents", "skills", "autonomous-task", "SKILL.md"), "diverged\n")
		}},
		{"conflicting skill marker", func(t *testing.T, repo string) {
			writeFile(t, filepath.Join(repo, ".agents", "skills", "autonomous-task", "SKILL.md"), "---\nname: autonomous-task\ndescription: test\ntaskrail_version: v0.4.0\nmetadata:\n  taskrail_version: v0.5.0\n---\n")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := seedFixtureRepo(t)
			writeFile(t, markerFile(repo), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n")
			tt.edit(t, repo)
			before := treeDigest(t, repo)
			if _, err := buildLayout2MigrationCandidate(repo); err == nil {
				t.Fatal("candidate construction unexpectedly succeeded")
			}
			if after := treeDigest(t, repo); before != after {
				t.Fatal("refused candidate construction changed repository bytes")
			}
		})
	}

	t.Run("same basename decoy is unrelated", func(t *testing.T) {
		repo := seedFixtureRepo(t)
		writeFile(t, markerFile(repo), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n")
		writeFile(t, filepath.Join(repo, "elsewhere", "AUTONOMY.tsv"), "decoy\n")
		if _, err := buildLayout2MigrationCandidate(repo); err != nil {
			t.Fatalf("decoy blocked candidate: %v", err)
		}
	})

	t.Run("existing regular notes permit drop-only candidate", func(t *testing.T) {
		repo := seedFixtureRepo(t)
		writeFile(t, markerFile(repo), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n")
		writeFile(t, filepath.Join(repo, "planning", "NOTES.md"), "human-owned\n")
		candidate, err := buildLayout2MigrationCandidate(repo)
		if err != nil {
			t.Fatalf("build candidate: %v", err)
		}
		if !candidate.NotesPresent || len(candidate.NotesExtractionBytes) != 0 || len(candidate.NotesTemplateBytes) != 0 {
			t.Fatalf("notes classification = present:%v extraction:%d template:%d", candidate.NotesPresent, len(candidate.NotesExtractionBytes), len(candidate.NotesTemplateBytes))
		}
	})
}

func skillOutcome(skills []MigrationSkillCandidate, path string) MigrationSkillOutcome {
	for _, skill := range skills {
		if skill.Path == path {
			return skill.Outcome
		}
	}
	return ""
}

func treeDigest(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%s\x00", filepath.ToSlash(rel), entry.Type())
		if entry.Type().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			h.Write(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
