package toolchain_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// ciActionUses returns the action reference from every `uses:` step directive in
// a workflow file, skipping YAML comment lines and trailing inline comments. It
// deliberately ignores prose so a historical comment mentioning an action cannot
// flip the mise-provisioning guard below (a bare substring search over the file
// would).
func ciActionUses(content string) []string {
	var uses []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		directive := strings.TrimPrefix(trimmed, "- ") // steps write `- uses:` or `uses:`
		if !strings.HasPrefix(directive, "uses:") {
			continue
		}
		ref := strings.TrimSpace(strings.TrimPrefix(directive, "uses:"))
		if i := strings.Index(ref, " #"); i >= 0 {
			ref = strings.TrimSpace(ref[:i])
		}
		uses = append(uses, ref)
	}
	return uses
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

// mise-action v4.2.1 deliberately stopped exporting PATH changes from
// mise.toml's [env] table. Pin the reviewed action revision so another floating
// major-tag update cannot silently change CI environment behavior again.
func TestWorkflowsPinMiseAction(t *testing.T) {
	const want = "jdx/mise-action@dad1bfd3df957f44999b559dd69dc1671cb4e9ea" // v4.2.1
	root := repoRoot(t)
	for _, rel := range []string{".github/workflows/ci.yml", ".github/workflows/planning.yml"} {
		found := false
		for _, ref := range ciActionUses(readFile(t, root, rel)) {
			if !strings.HasPrefix(ref, "jdx/mise-action@") {
				continue
			}
			found = true
			if ref != want {
				t.Errorf("%s uses %q, want immutable v4.2.1 pin %q", rel, ref, want)
			}
		}
		if !found {
			t.Errorf("%s must provision the toolchain via mise-action", rel)
		}
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
