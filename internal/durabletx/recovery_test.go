package durabletx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRecoverClearsPreparedFence(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	seed(t, repo, "planning/STATE.md", "before")
	testHookAfterPhase = func(phase Phase) error {
		if phase == PhasePublishing {
			return errors.New("interrupted before publication")
		}
		return nil
	}
	t.Cleanup(func() { testHookAfterPhase = nil })

	_, err := Run(context.Background(), lock, repo, request("init", member("planning/STATE.md", "after")))
	id := txError(t, err).TransactionID
	preview, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id})
	if err != nil {
		t.Fatalf("preview recovery: %v", err)
	}
	if preview.Action != ClearFence || preview.Applied {
		t.Fatalf("preview = %+v, want unapplied clear_fence", preview)
	}
	testHookAfterPhase = nil
	applied, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id, Apply: true})
	if err != nil {
		t.Fatalf("apply recovery: %v", err)
	}
	if !applied.Applied || retained(t, repo) != "" {
		t.Fatalf("applied = %+v, retained = %q", applied, retained(t, repo))
	}
	if got, _ := read(t, repo, "planning/STATE.md"); got != "before" {
		t.Fatalf("state = %q, want original", got)
	}
}

func TestCompletedRecoveryResumesCleanupWithoutOriginalsOrJournal(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	seed(t, repo, "planning/STATE.md", "before")
	testHookAfterPhase = func(phase Phase) error {
		if phase == PhaseCandidatePublished {
			return errors.New("interrupt")
		}
		return nil
	}
	t.Cleanup(func() { testHookAfterPhase = nil })
	_, err := Run(context.Background(), lock, repo, request("init", member("planning/STATE.md", "after")))
	id := txError(t, err).TransactionID
	testHookAfterPhase = nil
	store, err := openStore(lock, repo)
	if err != nil {
		t.Fatal(err)
	}
	doc, entries, _, err := loadRetained(store, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := markComplete(store, id, doc.Command, AcceptCandidate, entries); err != nil {
		t.Fatal(err)
	}
	tx := filepath.Join(TransactionsDir(repo), id)
	if err := os.Remove(filepath.Join(tx, originalsDirName, "00000000")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(tx, manifestName)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(tx, journalName)); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	result, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id, Apply: true})
	if err != nil {
		t.Fatalf("resume cleanup: %v", err)
	}
	if !result.Applied || result.Action != AcceptCandidate || retained(t, repo) != "" {
		t.Fatalf("result = %+v, retained = %q", result, retained(t, repo))
	}
}

func TestRecoverClearsInterruptedPreparationMarker(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	store, err := openStore(lock, repo)
	if err != nil {
		t.Fatal(err)
	}
	id := "0123456789abcdef0123456789abcdef"
	if err := store.ensureDirectory(store.base, store.baseAbsolute, store.transactions); err != nil {
		t.Fatal(err)
	}
	data, err := encodeDocument(journal{TransactionID: id, Command: "init", Phase: PhasePrepared})
	if err != nil {
		t.Fatal(err)
	}
	if err := publishDocument(store.base, preparingPath(store, id), data); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	preview, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id})
	if err != nil || preview.Action != ClearFence {
		t.Fatalf("preview = %+v, err = %v", preview, err)
	}
	if _, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id, Apply: true}); err != nil {
		t.Fatalf("clear preparation: %v", err)
	}
	if retained(t, repo) != "" {
		t.Fatalf("preparation fence retained")
	}
}

func TestRecoverResumesAfterFenceDirectoryMove(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	testHookAfterFenceMove = func() error { return errors.New("interrupted after fence move") }
	t.Cleanup(func() { testHookAfterFenceMove = nil })
	_, err := Run(context.Background(), lock, repo, request("init", member("planning/STATE.md", "after")))
	txErr := txError(t, err)
	if txErr.Kind != KindRecovery {
		t.Fatalf("kind = %q, want recovery_pending", txErr.Kind)
	}
	testHookAfterFenceMove = nil
	result, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: txErr.TransactionID, Apply: true})
	if err != nil {
		t.Fatalf("resume after fence move: %v", err)
	}
	if !result.Applied || result.Action != AcceptCandidate || retained(t, repo) != "" {
		t.Fatalf("result = %+v, retained = %q", result, retained(t, repo))
	}
}

func TestRunMapsManagedMembersToLocalStorage(t *testing.T) {
	repo := newLocalRepository(t)
	lock := acquire(t, repo, ownerCapability())
	if _, err := Run(context.Background(), lock, repo, request("init", member("planning/STATE.md", "local"))); err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(repo.StorageRoot(), "planning", "STATE.md"))
	if err != nil || string(data) != "local" {
		t.Fatalf("local managed bytes = %q, err = %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(repo.Root, "planning", "STATE.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("committed decoy exists: %v", err)
	}
}

func TestDelegatedLocalWriteCannotReinterpretManagedGrantAsWorktree(t *testing.T) {
	repo := newLocalRepository(t)
	lock := acquire(t, repo, ownerCapability())
	capability := ownerCapability()
	capability.SelectedTask = "T-1"
	capability.Writes = []string{"planning/STATE.md"}
	own := delegatedOwnership{Lock: lock, capability: capability}
	req := request("init", Member{Kind: Worktree, Reported: "planning/STATE.md", Path: "planning/STATE.md", Content: []byte("bad")})
	req.SelectedTask = "T-1"
	_, err := Run(context.Background(), own, repo, req)
	if got := txError(t, err); got.Kind != KindRefused {
		t.Fatalf("kind = %q, want refused", got.Kind)
	}
}

func TestRunRejectsRepositoryContextNotBoundToLock(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	forged := repo
	forged.GitCommonDir = t.TempDir()
	_, err := Run(context.Background(), lock, forged, request("init", member("planning/STATE.md", "after")))
	if got := txError(t, err); got.Kind != KindRefused {
		t.Fatalf("kind = %q, want refused", got.Kind)
	}
}

func TestRecoverRequiresMatchingLockTransactionIdentity(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	_, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	if got := txError(t, err); got.Kind != KindRefused {
		t.Fatalf("kind = %q, want refused", got.Kind)
	}
}

func TestRollingBackRetryAcceptsTaskrailRestoredIdentity(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	seed(t, repo, "planning/A.md", "old-a")
	seed(t, repo, "planning/B.md", "old-b")
	testHookAfterMember = func(phase Phase, _ string) error {
		switch phase {
		case PhasePublishing:
			return errors.New("stop publication")
		case PhaseRollingBack:
			return errors.New("stop after restored bytes")
		default:
			return nil
		}
	}
	t.Cleanup(func() { testHookAfterMember = nil })
	_, err := Run(context.Background(), lock, repo, request("init",
		member("planning/A.md", "new-a"), member("planning/B.md", "new-b")))
	id := txError(t, err).TransactionID
	testHookAfterMember = nil
	preview, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id})
	if err != nil || preview.Action != RestoreOriginal {
		t.Fatalf("preview = %+v, err = %v", preview, err)
	}
	if _, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id, Apply: true}); err != nil {
		t.Fatalf("clear restored rollback: %v", err)
	}
}

func TestRecoverRefusesMalformedExistingCompletion(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	testHookAfterPhase = func(phase Phase) error {
		if phase == PhaseCandidatePublished {
			return errors.New("interrupt")
		}
		return nil
	}
	t.Cleanup(func() { testHookAfterPhase = nil })
	_, err := Run(context.Background(), lock, repo, request("init", member("planning/STATE.md", "after")))
	id := txError(t, err).TransactionID
	testHookAfterPhase = nil
	complete := filepath.Join(TransactionsDir(repo), id, completionName)
	if err := os.WriteFile(complete, []byte(`{"action":"accept_candidate"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id, Apply: true}); err == nil {
		t.Fatal("Recover ignored malformed existing completion")
	}
	if retained(t, repo) != id {
		t.Fatal("malformed completion fence was cleared")
	}
}

func TestRecoverRestoresMixedPublication(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	seed(t, repo, "planning/A.md", "old-a")
	seed(t, repo, "planning/B.md", "old-b")
	testHookAfterMember = func(phase Phase, _ string) error {
		if phase == PhasePublishing {
			panic("simulated process death")
		}
		return nil
	}
	t.Cleanup(func() { testHookAfterMember = nil })
	func() {
		defer func() { _ = recover() }()
		_, _ = Run(context.Background(), lock, repo, request("init",
			member("planning/A.md", "new-a"), member("planning/B.md", "new-b")))
	}()
	testHookAfterMember = nil
	id := retained(t, repo)
	preview, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id})
	if err != nil {
		t.Fatalf("preview recovery: %v", err)
	}
	if preview.Action != RestoreOriginal {
		t.Fatalf("action = %q, want restore_original", preview.Action)
	}
	if _, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id, Apply: true}); err != nil {
		t.Fatalf("apply recovery: %v", err)
	}
	if a, _ := read(t, repo, "planning/A.md"); a != "old-a" {
		t.Fatalf("A = %q, want old-a", a)
	}
	if b, _ := read(t, repo, "planning/B.md"); b != "old-b" {
		t.Fatalf("B = %q, want old-b", b)
	}
}

func TestRecoverAcceptsCompleteValidatedCandidate(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	seed(t, repo, "planning/STATE.md", "before")
	testHookAfterPhase = func(phase Phase) error {
		if phase == PhaseCandidatePublished {
			return errors.New("interrupted after publication")
		}
		return nil
	}
	t.Cleanup(func() { testHookAfterPhase = nil })
	_, err := Run(context.Background(), lock, repo, request("init", member("planning/STATE.md", "after")))
	id := txError(t, err).TransactionID
	testHookAfterPhase = nil
	validated := 0
	validator := func(command string, snapshots []Evidence) error {
		validated++
		if command != "init" || len(snapshots) != 1 {
			t.Fatalf("validator input = %q %+v", command, snapshots)
		}
		return nil
	}
	preview, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id, Validate: validator})
	if err != nil || preview.Action != AcceptCandidate {
		t.Fatalf("preview = %+v, err = %v", preview, err)
	}
	applied, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id, Apply: true, Validate: validator})
	if err != nil {
		t.Fatalf("apply recovery: %v", err)
	}
	if !applied.Applied || validated != 3 {
		t.Fatalf("applied = %+v, validator calls = %d", applied, validated)
	}
}

func TestRecoverRefusesUnexpectedBytesWithoutOverwrite(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	seed(t, repo, "planning/STATE.md", "before")
	testHookAfterPhase = func(phase Phase) error {
		if phase == PhasePublishing {
			return errors.New("interrupt")
		}
		return nil
	}
	t.Cleanup(func() { testHookAfterPhase = nil })
	_, err := Run(context.Background(), lock, repo, request("init", member("planning/STATE.md", "after")))
	id := txError(t, err).TransactionID
	testHookAfterPhase = nil
	physical := filepath.Join(repo.Root, "planning", "STATE.md")
	if err := os.WriteFile(physical, []byte("external"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id, Apply: true})
	if got := txError(t, err); got.Kind != KindConflict {
		t.Fatalf("kind = %q, want conflict", got.Kind)
	}
	if data, err := os.ReadFile(physical); err != nil || string(data) != "external" {
		t.Fatalf("external bytes = %q, err = %v", data, err)
	}
}

func TestRestoreRetryRefusesUnexpectedBytesWithoutOverwrite(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	seed(t, repo, "planning/A.md", "old-a")
	seed(t, repo, "planning/B.md", "old-b")
	testHookAfterMember = func(phase Phase, _ string) error {
		if phase == PhasePublishing {
			panic("simulated writer death")
		}
		return nil
	}
	t.Cleanup(func() { testHookAfterMember = nil })
	func() {
		defer func() { _ = recover() }()
		_, _ = Run(context.Background(), lock, repo, request("init",
			member("planning/A.md", "new-a"), member("planning/B.md", "new-b")))
	}()
	id := retained(t, repo)
	testHookAfterMember = func(phase Phase, _ string) error {
		if phase == PhaseRecoveryRestoring {
			return errors.New("simulated recovery death")
		}
		return nil
	}
	if _, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id, Apply: true}); err == nil {
		t.Fatal("interrupted recovery succeeded")
	}
	testHookAfterMember = nil
	physical := filepath.Join(repo.Root, "planning", "A.md")
	if err := os.WriteFile(physical, []byte("external"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id, Apply: true})
	if got := txError(t, err); got.Kind != KindConflict {
		t.Fatalf("kind = %q, want conflict", got.Kind)
	}
	if data, err := os.ReadFile(physical); err != nil || string(data) != "external" {
		t.Fatalf("external bytes = %q, err = %v", data, err)
	}
}

func TestAcceptRecoveryRechecksWholeSetAfterValidation(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	seed(t, repo, "planning/STATE.md", "before")
	testHookAfterPhase = func(phase Phase) error {
		if phase == PhaseCandidatePublished {
			return errors.New("interrupt")
		}
		return nil
	}
	t.Cleanup(func() { testHookAfterPhase = nil })
	_, err := Run(context.Background(), lock, repo, request("init", member("planning/STATE.md", "after")))
	id := txError(t, err).TransactionID
	testHookAfterPhase = nil
	physical := filepath.Join(repo.Root, "planning", "STATE.md")
	validationCalls := 0
	validator := func(string, []Evidence) error {
		validationCalls++
		if validationCalls == 2 {
			return os.WriteFile(physical, []byte("external"), 0o644)
		}
		return nil
	}
	_, err = Recover(context.Background(), lock, repo, RecoveryRequest{TransactionID: id, Apply: true, Validate: validator})
	if got := txError(t, err); got.Kind != KindConflict {
		t.Fatalf("kind = %q, want conflict", got.Kind)
	}
	if retained(t, repo) != id {
		t.Fatalf("fence was cleared after validation race")
	}
}
