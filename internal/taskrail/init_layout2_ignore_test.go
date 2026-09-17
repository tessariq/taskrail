package taskrail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/durabletx"
)

// T-416: the layout-1 to layout-2 upgrade path establishes the same
// committed-mode artifact-ignore invariant a fresh committed init establishes
// (T-398). The preview names the .gitignore candidate without writing, the
// gated apply publishes it inside the one durable migration transaction, and
// the shared ignore-policy boundary keeps user rules, equivalent rules, and
// deliberate negations byte-for-byte through the upgrade and a later
// idempotent init.

// seedLayout1GitRepo seeds a real Git repository holding a valid committed
// layout-1 Taskrail state, committed the way an adopter's tree would be.
func seedLayout1GitRepo(t *testing.T) string {
	t.Helper()
	repo := realGitRepo(t)
	seedFixtureTree(t, repo)
	writeFile(t, markerFile(repo), "layout_version: 1\nspecs_dir: specs\nplanning_dir: planning\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-m", "layout 1 fixture")
	return repo
}

func layout2IgnoreEntry(t *testing.T, writes []WriteEntry) *WriteEntry {
	t.Helper()
	for i, write := range writes {
		if write.Path == gitignoreFile {
			return &writes[i]
		}
	}
	return nil
}

func readRepoBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// Acceptance 1: a write-free upgrade preview reports the .gitignore candidate
// without changing repository bytes; the gated apply publishes layout 2 with
// the rule, and a file below the configured planning artifacts/ tree never
// reaches git status --porcelain --untracked-files=all.
func TestLayout2UpgradePublishesArtifactsIgnore(t *testing.T) {
	t.Parallel()

	repo := seedLayout1GitRepo(t)
	requireRecoveryDirectoryDurability(t, repo)
	if _, err := os.Stat(filepath.Join(repo, gitignoreFile)); !os.IsNotExist(err) {
		t.Fatalf("fixture repo already carries a %s: %v", gitignoreFile, err)
	}

	preview, err := layout1Service(t, repo).Init(InitInput{})
	if err != nil {
		t.Fatalf("upgrade preview: %v", err)
	}
	if entry := layout2IgnoreEntry(t, preview.Writes); entry == nil ||
		entry.Kind != writeKindConfig || entry.Action != writeActionCreate {
		t.Fatalf("preview .gitignore entry = %+v, want a %s %s", entry, writeKindConfig, writeActionCreate)
	}
	if _, err := os.Stat(filepath.Join(repo, gitignoreFile)); !os.IsNotExist(err) {
		t.Fatalf("preview wrote %s: %v", gitignoreFile, err)
	}

	applied, err := layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true, DropContinuationNotes: true})
	if err != nil {
		t.Fatalf("upgrade apply: %v", err)
	}
	if applied.Outcome != InitMigrated || !applied.Applied {
		t.Fatalf("outcome = %q applied=%v", applied.Outcome, applied.Applied)
	}
	if !slices.Equal(applied.Writes, preview.Writes) {
		t.Fatalf("applied writes = %+v, preview = %+v", applied.Writes, preview.Writes)
	}
	assertMigratedToLayout2(t, repo)
	want := "# taskrail artifacts begin\n/planning/artifacts/\n# taskrail artifacts end\n"
	if got := gitignoreBytes(t, repo); got != want {
		t.Fatalf(".gitignore = %q, want %q", got, want)
	}

	writeFile(t, filepath.Join(repo, "planning", "artifacts", "verify", "T-009", "20260917T000000Z-probe", "report.json"), "{}\n")
	for _, path := range gitStatusPorcelain(t, repo) {
		if strings.HasPrefix(path, "planning/artifacts/") {
			t.Fatalf("git status lists artifact path %s", path)
		}
	}
}

// Acceptance 2: the upgrade preserves unrelated user .gitignore bytes exactly
// — with or without a final newline — and adds at most one marked artifact
// rule. An existing equivalent rule, the marked Taskrail block, or a deliberate
// negation is preserved byte-for-byte and neither the upgrade nor a later
// idempotent init overrides or duplicates it.
func TestLayout2UpgradeGitignoreMatrix(t *testing.T) {
	block := "# taskrail artifacts begin\n/planning/artifacts/\n# taskrail artifacts end\n"
	for _, tc := range []struct {
		name   string
		seeded string
		want   string
		action string
	}{
		{name: "unrelated bytes without final newline", seeded: "node_modules/\n/dist",
			want: "node_modules/\n/dist\n" + block, action: writeActionRefresh},
		{name: "unrelated bytes with final newline", seeded: "vendor/\nbuild/\n",
			want: "vendor/\nbuild/\n" + block, action: writeActionRefresh},
		{name: "equivalent unanchored rule", seeded: "planning/artifacts/\n", want: "planning/artifacts/\n", action: writeActionPreserve},
		{name: "equivalent anchored rule", seeded: "/planning/artifacts/\n", want: "/planning/artifacts/\n", action: writeActionPreserve},
		{name: "equivalent directory rule", seeded: "planning/artifacts\n", want: "planning/artifacts\n", action: writeActionPreserve},
		{name: "deliberate user negation", seeded: "!/planning/artifacts/\n", want: "!/planning/artifacts/\n", action: writeActionPreserve},
		{name: "existing taskrail block", seeded: block, want: block, action: writeActionPreserve},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := seedLayout1Repo(t)
			requireRecoveryDirectoryDurability(t, repo)
			writeFile(t, filepath.Join(repo, gitignoreFile), tc.seeded)
			writeFile(t, filepath.Join(repo, "unrelated.txt"), "keep me\n")
			unrelatedBefore := readRepoBytes(t, filepath.Join(repo, "unrelated.txt"))

			preview, err := layout1Service(t, repo).Init(InitInput{})
			if err != nil {
				t.Fatalf("preview: %v", err)
			}
			if entry := layout2IgnoreEntry(t, preview.Writes); entry == nil || entry.Action != tc.action {
				t.Fatalf("preview .gitignore entry = %+v, want action %q", entry, tc.action)
			}

			if _, err := layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true, DropContinuationNotes: true}); err != nil {
				t.Fatalf("apply: %v", err)
			}
			assertMigratedToLayout2(t, repo)
			if got := gitignoreBytes(t, repo); got != tc.want {
				t.Fatalf("upgraded .gitignore = %q, want %q", got, tc.want)
			}

			// A later idempotent init over the published layout 2 keeps the
			// same bytes and never duplicates the marked block.
			if _, err := layout1Service(t, repo).Init(InitInput{}); err != nil {
				t.Fatalf("later init: %v", err)
			}
			got := gitignoreBytes(t, repo)
			if got != tc.want {
				t.Fatalf("later init changed .gitignore: %q, want %q", got, tc.want)
			}
			if strings.Count(got, artifactsIgnoreBegin) > 1 {
				t.Fatalf("ignore block duplicated:\n%s", got)
			}
			if got := readRepoBytes(t, filepath.Join(repo, "unrelated.txt")); !bytes.Equal(got, unrelatedBefore) {
				t.Fatalf("unrelated user file changed: %q", got)
			}
		})
	}
}

// F2/G1 coverage: a non-regular .gitignore on the upgrade path gets the same
// treatment fresh init gives it. A symlinked target that does not manage the
// rule refuses with path_blocked through both the write-free preview and the
// gated apply, changing no byte of the link, the target, or the tree.
func TestLayout2UpgradeRefusesUnmanagedLinkedGitignore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlinks require privileges this test cannot assume")
	}
	t.Parallel()

	repo := seedLayout1Repo(t)
	writeFile(t, filepath.Join(repo, "ignore-actual"), "node_modules/\n")
	if err := os.Symlink("ignore-actual", filepath.Join(repo, gitignoreFile)); err != nil {
		t.Fatal(err)
	}
	before := treeDigest(t, repo)
	targetBefore := readRepoBytes(t, filepath.Join(repo, "ignore-actual"))

	for _, in := range []InitInput{{}, {Apply: true, ConfirmQuiescent: true, DropContinuationNotes: true}} {
		_, err := layout1Service(t, repo).Init(in)
		if code := MachineFailureFor(err).Code; code != MachineCodePathBlocked {
			t.Fatalf("input %+v code = %q, want %q (%v)", in, code, MachineCodePathBlocked, err)
		}
	}
	if after := treeDigest(t, repo); before != after {
		t.Fatal("refusal changed repository bytes")
	}
	if got := readRepoBytes(t, filepath.Join(repo, "ignore-actual")); !bytes.Equal(got, targetBefore) {
		t.Fatalf("linked target changed: %q", got)
	}
}

// F2/G1 coverage: a symlinked .gitignore whose target already manages the
// artifacts rule is preserved without binding or writing through the link —
// the upgrade completes layout 2 with the target byte-for-byte intact.
func TestLayout2UpgradePreservesLinkedGitignoreThatManagesArtifacts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlinks require privileges this test cannot assume")
	}
	t.Parallel()

	repo := seedLayout1Repo(t)
	requireRecoveryDirectoryDurability(t, repo)
	managed := "# taskrail artifacts begin\n/planning/artifacts/\n# taskrail artifacts end\n"
	writeFile(t, filepath.Join(repo, "ignore-actual"), managed)
	if err := os.Symlink("ignore-actual", filepath.Join(repo, gitignoreFile)); err != nil {
		t.Fatal(err)
	}

	preview, err := layout1Service(t, repo).Init(InitInput{})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if entry := layout2IgnoreEntry(t, preview.Writes); entry == nil || entry.Action != writeActionPreserve {
		t.Fatalf("preview .gitignore entry = %+v, want preserve", entry)
	}
	if _, err := layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true, DropContinuationNotes: true}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	assertMigratedToLayout2(t, repo)
	if got := readRepoBytes(t, filepath.Join(repo, "ignore-actual")); !bytes.Equal(got, []byte(managed)) {
		t.Fatalf("linked target changed: %q", got)
	}
	if got := gitignoreBytes(t, repo); got != managed {
		t.Fatalf(".gitignore no longer resolves to the managed target: %q", got)
	}
}

// G2 coverage: outside any Git worktree there is no ignore state to manage, so
// the upgrade preview reports no .gitignore entry and the gated apply
// publishes layout 2 without creating one.
func TestLayout2UpgradeOutsideGitManagesNoIgnoreEntry(t *testing.T) {
	t.Parallel()

	repo := seedLayout1Repo(t)
	requireRecoveryDirectoryDurability(t, repo)
	if err := os.RemoveAll(filepath.Join(repo, ".git")); err != nil {
		t.Fatalf("remove stub .git: %v", err)
	}

	preview, err := layout1Service(t, repo).Init(InitInput{})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if entry := layout2IgnoreEntry(t, preview.Writes); entry != nil {
		t.Fatalf("non-Git upgrade preview reported .gitignore %+v", entry)
	}
	if _, err := layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true, DropContinuationNotes: true}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	assertMigratedToLayout2(t, repo)
	if _, err := os.Stat(filepath.Join(repo, gitignoreFile)); !os.IsNotExist(err) {
		t.Fatalf("non-Git upgrade wrote %s: %v", gitignoreFile, err)
	}
}

// installMigrationValidatedHook installs the migration transaction's
// post-publication validation hook for one test and guarantees its removal,
// because the hook is package-global. The owning test must not run in
// parallel: the hook fires for every concurrently running migration apply.
func installMigrationValidatedHook(t *testing.T, hook func() error) {
	t.Helper()
	testHookMigrationCandidateValidated = hook
	t.Cleanup(func() { testHookMigrationCandidateValidated = nil })
}

// Acceptance 3 (failure half): a deterministic failure after the
// artifact-ignore candidate has published but before the migration completes
// rolls the complete write set back — the original .gitignore bytes, the
// original marker, and the original state all return, unrelated user files
// stay untouched, and no retained transaction remains.
func TestLayout2UpgradeRollsBackGitignoreOnLateFailure(t *testing.T) {
	repo := seedLayout1Repo(t)
	requireRecoveryDirectoryDurability(t, repo)
	seeded := "vendor/\n"
	writeFile(t, filepath.Join(repo, gitignoreFile), seeded)
	writeFile(t, filepath.Join(repo, "unrelated.txt"), "keep me\n")
	markerBefore := readRepoBytes(t, markerFile(repo))
	stateBefore := readRepoBytes(t, filepath.Join(repo, "planning", "STATE.md"))

	invocations := 0
	installMigrationValidatedHook(t, func() error {
		invocations++
		if invocations < 2 {
			return nil
		}
		data, err := os.ReadFile(filepath.Join(repo, gitignoreFile))
		if err != nil || !strings.Contains(string(data), artifactsIgnoreBegin) {
			return fmt.Errorf("artifacts ignore candidate was not published with the migration: %v", err)
		}
		return errors.New("deterministic post-publication failure")
	})

	_, err := layout1Service(t, repo).Init(InitInput{Apply: true, ConfirmQuiescent: true, DropContinuationNotes: true})
	if err == nil || !strings.Contains(err.Error(), "deterministic post-publication failure") {
		t.Fatalf("apply failure = %v, want the deterministic post-publication failure", err)
	}
	if got := gitignoreBytes(t, repo); got != seeded {
		t.Fatalf("rollback left .gitignore = %q, want the original %q", got, seeded)
	}
	if got := readRepoBytes(t, markerFile(repo)); !bytes.Equal(got, markerBefore) {
		t.Fatalf("rollback left a non-original marker:\n%s", got)
	}
	if got := readRepoBytes(t, filepath.Join(repo, "planning", "STATE.md")); !bytes.Equal(got, stateBefore) {
		t.Fatalf("rollback left a non-original state:\n%s", got)
	}
	if got := readRepoBytes(t, filepath.Join(repo, "unrelated.txt")); string(got) != "keep me\n" {
		t.Fatalf("rollback disturbed an unrelated user file: %q", got)
	}
	if _, serviceErr := NewService(repo); serviceErr != nil {
		t.Fatalf("rolled-back repository still refuses ordinary discovery: %v", serviceErr)
	}
}

// Acceptance 3 (recovery half): an interrupted migration whose candidate write
// set includes the .gitignore recovers by accepting the candidate, leaving the
// new ignore rule together with the final layout-2 state while unrelated user
// files survive byte-for-byte.
func TestLayout2UpgradeRecoveryAcceptKeepsArtifactsIgnore(t *testing.T) {
	t.Parallel()

	repo := seedLayout1Repo(t)
	requireRecoveryDirectoryDurability(t, repo)
	seeded := "vendor/\n"
	writeFile(t, filepath.Join(repo, gitignoreFile), seeded)
	writeFile(t, filepath.Join(repo, "unrelated.txt"), "keep me\n")
	markerBefore := readRepoBytes(t, markerFile(repo))
	stateBefore := readRepoBytes(t, filepath.Join(repo, "planning", "STATE.md"))

	svc := layout1Service(t, repo)
	candidate, err := buildLayout2MigrationCandidate(repo)
	if err != nil {
		t.Fatalf("candidate: %v", err)
	}
	request, err := svc.migrationTransaction(InitInput{Apply: true, ConfirmQuiescent: true, DropContinuationNotes: true}, candidate, recoverFixtureID)
	if err != nil {
		t.Fatalf("migration request: %v", err)
	}

	var ignore *durabletx.Member
	for i := range request.Members {
		if request.Members[i].Kind == durabletx.Worktree && request.Members[i].Reported == gitignoreFile {
			ignore = &request.Members[i]
		}
	}
	if ignore == nil {
		t.Fatal("migration transaction does not publish the artifacts ignore")
	}
	want := appendArtifactsIgnore([]byte(seeded), svc.artifactsIgnoreEntry())
	if !bytes.Equal(ignore.Content, want) || ignore.Fence != nil {
		t.Fatalf("ignore member = %q, want the appended block without a fence", ignore.Content)
	}

	fenceBytes, err := fencedMarkerBytes(candidate.Marker, recoverFixtureID)
	if err != nil {
		t.Fatal(err)
	}
	notesBytes := []byte(starterNotes())
	fabricateRetained(t, gitAwareRepository(repo), recoverFixtureID, "init", "validating", []recoverMember{
		{kind: durabletx.Managed, reported: candidate.MarkerPath, path: candidate.MarkerPath,
			original: markerBefore, candidate: candidate.MarkerBytes, fence: fenceBytes, present: true, onDisk: fenceBytes},
		{kind: durabletx.Managed, reported: candidate.NotesPath, path: candidate.NotesPath,
			candidate: notesBytes, present: true, onDisk: notesBytes},
		{kind: durabletx.Managed, reported: candidate.StatePath, path: candidate.StatePath,
			original: stateBefore, candidate: candidate.StateBytes, present: true, onDisk: candidate.StateBytes},
		{kind: durabletx.Worktree, reported: gitignoreFile, path: gitignoreFile,
			original: []byte(seeded), candidate: want, present: true, onDisk: want},
	}, "")

	if _, serviceErr := NewService(repo); MachineFailureFor(serviceErr).Code != MachineCodeRecoveryPending {
		t.Fatalf("ordinary discovery = %v, want recovery_pending", serviceErr)
	}
	recovery, err := NewRecoveryService(repo)
	if err != nil {
		t.Fatalf("recovery service: %v", err)
	}
	preview, err := recovery.RecoverTransaction(context.Background(), recoverFixtureID, false)
	if err != nil {
		t.Fatalf("recovery preview: %v", err)
	}
	if preview.Action != "accept_candidate" || preview.Applied {
		t.Fatalf("preview = %+v, want an unapplied accept_candidate", preview)
	}
	applied, err := recovery.RecoverTransaction(context.Background(), recoverFixtureID, true)
	if err != nil {
		t.Fatalf("recovery apply: %v", err)
	}
	if !applied.Applied || !applied.Validation.Valid {
		t.Fatalf("applied = %+v", applied)
	}
	assertMigratedToLayout2(t, repo)
	if got := gitignoreBytes(t, repo); got != string(want) {
		t.Fatalf("recovered .gitignore = %q, want the accepted rule %q", got, want)
	}
	if got := readRepoBytes(t, filepath.Join(repo, "unrelated.txt")); string(got) != "keep me\n" {
		t.Fatalf("recovery disturbed an unrelated user file: %q", got)
	}
}
