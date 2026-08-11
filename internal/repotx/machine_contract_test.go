package repotx_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tessariq/taskrail/internal/repolock"
	"github.com/tessariq/taskrail/internal/repotx"
	"github.com/tessariq/taskrail/internal/taskrail"
)

// A transaction's evidence is only useful if it publishes unchanged. This test
// runs a real failure, renders its snapshots into a common error envelope, and
// holds the document to the strict schema-version-1 decoder, so a path spelling,
// digest case, or ordering rule that drifts from the machine contract fails here
// rather than at an agent.
func TestTransactionSnapshotsSatisfyTheMachineContract(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temporary root: %v", err)
	}
	repo := repolock.Repository{Root: root, Mode: repolock.ModeCommitted}
	task := filepath.Join(root, "planning", "tasks", "T-1.md")
	if err := os.MkdirAll(filepath.Dir(task), 0o755); err != nil {
		t.Fatalf("seed task directory: %v", err)
	}
	if err := os.WriteFile(task, []byte("original task"), 0o644); err != nil {
		t.Fatalf("seed task: %v", err)
	}

	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: repo,
		Command:    "complete",
		Capability: repolock.Capability{Commands: []string{"complete"}, TaskFields: []string{"status"}},
	})
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	defer func() { _ = lock.Release() }()

	gitPhysical := filepath.Join(root, ".git", "info", "exclude")
	_, err = repotx.Commit(context.Background(), lock, repotx.Request{
		Command:      "complete",
		SelectedTask: "T-1",
		Published: []repotx.Candidate{
			{
				Path:    repotx.Path{Kind: repotx.Managed, Reported: "planning/tasks/T-1.md", Physical: task},
				Content: []byte("completed task"),
			},
			{
				Path: repotx.Path{
					Kind:     repotx.Worktree,
					Reported: ".agents/skills/taskrail/SKILL.md",
					Physical: filepath.Join(root, ".agents", "skills", "taskrail", "SKILL.md"),
				},
				Content: []byte("skill"),
			},
			{
				Path: repotx.Path{
					Kind:     repotx.Git,
					Reported: filepath.ToSlash(gitPhysical),
					Physical: gitPhysical,
				},
				Content: []byte("exclusion"),
			},
		},
		Validate: func([]repotx.Snapshot) error {
			return os.WriteFile(task, []byte("someone else's edit"), 0o644)
		},
	})

	var txErr *repotx.Error
	if !errors.As(err, &txErr) {
		t.Fatalf("error %v is not a transaction failure", err)
	}
	code, registered := txErr.MachineCode()
	if !registered || code != "write_conflict" {
		t.Fatalf("machine code = (%q, registered=%v), want write_conflict", code, registered)
	}

	document, err := json.Marshal(map[string]any{
		"schema_version": 1,
		"command":        "complete",
		"warnings":       []any{},
		"error": map[string]any{
			"code":    code,
			"message": "the transaction snapshot went stale",
			"details": map[string]any{
				"applied":    false,
				"violations": []any{},
				"paths":      []any{},
				"snapshots":  wireSnapshots(txErr.Snapshots()),
				"recovery":   nil,
			},
		},
	})
	if err != nil {
		t.Fatalf("encode document: %v", err)
	}

	envelope, err := taskrail.DecodeMachineEnvelope(document)
	if err != nil {
		t.Fatalf("decode transaction evidence as a machine document: %v\n%s", err, document)
	}
	if len(envelope.Error.Details.Snapshots) != len(txErr.Snapshots()) {
		t.Fatalf("decoded %d snapshots, want %d",
			len(envelope.Error.Details.Snapshots), len(txErr.Snapshots()))
	}
	for i, got := range envelope.Error.Details.Snapshots {
		want := txErr.Snapshots()[i]
		if got.PathKind != string(want.Kind) || got.Path != want.Path {
			t.Fatalf("snapshot %d decoded as (%s, %s), want (%s, %s)",
				i, got.PathKind, got.Path, want.Kind, want.Path)
		}
	}
}

// repotx and internal/taskrail each own a copy of the path-shape rules, because
// taskrail will import repotx and the reverse dependency would be a cycle. This
// table is what keeps the two copies one contract: every spelling has to be
// accepted or rejected by both, so a rule that drifts on either side fails here.
func TestPathShapeRulesAgreeWithTheMachineDecoder(t *testing.T) {
	for _, tc := range []struct {
		name  string
		kind  repotx.PathKind
		path  string
		valid bool
	}{
		{name: "managed relative", kind: repotx.Managed, path: "planning/STATE.md", valid: true},
		{name: "worktree relative", kind: repotx.Worktree, path: ".agents/skills/SKILL.md", valid: true},
		{name: "managed dot-dot", kind: repotx.Managed, path: "planning/../STATE.md"},
		{name: "managed dot segment", kind: repotx.Managed, path: "./STATE.md"},
		{name: "managed empty segment", kind: repotx.Managed, path: "planning//STATE.md"},
		{name: "managed trailing separator", kind: repotx.Managed, path: "planning/STATE.md/"},
		{name: "managed absolute", kind: repotx.Managed, path: "/planning/STATE.md"},
		{name: "managed backslash", kind: repotx.Managed, path: `planning\STATE.md`},
		{name: "managed empty", kind: repotx.Managed, path: ""},
		{name: "git absolute", kind: repotx.Git, path: "/srv/repo/.git/info/exclude", valid: true},
		{name: "git drive absolute", kind: repotx.Git, path: "C:/repo/.git/info/exclude", valid: true},
		{name: "git relative", kind: repotx.Git, path: ".git/info/exclude"},
		{name: "git dot-dot", kind: repotx.Git, path: "/srv/../repo/.git/info/exclude"},
		{name: "git root only", kind: repotx.Git, path: "/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := decoderAccepts(t, tc.kind, tc.path); got != tc.valid {
				t.Fatalf("machine decoder accepted = %v, want %v", got, tc.valid)
			}
			if got := transactionAccepts(t, tc.kind, tc.path); got != tc.valid {
				t.Fatalf("transaction accepted = %v, want %v", got, tc.valid)
			}
		})
	}
}

// decoderAccepts reports whether the strict machine decoder admits one snapshot
// spelled this way.
func decoderAccepts(t *testing.T, kind repotx.PathKind, path string) bool {
	t.Helper()
	document, err := json.Marshal(map[string]any{
		"schema_version": 1,
		"command":        "complete",
		"warnings":       []any{},
		"error": map[string]any{
			"code":    "write_conflict",
			"message": "path shape probe",
			"details": map[string]any{
				"applied":    false,
				"violations": []any{},
				"paths":      []any{},
				"snapshots": []any{map[string]any{
					"path_kind":        string(kind),
					"path":             path,
					"original_sha256":  nil,
					"candidate_sha256": nil,
					"current_sha256":   nil,
				}},
				"recovery": nil,
			},
		},
	})
	if err != nil {
		t.Fatalf("encode probe document: %v", err)
	}
	_, err = taskrail.DecodeMachineEnvelope(document)
	return err == nil
}

// transactionAccepts reports whether a transaction admits one published path
// spelled this way. The physical location is always a legitimate one inside the
// repository, so only the reported spelling is under test.
func transactionAccepts(t *testing.T, kind repotx.PathKind, path string) bool {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temporary root: %v", err)
	}
	repo := repolock.Repository{Root: root, Mode: repolock.ModeCommitted}
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: repo,
		Command:    "complete",
		Capability: repolock.Capability{Commands: []string{"complete"}},
	})
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	defer func() { _ = lock.Release() }()

	_, err = repotx.Commit(context.Background(), lock, repotx.Request{
		Command: "complete",
		Published: []repotx.Candidate{{
			Path: repotx.Path{
				Kind:     kind,
				Reported: path,
				Physical: filepath.Join(root, "probe.md"),
			},
			Content: []byte("probe"),
		}},
	})
	return err == nil
}

func wireSnapshots(snapshots []repotx.Snapshot) []any {
	wire := make([]any, 0, len(snapshots))
	for _, snapshot := range snapshots {
		wire = append(wire, map[string]any{
			"path_kind":        string(snapshot.Kind),
			"path":             snapshot.Path,
			"original_sha256":  snapshot.OriginalSHA256,
			"candidate_sha256": snapshot.CandidateSHA256,
			"current_sha256":   snapshot.CurrentSHA256,
		})
	}
	return wire
}
