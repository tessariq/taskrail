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
