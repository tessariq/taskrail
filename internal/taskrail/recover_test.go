package taskrail

import (
	"context"
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
	"time"

	"github.com/tessariq/taskrail/internal/durablefs"
	"github.com/tessariq/taskrail/internal/durabletx"
	"github.com/tessariq/taskrail/internal/repolock"
)

// The public recovery boundary over fabricated retained state: the exact bytes
// one interrupted durable transaction leaves behind, written without the
// engine's private test hooks so the service observes what any crash would
// leave for `taskrail recover` (specs/v0.5.0.md#repository-discovery-locking-and-recovery).

const recoverFixtureID = "0123456789abcdef0123456789abcdef"

// recoverMember is one fabricated manifest member plus the physical state the
// interruption left behind.
type recoverMember struct {
	kind     durabletx.PathKind
	reported string
	path     string
	// original records the exact pre-transaction state; nil records absence.
	original []byte
	// candidate records the intended publication; nil records a consumed-only
	// member the transaction never publishes.
	candidate []byte
	// present and onDisk describe the member as recovery finds it.
	present bool
	onDisk  []byte
}

// fabricated document mirrors the engine's canonical journal, manifest, and
// completion shapes member-for-member, because loadRetained decodes strictly.
type fabricatedIdentity struct {
	Volume uint64 `json:"volume"`
	File   uint64 `json:"file"`
	Mount  uint64 `json:"mount"`
}

type fabricatedState struct {
	SHA256   string              `json:"sha256"`
	Mode     uint32              `json:"mode"`
	Identity *fabricatedIdentity `json:"identity"`
}

type fabricatedManifestMember struct {
	Kind      durabletx.PathKind   `json:"kind"`
	Reported  string               `json:"reported"`
	Path      string               `json:"path"`
	Published bool                 `json:"published"`
	Ancestors []fabricatedIdentity `json:"ancestors"`
	Original  *fabricatedState     `json:"original"`
	Candidate *fabricatedState     `json:"candidate"`
}

type fabricatedManifest struct {
	TransactionID string                     `json:"transaction_id"`
	Command       string                     `json:"command"`
	Members       []fabricatedManifestMember `json:"members"`
}

type fabricatedJournal struct {
	TransactionID string `json:"transaction_id"`
	Command       string `json:"command"`
	Phase         string `json:"phase"`
}

type fabricatedCompletion struct {
	Action   string             `json:"action"`
	Manifest fabricatedManifest `json:"manifest"`
}

// fabricateRetained writes the retained state one interrupted durable
// transaction leaves behind: originals observed, then the interruption's
// partial effect placed, then journal, manifest, and originals recorded in the
// engine's canonical shapes. completion, when non-empty, additionally records
// a finished action whose cleanup was interrupted.
func fabricateRetained(t *testing.T, repo repolock.Repository, id, command, phase string, members []recoverMember, completion string) {
	t.Helper()
	sorted := slices.Clone(members)
	slices.SortStableFunc(sorted, func(a, b recoverMember) int {
		if kinds := strings.Compare(string(a.kind), string(b.kind)); kinds != 0 {
			return kinds
		}
		return strings.Compare(a.reported, b.reported)
	})

	// Place every recorded original first, so each original's digest, mode, and
	// native identity are observed exactly as the writer would have recorded
	// them, and every parent directory exists for ancestor evidence.
	for _, member := range sorted {
		physical := filepath.Join(recoverRootOf(member.kind, repo), filepath.FromSlash(member.path))
		if err := os.MkdirAll(filepath.Dir(physical), 0o755); err != nil {
			t.Fatalf("create parent %s: %v", filepath.Dir(physical), err)
		}
		if member.original != nil {
			if err := os.WriteFile(physical, member.original, 0o644); err != nil {
				t.Fatalf("place original %s: %v", physical, err)
			}
		}
	}

	manifest := fabricatedManifest{TransactionID: id, Command: command}
	for i, member := range sorted {
		base := recoverRootOf(member.kind, repo)
		parent := path.Dir(member.path)
		tree, err := durablefs.ObserveTree(base, parent)
		if err != nil {
			t.Fatalf("observe %s: %v", parent, err)
		}
		ancestors := slices.Clone(tree.Ancestors)
		if tree.Present {
			ancestors = append(ancestors, tree.Identity)
		}
		recorded := fabricatedManifestMember{
			Kind: member.kind, Reported: member.reported, Path: member.path,
			Published: member.candidate != nil,
			Ancestors: make([]fabricatedIdentity, 0, len(ancestors)),
		}
		for _, ancestor := range ancestors {
			recorded.Ancestors = append(recorded.Ancestors, fabricatedIdentity(ancestor))
		}
		if member.original != nil {
			data, snapshot, err := durablefs.ReadFile(base, member.path, 1<<20)
			if err != nil || string(data) != string(member.original) {
				t.Fatalf("observe original %s: %v", member.path, err)
			}
			identity := fabricatedIdentity(snapshot.Identity)
			recorded.Original = &fabricatedState{
				SHA256: snapshot.SHA256, Mode: uint32(snapshot.Mode), Identity: &identity,
			}
			originalPath := filepath.Join(durabletx.TransactionsDir(repo), id, "originals", fmt.Sprintf("%08d", i))
			if err := os.MkdirAll(filepath.Dir(originalPath), 0o755); err != nil {
				t.Fatalf("create originals: %v", err)
			}
			if err := os.WriteFile(originalPath, member.original, 0o644); err != nil {
				t.Fatalf("record original %s: %v", originalPath, err)
			}
		}
		if member.candidate != nil {
			recorded.Candidate = &fabricatedState{
				SHA256: sha256HexDigest(member.candidate), Mode: uint32(durablefs.PortableMode(0o644)),
			}
		}
		manifest.Members = append(manifest.Members, recorded)
	}

	// Place the interruption's effect: the mixed or complete candidate state
	// recovery will observe.
	for _, member := range sorted {
		physical := filepath.Join(recoverRootOf(member.kind, repo), filepath.FromSlash(member.path))
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
	if err := os.MkdirAll(tx, 0o755); err != nil {
		t.Fatalf("create transactions: %v", err)
	}
	writeRecoverJSON(t, filepath.Join(tx, id, "manifest.json"), manifest)
	writeRecoverJSON(t, filepath.Join(tx, id, "journal.json"), fabricatedJournal{TransactionID: id, Command: command, Phase: phase})
	if completion != "" {
		writeRecoverJSON(t, filepath.Join(tx, id, "complete.json"), fabricatedCompletion{Action: completion, Manifest: manifest})
	}
}

func recoverRootOf(kind durabletx.PathKind, repo repolock.Repository) string {
	switch kind {
	case durabletx.Managed:
		return repo.StorageRoot()
	case durabletx.Git:
		return repo.GitCommonDir
	default:
		return repo.Root
	}
}

func sha256HexDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeRecoverJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode %s: %v", filepath.Base(path), err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func newRecoverService(t *testing.T, paths Paths) *Service {
	t.Helper()
	return &Service{paths: paths, now: time.Now}
}

func committedRecoverPaths(t *testing.T, root string) Paths {
	t.Helper()
	return pathsFromLayout(root, defaultLayoutConfig(), committedStorage())
}

func TestRecoverTransactionRejectsMalformedID(t *testing.T) {
	t.Parallel()
	svc := newRecoverService(t, committedRecoverPaths(t, t.TempDir()))
	_, err := svc.RecoverTransaction(context.Background(), "NOT-HEX", false)
	if MachineFailureFor(err).Code != MachineCodeInvalidArguments {
		t.Fatalf("code = %q, want invalid_arguments (%v)", MachineFailureFor(err).Code, err)
	}
}

func TestRecoverTransactionUnknownIDRefuses(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	svc := newRecoverService(t, committedRecoverPaths(t, root))
	_, err := svc.RecoverTransaction(context.Background(), recoverFixtureID, false)
	if MachineFailureFor(err).Code != MachineCodeInvalidArguments {
		t.Fatalf("code = %q, want invalid_arguments (%v)", MachineFailureFor(err).Code, err)
	}
}

func TestRecoverTransactionClearsPreparedFence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedFixtureTree(t, root)
	paths := committedRecoverPaths(t, root)
	fabricateRetained(t, paths.LockRepository(), recoverFixtureID, "init", "prepared", []recoverMember{
		{kind: durabletx.Managed, reported: "planning/A.md", path: "planning/A.md",
			original: []byte("before"), candidate: []byte("after"), present: true, onDisk: []byte("before")},
	}, "")
	svc := newRecoverService(t, paths)

	preview, err := svc.RecoverTransaction(context.Background(), recoverFixtureID, false)
	if err != nil {
		t.Fatalf("preview recovery: %v", err)
	}
	if preview.Action != "clear_fence" || preview.Applied || preview.Command != "init" {
		t.Fatalf("preview = %+v, want unapplied clear_fence for init", preview)
	}
	if got := readFileString(t, filepath.Join(root, "planning", "A.md")); got != "before" {
		t.Fatalf("preview changed bytes: %q", got)
	}
	if _, statErr := os.Stat(filepath.Join(durabletx.TransactionsDir(paths.LockRepository()), recoverFixtureID)); statErr != nil {
		t.Fatalf("preview disturbed retained state: %v", statErr)
	}
	if !preview.Validation.Valid {
		t.Fatalf("preview validation = %+v", preview.Validation)
	}

	applied, err := svc.RecoverTransaction(context.Background(), recoverFixtureID, true)
	if err != nil {
		t.Fatalf("apply recovery: %v", err)
	}
	if !applied.Applied || applied.Action != "clear_fence" {
		t.Fatalf("applied = %+v", applied)
	}
	if _, statErr := os.Stat(filepath.Join(durabletx.TransactionsDir(paths.LockRepository()), recoverFixtureID)); !os.IsNotExist(statErr) {
		t.Fatalf("applied clear retained the fence: %v", statErr)
	}
}

func TestRecoverTransactionRestoresOnlyCandidateValuedComponents(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	paths := committedRecoverPaths(t, root)
	repo := paths.LockRepository()
	fabricateRetained(t, repo, recoverFixtureID, "init", "publishing", []recoverMember{
		{kind: durabletx.Managed, reported: "planning/A.md", path: "planning/A.md",
			original: []byte("old-a"), candidate: []byte("new-a"), present: true, onDisk: []byte("new-a")},
		{kind: durabletx.Managed, reported: "planning/B.md", path: "planning/B.md",
			original: []byte("old-b"), candidate: []byte("new-b"), present: true, onDisk: []byte("old-b")},
		{kind: durabletx.Managed, reported: "planning/C.md", path: "planning/C.md",
			original: []byte("consumed"), present: true, onDisk: []byte("consumed")},
	}, "")
	svc := newRecoverService(t, paths)

	preview, err := svc.RecoverTransaction(context.Background(), recoverFixtureID, false)
	if err != nil {
		t.Fatalf("preview recovery: %v", err)
	}
	if preview.Action != "restore_original" || preview.Applied {
		t.Fatalf("preview = %+v, want unapplied restore_original", preview)
	}

	applied, err := svc.RecoverTransaction(context.Background(), recoverFixtureID, true)
	if err != nil {
		t.Fatalf("apply recovery: %v", err)
	}
	if !applied.Applied || applied.Action != "restore_original" {
		t.Fatalf("applied = %+v", applied)
	}
	if got := readFileString(t, filepath.Join(root, "planning", "A.md")); got != "old-a" {
		t.Fatalf("A = %q, want restored original", got)
	}
	if got := readFileString(t, filepath.Join(root, "planning", "B.md")); got != "old-b" {
		t.Fatalf("B = %q, want untouched original", got)
	}
	if got := readFileString(t, filepath.Join(root, "planning", "C.md")); got != "consumed" {
		t.Fatalf("C = %q, want untouched consumed member", got)
	}
	if _, statErr := os.Stat(filepath.Join(durabletx.TransactionsDir(repo), recoverFixtureID)); !os.IsNotExist(statErr) {
		t.Fatalf("restore retained the fence: %v", statErr)
	}
	if len(applied.Snapshots) != 3 {
		t.Fatalf("snapshots = %+v, want the complete three-member set", applied.Snapshots)
	}
}

func TestRecoverTransactionRefusesUnexpectedBytes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	paths := committedRecoverPaths(t, root)
	repo := paths.LockRepository()
	fabricateRetained(t, repo, recoverFixtureID, "init", "publishing", []recoverMember{
		{kind: durabletx.Managed, reported: "planning/A.md", path: "planning/A.md",
			original: []byte("old-a"), candidate: []byte("new-a"), present: true, onDisk: []byte("new-a")},
		{kind: durabletx.Managed, reported: "planning/B.md", path: "planning/B.md",
			original: []byte("old-b"), candidate: []byte("new-b"), present: true, onDisk: []byte("old-b")},
	}, "")
	if err := os.WriteFile(filepath.Join(root, "planning", "B.md"), []byte("external"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newRecoverService(t, paths)

	for _, apply := range []bool{false, true} {
		_, err := svc.RecoverTransaction(context.Background(), recoverFixtureID, apply)
		failure := MachineFailureFor(err)
		if err == nil || failure.Code != MachineCodeWriteConflict {
			t.Fatalf("apply=%v code = %q, want write_conflict (%v)", apply, failure.Code, err)
		}
		if failure.Recovery == nil || failure.Recovery.TransactionID != recoverFixtureID || failure.Recovery.Command != "init" {
			t.Fatalf("apply=%v recovery ref = %+v", apply, failure.Recovery)
		}
		if len(failure.Snapshots) != 2 {
			t.Fatalf("apply=%v snapshots = %+v", apply, failure.Snapshots)
		}
	}
	if got := readFileString(t, filepath.Join(root, "planning", "B.md")); got != "external" {
		t.Fatalf("external bytes = %q, want them untouched", got)
	}
	if got := readFileString(t, filepath.Join(root, "planning", "A.md")); got != "new-a" {
		t.Fatalf("A = %q, want candidate bytes untouched by the refusal", got)
	}
	if _, statErr := os.Stat(filepath.Join(durabletx.TransactionsDir(repo), recoverFixtureID)); statErr != nil {
		t.Fatalf("refusal lost retained evidence: %v", statErr)
	}
}

func TestRecoverTransactionRefusesSubstitutedAncestor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	paths := committedRecoverPaths(t, root)
	repo := paths.LockRepository()
	fabricateRetained(t, repo, recoverFixtureID, "init", "publishing", []recoverMember{
		{kind: durabletx.Managed, reported: "planning/A.md", path: "planning/A.md",
			original: []byte("old-a"), candidate: []byte("new-a"), present: true, onDisk: []byte("new-a")},
		{kind: durabletx.Managed, reported: "planning/tasks/T-900-x.md", path: "planning/tasks/T-900-x.md",
			original: []byte("task"), candidate: []byte("task-new"), present: true, onDisk: []byte("task")},
	}, "")
	// Substitute the tasks directory behind identical leaf bytes: the ancestor
	// identity the manifest recorded no longer prefixes the current chain. The
	// original is renamed aside rather than removed because a remove+recreate
	// can reuse the freed inode, which stat identity cannot distinguish.
	tasks := filepath.Join(root, "planning", "tasks")
	if err := os.Rename(tasks, tasks+"-substituted"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tasks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasks, "T-900-x.md"), []byte("task"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newRecoverService(t, paths)

	_, err := svc.RecoverTransaction(context.Background(), recoverFixtureID, true)
	failure := MachineFailureFor(err)
	if err == nil || failure.Code != MachineCodeWriteConflict {
		t.Fatalf("code = %q, want write_conflict (%v)", failure.Code, err)
	}
	if got := readFileString(t, filepath.Join(tasks, "T-900-x.md")); got != "task" {
		t.Fatalf("substituted bytes = %q, want them untouched", got)
	}
}

func TestRecoverTransactionRefusesLockHolder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	paths := committedRecoverPaths(t, root)
	repo := paths.LockRepository()
	fabricateRetained(t, repo, recoverFixtureID, "init", "prepared", []recoverMember{
		{kind: durabletx.Managed, reported: "planning/A.md", path: "planning/A.md",
			original: []byte("old-a"), candidate: []byte("new-a"), present: true, onDisk: []byte("old-a")},
	}, "")
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: repo, Command: "init",
		Capability: repolock.Capability{Commands: []string{"init"}},
	})
	if err != nil {
		t.Fatalf("acquire holder lock: %v", err)
	}
	defer func() { _ = lock.Release() }()

	svc := newRecoverService(t, paths)
	_, err = svc.RecoverTransaction(context.Background(), recoverFixtureID, false)
	failure := MachineFailureFor(err)
	if err == nil || failure.Code != MachineCodeLockHeld {
		t.Fatalf("code = %q, want lock_held (%v)", failure.Code, err)
	}
	if !strings.Contains(err.Error(), "taskrail lock status") {
		t.Fatalf("refusal does not name the operator path: %v", err)
	}
	if _, statErr := os.Stat(repolock.LockPath(repo)); statErr != nil {
		t.Fatalf("refusal disturbed the held lock: %v", statErr)
	}
}

func TestRecoverTransactionRefusesAcceptWithoutRegisteredValidator(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	paths := committedRecoverPaths(t, root)
	repo := paths.LockRepository()
	fabricateRetained(t, repo, recoverFixtureID, "init", "candidate_published", []recoverMember{
		{kind: durabletx.Managed, reported: "planning/A.md", path: "planning/A.md",
			original: []byte("old-a"), candidate: []byte("new-a"), present: true, onDisk: []byte("new-a")},
	}, "")
	svc := newRecoverService(t, paths)

	for _, apply := range []bool{false, true} {
		_, err := svc.RecoverTransaction(context.Background(), recoverFixtureID, apply)
		failure := MachineFailureFor(err)
		if err == nil || failure.Code != MachineCodeValidationFailed {
			t.Fatalf("apply=%v code = %q, want validation_failed (%v)", apply, failure.Code, err)
		}
		if !strings.Contains(err.Error(), "validator") {
			t.Fatalf("apply=%v refusal does not name the missing validator: %v", apply, err)
		}
	}
	if got := readFileString(t, filepath.Join(root, "planning", "A.md")); got != "new-a" {
		t.Fatalf("A = %q, want candidate bytes preserved by the refusal", got)
	}
	if _, statErr := os.Stat(filepath.Join(durabletx.TransactionsDir(repo), recoverFixtureID)); statErr != nil {
		t.Fatalf("refusal lost retained evidence: %v", statErr)
	}
}

func TestRecoverTransactionResumesCompletedAccept(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	paths := committedRecoverPaths(t, root)
	repo := paths.LockRepository()
	fabricateRetained(t, repo, recoverFixtureID, "import", "recovery_accepting", []recoverMember{
		{kind: durabletx.Managed, reported: "planning/A.md", path: "planning/A.md",
			original: []byte("old-a"), candidate: []byte("new-a"), present: true, onDisk: []byte("new-a")},
	}, "accept_candidate")
	svc := newRecoverService(t, paths)

	preview, err := svc.RecoverTransaction(context.Background(), recoverFixtureID, false)
	if err != nil {
		t.Fatalf("preview recovery: %v", err)
	}
	if preview.Action != "accept_candidate" || preview.Applied {
		t.Fatalf("preview = %+v, want unapplied accept_candidate", preview)
	}
	applied, err := svc.RecoverTransaction(context.Background(), recoverFixtureID, true)
	if err != nil {
		t.Fatalf("apply recovery: %v", err)
	}
	if !applied.Applied || applied.Action != "accept_candidate" || applied.Command != "import" {
		t.Fatalf("applied = %+v", applied)
	}
	if got := readFileString(t, filepath.Join(root, "planning", "A.md")); got != "new-a" {
		t.Fatalf("A = %q, want the accepted candidate", got)
	}
	if _, statErr := os.Stat(filepath.Join(durabletx.TransactionsDir(repo), recoverFixtureID)); !os.IsNotExist(statErr) {
		t.Fatalf("resumed cleanup retained the fence: %v", statErr)
	}
}

// The A4 mixed-path contract: one transaction carrying managed logical,
// worktree-physical, and canonical absolute Git exclusion-store members recovers
// through each typed root. The managed and worktree members deliberately share
// one reported spelling, which only the path kinds keep distinct, and the
// managed member reports its logical namespace rather than the physical overlay
// location its bytes live at.
func TestRecoverTransactionRecoversMixedPathKindsInLocalStorage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	seedFixtureTree(t, filepath.Join(root, localStorageRoot))
	paths := pathsFromDiscovery(root, defaultLayoutConfig(), localStorage(), gitContext{
		WorktreeRoot: root,
		GitDir:       filepath.Join(root, ".git"),
		GitCommonDir: filepath.Join(root, ".git"),
	})
	repo := paths.LockRepository()
	exclusion := filepath.ToSlash(filepath.Join(repo.GitCommonDir, "info", "exclude"))
	fabricateRetained(t, repo, recoverFixtureID, "local promote", "publishing", []recoverMember{
		{kind: durabletx.Git, reported: exclusion, path: "info/exclude",
			original: []byte("old-exclusion"), candidate: []byte("new-exclusion"), present: true, onDisk: []byte("new-exclusion")},
		{kind: durabletx.Managed, reported: "planning/STATE.md", path: "planning/STATE.md",
			original: []byte("overlay-before"), candidate: []byte("overlay-after"), present: true, onDisk: []byte("overlay-after")},
		{kind: durabletx.Worktree, reported: "planning/STATE.md", path: "planning/STATE.md",
			original: []byte("decoy-before"), candidate: []byte("decoy-after"), present: true, onDisk: []byte("decoy-before")},
	}, "")
	svc := newRecoverService(t, paths)

	preview, err := svc.RecoverTransaction(context.Background(), recoverFixtureID, false)
	if err != nil {
		t.Fatalf("preview recovery: %v", err)
	}
	if preview.Action != "restore_original" {
		t.Fatalf("action = %q, want restore_original", preview.Action)
	}
	if len(preview.Snapshots) != 3 {
		t.Fatalf("snapshots = %+v", preview.Snapshots)
	}
	for _, snapshot := range preview.Snapshots {
		if strings.Contains(snapshot.Path, localStorageRoot) {
			t.Fatalf("snapshot %q exposes the physical overlay as semantic data", snapshot.Path)
		}
		if snapshot.PathKind == "git" && !filepath.IsAbs(snapshot.Path) {
			t.Fatalf("git snapshot %q is not the canonical absolute exclusion path", snapshot.Path)
		}
	}
	kinds := map[string]int{}
	for _, snapshot := range preview.Snapshots {
		if snapshot.Path != "planning/STATE.md" {
			continue
		}
		kinds[snapshot.PathKind]++
	}
	if kinds["managed"] != 1 || kinds["worktree"] != 1 {
		t.Fatalf("equal-looking members were conflated: %+v", preview.Snapshots)
	}

	applied, err := svc.RecoverTransaction(context.Background(), recoverFixtureID, true)
	if err != nil {
		t.Fatalf("apply recovery: %v", err)
	}
	if !applied.Applied {
		t.Fatalf("applied = %+v", applied)
	}
	if got := readFileString(t, filepath.Join(root, localStorageRoot, "planning", "STATE.md")); got != "overlay-before" {
		t.Fatalf("overlay STATE = %q, want restored original", got)
	}
	if got := readFileString(t, filepath.Join(root, "planning", "STATE.md")); got != "decoy-before" {
		t.Fatalf("worktree decoy STATE = %q, want untouched original", got)
	}
	if got := readFileString(t, filepath.Join(root, ".git", "info", "exclude")); got != "old-exclusion" {
		t.Fatalf("exclusion store = %q, want restored original", got)
	}
	if _, statErr := os.Stat(filepath.Join(durabletx.TransactionsDir(repo), recoverFixtureID)); !os.IsNotExist(statErr) {
		t.Fatalf("mixed recovery retained the fence: %v", statErr)
	}
}
