package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/repolock"
)

// A lock and a retained transaction are the ordinary aftermath of an
// interrupted durable writer, and the way out of it runs through `lock status`:
// `recover` refuses that lock and names its operands as the remedy. So every
// retained shape the durable store can leave behind has to remain inspectable —
// a fence `lock status` cannot parse still cannot make the lock unreportable
// (specs/v0.5.0.md#repository-discovery-locking-and-recovery).

const fencedLockID = "77777777777777777777777777777777"

const fencedTxID = "0123456789abcdef0123456789abcdef"

// seedFencedLock writes a lock naming fencedTxID, as a durable transaction's
// lock always does, and returns its raw bytes.
func seedFencedLock(t *testing.T, root string, pid int, host string) []byte {
	t.Helper()
	transaction := fencedTxID
	owner := repolock.Owner{
		LockID:         fencedLockID,
		Command:        "init",
		PID:            pid,
		Host:           host,
		StartedAt:      "2001-02-03T04:05:06Z",
		RepositoryRoot: root,
		StorageMode:    repolock.ModeCommitted,
		StorageRoot:    root,
		TransactionID:  &transaction,
	}
	raw, err := json.Marshal(owner)
	if err != nil {
		t.Fatalf("marshal fenced lock: %v", err)
	}
	path := repolock.LockPath(gitLockRepository(root))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create lock root: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write fenced lock: %v", err)
	}
	return raw
}

// writeRetained places one file beneath the transactions root, relative to it,
// creating parents. It writes the raw shapes the durable store leaves rather
// than going through the store, so each interruption point is reproducible
// without killing a process.
func writeRetained(t *testing.T, root, relative, body string) {
	t.Helper()
	path := filepath.Join(root, ".git", "taskrail", "transactions", filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create retained parent for %s: %v", relative, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write retained %s: %v", relative, err)
	}
}

func preparingMarker() string {
	return `{"transaction_id":"` + fencedTxID + `","command":"init","phase":"prepared"}`
}

func clearingMarker() string {
	return `{"action":"clear_fence","manifest":{"transaction_id":"` + fencedTxID +
		`","command":"init","members":[]}}`
}

// retainedShapes are the transaction-store states an interrupted durable writer
// can leave. The preparing marker is the one T-393 met in practice: preparation
// records every original before it writes the journal, so a large migration
// spends most of its life in exactly this shape. Only "journal" was already
// reported before T-393 — it is the control the other four are read against.
func retainedShapes() map[string]func(t *testing.T, root string) {
	return map[string]func(t *testing.T, root string){
		"journal": func(t *testing.T, root string) {
			writeRetained(t, root, fencedTxID+"/journal.json", preparingMarker())
		},
		"preparing marker": func(t *testing.T, root string) {
			writeRetained(t, root, fencedTxID+".preparing.json", preparingMarker())
		},
		"preparing marker over a partial transaction": func(t *testing.T, root string) {
			writeRetained(t, root, fencedTxID+".preparing.json", preparingMarker())
			writeRetained(t, root, fencedTxID+"/originals/00000000", "before")
		},
		"clearing marker": func(t *testing.T, root string) {
			writeRetained(t, root, fencedTxID+".clearing.json", clearingMarker())
		},
		"unparseable journal": func(t *testing.T, root string) {
			writeRetained(t, root, fencedTxID+"/journal.json", "{")
		},
	}
}

func TestLockStatusReportsTakeoverOperandsForEveryRetainedShape(t *testing.T) {
	for name, seed := range retainedShapes() {
		t.Run(name, func(t *testing.T) {
			root := setupRepo(t)
			raw := seedFencedLock(t, root, deadCLIPID(t), "a-host-that-is-not-this-one")
			seed(t, root)
			before := treeDigests(t, root)

			out, err := runRoot(t, "lock", "status", "--json")
			if err != nil {
				t.Fatalf("fenced lock status: %v (%s)", err, out)
			}
			var status lockStatusJSON
			decodeMachineResult(t, out, &status)
			if !status.Held || status.SHA256 == nil || *status.SHA256 != sha256Digest(raw) {
				t.Fatalf("fenced lock status = %+v", status)
			}
			if status.Owner == nil || status.Owner.TransactionID == nil || *status.Owner.TransactionID != fencedTxID {
				t.Fatalf("fenced lock status owner = %+v", status.Owner)
			}

			text, err := runRoot(t, "lock", "status")
			if err != nil {
				t.Fatalf("fenced lock status text: %v (%s)", err, text)
			}
			if !strings.Contains(text, expectedRecoverGuidance(raw)) {
				t.Fatalf("fenced lock status guidance = %q, want %q", text, expectedRecoverGuidance(raw))
			}
			assertSameTree(t, before, treeDigests(t, root))
		})
	}
}

// The operands are only a way out if they are followable verbatim: what the
// text prints is what recover accepts, with no hand-computed digest and no
// reading of a storage file.
func TestRecoverAcceptsTheOperandsLockStatusReportsUnderAPreparingFence(t *testing.T) {
	root := setupRepo(t)
	seedFencedLock(t, root, deadCLIPID(t), "a-host-that-is-not-this-one")
	writeRetained(t, root, fencedTxID+".preparing.json", preparingMarker())

	text, err := runRoot(t, "lock", "status")
	if err != nil {
		t.Fatalf("fenced lock status: %v (%s)", err, text)
	}
	operands := reportedRecoverArgs(t, text)

	out, err := runRoot(t, append(operands, "--apply", "--json")...)
	if err != nil {
		t.Fatalf("recover with reported operands: %v (%s)", err, out)
	}
	var recovered recoverResultJSON
	decodeMachineResult(t, out, &recovered)
	if !recovered.Applied || recovered.Action != "clear_fence" || recovered.Takeover != "applied" {
		t.Fatalf("recover with reported operands = %+v", recovered)
	}
	if _, err := os.Stat(repolock.LockPath(gitLockRepository(root))); !os.IsNotExist(err) {
		t.Fatalf("recovery left the taken-over lock behind: %v", err)
	}
}

// Reporting the operands is not weakening them: the takeover still compares
// exactly what the operator observed, and still refuses a live owner.
func TestLockTakeoverStaysStrictUnderAPreparingFence(t *testing.T) {
	host, err := os.Hostname()
	if err != nil {
		t.Fatalf("hostname: %v", err)
	}
	tests := []struct {
		name string
		pid  int
		host string
		args func(digest string) []string
		code string
	}{
		{"stale digest", deadCLIPID(t), "a-host-that-is-not-this-one", func(string) []string {
			return []string{"recover", fencedTxID, "--take-over-lock", fencedLockID, "--expect-sha256", strings.Repeat("a", 64)}
		}, "source_changed"},
		{"unknown lock id", deadCLIPID(t), "a-host-that-is-not-this-one", func(digest string) []string {
			return []string{"recover", fencedTxID, "--take-over-lock", strings.Repeat("8", 32), "--expect-sha256", digest}
		}, "source_changed"},
		{"live owner", os.Getpid(), host, func(digest string) []string {
			return []string{"recover", fencedTxID, "--take-over-lock", fencedLockID, "--expect-sha256", digest}
		}, "lock_held"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := setupRepo(t)
			raw := seedFencedLock(t, root, test.pid, test.host)
			writeRetained(t, root, fencedTxID+".preparing.json", preparingMarker())
			before := treeDigests(t, root)

			out, err := runRoot(t, append(test.args(sha256Digest(raw)), "--apply", "--json")...)
			if err == nil {
				t.Fatalf("takeover succeeded: %s", out)
			}
			if failure := decodeMachineError(t, out); failure.Code != test.code {
				t.Fatalf("takeover code = %q, want %q (%s)", failure.Code, test.code, failure.Message)
			}
			assertSameTree(t, before, treeDigests(t, root))
		})
	}
}

func expectedRecoverGuidance(raw []byte) string {
	return "recover: taskrail recover " + fencedTxID + " --take-over-lock " + fencedLockID +
		" --expect-sha256 " + sha256Digest(raw)
}

// reportedRecoverArgs takes the printed remedy exactly as an operator would:
// by running the command line the text hands them, with nothing recomputed.
func reportedRecoverArgs(t *testing.T, text string) []string {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		rest, found := strings.CutPrefix(strings.TrimSpace(line), "recover: taskrail ")
		if !found {
			continue
		}
		return strings.Fields(rest)
	}
	t.Fatalf("lock status reported no recover command: %q", text)
	return nil
}

// treeDigests records every file beneath root by content, so a read-only
// command can be proven to have published nothing at all.
func treeDigests(t *testing.T, root string) map[string]string {
	t.Helper()
	digests := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		digests[filepath.ToSlash(relative)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return digests
}

func assertSameTree(t *testing.T, before, after map[string]string) {
	t.Helper()
	for path, digest := range before {
		if now, ok := after[path]; !ok || now != digest {
			t.Fatalf("%s changed (present %v, %q -> %q)", path, ok, digest, now)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Fatalf("%s was published by a read-only command", path)
		}
	}
}
