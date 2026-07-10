package toolchain_test

import (
	"regexp"
	"strings"
	"testing"
)

// requiredCrossTargets are the non-native GOOS/GOARCH pairs the CI cross-compile
// smoke must build ./cmd/taskrail for (T-073, specs/v0.3.0.md#goals). windows/arm64
// is the optional release target T-058 may add; compile-checking it here keeps the
// optional artifact from breaking silently between tags.
var requiredCrossTargets = []string{
	"windows/amd64",
	"windows/arm64",
	"darwin/amd64",
	"darwin/arm64",
	"linux/arm64",
}

// ciRunSteps returns the command from every `run:` directive in a workflow file,
// skipping comment lines and trailing inline comments, mirroring ciActionUses.
func ciRunSteps(content string) []string {
	var runs []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		directive := strings.TrimPrefix(trimmed, "- ") // steps write `- run:` or `run:`
		if !strings.HasPrefix(directive, "run:") {
			continue
		}
		cmd := strings.TrimSpace(strings.TrimPrefix(directive, "run:"))
		if i := strings.Index(cmd, " #"); i >= 0 {
			cmd = strings.TrimSpace(cmd[:i])
		}
		if cmd != "" {
			runs = append(runs, cmd)
		}
	}
	return runs
}

// The cross-compile smoke must be routed through a Taskfile target (consistent
// with T-074's `task ...` contract), not an inline `go build` matrix in CI. A raw
// `go build` step would bypass Taskfile.yml as the single source of build commands
// and trip the raw-go guard.
func TestCIInvokesCrossCompileViaTask(t *testing.T) {
	root := repoRoot(t)
	runs := ciRunSteps(readFile(t, root, ".github/workflows/ci.yml"))

	taskRe := regexp.MustCompile(`^task\s+build:cross\b`)
	found := false
	for _, cmd := range runs {
		if taskRe.MatchString(cmd) {
			found = true
		}
	}
	if !found {
		t.Errorf("ci.yml must run `task build:cross` for the cross-compile smoke; run steps: %v", runs)
	}
}

// The build:cross Taskfile target must compile ./cmd/taskrail for every non-native
// release target so a platform-specific compile break is caught on each push/PR
// rather than only at tag time when GoReleaser runs.
func TestTaskfileCrossTargetCoversRequiredPlatforms(t *testing.T) {
	root := repoRoot(t)
	taskfile := readFile(t, root, "Taskfile.yml")

	if !strings.Contains(taskfile, "build:cross:") {
		t.Fatal("Taskfile.yml must define a build:cross target for the cross-compile smoke")
	}
	for _, target := range requiredCrossTargets {
		if !strings.Contains(taskfile, target) {
			t.Errorf("Taskfile.yml build:cross must build %q; missing from target list", target)
		}
	}
}
