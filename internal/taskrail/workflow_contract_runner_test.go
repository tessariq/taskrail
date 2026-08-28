package taskrail

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestSelectWorkflowSuiteTestsRejectsUnmatchedSelector(t *testing.T) {
	suite := WorkflowContractSuite{Name: "unmatched", TestRun: "^TestAbsent$"}
	_, err := selectWorkflowSuiteTests(suite, []string{"TestPresent"})
	if err == nil || !strings.Contains(err.Error(), "selects no tests") {
		t.Fatalf("selectWorkflowSuiteTests error = %v, want unmatched-selector rejection", err)
	}
}

func TestWorkflowTestEventsRejectsMissingOrUnexpectedTests(t *testing.T) {
	output := []byte("{\"Action\":\"run\",\"Test\":\"TestPresent\"}\n{\"Action\":\"pass\",\"Test\":\"TestPresent\"}\n")
	run, passed, err := workflowTestEvents(output)
	if err != nil {
		t.Fatalf("workflowTestEvents: %v", err)
	}
	if got, want := run, []string{"TestPresent"}; !slices.Equal(got, want) {
		t.Errorf("run = %v, want %v", got, want)
	}
	if got, want := passed, []string{"TestPresent"}; !slices.Equal(got, want) {
		t.Errorf("passed = %v, want %v", got, want)
	}
}

func TestValidateWorkflowSuiteExecutionRejectsCountDrift(t *testing.T) {
	suite := WorkflowContractSuite{Name: "count-drift"}
	for _, tc := range []struct {
		name     string
		executed []string
		passed   []string
		want     string
	}{
		{name: "missing execution", executed: nil, passed: nil, want: "executed tests"},
		{name: "unexpected execution", executed: []string{"TestExpected", "TestExtra"}, passed: []string{"TestExpected", "TestExtra"}, want: "executed tests"},
		{name: "missing pass", executed: []string{"TestExpected"}, passed: nil, want: "passed tests"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWorkflowSuiteExecution(suite, []string{"TestExpected"}, tc.executed, tc.passed)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("validateWorkflowSuiteExecution error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoopLaunchSuiteSelectorsSeparatePortableContainmentAndHelper(t *testing.T) {
	manifest, err := WorkflowContractTestSurfaceManifest()
	if err != nil {
		t.Fatalf("WorkflowContractTestSurfaceManifest: %v", err)
	}
	listed := []string{
		"TestLoopLaunchChildTransportsExactPromptAndIdentity",
		"TestLoopLaunchChildContainsUnixProcessGroup",
		"TestLoopLaunchChildTerminatesOnContextCancellation",
		"TestLoopLaunchChildHelper",
	}
	want := map[string][]string{
		"loop-launch-portable":  {"TestLoopLaunchChildTransportsExactPromptAndIdentity"},
		"loop-containment-unix": {"TestLoopLaunchChildContainsUnixProcessGroup", "TestLoopLaunchChildTerminatesOnContextCancellation"},
	}
	for _, suite := range manifest.Suites {
		expected, ok := want[suite.Name]
		if !ok {
			continue
		}
		selected, err := selectWorkflowSuiteTests(suite, listed)
		if err != nil {
			t.Fatalf("select %s: %v", suite.Name, err)
		}
		if !reflect.DeepEqual(selected, expected) {
			t.Errorf("%s selected %v, want %v", suite.Name, selected, expected)
		}
		delete(want, suite.Name)
	}
	if len(want) != 0 {
		t.Fatalf("manifest omits launch suites: %v", want)
	}
}
