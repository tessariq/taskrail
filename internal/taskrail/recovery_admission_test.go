package taskrail

import (
	"os"
	"path/filepath"
	"testing"
)

const testTransactionID = "0123456789abcdef0123456789abcdef"

func TestRecoveryAdmissionUsesCanonicalRepositoryRoot(t *testing.T) {
	t.Run("git committed and local share common root", func(t *testing.T) {
		repo := realGitRepo(t)
		linked := filepath.Join(t.TempDir(), "linked")
		runGit(t, repo, "worktree", "add", "-b", "recovery-fence", linked)
		writeFile(t, filepath.Join(linked, ".taskrail", "config.yml"), layout2Marker("local", "specs", "planning"))
		seedFixtureTree(t, filepath.Join(linked, localStorageRoot))
		writeRecoveryJournal(t, filepath.Join(repo, ".git", "taskrail"), "status")

		_, err := NewService(linked)
		assertRecoveryPending(t, err, true)
	})

	t.Run("non git committed", func(t *testing.T) {
		repo := t.TempDir()
		writeFile(t, filepath.Join(repo, ".taskrail", "config.yml"), layout2Marker("committed", "specs", "planning"))
		seedFixtureTree(t, repo)
		writeRecoveryJournal(t, filepath.Join(repo, ".taskrail", "runtime"), "validate")

		_, err := NewService(repo)
		assertRecoveryPending(t, err, true)
	})
}

func TestRecoveryAdmissionFailsClosedForMalformedStateAndDecoys(t *testing.T) {
	for name, journal := range map[string]*string{
		"missing journal":        nil,
		"malformed journal":      stringPointer("not json"),
		"unknown member":         stringPointer(`{"transaction_id":"` + testTransactionID + `","command":"status","phase":"prepared","extra":true}`),
		"duplicate member":       stringPointer(`{"transaction_id":"` + testTransactionID + `","command":"status","command":"start","phase":"prepared"}`),
		"noncanonical command":   stringPointer(`{"transaction_id":"` + testTransactionID + `","command":"taskrail status","phase":"prepared"}`),
		"mismatched transaction": stringPointer(`{"transaction_id":"ffffffffffffffffffffffffffffffff","command":"status","phase":"prepared"}`),
	} {
		t.Run(name, func(t *testing.T) {
			repo := seedFixtureRepo(t)
			canonical := filepath.Join(repo, ".git", "taskrail", "transactions", testTransactionID)
			if err := os.MkdirAll(canonical, 0o755); err != nil {
				t.Fatal(err)
			}
			if journal != nil {
				writeFile(t, filepath.Join(canonical, "journal.json"), *journal)
			}
			writeRecoveryJournal(t, filepath.Join(repo, ".taskrail", "runtime"), "status")

			_, err := NewService(repo)
			assertRecoveryPending(t, err, false)
		})
	}
}

func TestRecoveryAdmissionDetectsReplacementAcrossOperation(t *testing.T) {
	repo := seedFixtureRepo(t)
	svc, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	writeRecoveryJournal(t, filepath.Join(repo, ".git", "taskrail"), "status")
	if err := svc.CheckRecovery(); err == nil {
		t.Fatal("CheckRecovery succeeded after transaction appeared")
	} else {
		assertRecoveryPending(t, err, true)
	}
}

func TestRecoveryAdmissionRefusesLinkedAndSpecialTransactionState(t *testing.T) {
	for _, kind := range []string{"symlink", "file"} {
		t.Run(kind, func(t *testing.T) {
			repo := seedFixtureRepo(t)
			root := filepath.Join(repo, ".git", "taskrail")
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "transactions")
			var err error
			if kind == "symlink" {
				err = os.Symlink(t.TempDir(), path)
			} else {
				err = os.WriteFile(path, []byte("special"), 0o644)
			}
			if err != nil {
				t.Fatal(err)
			}
			_, got := NewService(repo)
			assertRecoveryPending(t, got, false)
		})
	}
}

func assertRecoveryPending(t *testing.T, err error, evidence bool) {
	t.Helper()
	if err == nil || err.Error() != "repository recovery is pending" {
		t.Fatalf("error = %v, want exact recovery refusal", err)
	}
	failure := MachineFailureFor(err)
	if failure.Code != MachineCodeRecoveryPending || (failure.Recovery != nil) != evidence {
		t.Fatalf("failure = %+v, evidence=%t", failure, evidence)
	}
	if evidence && (failure.Recovery.TransactionID != testTransactionID || failure.Recovery.Phase != "prepared") {
		t.Fatalf("recovery evidence = %+v", failure.Recovery)
	}
}

func writeRecoveryJournal(t *testing.T, root, command string) {
	t.Helper()
	dir := filepath.Join(root, "transactions", testTransactionID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "journal.json"), `{"transaction_id":"`+testTransactionID+`","command":"`+command+`","phase":"prepared"}`)
}

func stringPointer(value string) *string { return &value }
