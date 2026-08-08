package repolock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// held tracks the lock paths this process owns. The filesystem already makes a
// second acquisition impossible, but it cannot tell contention from misuse:
// without this, a writer that forgot it holds the lock would read "another owner
// holds it" about itself and wait for a release only it can perform.
var held = struct {
	sync.Mutex
	paths map[string]struct{}
}{paths: make(map[string]struct{})}

func claimInProcess(path string) error {
	held.Lock()
	defer held.Unlock()
	if _, ok := held.paths[path]; ok {
		return fmt.Errorf("%w: %s", ErrSameProcess, path)
	}
	held.paths[path] = struct{}{}
	return nil
}

func releaseInProcess(path string) {
	held.Lock()
	defer held.Unlock()
	delete(held.paths, path)
}

// Request describes one writer's claim on a repository.
type Request struct {
	Repository Repository
	// Command is the canonical command taking the lock. It must be within
	// Capability, so an owner cannot record authority it did not declare.
	Command string
	// TransactionID names the transaction this lock covers, when there is one.
	TransactionID string
	// Capability bounds what this owner may mutate while it holds the lock.
	Capability Capability
	// ExecutablePath, when set, acquires the lock for delegation: its digest is
	// recorded and Acquire mints a one-off token children present to join.
	ExecutablePath string
}

// Lock is one acquired ownership of a repository's mutation lock. It is not safe
// for concurrent use by multiple goroutines; a transaction owns exactly one.
type Lock struct {
	path       string
	owner      Owner
	capability Capability
	delegation *Delegation
	released   bool
}

// Status is a read-only observation of a repository's mutation lock. Inspecting
// takes no lock and writes nothing, because read-only callers never need one.
type Status struct {
	Held   bool
	SHA256 string
	Owner  *Owner
}

// Acquire takes the repository mutation lock, refusing rather than waiting when
// another owner holds it. Acquisition is interruption-aware: a cancelled context
// refuses before the claim and removes its own record if cancellation lands
// after it, so an interrupted caller never leaves a lock nobody can release.
func Acquire(ctx context.Context, req Request) (*Lock, error) {
	if err := validateClaim(req.Repository, req.Command, req.Capability); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	owner, delegation, err := newOwner(req)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(owner)
	if err != nil {
		return nil, fmt.Errorf("encode lock metadata: %w", err)
	}

	path := LockPath(req.Repository)
	if err := claimInProcess(path); err != nil {
		return nil, err
	}
	if err := writeLock(ctx, path, data); err != nil {
		releaseInProcess(path)
		return nil, err
	}
	return &Lock{
		path:       path,
		owner:      owner,
		capability: req.Capability.normalized(),
		delegation: delegation,
	}, nil
}

// writeLock publishes the lock record. The metadata is written and flushed to a
// private staging file first and only then linked into place, so the lock path
// is never observably half-written: a concurrent reader sees either no lock or
// the complete record. Linking is also the exclusive claim — it fails when the
// path already exists — so publication and claiming are the same atomic step.
//
// A filesystem without hard links (a network share, FAT) falls back to claiming
// the path directly and filling it in. That reintroduces a brief window where a
// reader can see an empty file, which is a refusal either way, and is preferable
// to refusing to lock at all there.
func writeLock(ctx context.Context, path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create lock directory %s: %w", filepath.Dir(path), err)
	}
	staged, err := stageLock(path, data)
	if err != nil {
		return err
	}
	if err := publishLock(ctx, staged, path, data); err != nil {
		return errors.Join(err, removeLock(staged))
	}
	// Publication succeeded, so the lock is real and only the handle this call is
	// about to return can release it. A staging sibling that will not delete is
	// inert debris under a private name; reporting it would fail an acquisition
	// that did happen and strand the very lock just published.
	_ = os.Remove(staged)
	return nil
}

// publishLock links the staged record into place. A link failure that is not
// "already exists" means this filesystem will not publish by linking, so the
// path is claimed directly instead; that fallback is exclusive in its own right,
// so misreading a transient link error can never publish over another owner. The
// original link failure is carried into any fallback failure, because "the
// fallback also failed" without it hides why the fallback ran at all.
func publishLock(ctx context.Context, staged, path string, data []byte) error {
	if err := os.Link(staged, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return heldError(path)
		}
		if fallback := claimAndFill(path, data); fallback != nil {
			return errors.Join(fmt.Errorf("publish lock %s by link: %w", path, err), fallback)
		}
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(err, removeLock(path))
	}
	return nil
}

// stageLock writes data to a private sibling of path and returns its name, so
// publication has complete, already-flushed bytes to link into place. The staged
// file carries the mode the published lock must end up with: publication is a
// hard link, so both names share one inode and one permission set, and an
// owner-only lock would be unreadable to another user's `lock status`.
func stageLock(path string, data []byte) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".staging-*")
	if err != nil {
		return "", fmt.Errorf("stage lock %s: %w", path, err)
	}
	if err := stageContent(file, data); err != nil {
		return "", errors.Join(err, removeLock(file.Name()))
	}
	return file.Name(), nil
}

func stageContent(file *os.File, data []byte) error {
	if err := writeAndSync(file, data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close staged lock %s: %w", file.Name(), err)
	}
	if err := os.Chmod(file.Name(), lockFileMode); err != nil {
		return fmt.Errorf("set staged lock mode %s: %w", file.Name(), err)
	}
	return nil
}

// claimAndFill is the link-free fallback: create the lock path exclusively, then
// write into it, removing the record this call created if anything fails.
func claimAndFill(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, lockFileMode)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return heldError(path)
		}
		return fmt.Errorf("create lock %s: %w", path, err)
	}
	if err := writeAndSync(file, data); err != nil {
		file.Close()
		return errors.Join(err, removeLock(path))
	}
	if err := file.Close(); err != nil {
		return errors.Join(fmt.Errorf("close lock %s: %w", path, err), removeLock(path))
	}
	// O_CREATE masks its mode argument through the process umask, so a restrictive
	// umask would publish an owner-only lock here while the linking path publishes
	// 0644. Set the mode explicitly so both paths agree whatever the umask is.
	if err := os.Chmod(path, lockFileMode); err != nil {
		return errors.Join(fmt.Errorf("set lock mode %s: %w", path, err), removeLock(path))
	}
	return nil
}

// removeLock reports a cleanup failure instead of discarding it: a record this
// process could not withdraw is a lock an operator will later have to explain.
func removeLock(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove lock %s: %w", path, err)
	}
	return nil
}

func writeAndSync(file *os.File, data []byte) error {
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write lock %s: %w", file.Name(), err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync lock %s: %w", file.Name(), err)
	}
	return nil
}

// heldError names the existing owner when it can be read. Unreadable or invalid
// metadata is reported as both held and malformed: the caller must refuse either
// way, and the distinction is what tells an operator whether to inspect the
// record or look for the live owner. An owner that released between the failed
// claim and this read is still reported as held and never as ErrNotHeld — the
// claim did fail, and a caller testing for "free now" must not read this as one.
func heldError(path string) error {
	owner, _, err := readOwner(path)
	switch {
	case errors.Is(err, ErrNotHeld):
		return fmt.Errorf("%w: %s was claimed and released while acquiring", ErrHeld, path)
	case errors.Is(err, ErrMalformed):
		return fmt.Errorf("%w: %s carries unusable metadata: %w", ErrHeld, path, err)
	case err != nil:
		return fmt.Errorf("%w: %s: %w", ErrHeld, path, err)
	}
	return fmt.Errorf("%w by %s (pid %d on %s) running %s since %s",
		ErrHeld, owner.LockID, owner.PID, owner.Host, owner.Command, owner.StartedAt)
}

func newOwner(req Request) (Owner, *Delegation, error) {
	lockID, err := randomHex(16)
	if err != nil {
		return Owner{}, nil, err
	}
	host, err := os.Hostname()
	if err != nil {
		return Owner{}, nil, fmt.Errorf("resolve host: %w", err)
	}
	owner := Owner{
		LockID:         lockID,
		Command:        req.Command,
		PID:            os.Getpid(),
		Host:           host,
		StartedAt:      time.Now().UTC().Format(time.RFC3339),
		RepositoryRoot: req.Repository.Root,
		StorageMode:    req.Repository.Mode,
		StorageRoot:    req.Repository.StorageRoot(),
	}
	if req.TransactionID != "" {
		owner.TransactionID = &req.TransactionID
	}
	if req.ExecutablePath == "" {
		return owner, nil, nil
	}

	executable, err := ExecutableDigest(req.ExecutablePath)
	if err != nil {
		return Owner{}, nil, err
	}
	token, err := randomHex(32)
	if err != nil {
		return Owner{}, nil, err
	}
	digest := sha256Hex([]byte(token))
	owner.ExecutableSHA256 = &executable
	owner.DelegationDigest = &digest
	return owner, &Delegation{Token: token, ExecutableSHA256: executable}, nil
}

// Owner reports the metadata this lock published.
func (l *Lock) Owner() Owner { return l.owner }

// Capability reports the bound this ownership was acquired under.
func (l *Lock) Capability() Capability { return l.capability }

// Authorize refuses a command or task field outside this ownership's capability.
// Writers call it before mutating anything.
func (l *Lock) Authorize(command string, fields ...string) error {
	if l.released {
		return fmt.Errorf("%w: %s", ErrReleased, l.path)
	}
	return l.capability.Allows(command, fields...)
}

// Delegation returns the secret a child writer presents to join this lock,
// available only when the lock was acquired for delegation.
func (l *Lock) Delegation() (Delegation, error) {
	if l.released {
		return Delegation{}, fmt.Errorf("%w: %s", ErrReleased, l.path)
	}
	if l.delegation == nil {
		return Delegation{}, fmt.Errorf("%w: %s", ErrNotDelegated, l.path)
	}
	return *l.delegation, nil
}

// Release removes this ownership's record. It is compare-and-delete: a lock
// record that is no longer this handle's belongs to someone else and is left
// alone, so a release can never delete a successor's claim.
//
// Release is deliberately not cancellable. An interrupted acquisition withdraws
// its own record, but abandoning a release would strand a lock nothing can
// clear — the opposite of what interruption-awareness is for here.
//
// A handle is spent once it can no longer act: after a successful delete, and
// after finding the record gone or owned by someone else. A delete that fails
// leaves the handle live, because the lock is still ours and still there.
func (l *Lock) Release() error {
	if l.released {
		return fmt.Errorf("%w: %s", ErrReleased, l.path)
	}
	current, _, err := readOwner(l.path)
	if err != nil {
		l.spend()
		if errors.Is(err, ErrNotHeld) {
			return fmt.Errorf("%w: %s disappeared before release", ErrNotHeld, l.path)
		}
		return fmt.Errorf("release %s: %w", l.path, err)
	}
	if current.LockID != l.owner.LockID {
		l.spend()
		return fmt.Errorf("refusing to release %s: it is now held by %s, not %s",
			l.path, current.LockID, l.owner.LockID)
	}
	if err := os.Remove(l.path); err != nil {
		return fmt.Errorf("remove lock %s: %w", l.path, err)
	}
	l.spend()
	return nil
}

// spend retires the handle and the in-process claim together, so the two can
// never disagree about whether this process still holds the lock.
func (l *Lock) spend() {
	l.released = true
	releaseInProcess(l.path)
}

// Inspect reports the current lock without taking it or writing anything.
func Inspect(repo Repository) (Status, error) {
	if err := repo.validate(); err != nil {
		return Status{}, err
	}
	path := LockPath(repo)
	owner, digest, err := readOwner(path)
	if err != nil {
		if errors.Is(err, ErrNotHeld) {
			return Status{}, nil
		}
		return Status{}, err
	}
	return Status{Held: true, SHA256: digest, Owner: &owner}, nil
}

// readOwner reads and validates the lock record, also returning the raw file's
// digest so a caller can compare-and-swap against exactly these bytes. An absent
// file is ErrNotHeld; unreadable or invalid content is ErrMalformed and is never
// repaired or removed here.
func readOwner(path string) (Owner, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Owner{}, "", fmt.Errorf("%w: %s", ErrNotHeld, path)
		}
		return Owner{}, "", fmt.Errorf("read lock %s: %w", path, err)
	}
	var owner Owner
	if err := json.Unmarshal(data, &owner); err != nil {
		return Owner{}, "", fmt.Errorf("%w: %s: %w", ErrMalformed, path, err)
	}
	if err := owner.validate(); err != nil {
		return Owner{}, "", fmt.Errorf("%w: %s: %w", ErrMalformed, path, err)
	}
	return owner, sha256Hex(data), nil
}
