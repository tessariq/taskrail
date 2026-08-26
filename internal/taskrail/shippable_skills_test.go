package taskrail

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// shippableSkillsDir holds the repo-agnostic skill set embedded and installed by
// `taskrail init --with-skills` (T-030), resolved relative to this package. Since
// T-055 it is the single skill source: the bespoke repo-root skills/ tree was
// retired and this repository adopts the packaged set like any adopter.
const shippableSkillsDir = "skills"

// shippableSkills is the exact set promoted to the product surface, per the
// portability contract in docs/workflow/skills-productization.md (T-031).
var shippableSkills = []string{
	"autonomous-backlog",
	"autonomous-task",
	"autonomous-verify",
	"autonomous-recovery",
	"autonomous-manual-test",
	"taskrail-loop",
	"taskrail-import",
	"taskrail-retrofit",
	"taskrail-repair",
	"taskrail-spec",
	"taskrail-spec-review",
	"taskrail-decompose",
	"taskrail-sdd-handoff",
	"taskrail-gap",
	"taskrail-task-review",
	"taskrail-workflow-adversarial",
}

// taskAuthoringSkills create tracked tasks via `taskrail task new`. taskrail-import
// and taskrail-retrofit are excluded: they author tasks through
// `taskrail import --apply`, covered by TestImportSkillInvokesImportCommand and
// TestRetrofitSkillDrivesGuidedFlow.
var taskAuthoringSkills = []string{
	"autonomous-backlog",
	"autonomous-task",
	"autonomous-verify",
	"taskrail-spec",
}

// dogfoodingOnlySkills lists skills that must never leak into the shippable set.
// It is currently empty: autonomous-recovery graduated in T-054 once the widened
// repair surface removed its need to bypass the CLI, and autonomous-manual-test
// in T-081 (its manual-test artifacts stay ephemeral and gitignored rather than
// becoming a product invariant, so shipping the skill needs no new invariant).
// The guard stays so a future dogfooding-only skill re-arms it without new wiring.
var dogfoodingOnlySkills = []string{}

func shippableSkillPath(name string) string {
	return filepath.Join(shippableSkillsDir, name, "SKILL.md")
}

func readShippableSkill(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(shippableSkillPath(name))
	if err != nil {
		t.Fatalf("read shippable skill %s: %v", name, err)
	}
	return string(data)
}

// assertSkillReferences fails if the skill body omits any of the wanted
// substrings, keeping per-skill command-flow assertions to one call site.
func assertSkillReferences(t *testing.T, name string, wants ...string) {
	t.Helper()
	body := strings.Join(strings.Fields(readShippableSkill(t, name)), " ")
	for _, want := range wants {
		want = strings.Join(strings.Fields(want), " ")
		if !strings.Contains(body, want) {
			t.Errorf("%s skill must reference %q", name, want)
		}
	}
}

func TestShippableSkillsExist(t *testing.T) {
	for _, name := range shippableSkills {
		if got := readShippableSkill(t, name); strings.TrimSpace(got) == "" {
			t.Errorf("shippable skill %s is empty", name)
		}
	}
}

// The whole point of the shippable set: it invokes the installed binary, never
// `go run ./cmd/taskrail`, which only resolves inside this source tree.
func TestShippableSkillsNeverUseGoRun(t *testing.T) {
	for _, name := range shippableSkills {
		if strings.Contains(readShippableSkill(t, name), "go run") {
			t.Errorf("shippable skill %s must not reference 'go run'", name)
		}
	}
}

// Shippable skills invoke the binary through the configurable entry point
// (${TASKRAIL:-taskrail}, T-051) and never hardcode a `taskrail <cmd>` prefix,
// which would defeat the override and, in this repo, silently resolve a stale
// installed binary. Prose references to the `taskrail` binary (no trailing
// subcommand) are fine; only backtick-prefixed invocations are forbidden.
func TestShippableSkillsUseConfigurableEntryPoint(t *testing.T) {
	const entryPoint = "${TASKRAIL:-taskrail}"
	// A backtick-prefixed invocation: a code span opening on the binary name
	// followed by a subcommand. \s (not a literal space) also catches an
	// invocation that word-wraps immediately after `taskrail`. The trailing
	// whitespace is what distinguishes it from a bare `taskrail` prose
	// reference, whose closing backtick abuts the name.
	hardcoded := regexp.MustCompile("`taskrail\\s")
	for _, name := range shippableSkills {
		body := readShippableSkill(t, name)
		if !strings.Contains(body, entryPoint) {
			t.Errorf("shippable skill %s must invoke the binary via %q", name, entryPoint)
		}
		if loc := hardcoded.FindString(body); loc != "" {
			t.Errorf("shippable skill %s must not hardcode a `taskrail <cmd>` invocation (%q); use %q", name, loc, entryPoint)
		}
	}
}

func TestShippableSkillsConsumeStructuredResultsAsJSON(t *testing.T) {
	required := map[string][]string{
		"autonomous-backlog":            {"} validate --json", "} next --json", "} start <task-id> --json", "} verify <task-id> --result pass --summary \"...\" --json", "} verify <task-id> --result fail --summary \"...\" --json", "} task new --follow-up <task-id> --title \"...\" --json", "} complete <task-id> --note \"...\" --json", "} block <task-id> --reason \"...\" --json"},
		"autonomous-task":               {"} validate --json", "} start <task-id> --json", "} verify <task-id> --result pass --summary \"...\" --json", "} verify <task-id> --result fail --summary \"...\" --json", "} task new --follow-up <task-id> --title \"...\" --json", "} complete <task-id> --note \"...\" --json", "} block <task-id> --reason \"...\" --json"},
		"autonomous-verify":             {"} validate --json", "} verify <task-id> --result pass --summary \"...\" --json", "} verify <task-id> --result fail --summary \"...\" --json", "} task new --follow-up <task-id> --title \"...\" --json", "} verify <task-id> --result fail --summary \"...\" --create-followup --json"},
		"autonomous-recovery":           {"} validate --json", "} repair --json", "} repair --apply --json"},
		"autonomous-manual-test":        {"} validate --json"},
		"taskrail-loop":                 {"} loop --dry-run --json", "} loop", "--result-file <absolute-external-path>", "--parallel <n>", "--workspace-root <absolute-external-path>", "--clone-depth <n|full>", "--keep-workspaces <never|failure|always>", "--delivery <local|review>", "--review-adapter <path>", "--allow-prompt-override-sha256 <digest>", "-- <child-command> <args...>", "} lock status --json", "} recover <transaction-id>", "--take-over-lock <lock-id> --expect-sha256 <digest> --json", "--apply"},
		"taskrail-import":               {"} import --apply draft.json --json", "} validate --json"},
		"taskrail-retrofit":             {"} retrofit <notes.md> --json", "} retrofit --json", "} retrofit <notes.md> --apply --json", "} import --apply draft.json --json", "} validate --json"},
		"taskrail-repair":               {"} validate --json", "} repair --json", "} repair --apply --json"},
		"taskrail-spec":                 {"} spec list --json", "} spec show <version> --anchors --json", "} spec diff <current-version> <target-version> --json", "} spec activate <version> --json", "} task new --title \"...\" --area", "<anchor> --json", "} task repoint <id> --area <anchor> --dry-run --json", "} task repoint <id> --area <anchor> --json", "} spec add <version> --json", "} validate --json"},
		"taskrail-spec-review":          {"} spec show <version> --json", "} prompt render spec-consistency --spec <version> --review <proposal-dir>/consistency.json --json", "} review publish --type spec --proposal <proposal-dir> --destination <planning-dir>/reviews/spec/<version>/<session-id> --spec <version> --expect-spec-sha256 <digest> --json"},
		"taskrail-decompose":            {"} coverage --json", "} spec show <version> --json", "} spec show <version> --anchors --json", "} prompt render task-decomposition", "} prompt render task-decomposition-adversarial", "} review publish --type decomposition", "--dry-run --json", "} import --apply <published>/draft.json --expect-sha256 <draft-sha256> --review-manifest <published>/manifest.json --expect-review-sha256 <manifest-sha256> --json", "} validate --json"},
		"taskrail-gap":                  {"} coverage --gaps --json", "} coverage --gaps --area <anchor> --json", "} task new --title \"...\" --area <anchor> --json", "} import --apply <draft.json> --json", "} validate --json"},
		"taskrail-task-review":          {"} task show <task-id> --json", "} spec show <version> --json", "} coverage --area <spec-anchor> --json", "} prompt render task-review --task <task-id> --review <proposal>/review.json --json", "} review publish --type task --proposal <proposal> --destination <destination> --task <task-id> --expect-task-sha256 <digest> --expect-spec-sha256 <digest> --dry-run --json", "} review publish --type task --proposal <proposal> --destination <destination> --task <task-id> --expect-task-sha256 <digest> --expect-spec-sha256 <digest> --json"},
		"taskrail-workflow-adversarial": {"} status --json", "} spec show <version> --json", "} task loop list --json", "} task show <task-id> --json", "} review show <memory> --json", "} prompt render workflow-adversarial --spec <version> --memory <memory> --review <proposal>/report.json --json", "} review publish --type workflow --review <proposal>/report.json --memory <memory> --destination <destination> --spec <version> --expect-spec-sha256 <digest> --expect-head <head> --expect-product-sha256 <digest>", "--dry-run --json"},
	}
	for _, name := range shippableSkills {
		body := strings.Join(strings.Fields(readShippableSkill(t, name)), " ")
		for _, command := range required[name] {
			command = strings.Join(strings.Fields(command), " ")
			if !strings.Contains(body, command) {
				t.Errorf("%s skill must reference %q", name, command)
			}
		}
	}
}

// Full-task skills must carry the complete lifecycle guidance because they can
// start, implement, review, and close a selected task without another workflow
// prompt supplying the missing branches.
func TestFullTaskSkillsFollowCanonicalLifecycle(t *testing.T) {
	for _, name := range []string{"autonomous-backlog", "autonomous-task"} {
		t.Run(name, func(t *testing.T) {
			body := strings.Join(strings.Fields(readShippableSkill(t, name)), " ")
			for _, want := range []string{
				"Do not open logical managed paths directly.",
				"one independently meaningful observable outcome",
				"reviewed decomposition or clarification",
				"Do not rewrite scope after lifecycle work begins.",
				"failing test whenever practical",
				"sandbox-first manual testing",
				"one fresh reviewer by default",
				"one to three reviewers, each with a named, non-duplicative lens",
				"`fix-now`, `separate-followup`, `blocked`, or `rejected`",
				"Repair all current-scope findings.",
				"second broad round is allowed only",
				"final-diff review",
				"objective closure evidence permits completion",
				"completed-unverified",
				"completed-audit-fail",
				"Never repeat completion or compensate with block.",
				"committed mode",
				"local mode",
				"status --json",
				"never force-add ignored Taskrail metadata",
				"task release",
				"selected-task `verify --create-followup`",
				"never invoke `local promote`",
				"remain caller-owned",
			} {
				if !strings.Contains(body, want) {
					t.Errorf("%s skill must describe %q", name, want)
				}
			}

			complete := strings.Index(body, "complete <task-id> --note \"...\" --json")
			pass := strings.Index(body, "verify <task-id> --result pass --summary \"...\" --json")
			if complete < 0 || pass < 0 || complete > pass {
				t.Errorf("%s skill must complete before passing verification", name)
			}
			block := strings.Index(body, "block <task-id> --reason \"...\" --json")
			fail := strings.Index(body, "verify <task-id> --result fail --summary \"...\" --json")
			if block < 0 || fail < 0 || block > fail {
				t.Errorf("%s skill must block before failing verification", name)
			}

			cannotProceedStart := strings.Index(body, "If work cannot proceed")
			reworkStart := strings.Index(body, "For deliberate rework")
			if cannotProceedStart < 0 || reworkStart < 0 || cannotProceedStart > reworkStart {
				t.Errorf("%s skill must define distinct cannot-proceed and rework branches", name)
			} else {
				cannotProceed := body[cannotProceedStart:reworkStart]
				cannotProceedBlock := strings.Index(cannotProceed, "block <task-id> --reason \"...\" --json")
				cannotProceedFail := strings.Index(cannotProceed, "verify <task-id> --result fail --summary \"...\" --json")
				if cannotProceedBlock < 0 || cannotProceedFail < 0 || cannotProceedBlock > cannotProceedFail {
					t.Errorf("%s skill must block then fail verification in its cannot-proceed branch", name)
				}
				if strings.Contains(cannotProceed, "complete <task-id>") {
					t.Errorf("%s skill must not complete in its cannot-proceed branch", name)
				}
				if !strings.Contains(cannotProceed, "check its exit") || !strings.Contains(cannotProceed, "and stop.") {
					t.Errorf("%s skill must check and stop after its cannot-proceed branch", name)
				}
			}

			if !strings.Contains(body, "Immediately before every state writer, apply the source-checkout guard") {
				t.Errorf("%s skill must require a source-checkout guard immediately before every writer", name)
			}
			if strings.Contains(body, ".taskrail/local") {
				t.Errorf("%s skill must not derive a local managed path", name)
			}
		})
	}
}

// The loop skill is the interactive parent supervisor, not another coordinator.
// Its instructions must bind every mutating launch to a reviewed dry-run and
// leave selection, lifecycle, integration, delivery, and recovery writes to the
// Taskrail command or an explicitly caller-owned adapter.
func TestTaskrailLoopSkillSupervisesOneConfirmedInvocation(t *testing.T) {
	assertSkillReferences(t, "taskrail-loop",
		"safe defaults",
		"missing provider command",
		"dry-run",
		"explicit confirmation",
		"result file",
		"exactly once",
		"deterministic rank",
		"coordinator",
		"does not select a task",
		"does not cherry-pick",
		"does not stage",
		"does not commit",
		"does not merge",
		"does not push",
		"not_checked",
		"checks: pass",
		"caller-owned adapter",
		"external evidence",
		"action: run",
		"action:none",
		"action:invalid",
		"A `result` carries the terminal diagnostic",
		"A postflight `error` carries that diagnostic under `error.details`",
		"error.details",
		"common error",
		"may include a recovery record",
		"`error.details.recovery`",
		"malformed",
		"absent file",
		"lock status",
		"take-over-lock",
		"completed_unverified",
		"no retry",
		"fresh dry-run",
		"new result destination",
		"no background wait",
		"never automatically relaunch",
	)
}

func TestShippableSkillsNameExactTextExceptions(t *testing.T) {
	exceptions := map[string]string{
		"taskrail-spec":     "spec show <version>` is an exact-text exception",
		"taskrail-import":   "--emit-prompt` is an exact-text exception",
		"taskrail-retrofit": "--emit-prompt` is an exact-text exception",
	}
	for name, exception := range exceptions {
		assertSkillReferences(t, name, exception)
	}
}

func TestDecomposeSkillDefinesReviewedV2Flow(t *testing.T) {
	assertSkillReferences(t, "taskrail-decompose",
		"ImportDraft v2",
		"final post-spec manifest",
		"If the selected spec is active",
		"If it is inactive, enumerate its live anchors",
		"inactive session is draft/review-only",
		"Every normative requirement has one quote or line-range source",
		"at most two passes",
		"fresh-context",
		"exact SHA-256",
		"source freshness",
		"prompt resolution changes",
		"abandons the session",
		"one outcome",
		"do-not-split",
		"integrated-behavior owner",
		"durable-oracle",
		"implicitly held",
		"Preview publication",
		"Never apply proposal bytes",
	)
}

func TestSDDHandoffSkillPreservesConservativeBoundary(t *testing.T) {
	const name = "taskrail-sdd-handoff"
	embedded := embeddedSkillFiles(t)
	prefix := name + "/references/"
	references := map[string]bool{}
	for path := range embedded {
		if strings.HasPrefix(path, prefix) {
			references[strings.TrimPrefix(path, prefix)] = true
		}
	}
	if len(references) != 2 || !references["openspec.md"] || !references["spec-kit.md"] {
		t.Errorf("%s references = %v, want exactly openspec.md and spec-kit.md", name, references)
	}

	assertSkillReferences(t, name,
		"operator-selected local artifact set",
		"content rather than directory names",
		"assumptions",
		"unresolved decisions",
		"stop for operator review",
		"taskrail-spec",
		"taskrail-import",
		"taskrail-decompose",
		"spec show <version> --anchors --json",
		"omit `loop_policy` and `loop_reason`",
		"implicitly held",
		"no automatic apply",
		"does not prove provenance, approval, completeness, synchronization, change detection, round-trip fidelity, or continuing ownership",
	)

	assertSkillReferences(t, name,
		"does not add a binary adapter, provider API, synchronization service, provenance store, or format conversion",
	)
}

func TestSDDHandoffSkillRefusesAmbiguousEvidence(t *testing.T) {
	const name = "taskrail-sdd-handoff"
	assertSkillReferences(t, name,
		"filename that suggests approval",
		"Stop for operator review when approval, ownership, conflicting requirements or requirement meaning, semantic sizing under the T-251 rubric (including whether to split or merge), integration ownership, dependencies, or target anchors are ambiguous, or when incomplete task evidence prevents a decision. Do not guess task boundaries",
		"Do not create specs or tasks, invoke `import --apply`, change lifecycle state, or publish review evidence",
	)

	embedded := embeddedSkillFiles(t)
	for _, path := range []string{
		name + "/references/openspec.md",
		name + "/references/spec-kit.md",
	} {
		content, ok := embedded[path]
		if !ok {
			t.Errorf("missing %s", path)
			continue
		}
		content = strings.Join(strings.Fields(content), " ")
		if !strings.Contains(content, "does not prove provenance, approval, completeness, synchronization, change detection, round-trip fidelity, or continuing ownership of source artifacts") {
			t.Errorf("%s must state the full source-artifact limitation", path)
		}
	}
}

func TestImportSkillDocumentsV1CompatibilityAndImplicitHold(t *testing.T) {
	assertSkillReferences(t, "taskrail-import",
		"ImportDraft v1",
		"legacy `body` member is accepted but ignored",
		"does not certify semantic sizing",
		"omit `loop_policy` and `loop_reason`",
		"implicitly held",
	)
}

// Shippable skills create tasks through the real command, not hand-authored
// markdown (Decision 3 in the productization contract). Matches the resolved
// subcommand tail, not the binary prefix, since the entry point renders as
// `${TASKRAIL:-taskrail} task new` (T-051).
func TestShippableSkillsUseTaskNew(t *testing.T) {
	for _, name := range taskAuthoringSkills {
		if !strings.Contains(readShippableSkill(t, name), "} task new") {
			t.Errorf("shippable skill %s must reference '} task new' for task creation", name)
		}
	}
}

// The import skill drives the agent-in-the-loop import path (T-034): it invokes
// the installed binary's emit-prompt and apply steps, never a built-in LLM call.
func TestImportSkillInvokesImportCommand(t *testing.T) {
	assertSkillReferences(t, "taskrail-import", "} import", "--emit-prompt", "--apply")
}

// The decompose skill composes the shipped spec-decomposition primitives (T-098):
// it finds uncovered active-spec areas with `coverage --json`, confirms their live
// anchors with `spec show --anchors`, then authors tasks through `import --apply`
// and closes with a validate. It authors via import, not `task new`, so it stays
// out of taskAuthoringSkills like taskrail-import. Anchoring on resolved subcommand
// tails (not bare flags) keeps the assertion from passing on unrelated prose.
func TestDecomposeSkillComposesSpecPrimitives(t *testing.T) {
	assertSkillReferences(t, "taskrail-decompose",
		"} coverage --json",
		"} spec show <version> --anchors",
		"} import --apply",
		"} validate",
	)
}

// The gap skill composes the structural gap detector with an agent's semantic
// review (T-101): it runs `coverage --gaps --json` for mechanical candidates, then
// routes the agent's judgement-derived candidates to human promotion via `task new`
// or `import --apply`, never auto-creating state. Anchoring on resolved subcommand
// tails (not bare flags) keeps the assertion from passing on unrelated prose; the
// "structural"/"semantic" terms guard that the mechanical-vs-judgement split stays
// documented.
func TestGapSkillComposesStructuralAndSemantic(t *testing.T) {
	assertSkillReferences(t, "taskrail-gap",
		"} coverage --gaps --json",
		"} task new",
		"} import --apply",
		"structural",
		"semantic",
	)
}

func TestTaskReviewSkillPreservesAdvisoryDigestBoundBoundary(t *testing.T) {
	assertSkillReferences(t, "taskrail-task-review",
		"exactly one `review.json`",
		"schema v1",
		"`prompt_contract_version`, `prompt_template_sha256`, `prompt_source`",
		"outcome/spec alignment",
		"do-not-split test",
		"integrated delivery",
		"task author <task-id>",
		"task dependency add <task-id> <dependency-id>",
		"task dependency remove <task-id> <dependency-id>",
		"reviewed implicit-hold follow-up",
		"never changes task status, task-local loop policy",
		"When the selected task's referenced spec is active",
		"historical or future spec",
		"explicitly invoked consuming workflow or the human",
		"do not start another review session",
		"remain reviewable but are not authored through this skill",
	)
}

func TestWorkflowAdversarialSkillPreservesSandboxedReportOnlyBoundary(t *testing.T) {
	assertSkillReferences(t, "taskrail-workflow-adversarial",
		"exact `review_not_found`",
		"at most three",
		"clean attached source worktree",
		"isolated sandbox",
		"terminal observable evidence",
		"Manual evidence requires all four nullable members to be null.",
		"Only command or file evidence",
		"cleanup failure",
		"exactly one strict transient `report.json`",
		"stale",
		"finding dispositions",
		"never writes `INDEX.json` or a final run file directly",
		"never edits product code, specs, tasks, lifecycle status, task-local loop policy, verification results, or Git history",
		"never promotes a finding",
		"committed and local storage modes",
		"fresh context",
		"same-context",
		"Exact V1 Derivation Reference",
		"never stages, commits, merges, pushes",
		"index_sha256_after",
		"two-space indentation",
		"SetEscapeHTML(false)",
		"256 KiB",
		"256 surface rows",
	)
}

// The retrofit skill drives the guided bootstrap end to end (T-043): dry-run
// detection, an explicit --apply, then the emit-prompt -> import --apply adopt
// path that persists reviewed tasks (T-042), closing with a validate.
func TestRetrofitSkillDrivesGuidedFlow(t *testing.T) {
	// Anchor on the full workflow commands, not bare flags: a bare "--apply"
	// would also match the Rules prose, so the assertion must not pass if the
	// apply/emit-prompt workflow steps were dropped.
	assertSkillReferences(t, "taskrail-retrofit",
		"} retrofit <notes.md> --apply",
		"} retrofit <notes.md> --emit-prompt",
		"} import --apply",
		"} validate",
	)
}

// The repair skill drives the conservative dry-run -> apply -> re-validate loop
// through the installed binary, so autonomous-recovery no longer needs to bypass
// the CLI (skills-productization.md, T-050).
func TestRepairSkillDrivesConservativeLoop(t *testing.T) {
	assertSkillReferences(t, "taskrail-repair",
		"} repair",
		"} repair --apply",
		"} validate",
	)
}

// The repair skill covers the parallel-PR merge-conflict scenario (T-089): a
// conflicting STATE.md is never hand-merged. Because STATE.md is a projection of
// the task files, the conflict is sidestepped — take either side, then
// `repair --apply` re-projects from the merged task files. The assertion anchors
// on the conflict framing, the resolution command, and the boundary repair will
// not cross (a real conflict on the same task file stays human-resolved), so it
// fails if any of the three is dropped.
func TestRepairSkillCoversStateConflict(t *testing.T) {
	assertSkillReferences(t, "taskrail-repair",
		"conflict",
		"take either side",
		"} repair --apply",
		"task file",
	)
}

// The retargeted recovery skill must route through repair and must no longer
// permit hand-editing authoritative state (its old bootstrap-edit fallback).
func TestRecoverySkillRoutesThroughRepair(t *testing.T) {
	// Recovery now lives only in the embedded package and its committed
	// .agents/.claude copies; the retired repo-root skills/ tree is gone (T-055).
	for _, dir := range committedSkillTargets {
		path := filepath.Join(dir, "autonomous-recovery", "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		body := string(data)
		// Matches the resolved subcommand tail of the configurable entry point
		// (`${TASKRAIL:-taskrail} repair`, T-051/T-055), not a bare `taskrail`
		// prefix, consistent with the other packaged-skill assertions above.
		if !strings.Contains(body, "} repair") {
			t.Errorf("%s must route recovery through the repair command", path)
		}
		if strings.Contains(body, "bootstrap-era manual edits") {
			t.Errorf("%s must drop the bootstrap-era manual-edit fallback", path)
		}
	}
}

// The spec skill must cover the packaged spec workflow command surface.
func TestSpecSkillCoversSpecCommands(t *testing.T) {
	assertSkillReferences(t, "taskrail-spec",
		"} spec show",
		"--anchors",
		"} spec list",
		"} spec diff",
		"} spec activate",
		"} spec add",
		"} task new",
		"} task repoint",
		"--area",
	)
}

func TestSpecReviewSkillStagesIndependentDigestBoundLenses(t *testing.T) {
	assertSkillReferences(t, "taskrail-spec-review",
		"four independent",
		"when supported, use separate contexts",
		"without earlier conclusions as facts",
		"one schema-v1 JSON object",
		"manifest.json",
		"exactly one disposition",
		"finding_id`, `lens`, `severity`, `disposition`, `rationale`",
		"must equal the referenced finding",
		"`resulting_spec_ref` is required for accepted",
		"`target_version` is required for deferred",
		"required for accepted",
		"required for deferred",
		"High and medium findings may be accepted, rejected, or deferred",
		"all four lens observations",
		"prompt-template drift",
		"Additions never silently expand scope",
		"never edit or activate specs",
		"never create tasks",
		"<artifacts-dir>/review-proposals/spec/<session-id>",
		"review publish --type spec",
	)
}

// Dogfooding-only skills stay out of the shippable directory entirely.
func TestDogfoodingOnlySkillsAreNotShipped(t *testing.T) {
	for _, name := range dogfoodingOnlySkills {
		if _, err := os.Stat(shippableSkillPath(name)); err == nil {
			t.Errorf("dogfooding-only skill %s must not appear in the shippable set", name)
		}
	}
}

// TestEmbeddedPackageMatchesDeclaredShippableSet asserts the embedded package
// ships exactly the declared skills — no more, no less. Unlike the dogfooding-only
// guard above (vacuous while that list is empty), this catches the concrete risk:
// a skill directory added to internal/taskrail/skills/ but never declared in
// shippableSkills would silently ship undocumented and unasserted. It also fails
// if a declared skill's directory disappears from the package.
func TestEmbeddedPackageMatchesDeclaredShippableSet(t *testing.T) {
	declared := map[string]bool{}
	for _, name := range shippableSkills {
		declared[name] = true
	}
	packaged := map[string]bool{}
	for rel := range embeddedSkillFiles(t) {
		packaged[strings.SplitN(rel, "/", 2)[0]] = true
	}
	for name := range packaged {
		if !declared[name] {
			t.Errorf("embedded package ships %q, which is not declared in shippableSkills", name)
		}
	}
	for name := range declared {
		if !packaged[name] {
			t.Errorf("declared shippable skill %q has no directory in the embedded package", name)
		}
	}
}
