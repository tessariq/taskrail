package repolock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// stageExecutable writes a distinct fake executable so a test can bind a lock to
// one executable identity and prove a different one cannot join.
func stageExecutable(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "taskrail")
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("stage executable: %v", err)
	}
	return path
}

func acquireDelegating(t *testing.T, repo Repository, executable string) (*Lock, Delegation) {
	t.Helper()
	lock, err := Acquire(context.Background(), Request{
		Repository:     repo,
		Command:        "loop",
		Capability:     Capability{Commands: []string{"loop"}, TaskFields: nil},
		ExecutablePath: executable,
	})
	if err != nil {
		t.Fatalf("acquire delegating lock: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })
	delegation, err := lock.Delegation()
	if err != nil {
		t.Fatalf("delegation: %v", err)
	}
	return lock, delegation
}

func childCapability() Capability {
	return Capability{Commands: []string{"complete"}, TaskFields: []string{"status", "updated_at"}}
}

func joinRequest(repo Repository, delegation Delegation) JoinRequest {
	return JoinRequest{
		Repository:       repo,
		Command:          "complete",
		Token:            delegation.Token,
		ExecutableSHA256: delegation.ExecutableSHA256,
		Capability:       childCapability(),
	}
}

func TestDelegationExposesOnlyTheTokenDigest(t *testing.T) {
	repo := committedRepo(t)
	executable := stageExecutable(t, "taskrail-bytes")
	lock, delegation := acquireDelegating(t, repo, executable)

	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(delegation.Token) {
		t.Fatalf("delegation token %q is not 64 lower-case hex characters", delegation.Token)
	}
	raw := readLockBytes(t, repo)
	if strings.Contains(string(raw), delegation.Token) {
		t.Fatal("the delegation token leaked into the lock metadata")
	}
	owner := lock.Owner()
	if owner.DelegationDigest == nil {
		t.Fatal("delegating lock recorded no delegation digest")
	}
	digest := sha256.Sum256([]byte(delegation.Token))
	if *owner.DelegationDigest != hex.EncodeToString(digest[:]) {
		t.Fatalf("delegation_digest = %q, want the token digest", *owner.DelegationDigest)
	}

	want, err := ExecutableDigest(executable)
	if err != nil {
		t.Fatalf("executable digest: %v", err)
	}
	if owner.ExecutableSHA256 == nil || *owner.ExecutableSHA256 != want || delegation.ExecutableSHA256 != want {
		t.Fatalf("executable digest not bound to the lock: %+v", owner)
	}
}

func TestDelegationTokensAreDistinctPerAcquisition(t *testing.T) {
	executable := stageExecutable(t, "taskrail-bytes")
	seen := make(map[string]struct{}, 8)
	for range 8 {
		repo := committedRepo(t)
		_, delegation := acquireDelegating(t, repo, executable)
		if _, duplicate := seen[delegation.Token]; duplicate {
			t.Fatal("two acquisitions minted the same delegation token")
		}
		seen[delegation.Token] = struct{}{}
	}
}

func TestUndelegatedLockOffersNoDelegation(t *testing.T) {
	lock := acquire(t, committedRepo(t))
	if _, err := lock.Delegation(); !errors.Is(err, ErrNotDelegated) {
		t.Fatalf("delegation error = %v, want ErrNotDelegated", err)
	}
}

func TestJoinAcceptsAMatchingDelegateWithoutMutatingTheLock(t *testing.T) {
	repo := committedRepo(t)
	_, delegation := acquireDelegating(t, repo, stageExecutable(t, "taskrail-bytes"))
	before := readLockBytes(t, repo)

	joined, err := Join(joinRequest(repo, delegation))
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if err := joined.Authorize("complete", "status"); err != nil {
		t.Fatalf("authorize delegated write: %v", err)
	}
	if got := readLockBytes(t, repo); !slices.Equal(got, before) {
		t.Fatal("a delegated join rewrote the lock file")
	}
}

func TestJoinRefusesEveryMismatchWithoutMutation(t *testing.T) {
	executable := stageExecutable(t, "taskrail-bytes")

	tests := []struct {
		name    string
		mutate  func(t *testing.T, repo Repository, delegation Delegation) JoinRequest
		wantErr error
	}{
		{
			name: "wrong token",
			mutate: func(_ *testing.T, repo Repository, delegation Delegation) JoinRequest {
				req := joinRequest(repo, delegation)
				req.Token = strings.Repeat("f", 64)
				return req
			},
			wantErr: ErrRefused,
		},
		{
			name: "wrong executable identity",
			mutate: func(_ *testing.T, repo Repository, delegation Delegation) JoinRequest {
				req := joinRequest(repo, delegation)
				req.ExecutableSHA256 = strings.Repeat("0", 64)
				return req
			},
			wantErr: ErrRefused,
		},
		{
			// A different root resolves a different lock path entirely, so the
			// delegate finds no lock rather than joining someone else's.
			name: "different repository root",
			mutate: func(t *testing.T, repo Repository, delegation Delegation) JoinRequest {
				other := repo
				other.Root = t.TempDir()
				other.GitCommonDir = filepath.Join(other.Root, ".git")
				return joinRequest(other, delegation)
			},
			wantErr: ErrNotHeld,
		},
		{
			name: "mixed storage mode",
			mutate: func(_ *testing.T, repo Repository, delegation Delegation) JoinRequest {
				mixed := repo
				mixed.Mode = ModeLocal
				return joinRequest(mixed, delegation)
			},
			wantErr: ErrRefused,
		},
		{
			name: "command outside the delegated set",
			mutate: func(_ *testing.T, repo Repository, delegation Delegation) JoinRequest {
				req := joinRequest(repo, delegation)
				req.Command = "task new"
				req.Capability = Capability{Commands: []string{"task new"}, TaskFields: []string{"status"}}
				return req
			},
			wantErr: ErrRefused,
		},
		{
			name: "loop policy mutation is never delegated",
			mutate: func(_ *testing.T, repo Repository, delegation Delegation) JoinRequest {
				req := joinRequest(repo, delegation)
				req.Command = "task loop allow"
				req.Capability = Capability{Commands: []string{"task loop allow"}, TaskFields: []string{"loop_policy"}}
				return req
			},
			wantErr: ErrRefused,
		},
		{
			name: "task field outside the delegated write set",
			mutate: func(_ *testing.T, repo Repository, delegation Delegation) JoinRequest {
				req := joinRequest(repo, delegation)
				req.Capability = Capability{Commands: []string{"complete"}, TaskFields: []string{"status", "spec_ref"}}
				return req
			},
			wantErr: ErrRefused,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := committedRepo(t)
			_, delegation := acquireDelegating(t, repo, executable)
			before := readLockBytes(t, repo)

			if _, err := Join(test.mutate(t, repo, delegation)); !errors.Is(err, test.wantErr) {
				t.Fatalf("join error = %v, want %v", err, test.wantErr)
			}
			if got := readLockBytes(t, repo); !slices.Equal(got, before) {
				t.Fatal("a refused join rewrote the lock file")
			}
		})
	}
}

func TestJoinRefusesAnUndelegatedOrAbsentLock(t *testing.T) {
	repo := committedRepo(t)
	// Only the world changes between the two assertions, never the request.
	req := JoinRequest{
		Repository:       repo,
		Command:          "complete",
		Token:            strings.Repeat("a", 64),
		ExecutableSHA256: strings.Repeat("b", 64),
		Capability:       childCapability(),
	}

	if _, err := Join(req); !errors.Is(err, ErrNotHeld) {
		t.Fatalf("join with no lock error = %v, want ErrNotHeld", err)
	}

	acquire(t, repo)
	if _, err := Join(req); !errors.Is(err, ErrRefused) {
		t.Fatalf("join with an undelegated lock error = %v, want ErrRefused", err)
	}
}

func TestJoinedOwnershipCannotWidenItsCapability(t *testing.T) {
	repo := committedRepo(t)
	_, delegation := acquireDelegating(t, repo, stageExecutable(t, "taskrail-bytes"))
	joined, err := Join(joinRequest(repo, delegation))
	if err != nil {
		t.Fatalf("join: %v", err)
	}

	if _, err := joined.Narrow(Capability{
		Commands:   []string{"complete", "verify"},
		TaskFields: []string{"status", "updated_at"},
	}); err == nil {
		t.Fatal("a delegated join widened its command set")
	}
	if _, err := joined.Narrow(Capability{
		Commands:   []string{"complete"},
		TaskFields: []string{"status", "updated_at", "implementation_notes"},
	}); err == nil {
		t.Fatal("a delegated join widened its task-field write set")
	}

	narrowed, err := joined.Narrow(Capability{Commands: []string{"complete"}, TaskFields: []string{"status"}})
	if err != nil {
		t.Fatalf("narrow: %v", err)
	}
	if err := narrowed.Authorize("complete", "updated_at"); err == nil {
		t.Fatal("a narrowed join still authorizes a dropped field")
	}
}

// The protocol has to work identically against an explicitly supplied local
// repository context, since local mode differs only in where managed bytes live.
func TestLocalModeSupportsTheWholeOwnershipAndDelegationCycle(t *testing.T) {
	repo := localRepo(t)
	lock, err := Acquire(context.Background(), Request{
		Repository:     repo,
		Command:        "loop",
		Capability:     Capability{Commands: []string{"loop"}},
		ExecutablePath: stageExecutable(t, "taskrail-bytes"),
	})
	if err != nil {
		t.Fatalf("acquire in local mode: %v", err)
	}
	delegation, err := lock.Delegation()
	if err != nil {
		t.Fatalf("delegation: %v", err)
	}

	joined, err := Join(joinRequest(repo, delegation))
	if err != nil {
		t.Fatalf("join in local mode: %v", err)
	}
	if joined.Owner().StorageRoot != repo.StorageRoot() {
		t.Fatalf("joined storage root = %q, want %q", joined.Owner().StorageRoot, repo.StorageRoot())
	}
	if err := joined.Authorize("complete", "status"); err != nil {
		t.Fatalf("authorize delegated local write: %v", err)
	}

	// A committed-mode caller in the same worktree resolves the same lock file
	// but must not be able to join it: mixed-mode ownership is ambiguous.
	committed := repo
	committed.Mode = ModeCommitted
	if LockPath(committed) != LockPath(repo) {
		t.Fatal("local and committed modes resolved different lock paths in one worktree")
	}
	if _, err := Join(joinRequest(committed, delegation)); !errors.Is(err, ErrRefused) {
		t.Fatalf("mixed-mode join error = %v, want ErrRefused", err)
	}
	if _, err := Acquire(context.Background(), writerRequest(committed)); !errors.Is(err, ErrSameProcess) {
		t.Fatalf("committed acquire error = %v, want ErrSameProcess", err)
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("release local lock: %v", err)
	}
}

func TestDelegatedCapabilityExcludesCreationAndPolicyMutation(t *testing.T) {
	delegated := DelegatedCapability()
	for _, command := range []string{"start", "complete", "block", "unblock", "verify"} {
		if err := delegated.Allows(command); err != nil {
			t.Fatalf("delegated capability rejects %q: %v", command, err)
		}
	}
	for _, command := range []string{"task new", "task loop allow", "task loop hold", "task loop clear", "spec activate"} {
		if err := delegated.Allows(command); err == nil {
			t.Fatalf("delegated capability allows %q", command)
		}
	}
	for _, field := range []string{"id", "title", "priority", "spec_ref", "dependencies", "loop_policy", "loop_reason"} {
		if err := delegated.Allows("complete", field); err == nil {
			t.Fatalf("delegated capability allows writing %q", field)
		}
	}
}

func TestExecutableDigestRejectsAMissingFile(t *testing.T) {
	if _, err := ExecutableDigest(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("expected a missing executable to fail")
	}
}
