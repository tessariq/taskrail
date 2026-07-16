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
	"taskrail-import",
	"taskrail-retrofit",
	"taskrail-repair",
	"taskrail-spec",
	"taskrail-decompose",
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
	body := readShippableSkill(t, name)
	for _, want := range wants {
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

// The spec skill drives the spec command family (T-064): it discovers spec_ref
// anchors with `spec show --anchors --json` before authoring tracked work, and
// documents `spec list`, `spec activate`, and `spec add`. Anchoring on the
// resolved subcommand tails (not bare flags) keeps the assertion from passing on
// unrelated prose.
func TestSpecSkillCoversSpecCommands(t *testing.T) {
	assertSkillReferences(t, "taskrail-spec",
		"} spec show",
		"--anchors",
		"} spec list",
		"} spec activate",
		"} spec add",
		"} task new",
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
