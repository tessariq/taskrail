package toolchain_test

import (
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/taskrail"
)

// The workflow-contract runner owns the suite selectors. The Task target must
// invoke it unchanged so local and CI coverage cannot drift into separate lists.
func TestTaskfileDefinesWorkflowContractTarget(t *testing.T) {
	root := repoRoot(t)
	target := strings.Join(taskfileBlock(readFile(t, root, "Taskfile.yml"), "test:workflow-contract:"), "\n")
	if target == "" {
		t.Fatal("Taskfile.yml must define a test:workflow-contract target")
	}
	if !strings.Contains(target, "go run ./internal/taskrail/cmd/workflow-contract") {
		t.Error("test:workflow-contract must invoke the repository-local workflow-contract runner")
	}
	if strings.Contains(target, "go test") || strings.Contains(target, "-run") {
		t.Error("test:workflow-contract must not recreate workflow suite selectors")
	}
}

// Every native build-test leg must consume the one local target. The manifest
// owns platform classification; this test only protects the CI wiring that
// dispatches it to the Linux, macOS, and native Windows matrix assignments.
func TestCIWiresWorkflowContractTargetToEveryNativeMatrixLeg(t *testing.T) {
	ci := readFile(t, repoRoot(t), ".github/workflows/ci.yml")
	job := ciJobBlock(ci, "build-test")
	if job == "" {
		t.Fatal("ci.yml must define a build-test job")
	}

	runners := ciMatrixRunners(job)
	wantRunners := []string{"ubuntu-latest", "ubuntu-24.04-arm", "windows-latest", "macos-latest"}
	if strings.Join(runners, ",") != strings.Join(wantRunners, ",") {
		t.Fatalf("build-test runners = %v, want %v", runners, wantRunners)
	}

	found := 0
	for _, step := range workflowStepBlocks(job) {
		if !hasActiveYAMLLine(step, "run: task test:workflow-contract") {
			continue
		}
		found++
		if !hasActiveYAMLLine(step, "- name: Workflow contract") {
			t.Error("workflow-contract command must belong to the named Workflow contract step")
		}
		if hasActiveYAMLKey(step, "if:") {
			t.Error("workflow-contract target must run unconditionally on every build-test matrix leg")
		}
		if hasActiveYAMLKey(step, "continue-on-error:") {
			t.Error("workflow-contract step must fail the build-test matrix leg")
		}
	}
	if found != 1 {
		t.Errorf("build-test must contain exactly one workflow-contract target step, found %d", found)
	}
	if hasActiveYAMLKey(strings.Split(job, "\n"), "continue-on-error:") {
		t.Error("build-test job must not permit failures through continue-on-error")
	}

	manifest, err := taskrail.WorkflowContractTestSurfaceManifest()
	if err != nil {
		t.Fatalf("WorkflowContractTestSurfaceManifest: %v", err)
	}
	for _, suite := range manifest.Suites {
		if !matrixSupportsWorkflowSuite(runners, suite.Platform) {
			t.Errorf("build-test runners %v do not cover workflow suite %q platform %q", runners, suite.Name, suite.Platform)
		}
	}
}

func TestPlanningLaneRunsWorkflowContractForDocumentationChanges(t *testing.T) {
	planning := readFile(t, repoRoot(t), ".github/workflows/planning.yml")
	job := ciJobBlock(planning, "validate")
	found := 0
	for _, step := range workflowStepBlocks(job) {
		if !hasActiveYAMLLine(step, "run: task test:workflow-contract") {
			continue
		}
		found++
		if !hasActiveYAMLLine(step, "- name: Workflow contract") || hasActiveYAMLKey(step, "if:") || hasActiveYAMLKey(step, "continue-on-error:") {
			t.Error("planning workflow contract step must be named, unconditional, and blocking")
		}
	}
	if found != 1 {
		t.Errorf("planning lane must contain exactly one workflow-contract target step, found %d", found)
	}
	if hasActiveYAMLKey(strings.Split(job, "\n"), "continue-on-error:") {
		t.Error("planning validation job must not permit failures through continue-on-error")
	}
}

func TestActiveYAMLLineChecksIgnoreComments(t *testing.T) {
	lines := []string{"      # run: task test:workflow-contract", "      # continue-on-error: true"}
	if hasActiveYAMLLine(lines, "run: task test:workflow-contract") || hasActiveYAMLKey(lines, "continue-on-error:") {
		t.Fatal("commented workflow fields must not satisfy active CI wiring checks")
	}
}

func hasActiveYAMLLine(lines []string, want string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") && trimmed == want {
			return true
		}
	}
	return false
}

func hasActiveYAMLKey(lines []string, key string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") && strings.HasPrefix(trimmed, key) {
			return true
		}
	}
	return false
}

func ciJobBlock(content, job string) string {
	var lines []string
	inJob := false
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
			if strings.TrimSpace(line) == job+":" {
				inJob = true
			} else if inJob {
				break
			}
		}
		if inJob {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func matrixSupportsWorkflowSuite(runners []string, platform taskrail.WorkflowSuitePlatform) bool {
	for _, runner := range runners {
		switch platform {
		case taskrail.WorkflowSuitePortable:
			return true
		case taskrail.WorkflowSuiteUnix:
			if strings.HasPrefix(runner, "ubuntu-") || strings.HasPrefix(runner, "macos-") {
				return true
			}
		case taskrail.WorkflowSuiteWindows:
			if strings.HasPrefix(runner, "windows-") {
				return true
			}
		}
	}
	return false
}
