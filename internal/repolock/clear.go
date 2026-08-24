package repolock

import (
	"errors"
	"fmt"
	"os"
)

// ErrLiveOwner reports that the lock's recorded owner is provably alive on
// this host, so `lock clear` refuses: PID and host prove liveness when they
// can, but they never prove absence, and clearing a live owner's lock is not
// an operator decision Taskrail makes for them.
var ErrLiveOwner = errors.New("repository mutation lock owner is provably live on this host")

// ErrChanged reports that the bytes at the lock path are not the exact ones
// the operator observed, so the compare-and-delete declines to act on them.
var ErrChanged = errors.New("repository mutation lock changed since it was observed")

// ClearRequest names the exact abandoned lock an operator observed through
// Inspect and authorizes removing. Neither field is a lease: together they say
// "these were the bytes I saw", and Clear still refuses a provably live
// same-host owner before deleting.
type ClearRequest struct {
	Repository Repository
	LockID     string
	// ExpectSHA256 is the raw lock-file digest the operator observed, so the
	// delete happens only against unchanged bytes.
	ExpectSHA256 string
}

// ObserveClear validates the exact observation an operator must make before
// clearing or taking over a lock. It never changes the record.
func ObserveClear(req ClearRequest) (Owner, error) {
	if err := req.Repository.validate(); err != nil {
		return Owner{}, err
	}
	if !lockIDPattern.MatchString(req.LockID) {
		return Owner{}, fmt.Errorf("lock id %q is not a lower-case 32-hex id", req.LockID)
	}
	if !digestPattern.MatchString(req.ExpectSHA256) {
		return Owner{}, fmt.Errorf("expected digest %q is not a lower-case 64-hex digest", req.ExpectSHA256)
	}

	path := LockPath(req.Repository)
	owner, digest, err := readOwner(path)
	if err != nil {
		return Owner{}, err
	}
	if owner.LockID != req.LockID {
		return Owner{}, fmt.Errorf("%w: %s is held by %s, not %s", ErrChanged, path, owner.LockID, req.LockID)
	}
	if digest != req.ExpectSHA256 {
		return Owner{}, fmt.Errorf("%w: %s now has digest %s, not the observed %s", ErrChanged, path, digest, req.ExpectSHA256)
	}
	if live, err := ownerProvablyLive(owner); err != nil {
		return Owner{}, err
	} else if live {
		return Owner{}, fmt.Errorf("%w: pid %d is running %s since %s", ErrLiveOwner, owner.PID, owner.Command, owner.StartedAt)
	}
	return owner, nil
}

// Clear removes exactly the named, unchanged lock record and reports its prior
// raw-file digest. It never touches anything else under the lock root, so
// retained transaction data survives clearing ownership. The protocol treats
// PID, host, and age as evidence, never as a lease: a provably live same-host
// owner refuses, an absent or provably dead one clears, and nothing is ever
// removed automatically.
func Clear(req ClearRequest) (string, error) {
	path := LockPath(req.Repository)
	if _, err := ObserveClear(req); err != nil {
		return "", err
	}

	// The deletion is compare-and-delete in the same shape as Release: the
	// record was just read and matched, and a record replaced between that
	// read and this remove is either removed by its own owner first (the
	// remove then reports absence as ErrChanged) or belongs to a successor
	// the digest check already declined to name.
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("%w: %s disappeared while clearing it", ErrChanged, path)
		}
		return "", fmt.Errorf("remove lock %s: %w", path, err)
	}
	return req.ExpectSHA256, nil
}

// ownerProvablyLive reports whether the recorded owner is provably running on
// this host. A different host can never be proven live from here, which is the
// point: the common directory is not a distributed lease, so cross-host
// staleness stays the operator's call.
func ownerProvablyLive(owner Owner) (bool, error) {
	if owner.PID <= 0 {
		return false, fmt.Errorf("owner pid %d is not a process id", owner.PID)
	}
	host, err := os.Hostname()
	if err != nil {
		return false, fmt.Errorf("resolve host: %w", err)
	}
	return owner.Host == host && processAlive(owner.PID), nil
}
