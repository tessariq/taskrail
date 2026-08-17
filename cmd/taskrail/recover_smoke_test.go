package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/durablefs"
	"github.com/tessariq/taskrail/internal/durabletx"
	"github.com/tessariq/taskrail/internal/repolock"
)

// The `recover` command surface over fabricated retained state: the exact bytes
// one interrupted durable transaction leaves behind, driven through the real
// CLI so the human text, the schema-1 envelope, and the fence interplay are all
// what an operator or agent would see
// (specs/v0.5.0.md#repository-discovery-locking-and-recovery).

const recoverTxID = "0123456789abcdef0123456789abcdef"

// recoverMember is one fabricated manifest member plus the physical state the
// interruption left behind. committedRoot is the repository root every kind
// except Git binds beneath; Git members bind beneath committedRoot/.git.
type recoverMember struct {
	kind      durabletx.PathKind
	reported  string
	path      string
	original  []byte
	candidate []byte
	present   bool
	onDisk    []byte
}

type recoverIdentity struct {
	Volume uint64 `json:"volume"`
	File   uint64 `json:"file"`
	Mount  uint64 `json:"mount"`
}

type recoverState struct {
	SHA256   string           `json:"sha256"`
	Mode     uint32           `json:"mode"`
	Identity *recoverIdentity `json:"identity"`
}

type recoverManifestMember struct {
	Kind      durabletx.PathKind `json:"kind"`
	Reported  string             `json:"reported"`
	Path      string             `json:"path"`
	Published bool               `json:"published"`
	Ancestors []recoverIdentity  `json:"ancestors"`
	Original  *recoverState      `json:"original"`
	Candidate *recoverState      `json:"candidate"`
}

type recoverManifest struct {
	TransactionID string                  `json:"transaction_id"`
	Command       string                  `json:"command"`
	Members       []recoverManifestMember `json:"members"`
}

type recoverJournal struct {
	TransactionID string `json:"transaction_id"`
	Command       string `json:"command"`
	Phase         string `json:"phase"`
}

type recoverCompletion struct {
	Action   string          `json:"action"`
	Manifest recoverManifest `json:"manifest"`
}

// fabricateRetainedCLI writes the retained state one interrupted durable
// transaction leaves behind, in the engine's canonical document shapes. repo is
// the lock-protocol repository the CLI's discovery would resolve.
func fabricateRetainedCLI(t *testing.T, repo repolock.Repository, id, command, phase, completion string, members []recoverMember) {
	t.Helper()
	sorted := slices.Clone(members)
	slices.SortStableFunc(sorted, func(a, b recoverMember) int {
		if kinds := strings.Compare(string(a.kind), string(b.kind)); kinds != 0 {
			return kinds
		}
		return strings.Compare(a.reported, b.reported)
	})
	for _, member := range sorted {
		physical := filepath.Join(recoverCLIRootOf(member.kind, repo), filepath.FromSlash(member.path))
		if err := os.MkdirAll(filepath.Dir(physical), 0o755); err != nil {
			t.Fatalf("create parent %s: %v", physical, err)
		}
		if member.original != nil {
			if err := os.WriteFile(physical, member.original, 0o644); err != nil {
				t.Fatalf("place original %s: %v", physical, err)
			}
		}
	}
	manifest := recoverManifest{TransactionID: id, Command: command}
	for i, member := range sorted {
		base := recoverCLIRootOf(member.kind, repo)
		tree, err := durablefs.ObserveTree(base, path.Dir(member.path))
		if err != nil {
			t.Fatalf("observe %s: %v", member.path, err)
		}
		ancestors := slices.Clone(tree.Ancestors)
		if tree.Present {
			ancestors = append(ancestors, tree.Identity)
		}
		recorded := recoverManifestMember{
			Kind: member.kind, Reported: member.reported, Path: member.path,
			Published: member.candidate != nil,
			Ancestors: make([]recoverIdentity, 0, len(ancestors)),
		}
		for _, ancestor := range ancestors {
			recorded.Ancestors = append(recorded.Ancestors, recoverIdentity(ancestor))
		}
		if member.original != nil {
			data, snapshot, err := durablefs.ReadFile(base, member.path, 1<<20)
			if err != nil || string(data) != string(member.original) {
				t.Fatalf("observe original %s: %v", member.path, err)
			}
			identity := recoverIdentity(snapshot.Identity)
			recorded.Original = &recoverState{SHA256: snapshot.SHA256, Mode: uint32(snapshot.Mode), Identity: &identity}
			originalPath := filepath.Join(durabletx.TransactionsDir(repo), id, "originals", fmt.Sprintf("%08d", i))
			if err := os.MkdirAll(filepath.Dir(originalPath), 0o755); err != nil {
				t.Fatalf("create originals: %v", err)
			}
			if err := os.WriteFile(originalPath, member.original, 0o644); err != nil {
				t.Fatalf("record original %s: %v", originalPath, err)
			}
		}
		if member.candidate != nil {
			recorded.Candidate = &recoverState{SHA256: recoverDigest(member.candidate), Mode: uint32(durablefs.PortableMode(0o644))}
		}
		manifest.Members = append(manifest.Members, recorded)
	}
	for _, member := range sorted {
		physical := filepath.Join(recoverCLIRootOf(member.kind, repo), filepath.FromSlash(member.path))
		switch {
		case member.present:
			if err := os.WriteFile(physical, member.onDisk, 0o644); err != nil {
				t.Fatalf("place current %s: %v", physical, err)
			}
		default:
			if err := os.RemoveAll(physical); err != nil {
				t.Fatalf("remove current %s: %v", physical, err)
			}
		}
	}
	tx := durabletx.TransactionsDir(repo)
	if err := os.MkdirAll(filepath.Join(tx, id), 0o755); err != nil {
		t.Fatalf("create transactions: %v", err)
	}
	writeRecoverCLIJSON(t, filepath.Join(tx, id, "manifest.json"), manifest)
	writeRecoverCLIJSON(t, filepath.Join(tx, id, "journal.json"), recoverJournal{TransactionID: id, Command: command, Phase: phase})
	if completion != "" {
		writeRecoverCLIJSON(t, filepath.Join(tx, id, "complete.json"), recoverCompletion{Action: completion, Manifest: manifest})
	}
}

func recoverCLIRootOf(kind durabletx.PathKind, repo repolock.Repository) string {
	if kind == durabletx.Git {
		return repo.GitCommonDir
	}
	return repo.Root
}

func recoverDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeRecoverCLIJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode %s: %v", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// recoverResultJSON re-declares the RecoverResult fields the acceptance
// assertions pin, so an accidental shape change fails here rather than only in
// a consumer.
type recoverResultJSON struct {
	TransactionID string `json:"transaction_id"`
	Command       string `json:"command"`
	Action        string `json:"action"`
	Applied       bool   `json:"applied"`
	Snapshots     []struct {
		PathKind  string  `json:"path_kind"`
		Path      string  `json:"path"`
		Original  *string `json:"original_sha256"`
		Candidate *string `json:"candidate_sha256"`
		Current   *string `json:"current_sha256"`
	} `json:"snapshots"`
	Validation struct {
		Valid      bool     `json:"valid"`
		Violations []string `json:"violations"`
	} `json:"validation"`
}

func TestRecoverCommandPreviewsClearFenceReadOnlyThenApplies(t *testing.T) {
	root := setupRepo(t)
	requireRecoveryDirectoryDurability(t, root)
	fabricateRetainedCLI(t, gitLockRepository(root), recoverTxID, "init", "prepared", "", []recoverMember{
		{kind: durabletx.Managed, reported: "planning/A.md", path: "planning/A.md",
			original: []byte("before"), candidate: []byte("after"), present: true, onDisk: []byte("before")},
	})

	// The fence admits recover and refuses every other semantic command.
	if _, err := runRoot(t, "status", "--json"); err == nil {
		t.Fatal("status worked under a retained fence")
	}
	before := readAllFiles(t, root)
	out, err := runRoot(t, "recover", recoverTxID)
	if err != nil {
		t.Fatalf("preview: %v (%s)", err, out)
	}
	if !strings.Contains(out, "clear_fence") || !strings.Contains(out, "preview") ||
		!strings.Contains(out, "apply with: taskrail recover "+recoverTxID+" --apply") {
		t.Fatalf("preview text = %q", out)
	}
	if after := readAllFiles(t, root); !sameFileSet(before, after) {
		t.Fatal("preview changed the repository's files")
	}
	if _, err := runRoot(t, "status", "--json"); err == nil {
		t.Fatal("preview cleared the fence")
	}

	out, err = runRoot(t, "recover", recoverTxID, "--apply")
	if err != nil {
		t.Fatalf("apply: %v (%s)", err, out)
	}
	if !strings.Contains(out, "applied: clear_fence") {
		t.Fatalf("apply text = %q", out)
	}
	if _, err := runRoot(t, "status", "--json"); err != nil {
		t.Fatalf("repository still fenced after apply: %v", err)
	}
	if got := readFileAt(t, filepath.Join(root, "planning", "A.md")); got != "before" {
		t.Fatalf("A = %q, want the untouched original", got)
	}
}

func TestRecoverCommandRestoresMixedPublication(t *testing.T) {
	root := setupRepo(t)
	requireRecoveryDirectoryDurability(t, root)
	exclusion := filepath.ToSlash(filepath.Join(root, ".git", "info", "exclude"))
	fabricateRetainedCLI(t, gitLockRepository(root), recoverTxID, "init", "publishing", "", []recoverMember{
		{kind: durabletx.Git, reported: exclusion, path: "info/exclude",
			original: []byte("old-exclusion"), candidate: []byte("new-exclusion"), present: true, onDisk: []byte("new-exclusion")},
		{kind: durabletx.Managed, reported: "planning/A.md", path: "planning/A.md",
			original: []byte("old-a"), candidate: []byte("new-a"), present: true, onDisk: []byte("new-a")},
		{kind: durabletx.Managed, reported: "planning/B.md", path: "planning/B.md",
			original: []byte("old-b"), candidate: []byte("new-b"), present: true, onDisk: []byte("old-b")},
		{kind: durabletx.Worktree, reported: ".claude/skills/taskrail-gap/SKILL.md", path: ".claude/skills/taskrail-gap/SKILL.md",
			original: []byte("old-skill"), candidate: []byte("new-skill"), present: true, onDisk: []byte("new-skill")},
	})

	jsonOut, err := runRoot(t, "recover", recoverTxID, "--json")
	if err != nil {
		t.Fatalf("preview: %v (%s)", err, jsonOut)
	}
	var preview recoverResultJSON
	decodeMachineResult(t, jsonOut, &preview)
	if preview.TransactionID != recoverTxID || preview.Command != "init" ||
		preview.Action != "restore_original" || preview.Applied {
		t.Fatalf("preview result = %+v", preview)
	}
	// Typed snapshots name each path class exactly: the Git exclusion store is
	// canonical absolute, the skill destination is worktree-physical, and the
	// semantic members are managed logical paths.
	seen := map[string]int{}
	for _, snapshot := range preview.Snapshots {
		seen[snapshot.PathKind+" "+snapshot.Path]++
	}
	if seen["git "+exclusion] != 1 {
		t.Fatalf("snapshots = %+v, want the absolute exclusion path", preview.Snapshots)
	}
	if seen["worktree .claude/skills/taskrail-gap/SKILL.md"] != 1 {
		t.Fatalf("snapshots = %+v, want the worktree skill destination", preview.Snapshots)
	}

	jsonOut, err = runRoot(t, "recover", recoverTxID, "--apply", "--json")
	if err != nil {
		t.Fatalf("apply: %v (%s)", err, jsonOut)
	}
	var applied recoverResultJSON
	decodeMachineResult(t, jsonOut, &applied)
	if !applied.Applied || applied.Action != "restore_original" || applied.Validation.Valid != true {
		t.Fatalf("applied result = %+v", applied)
	}
	if got := readFileAt(t, filepath.Join(root, "planning", "A.md")); got != "old-a" {
		t.Fatalf("A = %q, want restored", got)
	}
	if got := readFileAt(t, filepath.Join(root, "planning", "B.md")); got != "old-b" {
		t.Fatalf("B = %q, want untouched original", got)
	}
	if got := readFileAt(t, filepath.Join(root, ".claude", "skills", "taskrail-gap", "SKILL.md")); got != "old-skill" {
		t.Fatalf("skill = %q, want restored", got)
	}
	if got := readFileAt(t, filepath.Join(root, ".git", "info", "exclude")); got != "old-exclusion" {
		t.Fatalf("exclusion = %q, want restored", got)
	}
}

func TestRecoverCommandRefusalsClassifyAndPreserveEvidence(t *testing.T) {
	tests := []struct {
		name string
		args []string
		code string
	}{
		{"malformed id", []string{"recover", "NOT-HEX"}, "invalid_arguments"},
		{"unknown id", []string{"recover", strings.Repeat("b", 32)}, "invalid_arguments"},
		{"argument count", []string{"recover"}, "invalid_arguments"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupRepo(t)
			args := append(append([]string{}, test.args...), "--json")
			out, err := runRoot(t, args...)
			if err == nil {
				t.Fatalf("expected a refusal, got %q", out)
			}
			if failure := decodeMachineError(t, out); failure.Code != test.code {
				t.Fatalf("code = %q, want %q (%s)", failure.Code, test.code, failure.Message)
			}
		})
	}

	// An accept_candidate derivation whose owning command has shipped no
	// recovery validator refuses with validation_failed and preserves every
	// byte, including the fence.
	root := setupRepo(t)
	fabricateRetainedCLI(t, gitLockRepository(root), recoverTxID, "import", "candidate_published", "", []recoverMember{
		{kind: durabletx.Managed, reported: "planning/A.md", path: "planning/A.md",
			original: []byte("old-a"), candidate: []byte("new-a"), present: true, onDisk: []byte("new-a")},
	})
	before := readAllFiles(t, root)
	out, err := runRoot(t, "recover", recoverTxID, "--apply", "--json")
	if err == nil {
		t.Fatalf("expected a refusal, got %q", out)
	}
	failure := decodeMachineError(t, out)
	if failure.Code != "validation_failed" {
		t.Fatalf("code = %q, want validation_failed (%s)", failure.Code, failure.Message)
	}
	if after := readAllFiles(t, root); !sameFileSet(before, after) {
		t.Fatal("the refusal changed repository bytes")
	}
}

func TestRecoverCommandRefusesUnexpectedBytesAsWriteConflict(t *testing.T) {
	root := setupRepo(t)
	fabricateRetainedCLI(t, gitLockRepository(root), recoverTxID, "init", "publishing", "", []recoverMember{
		{kind: durabletx.Managed, reported: "planning/A.md", path: "planning/A.md",
			original: []byte("old-a"), candidate: []byte("new-a"), present: true, onDisk: []byte("new-a")},
		{kind: durabletx.Managed, reported: "planning/B.md", path: "planning/B.md",
			original: []byte("old-b"), candidate: []byte("new-b"), present: true, onDisk: []byte("old-b")},
	})
	if err := os.WriteFile(filepath.Join(root, "planning", "B.md"), []byte("external"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := readAllFiles(t, root)

	out, err := runRoot(t, "recover", recoverTxID, "--apply", "--json")
	if err == nil {
		t.Fatalf("expected a refusal, got %q", out)
	}
	failure := decodeMachineError(t, out)
	if failure.Code != "write_conflict" {
		t.Fatalf("code = %q, want write_conflict (%s)", failure.Code, failure.Message)
	}
	if failure.Details.Recovery == nil || failure.Details.Recovery.TransactionID != recoverTxID {
		t.Fatalf("recovery ref = %+v", failure.Details.Recovery)
	}
	if len(failure.Details.Snapshots) != 2 {
		t.Fatalf("snapshots = %+v, want the complete member evidence", failure.Details.Snapshots)
	}
	if after := readAllFiles(t, root); !sameFileSet(before, after) {
		t.Fatal("a refused recovery overwrote external bytes")
	}
}

// The lock a transaction named is part of its identity: any holder — live or
// abandoned — refuses recovery before any evidence is read, the refusal names
// the operator lock surface, and the held lock is left exactly where it is.
func TestRecoverCommandRefusesLockHolders(t *testing.T) {
	root := setupRepo(t)
	raw := seedStaleLock(t, root)
	fabricateRetainedCLI(t, gitLockRepository(root), recoverTxID, "init", "prepared", "", []recoverMember{
		{kind: durabletx.Managed, reported: "planning/A.md", path: "planning/A.md",
			original: []byte("before"), candidate: []byte("after"), present: true, onDisk: []byte("before")},
	})

	out, err := runRoot(t, "recover", recoverTxID, "--json")
	if err == nil {
		t.Fatalf("expected a refusal, got %q", out)
	}
	failure := decodeMachineError(t, out)
	if failure.Code != "lock_held" {
		t.Fatalf("code = %q, want lock_held (%s)", failure.Code, failure.Message)
	}
	if !strings.Contains(failure.Message, "taskrail lock status") {
		t.Fatalf("refusal does not name the operator path: %s", failure.Message)
	}
	if after, readErr := os.ReadFile(repolock.LockPath(gitLockRepository(root))); readErr != nil || string(after) != string(raw) {
		t.Fatalf("the refusal disturbed the held lock: %q %v", after, readErr)
	}
}

// A completed recovery whose cleanup was interrupted resumes to a valid
// unfenced state — the accept path needs no validator, because the recorded
// completion already carries the owning command's validated decision.
func TestRecoverCommandResumesCompletedAccept(t *testing.T) {
	root := setupRepo(t)
	requireRecoveryDirectoryDurability(t, root)
	fabricateRetainedCLI(t, gitLockRepository(root), recoverTxID, "import", "recovery_accepting", "accept_candidate", []recoverMember{
		{kind: durabletx.Managed, reported: "planning/A.md", path: "planning/A.md",
			original: []byte("old-a"), candidate: []byte("new-a"), present: true, onDisk: []byte("new-a")},
	})

	jsonOut, err := runRoot(t, "recover", recoverTxID, "--apply", "--json")
	if err != nil {
		t.Fatalf("apply: %v (%s)", err, jsonOut)
	}
	var applied recoverResultJSON
	decodeMachineResult(t, jsonOut, &applied)
	if !applied.Applied || applied.Action != "accept_candidate" || applied.Command != "import" {
		t.Fatalf("applied = %+v", applied)
	}
	if got := readFileAt(t, filepath.Join(root, "planning", "A.md")); got != "new-a" {
		t.Fatalf("A = %q, want the accepted candidate", got)
	}
	if _, err := runRoot(t, "status", "--json"); err != nil {
		t.Fatalf("repository still fenced: %v", err)
	}

	// A second invocation names no retained transaction anymore.
	out, err := runRoot(t, "recover", recoverTxID, "--json")
	if err == nil {
		t.Fatalf("expected a refusal, got %q", out)
	}
	if failure := decodeMachineError(t, out); failure.Code != "invalid_arguments" {
		t.Fatalf("code = %q, want invalid_arguments (%s)", failure.Code, failure.Message)
	}
}

// A non-Git root-local repository retains its transactions beneath
// `.taskrail/runtime/`, and recovery works there without Git metadata.
func TestRecoverCommandWorksInANonGitRepository(t *testing.T) {
	root := t.TempDir()
	requireRecoveryDirectoryDurability(t, root)
	if err := os.MkdirAll(filepath.Join(root, ".taskrail"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".taskrail", "config.yml"), []byte("layout_version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	if err := os.MkdirAll(filepath.Join(root, "planning"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "planning", "A.md"), []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	fabricateRetainedCLI(t, repolock.Repository{Root: root, Mode: repolock.ModeCommitted}, recoverTxID, "init", "prepared", "", []recoverMember{
		{kind: durabletx.Managed, reported: "planning/A.md", path: "planning/A.md",
			original: []byte("before"), candidate: []byte("after"), present: true, onDisk: []byte("before")},
	})
	retained := filepath.Join(root, ".taskrail", "runtime", "transactions", recoverTxID)
	if info, err := os.Stat(retained); err != nil || !info.IsDir() {
		t.Fatalf("retained state resolved to %s: %v", retained, err)
	}
	out, err := runRoot(t, "recover", recoverTxID, "--apply", "--json")
	if err != nil {
		t.Fatalf("apply: %v (%s)", err, out)
	}
	var applied recoverResultJSON
	decodeMachineResult(t, out, &applied)
	if !applied.Applied || applied.Action != "clear_fence" {
		t.Fatalf("applied = %+v", applied)
	}
	if _, err := os.Stat(retained); !os.IsNotExist(err) {
		t.Fatal("non-Git recovery retained the fence")
	}
}

func requireRecoveryDirectoryDurability(t *testing.T, root string) {
	t.Helper()
	directory, err := os.Open(root)
	if err != nil {
		t.Fatalf("open directory durability probe: %v", err)
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		t.Skipf("filesystem does not support durable directory sync: %v", syncErr)
	}
	if closeErr != nil {
		t.Fatalf("close directory durability probe: %v", closeErr)
	}
}

func readFileAt(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// sameFileSet compares two readAllFiles maps for exact byte equality.
func sameFileSet(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for path, content := range a {
		if other, ok := b[path]; !ok || other != content {
			return false
		}
	}
	return true
}
