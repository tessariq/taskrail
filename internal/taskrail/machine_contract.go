package taskrail

import (
	"fmt"
	"regexp"
	"slices"
)

// The v0.5 machine API has two normative sources: `specs/v0.5.0.md` owns command
// behavior and `specs/contracts/v0.5.0-machine-api.md` owns wire shape. This file
// is the single executable projection of that pair, so producers, strict
// decoders, and drift enforcement read one inventory instead of re-deriving the
// contract from implementation structs, help text, examples, or skills.

// MachineSchemaVersion is the one envelope schema version v0.5 emits, for every
// command and for the loop result file.
const MachineSchemaVersion = 1

// MachineSurface separates the two documents a command can publish. Loop
// execution streams child output, so its final envelope reaches an agent through
// the result file rather than stdout.
type MachineSurface string

const (
	MachineSurfaceStdout     MachineSurface = "stdout"
	MachineSurfaceResultFile MachineSurface = "result-file"
)

// MachineCommandOrigin records whether the current CLI already constructs the
// command path. It describes the binary, not the contract: a constructed command
// may still predate the common envelope, and a planned one is equally normative.
type MachineCommandOrigin string

const (
	MachineOriginConstructed MachineCommandOrigin = "constructed"
	MachineOriginPlanned     MachineCommandOrigin = "planned"
)

// MachineCommandEntry is one schema-version-1 inventory entry.
type MachineCommandEntry struct {
	// CompanionRow is the companion registry's command cell verbatim, so a loop
	// row that qualifies one canonical path stays distinguishable.
	CompanionRow  string
	Command       string
	Surface       MachineSurface
	Origin        MachineCommandOrigin
	Results       []string
	NonzeroResult string
	Warnings      []string
	Errors        []string
}

// canonicalCommandPath is the companion's `command` grammar: lower-case tokens
// joined by one ASCII space, without executable name, flags, `--`, or operands.
var canonicalCommandPath = regexp.MustCompile(`^[a-z][a-z-]*( [a-z][a-z-]*)*$`)

// The closed registries. Both are byte-checked against their normative source.
var (
	machineWarningCodes = []string{
		"unknown_skill_version", "skill_version_skew", "empty_derived_slug",
		"selected_off_spec", "selected_non_active_spec", "skipped_non_active_spec",
		"local_initialized", "local_head_drift", "verify_pass_before_complete",
	}
	machineErrorCodes = []string{
		"invalid_arguments", "not_initialized", "incompatible_layout",
		"migration_in_progress", "repository_invalid", "validation_failed",
		"task_not_found", "review_not_found", "invalid_status", "invalid_reason",
		"invalid_digest", "source_changed", "invalid_proposal", "destination_exists",
		"path_blocked", "git_state", "lock_held", "delegated_write_refused",
		"prompt_not_found", "prompt_invalid", "policy_invalid", "dependency_exists",
		"dependency_absent", "dependency_cycle", "cancelled_dependency",
		"write_conflict", "recovery_pending", "partial_write", "rollback_failed",
		"unsupported", "result_file_publish_failed", "blocked_fail", "rework_fail",
		"completed_unverified", "completed_audit_fail", "child_failed",
		"no_progress", "invalid_postflight",
	}
)

// The companion's exact reusable error sets. Every command's subset is one of
// these, optionally unioned with the extra codes the assignment table names, so
// no entry invents an applicability rule of its own.
var (
	readErrors = []string{
		"invalid_arguments", "not_initialized", "incompatible_layout",
		"migration_in_progress", "repository_invalid", "recovery_pending", "unsupported",
	}
	writerErrors = mergeCodes(readErrors, []string{
		"validation_failed", "lock_held", "delegated_write_refused",
		"write_conflict", "partial_write", "rollback_failed",
	})
	lifecycleErrors         = mergeCodes(writerErrors, []string{"task_not_found", "invalid_status"})
	reasonedLifecycleErrors = mergeCodes(lifecycleErrors, []string{"invalid_reason"})
	taskReleaseErrors       = []string{
		"invalid_arguments", "not_initialized", "incompatible_layout",
		"migration_in_progress", "task_not_found", "invalid_status", "invalid_reason",
		"repository_invalid", "lock_held", "delegated_write_refused", "write_conflict",
		"recovery_pending", "partial_write", "rollback_failed",
	}
	taskAuthorErrors = []string{
		"invalid_arguments", "not_initialized", "incompatible_layout",
		"migration_in_progress", "task_not_found", "invalid_status", "invalid_digest",
		"invalid_proposal", "lock_held", "write_conflict", "recovery_pending",
		"partial_write", "rollback_failed",
	}
	dependencyErrors = mergeCodes(writerErrors, []string{
		"task_not_found", "invalid_status", "dependency_exists",
		"dependency_absent", "dependency_cycle", "cancelled_dependency",
	})
	reviewErrors = []string{
		"invalid_arguments", "not_initialized", "incompatible_layout",
		"migration_in_progress", "repository_invalid", "invalid_digest", "source_changed",
		"invalid_proposal", "prompt_invalid", "destination_exists", "path_blocked",
		"lock_held", "write_conflict", "recovery_pending", "partial_write", "rollback_failed",
	}
	contentWriterErrors = mergeCodes(writerErrors, []string{
		"invalid_digest", "source_changed", "destination_exists", "path_blocked", "git_state",
	})
	loopDryRunErrors = mergeCodes(readErrors, []string{
		"validation_failed", "git_state", "lock_held", "prompt_not_found",
		"prompt_invalid", "policy_invalid", "invalid_digest",
	})
	loopExecutionPreflightErrors = mergeCodes(loopDryRunErrors, []string{"destination_exists", "path_blocked"})
	loopPostflightErrors         = []string{
		"blocked_fail", "rework_fail", "completed_unverified",
		"completed_audit_fail", "child_failed", "no_progress", "invalid_postflight",
	}
)

// The companion's exact warning assignment. Skill skew accompanies every
// registry command after repository discovery, and head drift accompanies every
// initialized local-mode command except `local status`, which reports drift in
// its result instead.
var (
	warnsAmbient   = []string{"unknown_skill_version", "skill_version_skew"}
	warnsDrift     = []string{"local_head_drift"}
	warnsBootstrap = []string{"local_initialized"}
	warnsSlug      = []string{"empty_derived_slug"}
	warnsSelection = []string{"selected_off_spec", "selected_non_active_spec", "skipped_non_active_spec"}
)

// errs unions one reusable set with the extra codes the assignment table names.
func errs(base []string, extra ...string) []string {
	return mergeCodes(base, extra)
}

func warns(groups ...[]string) []string {
	return mergeCodes(append([][]string{warnsAmbient, warnsDrift}, groups...)...)
}

// warnsNoDrift is the `local status` exception to the head-drift rule.
func warnsNoDrift() []string {
	return mergeCodes(warnsAmbient)
}

func mergeCodes(groups ...[]string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, group := range groups {
		for _, code := range group {
			if _, exists := seen[code]; exists {
				continue
			}
			seen[code] = struct{}{}
			out = append(out, code)
		}
	}
	slices.Sort(out)
	return out
}

// machineEntry keeps the inventory table readable: the companion owns row,
// results, and the nonzero-result exception, so those three stay verbatim at the
// call site.
func machineEntry(row, command string, origin MachineCommandOrigin, results []string, nonzero string, warnings, errors []string) MachineCommandEntry {
	return MachineCommandEntry{
		CompanionRow:  row,
		Command:       command,
		Surface:       MachineSurfaceStdout,
		Origin:        origin,
		Results:       results,
		NonzeroResult: nonzero,
		Warnings:      warnings,
		Errors:        errors,
	}
}

func built(row, command string, results []string, nonzero string, warnings, errors []string) MachineCommandEntry {
	return machineEntry(row, command, MachineOriginConstructed, results, nonzero, warnings, errors)
}

func planned(row, command string, results []string, nonzero string, warnings, errors []string) MachineCommandEntry {
	return machineEntry(row, command, MachineOriginPlanned, results, nonzero, warnings, errors)
}

// machineInventory follows the companion registry's order, which is the one
// deterministic ordering a consumer can re-derive from the normative source.
var machineInventory = []MachineCommandEntry{
	built("`init`", "init", []string{"InitResult"}, "never",
		warns(), errs(contentWriterErrors)),
	built("`retrofit`", "retrofit", []string{"RetrofitResult", "EmitPromptResult"}, "never",
		warns(), errs(contentWriterErrors)),
	built("`validate`", "validate", []string{"ValidateResult"}, "`valid:false`",
		warns(), errs(readErrors)),
	built("`repair`", "repair", []string{"RepairResult"}, "never",
		warns(warnsBootstrap), errs(writerErrors, "destination_exists", "path_blocked")),
	built("`coverage`", "coverage", []string{"CoverageReport", "GapReport"}, "selected `--min`/`--fail-on` gate",
		warns(), errs(readErrors)),
	built("`status`", "status", []string{"StatusResult"}, "never",
		warns(warnsSelection), errs(readErrors)),
	built("`stats`", "stats", []string{"StatsResult"}, "never",
		warns(), errs(readErrors)),
	built("`next`", "next", []string{"NextResult"}, "never",
		warns(warnsBootstrap, warnsSelection), errs(writerErrors, "destination_exists", "path_blocked")),
	built("`start`", "start", []string{"StartResult"}, "never",
		warns(warnsBootstrap), errs(lifecycleErrors)),
	built("`complete`", "complete", []string{"CompleteResult"}, "never",
		warns(warnsBootstrap), errs(lifecycleErrors)),
	built("`block`", "block", []string{"BlockResult"}, "never",
		warns(warnsBootstrap), errs(reasonedLifecycleErrors)),
	built("`unblock`", "unblock", []string{"UnblockResult"}, "never",
		warns(warnsBootstrap), errs(lifecycleErrors)),
	built("`verify`", "verify", []string{"VerifyResult"}, "never",
		warns(warnsBootstrap, []string{"verify_pass_before_complete"}),
		errs(reasonedLifecycleErrors, "destination_exists", "path_blocked")),
	built("`task new`", "task new", []string{"TaskNewResult"}, "never",
		warns(warnsBootstrap, warnsSlug), errs(writerErrors, "destination_exists", "path_blocked")),
	built("`task rename`", "task rename", []string{"TaskRenameResult"}, "never",
		warns(warnsBootstrap, warnsSlug),
		errs(lifecycleErrors, "source_changed", "destination_exists", "path_blocked")),
	built("`task repoint`", "task repoint", []string{"TaskRepointResult"}, "never",
		warns(warnsBootstrap), errs(lifecycleErrors, "path_blocked")),
	planned("`task release`", "task release", []string{"TaskReleaseResult"}, "never",
		warns(warnsBootstrap), errs(taskReleaseErrors)),
	planned("`task show`", "task show", []string{"TaskShowResult"}, "never",
		warns(), errs(readErrors, "task_not_found")),
	planned("`task author`", "task author", []string{"TaskAuthorResult"}, "never",
		warns(warnsBootstrap), errs(taskAuthorErrors)),
	planned("`task dependency add`", "task dependency add", []string{"DependencyResult"}, "never",
		warns(), errs(dependencyErrors)),
	planned("`task dependency remove`", "task dependency remove", []string{"DependencyResult"}, "never",
		warns(), errs(dependencyErrors)),
	planned("`task loop list`", "task loop list", []string{"TaskLoopListResult"}, "non-empty `violations`",
		warns(), errs(readErrors)),
	planned("`task loop allow`", "task loop allow", []string{"LoopPolicyMutationResult"}, "never",
		warns(warnsBootstrap), errs(lifecycleErrors, "invalid_reason", "policy_invalid")),
	planned("`task loop hold`", "task loop hold", []string{"LoopPolicyMutationResult"}, "never",
		warns(warnsBootstrap), errs(lifecycleErrors, "invalid_reason", "policy_invalid")),
	planned("`task loop clear`", "task loop clear", []string{"LoopPolicyMutationResult"}, "never",
		warns(warnsBootstrap), errs(lifecycleErrors, "invalid_reason", "policy_invalid")),
	built("`spec list`", "spec list", []string{"SpecListResult"}, "never",
		warns(), errs(readErrors)),
	built("`spec show`", "spec show", []string{"SpecShowResult"}, "never",
		warns(), errs(readErrors, "path_blocked")),
	built("`spec add`", "spec add", []string{"SpecAddResult"}, "never",
		warns(warnsBootstrap), errs(writerErrors, "destination_exists", "path_blocked")),
	built("`spec diff`", "spec diff", []string{"SpecDiffResult"}, "never",
		warns(), errs(readErrors)),
	built("`spec activate`", "spec activate", []string{"SpecActivateResult"}, "never",
		warns(warnsBootstrap), errs(writerErrors, "destination_exists", "path_blocked")),
	built("`import`", "import", []string{"ImportPreviewResult", "EmitPromptResult", "ImportV1ApplyResult", "ImportV2ApplyResult"}, "never",
		warns(warnsBootstrap, warnsSlug), errs(contentWriterErrors, "invalid_proposal", "prompt_invalid")),
	planned("`prompt list`", "prompt list", []string{"PromptListResult"}, "never",
		warns(), errs(readErrors, "prompt_invalid")),
	planned("`prompt show`", "prompt show", []string{"PromptContentResult"}, "never",
		warns(), errs(readErrors, "prompt_not_found", "prompt_invalid", "task_not_found", "path_blocked")),
	planned("`prompt render`", "prompt render", []string{"PromptContentResult"}, "never",
		warns(), errs(readErrors, "prompt_not_found", "prompt_invalid", "task_not_found", "path_blocked")),
	planned("`review publish`", "review publish", []string{"ReviewPublishResult"}, "never",
		warns(), errs(reviewErrors)),
	planned("`review show`", "review show", []string{"ReviewShowResult"}, "never",
		warns(), errs(readErrors, "review_not_found", "path_blocked")),
	planned("`local status`", "local status", []string{"LocalStatusResult"}, "never",
		warnsNoDrift(), errs(readErrors)),
	planned("`local path`", "local path", []string{"LocalPathResult"}, "never",
		warns(), errs(readErrors)),
	planned("`local promote`", "local promote", []string{"LocalPromoteResult"}, "never",
		warns(), errs(contentWriterErrors)),
	planned("`lock status`", "lock status", []string{"LockStatusResult"}, "never",
		warns(), errs(readErrors)),
	planned("`lock clear`", "lock clear", []string{"LockClearResult"}, "never",
		warns(), errs(readErrors, "invalid_digest", "source_changed", "lock_held", "write_conflict")),
	planned("`recover`", "recover", []string{"RecoverResult"}, "never",
		warns(), errs(writerErrors, "invalid_digest", "source_changed")),
	planned("`loop` dry-run", "loop", []string{"LoopDryRunResult"}, "`action:invalid`",
		warns(), errs(loopDryRunErrors)),
	loopResultFileEntry(),
}

// loopResultFileEntry is the only non-stdout document. It carries three
// assignment rows at once: execution preflight refusals, the publication failure
// code, and the postflight outcome codes, whose diagnostic replaces the common
// error details.
func loopResultFileEntry() MachineCommandEntry {
	e := planned("`loop` result file", "loop", []string{"LoopDiagnostic"}, "failures use `error`",
		warns(warnsBootstrap),
		mergeCodes(loopExecutionPreflightErrors, []string{"result_file_publish_failed"}, loopPostflightErrors))
	e.Surface = MachineSurfaceResultFile
	return e
}

// MachineCommandInventory returns the complete inventory in companion order.
func MachineCommandInventory() []MachineCommandEntry {
	out := make([]MachineCommandEntry, len(machineInventory))
	for i, e := range machineInventory {
		out[i] = copyMachineEntry(e)
	}
	return out
}

// MachineCommandEntryFor resolves one entry by canonical command path and the
// document surface it publishes.
func MachineCommandEntryFor(command string, surface MachineSurface) (MachineCommandEntry, bool) {
	for _, e := range machineInventory {
		if e.Command == command && e.Surface == surface {
			return copyMachineEntry(e), true
		}
	}
	return MachineCommandEntry{}, false
}

// MachineWarningCodes returns the closed warning union.
func MachineWarningCodes() []string {
	return slices.Clone(machineWarningCodes)
}

// MachineErrorCodes returns the closed error-code registry.
func MachineErrorCodes() []string {
	return slices.Clone(machineErrorCodes)
}

func copyMachineEntry(e MachineCommandEntry) MachineCommandEntry {
	e.Results = slices.Clone(e.Results)
	e.Warnings = slices.Clone(e.Warnings)
	e.Errors = slices.Clone(e.Errors)
	return e
}

// ValidateMachineInventory enforces the properties a schema decoder relies on:
// one entry per published document, canonical command paths, and codes drawn
// only from the two closed registries.
func ValidateMachineInventory() error {
	warningCodes := codeSet(machineWarningCodes)
	errorCodes := codeSet(machineErrorCodes)
	seenRow := map[string]struct{}{}
	seenDocument := map[string]struct{}{}
	for _, e := range machineInventory {
		if _, exists := seenRow[e.CompanionRow]; exists {
			return fmt.Errorf("inventory repeats companion row %q", e.CompanionRow)
		}
		seenRow[e.CompanionRow] = struct{}{}
		document := e.Command + " " + string(e.Surface)
		if _, exists := seenDocument[document]; exists {
			return fmt.Errorf("inventory repeats document %q", document)
		}
		seenDocument[document] = struct{}{}
		if !canonicalCommandPath.MatchString(e.Command) {
			return fmt.Errorf("command %q is not a canonical command path", e.Command)
		}
		if e.Surface != MachineSurfaceStdout && e.Surface != MachineSurfaceResultFile {
			return fmt.Errorf("command %q has unknown surface %q", e.Command, e.Surface)
		}
		if e.Origin != MachineOriginConstructed && e.Origin != MachineOriginPlanned {
			return fmt.Errorf("command %q has unknown origin %q", e.Command, e.Origin)
		}
		if len(e.Results) == 0 {
			return fmt.Errorf("command %q names no result shape", e.Command)
		}
		if e.NonzeroResult == "" {
			return fmt.Errorf("command %q does not state its report-result exit exception", e.Command)
		}
		if err := checkCodes(e.Command, "warning", e.Warnings, warningCodes); err != nil {
			return err
		}
		if err := checkCodes(e.Command, "error", e.Errors, errorCodes); err != nil {
			return err
		}
	}
	return nil
}

func checkCodes(command, kind string, codes []string, registry map[string]struct{}) error {
	if len(codes) == 0 {
		return fmt.Errorf("command %q names no %s subset", command, kind)
	}
	for i, code := range codes {
		if _, ok := registry[code]; !ok {
			return fmt.Errorf("command %q names %s code %q outside the closed registry", command, kind, code)
		}
		if i > 0 && codes[i-1] >= code {
			return fmt.Errorf("command %q %s subset is not sorted and unique at %q", command, kind, code)
		}
	}
	return nil
}

func codeSet(codes []string) map[string]struct{} {
	set := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		set[code] = struct{}{}
	}
	return set
}
