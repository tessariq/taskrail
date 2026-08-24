package taskrail

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tessariq/taskrail/internal/repolock"
)

func TestLoopOwnershipPinsOneExecutableAndRetainsItsLock(t *testing.T) {
	clearLoopChildEnvironment(t)
	_, svc := loopFixture(t)
	executable := filepath.Join(t.TempDir(), "taskrail")
	if err := os.WriteFile(executable, []byte("pinned taskrail bytes"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	original := loopExecutablePath
	loopExecutablePath = func() (string, error) { return executable, nil }
	t.Cleanup(func() { loopExecutablePath = original })

	ownership, err := svc.beginLoopOwnership(context.Background())
	if err != nil {
		t.Fatalf("begin loop ownership: %v", err)
	}
	defer func() { _ = ownership.close() }()

	if !filepath.IsAbs(ownership.executable.Path) {
		t.Fatalf("staged executable path = %q, want absolute", ownership.executable.Path)
	}
	if got, err := repolock.ExecutableDigest(ownership.executable.Path); err != nil || got != ownership.executable.SHA256 {
		t.Fatalf("staged executable digest = %q, %v; want %q", got, err, ownership.executable.SHA256)
	}
	status, err := repolock.Inspect(svc.paths.LockRepository())
	if err != nil || !status.Held || status.Owner == nil {
		t.Fatalf("loop lock status = %+v, %v", status, err)
	}
	if status.Owner.Command != "loop" || status.Owner.TransactionID == nil {
		t.Fatalf("loop lock owner = %+v", status.Owner)
	}
	lockID := status.Owner.LockID

	first, err := ownership.delegate(svc.loopDelegationGrant("T-001-first"))
	if err != nil {
		t.Fatalf("delegate first task: %v", err)
	}
	if first.InvocationID != *status.Owner.TransactionID || first.Executable != ownership.executable.Path || first.SHA256 != ownership.executable.SHA256 {
		t.Fatalf("first child identity = %+v", first)
	}
	status, err = repolock.Inspect(svc.paths.LockRepository())
	if err != nil || status.Owner == nil || status.Owner.ExecutableSHA256 == nil || *status.Owner.ExecutableSHA256 != first.SHA256 || status.Owner.DelegationDigest == nil {
		t.Fatalf("delegated loop lock status = %+v, %v", status, err)
	}
	if got := string(readBytes(t, repolock.LockPath(svc.paths.LockRepository()))); containsAny(got, first.Token) {
		t.Fatal("loop lock metadata leaked the child token")
	}
	join := repolock.JoinRequest{
		Repository: svc.paths.LockRepository(), Command: "complete", InvocationID: first.InvocationID, Token: first.Token, ExecutableSHA256: first.SHA256,
		Grant:      svc.loopDelegationGrant("T-001-first"),
		Capability: repolock.Capability{Commands: []string{"complete"}, TaskFields: []string{"status"}, SelectedTask: "T-001-first", Writes: []string{"planning/STATE.md", "planning/tasks/T-001-first.md"}},
	}
	if _, err := repolock.Join(join); err != nil {
		t.Fatalf("join first child: %v", err)
	}
	lockBefore := readBytes(t, repolock.LockPath(svc.paths.LockRepository()))
	for _, tc := range []struct {
		name   string
		mutate func(*repolock.JoinRequest)
	}{
		{name: "forged broad grant", mutate: func(req *repolock.JoinRequest) {
			req.Grant.Writes = append(req.Grant.Writes, "planning/tasks/T-009-forged.md")
		}},
		{name: "widened command", mutate: func(req *repolock.JoinRequest) {
			req.Command, req.Capability.Commands = "task release", []string{"task release"}
		}},
		{name: "widened task field", mutate: func(req *repolock.JoinRequest) {
			req.Capability.TaskFields = append(req.Capability.TaskFields, "loop_policy")
		}},
		{name: "another task", mutate: func(req *repolock.JoinRequest) {
			req.Capability.SelectedTask = "T-002-second"
			req.Capability.Writes = []string{"planning/STATE.md", "planning/tasks/T-002-second.md"}
		}},
		{name: "another verification artifact directory", mutate: func(req *repolock.JoinRequest) {
			req.Capability.Writes = append(req.Capability.Writes, "planning/artifacts/verify/T-002-second/report.json")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := join
			tc.mutate(&req)
			if _, err := repolock.Join(req); !errors.Is(err, repolock.ErrRefused) {
				t.Fatalf("widened loop grant join = %v, want refusal", err)
			}
			if got := readBytes(t, repolock.LockPath(svc.paths.LockRepository())); got != lockBefore {
				t.Fatal("refused loop grant join changed lock metadata")
			}
		})
	}

	second, err := ownership.delegate(svc.loopDelegationGrant("T-002-second"))
	if err != nil {
		t.Fatalf("delegate second task: %v", err)
	}
	if second.Token == first.Token || second.Executable != first.Executable || second.SHA256 != first.SHA256 {
		t.Fatalf("second child identity = %+v, first = %+v", second, first)
	}
	status, err = repolock.Inspect(svc.paths.LockRepository())
	if err != nil || !status.Held || status.Owner == nil || status.Owner.LockID != lockID {
		t.Fatalf("rotated loop lock status = %+v, %v", status, err)
	}
	if _, err := repolock.Join(repolock.JoinRequest{
		Repository: svc.paths.LockRepository(), Command: "complete", InvocationID: first.InvocationID, Token: first.Token, ExecutableSHA256: first.SHA256,
		Grant:      svc.loopDelegationGrant("T-001-first"),
		Capability: repolock.Capability{Commands: []string{"complete"}, TaskFields: []string{"status"}, SelectedTask: "T-001-first", Writes: []string{"planning/STATE.md", "planning/tasks/T-001-first.md"}},
	}); !errors.Is(err, repolock.ErrRefused) {
		t.Fatalf("old delegation join error = %v, want refusal", err)
	}

	staged := ownership.executable.Path
	if err := ownership.close(); err != nil {
		t.Fatalf("close loop ownership: %v", err)
	}
	if _, err := os.Lstat(staged); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged executable remains after close: %v", err)
	}
	status, err = repolock.Inspect(svc.paths.LockRepository())
	if err != nil || status.Held {
		t.Fatalf("loop lock after close = %+v, %v", status, err)
	}
}

func TestStageLoopExecutableRefusesCollisionAndReplacementCleanup(t *testing.T) {
	common := t.TempDir()
	source := filepath.Join(t.TempDir(), "taskrail")
	if err := os.WriteFile(source, []byte("source executable"), 0o755); err != nil {
		t.Fatalf("write source: %v", err)
	}
	digest, err := repolock.ExecutableDigest(source)
	if err != nil {
		t.Fatalf("hash source: %v", err)
	}
	dir := filepath.Join(common, "taskrail", "executables")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create stage directory: %v", err)
	}
	collision := filepath.Join(dir, digest)
	if err := os.WriteFile(collision, []byte("someone else's executable"), 0o700); err != nil {
		t.Fatalf("write collision: %v", err)
	}
	if _, err := stageLoopExecutable(common, source); err == nil {
		t.Fatal("staging replaced an existing hash-named file")
	}
	if got := string(readBytes(t, collision)); got != "someone else's executable" {
		t.Fatalf("collision bytes = %q", got)
	}
	if err := os.Remove(collision); err != nil {
		t.Fatalf("remove collision: %v", err)
	}

	staged, err := stageLoopExecutable(common, source)
	if err != nil {
		t.Fatalf("stage executable: %v", err)
	}
	if err := os.Remove(staged.Path); err != nil {
		t.Fatalf("replace staged executable: %v", err)
	}
	if err := os.WriteFile(staged.Path, []byte("replacement"), 0o700); err != nil {
		t.Fatalf("write replacement: %v", err)
	}
	if err := staged.remove(); err == nil {
		t.Fatal("cleanup removed a replacement staged executable")
	}
	if got := string(readBytes(t, staged.Path)); got != "replacement" {
		t.Fatalf("replacement bytes = %q", got)
	}
	if err := os.Remove(staged.Path); err != nil {
		t.Fatalf("replace staged executable with matching bytes: %v", err)
	}
	if err := os.WriteFile(staged.Path, []byte("source executable"), 0o700); err != nil {
		t.Fatalf("write matching replacement: %v", err)
	}
	if err := staged.remove(); err == nil {
		t.Fatal("cleanup removed a same-byte replacement staged executable")
	}
}

func TestStagedLoopExecutablePathPreservesWindowsSuffix(t *testing.T) {
	dir := filepath.Join("git", "taskrail", "executables")
	digest := "0123456789abcdef"
	for _, tc := range []struct {
		name   string
		goos   string
		source string
		want   string
	}{
		{name: "windows executable", goos: "windows", source: filepath.Join("bin", "taskrail.exe"), want: digest + ".exe"},
		{name: "windows uppercase suffix", goos: "windows", source: filepath.Join("bin", "taskrail.EXE"), want: digest + ".EXE"},
		{name: "unix executable", goos: "linux", source: filepath.Join("bin", "taskrail"), want: digest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := stagedLoopExecutablePath(dir, digest, tc.source, tc.goos)
			if got != filepath.Join(dir, tc.want) {
				t.Fatalf("staged executable path = %q, want %q", got, filepath.Join(dir, tc.want))
			}
		})
	}
}

func TestLoopOwnershipRefusesToRotateAfterStagedBytesChange(t *testing.T) {
	clearLoopChildEnvironment(t)
	_, svc := loopFixture(t)
	executable := filepath.Join(t.TempDir(), "taskrail")
	const original = "pinned taskrail bytes"
	if err := os.WriteFile(executable, []byte(original), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	previous := loopExecutablePath
	loopExecutablePath = func() (string, error) { return executable, nil }
	t.Cleanup(func() { loopExecutablePath = previous })

	ownership, err := svc.beginLoopOwnership(context.Background())
	if err != nil {
		t.Fatalf("begin loop ownership: %v", err)
	}
	if _, err := ownership.delegate(svc.loopDelegationGrant("T-001-first")); err != nil {
		t.Fatalf("delegate first task: %v", err)
	}
	before, err := repolock.Inspect(svc.paths.LockRepository())
	if err != nil || before.Owner == nil {
		t.Fatalf("inspect first delegation: %+v, %v", before, err)
	}
	if err := os.WriteFile(ownership.executable.Path, []byte("changed executable bytes"), 0o700); err != nil {
		t.Fatalf("change staged executable: %v", err)
	}
	if _, err := ownership.delegate(svc.loopDelegationGrant("T-002-second")); err == nil {
		t.Fatal("rotated delegation after staged executable bytes changed")
	}
	after, err := repolock.Inspect(svc.paths.LockRepository())
	if err != nil || after.Owner == nil || after.Owner.ExecutableSHA256 == nil || *after.Owner.ExecutableSHA256 != *before.Owner.ExecutableSHA256 || *after.Owner.DelegationDigest != *before.Owner.DelegationDigest {
		t.Fatalf("changed executable rotated lock metadata: before=%+v after=%+v err=%v", before, after, err)
	}
	if err := os.WriteFile(ownership.executable.Path, []byte(original), 0o700); err != nil {
		t.Fatalf("restore staged executable: %v", err)
	}
	if err := ownership.close(); err != nil {
		t.Fatalf("close loop ownership: %v", err)
	}
}

func TestLoopOwnershipRefusesInheritedChildIdentity(t *testing.T) {
	for _, name := range loopChildEnvironmentNames {
		t.Run(name, func(t *testing.T) {
			clearLoopChildEnvironment(t)
			_, svc := loopFixture(t)
			t.Setenv(name, "conflicting")

			if _, err := svc.beginLoopOwnership(context.Background()); err == nil {
				t.Fatal("begin loop ownership accepted inherited child identity")
			}
		})
	}
}

func clearLoopChildEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range loopChildEnvironmentNames {
		value, present := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
		if present {
			t.Cleanup(func() { _ = os.Setenv(name, value) })
		}
	}
}

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if value != "" && len(value) <= len(text) {
			for i := 0; i+len(value) <= len(text); i++ {
				if text[i:i+len(value)] == value {
					return true
				}
			}
		}
	}
	return false
}
