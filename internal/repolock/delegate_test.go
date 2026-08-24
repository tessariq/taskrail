package repolock

import (
	"context"
	"crypto/hmac"
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
		Repository: repo,
		Command:    "loop",
		Capability: Capability{
			Commands:     []string{"loop"},
			SelectedTask: "T-1",
			Writes:       []string{"planning/tasks/T-1.md", "planning/STATE.md"},
		},
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
	return Capability{
		Commands:     []string{"complete"},
		TaskFields:   []string{"status", "updated_at"},
		SelectedTask: "T-1",
		Writes:       []string{"planning/tasks/T-1.md", "planning/STATE.md"},
	}
}

func joinRequest(repo Repository, delegation Delegation) JoinRequest {
	return JoinRequest{
		Repository:       repo,
		Command:          "complete",
		Token:            delegation.Token,
		ExecutableSHA256: delegation.ExecutableSHA256,
		Grant:            delegation.Grant,
		Capability:       childCapability(),
	}
}

func TestDelegationExposesOnlyTheAuthenticatedGrantDigest(t *testing.T) {
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
	mac := hmac.New(sha256.New, []byte(delegation.Token))
	_, _ = mac.Write([]byte(`{"selected_task":"T-1","writes":["planning/STATE.md","planning/tasks/T-1.md"]}`))
	if *owner.DelegationDigest != hex.EncodeToString(mac.Sum(nil)) {
		t.Fatalf("delegation_digest = %q, want the canonical grant digest", *owner.DelegationDigest)
	}

	want, err := ExecutableDigest(executable)
	if err != nil {
		t.Fatalf("executable digest: %v", err)
	}
	if owner.ExecutableSHA256 == nil || *owner.ExecutableSHA256 != want || delegation.ExecutableSHA256 != want {
		t.Fatalf("executable digest not bound to the lock: %+v", owner)
	}
}

func TestJoinAuthenticatesTheOwnerDeclaredGrantBeforeNarrowing(t *testing.T) {
	repo := committedRepo(t)
	_, delegation := acquireDelegating(t, repo, stageExecutable(t, "taskrail-bytes"))

	narrow := joinRequest(repo, delegation)
	narrow.Capability.Writes = []string{"planning/tasks/T-1.md"}
	if _, err := Join(narrow); err != nil {
		t.Fatalf("join with a narrowed write set: %v", err)
	}

	for _, tc := range []struct {
		name   string
		mutate func(*JoinRequest)
	}{
		{
			name: "another selected task",
			mutate: func(req *JoinRequest) {
				req.Grant.SelectedTask = "T-2"
				req.Capability.SelectedTask = "T-2"
			},
		},
		{
			name: "a wider task request under the valid grant",
			mutate: func(req *JoinRequest) {
				req.Capability.SelectedTask = "T-2"
			},
		},
		{
			name: "an added write path",
			mutate: func(req *JoinRequest) {
				req.Grant.Writes = append(req.Grant.Writes, "planning/tasks/T-2.md")
				req.Capability.Writes = append(req.Capability.Writes, "planning/tasks/T-2.md")
			},
		},
		{
			name: "an unbounded requested task",
			mutate: func(req *JoinRequest) {
				req.Capability.SelectedTask = ""
			},
		},
		{
			name: "an unbounded requested write set",
			mutate: func(req *JoinRequest) {
				req.Capability.Writes = nil
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := joinRequest(repo, delegation)
			tc.mutate(&req)
			if _, err := Join(req); !errors.Is(err, ErrRefused) {
				t.Fatalf("join error = %v, want ErrRefused", err)
			}
		})
	}
}

func TestDelegationGrantDigestIsCanonical(t *testing.T) {
	token := strings.Repeat("a", 64)
	canonical, err := delegationGrant(Capability{
		SelectedTask: "T-1",
		Writes:       []string{"planning/STATE.md", "planning/tasks/T-1.md"},
	})
	if err != nil {
		t.Fatalf("canonical grant: %v", err)
	}
	unordered, err := delegationGrant(Capability{
		SelectedTask: " T-1 ",
		Writes:       []string{" planning/tasks/T-1.md ", "planning/STATE.md", "planning/STATE.md"},
	})
	if err != nil {
		t.Fatalf("unordered grant: %v", err)
	}
	if delegationDigest(token, canonical) != delegationDigest(token, unordered) {
		t.Fatal("equivalent grants produced different delegation digests")
	}
}

func TestDelegatingAcquisitionRequiresATaskAndWriteSet(t *testing.T) {
	repo := committedRepo(t)
	executable := stageExecutable(t, "taskrail-bytes")
	for _, capability := range []Capability{
		{Commands: []string{"loop"}, Writes: []string{"planning/STATE.md"}},
		{Commands: []string{"loop"}, SelectedTask: "T-1"},
	} {
		if _, err := Acquire(context.Background(), Request{
			Repository: repo, Command: "loop", Capability: capability, ExecutablePath: executable,
		}); err == nil {
			t.Fatal("delegating acquisition accepted an unbounded grant")
		}
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
	if joined.Repository() != repo {
		t.Fatalf("joined repository = %+v, want %+v", joined.Repository(), repo)
	}
	if got := readLockBytes(t, repo); !slices.Equal(got, before) {
		t.Fatal("a delegated join rewrote the lock file")
	}
}

func TestJoinAcceptsFilesystemEquivalentRepositoryRoots(t *testing.T) {
	repo := committedRepo(t)
	aliasParent := t.TempDir()
	alias := filepath.Join(aliasParent, "repo-alias")
	if err := os.Symlink(repo.Root, alias); err != nil {
		t.Skipf("filesystem does not permit directory symlinks: %v", err)
	}
	_, delegation := acquireDelegating(t, repo, stageExecutable(t, "taskrail-bytes"))

	equivalent := repo
	equivalent.Root = alias
	equivalent.GitCommonDir = filepath.Join(alias, ".git")
	joined, err := Join(joinRequest(equivalent, delegation))
	if err != nil {
		t.Fatalf("join through equivalent repository root: %v", err)
	}
	if joined.Repository() != equivalent {
		t.Fatalf("joined repository = %+v, want request context %+v", joined.Repository(), equivalent)
	}
	if sameRoot(repo.Root, t.TempDir()) {
		t.Fatal("genuinely different repository roots compared equal")
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
		Commands:     []string{"complete", "verify"},
		TaskFields:   []string{"status", "updated_at"},
		SelectedTask: "T-1",
		Writes:       []string{"planning/tasks/T-1.md", "planning/STATE.md"},
	}); err == nil {
		t.Fatal("a delegated join widened its command set")
	}
	if _, err := joined.Narrow(Capability{
		Commands:     []string{"complete"},
		TaskFields:   []string{"status", "updated_at", "implementation_notes"},
		SelectedTask: "T-1",
		Writes:       []string{"planning/tasks/T-1.md", "planning/STATE.md"},
	}); err == nil {
		t.Fatal("a delegated join widened its task-field write set")
	}

	narrowed, err := joined.Narrow(Capability{
		Commands:     []string{"complete"},
		TaskFields:   []string{"status"},
		SelectedTask: "T-1",
		Writes:       []string{"planning/tasks/T-1.md"},
	})
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
		Repository: repo,
		Command:    "loop",
		Capability: Capability{
			Commands:     []string{"loop"},
			SelectedTask: "T-1",
			Writes:       []string{"planning/tasks/T-1.md", "planning/STATE.md"},
		},
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
