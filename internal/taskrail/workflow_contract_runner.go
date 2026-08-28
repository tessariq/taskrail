package taskrail

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

// WorkflowContractSuiteResult records the exact top-level tests a suite ran.
type WorkflowContractSuiteResult struct {
	Name          string   `json:"name"`
	Package       string   `json:"package"`
	Outcome       string   `json:"outcome"`
	SkipReason    string   `json:"skip_reason,omitempty"`
	ExecutedTests []string `json:"executed_tests"`
}

// RunWorkflowContractSuites runs the current platform's suites from the exact
// generated manifest. It is repository-local CI plumbing, not a product command.
func RunWorkflowContractSuites(ctx context.Context, repoRoot string) ([]WorkflowContractSuiteResult, error) {
	manifest, err := WorkflowContractTestSurfaceManifest()
	if err != nil {
		return nil, err
	}
	if err := ValidateWorkflowContractTestSurfaceManifest(manifest); err != nil {
		return nil, err
	}

	results := make([]WorkflowContractSuiteResult, 0, len(manifest.Suites))
	for _, suite := range manifest.Suites {
		if reason := workflowSuiteSkipReason(suite.Platform, runtime.GOOS); reason != "" {
			results = append(results, WorkflowContractSuiteResult{
				Name: suite.Name, Package: suite.Package, Outcome: "skipped", SkipReason: reason,
				ExecutedTests: []string{},
			})
			continue
		}
		listed, err := listWorkflowSuiteTests(ctx, repoRoot, suite)
		if err != nil {
			return nil, err
		}
		selected, err := selectWorkflowSuiteTests(suite, listed)
		if err != nil {
			return nil, err
		}
		executed, err := executeWorkflowSuite(ctx, repoRoot, suite, selected)
		if err != nil {
			return nil, err
		}
		results = append(results, WorkflowContractSuiteResult{
			Name: suite.Name, Package: suite.Package, Outcome: "passed", ExecutedTests: executed,
		})
	}
	return results, nil
}

func workflowSuiteSkipReason(platform WorkflowSuitePlatform, goos string) string {
	switch platform {
	case WorkflowSuitePortable:
		return ""
	case WorkflowSuiteUnix:
		if goos == "windows" {
			return "requires Unix process containment"
		}
	case WorkflowSuiteWindows:
		if goos != "windows" {
			return "requires native Windows process containment"
		}
	}
	return ""
}

func listWorkflowSuiteTests(ctx context.Context, repoRoot string, suite WorkflowContractSuite) ([]string, error) {
	output, err := runWorkflowGoTest(ctx, repoRoot, "-list", suite.TestRun, suite.Package)
	if err != nil {
		return nil, fmt.Errorf("list workflow contract suite %q: %w", suite.Name, err)
	}
	var tests []string
	for _, line := range strings.Fields(string(output)) {
		if strings.HasPrefix(line, "Test") {
			tests = append(tests, line)
		}
	}
	return tests, nil
}

func selectWorkflowSuiteTests(suite WorkflowContractSuite, listed []string) ([]string, error) {
	selector, err := regexp.Compile(suite.TestRun)
	if err != nil {
		return nil, fmt.Errorf("workflow contract suite %q has invalid test pattern: %w", suite.Name, err)
	}
	selected := make([]string, 0, len(listed))
	for _, name := range listed {
		if selector.MatchString(name) {
			selected = append(selected, name)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("workflow contract suite %q selector %q selects no tests", suite.Name, suite.TestRun)
	}
	slices.Sort(selected)
	return selected, nil
}

func executeWorkflowSuite(ctx context.Context, repoRoot string, suite WorkflowContractSuite, expected []string) ([]string, error) {
	output, err := runWorkflowGoTest(ctx, repoRoot, "-count=1", "-json", "-run", suite.TestRun, suite.Package)
	if err != nil {
		return nil, fmt.Errorf("run workflow contract suite %q: %w", suite.Name, err)
	}
	executed, passed, err := workflowTestEvents(output)
	if err != nil {
		return nil, fmt.Errorf("read workflow contract suite %q results: %w", suite.Name, err)
	}
	if err := validateWorkflowSuiteExecution(suite, expected, executed, passed); err != nil {
		return nil, err
	}
	return executed, nil
}

func validateWorkflowSuiteExecution(suite WorkflowContractSuite, expected, executed, passed []string) error {
	if !slices.Equal(executed, expected) {
		return fmt.Errorf("workflow contract suite %q executed tests %v, want %v", suite.Name, executed, expected)
	}
	if !slices.Equal(passed, expected) {
		return fmt.Errorf("workflow contract suite %q passed tests %v, want %v", suite.Name, passed, expected)
	}
	return nil
}

func runWorkflowGoTest(ctx context.Context, repoRoot string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "go", append([]string{"test"}, args...)...)
	command.Dir = filepath.Clean(repoRoot)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("go test %s: %w\n%s", strings.Join(args, " "), err, output.String())
	}
	return output.Bytes(), nil
}

func workflowTestEvents(output []byte) ([]string, []string, error) {
	type event struct {
		Action string
		Test   string
	}
	seenRun := map[string]struct{}{}
	seenPass := map[string]struct{}{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		var event event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		if event.Test == "" || strings.Contains(event.Test, "/") {
			continue
		}
		switch event.Action {
		case "run":
			seenRun[event.Test] = struct{}{}
		case "pass":
			seenPass[event.Test] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	return sortedWorkflowTestNames(seenRun), sortedWorkflowTestNames(seenPass), nil
}

func sortedWorkflowTestNames(names map[string]struct{}) []string {
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	slices.Sort(result)
	return result
}
