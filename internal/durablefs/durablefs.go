// Package durablefs provides the handle-bound filesystem operations used by
// durable repository transactions. It deliberately does not define transaction
// phases, journals, or recovery policy.
package durablefs

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tessariq/taskrail/internal/repolock"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

var (
	ErrAlias       = errors.New("filesystem name alias refused")
	ErrConflict    = errors.New("filesystem snapshot conflict")
	ErrInvalidPath = errors.New("invalid relative filesystem path")
	ErrNotRegular  = errors.New("entry is not a regular unlinked file")
	ErrUnsupported = errors.New("filesystem durability or identity is unsupported")
)

// Barrier identifies one successful durability boundary in publication order.
type Barrier string

const (
	BarrierContent   Barrier = "file-content"
	BarrierMetadata  Barrier = "file-metadata"
	BarrierDirectory Barrier = "parent-directory"
)

// Identity is native substitution evidence. Volume and File identify the entry;
// Mount is additional evidence where the platform exposes it. Identifiers can be
// reused after deletion, so callers must retain Snapshot's semantic fields too.
type Identity struct {
	Volume uint64
	File   uint64
	Mount  uint64
}

// Snapshot is the restart-portable semantic state plus native identity evidence
// observed from one open file handle.
type Snapshot struct {
	SHA256   string
	Mode     fs.FileMode
	Links    uint64
	Identity Identity
}

// MutationError reports a namespace mutation that may have committed before a
// later durability or cleanup failure. Staging is non-empty when a retained
// private alias requires deterministic transaction-level recovery.
type MutationError struct {
	Operation string
	Path      string
	Staging   string
	Committed bool
	Err       error
}

func (e *MutationError) Error() string {
	return fmt.Sprintf("%s %s (committed=%t, staging=%q): %v", e.Operation, e.Path, e.Committed, e.Staging, e.Err)
}

func (e *MutationError) Unwrap() error { return e.Err }

type ownership interface {
	Owner() repolock.Owner
	Repository() repolock.Repository
	Authorize(command string, fields ...string) error
}

// Root retains the repository root handle and the repository lock ownership
// under which mutations are permitted.
type Root struct {
	handle   *os.Root
	identity Identity
	own      ownership
	closed   bool
}

// Entry retains the exact parent directory handle used to bind a regular leaf.
// Mutations rebind the namespace and compare ancestor and leaf snapshots before
// operating through this retained handle.
type Entry struct {
	root      *Root
	path      string
	leaf      string
	parent    *os.Root
	ancestors []Identity
	snapshot  Snapshot
}

// Directory is the bound result of a directory creation.
type Directory struct {
	Path     string
	Identity Identity
}

var (
	testHookBeforeMutation func(operation, path string)
	testHookBarrier        func(Barrier) error
	testHookBeforeRootOpen func()
	testHookAfterDirOpen   func(string)
	testHookBeforeStageCAS func(*os.Root, string)
	testHookRemoveStage    func(*os.Root, string) error
	testHookBeforeLink     func(*os.Root, string)
	testHookAfterCommit    func(string, string)
	testHookEntryClose     func(*os.Root) error
)

// Open binds an absolute repository root without accepting a symlink or reparse
// point in its spelling and requires ownership of that repository's writer lock.
func Open(path string, own ownership) (*Root, error) {
	abs, err := absoluteRoot(path)
	if err != nil {
		return nil, err
	}
	if owner := own.Owner(); owner.RepositoryRoot != abs {
		return nil, fmt.Errorf("repository lock covers %s, not %s", owner.RepositoryRoot, abs)
	}
	return openRoot(abs, own)
}

// OpenAt binds a root the held lock covers but the repository tree does not
// contain. Taskrail's own runtime state lives beneath the Git common directory,
// which a linked worktree is not an ancestor of, so requiring the repository
// root would leave that state unwritable. Every no-follow, identity, and
// durability guarantee is the one Open gives; only the containment check
// differs, exactly as repository transactions exempt Git metadata paths.
func OpenAt(path string, repo repolock.Repository, own ownership) (*Root, error) {
	abs, err := absoluteRoot(path)
	if err != nil {
		return nil, err
	}
	owner := own.Owner()
	if owner.RepositoryRoot != repo.Root || owner.StorageMode != repo.Mode || owner.StorageRoot != repo.StorageRoot() {
		return nil, fmt.Errorf("repository context does not match held lock")
	}
	if own.Repository() != repo {
		return nil, fmt.Errorf("repository roots do not match held lock")
	}
	if abs != repo.StorageRoot() && abs != repo.GitCommonDir {
		return nil, fmt.Errorf("root %s is not covered by the repository context", abs)
	}
	return openRoot(abs, own)
}

func absoluteRoot(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	if filepath.Clean(abs) != abs {
		return "", fmt.Errorf("%w: non-canonical root %q", ErrInvalidPath, path)
	}
	return abs, nil
}

func openRoot(abs string, own ownership) (*Root, error) {
	if err := own.Authorize(own.Owner().Command); err != nil {
		return nil, err
	}
	nativePath := nativeRootPath(abs)
	observed, err := openRootObserved(nativePath)
	if err != nil {
		return nil, err
	}
	defer observed.Close()
	observedIdentity, _, err := nativeIdentity(observed)
	if err != nil {
		return nil, err
	}
	if testHookBeforeRootOpen != nil {
		testHookBeforeRootOpen()
	}
	handle, err := os.OpenRoot(nativePath)
	if err != nil {
		return nil, fmt.Errorf("open repository root: %w", err)
	}
	identity, _, err := directoryIdentity(handle)
	if err != nil {
		handle.Close()
		return nil, err
	}
	if identity != observedIdentity {
		handle.Close()
		return nil, fmt.Errorf("%w: repository root changed while binding", ErrConflict)
	}
	return &Root{handle: handle, identity: identity, own: own}, nil
}

func (r *Root) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return r.handle.Close()
}

func (r *Root) authorize() error {
	if r.closed {
		return fs.ErrClosed
	}
	return r.own.Authorize(r.own.Owner().Command)
}

// Snapshot returns the exact evidence captured by Bind or the last successful
// mutation.
func (e *Entry) Snapshot() Snapshot { return e.snapshot }

// Close releases the retained parent handle. A closed entry cannot be mutated;
// its value Snapshot remains usable as restart evidence.
func (e *Entry) Close() error {
	if e.parent == nil {
		return nil
	}
	err := e.parent.Close()
	if testHookEntryClose != nil {
		err = errors.Join(err, testHookEntryClose(e.parent))
	}
	e.parent = nil
	return err
}

// Bind opens a regular file through retained directory handles without accepting
// links, aliases, special entries, or a link count other than one.
func (r *Root) Bind(path string) (*Entry, error) {
	parent, leaf, ancestors, err := r.bindParent(path)
	if err != nil {
		return nil, err
	}
	snapshot, err := observeFile(parent, leaf)
	if err != nil {
		parent.Close()
		return nil, err
	}
	return &Entry{root: r, path: path, leaf: leaf, parent: parent, ancestors: ancestors, snapshot: snapshot}, nil
}

// Rebind compares both semantic snapshot fields and identity evidence. Recovery
// policy may use semantic fields as its oracle; this primitive reports any
// identity disagreement as an additional substitution signal.
func (r *Root) Rebind(path string, expected Snapshot) (*Entry, error) {
	entry, err := r.Bind(path)
	if err != nil {
		return nil, err
	}
	if entry.snapshot != expected {
		entry.parent.Close()
		return nil, fmt.Errorf("%w: %s changed across restart", ErrConflict, path)
	}
	return entry, nil
}

func (r *Root) bindParent(path string) (*os.Root, string, []Identity, error) {
	parts, err := splitPath(path)
	if err != nil {
		return nil, "", nil, err
	}
	current, err := r.handle.OpenRoot(".")
	if err != nil {
		return nil, "", nil, err
	}
	ancestors := []Identity{r.identity}
	for _, part := range parts[:len(parts)-1] {
		if err := exactName(current, part, true); err != nil {
			current.Close()
			return nil, "", nil, err
		}
		info, err := current.Lstat(part)
		if err != nil {
			current.Close()
			return nil, "", nil, err
		}
		if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
			current.Close()
			return nil, "", nil, fmt.Errorf("%w: ancestor %q is not a plain directory", ErrNotRegular, part)
		}
		next, err := current.OpenRoot(part)
		if err != nil {
			current.Close()
			return nil, "", nil, err
		}
		if testHookAfterDirOpen != nil {
			testHookAfterDirOpen(part)
		}
		observed, err := openObserved(current, part, true)
		if err != nil {
			next.Close()
			current.Close()
			return nil, "", nil, err
		}
		identity, _, err := nativeIdentity(observed)
		observed.Close()
		if err != nil {
			next.Close()
			current.Close()
			return nil, "", nil, err
		}
		retainedIdentity, _, err := directoryIdentity(next)
		if err != nil || retainedIdentity != identity {
			next.Close()
			current.Close()
			return nil, "", nil, fmt.Errorf("%w: retained ancestor %q has a different identity", ErrConflict, part)
		}
		// Re-observe the name after opening it. This is not an atomic exclusion
		// against a hostile external actor, but any substitution present at this
		// operation boundary is refused before the handle becomes authoritative.
		if err := exactName(current, part, true); err != nil {
			next.Close()
			current.Close()
			return nil, "", nil, err
		}
		after, err := current.Lstat(part)
		if err != nil || !after.IsDir() || after.Mode()&fs.ModeSymlink != 0 {
			next.Close()
			current.Close()
			return nil, "", nil, fmt.Errorf("%w: ancestor %q changed while binding", ErrConflict, part)
		}
		confirm, err := current.OpenRoot(part)
		if err != nil {
			next.Close()
			current.Close()
			return nil, "", nil, err
		}
		confirmedIdentity, _, confirmErr := directoryIdentity(confirm)
		confirm.Close()
		if confirmErr != nil || confirmedIdentity != identity {
			next.Close()
			current.Close()
			return nil, "", nil, fmt.Errorf("%w: ancestor %q identity changed while binding", ErrConflict, part)
		}
		current.Close()
		current = next
		ancestors = append(ancestors, identity)
	}
	return current, parts[len(parts)-1], ancestors, nil
}

func splitPath(path string) ([]string, error) {
	if path == "" || strings.Contains(path, `\`) || filepath.IsAbs(path) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidPath, path)
	}
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, fmt.Errorf("%w: %q", ErrInvalidPath, path)
		}
	}
	return parts, nil
}

func exactName(parent *os.Root, name string, required bool) error {
	dir, err := parent.Open(".")
	if err != nil {
		return err
	}
	entries, readErr := dir.ReadDir(-1)
	closeErr := dir.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	wanted := aliasKey(name)
	found := false
	for _, entry := range entries {
		if aliasKey(entry.Name()) != wanted {
			continue
		}
		if entry.Name() != name || found {
			return fmt.Errorf("%w: %q collides with %q", ErrAlias, name, entry.Name())
		}
		found = true
	}
	if required && !found {
		return &fs.PathError{Op: "bind", Path: name, Err: fs.ErrNotExist}
	}
	if !required && found {
		return &fs.PathError{Op: "publish", Path: name, Err: fs.ErrExist}
	}
	return nil
}

func aliasKey(name string) string {
	return cases.Fold().String(norm.NFC.String(name))
}

func observeFile(parent *os.Root, leaf string) (Snapshot, error) {
	if err := exactName(parent, leaf, true); err != nil {
		return Snapshot{}, err
	}
	info, err := parent.Lstat(leaf)
	if err != nil {
		return Snapshot{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0 {
		return Snapshot{}, fmt.Errorf("%w: %q is not a plain regular file", ErrNotRegular, leaf)
	}
	file, err := openObserved(parent, leaf, false)
	if err != nil {
		return Snapshot{}, err
	}
	defer file.Close()
	identity, links, err := nativeIdentity(file)
	if err != nil {
		return Snapshot{}, err
	}
	if links != 1 {
		return Snapshot{}, fmt.Errorf("%w: %q has %d links", ErrNotRegular, leaf, links)
	}
	opened, err := file.Stat()
	if err != nil {
		return Snapshot{}, err
	}
	if !opened.Mode().IsRegular() {
		return Snapshot{}, fmt.Errorf("%w: %q changed type while binding", ErrConflict, leaf)
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return Snapshot{}, err
	}
	afterIdentity, afterLinks, err := nativeIdentity(file)
	if err != nil {
		return Snapshot{}, err
	}
	after, err := file.Stat()
	if err != nil {
		return Snapshot{}, err
	}
	if identity != afterIdentity || links != afterLinks || opened.Mode() != after.Mode() {
		return Snapshot{}, fmt.Errorf("%w: %q changed while binding", ErrConflict, leaf)
	}
	latest, err := parent.Lstat(leaf)
	if err != nil || !latest.Mode().IsRegular() || latest.Mode()&fs.ModeSymlink != 0 {
		return Snapshot{}, fmt.Errorf("%w: %q changed after reading", ErrConflict, leaf)
	}
	confirm, err := openObserved(parent, leaf, false)
	if err != nil {
		return Snapshot{}, err
	}
	confirmedIdentity, confirmedLinks, confirmErr := nativeIdentity(confirm)
	confirm.Close()
	if confirmErr != nil || confirmedIdentity != identity || confirmedLinks != links {
		return Snapshot{}, fmt.Errorf("%w: %q identity changed after reading", ErrConflict, leaf)
	}
	return Snapshot{SHA256: hex.EncodeToString(digest.Sum(nil)), Mode: stableMode(opened.Mode()), Links: links, Identity: identity}, nil
}

func directoryIdentity(root *os.Root) (Identity, uint64, error) {
	file, err := root.Open(".")
	if err != nil {
		return Identity{}, 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Identity{}, 0, err
	}
	if !info.IsDir() {
		return Identity{}, 0, fmt.Errorf("%w: bound ancestor is not a directory", ErrNotRegular)
	}
	return nativeIdentity(file)
}

func stableMode(mode fs.FileMode) fs.FileMode {
	if runtime.GOOS == "windows" {
		if mode&0o222 == 0 {
			return 0o444
		}
		return 0o666
	}
	return mode & (fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky)
}

func nativeRootPath(path string) string {
	if runtime.GOOS == "darwin" && (path == "/var" || strings.HasPrefix(path, "/var/")) {
		return "/private" + path
	}
	return path
}

func randomName() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return ".taskrail-durable-" + hex.EncodeToString(value[:]), nil
}

func barrier(step Barrier, sync func() error) error {
	if err := sync(); err != nil {
		return classifySyncError(step, err)
	}
	if testHookBarrier != nil {
		if err := testHookBarrier(step); err != nil {
			return fmt.Errorf("%s barrier: %w", step, err)
		}
	}
	return nil
}

func syncDirectory(parent *os.Root) error {
	dir, err := parent.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	return nativeSync(dir)
}

func sameIdentities(a, b []Identity) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func runMutationHook(operation, path string) {
	if testHookBeforeMutation != nil {
		testHookBeforeMutation(operation, path)
	}
}
