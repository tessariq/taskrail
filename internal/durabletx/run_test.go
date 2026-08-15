package durabletx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tessariq/taskrail/internal/durablefs"
	"github.com/tessariq/taskrail/internal/repolock"
)

func TestRunPublishesTheCompleteSetAndClearsTheFence(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	seed(t, repo, "planning/STATE.md", "before")

	result, err := Run(context.Background(), lock, repo, request("init",
		member("planning/STATE.md", "after"),
		member("planning/tasks/T-1.md", "created"),
	))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if state, _ := read(t, repo, "planning/STATE.md"); state != "after" {
		t.Fatalf("planning/STATE.md = %q, want the candidate", state)
	}
	if created, ok := read(t, repo, "planning/tasks/T-1.md"); !ok || created != "created" {
		t.Fatalf("planning/tasks/T-1.md = %q (present=%t), want the candidate", created, ok)
	}
	if id := retained(t, repo); id != "" {
		t.Fatalf("fence %q retained after a completed transaction", id)
	}
	if result.TransactionID != *lock.Owner().TransactionID || result.Phase != PhaseCandidatePublished {
		t.Fatalf("result = %+v, want a committed transaction identity", result)
	}
}

func TestRunRequiresTransactionIdentityBoundToLock(t *testing.T) {
	repo := newRepository(t)
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: repo,
		Command:    "init",
		Capability: ownerCapability(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lock.Release() })
	if _, err := Run(context.Background(), lock, repo, request("init", member("planning/STATE.md", "after"))); err == nil {
		t.Fatal("Run accepted a lock without transaction identity")
	}
}

func TestRunRejectsReportedPathThatDiffersFromPublicationPath(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	_, err := Run(context.Background(), lock, repo, request("init", Member{
		Kind: Managed, Reported: "planning/STATE.md", Path: "planning/other.md", Content: []byte("bad"),
	}))
	if err == nil {
		t.Fatal("Run accepted divergent reported and publication paths")
	}
	if _, err := os.Stat(filepath.Join(repo.Root, "planning", "other.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unauthorized path was created: %v", err)
	}
}

func TestRunRechecksConsumedPathsBeforePublication(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	seed(t, repo, "planning/input.md", "input")
	request := request("init", member("planning/STATE.md", "candidate"))
	request.Consumed = []Path{{Kind: Managed, Reported: "planning/input.md", Path: "planning/input.md"}}
	request.Validate = func([]Evidence) error {
		return os.WriteFile(filepath.Join(repo.Root, "planning", "input.md"), []byte("changed"), 0o644)
	}
	_, err := Run(context.Background(), lock, repo, request)
	if err == nil {
		t.Fatal("Run published from a stale consumed path")
	}
	if _, ok := read(t, repo, "planning/STATE.md"); ok {
		t.Fatal("candidate was published after consumed-path conflict")
	}
}

func TestPreparedJournalRecordsCompleteExactEvidence(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	seed(t, repo, "planning/input.md", "input")
	seed(t, repo, "planning/STATE.md", "before")
	req := request("init", member("planning/STATE.md", "after"), member("planning/tasks/T-1.md", "created"))
	req.Consumed = []Path{{Kind: Managed, Reported: "planning/input.md", Path: "planning/input.md"}}
	testHookAfterPhase = func(phase Phase) error {
		if phase == PhasePrepared {
			return errors.New("inspect prepared evidence")
		}
		return nil
	}
	t.Cleanup(func() { testHookAfterPhase = nil })
	_, err := Run(context.Background(), lock, repo, req)
	id := txError(t, err).TransactionID
	store, err := openStore(lock, repo)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var saved manifest
	if err := store.readDocument(store.transactionDir(id)+"/"+manifestName, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.TransactionID != id || saved.Command != "init" || len(saved.Members) != 3 {
		t.Fatalf("manifest = %+v", saved)
	}
	if saved.Members[0].Reported != "planning/STATE.md" || saved.Members[1].Reported != "planning/input.md" || saved.Members[2].Reported != "planning/tasks/T-1.md" {
		t.Fatalf("manifest ordering = %+v", saved.Members)
	}
	state := saved.Members[0]
	if state.Original == nil || state.Original.Identity == nil || state.Original.SHA256 != digest([]byte("before")) ||
		state.Candidate == nil || state.Candidate.SHA256 != digest([]byte("after")) || len(state.Ancestors) == 0 {
		t.Fatalf("existing evidence = %+v", state)
	}
	consumed := saved.Members[1]
	if consumed.Published || consumed.Candidate != nil || consumed.Original == nil || consumed.Original.SHA256 != digest([]byte("input")) {
		t.Fatalf("consumed evidence = %+v", consumed)
	}
	created := saved.Members[2]
	if created.Original != nil || created.Candidate == nil || created.Candidate.SHA256 != digest([]byte("created")) {
		t.Fatalf("absent evidence = %+v", created)
	}
	original, _, err := durablefs.ReadFile(store.baseAbsolute, store.transactionDir(id)+"/"+originalsDirName+"/00000000", maximumJournalBytes)
	if err != nil || string(original) != "before" {
		t.Fatalf("recorded original = %q, err = %v", original, err)
	}
}

func TestRunCancellationAfterPublicationRollsBackAndClearsFence(t *testing.T) {
	repo := newRepository(t)
	lock := acquire(t, repo, ownerCapability())
	seed(t, repo, "planning/STATE.md", "before")
	ctx, cancel := context.WithCancel(context.Background())
	validationCalls := 0
	req := request("init", member("planning/STATE.md", "after"))
	req.Validate = func([]Evidence) error {
		validationCalls++
		if validationCalls == 2 {
			cancel()
		}
		return nil
	}
	_, err := Run(ctx, lock, repo, req)
	if err == nil {
		t.Fatal("Run ignored cancellation after publication")
	}
	if got, _ := read(t, repo, "planning/STATE.md"); got != "before" {
		t.Fatalf("state = %q, want rolled-back original", got)
	}
	if retained(t, repo) != "" {
		t.Fatal("canceled transaction retained a fence after successful rollback")
	}
}
