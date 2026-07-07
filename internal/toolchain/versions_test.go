// Package toolchain holds guard tests that keep the pinned developer toolchain
// (mise.toml) consistent with the versions referenced elsewhere in the repo.
//
// The versions in mise.toml are meant to be the single source of truth for the
// developer environment (see specs/v0.2.0.md#mise-toolchain-management): the Go
// pin must match go.mod, and the lefthook pin must match the version the
// `task hooks:install` guidance installs (Taskfile.yml and README.md). These
// tests fail loudly when those drift apart.
package toolchain_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoRoot walks up from the test's working directory until it finds go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repo root (go.mod not found walking up)")
		}
		dir = parent
	}
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// miseTool extracts a tool pin from the [tools] table of mise.toml, e.g.
// `go = "1.26"` -> "1.26". It only scans the [tools] section so a task named
// after a tool cannot shadow the pin.
func miseTool(t *testing.T, mise, tool string) string {
	t.Helper()
	inTools := false
	re := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(tool) + `\s*=\s*"([^"]+)"`)
	for _, line := range strings.Split(mise, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			// Strip a trailing inline comment so `[tools] # note` still counts.
			header := strings.TrimSpace(strings.SplitN(trimmed, "#", 2)[0])
			inTools = header == "[tools]"
			continue
		}
		if !inTools {
			continue
		}
		if m := re.FindStringSubmatch(line); m != nil {
			return m[1]
		}
	}
	t.Fatalf("tool %q not found in [tools] table of mise.toml", tool)
	return ""
}

func TestMiseGoPinMatchesGoMod(t *testing.T) {
	root := repoRoot(t)
	goMod := readFile(t, root, "go.mod")
	mise := readFile(t, root, "mise.toml")

	m := regexp.MustCompile(`(?m)^go\s+(\d+\.\d+(?:\.\d+)?)\b`).FindStringSubmatch(goMod)
	if m == nil {
		t.Fatal("could not find go directive in go.mod")
	}
	goModVersion := m[1]

	if got := miseTool(t, mise, "go"); got != goModVersion {
		t.Errorf("mise.toml go pin %q does not match go.mod go directive %q", got, goModVersion)
	}
}

// lefthookRefs returns every version in a `lefthook@vX.Y.Z` reference
// (Taskfile.yml, README.md) without the leading v, matching mise's pin form.
// Returning all matches — not just the first — keeps a later stale install line
// from hiding behind an up-to-date one earlier in the file.
func lefthookRefs(t *testing.T, content, rel string) []string {
	t.Helper()
	matches := regexp.MustCompile(`lefthook@v(\d+\.\d+\.\d+)`).FindAllStringSubmatch(content, -1)
	if matches == nil {
		t.Fatalf("no lefthook@vX.Y.Z reference found in %s", rel)
	}
	versions := make([]string, len(matches))
	for i, m := range matches {
		versions[i] = m[1]
	}
	return versions
}

func TestMiseLefthookPinMatchesHookGuidance(t *testing.T) {
	root := repoRoot(t)
	mise := readFile(t, root, "mise.toml")
	misePin := miseTool(t, mise, "lefthook")

	for _, rel := range []string{"Taskfile.yml", "README.md"} {
		for _, ref := range lefthookRefs(t, readFile(t, root, rel), rel) {
			if ref != misePin {
				t.Errorf("lefthook pin mismatch: mise.toml=%q %s=%q", misePin, rel, ref)
			}
		}
	}
}
