package toolchain_test

import (
	"regexp"
	"strings"
	"testing"
)

// The adopted skills (T-051) resolve the Taskrail binary with a plain
// ${TASKRAIL:-taskrail} fallback and no env override. For that fallback to hit
// the working-tree build, mise must expose a repo-local bin directory on the
// mise-provided PATH — the same PATH contract that already carries
// go/task/lefthook (specs/v0.3.0.md#goals). These guards keep that wiring, the
// freshness guard, and the "no TASKRAIL override" contract from drifting.

// miseSection returns the body lines of the named TOML table (e.g. "[env]") in
// content, excluding the header line and stopping at the next table header. This
// scoping is what lets the guards below trust a match: a key or value in another
// section cannot satisfy them.
func miseSection(content, name string) []string {
	var body []string
	inSection := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			// Strip a trailing inline comment so `[env] # note` still counts.
			header := strings.TrimSpace(strings.SplitN(trimmed, "#", 2)[0])
			inSection = header == name
			continue
		}
		if inSection {
			body = append(body, line)
		}
	}
	return body
}

// miseEnvPath returns the raw value of the `_.path` key inside the [env] table
// of mise.toml, or "" if absent.
func miseEnvPath(content string) string {
	re := regexp.MustCompile(`^\s*_\.path\s*=\s*(.+)$`)
	for _, line := range miseSection(content, "[env]") {
		if m := re.FindStringSubmatch(line); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

// mise.toml must add a repo-local bin directory to the mise-provided PATH so a
// bare `taskrail` resolves to the working-tree build under mise.
func TestMiseExposesBinOnPath(t *testing.T) {
	mise := readFile(t, repoRoot(t), "mise.toml")
	path := miseEnvPath(mise)
	if path == "" {
		t.Fatal("mise.toml [env] must set _.path to expose the working-tree taskrail on PATH")
	}
	// Assert on the trailing path segment, not a bare "bin" substring, so an
	// unrelated path (e.g. /usr/sbin, /opt/binutils) cannot satisfy the guard.
	if !regexp.MustCompile(`[/"]bin["/\]]`).MatchString(path) {
		t.Errorf("mise.toml [env] _.path = %s must expose a repo-local bin directory", path)
	}
}

// miseSetupRun returns the body of the [tasks.setup] table so the setup steps
// can be asserted regardless of `run` list formatting.
func miseSetupRun(content string) string {
	return strings.Join(miseSection(content, "[tasks.setup]"), "\n")
}

// `mise run setup` must build the working-tree taskrail onto the mise PATH so a
// fresh clone gets a resolvable, current binary without any manual step.
func TestMiseSetupBuildsTaskrail(t *testing.T) {
	setup := miseSetupRun(readFile(t, repoRoot(t), "mise.toml"))
	if !strings.Contains(setup, "taskrail:install") {
		t.Errorf("mise.toml [tasks.setup] must run `task taskrail:install`; got:\n%s", setup)
	}
}

// The Taskfile owns both halves of the binary contract: an install target that
// (re)builds onto the PATH and a freshness check that fails when the on-PATH
// binary is stale versus the working tree.
func TestTaskfileDefinesTaskrailInstallAndCheck(t *testing.T) {
	taskfile := readFile(t, repoRoot(t), "Taskfile.yml")
	for _, target := range []string{"taskrail:install:", "taskrail:check:"} {
		if !strings.Contains(taskfile, target) {
			t.Errorf("Taskfile.yml must define a %q target", strings.TrimSuffix(target, ":"))
		}
	}
}

// taskfileBlock returns the indented body of the top-level Taskfile task named
// header (e.g. "taskrail:check:"), from just after the header line until the
// next line at the same (2-space) indent. It lets a guard reason about one
// task's steps without a full YAML parse.
func taskfileBlock(content, header string) []string {
	lines := strings.Split(content, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == header && strings.HasPrefix(line, "  ") {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil
	}
	var body []string
	for _, line := range lines[start:] {
		if strings.TrimSpace(line) == "" {
			body = append(body, line)
			continue
		}
		// A non-blank line indented <=2 spaces starts the next task.
		if len(line)-len(strings.TrimLeft(line, " ")) <= 2 {
			break
		}
		body = append(body, line)
	}
	return body
}

// The freshness guard compares two independently produced builds byte-for-byte,
// so both build sites must use identical, reproducible flags. Divergent flags
// (e.g. one pinning CGO_ENABLED=0 and the other inheriting the ambient default)
// make a freshly installed binary compare as stale — the exact drift this guard
// forbids. Both taskrail build sites must therefore pin CGO_ENABLED and -trimpath.
func TestTaskrailBuildsShareReproducibleFlags(t *testing.T) {
	taskfile := readFile(t, repoRoot(t), "Taskfile.yml")
	for _, header := range []string{"taskrail:install:", "taskrail:check:"} {
		block := strings.Join(taskfileBlock(taskfile, header), "\n")
		if !strings.Contains(block, "go build") {
			t.Errorf("%s must build ./cmd/taskrail", header)
			continue
		}
		if !strings.Contains(block, "CGO_ENABLED") {
			t.Errorf("%s must pin CGO_ENABLED so its build is byte-reproducible", header)
		}
		if !strings.Contains(block, "-trimpath") {
			t.Errorf("%s must build with -trimpath so its build is byte-reproducible", header)
		}
	}
}

// The freshness check must run on a stock native Windows install (no
// Git-for-Windows/MSYS/WSL on PATH), so it may not lean on external coreutils
// that ship only with a POSIX userland. mktemp/cmp/trap are absent there; the
// check must use a cross-platform mechanism instead (T-082).
func TestTaskrailCheckIsPortable(t *testing.T) {
	taskfile := readFile(t, repoRoot(t), "Taskfile.yml")
	block := strings.Join(taskfileBlock(taskfile, "taskrail:check:"), "\n")
	for _, tool := range []string{"mktemp", "cmp ", "cmp -", "trap "} {
		if strings.Contains(block, tool) {
			t.Errorf("taskrail:check must not rely on %q (absent on stock native Windows); use a cross-platform mechanism", strings.TrimSpace(tool))
		}
	}
}

// Building onto a directory nothing resolves `taskrail` from leaves the caller
// no better off, which is how a shipped release silently kept serving
// tracked-work commands (T-123). The install target must therefore verify its own
// output is reachable, not just that the build succeeded.
func TestTaskrailInstallGuardsPathReachability(t *testing.T) {
	taskfile := readFile(t, repoRoot(t), "Taskfile.yml")
	block := strings.Join(taskfileBlock(taskfile, "taskrail:install:"), "\n")
	if !strings.Contains(block, "cmd/pathcheck") {
		t.Errorf("taskrail:install must verify its output is reachable as `taskrail`; got:\n%s", block)
	}
}

// The freshness guard can only tell "stale" from "you are running a different
// binary" if it knows where taskrail:install writes. Passing that path is what
// makes its two messages distinguishable (T-123).
func TestTaskrailCheckKnowsTheWorkingTreeBuildPath(t *testing.T) {
	taskfile := readFile(t, repoRoot(t), "Taskfile.yml")
	// Strip go-template expressions (the Windows .exe suffix) so the invocation
	// splits into plain arguments.
	block := regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(
		strings.Join(taskfileBlock(taskfile, "taskrail:check:"), "\n"), "")
	freshcheck := regexp.MustCompile(`cmd/freshcheck\s+(\S+)\s+(\S+)`).FindStringSubmatch(block)
	if freshcheck == nil {
		t.Fatalf("taskrail:check must pass the working-tree build path to freshcheck; got:\n%s", block)
	}
	if !strings.HasPrefix(freshcheck[2], "bin/taskrail") {
		t.Errorf("freshcheck's working-tree build argument %q must be the taskrail:install output", freshcheck[2])
	}
}

// Tracked-work commands write task and state files, so a stale binary here
// corrupts committed workflow state silently — the expensive variant of the
// T-123 trap. The opt-in pre-commit hook closes that window locally by refusing
// a commit whose binary is not the working-tree build.
func TestLefthookGuardsBinaryFreshnessBeforeCommit(t *testing.T) {
	lefthook := readFile(t, repoRoot(t), "lefthook.yml")
	_, stage, found := strings.Cut(lefthook, "pre-commit:")
	if !found {
		t.Fatal("lefthook.yml must define a pre-commit stage")
	}
	// The stage ends where the next one begins; absent a later stage it runs to
	// the end of the file, which is what Cut's miss already returns.
	stage, _, _ = strings.Cut(stage, "\ncommit-msg:")
	if !strings.Contains(stage, "task taskrail:check") {
		t.Errorf("lefthook pre-commit must run `task taskrail:check` so a stale binary cannot write committed state; got:\n%s", stage)
	}
}

// ciJobBlocks splits a workflow file into its top-level jobs, keyed by job id.
// Job ids sit at two-space indent under `jobs:`; anything shallower ends the
// section. Splitting by job is what lets a guard assert ordering *within* a job
// rather than across the whole file.
func ciJobBlocks(content string) map[string][]string {
	jobs := map[string][]string{}
	id := regexp.MustCompile(`^  ([a-z][a-z0-9-]*):\s*$`)
	current := ""
	inJobs := false
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "jobs:") {
			inJobs = true
			continue
		}
		if !inJobs {
			continue
		}
		if m := id.FindStringSubmatch(line); m != nil {
			current = m[1]
			continue
		}
		if current != "" {
			jobs[current] = append(jobs[current], line)
		}
	}
	return jobs
}

// A job id in one place and a step in another must not be conflated: the
// splitter has to attribute each line to the job it sits under.
func TestCIJobBlocksAttributesStepsToJobs(t *testing.T) {
	jobs := ciJobBlocks("jobs:\n  first:\n    steps:\n      - run: a\n  second:\n    steps:\n      - run: b\n")
	if len(jobs) != 2 {
		t.Fatalf("want two jobs, got %d: %v", len(jobs), jobs)
	}
	if !strings.Contains(strings.Join(jobs["first"], "\n"), "run: a") ||
		strings.Contains(strings.Join(jobs["first"], "\n"), "run: b") {
		t.Errorf("steps attributed to the wrong job: %v", jobs)
	}
}

// firstIndexContaining returns the index of the first non-comment line
// containing any of needles, or -1. Comment lines are skipped so a step
// annotated with the command it guards cannot masquerade as that command.
func firstIndexContaining(lines []string, needles ...string) int {
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		for _, needle := range needles {
			if strings.Contains(line, needle) {
				return i
			}
		}
	}
	return -1
}

// mise-action does not propagate [env] _.path to workflow steps, so any CI job
// that installs the working-tree binary must first expose the bin directory
// through GitHub's PATH handoff. Without that the install guard would fail on the
// runner for the very reason it exists to report (T-123).
func TestCIExposesBinBeforeBuildingTaskrail(t *testing.T) {
	ci := readFile(t, repoRoot(t), ".github/workflows/ci.yml")
	for job, lines := range ciJobBlocks(ci) {
		install := firstIndexContaining(lines, "task taskrail:install", "task test")
		if install < 0 {
			continue
		}
		expose := firstIndexContaining(lines, `/bin" >> "$GITHUB_PATH"`)
		if expose < 0 {
			t.Errorf("ci.yml job %q builds the working-tree taskrail but never exposes bin/ on the runner PATH", job)
			continue
		}
		if expose > install {
			t.Errorf("ci.yml job %q must expose bin/ on the runner PATH before building the working-tree taskrail", job)
		}
	}
}

// CI must exercise the freshness guard so a stale on-PATH binary is caught in
// the pipeline, not silently trusted.
func TestCIRunsTaskrailFreshnessCheck(t *testing.T) {
	ci := readFile(t, repoRoot(t), ".github/workflows/ci.yml")
	if !strings.Contains(ci, "task taskrail:check") {
		t.Error("ci.yml must run `task taskrail:check` to guard binary freshness")
	}
}

// taskrailEnvOverride reports lines that *assign* a TASKRAIL environment
// variable (shell `TASKRAIL=`/`export TASKRAIL`, YAML `TASKRAIL:`), which the
// contract forbids. A read of the fallback (`${TASKRAIL:-taskrail}`) is not an
// assignment and must not be flagged.
func taskrailEnvOverride(content string) []string {
	shellSet := regexp.MustCompile(`(^|\s|export\s+)TASKRAIL\s*=`)
	yamlSet := regexp.MustCompile(`^\s*TASKRAIL\s*:`)
	var offenders []string
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "${TASKRAIL") {
			continue // fallback read, not an override
		}
		if shellSet.MatchString(line) || yamlSet.MatchString(line) {
			offenders = append(offenders, strings.TrimSpace(line))
		}
	}
	return offenders
}

func TestTaskrailEnvOverrideDetectorIgnoresFallback(t *testing.T) {
	if got := taskrailEnvOverride(`bin="${TASKRAIL:-taskrail}"`); got != nil {
		t.Errorf("fallback read flagged as override: %v", got)
	}
	if got := taskrailEnvOverride(`export TASKRAIL=/tmp/x`); len(got) == 0 {
		t.Error("shell assignment not detected as override")
	}
}

// No TASKRAIL env override may be set anywhere: the mise PATH contract makes the
// plain fallback correct by construction, so an override would only mask drift.
func TestNoTaskrailEnvOverride(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{"mise.toml", "Taskfile.yml", ".github/workflows/ci.yml"} {
		if offenders := taskrailEnvOverride(readFile(t, root, rel)); len(offenders) > 0 {
			t.Errorf("%s sets a TASKRAIL override; the mise PATH fallback must be used instead:\n%s",
				rel, strings.Join(offenders, "\n"))
		}
	}
}

// The working-tree binary is a build artifact and must stay out of version
// control, mirroring the existing /taskrail ignore.
func TestBinDirGitignored(t *testing.T) {
	gitignore := readFile(t, repoRoot(t), ".gitignore")
	if !regexp.MustCompile(`(?m)^/?bin/?\s*$`).MatchString(gitignore) {
		t.Error(".gitignore must ignore the repo-local bin/ directory")
	}
}
