package toolchain_test

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
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
	actionCommitSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)
	versionNote     = regexp.MustCompile(`^#\s*v(\d+)\.(\d+)\.(\d+)$`)
)

// pinVersion reads the `# vX.Y.Z` annotation Dependabot maintains beside a
// SHA pin, reporting false when the comment is absent or not a release version.
func pinVersion(comment string) (miseVersion, bool) {
	m := versionNote.FindStringSubmatch(comment)
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

// workflowFiles reads every workflow in the repository. Enumerating the
// directory keeps a newly added workflow covered by the guards below without
// anyone remembering to extend a list here.
func workflowFiles(t *testing.T) []workflowFile {
	t.Helper()
	root := repoRoot(t)
	const dir = ".github/workflows"
	entries, err := os.ReadDir(filepath.Join(root, dir))
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var files []workflowFile
	for _, entry := range entries {
		// GitHub accepts both spellings; skipping .yaml would leave a workflow
		// added under that name unguarded, which is the hole this helper exists
		// to close.
		ext := filepath.Ext(entry.Name())
		if entry.IsDir() || (ext != ".yml" && ext != ".yaml") {
			continue
		}
		rel := path.Join(dir, entry.Name())
		files = append(files, workflowFile{name: rel, content: readFile(t, root, rel)})
	}
	return files
}

// actionPinProblems reports how one `uses:` step fails the pin contract: an
// immutable commit SHA, annotated with the release it resolves to. A mutable
// reference (`@v7`, `@main`) lets whoever controls that tag change what CI and
// the release pipeline execute with no diff in this repository, and the
// annotation is what keeps the opaque SHA reviewable and Dependabot-updatable.
// Not every `uses:` form takes a commit SHA, so each non-repository shape is
// held to whatever immutability it can actually offer rather than exempted
// outright: a local action ships with the checkout, and a container image pins
// through its own digest syntax.
func actionPinProblems(workflow string, use actionUse) []string {
	if strings.HasPrefix(use.ref, "./") {
		return nil
	}
	if image, ok := strings.CutPrefix(use.ref, "docker://"); ok {
		if strings.Contains(image, "@sha256:") {
			return nil
		}
		return []string{fmt.Sprintf(
			"%s runs container %q; want an immutable @sha256: digest", workflow, image)}
	}

	action, revision, ok := strings.Cut(use.ref, "@")
	if !ok {
		return []string{fmt.Sprintf("%s uses %q without a pinned revision", workflow, use.ref)}
	}

	var problems []string
	if !actionCommitSHA.MatchString(revision) {
		problems = append(problems, fmt.Sprintf(
			"%s pins %s at %q; want an immutable 40-character commit SHA", workflow, action, revision))
	}
	if _, ok := pinVersion(use.comment); !ok {
		problems = append(problems, fmt.Sprintf(
			"%s pins %s at %s; annotate it with exactly `# vX.Y.Z`, got %q",
			workflow, action, revision, use.comment))
	}
	return problems
}

// misePinProblems reports only mise-action's extra rule — the annotated release
// must not fall below the reviewed floor. The shared contract (SHA shape,
// annotation) belongs to actionPinProblems alone, so a broken mise pin surfaces
// as one finding rather than the same string from two unrelated-looking tests.
// A missing or malformed annotation is therefore that guard's finding, not this
// one's, and yields nothing here.
func misePinProblems(workflow string, use actionUse) []string {
	version, ok := pinVersion(use.comment)
	if !ok || slices.Compare(version[:], miseVersionFloor[:]) >= 0 {
		return nil
	}
	return []string{fmt.Sprintf(
		"%s annotates mise-action %s as %s, below the reviewed %s floor",
		workflow, strings.TrimPrefix(use.ref, miseActionPrefix), version, miseVersionFloor)}
}

// misePinViolations reports the mise-action rules that go beyond the shared pin
// contract: the version floor, one revision across every workflow, and a mise
// step present in each workflow named by mustProvision. Presence is a parameter
// because release.yml deliberately provisions Go through setup-go, so requiring
// it everywhere would be wrong. The floor and consistency rules still run over
// every file passed in, so a mise step added to release.yml is covered the day
// it appears.
//
// It asserts properties rather than a literal SHA because Dependabot rewrites
// the YAML `uses:` lines and cannot touch this file: a literal here would turn
// every routine bump into a guaranteed CI failure.
//
// Accepted trade-off: the annotation is taken at its word — nothing here proves
// the SHA is the commit that release tagged, so a hand-edit repointing the pin at
// an unrelated commit under a plausible comment passes. The old literal caught
// that only by failing on every bump, Dependabot's included. Reviewing the pin
// change itself is the remaining control.
func misePinViolations(files []workflowFile, mustProvision []string) []string {
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
		if !found && slices.Contains(mustProvision, file.name) {
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

// Exercise the mise-only rules against synthetic workflows: a Dependabot-shaped
// bump (every `uses:` line rewritten to one new SHA and version comment) must
// stay silent, while a downgrade, a half-finished bump, and a workflow that
// stopped provisioning through mise must each be reported. Pin shape belongs to
// TestActionPinProblems and is deliberately not re-asserted here.
func TestMisePinViolations(t *testing.T) {
	step := func(revision, annotation string) string {
		return "      - name: Set up toolchain\n        uses: " + miseActionPrefix + revision + annotation + "\n"
	}
	oldSHA, newSHA := strings.Repeat("a", 40), strings.Repeat("b", 40)
	required := []string{"ci.yml", "planning.yml"}

	for _, tc := range []struct {
		name          string
		files         []workflowFile
		mustProvision []string
		wantViolation bool
	}{
		{
			name: "dependabot bump of every workflow",
			files: []workflowFile{
				{"ci.yml", step(newSHA, " # v4.2.3")},
				{"planning.yml", step(newSHA, " # v4.2.3")},
			},
			mustProvision: required,
		},
		{
			name:          "annotation below the reviewed floor",
			files:         []workflowFile{{"ci.yml", step(newSHA, " # v4.1.9")}},
			mustProvision: required,
			wantViolation: true,
		},
		{
			// The floor is inclusive, and the real workflows will not always sit
			// on it — without this case, flipping the comparison to reject the
			// reviewed release itself would go unnoticed after the next bump.
			name:          "annotation exactly at the reviewed floor",
			files:         []workflowFile{{"ci.yml", step(newSHA, " # "+miseVersionFloor.String())}},
			mustProvision: required,
		},
		{
			name: "one workflow left behind on the old SHA",
			files: []workflowFile{
				{"ci.yml", step(newSHA, " # v4.2.3")},
				{"planning.yml", step(oldSHA, " # v4.2.1")},
			},
			mustProvision: required,
			wantViolation: true,
		},
		{
			name:          "workflow that must provision through mise but does not",
			files:         []workflowFile{{"ci.yml", "      - name: Checkout\n        uses: actions/checkout@v7\n"}},
			mustProvision: required,
			wantViolation: true,
		},
		{
			// release.yml provisions Go through setup-go on purpose, so absence
			// there is correct — not every workflow owes a mise step.
			name:          "workflow outside mustProvision without a mise step",
			files:         []workflowFile{{"release.yml", "      - name: Checkout\n        uses: actions/checkout@v7\n"}},
			mustProvision: required,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := misePinViolations(tc.files, tc.mustProvision)
			if (len(got) > 0) != tc.wantViolation {
				t.Errorf("misePinViolations: got %v, want violation = %v", got, tc.wantViolation)
			}
		})
	}
}

// actionPinProblems is the contract every action must meet, so exercise the ways
// a pin can decay independently of any one action's extra rules — including the
// `uses:` forms that take no commit SHA, which must be judged on the
// immutability they can offer rather than waved through or wrongly flagged.
func TestActionPinProblems(t *testing.T) {
	sha := strings.Repeat("c", 40)
	digest := "sha256:" + strings.Repeat("d", 64)
	for _, tc := range []struct {
		name          string
		use           actionUse
		wantViolation bool
	}{
		{name: "annotated commit SHA", use: actionUse{ref: "actions/checkout@" + sha, comment: "# v7.0.1"}},
		{name: "action in a repository subdirectory", use: actionUse{ref: "owner/repo/sub@" + sha, comment: "# v1.2.3"}},
		{name: "local action shipped with the checkout", use: actionUse{ref: "./.github/actions/build"}},
		{name: "container pinned by digest", use: actionUse{ref: "docker://alpine@" + digest}},
		{name: "floating major tag", use: actionUse{ref: "actions/checkout@v7"}, wantViolation: true},
		{name: "branch reference", use: actionUse{ref: "actions/checkout@main"}, wantViolation: true},
		{name: "no revision at all", use: actionUse{ref: "actions/checkout"}, wantViolation: true},
		{name: "container on a mutable tag", use: actionUse{ref: "docker://alpine:3"}, wantViolation: true},
		{
			name:          "commit SHA without an annotation",
			use:           actionUse{ref: "actions/checkout@" + sha},
			wantViolation: true,
		},
		{
			name:          "uppercase hex is not a canonical SHA",
			use:           actionUse{ref: "actions/checkout@" + strings.ToUpper(sha), comment: "# v7.0.1"},
			wantViolation: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := actionPinProblems("ci.yml", tc.use)
			if (len(got) > 0) != tc.wantViolation {
				t.Errorf("actionPinProblems: got %v, want violation = %v", got, tc.wantViolation)
			}
		})
	}
}

// A mutable tag lets whoever controls it change what CI and the release pipeline
// execute with no diff in this repository. Enumerate the workflow directory
// rather than a fixed list so a newly added workflow is covered without editing
// this test — the case a hardcoded list would silently miss.
func TestWorkflowsPinActionsToCommitSHAs(t *testing.T) {
	files := workflowFiles(t)
	if len(files) == 0 {
		t.Fatal("no workflows found under .github/workflows")
	}
	for _, file := range files {
		for _, use := range ciActionUsesAnnotated(file.content) {
			for _, problem := range actionPinProblems(file.name, use) {
				t.Error(problem)
			}
		}
	}
}

// Holds the real workflows to the contract misePinViolations documents. The
// floor and consistency rules run over every workflow, while only the two that
// build with the repository toolchain are required to provision through mise —
// release.yml uses setup-go by design. Deliberately no literal SHA here: only
// Dependabot's YAML rewrite can move the pin, so a copy in this file would fail
// CI on every routine bump.
func TestWorkflowsPinMiseAction(t *testing.T) {
	mustProvision := []string{".github/workflows/ci.yml", ".github/workflows/planning.yml"}
	for _, problem := range misePinViolations(workflowFiles(t), mustProvision) {
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

// Filesystem capability skips must be visible on each native matrix leg rather
// than buried in the non-verbose full suite.
func TestCIRunsVerboseFilesystemPortabilitySuiteOnNativeMatrix(t *testing.T) {
	root := repoRoot(t)
	target := strings.Join(taskfileBlock(readFile(t, root, "Taskfile.yml"), "test:filesystem:"), "\n")
	for _, required := range []string{"go test -v", "./internal/durablefs", "./internal/durabletx", "./internal/repolock", "./internal/repotx", "./internal/taskrail", "TestParseFrontmatterHandlesCRLF", "TestDecodeDecompositionBundlePreservesCompleteValidBundle"} {
		if !strings.Contains(target, required) {
			t.Errorf("Taskfile test:filesystem target must contain %q", required)
		}
	}

	ci := readFile(t, root, ".github/workflows/ci.yml")
	found := false
	for _, block := range workflowStepBlocks(ci) {
		joined := strings.Join(block, "\n")
		if strings.Contains(joined, "run: task test:filesystem") {
			found = true
			if strings.Contains(joined, "if:") {
				t.Error("filesystem portability suite must run unconditionally on the native matrix")
			}
		}
	}
	if !found {
		t.Error("ci.yml build-test matrix must run `task test:filesystem`")
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
