package taskrail

import (
	"fmt"
	"slices"
	"strings"
)

// WriterDurability names the publication boundary a command form promises. A
// normal transaction rolls back handled errors but makes no crash-atomic claim.
type WriterDurability string

const (
	WriterDurabilityReadOnly  WriterDurability = "read-only"
	WriterDurabilityExplicit  WriterDurability = "non-transactional"
	WriterDurabilityNormal    WriterDurability = "normal-transaction"
	WriterDurabilityDurable   WriterDurability = "durable-flow"
	WriterDurabilityDirectory WriterDurability = "directory-commit"
	WriterDurabilityStreamed  WriterDurability = "streamed-publication"
)

// WriterRecovery identifies the component that can recover or preserve a
// publication boundary after interruption.
type WriterRecovery string

const (
	WriterRecoveryNone      WriterRecovery = "none"
	WriterRecoveryDurable   WriterRecovery = "durable-transaction"
	WriterRecoveryDirectory WriterRecovery = "directory-commit"
	WriterRecoveryExternal  WriterRecovery = "external-no-clobber"
)

const (
	WriterLockNone       = "none"
	WriterLockRepository = "repository-mutation-lock"
	WriterLockLoop       = "loop-invocation-lock"
)

// WriterTransaction is the implementation-derived v0.5 mutation matrix. Its
// path-set labels are deliberately semantic rather than physical so local
// storage retains the same classification as committed storage.
type WriterTransaction struct {
	Owner          string
	Command        string
	Surface        MachineSurface
	Storage        []string
	Lock           string
	Consumes       []string
	Publishes      []string
	Durability     WriterDurability
	Recovery       WriterRecovery
	PhaseEvidence  string
	Implementation []string
}

var writerTransactionMatrix = []WriterTransaction{
	writer("init", "init", MachineSurfaceStdout, bothStorage, WriterLockRepository, "layout config", "layout/config, managed ledger", WriterDurabilityNormal, WriterRecoveryNone, ""),
	writer("init layout migration", "init", MachineSurfaceStdout, bothStorage, WriterLockRepository, "layout config, managed ledger", "layout/config, managed ledger", WriterDurabilityDurable, WriterRecoveryDurable, "durable transaction fence"),
	writer("init local", "init", MachineSurfaceStdout, []string{"local"}, WriterLockRepository, "layout config, local overlay, git exclusion", "layout/config, local overlay, git exclusion", WriterDurabilityDurable, WriterRecoveryDurable, "durable transaction fence"),
	writer("init local skill refresh", "init", MachineSurfaceStdout, []string{"local"}, WriterLockRepository, "layout config, local overlay, git exclusion", "local overlay", WriterDurabilityDurable, WriterRecoveryDurable, "durable transaction fence"),
	writer("retrofit", "retrofit", MachineSurfaceStdout, bothStorage, WriterLockRepository, "external source, layout config", "layout/config, managed ledger", WriterDurabilityNormal, WriterRecoveryNone, ""),
	reader("validate", "validate", MachineSurfaceStdout, bothStorage, "layout/config, managed ledger, task corpus, specs"),
	writer("repair", "repair", MachineSurfaceStdout, bothStorage, WriterLockRepository, "layout/config, task corpus", "managed ledger", WriterDurabilityNormal, WriterRecoveryNone, ""),
	reader("coverage", "coverage", MachineSurfaceStdout, bothStorage, "layout/config, task corpus, specs"),
	reader("status", "status", MachineSurfaceStdout, bothStorage, "layout/config, managed ledger, task corpus"),
	reader("stats", "stats", MachineSurfaceStdout, bothStorage, "layout/config, managed ledger, task corpus, specs"),
	writer("next", "next", MachineSurfaceStdout, bothStorage, WriterLockRepository, "managed ledger, task corpus", "managed ledger", WriterDurabilityNormal, WriterRecoveryNone, ""),
	writer("start", "start", MachineSurfaceStdout, bothStorage, WriterLockRepository, "managed ledger, selected task, task corpus", "managed ledger, selected task", WriterDurabilityNormal, WriterRecoveryNone, ""),
	writer("complete", "complete", MachineSurfaceStdout, bothStorage, WriterLockRepository, "managed ledger, selected task, task corpus", "managed ledger, selected task", WriterDurabilityNormal, WriterRecoveryNone, ""),
	writer("block", "block", MachineSurfaceStdout, bothStorage, WriterLockRepository, "managed ledger, selected task, task corpus", "managed ledger, selected task", WriterDurabilityNormal, WriterRecoveryNone, ""),
	writer("unblock", "unblock", MachineSurfaceStdout, bothStorage, WriterLockRepository, "managed ledger, selected task, task corpus", "managed ledger, selected task", WriterDurabilityNormal, WriterRecoveryNone, ""),
	writer("verify", "verify", MachineSurfaceStdout, bothStorage, WriterLockRepository, "managed ledger, selected task, task corpus", "managed ledger, selected task, verification artifact", WriterDurabilityNormal, WriterRecoveryNone, ""),
	writer("task new", "task new", MachineSurfaceStdout, bothStorage, WriterLockRepository, "managed ledger, task corpus, specs", "managed ledger, task", WriterDurabilityNormal, WriterRecoveryNone, ""),
	writer("task rename", "task rename", MachineSurfaceStdout, bothStorage, WriterLockRepository, "managed ledger, selected task, task corpus, specs", "managed ledger, task", WriterDurabilityNormal, WriterRecoveryNone, ""),
	writer("task repoint", "task repoint", MachineSurfaceStdout, bothStorage, WriterLockRepository, "managed ledger, selected task, task corpus, specs", "managed ledger, selected task", WriterDurabilityNormal, WriterRecoveryNone, ""),
	writer("task release", "task release", MachineSurfaceStdout, bothStorage, WriterLockRepository, "managed ledger, selected task, task corpus", "managed ledger, selected task", WriterDurabilityNormal, WriterRecoveryNone, ""),
	writer("task author", "task author", MachineSurfaceStdout, bothStorage, WriterLockRepository, "selected task, external source", "selected task", WriterDurabilityNormal, WriterRecoveryNone, ""),
	writer("task dependency add", "task dependency add", MachineSurfaceStdout, bothStorage, WriterLockRepository, "managed ledger, selected task, task corpus", "managed ledger, selected task", WriterDurabilityNormal, WriterRecoveryNone, ""),
	writer("task dependency remove", "task dependency remove", MachineSurfaceStdout, bothStorage, WriterLockRepository, "managed ledger, selected task, task corpus", "managed ledger, selected task", WriterDurabilityNormal, WriterRecoveryNone, ""),
	reader("task show", "task show", MachineSurfaceStdout, bothStorage, "task corpus"),
	reader("task loop list", "task loop list", MachineSurfaceStdout, bothStorage, "task corpus, loop policy"),
	writer("task loop allow", "task loop allow", MachineSurfaceStdout, bothStorage, WriterLockRepository, "managed ledger, selected task, task corpus", "managed ledger, selected task", WriterDurabilityNormal, WriterRecoveryNone, ""),
	writer("task loop hold", "task loop hold", MachineSurfaceStdout, bothStorage, WriterLockRepository, "managed ledger, selected task, task corpus", "managed ledger, selected task", WriterDurabilityNormal, WriterRecoveryNone, ""),
	writer("task loop clear", "task loop clear", MachineSurfaceStdout, bothStorage, WriterLockRepository, "managed ledger, selected task, task corpus", "managed ledger, selected task", WriterDurabilityNormal, WriterRecoveryNone, ""),
	reader("spec list", "spec list", MachineSurfaceStdout, bothStorage, "layout/config, specs"),
	reader("spec show", "spec show", MachineSurfaceStdout, bothStorage, "specs"),
	writer("spec add", "spec add", MachineSurfaceStdout, bothStorage, WriterLockRepository, "layout/config, managed ledger", "spec", WriterDurabilityNormal, WriterRecoveryNone, ""),
	reader("spec diff", "spec diff", MachineSurfaceStdout, bothStorage, "specs, task corpus"),
	writer("spec activate", "spec activate", MachineSurfaceStdout, bothStorage, WriterLockRepository, "managed ledger, specs", "managed ledger", WriterDurabilityNormal, WriterRecoveryNone, ""),
	reader("import preview", "import", MachineSurfaceStdout, bothStorage, "external source, layout/config, specs"),
	writer("import v1 apply", "import", MachineSurfaceStdout, bothStorage, WriterLockRepository, "external source, layout/config, managed ledger, task corpus, specs", "task directory", WriterDurabilityNormal, WriterRecoveryNone, ""),
	writer("import v2 apply", "import", MachineSurfaceStdout, bothStorage, WriterLockRepository, "external source, layout/config, managed ledger, task corpus, specs, prompt template snapshot, prompt configuration snapshot", "task directory", WriterDurabilityDurable, WriterRecoveryDurable, "durable transaction fence"),
	reader("prompt list", "prompt list", MachineSurfaceStdout, bothStorage, "layout/config"),
	reader("prompt show", "prompt show", MachineSurfaceStdout, bothStorage, "layout/config, prompt template snapshot"),
	reader("prompt render", "prompt render", MachineSurfaceStdout, bothStorage, "layout/config, prompt template snapshot, task corpus, specs"),
	writer("review publish task", "review publish", MachineSurfaceStdout, bothStorage, WriterLockRepository, "proposal, selected task, specs, prompt template snapshot, prompt configuration snapshot", "review directory", WriterDurabilityDirectory, WriterRecoveryDirectory, "no-clobber directory rename"),
	writer("review publish spec", "review publish", MachineSurfaceStdout, bothStorage, WriterLockRepository, "proposal, specs, prompt template snapshot, prompt configuration snapshot", "review directory", WriterDurabilityDirectory, WriterRecoveryDirectory, "no-clobber directory rename"),
	writer("review publish decomposition", "review publish", MachineSurfaceStdout, bothStorage, WriterLockRepository, "proposal, specs, prompt template snapshot, prompt configuration snapshot", "review directory", WriterDurabilityDirectory, WriterRecoveryDirectory, "no-clobber directory rename"),
	writer("review publish workflow", "review publish", MachineSurfaceStdout, bothStorage, WriterLockRepository, "proposal, workflow memory, git snapshot, prompt template snapshot, prompt configuration snapshot", "workflow report, workflow index", WriterDurabilityDurable, WriterRecoveryDurable, "durable transaction fence"),
	reader("review show", "review show", MachineSurfaceStdout, bothStorage, "review directory"),
	reader("local status", "local status", MachineSurfaceStdout, bothStorage, "layout/config, local overlay"),
	reader("local path", "local path", MachineSurfaceStdout, bothStorage, "layout/config, local overlay"),
	writer("local promote", "local promote", MachineSurfaceStdout, bothStorage, WriterLockRepository, "layout/config, local overlay, managed ledger, git exclusion", "layout/config, managed ledger, git exclusion", WriterDurabilityDurable, WriterRecoveryDurable, "durable transaction fence"),
	reader("lock status", "lock status", MachineSurfaceStdout, bothStorage, "lock identity"),
	writer("lock clear", "lock clear", MachineSurfaceStdout, bothStorage, WriterLockRepository, "lock identity", "lock file", WriterDurabilityExplicit, WriterRecoveryNone, ""),
	writer("recover", "recover", MachineSurfaceStdout, bothStorage, WriterLockRepository, "transaction evidence, lock identity", "transaction recovery", WriterDurabilityDurable, WriterRecoveryDurable, "durable transaction fence"),
	reader("loop dry-run", "loop", MachineSurfaceStdout, []string{"committed"}, "layout/config, managed ledger, task corpus, loop policy"),
	writer("loop result publication", "loop", MachineSurfaceResultFile, []string{"committed"}, WriterLockLoop, "result destination, layout/config, managed ledger, task corpus", "result file", WriterDurabilityStreamed, WriterRecoveryExternal, "no-clobber external result file"),
}

var bothStorage = []string{"committed", "local"}

func writer(owner, command string, surface MachineSurface, storage []string, lock, consumes, publishes string, durability WriterDurability, recovery WriterRecovery, phase string) WriterTransaction {
	return WriterTransaction{Owner: owner, Command: command, Surface: surface, Storage: slices.Clone(storage), Lock: lock,
		Consumes: splitMatrixSet(consumes), Publishes: splitMatrixSet(publishes), Durability: durability, Recovery: recovery, PhaseEvidence: phase}
}

func reader(owner, command string, surface MachineSurface, storage []string, consumes string) WriterTransaction {
	return writer(owner, command, surface, storage, WriterLockNone, consumes, "none", WriterDurabilityReadOnly, WriterRecoveryNone, "")
}

func splitMatrixSet(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ", ") {
		values = append(values, item)
	}
	return values
}

// WriterTransactionMatrix returns a defensive copy of the complete v0.5 command
// matrix. Validation deliberately uses this matrix instead of a parallel docs
// table so newly registered command surfaces cannot become unclassified.
func WriterTransactionMatrix() []WriterTransaction {
	entries := make([]WriterTransaction, len(writerTransactionMatrix))
	for i, entry := range writerTransactionMatrix {
		entries[i] = entry
		entries[i].Storage = slices.Clone(entry.Storage)
		entries[i].Consumes = slices.Clone(entry.Consumes)
		entries[i].Publishes = slices.Clone(entry.Publishes)
		entries[i].Implementation = slices.Clone(writerImplementations[entry.Owner])
	}
	return entries
}

func validateWriterTransactionMatrix(entries []WriterTransaction) error {
	owners := map[string]struct{}{}
	covered := map[string]bool{}
	for _, entry := range entries {
		if entry.Owner == "" || entry.Command == "" {
			return fmt.Errorf("writer matrix entry has no owner or command")
		}
		if _, exists := owners[entry.Owner]; exists {
			return fmt.Errorf("writer matrix duplicates ownership for %q", entry.Owner)
		}
		owners[entry.Owner] = struct{}{}
		if entry.Surface != MachineSurfaceStdout && entry.Surface != MachineSurfaceResultFile {
			return fmt.Errorf("writer matrix %q has unknown surface %q", entry.Owner, entry.Surface)
		}
		if len(entry.Storage) == 0 || len(entry.Consumes) == 0 || len(entry.Publishes) == 0 {
			return fmt.Errorf("writer matrix %q omits a required set", entry.Owner)
		}
		if err := validateMatrixSet(entry.Owner, "storage", entry.Storage, matrixStorage); err != nil {
			return err
		}
		if err := validateMatrixSet(entry.Owner, "consumed", entry.Consumes, matrixInputs); err != nil {
			return err
		}
		if err := validateMatrixSet(entry.Owner, "published", entry.Publishes, matrixSinks); err != nil {
			return err
		}
		if err := validateWriterBoundary(entry); err != nil {
			return err
		}
		covered[entry.Command+"\x00"+string(entry.Surface)] = true
	}
	for _, machine := range MachineCommandInventory() {
		if !covered[machine.Command+"\x00"+string(machine.Surface)] {
			return fmt.Errorf("machine command %q has no writer matrix classification", machine.CompanionRow)
		}
	}
	return nil
}

func validateWriterBoundary(entry WriterTransaction) error {
	switch entry.Durability {
	case WriterDurabilityReadOnly:
		if entry.Lock != WriterLockNone || !slices.Equal(entry.Publishes, []string{"none"}) || entry.Recovery != WriterRecoveryNone {
			return fmt.Errorf("read-only matrix entry %q declares a write boundary", entry.Owner)
		}
	case WriterDurabilityExplicit:
		if entry.Lock != WriterLockRepository || entry.Recovery != WriterRecoveryNone {
			return fmt.Errorf("explicit matrix entry %q has an invalid lock or recovery owner", entry.Owner)
		}
	case WriterDurabilityNormal:
		if entry.Lock != WriterLockRepository || entry.Recovery != WriterRecoveryNone || entry.PhaseEvidence != "" {
			return fmt.Errorf("normal writer %q claims crash recovery or lacks the repository lock", entry.Owner)
		}
	case WriterDurabilityDurable:
		if entry.Lock != WriterLockRepository || entry.Recovery != WriterRecoveryDurable || entry.PhaseEvidence == "" {
			return fmt.Errorf("durable writer %q lacks recoverable phase evidence", entry.Owner)
		}
	case WriterDurabilityDirectory:
		if entry.Lock != WriterLockRepository || entry.Recovery != WriterRecoveryDirectory || entry.PhaseEvidence == "" {
			return fmt.Errorf("directory writer %q lacks its no-clobber commit boundary", entry.Owner)
		}
	case WriterDurabilityStreamed:
		if entry.Lock != WriterLockLoop || entry.Recovery != WriterRecoveryExternal || entry.PhaseEvidence == "" {
			return fmt.Errorf("streamed writer %q lacks external publication evidence", entry.Owner)
		}
	default:
		return fmt.Errorf("writer matrix %q has unknown durability %q", entry.Owner, entry.Durability)
	}
	if strings.HasPrefix(entry.Owner, "review publish") && (!slices.Contains(entry.Consumes, "prompt template snapshot") || !slices.Contains(entry.Consumes, "prompt configuration snapshot")) {
		return fmt.Errorf("review publisher %q omits prompt/config snapshots", entry.Owner)
	}
	return nil
}

func validateMatrixSet(owner, name string, values []string, allowed map[string]struct{}) error {
	seen := map[string]struct{}{}
	for _, value := range values {
		if _, ok := allowed[value]; !ok {
			return fmt.Errorf("writer matrix %q has unregistered %s %q", owner, name, value)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("writer matrix %q repeats %s %q", owner, name, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

var matrixStorage = matrixSet("committed", "local")

var matrixInputs = matrixSet(
	"external source", "git exclusion", "git snapshot", "layout config", "layout/config", "local overlay", "lock identity", "loop policy", "managed ledger", "proposal", "prompt configuration snapshot", "prompt template snapshot", "result destination", "review directory", "selected task", "specs", "task corpus", "transaction evidence", "workflow memory",
)

var matrixSinks = matrixSet(
	"git exclusion", "layout/config", "local overlay", "lock file", "managed ledger", "none", "result file", "review directory", "selected task", "spec", "task", "task directory", "transaction recovery", "verification artifact", "workflow index", "workflow report",
)

var writerImplementations = map[string][]string{
	"init":                         {"repotx.Commit:commitInitTransaction"},
	"init layout migration":        {"durabletx.Run:applyLayout2Upgrade"},
	"init local":                   {"durabletx.Run:applyLocalInit"},
	"init local skill refresh":     {"durabletx.Run:refreshLocalSkills"},
	"retrofit":                     {"repotx.Commit:commitInitTransaction"},
	"repair":                       {"repotx.Commit:commitStructuralWriter"},
	"next":                         {"repotx.Commit:commitLifecycle"},
	"start":                        {"repotx.Commit:commitLifecycle"},
	"complete":                     {"repotx.Commit:commitLifecycle"},
	"block":                        {"repotx.Commit:commitLifecycle"},
	"unblock":                      {"repotx.Commit:commitLifecycle"},
	"verify":                       {"repotx.Commit:commitVerify"},
	"task new":                     {"repotx.Commit:commitTaskWriter"},
	"task rename":                  {"repotx.Commit:commitTaskWriter"},
	"task repoint":                 {"repotx.Commit:commitTaskWriter"},
	"task release":                 {"repotx.Commit:commitLifecycle"},
	"task author":                  {"repotx.Commit:TaskAuthor"},
	"task dependency add":          {"repotx.Commit:commitTaskWriter"},
	"task dependency remove":       {"repotx.Commit:commitTaskWriter"},
	"task loop allow":              {"repotx.Commit:commitTaskWriter"},
	"task loop hold":               {"repotx.Commit:commitTaskWriter"},
	"task loop clear":              {"repotx.Commit:commitTaskWriter"},
	"spec add":                     {"repotx.Commit:commitStructuralWriter"},
	"spec activate":                {"repotx.Commit:commitStructuralWriter"},
	"import v1 apply":              {"repotx.Commit:ApplyImportDraft"},
	"import v2 apply":              {"durabletx.Run:applyReviewedImportDraftLocked"},
	"review publish task":          {"reviewdir.Publish:reviewPublishTask"},
	"review publish spec":          {"reviewdir.Publish:reviewPublishSpec"},
	"review publish decomposition": {"reviewdir.Publish:publishDecompositionReview"},
	"review publish workflow":      {"durabletx.Run:publishWorkflowReview"},
	"local promote":                {"durabletx.Run:localPromoteSemantic", "durabletx.Run:localPromotePendingSkills"},
}

func matrixSet(values ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}
