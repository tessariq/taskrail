package taskrail

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestSkillEvalRunnerBuildsCompletePairedSafeReport(t *testing.T) {
	registry := []SkillEvalCase{
		testSkillEvalCase("existing", "autonomous-task", "committed", true, "a"),
		testSkillEvalCase("new", "taskrail-loop", "local", false, "b"),
	}
	root := t.TempDir()
	report, err := (SkillEvalRunner{}).Run(context.Background(), SkillEvalRunInput{
		SessionID:                 "release-candidate",
		GeneratedAt:               time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC),
		TestedHead:                testSkillEvalDigest("head"),
		ProductSHA256:             testSkillEvalDigest("product"),
		CandidateSkillsSHA256:     testSkillEvalDigest("candidate-skills"),
		BaselineSkillsSHA256:      testSkillEvalDigest("baseline-skills"),
		CandidateSkillSHA256:      map[string]string{"autonomous-task": testSkillEvalDigest("candidate-task"), "taskrail-loop": testSkillEvalDigest("candidate-loop")},
		BaselineSkillSHA256:       map[string]string{"autonomous-task": testSkillEvalDigest("baseline-task")},
		FixturesSHA256:            testSkillEvalDigest("fixtures"),
		CandidateExecutableSHA256: testSkillEvalDigest("candidate-executable"),
		BaselineExecutableSHA256:  testSkillEvalDigest("baseline-executable"),
		ArtifactRoot:              root,
		Registry:                  registry,
		Adapter:                   skillEvalTestAdapter{},
		AdapterIdentity:           SkillEvalIdentity{Name: "adapter", Version: "1", Observed: true},
		ModelIdentity:             SkillEvalIdentity{Name: "model", Version: "1", Observed: true},
		DeterministicChecks:       SkillEvalDeterministicChecks{Outcome: "pass", Evidence: []string{"go-test-all"}},
		HumanReview:               "A maintainer reviewed each paired outcome.",
		CaseReviews: map[string]SkillEvalCaseReview{
			"existing": {Comparison: "better", HumanReview: "Candidate handled the case safely."},
			"new":      {Comparison: "candidate-only", HumanReview: "New skill has no baseline arm."},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Outcome != "pass" || len(report.Cases) != 2 {
		t.Fatalf("report = %#v", report)
	}
	if report.Cases[0].CaseID != "existing" || report.Cases[0].Baseline == nil || report.Cases[1].Baseline != nil {
		t.Fatalf("case arms = %#v", report.Cases)
	}
	if report.Cases[0].Candidate.SkillSHA256 != testSkillEvalDigest("candidate-task") || report.Cases[0].Baseline.SkillSHA256 != testSkillEvalDigest("baseline-task") {
		t.Fatalf("per-skill digest bindings = %#v", report.Cases[0])
	}
	if !slices.Equal(report.DeterministicChecks.Evidence, []string{"go-test-all"}) {
		t.Fatalf("evidence = %#v", report.DeterministicChecks.Evidence)
	}
	encoded, err := RenderSkillEvalReport(report)
	if err != nil {
		t.Fatalf("RenderSkillEvalReport: %v", err)
	}
	if len(encoded) == 0 || encoded[len(encoded)-1] != '\n' {
		t.Fatalf("report is not newline terminated: %q", encoded)
	}
	if strings.Contains(string(encoded), root) || strings.Contains(string(encoded), "raw/result.txt") {
		t.Fatalf("safe report leaked a raw evidence path: %s", encoded)
	}
}

func TestSkillEvalRunnerMarksMissingCandidateIncomplete(t *testing.T) {
	report, err := (SkillEvalRunner{}).Run(context.Background(), skillEvalTestInput(t, skillEvalMissingAdapter{}))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Outcome != "incomplete" || report.Cases[0].Candidate != nil || report.Cases[0].Comparison != "inconclusive" {
		t.Fatalf("report = %#v", report)
	}
	if !report.Adapter.Observed || !report.Model.Observed {
		t.Fatalf("one completed baseline arm must mark identities observed: %#v %#v", report.Adapter, report.Model)
	}
}

func TestSkillEvalRunnerRejectsDirectRegistryMutations(t *testing.T) {
	for _, mutate := range []func(*SkillEvalCase){
		func(item *SkillEvalCase) { item.Assertions = []string{"assertion", "assertion"} },
		func(item *SkillEvalCase) { item.Scenario.Actions = nil },
		func(item *SkillEvalCase) { item.Oracle.Assertions[0].Predicate = "unknown" },
		func(item *SkillEvalCase) {
			item.Scenario.Actions[0].ID, item.Oracle.Assertions[0].Action = "positive", "positive"
		},
	} {
		in := skillEvalTestInput(t, skillEvalTestAdapter{})
		mutate(&in.Registry[0])
		if _, err := (SkillEvalRunner{}).Execute(context.Background(), in); err == nil {
			t.Fatal("Execute accepted an ambiguous direct registry case")
		}
	}
}

func TestSkillEvalRunnerDerivesGradeFromMechanicalFacts(t *testing.T) {
	in := skillEvalTestInput(t, skillEvalMissingFactsAdapter{})
	report, err := (SkillEvalRunner{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Cases[0].Candidate.DeterministicGrade != "fail" || report.Outcome != "fail" {
		t.Fatalf("oracle report = %#v", report)
	}
}

func TestSkillEvalRunnerRejectsAssertionEchoesAndFabricatedFacts(t *testing.T) {
	in := skillEvalTestInput(t, skillEvalAssertionEchoAdapter{})
	stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
	if err != nil || stage.Report.Cases[0].Candidate.DeterministicGrade != "fail" {
		t.Fatalf("assertion echo did not produce a mechanical failure: stage=%#v err=%v", stage, err)
	}
	in = skillEvalTestInput(t, skillEvalFabricatedFactsAdapter{})
	if _, err := (SkillEvalRunner{}).Execute(context.Background(), in); err == nil {
		t.Fatal("Execute accepted fabricated facts that differ from the raw receipt")
	}
}

func TestSkillEvalRunnerUsesCandidateOnlyComparisonForNewSkills(t *testing.T) {
	in := skillEvalTestInput(t, skillEvalTestAdapter{})
	in.Registry[0].BaselineRequired = false
	in.BaselineSkillSHA256 = nil
	in.CaseReviews["existing"] = SkillEvalCaseReview{Comparison: "candidate-only", HumanReview: "Candidate behavior met every deterministic oracle."}
	report, err := (SkillEvalRunner{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Outcome != "pass" || report.Cases[0].Baseline != nil || report.Cases[0].Comparison != "candidate-only" {
		t.Fatalf("candidate-only report = %#v", report)
	}
}

func TestSkillEvalRunnerExecutesCompleteRegistryWithWorkingBinary(t *testing.T) {
	registry, err := loadSkillEvalRegistry(filepath.Join("testdata", "skill-evals", "v1", "cases"), shippableSkills)
	if err != nil {
		t.Fatalf("loadSkillEvalRegistry: %v", err)
	}
	in := skillEvalCompleteRegistryInput(t, registry, skillEvalScenarioAdapter{})
	in.ArtifactRoot, err = os.MkdirTemp("", "se")
	if err != nil {
		t.Fatalf("create short artifact root: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(in.ArtifactRoot) })
	if !skillEvalDirectoryDurabilitySupported(t, in.ArtifactRoot) {
		registry = slices.DeleteFunc(registry, func(item SkillEvalCase) bool {
			return item.StorageMode == "local"
		})
		in.Registry = registry
		for caseID := range in.CaseReviews {
			if !slices.ContainsFunc(registry, func(item SkillEvalCase) bool { return item.CaseID == caseID }) {
				delete(in.CaseReviews, caseID)
			}
		}
		t.Log("local-storage cases omitted: host does not support durable directory sync")
	}
	stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	wantArms := 0
	for _, item := range registry {
		wantArms++
		if item.BaselineRequired {
			wantArms++
		}
	}
	if got := len(stage.Report.Cases); got != len(registry) {
		t.Fatalf("staged cases = %d, want %d", got, len(registry))
	}
	if got := skillEvalStagedArmCount(stage); got != wantArms {
		t.Fatalf("staged arms = %d, want %d", got, wantArms)
	}
	report, err := (SkillEvalRunner{}).Resume(stage, in, "Maintainer reviewed all frozen working-binary outcomes.", in.CaseReviews)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if report.Outcome != "pass" {
		failed := 0
		for _, item := range report.Cases {
			if item.Candidate == nil || item.Candidate.Outcome != "pass" || item.Candidate.DeterministicGrade != "pass" {
				failed++
				evaluation := registry[slices.IndexFunc(registry, func(candidate SkillEvalCase) bool {
					return candidate.CaseID == item.CaseID
				})]
				facts, factsErr := decodeSkillEvalFacts(skillEvalRawRoot(in, evaluation, skillEvalCandidateArm))
				rawRoot := skillEvalRawRoot(in, evaluation, skillEvalCandidateArm)
				stdout, _ := os.ReadFile(filepath.Join(rawRoot, "initialize-taskrail-repository.stdout"))
				stderr, _ := os.ReadFile(filepath.Join(rawRoot, "initialize-taskrail-repository.stderr"))
				t.Logf("failed case %s candidate=%+v facts_err=%v", item.CaseID, item.Candidate, factsErr)
				for _, fact := range facts {
					t.Logf("failed case %s action=%s exit=%d", item.CaseID, fact.Action, fact.ExitCode)
				}
				t.Logf("failed case %s init_stdout=%q init_stderr=%q", item.CaseID, stdout, stderr)
			}
		}
		t.Fatalf("complete registry outcome = %q; failed cases = %d", report.Outcome, failed)
	}
}

func skillEvalDirectoryDurabilitySupported(t *testing.T, root string) bool {
	t.Helper()
	directory, err := os.Open(root)
	if err != nil {
		t.Fatalf("open directory durability probe: %v", err)
	}
	syncErr := directory.Sync()
	if err := directory.Close(); err != nil {
		t.Fatalf("close directory durability probe: %v", err)
	}
	return syncErr == nil
}

func TestRenderSkillEvalReportAcceptsExactIncompleteWaiver(t *testing.T) {
	report := skillEvalWaivedReport(t)
	encoded, err := RenderSkillEvalReport(report)
	if err != nil {
		t.Fatalf("RenderSkillEvalReport: %v", err)
	}
	decoded, err := DecodeSkillEvalReport(encoded, report)
	if err != nil {
		t.Fatalf("DecodeSkillEvalReport: %v", err)
	}
	if decoded.Outcome != "waived" || decoded.Waiver == nil || decoded.Waiver.Approver != "release-maintainer" {
		t.Fatalf("decoded waiver = %#v", decoded)
	}
}

func TestRenderSkillEvalReportRejectsInvalidWaiver(t *testing.T) {
	for _, mutate := range []func(*SkillEvalReport){
		func(report *SkillEvalReport) { report.Outcome = "incomplete" },
		func(report *SkillEvalReport) { report.DeterministicChecks.Outcome = "fail" },
		func(report *SkillEvalReport) { report.Waiver.Approver = "" },
		func(report *SkillEvalReport) { report.Waiver.Reason = "" },
		func(report *SkillEvalReport) { report.Waiver.UnavailableCapability = "" },
		func(report *SkillEvalReport) { report.Waiver.ResidualRisk = "" },
		func(report *SkillEvalReport) { report.Waiver.Followup = "" },
		func(report *SkillEvalReport) { report.Waiver.Reason = "/tmp/provider-transcript" },
		func(report *SkillEvalReport) { report.Waiver.AffectedCases = []string{"other"} },
		func(report *SkillEvalReport) { report.Waiver.AffectedSkills = []string{"other-skill"} },
		func(report *SkillEvalReport) { report.Waiver.CompensatingEvidence = []string{"z", "a"} },
	} {
		report := skillEvalWaivedReport(t)
		mutate(&report)
		if _, err := RenderSkillEvalReport(report); err == nil {
			t.Fatal("RenderSkillEvalReport accepted an invalid waiver")
		}
	}
}

func TestRenderSkillEvalReportRequiresOutcomeWaiverUnion(t *testing.T) {
	pass, err := (SkillEvalRunner{}).Run(context.Background(), skillEvalTestInput(t, skillEvalTestAdapter{}))
	if err != nil {
		t.Fatalf("Run pass report: %v", err)
	}
	pass.Waiver = skillEvalWaivedReport(t).Waiver
	if _, err := RenderSkillEvalReport(pass); err == nil {
		t.Fatal("RenderSkillEvalReport accepted a waiver on passing evidence")
	}

	incomplete, err := (SkillEvalRunner{}).Run(context.Background(), skillEvalTestInput(t, skillEvalMissingAdapter{}))
	if err != nil {
		t.Fatalf("Run incomplete report: %v", err)
	}
	incomplete.Outcome = "waived"
	if _, err := RenderSkillEvalReport(incomplete); err == nil {
		t.Fatal("RenderSkillEvalReport accepted waived with a null waiver")
	}

	failure, err := (SkillEvalRunner{}).Run(context.Background(), skillEvalTestInput(t, skillEvalResultAdapter{candidate: "fail", baseline: "pass"}))
	if err != nil {
		t.Fatalf("Run failed report: %v", err)
	}
	failure.Outcome = "waived"
	failure.Waiver = skillEvalWaivedReport(t).Waiver
	if _, err := RenderSkillEvalReport(failure); err == nil {
		t.Fatal("RenderSkillEvalReport accepted a waiver on failed evidence")
	}
}

func TestDecodeSkillEvalReportRejectsNoncanonicalWaiver(t *testing.T) {
	report := skillEvalWaivedReport(t)
	encoded, err := RenderSkillEvalReport(report)
	if err != nil {
		t.Fatalf("RenderSkillEvalReport: %v", err)
	}
	for _, mutate := range []func(string) string{
		func(data string) string {
			return strings.Replace(data, "\"schema_version\": 1,\n  \"session_id\"", "\"session_id\": \"session\",\n  \"schema_version\": 1", 1)
		},
		func(data string) string {
			return strings.Replace(data, "\"followup\": \"v0.5.0 release checklist\"", "\"unknown\": \"field\",\n    \"followup\": \"v0.5.0 release checklist\"", 1)
		},
		func(data string) string {
			return strings.Replace(data, "\"affected_skills\": [\n      \"autonomous-task\"\n    ]", "\"affected_skills\": [\n      \"other-skill\"\n    ]", 1)
		},
	} {
		if _, err := DecodeSkillEvalReport([]byte(mutate(string(encoded))), report); err == nil {
			t.Fatal("DecodeSkillEvalReport accepted a malformed waiver")
		}
	}
}

func TestDecodeSkillEvalReportBindsRunnerManifest(t *testing.T) {
	report := skillEvalWaivedReport(t)
	encoded, err := RenderSkillEvalReport(report)
	if err != nil {
		t.Fatalf("RenderSkillEvalReport: %v", err)
	}
	expected := report
	expected.manifest.Cases["registered-but-missing"] = skillEvalExpectedCase{}
	if _, err := DecodeSkillEvalReport(encoded, expected); err == nil {
		t.Fatal("DecodeSkillEvalReport accepted a report missing a registered case")
	}
}

func TestRenderSkillEvalReportRequiresEveryWaiverGate(t *testing.T) {
	for _, omitted := range skillEvalRequiredWaiverChecks {
		report := skillEvalWaivedReport(t)
		report.DeterministicChecks.Evidence = slices.DeleteFunc(report.DeterministicChecks.Evidence, func(value string) bool {
			return value == omitted
		})
		if _, err := RenderSkillEvalReport(report); err == nil {
			t.Fatalf("RenderSkillEvalReport accepted a waiver missing %q", omitted)
		}
	}
}

func TestSkillEvalRunnerRefusesUnsafeRawTree(t *testing.T) {
	in := skillEvalTestInput(t, skillEvalUnsafeRawAdapter{})
	if _, err := (SkillEvalRunner{}).Run(context.Background(), in); err == nil {
		t.Fatal("Run succeeded with a symlinked raw artifact")
	}
}

func TestSkillEvalRunnerOutcomePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name    string
		adapter SkillEvalAdapter
		checks  string
		want    string
	}{
		{"candidate failure", skillEvalResultAdapter{candidate: "fail", baseline: "pass"}, "pass", "fail"},
		{"missing comparison", skillEvalResultAdapter{candidate: "incomplete", baseline: "pass"}, "pass", "incomplete"},
		{"deterministic failure", skillEvalTestAdapter{}, "fail", "fail"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := skillEvalTestInput(t, tc.adapter)
			in.DeterministicChecks.Outcome = tc.checks
			report, err := (SkillEvalRunner{}).Run(context.Background(), in)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if report.Outcome != tc.want {
				t.Fatalf("outcome = %q, want %q", report.Outcome, tc.want)
			}
		})
	}
}

func TestRenderSkillEvalReportRefusesFalseFavorableOutcome(t *testing.T) {
	report, err := (SkillEvalRunner{}).Run(context.Background(), skillEvalTestInput(t, skillEvalResultAdapter{candidate: "fail", baseline: "pass"}))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	report.Outcome = "pass"
	if _, err := RenderSkillEvalReport(report); err == nil {
		t.Fatal("RenderSkillEvalReport accepted a false favorable outcome")
	}
}

func TestSkillEvalRunnerRefusesRawPathTraversal(t *testing.T) {
	for _, mutate := range []func(*SkillEvalCase, string){
		func(item *SkillEvalCase, unsafe string) { item.Skill = unsafe },
		func(item *SkillEvalCase, unsafe string) { item.CaseID = unsafe },
	} {
		in := skillEvalTestInput(t, skillEvalTestAdapter{})
		escapeRoot := t.TempDir()
		unsafe := filepath.ToSlash(filepath.Join("..", "..", "..", "..", "..", filepath.Base(escapeRoot)))
		mutate(&in.Registry[0], unsafe)
		in.CandidateSkillSHA256[in.Registry[0].Skill] = testSkillEvalDigest("unsafe-candidate")
		in.BaselineSkillSHA256[in.Registry[0].Skill] = testSkillEvalDigest("unsafe-baseline")
		in.CaseReviews[in.Registry[0].CaseID] = SkillEvalCaseReview{Comparison: "same", HumanReview: "Equivalent behavior."}
		if _, err := (SkillEvalRunner{}).Run(context.Background(), in); err == nil {
			t.Fatal("Run accepted a raw evidence traversal path")
		}
	}
}

func TestRenderSkillEvalReportRejectsSchemaMutations(t *testing.T) {
	for _, mutate := range []func(*SkillEvalReport){
		func(report *SkillEvalReport) { report.Cases[0].Baseline.DeterministicGrade = "" },
		func(report *SkillEvalReport) { report.Cases = append(report.Cases, report.Cases[0]) },
		func(report *SkillEvalReport) { report.HumanReview = "/tmp/provider-transcript" },
	} {
		report, err := (SkillEvalRunner{}).Run(context.Background(), skillEvalTestInput(t, skillEvalTestAdapter{}))
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		mutate(&report)
		if _, err := RenderSkillEvalReport(report); err == nil {
			t.Fatal("RenderSkillEvalReport accepted an invalid schema mutation")
		}
	}
}

func TestRenderSkillEvalReportRejectsBindingAndPortabilityMutations(t *testing.T) {
	for _, mutate := range []func(*SkillEvalReport){
		func(report *SkillEvalReport) { report.Cases[0].Candidate.SkillSHA256 = testSkillEvalDigest("forged") },
		func(report *SkillEvalReport) { report.Adapter.Observed = false },
		func(report *SkillEvalReport) { report.HumanReview = "[transcript](/tmp/provider-output)" },
		func(report *SkillEvalReport) { report.Cases[0].CaseID = strings.Repeat("a", 65) },
	} {
		report, err := (SkillEvalRunner{}).Run(context.Background(), skillEvalTestInput(t, skillEvalTestAdapter{}))
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		mutate(&report)
		if _, err := RenderSkillEvalReport(report); err == nil {
			t.Fatal("RenderSkillEvalReport accepted a binding or portability mutation")
		}
	}
}

func TestRenderSkillEvalReportRequiresCompleteRunnerRegistry(t *testing.T) {
	in := skillEvalTestInput(t, skillEvalTestAdapter{})
	in.Registry = append(in.Registry, testSkillEvalCase("new", "taskrail-loop", "local", false, "new-case"))
	in.CandidateSkillSHA256["taskrail-loop"] = testSkillEvalDigest("candidate-loop")
	in.CaseReviews["new"] = SkillEvalCaseReview{Comparison: "candidate-only", HumanReview: "New skill reviewed."}
	report, err := (SkillEvalRunner{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	report.Cases = report.Cases[1:]
	if _, err := RenderSkillEvalReport(report); err == nil {
		t.Fatal("RenderSkillEvalReport accepted a report missing a registered case")
	}
}

func skillEvalTestInput(t *testing.T, adapter SkillEvalAdapter) SkillEvalRunInput {
	t.Helper()
	return SkillEvalRunInput{
		SessionID:                 "session",
		GeneratedAt:               time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC),
		TestedHead:                testSkillEvalDigest("head"),
		ProductSHA256:             testSkillEvalDigest("product"),
		CandidateSkillsSHA256:     testSkillEvalDigest("candidate"),
		BaselineSkillsSHA256:      testSkillEvalDigest("baseline"),
		CandidateSkillSHA256:      map[string]string{"autonomous-task": testSkillEvalDigest("candidate-task")},
		BaselineSkillSHA256:       map[string]string{"autonomous-task": testSkillEvalDigest("baseline-task")},
		FixturesSHA256:            testSkillEvalDigest("fixtures"),
		CandidateExecutableSHA256: testSkillEvalDigest("candidate-exe"),
		BaselineExecutableSHA256:  testSkillEvalDigest("baseline-exe"),
		ArtifactRoot:              t.TempDir(),
		Registry:                  []SkillEvalCase{testSkillEvalCase("existing", "autonomous-task", "committed", true, "case")},
		Adapter:                   adapter,
		AdapterIdentity:           SkillEvalIdentity{Name: "adapter", Version: "1", Observed: true},
		ModelIdentity:             SkillEvalIdentity{Name: "model", Version: "1", Observed: true},
		DeterministicChecks:       SkillEvalDeterministicChecks{Outcome: "pass", Evidence: []string{"go test"}},
		HumanReview:               "Maintainer review.",
		CaseReviews:               map[string]SkillEvalCaseReview{"existing": {Comparison: "same", HumanReview: "Equivalent behavior."}},
	}
}

func skillEvalWaivedReport(t *testing.T) SkillEvalReport {
	t.Helper()
	report, err := (SkillEvalRunner{}).Run(context.Background(), skillEvalTestInput(t, skillEvalMissingAdapter{}))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	report.Outcome = "waived"
	report.DeterministicChecks.Evidence = slices.Clone(skillEvalRequiredWaiverChecks)
	report.Waiver = &SkillEvalWaiver{
		Approver:              "release-maintainer",
		Reason:                "Provider credentials are unavailable for the final run.",
		UnavailableCapability: "provider execution",
		AffectedSkills:        []string{"autonomous-task"},
		AffectedCases:         []string{"existing"},
		ResidualRisk:          "Behavioral evidence is incomplete.",
		CompensatingEvidence:  []string{"credential-free checks pass"},
		Followup:              "v0.5.0 release checklist",
	}
	return report
}

type skillEvalTestAdapter struct{}

func (skillEvalTestAdapter) Run(_ context.Context, request SkillEvalAdapterRequest) (SkillEvalAdapterResult, error) {
	if err := os.WriteFile(filepath.Join(request.RawRoot, "result.txt"), []byte(request.Arm), 0o600); err != nil {
		return SkillEvalAdapterResult{}, err
	}
	return skillEvalSuccessfulResult(request)
}

type skillEvalMissingAdapter struct{}

func (skillEvalMissingAdapter) Run(_ context.Context, request SkillEvalAdapterRequest) (SkillEvalAdapterResult, error) {
	if request.Arm == "candidate" {
		return SkillEvalAdapterResult{}, errSkillEvalArmUnavailable
	}
	return skillEvalTestAdapter{}.Run(context.Background(), request)
}

type skillEvalUnsafeRawAdapter struct{}

func (skillEvalUnsafeRawAdapter) Run(_ context.Context, request SkillEvalAdapterRequest) (SkillEvalAdapterResult, error) {
	if err := os.Symlink("outside", filepath.Join(request.RawRoot, "link")); err != nil {
		return SkillEvalAdapterResult{}, err
	}
	return skillEvalSuccessfulResult(request)
}

type skillEvalResultAdapter struct {
	candidate string
	baseline  string
}

func (adapter skillEvalResultAdapter) Run(_ context.Context, request SkillEvalAdapterRequest) (SkillEvalAdapterResult, error) {
	if err := os.WriteFile(filepath.Join(request.RawRoot, "result.txt"), []byte(request.Arm), 0o600); err != nil {
		return SkillEvalAdapterResult{}, err
	}
	outcome := adapter.candidate
	if request.Arm == "baseline" {
		outcome = adapter.baseline
	}
	result, err := skillEvalSuccessfulResult(request)
	result.Outcome = outcome
	return result, err
}

func testSkillEvalCase(caseID, skill, mode string, baseline bool, digest string) SkillEvalCase {
	return SkillEvalCase{
		CaseID: caseID, Skill: skill, StorageMode: mode, BaselineRequired: baseline,
		Prompt: "Run the scenario.", ExpectedObservation: "The assertions are observed.",
		Assertions: []string{"assertion"}, HumanReviewQuestions: []string{"Was the behavior safe?"},
		Scenario: SkillEvalScenario{Fixture: "fixture", Sandbox: caseID, Setup: []SkillEvalScenarioAction{{ID: "initialize-git", Operation: "git-command", Command: []string{"git", "init"}}}, Actions: []SkillEvalScenarioAction{{ID: "run-assertion", Operation: "taskrail-command", Command: []string{"taskrail", "validate", "--json"}}}},
		Oracle:   SkillEvalOracle{Assertions: []SkillEvalAssertionOracle{{Assertion: "assertion", Action: "run-assertion", Predicate: "command-exit-zero"}}}, FixtureSHA256: testSkillEvalDigest(digest),
	}
}

func skillEvalCompleteRegistryInput(t *testing.T, registry []SkillEvalCase, adapter SkillEvalAdapter) SkillEvalRunInput {
	t.Helper()
	in := skillEvalTestInput(t, adapter)
	in.Registry = registry
	in.CandidateSkillSHA256 = map[string]string{}
	in.BaselineSkillSHA256 = map[string]string{}
	in.CaseReviews = map[string]SkillEvalCaseReview{}
	for _, item := range registry {
		in.CandidateSkillSHA256[item.Skill] = testSkillEvalDigest("candidate-" + item.Skill)
		comparison := "candidate-only"
		if item.BaselineRequired {
			in.BaselineSkillSHA256[item.Skill] = testSkillEvalDigest("baseline-" + item.Skill)
			comparison = "same"
		}
		in.CaseReviews[item.CaseID] = SkillEvalCaseReview{Comparison: comparison, HumanReview: "Fake adapter outcome reviewed."}
	}
	return in
}

func skillEvalStagedArmCount(stage SkillEvalStage) int {
	count := 0
	for _, item := range stage.Report.Cases {
		if item.Candidate != nil {
			count++
		}
		if item.Baseline != nil {
			count++
		}
	}
	return count
}

type skillEvalScenarioAdapter struct{}

func (skillEvalScenarioAdapter) Run(ctx context.Context, request SkillEvalAdapterRequest) (SkillEvalAdapterResult, error) {
	if request.FixtureRoot == "" {
		return SkillEvalAdapterResult{}, fmt.Errorf("missing case fixture root")
	}
	seed, err := os.ReadFile(filepath.Join(request.FixtureRoot, "seed.json"))
	if err != nil {
		return SkillEvalAdapterResult{}, err
	}
	var config skillEvalSeed
	if err := json.Unmarshal(seed, &config); err != nil {
		return SkillEvalAdapterResult{}, err
	}
	sandbox := filepath.Join(request.RawRoot, "sandbox")
	if err := os.MkdirAll(sandbox, 0o700); err != nil {
		return SkillEvalAdapterResult{}, err
	}
	if err := os.WriteFile(filepath.Join(request.RawRoot, "agent-transcript.txt"), []byte("stubbed agent transcript\n"+request.Case.Prompt+"\n"), 0o600); err != nil {
		return SkillEvalAdapterResult{}, err
	}
	binary, err := skillEvalWorkingBinary(ctx, request.RawRoot)
	if err != nil {
		return SkillEvalAdapterResult{}, err
	}
	facts := make([]SkillEvalObservedFact, 0, len(request.Case.Scenario.Setup)+len(request.Case.Scenario.Actions))
	for _, action := range append(slices.Clone(request.Case.Scenario.Setup), request.Case.Scenario.Actions...) {
		fact, err := runSkillEvalScenarioCommand(ctx, sandbox, binary, request.RawRoot, action)
		if err != nil {
			return SkillEvalAdapterResult{}, err
		}
		facts = append(facts, fact)
	}
	if err := writeSkillEvalFacts(request.RawRoot, facts); err != nil {
		return SkillEvalAdapterResult{}, err
	}
	return SkillEvalAdapterResult{Outcome: "pass", Facts: facts}, nil
}

func skillEvalWorkingBinary(ctx context.Context, rawRoot string) (string, error) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return "", err
	}
	binary := filepath.Join(rawRoot, "taskrail")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	command := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/taskrail")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build working Taskrail binary: %w: %s", err, output)
	}
	return binary, nil
}

func runSkillEvalScenarioCommand(ctx context.Context, sandbox, binary, rawRoot string, action SkillEvalScenarioAction) (SkillEvalObservedFact, error) {
	before, err := skillEvalTreeDigest("taskrail-skill-eval-sandbox-v1", sandbox)
	if err != nil {
		return SkillEvalObservedFact{}, err
	}
	gitBefore := skillEvalGitDigest(ctx, sandbox)
	program, args := action.Command[0], action.Command[1:]
	if action.Operation == "taskrail-command" {
		program = binary
	}
	command := exec.CommandContext(ctx, program, args...)
	command.Dir = sandbox
	stdout, err := command.Output()
	stderr := []byte{}
	exitCode := 0
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			exitCode = exit.ExitCode()
			stderr = exit.Stderr
		} else {
			return SkillEvalObservedFact{}, err
		}
	}
	if err := os.WriteFile(filepath.Join(rawRoot, action.ID+".stdout"), stdout, 0o600); err != nil {
		return SkillEvalObservedFact{}, err
	}
	if err := os.WriteFile(filepath.Join(rawRoot, action.ID+".stderr"), stderr, 0o600); err != nil {
		return SkillEvalObservedFact{}, err
	}
	after, err := skillEvalTreeDigest("taskrail-skill-eval-sandbox-v1", sandbox)
	if err != nil {
		return SkillEvalObservedFact{}, err
	}
	fact := SkillEvalObservedFact{Action: action.ID, Operation: action.Operation, Command: action.Command, ExitCode: exitCode, StdoutSHA256: skillEvalBytesDigest(stdout), StderrSHA256: skillEvalBytesDigest(stderr), BeforeSHA256: before, AfterSHA256: after, GitBeforeSHA256: gitBefore, GitAfterSHA256: skillEvalGitDigest(ctx, sandbox), StoragePaths: []string{}}
	if action.Operation == "taskrail-command" && len(args) > 0 && args[0] == "validate" {
		var envelope struct {
			Result struct {
				Valid bool `json:"valid"`
			} `json:"result"`
		}
		fact.ValidationPassed = json.Unmarshal(stdout, &envelope) == nil && envelope.Result.Valid
	}
	return fact, nil
}

func skillEvalGitDigest(ctx context.Context, sandbox string) string {
	command := exec.CommandContext(ctx, "git", "status", "--porcelain=v1")
	command.Dir = sandbox
	output, _ := command.Output()
	return skillEvalBytesDigest(output)
}

type skillEvalMissingFactsAdapter struct{}

func (skillEvalMissingFactsAdapter) Run(_ context.Context, request SkillEvalAdapterRequest) (SkillEvalAdapterResult, error) {
	if err := os.WriteFile(filepath.Join(request.RawRoot, "result.txt"), []byte(request.Arm), 0o600); err != nil {
		return SkillEvalAdapterResult{}, err
	}
	facts := []SkillEvalObservedFact{{Action: request.Case.Scenario.Actions[0].ID, Command: request.Case.Scenario.Actions[0].Command, ExitCode: 1}}
	if err := writeSkillEvalFacts(request.RawRoot, facts); err != nil {
		return SkillEvalAdapterResult{}, err
	}
	return SkillEvalAdapterResult{Outcome: "pass", Facts: facts}, nil
}

func skillEvalSuccessfulResult(request SkillEvalAdapterRequest) (SkillEvalAdapterResult, error) {
	facts := make([]SkillEvalObservedFact, 0, len(request.Case.Scenario.Actions))
	for _, action := range append(slices.Clone(request.Case.Scenario.Setup), request.Case.Scenario.Actions...) {
		facts = append(facts, SkillEvalObservedFact{Action: action.ID, Operation: action.Operation, Command: action.Command, ExitCode: 0})
	}
	if err := writeSkillEvalFacts(request.RawRoot, facts); err != nil {
		return SkillEvalAdapterResult{}, err
	}
	return SkillEvalAdapterResult{Outcome: "pass", Facts: facts}, nil
}

type skillEvalAssertionEchoAdapter struct{}

func (skillEvalAssertionEchoAdapter) Run(_ context.Context, request SkillEvalAdapterRequest) (SkillEvalAdapterResult, error) {
	if err := os.WriteFile(filepath.Join(request.RawRoot, "result.txt"), []byte(request.Arm), 0o600); err != nil {
		return SkillEvalAdapterResult{}, err
	}
	facts := []SkillEvalObservedFact{{Action: request.Case.Assertions[0], Command: request.Case.Scenario.Actions[0].Command, ExitCode: 0}}
	if err := writeSkillEvalFacts(request.RawRoot, facts); err != nil {
		return SkillEvalAdapterResult{}, err
	}
	return SkillEvalAdapterResult{Outcome: "pass", Facts: facts}, nil
}

type skillEvalFabricatedFactsAdapter struct{}

func (skillEvalFabricatedFactsAdapter) Run(_ context.Context, request SkillEvalAdapterRequest) (SkillEvalAdapterResult, error) {
	if err := os.WriteFile(filepath.Join(request.RawRoot, "result.txt"), []byte(request.Arm), 0o600); err != nil {
		return SkillEvalAdapterResult{}, err
	}
	recorded, err := skillEvalSuccessfulResult(request)
	if err != nil {
		return SkillEvalAdapterResult{}, err
	}
	recorded.Facts[0].ExitCode = 1
	return recorded, nil
}

func testSkillEvalDigest(value string) string {
	return fmt.Sprintf("%064x", len(value))
}
