package repolock

import (
	"crypto/subtle"
	"fmt"
)

// Delegation is the secret an owner hands to the child writers it launches. The
// token exists only in the owner's memory and the child's environment; the lock
// file records its digest, so possession — not readability — is what authorizes
// a join.
type Delegation struct {
	Token            string
	ExecutableSHA256 string
}

// JoinRequest is a child writer's claim to already-held ownership. It proves the
// same repository, the same executable bytes, and the delegation token, and it
// declares the bound it intends to work within.
type JoinRequest struct {
	Repository       Repository
	Command          string
	Token            string
	ExecutableSHA256 string
	Capability       Capability
}

// Joined is delegated ownership of a lock someone else holds. It carries
// authority but no claim: it never releases the lock, because the owner holds it
// across the child's whole lifetime.
type Joined struct {
	owner      Owner
	capability Capability
}

// Join attaches to the repository's existing lock as a delegate. It writes
// nothing — the owner keeps the lock — and refuses unless the caller matches the
// owner's repository and storage identity, presents the delegation token and the
// owner's executable digest, and requests a capability within the fixed
// delegated bound. Every refusal happens before any mutation, so an unrelated or
// over-broad writer never reaches the repository.
func Join(req JoinRequest) (*Joined, error) {
	if err := validateClaim(req.Repository, req.Command, req.Capability); err != nil {
		return nil, err
	}

	path := LockPath(req.Repository)
	owner, _, err := readOwner(path)
	if err != nil {
		return nil, err
	}
	if err := matchesOwner(req, owner); err != nil {
		return nil, err
	}
	// The delegated bound is a protocol constant, so a child is limited to the
	// lifecycle write set whatever its parent declared for itself.
	capability, err := DelegatedCapability().Narrow(req.Capability)
	if err != nil {
		return nil, err
	}
	return &Joined{owner: owner, capability: capability}, nil
}

// matchesOwner checks the joining identity against the lock record. Repository
// and storage identity come first so a mixed-mode caller is turned away before
// any secret is compared at all.
func matchesOwner(req JoinRequest, owner Owner) error {
	if owner.RepositoryRoot != req.Repository.Root {
		return fmt.Errorf("%w: lock belongs to repository %s, not %s",
			ErrRefused, owner.RepositoryRoot, req.Repository.Root)
	}
	if owner.StorageMode != req.Repository.Mode {
		return fmt.Errorf("%w: lock is %s storage, not %s",
			ErrRefused, owner.StorageMode, req.Repository.Mode)
	}
	if owner.StorageRoot != req.Repository.StorageRoot() {
		return fmt.Errorf("%w: lock storage root is %s, not %s",
			ErrRefused, owner.StorageRoot, req.Repository.StorageRoot())
	}
	if owner.DelegationDigest == nil || owner.ExecutableSHA256 == nil {
		return fmt.Errorf("%w: lock %s was not acquired for delegation", ErrRefused, owner.LockID)
	}
	if subtle.ConstantTimeCompare([]byte(sha256Hex([]byte(req.Token))), []byte(*owner.DelegationDigest)) != 1 {
		return fmt.Errorf("%w: delegation token does not match lock %s", ErrRefused, owner.LockID)
	}
	if subtle.ConstantTimeCompare([]byte(req.ExecutableSHA256), []byte(*owner.ExecutableSHA256)) != 1 {
		return fmt.Errorf("%w: executable identity does not match lock %s", ErrRefused, owner.LockID)
	}
	return nil
}

// Owner reports the metadata of the lock this delegate joined.
func (j *Joined) Owner() Owner { return j.owner }

// Capability reports the bound this delegated ownership works within.
func (j *Joined) Capability() Capability { return j.capability }

// IsDelegate reports that this ownership was joined rather than claimed, so a
// writer can require the narrower bounds a delegate must arrive with.
func (j *Joined) IsDelegate() bool { return true }

// Authorize refuses a command or task field outside the delegated bound, before
// the caller mutates anything.
func (j *Joined) Authorize(command string, fields ...string) error {
	return j.capability.Allows(command, fields...)
}

// Narrow returns delegated ownership restricted further. It refuses any request
// that adds a command or task field, so delegated authority only ever shrinks.
func (j *Joined) Narrow(requested Capability) (*Joined, error) {
	capability, err := j.capability.Narrow(requested)
	if err != nil {
		return nil, err
	}
	return &Joined{owner: j.owner, capability: capability}, nil
}
