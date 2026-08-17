package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/repolock"
)

// lockStatusJSON re-declares the LockStatusResult fields the acceptance
// criteria pin down, including the exact nullable companion members.
type lockStatusJSON struct {
	Held   bool    `json:"held"`
	SHA256 *string `json:"sha256"`
	Owner  *struct {
		LockID           string  `json:"lock_id"`
		Command          string  `json:"command"`
		PID              int     `json:"pid"`
		Host             string  `json:"host"`
		StartedAt        string  `json:"started_at"`
		RepositoryRoot   string  `json:"repository_root"`
		StorageMode      string  `json:"storage_mode"`
		StorageRoot      string  `json:"storage_root"`
		TransactionID    *string `json:"transaction_id"`
		ExecutableSHA256 *string `json:"executable_sha256"`
		DelegationDigest *string `json:"delegation_digest"`
	} `json:"owner"`
}

type lockClearJSON struct {
	LockID      string `json:"lock_id"`
	Cleared     bool   `json:"cleared"`
	PriorSHA256 string `json:"prior_sha256"`
}

// gitLockRepository is the lock context discovery resolves for a setupRepo
// fixture: a plain `.git` directory is its own common directory.
func gitLockRepository(root string) repolock.Repository {
	return repolock.Repository{Root: root, GitCommonDir: filepath.Join(root, ".git"), Mode: repolock.ModeCommitted}
}

// seedStaleLock writes an abandoned lock record (dead foreign owner) and
// returns its marshaled bytes.
func seedStaleLock(t *testing.T, root string) []byte {
	t.Helper()
	owner := repolock.Owner{
		LockID:         strings.Repeat("7", 32),
		Command:        "verify",
		PID:            deadCLIPID(t),
		Host:           "a-host-that-is-not-this-one",
		StartedAt:      "2001-02-03T04:05:06Z",
		RepositoryRoot: root,
		StorageMode:    repolock.ModeCommitted,
		StorageRoot:    root,
	}
	data, err := json.Marshal(owner)
	if err != nil {
		t.Fatalf("marshal owner: %v", err)
	}
	path := repolock.LockPath(gitLockRepository(root))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create lock root: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	return data
}

// deadCLIPID returns a process id that has provably exited.
func deadCLIPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestNothingMatchesThis$")
	if err := cmd.Run(); err != nil {
		t.Fatalf("run short-lived process: %v", err)
	}
	return cmd.Process.Pid
}

func sha256Digest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func TestLockStatusReportsAbsenceAndWritesNothing(t *testing.T) {
	root := setupRepo(t)
	before := readAllFiles(t, root)

	out, err := runRoot(t, "lock", "status")
	if err != nil {
		t.Fatalf("lock status: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "no repository mutation lock is held") {
		t.Fatalf("absence text = %q", out)
	}

	jsonOut, err := runRoot(t, "lock", "status", "--json")
	if err != nil {
		t.Fatalf("lock status --json: %v (output %q)", err, jsonOut)
	}
	var report lockStatusJSON
	decodeMachineResult(t, jsonOut, &report)
	if report.Held || report.SHA256 != nil || report.Owner != nil {
		t.Fatalf("absent JSON = %+v", report)
	}

	after := readAllFiles(t, root)
	if len(before) != len(after) {
		t.Fatalf("lock status changed the file set")
	}
	for path, content := range before {
		if after[path] != content {
			t.Fatalf("lock status mutated %s", path)
		}
	}
}

func TestLockStatusReportsOwnerMetadataAndDigest(t *testing.T) {
	root := setupRepo(t)
	raw := seedStaleLock(t, root)

	out, err := runRoot(t, "lock", "status")
	if err != nil {
		t.Fatalf("lock status: %v (output %q)", err, out)
	}
	for _, want := range []string{strings.Repeat("7", 32), "command: verify", "started_at: 2001-02-03T04:05:06Z", "transaction_id: none"} {
		if !strings.Contains(out, want) {
			t.Errorf("human view missing %q: %q", want, out)
		}
	}
	if !strings.Contains(out, sha256Digest(raw)) {
		t.Errorf("human view missing the raw digest: %q", out)
	}

	jsonOut, err := runRoot(t, "lock", "status", "--json")
	if err != nil {
		t.Fatalf("lock status --json: %v (output %q)", err, jsonOut)
	}
	var report lockStatusJSON
	decodeMachineResult(t, jsonOut, &report)
	if !report.Held || report.Owner == nil || report.SHA256 == nil || *report.SHA256 != sha256Digest(raw) {
		t.Fatalf("held JSON = %+v", report)
	}
	if report.Owner.LockID != strings.Repeat("7", 32) || report.Owner.Command != "verify" || report.Owner.PID <= 0 {
		t.Fatalf("owner JSON = %+v", report.Owner)
	}
	if report.Owner.TransactionID != nil || report.Owner.ExecutableSHA256 != nil || report.Owner.DelegationDigest != nil {
		t.Fatalf("undelegated owner carries nullable metadata: %+v", report.Owner)
	}
	if report.Owner.RepositoryRoot != root || report.Owner.StorageMode != "committed" {
		t.Fatalf("owner identity JSON = %+v", report.Owner)
	}
}

// A delegated lock exposes the delegation digest and executable digest — and
// never the token itself — in both human and machine output.
func TestLockStatusExposesNoDelegationToken(t *testing.T) {
	root := setupRepo(t)
	executable := filepath.Join(t.TempDir(), "taskrail-copy")
	if err := os.WriteFile(executable, []byte("taskrail-bytes"), 0o755); err != nil {
		t.Fatalf("stage executable: %v", err)
	}
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: gitLockRepository(root),
		Command:    "verify",
		Capability: repolock.Capability{
			Commands:     []string{"verify"},
			TaskFields:   []string{"status"},
			SelectedTask: "T-1",
			Writes:       []string{"planning/tasks/T-1.md"},
		},
		ExecutablePath: executable,
	})
	if err != nil {
		t.Fatalf("acquire delegating lock: %v", err)
	}
	defer func() { _ = lock.Release() }()
	delegation, err := lock.Delegation()
	if err != nil {
		t.Fatalf("read delegation: %v", err)
	}

	out, err := runRoot(t, "lock", "status")
	if err != nil {
		t.Fatalf("lock status: %v (output %q)", err, out)
	}
	if strings.Contains(out, delegation.Token) {
		t.Fatalf("human view leaks the delegation token: %q", out)
	}
	if !strings.Contains(out, delegation.ExecutableSHA256) {
		t.Fatalf("human view missing the executable digest: %q", out)
	}

	jsonOut, err := runRoot(t, "lock", "status", "--json")
	if err != nil {
		t.Fatalf("lock status --json: %v (output %q)", err, jsonOut)
	}
	if strings.Contains(jsonOut, delegation.Token) {
		t.Fatalf("machine view leaks the delegation token: %s", jsonOut)
	}
	var report lockStatusJSON
	decodeMachineResult(t, jsonOut, &report)
	if report.Owner == nil || report.Owner.DelegationDigest == nil || report.Owner.ExecutableSHA256 == nil {
		t.Fatalf("delegated owner missing digest metadata: %+v", report.Owner)
	}
	if *report.Owner.DelegationDigest == delegation.Token || *report.Owner.ExecutableSHA256 != delegation.ExecutableSHA256 {
		t.Fatalf("delegated digests disagree: %+v", report.Owner)
	}
}

func TestLockClearRemovesTheUnchangedStaleLock(t *testing.T) {
	root := setupRepo(t)
	raw := seedStaleLock(t, root)
	lockPath := repolock.LockPath(gitLockRepository(root))

	out, err := runRoot(t, "lock", "clear", strings.Repeat("7", 32), "--expect-sha256", sha256Digest(raw))
	if err != nil {
		t.Fatalf("lock clear: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "cleared repository mutation lock "+strings.Repeat("7", 32)) {
		t.Fatalf("clear text = %q", out)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("clear left the lock behind: %v", err)
	}

	// Machine mode reports the same outcome for a second stale lock.
	raw2 := seedStaleLock(t, root)
	jsonOut, err := runRoot(t, "lock", "clear", strings.Repeat("7", 32), "--expect-sha256", sha256Digest(raw2), "--json")
	if err != nil {
		t.Fatalf("lock clear --json: %v (output %q)", err, jsonOut)
	}
	var report lockClearJSON
	decodeMachineResult(t, jsonOut, &report)
	if report.LockID != strings.Repeat("7", 32) || !report.Cleared || report.PriorSHA256 != sha256Digest(raw2) {
		t.Fatalf("clear JSON = %+v", report)
	}
}

// Retained transaction state beneath the lock root fences the whole command
// family, including the lock operator surface: clearing is refused before it
// begins and removes nothing, so retained recovery evidence can never be
// disturbed as a side effect of clearing ownership.
func TestLockCommandsAreFencedByRetainedTransactionState(t *testing.T) {
	root := setupRepo(t)
	raw := seedStaleLock(t, root)
	installRecoveryFence(t, root, "verify")

	for _, args := range [][]string{
		{"lock", "status", "--json"},
		{"lock", "clear", strings.Repeat("7", 32), "--expect-sha256", sha256Digest(raw), "--json"},
	} {
		out, err := runRoot(t, args...)
		if err == nil {
			t.Fatalf("%v succeeded: %s", args, out)
		}
		failure := decodeMachineError(t, out)
		if failure.Code != "recovery_pending" {
			t.Fatalf("%v code = %q, want recovery_pending (%s)", args, failure.Code, failure.Message)
		}
	}
	if _, err := os.Stat(repolock.LockPath(gitLockRepository(root))); err != nil {
		t.Fatalf("a fenced clear removed the lock: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "taskrail", "transactions")); err != nil {
		t.Fatalf("a fenced clear disturbed retained transactions: %v", err)
	}
}

func TestLockClearRefusalSurfacesAndCodes(t *testing.T) {
	root := setupRepo(t)
	raw := seedStaleLock(t, root)
	lockPath := repolock.LockPath(gitLockRepository(root))

	tests := []struct {
		name string
		args []string
		code string
	}{
		{"digest race", []string{"lock", "clear", strings.Repeat("7", 32), "--expect-sha256", strings.Repeat("a", 64)}, "source_changed"},
		{"replacement id", []string{"lock", "clear", strings.Repeat("8", 32), "--expect-sha256", sha256Digest(raw)}, "source_changed"},
		{"malformed expected digest", []string{"lock", "clear", strings.Repeat("7", 32), "--expect-sha256", "nope"}, "invalid_digest"},
		{"malformed lock id", []string{"lock", "clear", "NOT-HEX", "--expect-sha256", sha256Digest(raw)}, "invalid_arguments"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			jsonOut, err := runRoot(t, append(test.args, "--json")...)
			if err == nil {
				t.Fatalf("expected a refusal, got %q", jsonOut)
			}
			failure := decodeMachineError(t, jsonOut)
			if failure.Code != test.code {
				t.Fatalf("code = %q, want %q (%s)", failure.Code, test.code, failure.Message)
			}
			if _, statErr := os.Stat(lockPath); statErr != nil {
				t.Fatalf("a refused clear removed the lock: %v", statErr)
			}
		})
	}

	// An absent lock is an invalid_digest refusal on a repository with no
	// lock at all, not a success.
	absent := setupRepo(t)
	jsonOut, err := runRoot(t, "lock", "clear", strings.Repeat("7", 32), "--expect-sha256", strings.Repeat("a", 64), "--json")
	if err == nil {
		t.Fatalf("expected a refusal for an absent lock, got %q", jsonOut)
	}
	if failure := decodeMachineError(t, jsonOut); failure.Code != "invalid_digest" {
		t.Fatalf("absent lock code = %q, want invalid_digest", failure.Code)
	}
	if _, err := os.Stat(repolock.LockPath(gitLockRepository(absent))); !os.IsNotExist(err) {
		t.Fatal("a repository without a lock gained one")
	}
}

func TestLockClearRefusesAProvablyLiveOwner(t *testing.T) {
	root := setupRepo(t)
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: gitLockRepository(root),
		Command:    "start",
		Capability: repolock.Capability{Commands: []string{"start"}, TaskFields: []string{"status"}},
	})
	if err != nil {
		t.Fatalf("acquire live lock: %v", err)
	}
	defer func() { _ = lock.Release() }()
	raw, err := os.ReadFile(repolock.LockPath(gitLockRepository(root)))
	if err != nil {
		t.Fatalf("read live lock: %v", err)
	}

	jsonOut, err := runRoot(t, "lock", "clear", lock.Owner().LockID, "--expect-sha256", sha256Digest(raw), "--json")
	if err == nil {
		t.Fatalf("expected a refusal against a live owner, got %q", jsonOut)
	}
	failure := decodeMachineError(t, jsonOut)
	if failure.Code != "lock_held" {
		t.Fatalf("live owner code = %q, want lock_held (%s)", failure.Code, failure.Message)
	}
	if after, err := os.ReadFile(repolock.LockPath(gitLockRepository(root))); err != nil || string(after) != string(raw) {
		t.Fatalf("the refused clear disturbed the live lock: %q %v", after, err)
	}
}

// A non-Git root-local repository resolves the lock beneath its own
// `.taskrail/runtime/`, so inspection and guarded clearing work there too.
func TestLockCommandsWorkInANonGitRepository(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".taskrail"), 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".taskrail", "config.yml"), []byte("layout_version: 1\n"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	t.Chdir(root)
	before := readAllFiles(t, root)

	out, err := runRoot(t, "lock", "status")
	if err != nil || !strings.Contains(out, "no repository mutation lock is held") {
		t.Fatalf("non-Git status: %v (output %q)", err, out)
	}
	if len(before) != len(readAllFiles(t, root)) {
		t.Fatal("non-Git status wrote something")
	}

	nonGit := repolock.Repository{Root: root, Mode: repolock.ModeCommitted}
	owner := repolock.Owner{
		LockID:         strings.Repeat("9", 32),
		Command:        "verify",
		PID:            deadCLIPID(t),
		Host:           "a-host-that-is-not-this-one",
		StartedAt:      "2001-02-03T04:05:06Z",
		RepositoryRoot: root,
		StorageMode:    repolock.ModeCommitted,
		StorageRoot:    root,
	}
	raw, err := json.Marshal(owner)
	if err != nil {
		t.Fatalf("marshal owner: %v", err)
	}
	lockPath := repolock.LockPath(nonGit)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("create runtime lock root: %v", err)
	}
	if err := os.WriteFile(lockPath, raw, 0o644); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}
	if want := filepath.Join(root, ".taskrail", "runtime", "mutation.lock"); lockPath != want {
		t.Fatalf("non-Git lock path = %q, want %q", lockPath, want)
	}

	out, err = runRoot(t, "lock", "status")
	if err != nil || !strings.Contains(out, strings.Repeat("9", 32)) {
		t.Fatalf("non-Git held status: %v (output %q)", err, out)
	}
	if _, err := runRoot(t, "lock", "clear", strings.Repeat("9", 32), "--expect-sha256", sha256Digest(raw)); err != nil {
		t.Fatalf("non-Git clear: %v", err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("non-Git clear left the lock behind: %v", err)
	}
}
