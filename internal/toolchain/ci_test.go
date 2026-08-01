package toolchain_test

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// actionUse is one `uses:` step directive split into the action reference and
// the trailing inline comment, which by convention annotates a SHA pin with the
// human-readable version it resolves to.
type actionUse struct {
	ref     string
	comment string
}

// ciActionUsesAnnotated returns every `uses:` step directive in a workflow file,
// skipping YAML comment lines. It deliberately ignores prose so a historical
// comment mentioning an action cannot flip the mise-provisioning guard below (a
// bare substring search over the file would).
func ciActionUsesAnnotated(content string) []actionUse {
	var uses []actionUse
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		directive := strings.TrimPrefix(trimmed, "- ") // steps write `- uses:` or `uses:`
		if !strings.HasPrefix(directive, "uses:") {
			continue
		}
		use := actionUse{ref: strings.TrimSpace(strings.TrimPrefix(directive, "uses:"))}
		if i := strings.Index(use.ref, " #"); i >= 0 {
			use.comment = strings.TrimSpace(use.ref[i:])
			use.ref = strings.TrimSpace(use.ref[:i])
		}
		uses = append(uses, use)
	}
	return uses
}

// ciActionUses drops the version annotations for guards that only care which
// action a step runs.
func ciActionUses(content string) []string {
	var refs []string
	for _, use := range ciActionUsesAnnotated(content) {
		refs = append(refs, use.ref)
	}
	return refs
}

// CI must provision its toolchain through mise (jdx/mise-action) so the pinned
// versions in mise.toml are the single source of truth for local and CI alike
// (specs/v0.2.0.md#mise-toolchain-management). A lingering actions/setup-go step
// would reintroduce a second, independently pinned Go version for CI, so its
// absence is asserted too — over actual `uses:` steps, not raw file text.
func TestCIProvisionsToolchainViaMise(t *testing.T) {
	root := repoRoot(t)
	uses := ciActionUses(readFile(t, root, ".github/workflows/ci.yml"))

	mise := false
	for _, ref := range uses {
		if strings.HasPrefix(ref, "jdx/mise-action") {
			mise = true
		}
		if strings.HasPrefix(ref, "actions/setup-go") {
			t.Errorf("ci.yml uses %q; mise is the single toolchain provisioner", ref)
		}
	}
	if !mise {
		t.Error("ci.yml must provision the toolchain via a jdx/mise-action step")
	}
}

// workflowFile pairs a workflow's repo-relative path with its contents so
// cross-file assertions can name the offender.
type workflowFile struct {
	name    string
	content string
}

const miseActionPrefix = "jdx/mise-action@"

// miseVersion is a parsed `# vX.Y.Z` pin annotation, held major-first so
// slices.Compare orders two of them by release.
type miseVersion [3]int

func (v miseVersion) String() string {
	return fmt.Sprintf("v%d.%d.%d", v[0], v[1], v[2])
}

// miseVersionFloor is the reviewed release: v4.2.1 deliberately stopped
// exporting PATH changes from mise.toml's [env] table, and CI's GITHUB_PATH
// handoff depends on that. An older pin would reintroduce it.
var miseVersionFloor = miseVersion{4, 2, 1}

// Compiled once rather than per step, since both are consulted inside the
// per-`uses:` loop below.
var (
	miseCommitSHA   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	miseVersionNote = regexp.MustCompile(`^#\s*v(\d+)\.(\d+)\.(\d+)$`)
)

// misePinVersion reads the `# vX.Y.Z` annotation Dependabot maintains beside a
// SHA pin, reporting false when the comment is absent or not a release version.
func misePinVersion(comment string) (miseVersion, bool) {
	m := miseVersionNote.FindStringSubmatch(comment)
	if m == nil {
		return miseVersion{}, false
	}
	var version miseVersion
	for i := range version {
		n, err := strconv.Atoi(m[i+1])
		if err != nil { // digits out of int range: not a release we can order
			return miseVersion{}, false
		}
		version[i] = n
	}
	return version, true
}

// misePinProblems reports how one mise-action step fails the properties the pin
// buys: an immutable revision, annotated with the release it resolves to, at or
// above the reviewed floor.
func misePinProblems(workflow string, use actionUse) []string {
	revision := strings.TrimPrefix(use.ref, miseActionPrefix)

	var problems []string
	if !miseCommitSHA.MatchString(revision) {
		problems = append(problems, fmt.Sprintf(
			"%s pins mise-action at %q; want an immutable 40-character commit SHA", workflow, revision))
	}
	version, ok := misePinVersion(use.comment)
	switch {
	case !ok:
		problems = append(problems, fmt.Sprintf(
			"%s pins mise-action at %s; annotate it with exactly `# vX.Y.Z`, got %q",
			workflow, revision, use.comment))
	case slices.Compare(version[:], miseVersionFloor[:]) < 0:
		problems = append(problems, fmt.Sprintf(
			"%s annotates mise-action %s as %s, below the reviewed %s floor",
			workflow, revision, version, miseVersionFloor))
	}
	return problems
}

// misePinViolations reports every way the workflows' mise-action pins fail the
// contract, and nil when they hold. It asserts properties rather than a literal
// SHA because Dependabot rewrites the YAML `uses:` lines and cannot touch this
// file: a literal here would turn every routine bump into a guaranteed CI
// failure. Pins must also agree across workflows, so bumping one and forgetting
// another is itself a violation.
//
// Accepted trade-off: the annotation is taken at its word — nothing here proves
// the SHA is the commit that release tagged, so a hand-edit repointing the pin at
// an unrelated commit under a plausible comment passes. The old literal caught
// that only by failing on every bump, Dependabot's included. Reviewing the pin
// change itself is the remaining control.
func misePinViolations(files []workflowFile) []string {
	var problems []string
	var canonical string
	for _, file := range files {
		found := false
		for _, use := range ciActionUsesAnnotated(file.content) {
			if !strings.HasPrefix(use.ref, miseActionPrefix) {
				continue
			}
			found = true
			problems = append(problems, misePinProblems(file.name, use)...)
			if canonical == "" {
				canonical = use.ref
			} else if use.ref != canonical {
				problems = append(problems, fmt.Sprintf(
					"%s uses %q; every workflow must pin the same revision %q", file.name, use.ref, canonical))
			}
		}
		if !found {
			problems = append(problems, fmt.Sprintf("%s must provision the toolchain via mise-action", file.name))
		}
	}
	return problems
}

// The annotated parser must surface the trailing `# vX.Y.Z` that ciActionUses
// discards, since the version floor is only readable from that comment.
func TestCIActionUsesAnnotatedCapturesVersionComment(t *testing.T) {
	pin := miseActionPrefix + strings.Repeat("a", 40)
	got := ciActionUsesAnnotated("        uses: " + pin + " # v4.2.3\n")
	want := []actionUse{{ref: pin, comment: "# v4.2.3"}}
	if !slices.Equal(got, want) {
		t.Errorf("ciActionUsesAnnotated: got %v, want %v", got, want)
	}
}

// misePinViolations is the whole contract, so exercise it against synthetic
// workflows: a Dependabot-shaped bump (every `uses:` line rewritten to one new
// SHA and version comment) must stay silent, while each way the pin can decay
// must be reported.
func TestMisePinViolations(t *testing.T) {
	step := func(revision, annotation string) string {
		return "      - name: Set up toolchain\n        uses: " + miseActionPrefix + revision + annotation + "\n"
	}
	oldSHA, newSHA := strings.Repeat("a", 40), strings.Repeat("b", 40)

	for _, tc := range []struct {
		name          string
		files         []workflowFile
		wantViolation bool
	}{
		{
			name: "dependabot bump of every workflow",
			files: []workflowFile{
				{"ci.yml", step(newSHA, " # v4.2.3")},
				{"planning.yml", step(newSHA, " # v4.2.3")},
			},
		},
		{
			name:          "floating tag instead of a commit SHA",
			files:         []workflowFile{{"ci.yml", step("v4", "")}},
			wantViolation: true,
		},
		{
			name:          "truncated SHA",
			files:         []workflowFile{{"ci.yml", step(newSHA[:12], " # v4.2.3")}},
			wantViolation: true,
		},
		{
			name:          "pin without a version annotation",
			files:         []workflowFile{{"ci.yml", step(newSHA, "")}},
			wantViolation: true,
		},
		{
			name:          "annotation below the reviewed floor",
			files:         []workflowFile{{"ci.yml", step(newSHA, " # v4.1.9")}},
			wantViolation: true,
		},
		{
			// The floor is inclusive, and the real workflows will not always sit
			// on it — without this case, flipping the comparison to reject the
			// reviewed release itself would go unnoticed after the next bump.
			name:  "annotation exactly at the reviewed floor",
			files: []workflowFile{{"ci.yml", step(newSHA, " # "+miseVersionFloor.String())}},
		},
		{
			name: "one workflow left behind on the old SHA",
			files: []workflowFile{
				{"ci.yml", step(newSHA, " # v4.2.3")},
				{"planning.yml", step(oldSHA, " # v4.2.1")},
			},
			wantViolation: true,
		},
		{
			name:          "workflow with no mise-action step at all",
			files:         []workflowFile{{"ci.yml", "      - name: Checkout\n        uses: actions/checkout@v7\n"}},
			wantViolation: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := misePinViolations(tc.files)
			if (len(got) > 0) != tc.wantViolation {
				t.Errorf("misePinViolations: got %v, want violation = %v", got, tc.wantViolation)
			}
		})
	}
}

// Holds the real workflows to the contract misePinViolations documents.
// Deliberately no literal SHA here: only Dependabot's YAML rewrite can move the
// pin, so a copy in this file would fail CI on every routine bump.
func TestWorkflowsPinMiseAction(t *testing.T) {
	root := repoRoot(t)
	var files []workflowFile
	for _, rel := range []string{".github/workflows/ci.yml", ".github/workflows/planning.yml"} {
		files = append(files, workflowFile{name: rel, content: readFile(t, root, rel)})
	}
	for _, problem := range misePinViolations(files) {
		t.Error(problem)
	}
}

// mise-action does not propagate [env] _.path to later workflow steps. CI must
// therefore expose the working-tree bin directory through GitHub's supported
// PATH handoff before testing that a bare taskrail resolves to that build.
func TestCIExposesBinBeforeBareTaskrailSmoke(t *testing.T) {
	ci := readFile(t, repoRoot(t), ".github/workflows/ci.yml")
	expose := strings.Index(ci, `echo "${{ github.workspace }}/bin" >> "$GITHUB_PATH"`)
	smoke := strings.Index(ci, "run: taskrail validate")
	if expose < 0 {
		t.Fatal("ci.yml must add the workspace bin directory to GITHUB_PATH")
	}
	if smoke < 0 {
		t.Fatal("ci.yml must smoke a bare `taskrail validate`")
	}
	if expose > smoke {
		t.Error("ci.yml must expose the workspace bin directory before the bare taskrail smoke")
	}
}

// ciRunsRawGo reports every non-comment line in a workflow file that invokes the
// Go toolchain directly (go build/vet/test/run/...). CI must route those through
// `task` targets so mise.toml + Taskfile.yml stay the single source of build
// commands for local and CI alike; a raw `go` step bypasses that contract.
func ciRunsRawGo(content string) []string {
	rawGo := regexp.MustCompile(`\bgo\s+(build|vet|test|run|install|generate)\b`)
	var offenders []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Drop a trailing inline comment so a step annotated with the command it
		// replaced (`run: task build  # was: go build`) cannot trip the guard.
		if i := strings.Index(trimmed, " #"); i >= 0 {
			trimmed = strings.TrimSpace(trimmed[:i])
		}
		if rawGo.MatchString(trimmed) {
			offenders = append(offenders, trimmed)
		}
	}
	return offenders
}

// A step annotated with what it replaced (`run: task build  # was: go build`)
// must not trip the raw-go guard on its comment. Mirrors ciActionUses's handling.
func TestCIRunsRawGoIgnoresInlineComments(t *testing.T) {
	if got := ciRunsRawGo("run: task build  # was: go build ./cmd/taskrail"); got != nil {
		t.Errorf("inline comment tripped raw-go guard: %v", got)
	}
}

// CI must drive builds/tests through `task` (provisioned by mise-action) rather
// than invoking `go` directly, so Taskfile.yml is the single source of build
// commands across local, hooks, and CI.
func TestCIDelegatesGoToTask(t *testing.T) {
	root := repoRoot(t)
	offenders := ciRunsRawGo(readFile(t, root, ".github/workflows/ci.yml"))
	if len(offenders) > 0 {
		t.Errorf("ci.yml invokes the go toolchain directly; route through `task`:\n%s",
			strings.Join(offenders, "\n"))
	}
}

// ciMatrixRunners returns every runner label declared in the `runner: [ ... ]`
// matrix arrays of a workflow file. Comment lines are skipped so a commented-out
// runner cannot masquerade as active coverage.
func ciMatrixRunners(content string) []string {
	arrayRe := regexp.MustCompile(`^runner:\s*\[([^\]]*)\]`)
	var runners []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		m := arrayRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		for _, entry := range strings.Split(m[1], ",") {
			// Tolerate YAML-quoted labels (["ubuntu-latest"]) so a quoted
			// rewrite of the matrix cannot silently defeat the coverage guard.
			label := strings.Trim(strings.TrimSpace(entry), `"'`)
			if label != "" {
				runners = append(runners, label)
			}
		}
	}
	return runners
}

// A future editor may YAML-quote the runner labels; the parser must return them
// unquoted so the coverage assertions still match. Guards the strip-quotes step.
func TestCIMatrixRunnersStripsQuotes(t *testing.T) {
	got := ciMatrixRunners(`runner: ["ubuntu-latest", 'windows-latest', macos-latest]`)
	want := []string{"ubuntu-latest", "windows-latest", "macos-latest"}
	if !slices.Equal(got, want) {
		t.Errorf("ciMatrixRunners quoted labels: got %v, want %v", got, want)
	}
}

// CI must exercise the CLI on Linux, Windows, and macOS so OS-specific
// regressions (path separators, line endings, file modes) are caught before
// merge — notably the T-041 pending-spec path comparison, which only diverges on
// Windows (specs/v0.2.0.md#taskrail-import). Asserted over declared matrix
// runners rather than raw file text so prose cannot satisfy the guard.
func TestCIMatrixCoversRequiredOSes(t *testing.T) {
	root := repoRoot(t)
	runners := ciMatrixRunners(readFile(t, root, ".github/workflows/ci.yml"))

	for _, required := range []string{"ubuntu-latest", "windows-latest", "macos-latest"} {
		if !slices.Contains(runners, required) {
			t.Errorf("ci.yml matrix must include runner %q; have %v", required, runners)
		}
	}
}

// ciStepRunsUnderCondition reports whether any workflow step whose `if:`
// condition equality-matches cond has a `run:` body invoking runCmd. It reuses
// workflowStepBlocks to split the file into per-step blocks, so the guard
// asserts a command runs on a specific matrix leg, not merely somewhere else in
// the file. The `run:` scope persists across `run: |` continuation lines, and
// cond/runCmd are matched independently within the block so ordering of `if:`
// versus `run:` does not matter. The condition must be a positive `== '<cond>'`
// (single or double quoted); a `!=` (run everywhere-except) does not count, so a
// flipped operator can't silently pass the guard while dropping coverage.
func ciStepRunsUnderCondition(content, cond, runCmd string) bool {
	eq := []string{"== '" + cond + "'", `== "` + cond + `"`}
	for _, block := range workflowStepBlocks(content) {
		var inRun, sawCond, sawRun bool
		for _, line := range block {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			switch {
			case strings.HasPrefix(trimmed, "if:"):
				inRun = false
				for _, want := range eq {
					if strings.Contains(line, want) {
						sawCond = true
					}
				}
			case strings.HasPrefix(trimmed, "run:"):
				inRun = true
			}
			if inRun && strings.Contains(line, runCmd) {
				sawRun = true
			}
		}
		if sawCond && sawRun {
			return true
		}
	}
	return false
}

// TestCIStepRunsUnderConditionScopesToStep guards the parser: a cond in one step
// and the runCmd in a different step must NOT satisfy it — they must co-occur.
func TestCIStepRunsUnderConditionScopesToStep(t *testing.T) {
	twoSteps := "" +
		"      - name: A\n" +
		"        if: matrix.runner == 'windows-latest'\n" +
		"        run: echo hi\n" +
		"      - name: B\n" +
		"        run: task taskrail:check\n"
	if ciStepRunsUnderCondition(twoSteps, "windows-latest", "task taskrail:check") {
		t.Error("condition and run in different steps must not satisfy the guard")
	}
	oneStep := "" +
		"      - name: A\n" +
		"        if: matrix.runner == 'windows-latest'\n" +
		"        run: task taskrail:check\n"
	if !ciStepRunsUnderCondition(oneStep, "windows-latest", "task taskrail:check") {
		t.Error("condition and run in the same step must satisfy the guard")
	}

	// An inverted condition ("run everywhere EXCEPT windows") must NOT satisfy
	// the guard — otherwise a flipped operator would silently drop the very
	// Windows coverage T-091 adds while the test still passes.
	negated := "" +
		"      - name: A\n" +
		"        if: matrix.runner != 'windows-latest'\n" +
		"        run: task taskrail:check\n"
	if ciStepRunsUnderCondition(negated, "windows-latest", "task taskrail:check") {
		t.Error("a `!=` condition must not satisfy the equality guard")
	}
}

// The Windows portability claim for `taskrail:check` (T-082) was verified only
// by inspection and a stdlib cross-compile; T-091 requires an actual run on the
// native windows-latest leg. Assert a windows-conditional CI step exercises the
// freshness guard so a real Windows regression is caught in the pipeline.
func TestCIExercisesTaskrailCheckOnWindows(t *testing.T) {
	ci := readFile(t, repoRoot(t), ".github/workflows/ci.yml")
	if !ciStepRunsUnderCondition(ci, "windows-latest", "task taskrail:check") {
		t.Error("ci.yml must run `task taskrail:check` on a windows-latest-conditional step (T-091)")
	}
}
