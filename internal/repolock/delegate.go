package repolock

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Delegation is the secret an owner hands to the child writers it launches. The
// token exists only in the owner's memory and the child's environment; the lock
// file records a digest authenticating the token and Grant, so possession of the
// matching out-of-band values — not metadata readability — authorizes a join.
type Delegation struct {
	Token            string
	ExecutableSHA256 string
	Grant            Capability
}

// JoinRequest is a child writer's claim to already-held ownership. It proves the
// same repository, executable bytes, and owner grant, then declares the narrower
// capability it intends to work within.
type JoinRequest struct {
	Repository       Repository
	Command          string
	Token            string
	ExecutableSHA256 string
	// Grant is the owner-declared task and write set authenticated by the lock.
	Grant      Capability
	Capability Capability
}

// Joined is delegated ownership of a lock someone else holds. It carries
// authority but no claim: it never releases the lock, because the owner holds it
// across the child's whole lifetime.
type Joined struct {
	repository Repository
	owner      Owner
	capability Capability
}

// Join attaches to the repository's existing lock as a delegate. It writes
// nothing — the owner keeps the lock — and refuses unless the caller matches the
// owner's repository and storage identity, presents the authenticated grant and
// executable digest, and requests a capability within that grant and the fixed
// delegated command/field bound. Every refusal happens before any mutation, so
// an unrelated or over-broad writer never reaches the repository.
func Join(req JoinRequest) (*Joined, error) {
	if err := validateClaim(req.Repository, req.Command, req.Capability); err != nil {
		return nil, err
	}

	path := LockPath(req.Repository)
	owner, _, err := readOwner(path)
	if err != nil {
		return nil, err
	}
	grant, err := matchesOwner(req, owner)
	if err != nil {
		return nil, err
	}
	capability, err := grant.Narrow(req.Capability)
	if err != nil {
		return nil, err
	}
	return &Joined{repository: req.Repository, owner: owner, capability: capability}, nil
}

// matchesOwner checks the joining identity against the lock record. Repository
// and storage identity come first so a mixed-mode caller is turned away before
// any secret is compared at all.
func matchesOwner(req JoinRequest, owner Owner) (Capability, error) {
	if owner.RepositoryRoot != req.Repository.Root {
		return Capability{}, fmt.Errorf("%w: lock belongs to repository %s, not %s",
			ErrRefused, owner.RepositoryRoot, req.Repository.Root)
	}
	if owner.StorageMode != req.Repository.Mode {
		return Capability{}, fmt.Errorf("%w: lock is %s storage, not %s",
			ErrRefused, owner.StorageMode, req.Repository.Mode)
	}
	if owner.StorageRoot != req.Repository.StorageRoot() {
		return Capability{}, fmt.Errorf("%w: lock storage root is %s, not %s",
			ErrRefused, owner.StorageRoot, req.Repository.StorageRoot())
	}
	if owner.DelegationDigest == nil || owner.ExecutableSHA256 == nil {
		return Capability{}, fmt.Errorf("%w: lock %s was not acquired for delegation", ErrRefused, owner.LockID)
	}
	grant, err := delegationGrant(req.Grant)
	if err != nil {
		return Capability{}, fmt.Errorf("%w: %v", ErrRefused, err)
	}
	if subtle.ConstantTimeCompare([]byte(delegationDigest(req.Token, grant)), []byte(*owner.DelegationDigest)) != 1 {
		return Capability{}, fmt.Errorf("%w: delegation grant does not match lock %s", ErrRefused, owner.LockID)
	}
	if subtle.ConstantTimeCompare([]byte(req.ExecutableSHA256), []byte(*owner.ExecutableSHA256)) != 1 {
		return Capability{}, fmt.Errorf("%w: executable identity does not match lock %s", ErrRefused, owner.LockID)
	}
	return grant, nil
}

func delegationGrant(capability Capability) (Capability, error) {
	normalized := capability.normalized()
	if normalized.SelectedTask == "" {
		return Capability{}, fmt.Errorf("delegation grant names no selected task")
	}
	if len(normalized.Writes) == 0 {
		return Capability{}, fmt.Errorf("delegation grant names no write set")
	}
	grant := DelegatedCapability()
	grant.SelectedTask = normalized.SelectedTask
	grant.Writes = normalized.Writes
	return grant, nil
}

func delegationDigest(token string, grant Capability) string {
	canonical, _ := json.Marshal(struct {
		SelectedTask string   `json:"selected_task"`
		Writes       []string `json:"writes"`
	}{
		SelectedTask: grant.SelectedTask,
		Writes:       grant.Writes,
	})
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write(canonical)
	return hex.EncodeToString(mac.Sum(nil))
}

// Owner reports the metadata of the lock this delegate joined.
func (j *Joined) Owner() Owner { return j.owner }

// Repository reports the exact context authenticated by this delegated join.
func (j *Joined) Repository() Repository { return j.repository }

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
	return &Joined{repository: j.repository, owner: j.owner, capability: capability}, nil
}
