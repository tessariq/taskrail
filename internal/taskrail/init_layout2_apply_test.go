package taskrail

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/durablefs"
	"github.com/tessariq/taskrail/internal/durabletx"
	"github.com/tessariq/taskrail/internal/repolock"
	"gopkg.in/yaml.v3"
)

// The durable layout-2 apply matrix: the published bytes, the note decisions,
// the preserved task set, the fenced-marker refusals every other command owes,
// and the recovery of an interrupted migration
// (specs/v0.5.0.md#layout-compatibility-and-upgrade).

// assertMigratedToLayout2 proves the published end state: the strict final
// marker, a schema-2 state without continuation notes or a rendered Notes
// section, and no retained transaction fence.
func assertMigratedToLayout2(t *testing.T, repo string) {
	t.Helper()
	markerData, err := os.ReadFile(markerFile(repo))
	if err != nil {
		t.Fatal(err)
	}
	marker, err := decodeLayoutMarkerStrict(markerData)
	if err != nil {
		t.Fatalf("published marker is not strict: %v", err)
	}
	if marker.MigrationFence != nil {
		t.Fatal("published marker still carries its migration fence")
	}
	if marker.LayoutVersion != layout2Version || marker.StorageMode != StorageCommitted ||
		marker.ImplementationReviewMaxRounds != 1 {
		t.Fatalf("published marker = %+v, want layout 2 committed with review maximum 1", marker)
	}
	stateData, err := os.ReadFile(filepath.Join(repo, "planning", "STATE.md"))
	if err != nil {
		t.Fatal(err)
	}
	state, _, err := decodeStateStrict(stateData)
	if err != nil {
		t.Fatalf("published state is not strict schema 2: %v", err)
	}
	if state.SourceSchema != 2 {
		t.Fatalf("source schema = %d, want 2", state.SourceSchema)
	}
	if bytes.Contains(stateData, []byte("continuation_notes")) || bytes.Contains(stateData, []byte("## Notes\n")) {
		t.Fatalf("published state still carries legacy continuation content:\n%s", stateData)
	}
	snapshot, err := durablefs.ObserveTree(repo, filepath.Join(".git", "taskrail", "transactions"))
	if err != nil || snapshot.Present && len(snapshot.Entries) != 0 {
		t.Fatalf("transactions retained after the migration: %+v err = %v", snapshot.Entries, err)
	}
}

func TestApplyPublishesTheExactPreviewedCandidate(t *testing.T) {
	t.Parallel()

	repo := seedLayout1Repo(t)
	requireRecoveryDirectoryDurability(t, repo)
	writeTask(t, repo, "T-001-example", "Example", "todo", "high", "specs/v0.1.0.md#summary", nil)
	taskBytes, err := os.ReadFile(filepath.Join(repo, "planning", "tasks", "T-001-example.md"))
	if err != nil {
		t.Fatal(err)
	}

	preview, err := layout1Service(t, repo).Init(InitInput{})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	applied, err := layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true, DropContinuationNotes: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if applied.Outcome != InitMigrated || !applied.Applied || applied.ToVersion != layout2Version {
		t.Fatalf("outcome = %q applied=%v to=%d", applied.Outcome, applied.Applied, applied.ToVersion)
	}
	if !slices.Equal(applied.Writes, preview.Writes) {
		t.Fatalf("applied writes = %+v, preview = %+v", applied.Writes, preview.Writes)
	}
	if applied.Config.Path != preview.Config.Path || applied.Config.CandidateSHA256 != preview.Config.CandidateSHA256 {
		t.Fatalf("applied config = %+v, preview = %+v", applied.Config, preview.Config)
	}
	if !slices.Equal(applied.Skills, preview.Skills) {
		t.Fatalf("applied skills = %+v, preview = %+v", applied.Skills, preview.Skills)
	}
	if !slices.Equal(applied.ContinuationNotes, preview.ContinuationNotes) {
		t.Fatalf("applied continuation notes = %v, preview = %v", applied.ContinuationNotes, preview.ContinuationNotes)
	}
	if applied.Validation == nil || !applied.Validation.Valid {
		t.Fatalf("validation = %+v, want the strict published outcome", applied.Validation)
	}
	if choice := applied.Notes[0].ContinuationAction; choice == nil || *choice != continuationChoiceDrop {
		t.Fatalf("applied continuation action = %v, want the recorded drop", choice)
	}
	assertMigratedToLayout2(t, repo)
	published, err := os.ReadFile(filepath.Join(repo, "planning", "tasks", "T-001-example.md"))
	if err != nil || !bytes.Equal(published, taskBytes) {
		t.Fatalf("preserved task bytes changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "planning", "NOTES.md")); err != nil {
		t.Fatalf("dropped notes did not create the template sidecar: %v", err)
	}
}

func TestApplyNoteDecisions(t *testing.T) {
	t.Run("extract imports decoded notes verbatim in order", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		requireRecoveryDirectoryDurability(t, repo)
		replaceContinuationNotes(t, repo, "continuation_notes:\n  - first note\n  - \"second: note\"\n")
		if _, err := layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true, ExtractContinuationNotes: true}); err != nil {
			t.Fatalf("apply: %v", err)
		}
		notes, err := os.ReadFile(filepath.Join(repo, "planning", "NOTES.md"))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{notesImportedHeading, "### Note 1\n\nfirst note", "### Note 2\n\nsecond: note"} {
			if !strings.Contains(string(notes), want) {
				t.Fatalf("extracted notes lost %q:\n%s", want, notes)
			}
		}
		assertMigratedToLayout2(t, repo)
	})

	t.Run("an existing notes sidecar is preserved byte-for-byte", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		requireRecoveryDirectoryDurability(t, repo)
		writeFile(t, notesFile(repo), "# Human Notes\n\noperator prose\n")
		before, err := os.ReadFile(notesFile(repo))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true, DropContinuationNotes: true}); err != nil {
			t.Fatalf("apply: %v", err)
		}
		after, err := os.ReadFile(notesFile(repo))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("human notes changed: %v", err)
		}
		assertMigratedToLayout2(t, repo)
	})

	t.Run("a direct schema-2 source applies with no note decision", func(t *testing.T) {
		t.Parallel()
		repo := seedLayout1Repo(t)
		requireRecoveryDirectoryDurability(t, repo)
		writeSchema2State(t, repo)
		if _, err := layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true}); err != nil {
			t.Fatalf("apply: %v", err)
		}
		assertMigratedToLayout2(t, repo)
	})
}

// Preserved task policy: a task carrying a loop-policy pair keeps its exact
// bytes and the pair survives the migration untouched.
func TestApplyPreservesTaskLocalLoopPolicy(t *testing.T) {
	t.Parallel()

	repo := seedLayout1Repo(t)
	requireRecoveryDirectoryDurability(t, repo)
	path := filepath.Join(repo, "planning", "tasks", "T-001-example.md")
	writeFile(t, path, `---
id: T-001-example
title: Example
status: todo
priority: high
spec_ref: specs/v0.1.0.md#summary
dependencies: []
loop_policy: allow
loop_reason: operator approved this task for unattended execution
updated_at: "2026-08-01T00:00:00Z"
---

# T-001-example Example

## Description

Fixture task.
`)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true, DropContinuationNotes: true}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("task bytes changed across the migration: %v", err)
	}
}

// A notes sidecar that appears between the preview and an extracting apply is
// refused before any byte is written: extraction never merges into or replaces
// human-owned prose, and the re-gated candidate reclassifies the destination.
func TestApplyRefusesNotesThatAppearedSincePreview(t *testing.T) {
	t.Parallel()

	repo := seedLayout1Repo(t)
	svc := layout1Service(t, repo)
	preview, err := svc.Init(InitInput{})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !slices.Equal(preview.Notes[0].ContinuationChoices, []string{continuationChoiceExtract, continuationChoiceDrop}) {
		t.Fatalf("preview offered no extraction choice: %+v", preview.Notes[0])
	}
	writeFile(t, notesFile(repo), "# Appeared meanwhile\n")
	before := treeDigest(t, repo)
	_, err = layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true, ExtractContinuationNotes: true})
	if err == nil {
		t.Fatal("apply must refuse a notes sidecar that appeared since the preview")
	}
	if failure := MachineFailureFor(err); failure.Code != MachineCodeDestinationExists || failure.Applied {
		t.Fatalf("failure = %+v, want an unapplied destination_exists (%v)", failure, err)
	}
	if after := treeDigest(t, repo); before != after {
		t.Fatal("refused apply changed repository bytes")
	}
}

// The fenced-marker refusals: an interrupted migration blocks every ordinary
// command as recovery_pending when transaction evidence is retained, and as
// migration_in_progress when only the fenced marker remains. The shared
// recovery boundary is the one reader the fence admits.
func TestFencedMigrationBlocksOrdinaryCommands(t *testing.T) {
	t.Parallel()

	repo := seedLayout1Repo(t)
	original, err := os.ReadFile(markerFile(repo))
	if err != nil {
		t.Fatal(err)
	}
	transaction := "0123456789abcdef0123456789abcdef"
	fenceBytes, finalBytes := fencedFixtureMarkers(t, transaction)
	writeFile(t, markerFile(repo), string(fenceBytes))

	_, serviceErr := NewService(repo)
	if serviceErr == nil {
		t.Fatal("ordinary discovery admitted a fenced marker")
	}
	if code := MachineFailureFor(serviceErr).Code; code != MachineCodeMigrationInProgress {
		t.Fatalf("bare fence code = %q, want migration_in_progress (%v)", code, serviceErr)
	}
	if !strings.Contains(serviceErr.Error(), "taskrail recover "+transaction) {
		t.Fatalf("refusal does not name the recovery command: %v", serviceErr)
	}

	paths := gitAwareRepository(repo)
	fabricateRetained(t, paths, transaction, "init", "validating", []recoverMember{
		{kind: durabletx.Managed, reported: ".taskrail/config.yml", path: ".taskrail/config.yml",
			original: original, candidate: finalBytes, fence: fenceBytes, present: true, onDisk: fenceBytes},
	}, "")
	_, pendingErr := NewService(repo)
	if pendingErr == nil {
		t.Fatal("ordinary discovery admitted retained migration state")
	}
	if failure := MachineFailureFor(pendingErr); failure.Code != MachineCodeRecoveryPending || failure.Recovery == nil ||
		failure.Recovery.TransactionID != transaction {
		t.Fatalf("failure = %+v, want recovery_pending naming the transaction (%v)", failure, pendingErr)
	}

	recovery, err := NewRecoveryService(repo)
	if err != nil {
		t.Fatalf("recovery discovery refused the fenced repository: %v", err)
	}
	if recovery.paths.PlanningDir != filepath.Join(repo, "planning") {
		t.Fatalf("recovery paths = %+v, want the fenced marker's directories", recovery.paths)
	}
}

// The shared recovery of an interrupted migration: the validating interruption
// leaves the fence on the marker and the semantic candidates published, the
// preview derives accept_candidate, and the apply completes the retained final
// marker — all mechanically, with the init validator naming the retained
// evidence rather than re-derived content.
func TestRecoverCompletesInterruptedMigration(t *testing.T) {
	t.Parallel()

	repo := seedLayout1Repo(t)
	requireRecoveryDirectoryDurability(t, repo)
	original, err := os.ReadFile(markerFile(repo))
	if err != nil {
		t.Fatal(err)
	}
	transaction := "0123456789abcdef0123456789abcdef"
	fenceBytes, finalBytes := fencedFixtureMarkers(t, transaction)

	// The exact schema-2 state bytes the migration would have published, placed
	// as the interruption's completed semantic write.
	candidate, err := buildLayout2MigrationCandidate(repo)
	if err != nil {
		t.Fatal(err)
	}

	paths := gitAwareRepository(repo)
	writeFile(t, markerFile(repo), string(fenceBytes))
	writeFile(t, filepath.Join(repo, "planning", "STATE.md"), string(candidate.StateBytes))
	fabricateRetained(t, paths, transaction, "init", "validating", []recoverMember{
		{kind: durabletx.Managed, reported: ".taskrail/config.yml", path: ".taskrail/config.yml",
			original: original, candidate: finalBytes, fence: fenceBytes, present: true, onDisk: fenceBytes},
		{kind: durabletx.Managed, reported: "planning/STATE.md", path: "planning/STATE.md",
			original: []byte(schema1FixtureState()), candidate: candidate.StateBytes, present: true, onDisk: candidate.StateBytes},
	}, "")

	svc, err := NewRecoveryService(repo)
	if err != nil {
		t.Fatalf("recovery service: %v", err)
	}
	preview, err := svc.RecoverTransaction(context.Background(), transaction, false)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Action != "accept_candidate" || preview.Applied {
		t.Fatalf("preview = %+v, want an unapplied accept_candidate", preview)
	}
	applied, err := svc.RecoverTransaction(context.Background(), transaction, true)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !applied.Applied {
		t.Fatalf("applied = %+v", applied)
	}
	if !applied.Validation.Valid {
		t.Fatalf("completed migration recovery reports invalid validation: %+v", applied.Validation)
	}
	published, err := os.ReadFile(markerFile(repo))
	if err != nil || !bytes.Equal(published, finalBytes) {
		t.Fatalf("recovered marker = %s, want the retained final bytes (%v)", published, err)
	}
	marker, err := decodeLayoutMarkerStrict(published)
	if err != nil || marker.MigrationFence != nil {
		t.Fatalf("recovered marker is not the strict final shape: %+v (%v)", marker, err)
	}
}

// The shared recovery of an interrupted migration at the fence-published phase:
// only the fence bytes landed, so the single safe action restores the original
// marker.
func TestRecoverRestoresFenceOnlyInterruption(t *testing.T) {
	t.Parallel()

	repo := seedLayout1Repo(t)
	requireRecoveryDirectoryDurability(t, repo)
	original, err := os.ReadFile(markerFile(repo))
	if err != nil {
		t.Fatal(err)
	}
	transaction := "0123456789abcdef0123456789abcdef"
	fenceBytes, finalBytes := fencedFixtureMarkers(t, transaction)

	paths := gitAwareRepository(repo)
	writeFile(t, markerFile(repo), string(fenceBytes))
	fabricateRetained(t, paths, transaction, "init", "fence_published", []recoverMember{
		{kind: durabletx.Managed, reported: ".taskrail/config.yml", path: ".taskrail/config.yml",
			original: original, candidate: finalBytes, fence: fenceBytes, present: true, onDisk: fenceBytes},
	}, "")

	svc, err := NewRecoveryService(repo)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := svc.RecoverTransaction(context.Background(), transaction, false)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Action != "restore_original" {
		t.Fatalf("action = %q, want restore_original", preview.Action)
	}
	if _, err := svc.RecoverTransaction(context.Background(), transaction, true); err != nil {
		t.Fatalf("apply: %v", err)
	}
	restored, err := os.ReadFile(markerFile(repo))
	if err != nil || !bytes.Equal(restored, original) {
		t.Fatalf("restored marker = %s, want the original (%v)", restored, err)
	}
}

// gitAwareRepository is the lock repository discovery resolves for the seeded
// fixture repos: their `.git` stub places retained state beneath it, exactly
// where a real worktree's transactions live.
func gitAwareRepository(repo string) repolock.Repository {
	return repolock.Repository{Root: repo, GitCommonDir: filepath.Join(repo, ".git"), Mode: repolock.ModeCommitted}
}

// A configured planning directory that sorts before the marker path cannot
// uphold the fence-first publication order, so the preview and the apply
// refuse it symmetrically before anything is written.
func TestLayout2UpgradeRefusesPlanningDirBeforeTheMarker(t *testing.T) {
	t.Parallel()

	repo := initGitRepo(t)
	seedFixtureTree(t, repo)
	if err := os.Rename(filepath.Join(repo, "planning"), filepath.Join(repo, ".planning")); err != nil {
		t.Fatal(err)
	}
	state, err := os.ReadFile(filepath.Join(repo, ".planning", "STATE.md"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, ".planning", "STATE.md"), strings.Replace(string(state), "specs/v0.1.0.md", "specs/v0.1.0.md", 1))
	writeFile(t, markerFile(repo), "layout_version: 1\nspecs_dir: specs\nplanning_dir: .planning\n")

	svc := layout1Service(t, repo)
	before := treeDigest(t, repo)
	_, err = svc.Init(InitInput{})
	if err == nil {
		t.Fatal("preview admitted a planning directory that sorts before the marker")
	}
	if code := MachineFailureFor(err).Code; code != MachineCodeRepositoryInvalid {
		t.Fatalf("code = %q, want repository_invalid (%v)", code, err)
	}
	if _, err := svc.Init(InitInput{Apply: true, ConfirmQuiescent: true, DropContinuationNotes: true}); err == nil {
		t.Fatal("apply admitted a planning directory that sorts before the marker")
	}
	if after := treeDigest(t, repo); before != after {
		t.Fatal("refusal changed repository bytes")
	}
}

// fencedFixtureMarkers renders the fenced marker for one transaction and the
// strict final marker it would publish, both proven decodable in their exact
// shapes.
func fencedFixtureMarkers(t *testing.T, transaction string) (fenceBytes, finalBytes []byte) {
	t.Helper()
	marker := Layout2Config{LayoutVersion: 2, SpecsDir: "specs", PlanningDir: "planning",
		StorageMode: StorageCommitted, ImplementationReviewMaxRounds: 1}
	final, err := yaml.Marshal(marker)
	if err != nil {
		t.Fatal(err)
	}
	fenced := marker
	fenced.MigrationFence = &Layout2MigrationFence{FromLayoutVersion: 1, TransactionID: transaction}
	fence, err := yaml.Marshal(fenced)
	if err != nil {
		t.Fatal(err)
	}
	return fence, final
}

func schema1FixtureState() string {
	return `---
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
relevant_artifacts: []
continuation_notes:
  - Fixture repo.
---

# STATE
`
}
