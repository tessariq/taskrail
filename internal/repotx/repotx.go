// Package repotx implements the normal transaction every v0.5 semantic writer
// publishes through (specs/v0.5.0.md#repository-discovery-locking-and-recovery).
//
// A normal transaction holds the repository mutation lock, snapshots the
// complete consumed and published set, validates the complete candidate before
// the first write, atomically replaces each published file, and compare-and-swap
// rolls back a handled failure.
//
// The all-or-none promise covers handled command failures, not abrupt process or
// host death: nothing here records a durable manifest, so a crash mid-publication
// is the durable transaction's problem rather than this one's. Rollback is
// equally conservative — a published path whose bytes are no longer this
// transaction's candidate belongs to whoever changed it, and is preserved and
// reported instead of overwritten.
package repotx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

// PathKind is the class one snapshot path belongs to. Keeping the three classes
// apart lets a single transaction bind semantic files, physical repository
// files, and Git metadata without conflating how each is spelled.
type PathKind string

const (
	// Managed is a logical semantic path such as `planning/tasks/T-1.md`. Local
	// storage maps it onto an overlay root, so its logical and physical
	// spellings differ.
	Managed PathKind = "managed"
	// Worktree is a repository-relative physical path, such as an installed
	// skill or a local runtime file.
	Worktree PathKind = "worktree"
	// Git is a canonical absolute path to effective Git metadata.
	Git PathKind = "git"
)

// Path is one member of a transaction's consumed or published set. Reported is
// the spelling the machine contract publishes for Kind; Physical is where the
// bytes actually live. The two differ whenever local storage maps a logical
// namespace onto an overlay root, which is exactly why both are explicit here
// rather than derived.
type Path struct {
	Kind     PathKind
	Reported string
	Physical string
}

// Candidate is one published path plus the exact bytes this transaction
// commits to it — or, with Remove, the publication of that path's absence.
type Candidate struct {
	Path
	Content []byte
	// NoClobber publishes only to a path that was absent in the snapshot. It is
	// for publication points such as a newly versioned spec: a late creator must
	// win rather than be overwritten by the transaction's atomic replacement.
	NoClobber bool
	// PublishPriority orders semantic publication without changing snapshot and
	// diagnostic ordering. Lower priorities publish first and roll back last.
	PublishPriority int
	// Remove publishes the path's absence instead of bytes: publication
	// removes the file (an already-absent path is a no-op, mirroring
	// restore's leniency), and a rollback restores the snapshot's original
	// bytes. A removal candidate that also carries content is a defect no
	// writer could have built and is rejected before the snapshot.
	Remove bool
}

// Snapshot is the byte evidence one path contributes to a result or a failure.
// Every digest is lower-case 64-hex or nil. The machine contract admits no third
// value, so nil means the transaction has no bytes to report: the path was
// absent, it could not be read, or a failure elsewhere stopped the scan before
// this path's turn. For the path a failure names, its kind says which.
type Snapshot struct {
	Kind PathKind
	Path string
	// OriginalSHA256 is the path as this transaction first observed it.
	OriginalSHA256 *string
	// CandidateSHA256 is the bytes this transaction intended to publish. It is
	// nil for a consumed path, which the transaction reads but never writes.
	CandidateSHA256 *string
	// CurrentSHA256 is the path as it stands now, after publication, rollback, or
	// the conflict that stopped either.
	CurrentSHA256 *string
}

// Request is one writer's complete transaction: what it consumed, what it
// publishes, and the bound it claims to be working within.
type Request struct {
	// Command is the canonical command path publishing this transaction.
	Command string
	// SelectedTask is the task the transaction acts on, empty when there is none.
	SelectedTask string
	// TaskFields are the task fields the transaction writes.
	TaskFields []string
	// Consumed are the paths the transaction read but does not write. They join
	// the snapshot so a candidate built from stale reads is caught by the
	// compare-and-swap rather than published.
	Consumed []Path
	// Published is the exact write set.
	Published []Candidate
	// Validate checks the complete candidate before the first write. It receives
	// the preview snapshot so a validator can reason about exact bytes, and any
	// error it returns aborts the transaction with nothing written.
	Validate func(preview []Snapshot) error
}

func (r Request) validate() error {
	if strings.TrimSpace(r.Command) == "" {
		return fmt.Errorf("transaction names no command")
	}
	if len(r.Published) == 0 {
		return fmt.Errorf("transaction %q publishes nothing", r.Command)
	}
	for _, candidate := range r.Published {
		if candidate.PublishPriority < 0 {
			return fmt.Errorf("transaction %q assigns negative publication priority to %s", r.Command, candidate.Reported)
		}
		if candidate.Remove && len(candidate.Content) > 0 {
			return fmt.Errorf("transaction %q both removes and publishes bytes for %s path %q",
				r.Command, candidate.Kind, candidate.Reported)
		}
		if candidate.Remove && candidate.NoClobber {
			return fmt.Errorf("transaction %q both removes and no-clobber publishes %s path %q",
				r.Command, candidate.Kind, candidate.Reported)
		}
	}
	seenReported := make(map[string]struct{}, len(r.Consumed)+len(r.Published))
	seenPhysical := make(map[string]struct{}, len(r.Consumed)+len(r.Published))
	for _, path := range r.paths() {
		if err := path.validate(); err != nil {
			return err
		}
		key := string(path.Kind) + "\x00" + path.Reported
		if _, ok := seenReported[key]; ok {
			return fmt.Errorf("transaction names %s path %q twice", path.Kind, path.Reported)
		}
		seenReported[key] = struct{}{}
		physical := filepath.Clean(path.Physical)
		if _, ok := seenPhysical[physical]; ok {
			return fmt.Errorf("transaction maps two paths onto %s", physical)
		}
		seenPhysical[physical] = struct{}{}
	}
	return nil
}

func (r Request) paths() []Path {
	paths := make([]Path, 0, len(r.Consumed)+len(r.Published))
	paths = append(paths, r.Consumed...)
	for _, candidate := range r.Published {
		paths = append(paths, candidate.Path)
	}
	return paths
}

// publishedPaths are the reported paths a capability's write set has to cover.
func (r Request) publishedPaths() []string {
	paths := make([]string, 0, len(r.Published))
	for _, candidate := range r.Published {
		paths = append(paths, candidate.Reported)
	}
	return paths
}

// validate holds a path to the class its kind selects. Managed and worktree
// paths are canonical repository-relative; Git metadata paths are canonical
// absolute. The physical location is always absolute, so a transaction's bytes
// never depend on the invocation directory.
func (p Path) validate() error {
	switch p.Kind {
	case Managed, Worktree:
		if !canonicalRelative(p.Reported) {
			return fmt.Errorf("%s path %q is not canonical repository-relative", p.Kind, p.Reported)
		}
	case Git:
		if !canonicalAbsolute(p.Reported) {
			return fmt.Errorf("%s path %q is not canonical absolute", p.Kind, p.Reported)
		}
	default:
		return fmt.Errorf("unknown path kind %q for %q", p.Kind, p.Reported)
	}
	if !filepath.IsAbs(p.Physical) {
		return fmt.Errorf("%s path %q has non-absolute physical location %q", p.Kind, p.Reported, p.Physical)
	}
	return nil
}

// canonicalRelative rejects backslashes, absolute spellings, empty, dot, and
// dot-dot segments, so one reported path denotes exactly one location.
func canonicalRelative(path string) bool {
	if path == "" || strings.Contains(path, `\`) || strings.HasPrefix(path, "/") {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

// canonicalAbsolute accepts the two absolute spellings the machine contract
// admits — a POSIX root and a `C:/` drive root — followed by canonical segments.
func canonicalAbsolute(path string) bool {
	if rest, ok := strings.CutPrefix(path, "/"); ok {
		return canonicalRelative(rest)
	}
	if len(path) >= 3 && isDriveLetter(path[0]) && path[1] == ':' && path[2] == '/' {
		return canonicalRelative(path[3:])
	}
	return false
}

func isDriveLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func digestOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
