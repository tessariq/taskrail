// Package durabletx implements the durable transaction every v0.5 writer that
// must survive abrupt process or host death publishes through
// (specs/v0.5.0.md#repository-discovery-locking-and-recovery).
//
// A durable transaction records exact originals, candidate digests, path
// identities, its owning command, and its identity beneath the shared lock root
// before the first semantic publication, then persists each phase transition
// before the effect that transition announces. An interruption therefore always
// leaves retained transaction state, which is the repository-wide recovery fence
// the admission check refuses every semantic read and write against.
//
// This package owns the journal state machine and the recovery engine. Binding,
// publication, and durability barriers belong to durablefs; the repository-wide
// admission fence belongs to the service; the public `recover` command routing
// belongs to its own outcome.
package durabletx

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tessariq/taskrail/internal/repolock"
)

// PathKind is the class one member belongs to, which also selects the root it
// binds beneath: managed and worktree paths live inside the repository, and Git
// metadata lives beneath the Git common directory a linked worktree shares.
type PathKind string

const (
	Managed  PathKind = "managed"
	Worktree PathKind = "worktree"
	Git      PathKind = "git"
)

// Ownership is the held mutation lock a durable transaction publishes under. It
// is the interface repotx bounds normal transactions with, for the same reason:
// a delegate has to arrive already narrowed to one task and one write set.
type Ownership interface {
	Owner() repolock.Owner
	Repository() repolock.Repository
	Capability() repolock.Capability
	Authorize(command string, fields ...string) error
	IsDelegate() bool
}

var _ Ownership = (*repolock.Joined)(nil)

// Member is one published path plus the exact bytes the transaction commits to
// it. Path is relative to the root Kind selects and is always slash-spelled;
// Reported is the spelling the machine contract publishes.
type Member struct {
	Kind     PathKind
	Reported string
	Path     string
	Content  []byte
	// Mode is the mode a member absent before the transaction is created with.
	// It is ignored for an existing member, whose recorded original mode is
	// preserved, so publication never silently changes a file's permissions.
	Mode fs.FileMode
	// Fence is the intermediate byte state exactly one published member may be
	// temporarily published with after the originals are recorded durably and
	// before any other semantic byte changes. Its final Content publishes as
	// the transaction's last semantic operation, after post-publication
	// validation. A fence member must sort before every other published member,
	// so rollback and recovery restore its original last: while any candidate
	// byte remains on disk, the fence member still fences the repository
	// against readers and writers that predate the transaction.
	Fence []byte
	// PreSemantic members publish after durable preparation and before ordinary
	// candidates, so a writer can verify a required environment first.
	PreSemantic bool
	// PreSemanticPriority orders pre-semantic members without changing normal
	// canonical transaction ordering. Lower values publish first.
	PreSemanticPriority int
}

// Path is one consumed path. It participates in every whole-set comparison but
// is never published and therefore has no candidate digest.
type Path struct {
	Kind     PathKind
	Reported string
	Path     string
}

// Request is one writer's complete durable transaction.
type Request struct {
	// Command is the canonical command path publishing this transaction.
	Command string
	// SelectedTask is the task the transaction acts on, empty when there is none.
	SelectedTask string
	// TaskFields are the task fields the transaction writes.
	TaskFields []string
	// Consumed are inputs used to build the candidate but not published by this
	// transaction. They remain in every final whole-set comparison.
	Consumed []Path
	// Members is the exact published set.
	Members []Member
	// Validate runs against the published candidate before the fence is cleared.
	// It is the command-specific check recovery re-runs before accepting a
	// candidate an interruption left behind, so it must be a pure function of the
	// published bytes.
	Validate func([]Evidence) error
	// ValidateBeforeCandidates runs after pre-semantic members publish and before
	// ordinary candidate publication.
	ValidateBeforeCandidates func([]Evidence) error
}

// Result is a committed durable transaction.
type Result struct {
	TransactionID string
	Phase         Phase
	Members       []Evidence
}

// Evidence is one member's recorded and observed byte state. Digests are
// lower-case 64-hex or empty when the member is absent.
type Evidence struct {
	Kind            PathKind
	Reported        string
	OriginalSHA256  string
	CandidateSHA256 string
	CurrentSHA256   string
	// FenceSHA256 is the digest of the fence member's intermediate bytes, empty
	// for every member that publishes in one stage. The owning command's
	// recovery validator uses it to recognize a validated candidate whose fence
	// member has not yet published its final bytes.
	FenceSHA256 string
	// IdentityChanged reports that a member holding its recorded original bytes
	// no longer holds the identity those bytes were recorded through. It is a
	// substitution signal recovery reports rather than an action it derives:
	// every action leaves such a member untouched.
	IdentityChanged bool
}

const (
	transactionsDirName = "transactions"
	journalName         = "journal.json"
	manifestName        = "manifest.json"
	originalsDirName    = "originals"
	finalsDirName       = "finals"
	publishedDirName    = "published"
	// maximumJournalBytes bounds every recovery document read. Journals are the
	// state one interrupted command left behind, not a general storage format.
	maximumJournalBytes = 1 << 24
	directoryMode       = fs.FileMode(0o755)
	documentMode        = fs.FileMode(0o644)
	defaultMemberMode   = fs.FileMode(0o644)
)

var (
	transactionIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
	digestPattern        = regexp.MustCompile(`^[0-9a-f]{64}$`)
	commandPattern       = regexp.MustCompile(`^[a-z][a-z-]*( [a-z][a-z-]*)*$`)
)

// TransactionsDir is the absolute directory retained transaction state lives
// beneath. It is the lock's own directory plus `transactions`, so a writer, a
// recovery run, and the admission fence all resolve one location from the same
// repository context, whichever worktree or subdirectory they were invoked from.
func TransactionsDir(repo repolock.Repository) string {
	return filepath.Join(filepath.Dir(repolock.LockPath(repo)), transactionsDirName)
}

// transactionsPath is TransactionsDir expressed as a slash path relative to the
// root that contains it: the Git common directory when there is one, and the
// repository root otherwise.
func transactionsPath(repo repolock.Repository) (base string, relative string) {
	if repo.GitCommonDir != "" {
		return repo.GitCommonDir, "taskrail/" + transactionsDirName
	}
	return repo.Root, ".taskrail/runtime/" + transactionsDirName
}

func (r Request) validate(repo repolock.Repository) error {
	if !commandPattern.MatchString(r.Command) {
		return fmt.Errorf("durable transaction command %q is not a canonical command path", r.Command)
	}
	if len(r.Members) == 0 {
		return fmt.Errorf("durable transaction %q publishes nothing", r.Command)
	}
	seenReported := make(map[string]struct{}, len(r.Members)+len(r.Consumed))
	seenPath := make(map[string]struct{}, len(r.Members)+len(r.Consumed))
	for _, consumed := range r.Consumed {
		if err := (Member{Kind: consumed.Kind, Reported: consumed.Reported, Path: consumed.Path}).validate(repo); err != nil {
			return err
		}
		if err := recordUnique(repo, seenReported, seenPath, consumed.Kind, consumed.Reported, consumed.Path); err != nil {
			return err
		}
	}
	for _, m := range r.Members {
		if err := m.validate(repo); err != nil {
			return err
		}
		if err := recordUnique(repo, seenReported, seenPath, m.Kind, m.Reported, m.Path); err != nil {
			return err
		}
	}
	return validateFenceOrder(membersOf(r.Members))
}

// membersOf projects request members onto the manifest member shape the fence
// ordering rule shares between requests and decoded manifests.
func membersOf(members []Member) []manifestMember {
	out := make([]manifestMember, len(members))
	for i, m := range members {
		out[i] = manifestMember{Kind: m.Kind, Reported: m.Reported, Path: m.Path, Published: true}
		if m.Fence != nil {
			out[i].Fence = &fileState{}
		}
	}
	return out
}

// validateFenceOrder enforces the two structural rules of a fenced transaction:
// at most one fence member, and no published member sorting before it, which is
// what makes every restore path return the fence member to its original last.
func validateFenceOrder(members []manifestMember) error {
	fenced := -1
	for i, m := range members {
		if m.Fence == nil {
			continue
		}
		if fenced >= 0 {
			return fmt.Errorf("durable transaction names fence members %q and %q", members[fenced].Reported, m.Reported)
		}
		fenced = i
	}
	if fenced < 0 {
		return nil
	}
	for i, m := range members {
		if i == fenced || !m.Published {
			continue
		}
		if less(members[i], members[fenced]) {
			return fmt.Errorf("published member %q sorts before fence member %q", m.Reported, members[fenced].Reported)
		}
	}
	return nil
}

// less is the canonical member order prepareEntries sorts by: path kind, then
// reported path.
func less(a, b manifestMember) bool {
	if a.Kind != b.Kind {
		return string(a.Kind) < string(b.Kind)
	}
	return a.Reported < b.Reported
}

func recordUnique(repo repolock.Repository, reported, physical map[string]struct{}, kind PathKind, name, relative string) error {
	reportedKey := string(kind) + "\x00" + name
	if _, ok := reported[reportedKey]; ok {
		return fmt.Errorf("durable transaction names %s path %q twice", kind, name)
	}
	reported[reportedKey] = struct{}{}
	physicalKey := filepath.Join(rootOf(kind, repo), filepath.FromSlash(relative))
	if _, ok := physical[physicalKey]; ok {
		return fmt.Errorf("durable transaction maps two members onto %s", physicalKey)
	}
	physical[physicalKey] = struct{}{}
	return nil
}

func (m Member) validate(repo repolock.Repository) error {
	switch m.Kind {
	case Managed, Worktree:
		if !canonicalRelative(m.Reported) {
			return fmt.Errorf("%s member %q is not canonical repository-relative", m.Kind, m.Reported)
		}
		if m.Path != m.Reported {
			return fmt.Errorf("%s member %q maps to different path %q", m.Kind, m.Reported, m.Path)
		}
	case Git:
		if repo.GitCommonDir == "" {
			return fmt.Errorf("git member %q has no Git common directory to bind beneath", m.Reported)
		}
		if !canonicalAbsolute(m.Reported) {
			return fmt.Errorf("git member %q is not canonical absolute", m.Reported)
		}
		physical := filepath.ToSlash(filepath.Join(repo.GitCommonDir, filepath.FromSlash(m.Path)))
		if m.Reported != physical {
			return fmt.Errorf("git member %q maps to different path %q", m.Reported, physical)
		}
	default:
		return fmt.Errorf("unknown path kind %q for %q", m.Kind, m.Reported)
	}
	if strings.TrimSpace(m.Reported) == "" {
		return fmt.Errorf("%s member %q reports no path", m.Kind, m.Path)
	}
	if !canonicalRelative(m.Path) {
		return fmt.Errorf("%s member %q is not a canonical root-relative path", m.Kind, m.Path)
	}
	// A member that could publish into the journal area could rewrite the exact
	// evidence recovery derives its single safe action from.
	base, transactions := transactionsPath(repo)
	if rootOf(m.Kind, repo) == base && (m.Path == transactions || strings.HasPrefix(m.Path, transactions+"/")) {
		return fmt.Errorf("%s member %q publishes into retained transaction state", m.Kind, m.Path)
	}
	return nil
}

// rootOf is the absolute root a member of this kind binds beneath.
func rootOf(kind PathKind, repo repolock.Repository) string {
	if kind == Managed {
		return repo.StorageRoot()
	}
	if kind == Git {
		return repo.GitCommonDir
	}
	return repo.Root
}

func canonicalAbsolute(value string) bool {
	if rest, ok := strings.CutPrefix(value, "/"); ok {
		return canonicalRelative(rest)
	}
	return len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) &&
		value[1] == ':' && value[2] == '/' && canonicalRelative(value[3:])
}

// canonicalRelative rejects backslashes, absolute spellings, empty, dot, and
// dot-dot segments, so one member path denotes exactly one location.
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

// authorize refuses every write outside the ownership's bound before anything is
// read or recorded, and requires a delegate to arrive narrowed on both its task
// and its write set.
func authorize(own Ownership, req Request) error {
	capability := own.Capability()
	if own.IsDelegate() {
		if strings.TrimSpace(capability.SelectedTask) == "" {
			return refused(fmt.Errorf("%w: delegated %s names no selected task", repolock.ErrRefused, req.Command))
		}
		if len(capability.Writes) == 0 {
			return refused(fmt.Errorf("%w: delegated %s names no write set", repolock.ErrRefused, req.Command))
		}
		if own.Owner().StorageMode == repolock.ModeLocal {
			for _, member := range req.Members {
				if member.Kind == Worktree {
					return refused(fmt.Errorf("%w: delegated local write %q is ambiguous between managed and worktree storage", repolock.ErrRefused, member.Reported))
				}
			}
		}
	}
	if err := own.Authorize(req.Command, req.TaskFields...); err != nil {
		return refused(err)
	}
	if err := capability.AllowsTask(req.SelectedTask); err != nil {
		return refused(err)
	}
	reported := make([]string, 0, len(req.Members))
	for _, m := range req.Members {
		reported = append(reported, m.Reported)
	}
	if err := capability.AllowsWrites(reported); err != nil {
		return refused(err)
	}
	return nil
}

func refused(err error) error { return &Error{Kind: KindRefused, err: err} }

func newTransactionID(own Ownership) (string, error) {
	// A lock acquired for a named transaction already published that identity in
	// its metadata, so the journal has to use it rather than invent a second one.
	if declared := own.Owner().TransactionID; declared != nil {
		if !transactionIDPattern.MatchString(*declared) {
			return "", fmt.Errorf("lock names transaction %q, which is not a lower-case 32-hex id", *declared)
		}
		return *declared, nil
	}
	return "", fmt.Errorf("durable transaction lock names no transaction id")
}
