package taskrail

import (
	"fmt"
	"io/fs"
	"reflect"
	"regexp"
	"slices"
)

// WorkflowSuitePlatform records where a deterministic workflow-contract suite
// can run. CI consumes this boundary without treating host-specific process
// containment guarantees as portable assertions.
type WorkflowSuitePlatform string

const (
	WorkflowSuitePortable WorkflowSuitePlatform = "portable"
	WorkflowSuiteUnix     WorkflowSuitePlatform = "unix"
	WorkflowSuiteWindows  WorkflowSuitePlatform = "windows"
)

// WorkflowContractSuite identifies one focused suite without duplicating its
// feature-level cases in the cross-surface registry.
type WorkflowContractSuite struct {
	Name     string                `json:"name"`
	Package  string                `json:"package"`
	TestRun  string                `json:"test_run"`
	Platform WorkflowSuitePlatform `json:"platform"`
}

// WorkflowMachineDocument is the invocation boundary for one machine result.
type WorkflowMachineDocument struct {
	Command string         `json:"command"`
	Surface MachineSurface `json:"surface"`
}

// WorkflowContractManifest is the generated test-surface index. Its inventories
// come from the package, prompt, skill, and machine registries rather than from
// another hand-maintained list. Only the small suite-to-platform mapping is
// curated here for CI wiring.
type WorkflowContractManifest struct {
	PromptAssets     []string                  `json:"prompt_assets"`
	SkillResources   []string                  `json:"skill_resources"`
	Skills           []string                  `json:"skills"`
	MachineDocuments []WorkflowMachineDocument `json:"machine_documents"`
	Suites           []WorkflowContractSuite   `json:"suites"`
}

var workflowContractSuites = []WorkflowContractSuite{
	{
		Name: "prompt-skill-parity", Package: "./internal/taskrail",
		TestRun:  "^(TestWorkflowContractManifest|TestCommittedSkillsMatchPackage|TestPackagedAndCommittedSkillsAreAgentSkillsCompliant|TestEmbeddedPackageMatchesDeclaredShippableSet|TestShippableSkills|TestFullTaskSkillsFollowCanonicalLifecycle)",
		Platform: WorkflowSuitePortable,
	},
	{
		Name: "machine-schema-invocation", Package: "./internal/taskrail",
		TestRun:  "^(TestMachine|TestCheckMachine|TestOnlyMigratedCommandsPublishTheCommonEnvelope|TestInventoryAloneDrivesStrictEnvelopeDecoding)",
		Platform: WorkflowSuitePortable,
	},
	{
		Name: "local-skill-storage-boundaries", Package: "./internal/taskrail",
		TestRun:  "^(TestPlanLocalSkills|TestLocalSkill|TestStorageNeutral|TestInit.*Skill)",
		Platform: WorkflowSuitePortable,
	},
	{
		Name: "lifecycle-review-sdd-loop-policy", Package: "./internal/taskrail",
		TestRun:  "^(TestCanonicalLifecycle|TestLifecycleContract|TestReviewShow|TestSDDHandoff|TestResolveLoopPolicy|TestValidateLoopPolicy|TestTaskWritersPreserveLoopPolicy)",
		Platform: WorkflowSuitePortable,
	},
	{
		Name: "loop-lock-delegation", Package: "./internal/taskrail",
		TestRun:  "^(TestLoopOwnership|TestLoopGitConfig|TestLoopIntegrity|TestLoop.*Delegat|TestLock)",
		Platform: WorkflowSuitePortable,
	},
	{
		Name: "documentation-provider-boundaries", Package: "./internal/taskrail",
		TestRun: "^TestWorkflowContractCurrentGuidance", Platform: WorkflowSuitePortable,
	},
	{
		Name: "loop-launch-portable", Package: "./internal/taskrail",
		TestRun:  "^TestLoopLaunchChild(TransportsExactPromptAndIdentity|ResolvesSeparatorPathBeforeRepositoryCWD|ReportsExecutionFailures|DrainsBothStreamsConcurrently|DrainsSlowStreamsAfterLeaderExit|AppliesPerChildTimeout|DoesNotRelabelCallerDeadline)$",
		Platform: WorkflowSuitePortable,
	},
	{
		Name: "loop-containment-unix", Package: "./internal/taskrail",
		TestRun:  "^TestLoopLaunchChild(ContainsUnixProcessGroup|TerminatesDescendantsAfterLeaderExit|TerminatesOnContextCancellation|CleansUpAfterContainmentVerificationFailure|ReturnsAfterSurvivorEvidence|ForcesProcessGroupTermination)$",
		Platform: WorkflowSuiteUnix,
	},
	{
		Name: "loop-containment-windows", Package: "./internal/taskrail",
		TestRun: "^TestWindowsLoopContainment", Platform: WorkflowSuiteWindows,
	},
}

// WorkflowContractTestSurfaceManifest derives the current test-surface index.
// It deliberately does not claim provider execution, agent identity, semantic
// quality, or opaque real-agent provenance; those observations remain T-218's
// explicit evaluation boundary.
func WorkflowContractTestSurfaceManifest() (WorkflowContractManifest, error) {
	if err := validateWorkflowPromptRegistry(); err != nil {
		return WorkflowContractManifest{}, err
	}
	return deriveWorkflowContractManifest()
}

// ValidateWorkflowContractTestSurfaceManifest rejects a stale, incomplete, or
// platform-misclassified manifest before CI uses it to select focused suites.
func ValidateWorkflowContractTestSurfaceManifest(manifest WorkflowContractManifest) error {
	if err := validateWorkflowPromptRegistry(); err != nil {
		return err
	}
	want, err := deriveWorkflowContractManifest()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(manifest.PromptAssets, want.PromptAssets) {
		return fmt.Errorf("workflow contract prompt assets drift from the embedded registry")
	}
	if !reflect.DeepEqual(manifest.SkillResources, want.SkillResources) {
		return fmt.Errorf("workflow contract skill resources drift from the embedded package")
	}
	if !reflect.DeepEqual(manifest.Skills, want.Skills) {
		return fmt.Errorf("workflow contract skills drift from the shippable registry")
	}
	if !reflect.DeepEqual(manifest.MachineDocuments, want.MachineDocuments) {
		return fmt.Errorf("workflow contract machine documents drift from the machine registry")
	}
	if !reflect.DeepEqual(manifest.Suites, want.Suites) {
		for i := range manifest.Suites {
			if i >= len(want.Suites) || manifest.Suites[i] != want.Suites[i] {
				return fmt.Errorf("workflow contract suite %q or its platform classification drifts from the registry", manifest.Suites[i].Name)
			}
		}
		return fmt.Errorf("workflow contract suites or platform classifications drift from the registry")
	}
	if err := validateWorkflowContractSuites(manifest.Suites); err != nil {
		return err
	}
	return nil
}

func deriveWorkflowContractManifest() (WorkflowContractManifest, error) {
	promptAssets, err := embeddedPromptAssets()
	if err != nil {
		return WorkflowContractManifest{}, err
	}
	resources, err := packagedSkillFiles()
	if err != nil {
		return WorkflowContractManifest{}, err
	}
	documents := make([]WorkflowMachineDocument, 0, len(machineInventory))
	for _, entry := range MachineCommandInventory() {
		documents = append(documents, WorkflowMachineDocument{Command: entry.Command, Surface: entry.Surface})
	}
	return WorkflowContractManifest{
		PromptAssets:     promptAssets,
		SkillResources:   resources,
		Skills:           slices.Clone(shippableSkills),
		MachineDocuments: documents,
		Suites:           slices.Clone(workflowContractSuites),
	}, nil
}

func embeddedPromptAssets() ([]string, error) {
	assets, err := fs.Glob(builtinPrompts, "prompts/v1/*.md")
	if err != nil {
		return nil, fmt.Errorf("list embedded prompts: %w", err)
	}
	slices.Sort(assets)
	return assets, nil
}

func validateWorkflowPromptRegistry() error {
	assets, err := embeddedPromptAssets()
	if err != nil {
		return err
	}
	registeredAssets := make(map[string]struct{}, len(promptRegistry))
	registeredIDs := make(map[string]struct{}, len(promptRegistry))
	registeredKeys := make(map[string]struct{}, len(promptRegistry))
	for _, prompt := range promptRegistry {
		if prompt.id == "" || prompt.contract == "" || prompt.asset == "" {
			return fmt.Errorf("prompt registry has an incomplete definition")
		}
		if _, exists := registeredAssets[prompt.asset]; exists {
			return fmt.Errorf("prompt registry repeats embedded asset %q", prompt.asset)
		}
		key := promptKey(prompt.id, prompt.contract)
		if _, exists := registeredKeys[key]; exists {
			return fmt.Errorf("prompt registry repeats prompt identity key %q", key)
		}
		if _, exists := registeredIDs[prompt.id]; exists {
			return fmt.Errorf("prompt registry repeats prompt identity %q", prompt.id)
		}
		registeredAssets[prompt.asset] = struct{}{}
		registeredIDs[prompt.id] = struct{}{}
		registeredKeys[key] = struct{}{}
		tokens, ok := promptTokenDeclarations[prompt.id]
		if !ok {
			return fmt.Errorf("prompt registry %q has no token declaration", prompt.id)
		}
		data, err := builtinPrompts.ReadFile(prompt.asset)
		if err != nil {
			return fmt.Errorf("read registered embedded prompt %q: %w", prompt.asset, err)
		}
		if err := validatePromptTemplate(data); err != nil {
			return fmt.Errorf("validate registered embedded prompt %q: %w", prompt.asset, err)
		}
		declared := make(map[string]struct{}, len(tokens))
		for _, token := range tokens {
			if !validPromptTokenName(token) {
				return fmt.Errorf("prompt registry %q declares invalid token %q", prompt.id, token)
			}
			if _, exists := declared[token]; exists {
				return fmt.Errorf("prompt registry %q repeats token %q", prompt.id, token)
			}
			declared[token] = struct{}{}
		}
		if err := validatePromptTokenReferences(data, declared); err != nil {
			return fmt.Errorf("validate registered prompt tokens %q: %w", prompt.asset, err)
		}
	}
	if len(registeredAssets) != len(assets) {
		return fmt.Errorf("prompt registry has %d assets, embedded package has %d", len(registeredAssets), len(assets))
	}
	for _, asset := range assets {
		if _, ok := registeredAssets[asset]; !ok {
			return fmt.Errorf("embedded prompt %q is not registered", asset)
		}
	}
	for id := range promptTokenDeclarations {
		if _, err := promptDefinitionFor(id, ""); err != nil {
			return fmt.Errorf("prompt token declaration %q has no registered prompt", id)
		}
	}
	for id := range promptTransientRequirementsByID {
		if _, err := promptDefinitionFor(id, ""); err != nil {
			return fmt.Errorf("prompt transient requirement %q has no registered prompt", id)
		}
	}
	return nil
}

func cloneWorkflowContractManifest(manifest WorkflowContractManifest) WorkflowContractManifest {
	manifest.PromptAssets = slices.Clone(manifest.PromptAssets)
	manifest.SkillResources = slices.Clone(manifest.SkillResources)
	manifest.Skills = slices.Clone(manifest.Skills)
	manifest.MachineDocuments = slices.Clone(manifest.MachineDocuments)
	manifest.Suites = slices.Clone(manifest.Suites)
	return manifest
}

func validateWorkflowContractSuites(suites []WorkflowContractSuite) error {
	seen := make(map[string]struct{}, len(suites))
	for _, suite := range suites {
		if suite.Name == "" || suite.Package == "" || suite.TestRun == "" {
			return fmt.Errorf("workflow contract suite has an empty required field")
		}
		if _, exists := seen[suite.Name]; exists {
			return fmt.Errorf("workflow contract suite %q is duplicated", suite.Name)
		}
		seen[suite.Name] = struct{}{}
		if _, err := regexp.Compile(suite.TestRun); err != nil {
			return fmt.Errorf("workflow contract suite %q has invalid test pattern: %w", suite.Name, err)
		}
		switch suite.Platform {
		case WorkflowSuitePortable, WorkflowSuiteUnix, WorkflowSuiteWindows:
		default:
			return fmt.Errorf("workflow contract suite %q has unknown platform %q", suite.Name, suite.Platform)
		}
	}
	return nil
}
