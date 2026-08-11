package repotx

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/tessariq/taskrail/internal/repolock"
)

// Ownership is the held mutation lock a transaction publishes under. An owner's
// own lock and a delegate's join both satisfy it; IsDelegate is what separates
// them, because a delegate has to arrive already bound to one task and one write
// set while a direct operator's command bounds itself.
type Ownership interface {
	Owner() repolock.Owner
	Capability() repolock.Capability
	Authorize(command string, fields ...string) error
	IsDelegate() bool
}

// Result is a committed transaction. Its snapshots describe the bytes actually
// published, so a caller reports what happened rather than what it intended.
type Result struct {
	Snapshots []Snapshot
}

// testHookAfterPublish runs after each published path lands, and
// testHookBeforeMkdir just before one path's directories are created. Both are
// nil outside tests, which use them to change the repository mid-transaction —
// the only way to reach the races the contract promises to lose safely.
var (
	testHookAfterPublish func(Path)
	testHookBeforeMkdir  func(Path)
)

// Commit runs one normal transaction to completion. It authorizes the write
// against own's capability, snapshots the complete consumed and published set,
// validates the complete candidate, compare-and-swaps against the snapshot, and
// only then replaces each published file. Any handled failure after the first
// write rolls back every path still holding this transaction's candidate bytes.
//
// A request the transaction could not have produced — a malformed path, a
// duplicate, an empty write set — is an ordinary error rather than an *Error:
// it is a defect in the calling writer, not an outcome an agent can act on.
func Commit(ctx context.Context, own Ownership, req Request) (Result, error) {
	if err := req.validate(); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := authorize(own, req); err != nil {
		return Result{}, err
	}
	root, err := resolveRoot(own.Owner().RepositoryRoot)
	if err != nil {
		return Result{}, err
	}

	entries := plan(req)
	if err := snapshot(entries); err != nil {
		return Result{}, err
	}
	if req.Validate != nil {
		preview := evidence(entries)
		if err := req.Validate(preview); err != nil {
			return Result{}, failure(KindValidation, preview, err)
		}
	}
	if err := recheck(entries); err != nil {
		return Result{}, err
	}
	if err := publish(ctx, root, entries); err != nil {
		return Result{}, rollback(root, entries, err)
	}
	return Result{Snapshots: evidence(entries)}, nil
}

// authorize refuses every write outside the ownership's bound before anything is
// read. A delegate must additionally arrive narrowed: an unbounded delegated
// capability would let a child pick its own task or its own write set, which is
// the widening the protocol exists to prevent.
func authorize(own Ownership, req Request) error {
	capability := own.Capability()
	if own.IsDelegate() {
		if strings.TrimSpace(capability.SelectedTask) == "" {
			return failure(KindRefused, nil,
				fmt.Errorf("%w: delegated %s names no selected task", repolock.ErrRefused, req.Command))
		}
		if len(capability.Writes) == 0 {
			return failure(KindRefused, nil,
				fmt.Errorf("%w: delegated %s names no write set", repolock.ErrRefused, req.Command))
		}
	}
	if err := own.Authorize(req.Command, req.TaskFields...); err != nil {
		return failure(KindRefused, nil, err)
	}
	if err := capability.AllowsTask(req.SelectedTask); err != nil {
		return failure(KindRefused, nil, err)
	}
	if err := capability.AllowsWrites(req.publishedPaths()); err != nil {
		return failure(KindRefused, nil, err)
	}
	return contained(own.Owner(), req)
}

// contained keeps every semantic path inside the repository the lock covers. Git
// metadata is exempt because its canonical location is legitimately outside a
// linked worktree, and the lock file itself already lives there.
func contained(owner repolock.Owner, req Request) error {
	for _, path := range req.paths() {
		if path.Kind == Git {
			continue
		}
		if !inside(owner.RepositoryRoot, path.Physical) {
			return failure(KindOutsideRepository, nil, fmt.Errorf(
				"%s path %q resolves to %s, outside repository %s",
				path.Kind, path.Reported, path.Physical, owner.RepositoryRoot))
		}
	}
	return nil
}

// entry is one path's whole transaction life: what it was, what it should
// become, and what it is now.
type entry struct {
	path      Path
	candidate *Candidate
	original  *fileState
	current   *string
	published bool
}

type fileState struct {
	content []byte
	mode    fs.FileMode
	digest  string
}

// plan puts the complete set in the deterministic order the machine contract
// reports evidence in — path kind, then path — so publication, rollback, and
// diagnostics all follow one sequence and two runs of the same failure produce
// byte-identical output. Ordering is established here and nowhere else.
func plan(req Request) []*entry {
	entries := make([]*entry, 0, len(req.Consumed)+len(req.Published))
	for _, path := range req.Consumed {
		entries = append(entries, &entry{path: path})
	}
	for _, candidate := range req.Published {
		entries = append(entries, &entry{path: candidate.Path, candidate: &candidate})
	}
	slices.SortFunc(entries, func(a, b *entry) int {
		if kinds := strings.Compare(string(a.path.Kind), string(b.path.Kind)); kinds != 0 {
			return kinds
		}
		return strings.Compare(a.path.Reported, b.path.Reported)
	})
	return entries
}

func snapshot(entries []*entry) error {
	for _, e := range entries {
		state, err := observe(e)
		if err != nil {
			return withEvidence(err, entries)
		}
		e.original = state
	}
	return nil
}

// recheck is the whole-set compare-and-swap. Validation can take arbitrary time,
// so the snapshot the candidate was built from is proven current immediately
// before the first write; anything else is reported as a conflict and never
// overwritten. It covers consumed paths too, which publication never revisits.
func recheck(entries []*entry) error {
	changed := make([]string, 0)
	for _, e := range entries {
		state, err := observe(e)
		if err != nil {
			return withEvidence(err, entries)
		}
		if !sameState(e.original, state) {
			changed = append(changed, e.path.Reported)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	return failure(KindConflict, evidence(entries),
		fmt.Errorf("paths changed since the transaction snapshot: %s", strings.Join(changed, ", ")))
}

// publish replaces each candidate in plan order, re-proving that path's snapshot
// immediately before its own write. The whole-set recheck cannot cover this: a
// multi-file transaction writes over a span of time, and an edit landing after
// the recheck but before this file's turn would otherwise be overwritten with no
// trace. A mismatch aborts, and the caller rolls back what already landed.
func publish(ctx context.Context, root string, entries []*entry) error {
	for _, e := range entries {
		if e.candidate == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		state, err := observe(e)
		if err != nil {
			return withEvidence(err, entries)
		}
		if !sameState(e.original, state) {
			return failure(KindConflict, evidence(entries),
				fmt.Errorf("%s changed between the recheck and its publication", e.path.Reported))
		}
		if err := publishTo(root, e.path, e.candidate.Content, publishMode(e)); err != nil {
			return err
		}
		e.published = true
		e.current = pointer(digestOf(e.candidate.Content))
		if testHookAfterPublish != nil {
			testHookAfterPublish(e.path)
		}
	}
	return nil
}

// publishMode keeps an existing file's mode and gives a new one 0644, so
// publication never silently changes a file's permissions and never invents a
// mode a repository did not already use.
func publishMode(e *entry) fs.FileMode {
	if e.original != nil {
		return e.original.mode
	}
	return 0o644
}

// rollback undoes publication in reverse, one compare-and-swap at a time. A path
// still holding this transaction's candidate is restored to its original bytes,
// or removed when there were none. A path holding anything else belongs to
// whoever wrote it: it is preserved, named, and reported as a rollback failure
// rather than overwritten.
//
// A directory publication had to create stays. It holds no semantic state, and
// removing directories a concurrent writer may already be using would trade a
// harmless leftover for a real race.
func rollback(root string, entries []*entry, cause error) *Error {
	preserved := make([]string, 0)
	problems := []error{cause}
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if !e.published {
			continue
		}
		state, err := observe(e)
		if err != nil {
			// A path this transaction cannot re-read is a path it cannot prove it
			// restored, so it counts as preserved. Reporting a clean rollback here
			// would claim original bytes nobody looked at.
			preserved = append(preserved, e.path.Reported)
			problems = append(problems, err)
			continue
		}
		if state == nil || state.digest != digestOf(e.candidate.Content) {
			preserved = append(preserved, e.path.Reported)
			continue
		}
		if err := restore(root, e); err != nil {
			preserved = append(preserved, e.path.Reported)
			problems = append(problems, err)
			continue
		}
		e.current = digestPointer(e.original)
	}
	slices.Sort(preserved)
	if len(preserved) == 0 {
		// A cause the transaction already classified keeps that classification once
		// the repository is back to its original bytes: "an external edit blocked
		// this write" or "that path escapes the repository" is what the caller has
		// to report, not the generic "something failed and was undone".
		var classified *Error
		if errors.As(cause, &classified) {
			// Refresh its evidence rather than wrap it again: rollback moved bytes
			// after the cause was raised, and re-wrapping would repeat the kind in
			// the message without adding anything.
			classified.snapshots = evidence(entries)
			return classified
		}
		return failure(KindRolledBack, evidence(entries), cause)
	}
	rollbackErr := failure(KindRollbackFailed, evidence(entries), errors.Join(problems...))
	rollbackErr.Preserved = preserved
	return rollbackErr
}

// restore puts one published path back. It re-proves the resolved parent
// directory first, because a rollback writing through a link planted since
// publication would escape exactly the boundary publication defends.
func restore(root string, e *entry) error {
	if e.original == nil {
		if err := os.Remove(e.path.Physical); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("roll back %s: %w", e.path.Reported, err)
		}
		return nil
	}
	if err := publishTo(root, e.path, e.original.content, e.original.mode); err != nil {
		return fmt.Errorf("roll back %s: %w", e.path.Reported, err)
	}
	return nil
}

// evidence renders the current per-path observation as snapshots. It emits them
// in plan order, which is already the contract's order.
func evidence(entries []*entry) []Snapshot {
	snapshots := make([]Snapshot, 0, len(entries))
	for _, e := range entries {
		snapshot := Snapshot{
			Kind:           e.path.Kind,
			Path:           e.path.Reported,
			OriginalSHA256: digestPointer(e.original),
			CurrentSHA256:  e.current,
		}
		if e.candidate != nil {
			snapshot.CandidateSHA256 = pointer(digestOf(e.candidate.Content))
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func digestPointer(state *fileState) *string {
	if state == nil {
		return nil
	}
	return pointer(state.digest)
}

func pointer(value string) *string { return &value }
