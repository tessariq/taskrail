package taskrail

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/tessariq/taskrail/internal/durablefs"
)

func TestPrepareLoopResultFileClassifiesUnsafeDestinations(t *testing.T) {
	parent := t.TempDir()
	existing := filepath.Join(parent, "existing.json")
	if err := os.WriteFile(existing, []byte("existing"), 0o600); err != nil {
		t.Fatalf("write existing result: %v", err)
	}
	if _, err := PrepareLoopResultFile(existing); MachineFailureFor(err).Code != MachineCodeDestinationExists {
		t.Fatalf("existing result error = %v", err)
	}
	if _, err := PrepareLoopResultFile(filepath.Join(parent, "missing", "result.json")); MachineFailureFor(err).Code != MachineCodePathBlocked {
		t.Fatalf("missing parent error = %v", err)
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(parent, alias); err != nil {
		t.Skipf("create symlink: %v", err)
	}
	if _, err := PrepareLoopResultFile(filepath.Join(alias, "result.json")); MachineFailureFor(err).Code != MachineCodePathBlocked {
		t.Fatalf("aliased parent error = %v", err)
	}
}

func TestLoopResultFileRechecksParentAndNoClobberTarget(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "result-parent")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatalf("make parent: %v", err)
	}
	result, err := PrepareLoopResultFile(filepath.Join(parent, "result.json"))
	if err != nil {
		t.Fatalf("prepare result: %v", err)
	}
	testHookLoopResultBeforePublish = func() {
		if err := os.WriteFile(filepath.Join(parent, "result.json"), []byte("late"), 0o600); err != nil {
			t.Fatalf("create late target: %v", err)
		}
	}
	t.Cleanup(func() {
		testHookLoopResultBeforePublish = nil
		testHookLoopResultAfterPublish = nil
	})
	if err := result.Publish([]byte(`{"schema_version":1}`)); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("late target publish error = %v", err)
	}
	bytes, err := os.ReadFile(filepath.Join(parent, "result.json"))
	if err != nil || string(bytes) != "late" {
		t.Fatalf("late target = %q, %v", bytes, err)
	}

	testHookLoopResultBeforePublish = nil
	parent = filepath.Join(t.TempDir(), "swapped-parent")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatalf("make swap parent: %v", err)
	}
	result, err = PrepareLoopResultFile(filepath.Join(parent, "result.json"))
	if err != nil {
		t.Fatalf("prepare swapped result: %v", err)
	}
	if err := os.Rename(parent, parent+"-old"); err != nil {
		t.Fatalf("rename parent: %v", err)
	}
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatalf("replace parent: %v", err)
	}
	if err := result.Publish([]byte(`{"schema_version":1}`)); !errors.Is(err, durablefs.ErrConflict) {
		t.Fatalf("swapped parent publish error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "result.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("swapped parent received target: %v", err)
	}

	t.Run("late parent swap", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), "late-swapped-parent")
		if err := os.Mkdir(parent, 0o700); err != nil {
			t.Fatalf("make late swap parent: %v", err)
		}
		result, err := PrepareLoopResultFile(filepath.Join(parent, "result.json"))
		if err != nil {
			t.Fatalf("prepare late swapped result: %v", err)
		}
		testHookLoopResultAfterPublish = func() {
			if err := os.Rename(parent, parent+"-old"); err != nil {
				t.Skipf("filesystem cannot rename an open result parent: %v", err)
			}
			if err := os.Mkdir(parent, 0o700); err != nil {
				t.Fatalf("replace late parent: %v", err)
			}
		}
		if err := result.Publish([]byte(`{"schema_version":1}`)); !errors.Is(err, durablefs.ErrConflict) {
			t.Fatalf("late swapped parent publish error = %v", err)
		}
		if _, err := os.Stat(filepath.Join(parent, "result.json")); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("late swapped parent named target: %v", err)
		}
	})
}

func TestLoopResultPublicationAcceptsOnlyCommittedUnsupportedBarrier(t *testing.T) {
	expected := "result.json"
	committedUnsupported := &durablefs.MutationError{
		Operation: "publish",
		Path:      expected,
		Committed: true,
		Err:       durablefs.ErrUnsupported,
	}
	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "success", want: true},
		{name: "committed unsupported", err: committedUnsupported, want: true},
		{name: "pre-commit unsupported", err: &durablefs.MutationError{Operation: "publish", Path: expected, Err: durablefs.ErrUnsupported}},
		{name: "other committed error", err: &durablefs.MutationError{Operation: "publish", Path: expected, Committed: true, Err: fs.ErrPermission}},
		{name: "other destination", err: &durablefs.MutationError{Operation: "publish", Path: "other.json", Committed: true, Err: durablefs.ErrUnsupported}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := loopResultPublicationCommitted(test.err, expected); got != test.want {
				t.Fatalf("committed = %v, want %v for %v", got, test.want, test.err)
			}
		})
	}
}

func TestLoopResultFileRefusesRepositoryDestinations(t *testing.T) {
	repo, svc := loopFixture(t)
	result, err := PrepareLoopResultFile(filepath.Join(repo, "result.json"))
	if err != nil {
		t.Fatalf("prepare repository result: %v", err)
	}
	if err := svc.ValidateLoopResultFile(result); MachineFailureFor(err).Code != MachineCodePathBlocked {
		t.Fatalf("repository destination error = %v", err)
	}
}
