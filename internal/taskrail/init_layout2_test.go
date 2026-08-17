package taskrail

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// The layout 2 upgrade preview: the read-only decisions a layout-1 repository's
// operator must resolve before any migration can apply
// (specs/v0.5.0.md#layout-compatibility-and-upgrade). Every preview here must
// leave repository bytes untouched, and every apply gate must be observable
// while the durable publisher itself is still pending.

func seedLayout1Repo(t *testing.T) string {
	t.Helper()
	repo := seedFixtureRepo(t)
	writeFile(t, markerFile(repo), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n")
	return repo
}

func layout1Service(t *testing.T, repo string) *Service {
	t.Helper()
	return newTestService(t, repo, time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC))
}

// seedUpgradeSkills installs one marker-free parity copy and one stamped copy so
// a preview has both classifications to report.
func seedUpgradeSkills(t *testing.T, repo string) (parityPath, stampedPath string) {
	t.Helper()
	files, err := packagedSkillFiles()
	if err != nil {
		t.Fatal(err)
	}
	packageBytes, err := shippableSkillsFS.ReadFile(filepath.ToSlash(filepath.Join(shippableSkillsRoot, files[0])))
	if err != nil {
		t.Fatal(err)
	}
	parityPath = filepath.Join(repo, shippableSkillTargets[0], filepath.FromSlash(files[0]))
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
	stampedPath = filepath.Join(repo, shippableSkillTargets[1], filepath.FromSlash(files[0]))
	if err := os.MkdirAll(filepath.Dir(stampedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stampedPath, stamped, 0o644); err != nil {
		t.Fatal(err)
	}
	return parityPath, stampedPath
}

func writeSchema2State(t *testing.T, repo string) {
	t.Helper()
	writeFile(t, filepath.Join(repo, "planning", "STATE.md"), `---
schema_version: 2
updated_at: "2026-08-17T00:00:00Z"
active_spec_version: v0.1.0
active_spec_path: specs/v0.1.0.md
current_task: ""
current_task_title: ""
status_summary: idle
blockers: []
next_action: Select the next eligible task
last_verification_result: Not yet run
relevant_artifacts: []
---

# STATE
`)
}

func replaceContinuationNotes(t *testing.T, repo string, yamlBlock string) {
	t.Helper()
	path := filepath.Join(repo, "planning", "STATE.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, strings.Replace(string(data), "continuation_notes:\n  - Fixture repo.\n", yamlBlock, 1))
}

func digestOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestInitLayout2UpgradePreviewReportsCompleteDecisions(t *testing.T) {
	t.Parallel()

	repo := seedLayout1Repo(t)
	writeTask(t, repo, "T-001-example", "Example", "todo", "high", "specs/v0.1.0.md#summary", nil)
	parityPath, stampedPath := seedUpgradeSkills(t, repo)

	before := treeDigest(t, repo)
	result, err := layout1Service(t, repo).Init(InitInput{})
	if err != nil {
		t.Fatalf("upgrade preview: %v", err)
	}
	if after := treeDigest(t, repo); before != after {
		t.Fatal("upgrade preview changed repository bytes")
	}

	if result.Outcome != InitMigrationPreview || result.Applied {
		t.Fatalf("outcome = %q applied=%v, want a write-free migration_preview", result.Outcome, result.Applied)
	}
	if result.FromVersion != 1 || result.ToVersion != layout2Version {
		t.Fatalf("versions = %d -> %d, want 1 -> 2", result.FromVersion, result.ToVersion)
	}
	if result.StorageMode != string(StorageCommitted) {
		t.Fatalf("storage_mode = %q, want committed", result.StorageMode)
	}

	candidate, err := buildLayout2MigrationCandidate(repo)
	if err != nil {
		t.Fatalf("rebuild candidate: %v", err)
	}
	if result.Config.Path != markerRelPath() || result.Config.Action != configActionMigrate {
		t.Fatalf("config = %+v", result.Config)
	}
	if result.Config.CandidateSHA256 != digestOf(candidate.MarkerBytes) {
		t.Fatalf("candidate digest %q does not match the strict marker candidate", result.Config.CandidateSHA256)
	}
	if !strings.Contains(string(candidate.MarkerBytes), "implementation_review_max_rounds: 1\n") {
		t.Fatalf("marker candidate lost the default review maximum:\n%s", candidate.MarkerBytes)
	}

	wantWrites := []WriteEntry{
		{Path: ".taskrail/config.yml", Kind: writeKindConfig, Action: writeActionRefresh},
		{Path: "planning/NOTES.md", Kind: writeKindNote, Action: writeActionCreate},
		{Path: "planning/STATE.md", Kind: writeKindState, Action: writeActionRefresh},
		{Path: "planning/tasks/T-001-example.md", Kind: writeKindTask, Action: writeActionPreserve},
	}
	if !slices.Equal(result.Writes, wantWrites) {
		t.Fatalf("writes = %+v, want %+v", result.Writes, wantWrites)
	}

	note := result.Notes[0]
	if note.Path != "planning/NOTES.md" || note.FileAction != noteActionCreateTemplate {
		t.Fatalf("note entry = %+v", note)
	}
	if note.ContinuationAction != nil {
		t.Fatalf("preview continuation_action = %q, want null until an operator selects", *note.ContinuationAction)
	}
	if !slices.Equal(note.ContinuationChoices, []string{continuationChoiceExtract, continuationChoiceDrop}) {
		t.Fatalf("continuation_choices = %v, want [extract drop]", note.ContinuationChoices)
	}
	if !slices.Equal(result.ContinuationNotes, []string{"Fixture repo."}) {
		t.Fatalf("continuation_notes = %v", result.ContinuationNotes)
	}

	skills := map[string]string{
		relPath(repo, parityPath):  writeActionPreserve,
		relPath(repo, stampedPath): writeActionRefresh,
	}
	if len(result.Skills) != len(skills) {
		t.Fatalf("skills = %+v", result.Skills)
	}
	for _, skill := range result.Skills {
		if skills[skill.Path] != skill.Action {
			t.Fatalf("skill %+v action = %q, want %q", skill, skill.Action, skills[skill.Path])
		}
	}

	if result.Validation == nil || !result.Validation.Valid || len(result.Validation.Violations) != 0 {
		t.Fatalf("validation = %+v, want the validated candidate outcome", result.Validation)
	}
}

// A2: the note decisions a source repository actually allows.
func TestInitLayout2UpgradePreviewNoteDecisions(t *testing.T) {
	t.Parallel()

	t.Run("empty notes offer no choice", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		replaceContinuationNotes(t, repo, "continuation_notes: []\n")
		result, err := layout1Service(t, repo).Init(InitInput{})
		if err != nil {
			t.Fatalf("preview: %v", err)
		}
		if len(result.Notes[0].ContinuationChoices) != 0 || len(result.ContinuationNotes) != 0 {
			t.Fatalf("choices = %v notes = %v, want none", result.Notes[0].ContinuationChoices, result.ContinuationNotes)
		}
	})

	t.Run("existing notes sidecar offers drop only", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		writeFile(t, notesFile(repo), "# Repository Notes\n")
		result, err := layout1Service(t, repo).Init(InitInput{})
		if err != nil {
			t.Fatalf("preview: %v", err)
		}
		if note := result.Notes[0]; note.FileAction != noteActionPreserve ||
			!slices.Equal(note.ContinuationChoices, []string{continuationChoiceDrop}) {
			t.Fatalf("note entry = %+v, want preserve with drop-only choice", note)
		}
	})

	t.Run("multiple notes are reported verbatim in decoded order", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		replaceContinuationNotes(t, repo, `continuation_notes:
  - first note
  - "quoted: note"
  - |-
    multi
    line
`)
		result, err := layout1Service(t, repo).Init(InitInput{})
		if err != nil {
			t.Fatalf("preview: %v", err)
		}
		want := []string{"first note", "quoted: note", "multi\nline"}
		if !slices.Equal(result.ContinuationNotes, want) {
			t.Fatalf("continuation_notes = %q, want %q", result.ContinuationNotes, want)
		}
		if !slices.Equal(result.Notes[0].ContinuationChoices, []string{continuationChoiceExtract, continuationChoiceDrop}) {
			t.Fatalf("choices = %v", result.Notes[0].ContinuationChoices)
		}
	})

	t.Run("direct schema 2 source offers no note decision", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		writeSchema2State(t, repo)
		result, err := layout1Service(t, repo).Init(InitInput{})
		if err != nil {
			t.Fatalf("preview: %v", err)
		}
		if len(result.Notes[0].ContinuationChoices) != 0 || len(result.ContinuationNotes) != 0 {
			t.Fatalf("schema 2 source exposed note decisions: %+v", result.Notes[0])
		}
	})
}

// A3: skill classifications and the blockers that precede any migration.
func TestInitLayout2UpgradeSkillClassification(t *testing.T) {
	t.Parallel()

	t.Run("parity mirrors are preserved and demand no skill flags", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		requireRecoveryDirectoryDurability(t, repo)
		parityPath, stampedPath := seedUpgradeSkills(t, repo)
		parityBytes, err := os.ReadFile(parityPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(stampedPath); err != nil {
			t.Fatal(err)
		}
		result, err := layout1Service(t, repo).Init(InitInput{})
		if err != nil {
			t.Fatalf("preview: %v", err)
		}
		if len(result.Skills) == 0 || result.Skills[0].Path != relPath(repo, parityPath) ||
			result.Skills[0].Action != writeActionPreserve {
			t.Fatalf("skills = %+v, want one preserved parity mirror", result.Skills)
		}
		// Passing no skill flags at all, the apply publishes the migration and
		// leaves the marker-free parity mirror byte-identical.
		applied, err := layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true, DropContinuationNotes: true})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if applied.Outcome != InitMigrated || !applied.Applied {
			t.Fatalf("outcome = %q applied=%v, want the applied migration", applied.Outcome, applied.Applied)
		}
		if after, err := os.ReadFile(parityPath); err != nil || !bytes.Equal(after, parityBytes) {
			t.Fatalf("parity mirror changed: %v", err)
		}
	})

	t.Run("stamped copies gate apply on the combined forced refresh", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		_, stampedPath := seedUpgradeSkills(t, repo)
		_, err := layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true, DropContinuationNotes: true})
		if err == nil {
			t.Fatal("apply must demand the combined skill refresh")
		}
		if code := MachineFailureFor(err).Code; code != MachineCodeInvalidArguments {
			t.Fatalf("code = %q, want invalid_arguments (%v)", code, err)
		}
		if !strings.Contains(err.Error(), "init --apply --with-skills --force") ||
			!strings.Contains(err.Error(), relPath(repo, stampedPath)) {
			t.Fatalf("refusal is not actionable: %v", err)
		}
	})

	t.Run("divergent marker-free copy blocks the preview", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		parityPath, _ := seedUpgradeSkills(t, repo)
		writeFile(t, parityPath, "diverged\n")
		before := treeDigest(t, repo)
		_, err := layout1Service(t, repo).Init(InitInput{})
		if err == nil {
			t.Fatal("divergent marker-free copy must block the upgrade")
		}
		if code := MachineFailureFor(err).Code; code != MachineCodeRepositoryInvalid {
			t.Fatalf("code = %q, want repository_invalid (%v)", code, err)
		}
		if after := treeDigest(t, repo); before != after {
			t.Fatal("blocked preview changed repository bytes")
		}
	})

	t.Run("conflicting markers block the preview", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		_, stampedPath := seedUpgradeSkills(t, repo)
		// One copy carrying both version markers with different values is a
		// conflicting marker set no migration may normalize away.
		writeFile(t, stampedPath, "---\nname: autonomous-task\ndescription: test\n"+
			"taskrail_version: v0.4.0\nmetadata:\n  taskrail_version: v0.5.0\n---\n")
		before := treeDigest(t, repo)
		_, err := layout1Service(t, repo).Init(InitInput{})
		if err == nil {
			t.Fatal("conflicting markers must block the upgrade")
		}
		if after := treeDigest(t, repo); before != after {
			t.Fatal("blocked preview changed repository bytes")
		}
	})

	t.Run("repositories without installed copies report none", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		result, err := layout1Service(t, repo).Init(InitInput{})
		if err != nil {
			t.Fatalf("preview: %v", err)
		}
		if len(result.Skills) != 0 {
			t.Fatalf("skills = %+v, want none", result.Skills)
		}
	})
}

// A4: legacy-policy entries, aliases, invalid candidates, and inapplicable flags.
func TestInitLayout2UpgradeRefusals(t *testing.T) {
	t.Parallel()

	t.Run("legacy policy path is refused with guidance", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		writeFile(t, filepath.Join(repo, "planning", "AUTONOMY.tsv"), "T-001\tallow\n")
		before := treeDigest(t, repo)
		_, err := layout1Service(t, repo).Init(InitInput{})
		if err == nil {
			t.Fatal("legacy policy entry must refuse the upgrade")
		}
		if code := MachineFailureFor(err).Code; code != MachineCodeUnsupported {
			t.Fatalf("code = %q, want unsupported (%v)", code, err)
		}
		if !strings.Contains(err.Error(), "taskrail task loop") {
			t.Fatalf("refusal lacks operator guidance: %v", err)
		}
		if after := treeDigest(t, repo); before != after {
			t.Fatal("refusal changed repository bytes")
		}
	})

	t.Run("a symlink at the legacy policy path is still an entry", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		writeFile(t, filepath.Join(repo, "elsewhere"), "target\n")
		if err := os.Symlink(filepath.Join(repo, "elsewhere"), filepath.Join(repo, "planning", "AUTONOMY.tsv")); err != nil {
			t.Fatal(err)
		}
		_, err := layout1Service(t, repo).Init(InitInput{})
		if code := MachineFailureFor(err).Code; code != MachineCodeUnsupported {
			t.Fatalf("code = %q, want unsupported without following the link (%v)", code, err)
		}
	})

	t.Run("same-basename decoy remains unrelated", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		writeFile(t, filepath.Join(repo, "elsewhere", "AUTONOMY.tsv"), "decoy\n")
		if _, err := layout1Service(t, repo).Init(InitInput{}); err != nil {
			t.Fatalf("decoy blocked the preview: %v", err)
		}
	})

	t.Run("unsafe notes alias is refused", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		writeFile(t, filepath.Join(repo, "planning", "notes.md"), "# lower-case sibling\n")
		_, err := layout1Service(t, repo).Init(InitInput{})
		if code := MachineFailureFor(err).Code; code != MachineCodePathBlocked {
			t.Fatalf("code = %q, want path_blocked (%v)", code, err)
		}
	})

	t.Run("invalid candidate state is refused", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		path := filepath.Join(repo, "planning", "STATE.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, path, strings.Replace(string(data), "schema_version: 1", "schema_version: 1\nunknown: true", 1))
		before := treeDigest(t, repo)
		_, err = layout1Service(t, repo).Init(InitInput{})
		if err == nil {
			t.Fatal("invalid candidate must refuse the upgrade")
		}
		if code := MachineFailureFor(err).Code; code != MachineCodeRepositoryInvalid {
			t.Fatalf("code = %q, want repository_invalid (%v)", code, err)
		}
		if after := treeDigest(t, repo); before != after {
			t.Fatal("refusal changed repository bytes")
		}
	})
}

// A4/A5: the operator gates, and quiescence being required only for the real
// upgrade apply.
func TestInitLayout2UpgradeInputGates(t *testing.T) {
	t.Parallel()

	t.Run("preview rejects the quiescence assertion", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		_, err := layout1Service(t, repo).Init(InitInput{ConfirmQuiescent: true})
		if code := MachineFailureFor(err).Code; code != MachineCodeInvalidArguments {
			t.Fatalf("code = %q, want invalid_arguments (%v)", code, err)
		}
		if !strings.Contains(err.Error(), "--confirm-quiescent") {
			t.Fatalf("refusal is not actionable: %v", err)
		}
	})

	t.Run("preview rejects note selections", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		for _, in := range []InitInput{
			{ExtractContinuationNotes: true},
			{DropContinuationNotes: true},
		} {
			_, err := layout1Service(t, repo).Init(in)
			if code := MachineFailureFor(err).Code; code != MachineCodeInvalidArguments {
				t.Fatalf("input %+v code = %q, want invalid_arguments (%v)", in, code, err)
			}
			if !strings.Contains(err.Error(), "--apply") {
				t.Fatalf("refusal does not point at apply: %v", err)
			}
		}
	})

	t.Run("apply requires quiescence", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		_, err := layout1Service(t, repo).Init(InitInput{Apply: true})
		if code := MachineFailureFor(err).Code; code != MachineCodeInvalidArguments {
			t.Fatalf("code = %q, want invalid_arguments (%v)", code, err)
		}
		if !strings.Contains(err.Error(), "--confirm-quiescent") {
			t.Fatalf("refusal does not name the assertion: %v", err)
		}
	})

	t.Run("apply requires exactly one note selection when notes exist", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		for _, in := range []InitInput{
			{Apply: true, ConfirmQuiescent: true},
			{Apply: true, ConfirmQuiescent: true, ExtractContinuationNotes: true, DropContinuationNotes: true},
		} {
			_, err := layout1Service(t, repo).Init(in)
			if code := MachineFailureFor(err).Code; code != MachineCodeInvalidArguments {
				t.Fatalf("input %+v code = %q, want invalid_arguments (%v)", in, code, err)
			}
			if !strings.Contains(err.Error(), "--extract-continuation-notes") ||
				!strings.Contains(err.Error(), "--drop-continuation-notes") {
				t.Fatalf("refusal does not name both options: %v", err)
			}
		}
	})

	t.Run("empty notes reject either selection as unnecessary", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		replaceContinuationNotes(t, repo, "continuation_notes: []\n")
		_, err := layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true, ExtractContinuationNotes: true})
		if code := MachineFailureFor(err).Code; code != MachineCodeInvalidArguments {
			t.Fatalf("code = %q, want invalid_arguments (%v)", code, err)
		}
		if !strings.Contains(err.Error(), "unnecessary") {
			t.Fatalf("refusal does not explain the selection is unnecessary: %v", err)
		}
	})

	t.Run("direct schema 2 sources reject both selections", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		writeSchema2State(t, repo)
		_, err := layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true, DropContinuationNotes: true})
		if code := MachineFailureFor(err).Code; code != MachineCodeInvalidArguments {
			t.Fatalf("code = %q, want invalid_arguments (%v)", code, err)
		}
		if !strings.Contains(err.Error(), "schema 2") {
			t.Fatalf("refusal does not name the schema 2 source: %v", err)
		}
	})

	t.Run("extract over an existing notes sidecar directs a manual merge", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		writeFile(t, notesFile(repo), "# Repository Notes\n")
		_, err := layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true, ExtractContinuationNotes: true})
		if code := MachineFailureFor(err).Code; code != MachineCodeDestinationExists {
			t.Fatalf("code = %q, want destination_exists (%v)", code, err)
		}
		if !strings.Contains(err.Error(), "--drop-continuation-notes") {
			t.Fatalf("refusal does not name the drop retry: %v", err)
		}
	})

	t.Run("satisfied gates publish the migration and clear the fence", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		requireRecoveryDirectoryDurability(t, repo)
		_, err := layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true, DropContinuationNotes: true})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		assertMigratedToLayout2(t, repo)
	})

	t.Run("the combined prescribed command publishes and normalizes skills", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		requireRecoveryDirectoryDurability(t, repo)
		_, stampedPath := seedUpgradeSkills(t, repo)
		applied, err := layout1Service(t, repo).Init(InitInput{
			Apply: true, ConfirmQuiescent: true, DropContinuationNotes: true,
			WithSkills: true, ForceSkills: true, SkillVersion: "v0.5.0",
		})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if applied.Outcome != InitMigrated {
			t.Fatalf("outcome = %q, want the applied migration", applied.Outcome)
		}
		assertMigratedToLayout2(t, repo)
		normalized, err := os.ReadFile(stampedPath)
		if err != nil {
			t.Fatal(err)
		}
		version, err := skillVersionOf(normalized)
		if err != nil || version != "v0.5.0" {
			t.Fatalf("normalized marker = %q err = %v, want nested-only v0.5.0", version, err)
		}
		if _, marker, err := migrationSkillMarker(normalized); err != nil || marker != "nested" {
			t.Fatalf("normalized marker shape = %q err = %v, want nested-only", marker, err)
		}
	})
}

// Quiescence and the note selections exist only for the layout-2 upgrade; every
// other init outcome rejects them as inapplicable — including the bridged
// current-layout flow an explicit skill-install request takes.
func TestInitRejectsUpgradeOnlyInputsOutsideTheUpgrade(t *testing.T) {
	t.Parallel()

	repo := seedFixtureRepo(t)
	writeFile(t, markerFile(repo), "layout_version: 0\nspecs_dir: specs\nplanning_dir: planning\n")
	svc := newTestService(t, repo, time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(InitInput{Apply: true, ConfirmQuiescent: true}); MachineFailureFor(err).Code != MachineCodeInvalidArguments {
		t.Fatalf("legacy migration quiescence code = %v (%v)", MachineFailureFor(err).Code, err)
	}

	fresh := newTestService(t, initGitRepo(t), time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC))
	if _, err := fresh.Init(InitInput{ConfirmQuiescent: true}); MachineFailureFor(err).Code != MachineCodeInvalidArguments {
		t.Fatalf("fresh init quiescence code = %v (%v)", MachineFailureFor(err).Code, err)
	}
	if _, err := fresh.Init(InitInput{ExtractContinuationNotes: true}); MachineFailureFor(err).Code != MachineCodeInvalidArguments {
		t.Fatalf("fresh init extract code = %v (%v)", MachineFailureFor(err).Code, err)
	}

	bridged := seedLayout1Repo(t)
	if _, err := layout1Service(t, bridged).Init(InitInput{WithSkills: true, SkillVersion: "v0.1.0", ConfirmQuiescent: true}); MachineFailureFor(err).Code != MachineCodeInvalidArguments {
		t.Fatalf("bridged quiescence code = %v (%v)", MachineFailureFor(err).Code, err)
	}
	if _, err := layout1Service(t, bridged).Init(InitInput{WithSkills: true, SkillVersion: "v0.1.0", DropContinuationNotes: true}); MachineFailureFor(err).Code != MachineCodeInvalidArguments {
		t.Fatalf("bridged drop code = %v (%v)", MachineFailureFor(err).Code, err)
	}
}

// The candidate paths follow the marker's configured logical directories, so a
// repository keeping its layout elsewhere previews its own paths, never the
// defaults.
func TestInitLayout2PreviewFollowsConfiguredDirectories(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	seedFixtureTree(t, repo)
	if err := os.Rename(filepath.Join(repo, "specs"), filepath.Join(repo, "docs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(repo, "planning"), filepath.Join(repo, "work")); err != nil {
		t.Fatal(err)
	}
	spec, err := os.ReadFile(filepath.Join(repo, "docs", "v0.1.0.md"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "docs", "v0.1.0.md"), strings.Replace(string(spec), "specs/", "docs/", 1))
	state, err := os.ReadFile(filepath.Join(repo, "work", "STATE.md"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "work", "STATE.md"), strings.Replace(string(state), "specs/v0.1.0.md", "docs/v0.1.0.md", 1))
	writeFile(t, markerFile(repo), "layout_version: 1\nspecs_dir: docs\nplanning_dir: work\n")

	before := treeDigest(t, repo)
	result, err := layout1Service(t, repo).Init(InitInput{})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if after := treeDigest(t, repo); before != after {
		t.Fatal("preview changed repository bytes")
	}
	want := []WriteEntry{
		{Path: ".taskrail/config.yml", Kind: writeKindConfig, Action: writeActionRefresh},
		{Path: "work/NOTES.md", Kind: writeKindNote, Action: writeActionCreate},
		{Path: "work/STATE.md", Kind: writeKindState, Action: writeActionRefresh},
	}
	if !slices.Equal(result.Writes, want) {
		t.Fatalf("writes = %+v, want %+v", result.Writes, want)
	}
	if result.Notes[0].Path != "work/NOTES.md" {
		t.Fatalf("notes path = %q, want the configured planning directory", result.Notes[0].Path)
	}
}

// A5: preview and apply decide over one candidate, so the apply path's gates
// reference the same facts the preview reported.
func TestInitLayout2PreviewAndApplyIdentifyTheSameCandidate(t *testing.T) {
	t.Parallel()

	repo := seedLayout1Repo(t)
	writeTask(t, repo, "T-001-example", "Example", "todo", "high", "specs/v0.1.0.md#summary", nil)
	svc := layout1Service(t, repo)

	preview, err := svc.Init(InitInput{})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	candidate, err := buildLayout2MigrationCandidate(repo)
	if err != nil {
		t.Fatalf("candidate: %v", err)
	}
	paths := map[string]bool{candidate.MarkerPath: true, candidate.StatePath: true, candidate.NotesPath: true}
	for logical := range candidate.TaskBytes {
		paths[logical] = true
	}
	for _, write := range preview.Writes {
		if !paths[write.Path] {
			t.Fatalf("preview reports path %q outside the candidate set", write.Path)
		}
	}

	// The apply path builds the same candidate before its gates: its note gate
	// fires only because the same decoded notes exist.
	_, err = svc.Init(InitInput{Apply: true, ConfirmQuiescent: true})
	if err == nil || !strings.Contains(err.Error(), continuationChoiceExtract) {
		t.Fatalf("apply note gate does not reflect the previewed candidate: %v", err)
	}
	if !slices.Equal(preview.Notes[0].ContinuationChoices, []string{continuationChoiceExtract, continuationChoiceDrop}) {
		t.Fatalf("preview choices = %v", preview.Notes[0].ContinuationChoices)
	}
}

// The interim bridge: an explicit skill-install request on an upgradable
// repository is served by the current layout, not the write-free preview, so
// adopter flows keep working until the durable migration publisher ships.
func TestInitWithSkillsOnUpgradableRepoKeepsCurrentFlow(t *testing.T) {
	t.Parallel()

	repo := seedLayout1Repo(t)
	result, err := layout1Service(t, repo).Init(InitInput{WithSkills: true, SkillVersion: "v9.9.9"})
	if err != nil {
		t.Fatalf("init --with-skills: %v", err)
	}
	if result.Outcome != InitCurrent || result.ToVersion != currentLayoutVersion {
		t.Fatalf("outcome = %q to %d, want current at %d", result.Outcome, result.ToVersion, currentLayoutVersion)
	}
	if len(result.Skills) == 0 {
		t.Fatal("current-flow skill install did not report its inventory")
	}
}
