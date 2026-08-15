package durabletx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/tessariq/taskrail/internal/durablefs"
	"github.com/tessariq/taskrail/internal/repolock"
)

// Phase is one durable transaction state. The exact set and its spelling are the
// machine contract's, so retained state a command left behind is reportable as
// recovery evidence without translation.
type Phase string

const (
	// PhasePrepared is the complete recovery evidence — originals, candidate
	// digests, and path identities — durably recorded. Nothing semantic is
	// published yet.
	PhasePrepared Phase = "prepared"
	// PhaseFencePublished is the whole set re-proven unchanged immediately before
	// publication begins. Nothing semantic is published yet.
	PhaseFencePublished Phase = "fence_published"
	// PhasePublishing is persisted before the first semantic byte changes, so an
	// interruption during publication is never mistaken for one before it.
	PhasePublishing Phase = "publishing"
	// PhaseCandidatePublished is the complete candidate on disk.
	PhaseCandidatePublished Phase = "candidate_published"
	// PhaseValidating is the published candidate being checked by its command.
	PhaseValidating Phase = "validating"
	// PhaseRollingBack is the transaction undoing its own publication.
	PhaseRollingBack Phase = "rolling_back"
	// PhaseRecoveryRestoring, PhaseRecoveryAccepting, and PhaseRecoveryClearing
	// are the recovery engine's own phases. Persisting one pins the single action
	// a retry repeats, which is what makes recovery interruption-safe.
	PhaseRecoveryRestoring Phase = "recovery_restoring"
	PhaseRecoveryAccepting Phase = "recovery_accepting"
	PhaseRecoveryClearing  Phase = "recovery_clearing"
)

// canonicalSuccessors is the only order a durable transaction may persist its
// phases in. A transition outside it is a defect in the driver rather than an
// outcome, so it fails before anything is written.
var canonicalSuccessors = map[Phase][]Phase{
	PhasePrepared:           {PhaseFencePublished},
	PhaseFencePublished:     {PhasePublishing},
	PhasePublishing:         {PhaseCandidatePublished, PhaseRollingBack},
	PhaseCandidatePublished: {PhaseValidating, PhaseRollingBack},
	PhaseValidating:         {PhaseRollingBack},
	PhaseRollingBack:        {},
	PhaseRecoveryRestoring:  {},
	PhaseRecoveryAccepting:  {},
	PhaseRecoveryClearing:   {},
}

// recoveryPhases are the phases only the recovery engine persists, each reached
// from any retained writer phase or repeated from itself.
var recoveryPhases = []Phase{PhaseRecoveryRestoring, PhaseRecoveryAccepting, PhaseRecoveryClearing}

func (p Phase) known() bool {
	_, ok := canonicalSuccessors[p]
	return ok
}

// journal is the canonical admission document. Its members are exactly the three
// the machine contract publishes as recovery evidence, so the repository-wide
// fence can name the retained transaction without decoding the manifest.
type journal struct {
	TransactionID string `json:"transaction_id"`
	Command       string `json:"command"`
	Phase         Phase  `json:"phase"`
}

// manifest is the complete write-set evidence, recorded once before the first
// publication and never rewritten. Original bytes live beside it under
// `originals/`, because a digest cannot restore a file.
type manifest struct {
	TransactionID string           `json:"transaction_id"`
	Command       string           `json:"command"`
	Members       []manifestMember `json:"members"`
}

type manifestMember struct {
	Kind      PathKind   `json:"kind"`
	Reported  string     `json:"reported"`
	Path      string     `json:"path"`
	Published bool       `json:"published"`
	Ancestors []identity `json:"ancestors"`
	// Original is the exact state observed before publication, or null when the
	// member was absent. Absence is a recorded state, not missing evidence.
	Original *fileState `json:"original"`
	// Candidate is what publication commits. Its bytes are the repository's, so
	// only the digest and mode are recorded here.
	Candidate *fileState `json:"candidate"`
}

// fileState is one path's presence-bearing evidence: exact bytes, exact mode,
// and the native identity those were observed through.
type fileState struct {
	SHA256   string    `json:"sha256"`
	Mode     uint32    `json:"mode"`
	Identity *identity `json:"identity"`
}

type identity struct {
	Volume uint64 `json:"volume"`
	File   uint64 `json:"file"`
	Mount  uint64 `json:"mount"`
}

func recordedIdentity(observed durablefs.Identity) *identity {
	return &identity{Volume: observed.Volume, File: observed.File, Mount: observed.Mount}
}

func (i *identity) matches(observed durablefs.Identity) bool {
	if i == nil {
		return false
	}
	return i.Volume == observed.Volume && i.File == observed.File && i.Mount == observed.Mount
}

func stateOf(snapshot durablefs.Snapshot) fileState {
	return fileState{SHA256: snapshot.SHA256, Mode: uint32(snapshot.Mode), Identity: recordedIdentity(snapshot.Identity)}
}

// holds reports whether an observed snapshot is exactly this recorded state's
// bytes and mode. Identity is deliberately excluded: a file rewritten with
// identical bytes is semantically the state recorded, and identity is reported
// as a substitution signal rather than folded into that judgement.
func (s fileState) holds(snapshot durablefs.Snapshot) bool {
	return s.SHA256 == snapshot.SHA256 && s.Mode == uint32(snapshot.Mode)
}

func (s fileState) mode() fs.FileMode { return fs.FileMode(s.Mode) }

func (m manifest) validate(transactionID string, repo repolock.Repository) error {
	if m.TransactionID != transactionID {
		return fmt.Errorf("manifest names transaction %q beneath %q", m.TransactionID, transactionID)
	}
	if !commandPattern.MatchString(m.Command) {
		return fmt.Errorf("manifest command %q is not a canonical command path", m.Command)
	}
	if len(m.Members) == 0 {
		return fmt.Errorf("manifest for %s records no member", transactionID)
	}
	seen := make(map[string]struct{}, len(m.Members))
	physical := make(map[string]struct{}, len(m.Members))
	for i, member := range m.Members {
		if err := (Member{Kind: member.Kind, Reported: member.Reported, Path: member.Path}).validate(repo); err != nil {
			return err
		}
		key := string(member.Kind) + "\x00" + member.Reported
		if _, ok := seen[key]; ok {
			return fmt.Errorf("manifest records %s path %q twice", member.Kind, member.Reported)
		}
		seen[key] = struct{}{}
		physicalPath := filepath.Join(rootOf(member.Kind, repo), filepath.FromSlash(member.Path))
		if _, ok := physical[physicalPath]; ok {
			return fmt.Errorf("manifest maps two members onto %s", physicalPath)
		}
		physical[physicalPath] = struct{}{}
		if i > 0 {
			prior := m.Members[i-1]
			if string(prior.Kind) > string(member.Kind) || (prior.Kind == member.Kind && prior.Reported >= member.Reported) {
				return fmt.Errorf("manifest members are not in canonical order")
			}
		}
		if (member.Published && (member.Candidate == nil || member.Candidate.SHA256 == "")) ||
			(!member.Published && member.Candidate != nil) || (member.Original != nil && member.Original.SHA256 == "") {
			return fmt.Errorf("manifest member %q records no digest", member.Reported)
		}
		if len(member.Ancestors) == 0 || (member.Original != nil && member.Original.Identity == nil) {
			return fmt.Errorf("manifest member %q records incomplete identity evidence", member.Reported)
		}
		for _, state := range []*fileState{member.Original, member.Candidate} {
			if state != nil && (!digestPattern.MatchString(state.SHA256) || uint32(durablefs.PortableMode(fs.FileMode(state.Mode))) != state.Mode) {
				return fmt.Errorf("manifest member %q records invalid digest or mode", member.Reported)
			}
		}
		if member.Candidate != nil && member.Candidate.Identity != nil {
			return fmt.Errorf("manifest member %q records an identity for unpublished candidate bytes", member.Reported)
		}
	}
	return nil
}

func (j journal) validate(transactionID string) error {
	if j.TransactionID != transactionID {
		return fmt.Errorf("journal names transaction %q beneath %q", j.TransactionID, transactionID)
	}
	if !commandPattern.MatchString(j.Command) {
		return fmt.Errorf("journal command %q is not a canonical command path", j.Command)
	}
	if !j.Phase.known() {
		return fmt.Errorf("journal records unknown phase %q", j.Phase)
	}
	return nil
}

// encodeDocument renders one recovery document exactly as the strict decoders
// read it back: UTF-8, no byte order mark, no trailing value.
func encodeDocument(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode recovery document: %w", err)
	}
	return data, nil
}

// decodeDocument fails closed on anything a Taskrail writer could not have
// produced — unknown members, a trailing value, a wrong type — because retained
// state that cannot be read exactly is evidence an operator inspects rather than
// permission to guess an action.
func decodeDocument(data []byte, into any) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("recovery document is not UTF-8")
	}
	if err := rejectDuplicateKeys(data); err != nil {
		return err
	}
	if err := validateDocumentShape(data, into); err != nil {
		return err
	}
	if err := json.Unmarshal(data, into); err != nil {
		return fmt.Errorf("decode recovery document: %w", err)
	}
	return nil
}

func validateDocumentShape(data []byte, into any) error {
	raw := json.RawMessage(data)
	switch into.(type) {
	case *journal:
		return validateJournalShape(raw)
	case *manifest:
		return validateManifestShape(raw)
	case *completion:
		object, err := exactRawObject(raw, "completion", "action", "manifest")
		if err != nil {
			return err
		}
		if err := requireString(object["action"], "completion.action"); err != nil {
			return err
		}
		return validateManifestShape(object["manifest"])
	default:
		return fmt.Errorf("unsupported recovery document target %T", into)
	}
}

func validateJournalShape(raw json.RawMessage) error {
	object, err := exactRawObject(raw, "journal", "transaction_id", "command", "phase")
	if err != nil {
		return err
	}
	for _, name := range []string{"transaction_id", "command", "phase"} {
		if err := requireString(object[name], "journal."+name); err != nil {
			return err
		}
	}
	return nil
}

func validateManifestShape(raw json.RawMessage) error {
	object, err := exactRawObject(raw, "manifest", "transaction_id", "command", "members")
	if err != nil {
		return err
	}
	for _, name := range []string{"transaction_id", "command"} {
		if err := requireString(object[name], "manifest."+name); err != nil {
			return err
		}
	}
	var members []json.RawMessage
	if isNull(object["members"]) || json.Unmarshal(object["members"], &members) != nil {
		return fmt.Errorf("manifest.members is not an array")
	}
	for i, member := range members {
		if err := validateMemberShape(member, i); err != nil {
			return err
		}
	}
	return nil
}

func validateMemberShape(raw json.RawMessage, index int) error {
	what := fmt.Sprintf("manifest.members[%d]", index)
	object, err := exactRawObject(raw, what, "kind", "reported", "path", "published", "ancestors", "original", "candidate")
	if err != nil {
		return err
	}
	for _, name := range []string{"kind", "reported", "path"} {
		if err := requireString(object[name], what+"."+name); err != nil {
			return err
		}
	}
	var published bool
	if isNull(object["published"]) || json.Unmarshal(object["published"], &published) != nil {
		return fmt.Errorf("%s.published is not a boolean", what)
	}
	var ancestors []json.RawMessage
	if isNull(object["ancestors"]) || json.Unmarshal(object["ancestors"], &ancestors) != nil {
		return fmt.Errorf("%s.ancestors is not an array", what)
	}
	for i, ancestor := range ancestors {
		if err := validateIdentityShape(ancestor, fmt.Sprintf("%s.ancestors[%d]", what, i)); err != nil {
			return err
		}
	}
	for _, name := range []string{"original", "candidate"} {
		if !isNull(object[name]) {
			if err := validateStateShape(object[name], what+"."+name); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateStateShape(raw json.RawMessage, what string) error {
	object, err := exactRawObject(raw, what, "sha256", "mode", "identity")
	if err != nil {
		return err
	}
	if err := requireString(object["sha256"], what+".sha256"); err != nil {
		return err
	}
	var mode uint32
	if isNull(object["mode"]) || json.Unmarshal(object["mode"], &mode) != nil {
		return fmt.Errorf("%s.mode is not an unsigned integer", what)
	}
	if !isNull(object["identity"]) {
		return validateIdentityShape(object["identity"], what+".identity")
	}
	return nil
}

func validateIdentityShape(raw json.RawMessage, what string) error {
	object, err := exactRawObject(raw, what, "volume", "file", "mount")
	if err != nil {
		return err
	}
	for _, name := range []string{"volume", "file", "mount"} {
		var value uint64
		if isNull(object[name]) || json.Unmarshal(object[name], &value) != nil {
			return fmt.Errorf("%s.%s is not an unsigned integer", what, name)
		}
	}
	return nil
}

func exactRawObject(raw json.RawMessage, what string, names ...string) (map[string]json.RawMessage, error) {
	if isNull(raw) {
		return nil, fmt.Errorf("%s is null", what)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, fmt.Errorf("%s is not an object", what)
	}
	if len(object) != len(names) {
		return nil, fmt.Errorf("%s has missing or unknown members", what)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return nil, fmt.Errorf("%s is missing member %q", what, name)
		}
	}
	return object, nil
}

func requireString(raw json.RawMessage, what string) error {
	var value string
	if isNull(raw) || json.Unmarshal(raw, &value) != nil {
		return fmt.Errorf("%s is not a string", what)
	}
	return nil
}

func isNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func rejectDuplicateKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := scanJSONValue(decoder); err != nil {
		return fmt.Errorf("decode recovery document: %w", err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("recovery document has a trailing value")
		}
		return fmt.Errorf("decode recovery document trailer: %w", err)
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object member is not a string")
			}
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("duplicate object member %q", key)
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
	closing, err := decoder.Token()
	if err != nil {
		return err
	}
	want := json.Delim('}')
	if delimiter == '[' {
		want = ']'
	}
	if closing != want {
		return fmt.Errorf("JSON delimiter %q closes %q", closing, delimiter)
	}
	return nil
}

// store is the pair of bound roots one durable transaction operates through:
// the repository the lock covers, and the Git common directory a linked worktree
// shares its retained state beneath.
type store struct {
	repository *durablefs.Root
	managed    *durablefs.Root
	git        *durablefs.Root
	// base is the root the transactions subtree lives beneath; it aliases one of
	// the two roots above and is never closed separately.
	base         *durablefs.Root
	baseAbsolute string
	transactions string
	repo         repolock.Repository
}

func openStore(own Ownership, repo repolock.Repository) (*store, error) {
	repository, err := durablefs.Open(repo.Root, own)
	if err != nil {
		return nil, err
	}
	opened := &store{repository: repository, managed: repository, repo: repo}
	if repo.StorageRoot() != repo.Root {
		managed, err := durablefs.OpenAt(repo.StorageRoot(), repo, own)
		if err != nil {
			repository.Close()
			return nil, err
		}
		opened.managed = managed
	}
	base, transactions := transactionsPath(repo)
	opened.baseAbsolute, opened.transactions = base, transactions
	if repo.GitCommonDir == "" {
		opened.base = repository
		return opened, nil
	}
	git, err := durablefs.OpenAt(repo.GitCommonDir, repo, own)
	if err != nil {
		repository.Close()
		return nil, err
	}
	opened.git, opened.base = git, git
	return opened, nil
}

func (s *store) Close() error {
	err := s.repository.Close()
	if s.managed != s.repository {
		err = errors.Join(err, s.managed.Close())
	}
	if s.git != nil {
		if gitErr := s.git.Close(); err == nil {
			err = gitErr
		}
	}
	return err
}

// rootFor is the bound root one member publishes beneath.
func (s *store) rootFor(kind PathKind) *durablefs.Root {
	if kind == Managed {
		return s.managed
	}
	if kind == Git {
		return s.git
	}
	return s.repository
}

func (s *store) absoluteFor(kind PathKind) string {
	return rootOf(kind, s.repo)
}

func (s *store) transactionDir(transactionID string) string {
	return s.transactions + "/" + transactionID
}

// ensureDirectory creates every missing component of a slash path beneath root,
// leaving an existing plain directory alone. A component occupied by anything
// else refuses through the primitives rather than here.
func (s *store) ensureDirectory(root *durablefs.Root, absolute, relative string) error {
	components := splitSlash(relative)
	for i := range components {
		current := path.Join(components[:i+1]...)
		snapshot, err := durablefs.ObserveTree(absolute, current)
		if err != nil {
			return err
		}
		if snapshot.Present {
			continue
		}
		if _, err := root.Mkdir(current, directoryMode); err != nil {
			return err
		}
	}
	return nil
}

func splitSlash(relative string) []string {
	return slices.DeleteFunc(strings.Split(relative, "/"), func(part string) bool { return part == "" })
}

// readDocument reads one bounded recovery document through a stable no-follow
// read and decodes it strictly.
func (s *store) readDocument(relative string, into any) error {
	data, _, err := durablefs.ReadFile(s.baseAbsolute, relative, maximumJournalBytes)
	if err != nil {
		return err
	}
	return decodeDocument(data, into)
}

// writeJournal publishes the phase document that makes a transaction visible to
// the repository-wide admission fence.
func (s *store) writeJournal(transactionID string, doc journal) error {
	data, err := encodeDocument(doc)
	if err != nil {
		return err
	}
	return publishDocument(s.base, s.transactionDir(transactionID)+"/"+journalName, data)
}

// canAdvance holds the driver to the canonical order. A recovery phase is
// reachable from any retained writer phase, because recovery is what a retained
// transaction is handed to; it is not reachable from another recovery phase,
// because that would let one interrupted action be replaced by a different one.
func canAdvance(from, to Phase) bool {
	if slices.Contains(recoveryPhases, to) {
		return from.known() && !slices.Contains(recoveryPhases, from)
	}
	return slices.Contains(canonicalSuccessors[from], to)
}

// advance persists one phase transition and returns only after the new phase is
// durable, so the effect a phase announces can never reach disk before the phase
// that explains it.
func (s *store) advance(transactionID string, doc journal, next Phase) (journal, error) {
	if !canAdvance(doc.Phase, next) {
		return doc, fmt.Errorf("durable transaction cannot move from %q to %q", doc.Phase, next)
	}
	data, err := encodeDocument(journal{TransactionID: doc.TransactionID, Command: doc.Command, Phase: next})
	if err != nil {
		return doc, err
	}
	entry, err := s.base.Bind(s.transactionDir(transactionID) + "/" + journalName)
	if err != nil {
		return doc, err
	}
	replaced, err := entry.Replace(data, documentMode)
	if err != nil {
		entry.Close()
		return doc, err
	}
	if err := replaced.Close(); err != nil {
		return doc, err
	}
	doc.Phase = next
	return doc, nil
}
