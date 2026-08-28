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
	run, passed, skipped, err := workflowTestEvents(output)
	if err != nil {
		t.Fatalf("workflowTestEvents: %v", err)
	}
	if got, want := run, []string{"TestPresent"}; !slices.Equal(got, want) {
		t.Errorf("run = %v, want %v", got, want)
	}
	if got, want := passed, []string{"TestPresent"}; !slices.Equal(got, want) {
		t.Errorf("passed = %v, want %v", got, want)
	}
	if len(skipped) != 0 {
		t.Errorf("skipped = %v, want none", skipped)
	}
}

func TestWorkflowTestEventsRecordsExplicitSkipReasons(t *testing.T) {
	output := []byte("{\"Action\":\"run\",\"Test\":\"TestUnsupported\"}\n" +
		"{\"Action\":\"output\",\"Test\":\"TestUnsupported\",\"Output\":\"=== PAUSE TestUnsupported\\n\"}\n" +
		"{\"Action\":\"output\",\"Test\":\"TestUnsupported\",\"Output\":\"=== CONT  TestUnsupported\\n\"}\n" +
		"{\"Action\":\"output\",\"Test\":\"TestUnsupported\",\"Output\":\"    contract_test.go:42: native capability unavailable\\n\"}\n" +
		"{\"Action\":\"skip\",\"Test\":\"TestUnsupported\"}\n")
	_, _, skipped, err := workflowTestEvents(output)
	if err != nil {
		t.Fatalf("workflowTestEvents: %v", err)
	}
	want := []WorkflowContractTestSkip{{Name: "TestUnsupported", Reason: "native capability unavailable"}}
	if !reflect.DeepEqual(skipped, want) {
		t.Errorf("skipped = %#v, want %#v", skipped, want)
	}
}

func TestWorkflowTestEventsLeavesBareSkipReasonEmpty(t *testing.T) {
	output := []byte("{\"Action\":\"run\",\"Test\":\"TestUnsupported\"}\n" +
		"{\"Action\":\"output\",\"Test\":\"TestUnsupported\",\"Output\":\"    contract_test.go:41: setup completed\\n\"}\n" +
		"{\"Action\":\"output\",\"Test\":\"TestUnsupported\",\"Output\":\"    contract_test.go:42: \\n\"}\n" +
		"{\"Action\":\"skip\",\"Test\":\"TestUnsupported\"}\n")
	_, _, skipped, err := workflowTestEvents(output)
	if err != nil {
		t.Fatalf("workflowTestEvents: %v", err)
	}
	want := []WorkflowContractTestSkip{{Name: "TestUnsupported"}}
	if !reflect.DeepEqual(skipped, want) {
		t.Errorf("skipped = %#v, want %#v", skipped, want)
	}
}

func TestValidateWorkflowSuiteExecutionRejectsCountDrift(t *testing.T) {
	suite := WorkflowContractSuite{Name: "count-drift"}
	for _, tc := range []struct {
		name     string
		executed []string
		passed   []string
		skipped  []WorkflowContractTestSkip
		want     string
	}{
		{name: "missing execution", executed: nil, passed: nil, want: "executed tests"},
		{name: "unexpected execution", executed: []string{"TestExpected", "TestExtra"}, passed: []string{"TestExpected", "TestExtra"}, want: "executed tests"},
		{name: "missing terminal result", executed: []string{"TestExpected"}, want: "terminal results"},
		{name: "unreasoned skip", executed: []string{"TestExpected"}, skipped: []WorkflowContractTestSkip{{Name: "TestExpected"}}, want: "empty reason"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWorkflowSuiteExecution(suite, []string{"TestExpected"}, tc.executed, tc.passed, tc.skipped)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("validateWorkflowSuiteExecution error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestValidateWorkflowSuiteExecutionAcceptsExplicitSkip(t *testing.T) {
	suite := WorkflowContractSuite{Name: "native-capability"}
	skipped := []WorkflowContractTestSkip{{Name: "TestExpected", Reason: "native capability unavailable"}}
	if err := validateWorkflowSuiteExecution(suite, []string{"TestExpected"}, []string{"TestExpected"}, nil, skipped); err != nil {
		t.Fatalf("validateWorkflowSuiteExecution: %v", err)
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

func TestWorkflowSuiteSkipReasonsAreExplicitAndStable(t *testing.T) {
	for _, tc := range []struct {
		name     string
		platform WorkflowSuitePlatform
		goos     string
		want     string
	}{
		{name: "portable", platform: WorkflowSuitePortable, goos: "windows"},
		{name: "unix on Windows", platform: WorkflowSuiteUnix, goos: "windows", want: "requires Unix process containment"},
		{name: "Windows on Linux", platform: WorkflowSuiteWindows, goos: "linux", want: "requires native Windows process containment"},
		{name: "Unix on macOS", platform: WorkflowSuiteUnix, goos: "darwin"},
		{name: "Windows on Windows", platform: WorkflowSuiteWindows, goos: "windows"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := workflowSuiteSkipReason(tc.platform, tc.goos); got != tc.want {
				t.Errorf("workflowSuiteSkipReason(%q, %q) = %q, want %q", tc.platform, tc.goos, got, tc.want)
			}
		})
	}
}
