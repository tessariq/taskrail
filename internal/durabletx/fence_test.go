package durabletx

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/tessariq/taskrail/internal/repolock"
)

// The fence member: one published path that the transaction temporarily
// publishes with intermediate fence bytes before any other semantic byte, and
// whose final candidate bytes publish as the transaction's last semantic
// operation (specs/v0.5.0.md#layout-compatibility-and-upgrade). Every test here
// proves one invariant of that two-stage publication through the shared journal,
// publication, and recovery machinery.

func fencedMarker(old, fence, final string) Member {
	return Member{Kind: Managed, Reported: ".taskrail/config.yml", Path: ".taskrail/config.yml",
		Content: []byte(final), Fence: []byte(fence)}
}

func TestRunPublishesFenceBeforeSemanticBytesAndFinalMarkerLast(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	seed(t, repo, ".taskrail/config.yml", "layout 1")
	seed(t, repo, "planning/STATE.md", "state 1")

	// The publishing phase begins after the fence bytes landed and before any
	// semantic byte changed, which is the exact window the fence exists for.
	testHookAfterPhase = func(phase Phase) error {
		if phase == PhasePublishing {
			if got, _ := read(t, repo, ".taskrail/config.yml"); got != "fenced" {
				t.Errorf("marker = %q at publishing phase, want the fence bytes", got)
			}
			if got, _ := read(t, repo, "planning/STATE.md"); got != "state 1" {
				t.Errorf("state = %q at publishing phase, want the original", got)
			}
		}
		return nil
	}
	t.Cleanup(func() { testHookAfterPhase = nil })

	validations := 0
	request := request("init", fencedMarker("layout 1", "fenced", "final"), member("planning/STATE.md", "state 2"))
	request.Validate = func(snapshots []Evidence) error {
		validations++
		if validations != 2 {
			return nil
		}
		// The post-publication validation runs before the final marker bytes
		// publish, so the marker member still reports the fence digest.
		for _, snapshot := range snapshots {
			if snapshot.Reported != ".taskrail/config.yml" {
				continue
			}
			if snapshot.CurrentSHA256 != snapshot.FenceSHA256 {
				t.Errorf("marker current = %s, want the fence digest %s during validation",
					snapshot.CurrentSHA256, snapshot.FenceSHA256)
			}
		}
		return nil
	}
	if _, err := Run(context.Background(), lock, repo, request); err != nil {
		t.Fatalf("Run: %v", err)
	}
	testHookAfterPhase = nil
	if validations != 2 {
		t.Fatalf("validate ran %d times, want preview plus post-publication", validations)
	}
	if got, _ := read(t, repo, ".taskrail/config.yml"); got != "final" {
		t.Fatalf("marker = %q after Run, want the final candidate", got)
	}
	if got, _ := read(t, repo, "planning/STATE.md"); got != "state 2" {
		t.Fatalf("state = %q after Run, want the candidate", got)
	}
	if id := retained(t, repo); id != "" {
		t.Fatalf("fence %q retained after a completed transaction", id)
	}
}

func TestRunFenceMemberRestoresLastOnHandledFailure(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	seed(t, repo, ".taskrail/config.yml", "layout 1")
	seed(t, repo, "planning/STATE.md", "state 1")
	seed(t, repo, "planning/NOTES.md", "notes")

	var restored []string
	testHookAfterMember = func(phase Phase, reported string) error {
		if phase == PhaseRollingBack {
			restored = append(restored, reported)
		}
		if phase == PhasePublishing && reported == "planning/NOTES.md" {
			return errors.New("injected failure after the notes candidate published")
		}
		return nil
	}
	t.Cleanup(func() { testHookAfterMember = nil })

	_, err := Run(context.Background(), lock, repo, request("init",
		fencedMarker("layout 1", "fenced", "final"),
		member("planning/NOTES.md", "notes 2"),
		member("planning/STATE.md", "state 2"),
	))
	txErr := txError(t, err)
	if txErr.Kind != KindRolledBack {
		t.Fatalf("kind = %q, want rolled_back", txErr.Kind)
	}
	testHookAfterMember = nil
	for reported, want := range map[string]string{
		".taskrail/config.yml": "layout 1",
		"planning/STATE.md":    "state 1",
		"planning/NOTES.md":    "notes",
	} {
		if got, _ := read(t, repo, reported); got != want {
			t.Fatalf("%s = %q, want the original %q", reported, got, want)
		}
	}
	if !slices.Equal(restored, []string{"planning/NOTES.md", ".taskrail/config.yml"}) {
		t.Fatalf("restore order = %v, want candidate-written files before the fence member", restored)
	}
	if id := retained(t, repo); id != "" {
		t.Fatalf("fence %q retained after a completed rollback", id)
	}
}

func TestRunRejectsAFenceMemberThatDoesNotSortFirst(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	seed(t, repo, ".taskrail/config.yml", "layout 1")
	_, err := Run(context.Background(), lock, repo, request("init",
		member(".agents/skills/x/SKILL.md", "skill"),
		fencedMarker("layout 1", "fenced", "final"),
	))
	if err == nil {
		t.Fatal("a fence member sorting after another published member must refuse")
	}
}

func TestRunRejectsTwoFenceMembers(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	seed(t, repo, ".taskrail/config.yml", "layout 1")
	_, err := Run(context.Background(), lock, repo, request("init",
		fencedMarker("layout 1", "fenced", "final"),
		Member{Kind: Managed, Reported: ".taskrail/other.yml", Path: ".taskrail/other.yml",
			Content: []byte("x"), Fence: []byte("y")},
	))
	if err == nil {
		t.Fatal("two fence members must refuse")
	}
}

// interruptAtPhase fails the transaction at one durable phase so the retained
// journal state models an interruption there, then reports the transaction id.
// The lock is released before returning: recovery must acquire its own lock
// naming the retained transaction, like the operator's next process would.
func interruptAtPhase(t *testing.T, repo repolock.Repository, at Phase, request Request) string {
	t.Helper()
	lock := acquire(t, repo, ownerCapability())
	testHookAfterPhase = func(phase Phase) error {
		if phase == at {
			return errors.New("interrupted at " + string(at))
		}
		return nil
	}
	t.Cleanup(func() { testHookAfterPhase = nil })
	_, err := Run(context.Background(), lock, repo, request)
	id := txError(t, err).TransactionID
	testHookAfterPhase = nil
	if err := lock.Release(); err != nil {
		t.Fatalf("release interrupted lock: %v", err)
	}
	return id
}

func TestRecoverRestoresFenceBytesPublishedBeforePublishing(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, ".taskrail/config.yml", "layout 1")
	seed(t, repo, "planning/STATE.md", "state 1")
	id := interruptAtPhase(t, repo, PhasePublishing, request("init",
		fencedMarker("layout 1", "fenced", "final"), member("planning/STATE.md", "state 2")))
	if got, _ := read(t, repo, ".taskrail/config.yml"); got != "fenced" {
		t.Fatalf("marker = %q after the interruption, want the retained fence bytes", got)
	}
	lock := acquire(t, repo, ownerCapability())
	preview, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Action != RestoreOriginal {
		t.Fatalf("action = %q, want restore_original", preview.Action)
	}
	applied, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id, Apply: true})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !applied.Applied || retained(t, repo) != "" {
		t.Fatalf("applied = %+v retained = %q", applied, retained(t, repo))
	}
	if got, _ := read(t, repo, ".taskrail/config.yml"); got != "layout 1" {
		t.Fatalf("marker = %q, want the original", got)
	}
	if got, _ := read(t, repo, "planning/STATE.md"); got != "state 1" {
		t.Fatalf("state = %q, want the original", got)
	}
}

func TestRecoverAcceptsValidatedFencePendingCandidate(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, ".taskrail/config.yml", "layout 1")
	seed(t, repo, "planning/STATE.md", "state 1")
	// Interrupting at validating leaves the semantic candidates published while
	// the marker still holds the fence bytes; the marker member reports the
	// fence digest as its current state.
	id := interruptAtPhase(t, repo, PhaseValidating, request("init",
		fencedMarker("layout 1", "fenced", "final"), member("planning/STATE.md", "state 2")))
	if got, _ := read(t, repo, ".taskrail/config.yml"); got != "fenced" {
		t.Fatalf("marker = %q after the interruption, want the fence bytes", got)
	}
	if got, _ := read(t, repo, "planning/STATE.md"); got != "state 2" {
		t.Fatalf("state = %q after the interruption, want the candidate", got)
	}
	lock := acquire(t, repo, ownerCapability())
	var snapshot Evidence
	validated := 0
	request := RecoveryRequest{TransactionID: id, Validate: func(command string, snapshots []Evidence) error {
		for _, item := range snapshots {
			if item.Reported == ".taskrail/config.yml" {
				snapshot = item
			}
		}
		if snapshot.FenceSHA256 == "" {
			t.Fatal("fence member evidence does not report the fence digest")
		}
		// The preview and the apply's pre-check validate the fence-pending
		// candidate; only the apply's post-check runs after the retained
		// finals completed the marker.
		validated++
		want := snapshot.FenceSHA256
		if validated > 2 {
			want = snapshot.CandidateSHA256
		}
		if snapshot.CurrentSHA256 != want {
			t.Fatalf("marker current = %s, want %s", snapshot.CurrentSHA256, want)
		}
		return nil
	}}
	preview, err := Recover(context.Background(), lock, repo, request)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Action != AcceptCandidate {
		t.Fatalf("action = %q, want accept_candidate", preview.Action)
	}
	request.Apply = true
	if _, err := Recover(context.Background(), lock, repo, request); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, _ := read(t, repo, ".taskrail/config.yml"); got != "final" {
		t.Fatalf("marker = %q, want the completed final candidate", got)
	}
	if id := retained(t, repo); id != "" {
		t.Fatalf("fence %q retained after recovery", id)
	}
}

func TestRecoverClearsFencePhaseWithoutFenceBytes(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, ".taskrail/config.yml", "layout 1")
	// Interrupting exactly at the fence-published phase transition can leave the
	// marker untouched when the fence bytes had not yet published; nothing was
	// written, so the single safe action clears the retained transaction.
	id := interruptAtPhase(t, repo, PhaseFencePublished, request("init",
		fencedMarker("layout 1", "fenced", "final"), member("planning/STATE.md", "state 2")))
	lock := acquire(t, repo, ownerCapability())
	preview, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Action != ClearFence {
		t.Fatalf("action = %q, want clear_fence", preview.Action)
	}
	if _, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id, Apply: true}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, _ := read(t, repo, ".taskrail/config.yml"); got != "layout 1" {
		t.Fatalf("marker = %q, want the untouched original", got)
	}
}

func TestRecoverCompletesFenceFinalFromRetainedFinals(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, ".taskrail/config.yml", "layout 1")
	seed(t, repo, "planning/STATE.md", "state 1")
	// An accept that was itself interrupted between its phase transition and
	// the final publication retries mechanically from the retained finals.
	id := interruptAtPhase(t, repo, PhaseValidating, request("init",
		fencedMarker("layout 1", "fenced", "final"), member("planning/STATE.md", "state 2")))
	lock := acquire(t, repo, ownerCapability())
	store, err := openStore(lock, repo)
	if err != nil {
		t.Fatal(err)
	}
	doc, _, _, err := loadRetained(store, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.advance(id, doc, PhaseRecoveryAccepting); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id, Apply: true,
		Validate: func(string, []Evidence) error { return nil }}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, _ := read(t, repo, ".taskrail/config.yml"); got != "final" {
		t.Fatalf("marker = %q, want the retained final completed", got)
	}
}

func TestRecoverResumesInterruptedFenceRollback(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, ".taskrail/config.yml", "layout 1")
	seed(t, repo, "planning/STATE.md", "state 1")
	// A rollback interrupted after restoring the semantic candidates but before
	// the fence member (which restores last) leaves the journal rolling_back
	// with the marker still fenced. Recovery must retry the same restore, not
	// strand the repository on bytes no branch accepts.
	var restoreHook func(phase Phase, reported string) error
	restoreHook = func(phase Phase, reported string) error {
		if phase == PhasePublishing && reported == "planning/STATE.md" {
			return errors.New("injected publication failure")
		}
		if phase == PhaseRollingBack && reported == "planning/STATE.md" {
			// Fail the rollback after the semantic restore, while the fence
			// member still holds its fence bytes, and do not fail again.
			testHookAfterMember = nil
			return errors.New("interrupted rollback before the fence restore")
		}
		return nil
	}
	testHookAfterMember = restoreHook
	t.Cleanup(func() { testHookAfterMember = nil })

	lock := acquire(t, repo, ownerCapability())
	_, err := Run(context.Background(), lock, repo, request("init",
		fencedMarker("layout 1", "fenced", "final"), member("planning/STATE.md", "state 2")))
	id := txError(t, err).TransactionID
	testHookAfterMember = nil
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	if got, _ := read(t, repo, ".taskrail/config.yml"); got != "fenced" {
		t.Fatalf("marker = %q after the interrupted rollback, want the fence bytes", got)
	}
	if got, _ := read(t, repo, "planning/STATE.md"); got != "state 1" {
		t.Fatalf("state = %q after the interrupted rollback, want the restored original", got)
	}

	lock = acquire(t, repo, ownerCapability())
	preview, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Action != RestoreOriginal {
		t.Fatalf("action = %q, want restore_original", preview.Action)
	}
	if _, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id, Apply: true}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, _ := read(t, repo, ".taskrail/config.yml"); got != "layout 1" {
		t.Fatalf("marker = %q, want the original restored last", got)
	}
	if id := retained(t, repo); id != "" {
		t.Fatalf("fence %q retained after recovery", id)
	}
}

func TestRetainedCandidateReturnsFenceFinalBytes(t *testing.T) {
	repo := newRepository(t)
	seed(t, repo, ".taskrail/config.yml", "layout 1")
	seed(t, repo, "planning/STATE.md", "state 1")
	id := interruptAtPhase(t, repo, PhaseValidating, request("init",
		fencedMarker("layout 1", "fenced", "final"), member("planning/STATE.md", "state 2")))
	final, err := RetainedCandidate(repo, id, Managed, ".taskrail/config.yml")
	if err != nil {
		t.Fatalf("retained final: %v", err)
	}
	if string(final) != "final" {
		t.Fatalf("retained final = %q, want the candidate bytes", final)
	}
	if _, err := RetainedCandidate(repo, id, Managed, "planning/STATE.md"); err == nil {
		t.Fatal("a member without fence bytes retains no final to return")
	}
}
