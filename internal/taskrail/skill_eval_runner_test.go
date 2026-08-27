package taskrail

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestSkillEvalRunnerBuildsCompletePairedSafeReport(t *testing.T) {
	registry := []SkillEvalCase{
		{CaseID: "existing", Skill: "autonomous-task", StorageMode: "committed", BaselineRequired: true, FixtureSHA256: testSkillEvalDigest("a")},
		{CaseID: "new", Skill: "taskrail-loop", StorageMode: "local", FixtureSHA256: testSkillEvalDigest("b")},
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
		DeterministicChecks:       SkillEvalDeterministicChecks{Outcome: "pass", Evidence: []string{"go test ./..."}},
		HumanReview:               "A maintainer reviewed each paired outcome.",
		CaseReviews: map[string]SkillEvalCaseReview{
			"existing": {Comparison: "better", HumanReview: "Candidate handled the case safely."},
			"new":      {Comparison: "same", HumanReview: "New skill has no baseline arm."},
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
	if !slices.Equal(report.DeterministicChecks.Evidence, []string{"go test ./..."}) {
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
	in.Registry = append(in.Registry, SkillEvalCase{CaseID: "new", Skill: "taskrail-loop", StorageMode: "local", FixtureSHA256: testSkillEvalDigest("new-case")})
	in.CandidateSkillSHA256["taskrail-loop"] = testSkillEvalDigest("candidate-loop")
	in.CaseReviews["new"] = SkillEvalCaseReview{Comparison: "same", HumanReview: "New skill reviewed."}
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
		Registry:                  []SkillEvalCase{{CaseID: "existing", Skill: "autonomous-task", StorageMode: "committed", BaselineRequired: true, FixtureSHA256: testSkillEvalDigest("case")}},
		Adapter:                   adapter,
		AdapterIdentity:           SkillEvalIdentity{Name: "adapter", Version: "1", Observed: true},
		ModelIdentity:             SkillEvalIdentity{Name: "model", Version: "1", Observed: true},
		DeterministicChecks:       SkillEvalDeterministicChecks{Outcome: "pass", Evidence: []string{"go test"}},
		HumanReview:               "Maintainer review.",
		CaseReviews:               map[string]SkillEvalCaseReview{"existing": {Comparison: "same", HumanReview: "Equivalent behavior."}},
	}
}

type skillEvalTestAdapter struct{}

func (skillEvalTestAdapter) Run(_ context.Context, request SkillEvalAdapterRequest) (SkillEvalAdapterResult, error) {
	if err := os.WriteFile(filepath.Join(request.RawRoot, "result.txt"), []byte(request.Arm), 0o600); err != nil {
		return SkillEvalAdapterResult{}, err
	}
	return SkillEvalAdapterResult{Outcome: "pass", DeterministicGrade: "pass"}, nil
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
	return SkillEvalAdapterResult{Outcome: "pass", DeterministicGrade: "pass"}, nil
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
	return SkillEvalAdapterResult{Outcome: outcome, DeterministicGrade: map[string]string{"fail": "fail", "pass": "pass", "incomplete": "pass"}[outcome]}, nil
}

func testSkillEvalDigest(value string) string {
	return fmt.Sprintf("%064x", len(value))
}
