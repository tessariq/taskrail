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
