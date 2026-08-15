package durabletx

import (
	"testing"

	"github.com/tessariq/taskrail/internal/repolock"
)

func TestDecodeDocumentRejectsDuplicateMembersAndTrailingValues(t *testing.T) {
	for _, data := range []string{
		`{"transaction_id":"a","transaction_id":"b","command":"init","phase":"prepared"}`,
		`{"transaction_id":"a","command":"init","phase":"prepared"} {}`,
	} {
		var doc journal
		if err := decodeDocument([]byte(data), &doc); err == nil {
			t.Fatalf("decodeDocument(%s) succeeded", data)
		}
	}
}

func TestDecodeDocumentRequiresEveryPersistedMember(t *testing.T) {
	valid := `{"transaction_id":"0123456789abcdef0123456789abcdef","command":"init","members":[{"kind":"managed","reported":"planning/STATE.md","path":"planning/STATE.md","published":true,"ancestors":[{"volume":1,"file":2,"mount":3}],"original":null,"candidate":{"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","mode":420,"identity":null}}]}`
	missing := []string{
		`{"command":"init","members":[]}`,
		`{"transaction_id":"0123456789abcdef0123456789abcdef","command":"init","members":[{"kind":"managed","reported":"planning/STATE.md","path":"planning/STATE.md","ancestors":[],"original":null,"candidate":null}]}`,
		`{"transaction_id":"0123456789abcdef0123456789abcdef","command":"init","members":[{"kind":"managed","reported":"planning/STATE.md","path":"planning/STATE.md","published":true,"ancestors":[{"volume":1,"file":2}],"original":null,"candidate":null}]}`,
		`{"transaction_id":"0123456789abcdef0123456789abcdef","command":"init","members":[{"kind":"managed","reported":"planning/STATE.md","path":"planning/STATE.md","published":true,"ancestors":[{"volume":1,"file":2,"mount":3}],"original":null,"candidate":{"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","identity":null}}]}`,
	}
	var decoded manifest
	if err := decodeDocument([]byte(valid), &decoded); err != nil {
		t.Fatalf("valid manifest shape: %v", err)
	}
	for _, data := range missing {
		if err := decodeDocument([]byte(data), &decoded); err == nil {
			t.Fatalf("missing-member document succeeded: %s", data)
		}
	}
}

func TestCanonicalPhaseOrderRejectsSkippedAndReplacedRecoveryActions(t *testing.T) {
	allowed := [][2]Phase{
		{PhasePrepared, PhaseFencePublished},
		{PhaseFencePublished, PhasePublishing},
		{PhasePublishing, PhaseCandidatePublished},
		{PhasePublishing, PhaseRollingBack},
		{PhaseCandidatePublished, PhaseValidating},
		{PhaseCandidatePublished, PhaseRollingBack},
		{PhaseValidating, PhaseRollingBack},
		{PhasePublishing, PhaseRecoveryRestoring},
	}
	for _, transition := range allowed {
		if !canAdvance(transition[0], transition[1]) {
			t.Fatalf("canonical transition %q -> %q refused", transition[0], transition[1])
		}
	}
	for _, transition := range [][2]Phase{
		{PhasePrepared, PhasePublishing},
		{PhaseValidating, PhaseCandidatePublished},
		{PhaseRecoveryRestoring, PhaseRecoveryAccepting},
		{PhaseRecoveryClearing, PhaseRecoveryClearing},
	} {
		if canAdvance(transition[0], transition[1]) {
			t.Fatalf("noncanonical transition %q -> %q accepted", transition[0], transition[1])
		}
	}
}

func TestManifestRejectsTwoKindsMappedToOnePhysicalPath(t *testing.T) {
	repo := repolock.Repository{Root: t.TempDir(), Mode: repolock.ModeCommitted}
	state := fileState{SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Mode: 0o644}
	ancestors := []identity{{Volume: 1, File: 2, Mount: 3}}
	saved := manifest{
		TransactionID: "0123456789abcdef0123456789abcdef",
		Command:       "init",
		Members: []manifestMember{
			{Kind: Managed, Reported: "planning/STATE.md", Path: "planning/STATE.md", Published: true, Ancestors: ancestors, Candidate: &state},
			{Kind: Worktree, Reported: "planning/STATE.md", Path: "planning/STATE.md", Published: true, Ancestors: ancestors, Candidate: &state},
		},
	}
	if err := saved.validate(saved.TransactionID, repo); err == nil {
		t.Fatal("manifest accepted two members mapped to one physical path")
	}
}
