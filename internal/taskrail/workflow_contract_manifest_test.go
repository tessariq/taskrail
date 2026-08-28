package taskrail

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestWorkflowContractManifestDerivesAuthoritativeInventories(t *testing.T) {
	manifest, err := WorkflowContractTestSurfaceManifest()
	if err != nil {
		t.Fatalf("WorkflowContractTestSurfaceManifest: %v", err)
	}
	if err := ValidateWorkflowContractTestSurfaceManifest(manifest); err != nil {
		t.Fatalf("ValidateWorkflowContractTestSurfaceManifest: %v", err)
	}

	promptAssets, err := embeddedPromptAssets()
	if err != nil {
		t.Fatalf("embeddedPromptAssets: %v", err)
	}
	if got, want := manifest.PromptAssets, promptAssets; !reflect.DeepEqual(got, want) {
		t.Fatalf("prompt assets = %v, want embedded assets %v", got, want)
	}
	if got, want := manifest.Skills, shippableSkills; !reflect.DeepEqual(got, want) {
		t.Fatalf("skills = %v, want shippable skills %v", got, want)
	}
	if len(manifest.MachineDocuments) != len(MachineCommandInventory()) {
		t.Fatalf("machine documents = %d, want %d", len(manifest.MachineDocuments), len(MachineCommandInventory()))
	}
}

func TestWorkflowContractManifestRejectsInventoryDrift(t *testing.T) {
	manifest, err := WorkflowContractTestSurfaceManifest()
	if err != nil {
		t.Fatalf("WorkflowContractTestSurfaceManifest: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*WorkflowContractManifest)
		want   string
	}{
		{
			name: "missing embedded prompt",
			mutate: func(m *WorkflowContractManifest) {
				m.PromptAssets = m.PromptAssets[1:]
			},
			want: "prompt assets",
		},
		{
			name: "invented packaged skill",
			mutate: func(m *WorkflowContractManifest) {
				m.Skills = append(m.Skills, "invented")
			},
			want: "skills",
		},
		{
			name: "missing machine document",
			mutate: func(m *WorkflowContractManifest) {
				m.MachineDocuments = m.MachineDocuments[1:]
			},
			want: "machine documents",
		},
		{
			name: "portable containment assertion",
			mutate: func(m *WorkflowContractManifest) {
				for i := range m.Suites {
					if m.Suites[i].Name == "loop-containment-unix" {
						m.Suites[i].Platform = WorkflowSuitePortable
					}
				}
			},
			want: "loop-containment-unix",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := cloneWorkflowContractManifest(manifest)
			tc.mutate(&candidate)
			if err := ValidateWorkflowContractTestSurfaceManifest(candidate); err == nil || !containsAny(err.Error(), tc.want) {
				t.Fatalf("ValidateWorkflowContractTestSurfaceManifest error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestValidateWorkflowPromptRegistryRejectsDuplicateIdentityKeys(t *testing.T) {
	original := slices.Clone(promptRegistry)
	t.Cleanup(func() { promptRegistry = original })

	promptRegistry = append(promptRegistry, promptDefinition{
		id: original[0].id, contract: original[0].contract, asset: "prompts/v1/other.md",
	})
	if err := validateWorkflowPromptRegistry(); err == nil || !strings.Contains(err.Error(), "identity key") {
		t.Fatalf("validateWorkflowPromptRegistry error = %v, want duplicate identity-key rejection", err)
	}
}

func TestValidateWorkflowPromptRegistryRejectsDuplicatePromptIDsAcrossContracts(t *testing.T) {
	original := slices.Clone(promptRegistry)
	t.Cleanup(func() { promptRegistry = original })

	promptRegistry = append(promptRegistry, promptDefinition{
		id: original[0].id, contract: "v2", asset: "prompts/v2/other.md",
	})
	if err := validateWorkflowPromptRegistry(); err == nil || !strings.Contains(err.Error(), "prompt identity") {
		t.Fatalf("validateWorkflowPromptRegistry error = %v, want duplicate prompt-ID rejection", err)
	}
}

func TestWorkflowContractManifestClassifiesNativeContainmentSuites(t *testing.T) {
	manifest, err := WorkflowContractTestSurfaceManifest()
	if err != nil {
		t.Fatalf("WorkflowContractTestSurfaceManifest: %v", err)
	}
	want := map[string]WorkflowSuitePlatform{
		"loop-launch-portable":     WorkflowSuitePortable,
		"loop-containment-unix":    WorkflowSuiteUnix,
		"loop-containment-windows": WorkflowSuiteWindows,
	}
	for _, suite := range manifest.Suites {
		if platform, ok := want[suite.Name]; ok {
			if suite.Platform != platform {
				t.Errorf("%s platform = %q, want %q", suite.Name, suite.Platform, platform)
			}
			delete(want, suite.Name)
		}
	}
	for name := range want {
		t.Errorf("manifest omits platform-specific suite %q", name)
	}
}

func TestWorkflowContractCurrentGuidanceRetiresSourceCheckoutLoopAndKeepsProviderBoundary(t *testing.T) {
	guidance := map[string][]string{
		"CHANGELOG.md": {
			"## Unreleased", "provider-neutral", "workflow",
		},
		"README.md": {
			"provider-agnostic", "caller-owned executable", "does not embed credentials",
		},
		"AGENTS.md": {
			"Never hand-edit", "taskrail start", "taskrail validate",
		},
		"docs/commands.md": {
			"exact prompt template resolution", "without claiming that Taskrail observed or certified",
		},
		"docs/import-contract.md": {
			"provider-agnostic", "loop_policy", "loop_reason",
		},
		"docs/workflow/autonomous-contract.md": {
			"internal/taskrail/lifecycle_contract.go", "task release", "planning/NOTES.md",
		},
		"docs/workflow/agent-steering.md": {
			"one review wave", "context or review-budget exhaustion",
		},
		"docs/workflow/human-workflow.md": {
			"planning/NOTES.md", "direct-operator recovery",
		},
		"docs/workflow/skill-evaluation.md": {
			"does not choose a provider", "no provider runner", "provider output remains",
		},
		"docs/workflow/skills-overview.md": {
			"provider-neutral", "taskrail-sdd-handoff", "taskrail-workflow-adversarial",
		},
		"docs/workflow/skills-productization.md": {
			"provider-agnostic", "does not choose a provider", "taskrail-sdd-handoff",
		},
		"docs/workflow/releasing.md": {
			"do not include provider transcripts",
		},
	}
	for path, phrases := range guidance {
		data, err := os.ReadFile(filepath.Join("..", "..", path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		assertCurrentWorkflowGuidance(t, path, string(data), phrases)
	}

	for path, body := range embeddedSkillFiles(t) {
		assertDurableWorkflowBinding(t, "embedded skill "+path, body)
	}
	for path, body := range embeddedPromptFiles(t) {
		assertDurableWorkflowBinding(t, "embedded prompt "+path, body)
	}
}

func assertCurrentWorkflowGuidance(t *testing.T, path, body string, required []string) {
	t.Helper()
	normalized := strings.Join(strings.Fields(body), " ")
	for _, phrase := range required {
		if !strings.Contains(normalized, strings.Join(strings.Fields(phrase), " ")) {
			t.Errorf("%s must retain %q", path, phrase)
		}
	}
}

func embeddedPromptFiles(t *testing.T) map[string]string {
	t.Helper()
	assets, err := embeddedPromptAssets()
	if err != nil {
		t.Fatalf("list embedded prompts: %v", err)
	}
	files := make(map[string]string, len(assets))
	for _, asset := range assets {
		data, err := builtinPrompts.ReadFile(asset)
		if err != nil {
			t.Fatalf("read %s: %v", asset, err)
		}
		files[asset] = string(data)
	}
	return files
}

func TestDurableWorkflowBindingsRejectRetiredAndLocalGuidance(t *testing.T) {
	for _, fixture := range []string{
		"Run the temporary autonomous loop.",
		"`openai responses create`",
		"openai.exe responses create",
		"curl https://api.openai.com/v1/responses",
		"Taskrail certified reviewer identity.",
		"Taskrail observed prompt delivery.",
		"Taskrail verified semantic quality.",
		"Read /tmp/taskrail/result.json.",
		`Read C:\Users\operator\result.json.`,
	} {
		if violations := durableWorkflowBindingViolations(fixture); len(violations) == 0 {
			t.Errorf("fixture %q must be rejected", fixture)
		}
	}
}

func assertDurableWorkflowBinding(t *testing.T, path, body string) {
	t.Helper()
	for _, violation := range durableWorkflowBindingViolations(body) {
		t.Errorf("%s %s", path, violation)
	}
}

func durableWorkflowBindingViolations(body string) []string {
	lower := strings.ToLower(body)
	var violations []string
	for _, retired := range []string{
		"temporary source-checkout", "source-checkout autonomous loop", "temporary autonomous loop",
	} {
		if strings.Contains(lower, retired) {
			violations = append(violations, fmt.Sprintf("retains retired %q guidance", retired))
		}
	}
	if providerInvocationPattern.MatchString(body) {
		violations = append(violations, "invokes a named provider")
	}
	if providerAPIPattern.MatchString(body) {
		violations = append(violations, "invokes a named provider API")
	}
	if certifiedObservationPattern.MatchString(body) {
		violations = append(violations, "claims deterministic agent observation or certification")
	}
	for _, physicalPath := range []string{"/tmp/", "/home/", "/Users/", "/var/folders/", `C:\Users\`, "C:/Users/"} {
		if strings.Contains(body, physicalPath) {
			violations = append(violations, fmt.Sprintf("contains physical local path %q", physicalPath))
		}
	}
	return violations
}

var providerInvocationPattern = regexp.MustCompile(`(?im)(?:` + "`" + `|^\s*(?:[$#>]\s*)?)(openai|anthropic|gemini|claude|copilot)(?:\.exe)?(?:\s|$)`)
var providerAPIPattern = regexp.MustCompile(`(?i)https?://(?:api\.openai\.com|api\.anthropic\.com|generativelanguage\.googleapis\.com)(?:/|$)`)
var certifiedObservationPattern = regexp.MustCompile(`(?i)\btaskrail\s+(?:observed|certified|verified|proved)\s+(?:the\s+)?(?:prompt delivery|reviewer identity|semantic quality)\b`)
