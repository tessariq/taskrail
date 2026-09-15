package taskrail

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type skillEvalPublishFixture struct {
	root                string
	stagePath           string
	answersPath         string
	registryRoot        string
	artifactRoot        string
	planningDir         string
	candidateExecutable string
	baselineExecutable  string
	input               SkillEvalRunInput
	bindings            SkillEvalPublishBindings
}

type publishAnswerFile struct {
	SchemaVersion int                 `json:"schema_version"`
	HumanReview   string              `json:"human_review"`
	Cases         []publishAnswerCase `json:"cases"`
}

type publishAnswerCase struct {
	CaseID      string `json:"case_id"`
	Comparison  string `json:"comparison"`
	HumanReview string `json:"human_review"`
}

// defaultPublishAnswers answers every staged case the way its staged evidence
// permits: a paired case is equivalent, a no-baseline case is candidate-only.
// mutate then breaks exactly one thing for a defect variant.
func defaultPublishAnswers(t *testing.T, registry []SkillEvalCase, mutate func(*publishAnswerFile)) string {
	t.Helper()
	file := publishAnswerFile{SchemaVersion: 1, HumanReview: "Maintainer compared every frozen arm pair across sittings."}
	for _, item := range registry {
		answer := publishAnswerCase{CaseID: item.CaseID, Comparison: "candidate-only", HumanReview: "No v0.4.0 baseline exists for local storage."}
		if item.BaselineRequired {
			answer.Comparison, answer.HumanReview = "same", "Paired behavior is equivalent."
		}
		file.Cases = append(file.Cases, answer)
	}
	if mutate != nil {
		mutate(&file)
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		t.Fatalf("encode answers: %v", err)
	}
	return string(data) + "\n"
}

func publishAnswerFor(file *publishAnswerFile, caseID string) *publishAnswerCase {
	for index := range file.Cases {
		if file.Cases[index].CaseID == caseID {
			return &file.Cases[index]
		}
	}
	return nil
}

// stageSkillEvalPublishFixture stages a complete shipped-skill registry once,
// then seals it as producer-local stage.json plus registry-driven answers.
func stageSkillEvalPublishFixture(t *testing.T, adapter SkillEvalAdapter, answersMutate func(*publishAnswerFile)) *skillEvalPublishFixture {
	t.Helper()
	root := t.TempDir()
	registryRoot := filepath.Join(root, "registry")
	for _, skill := range shippableSkills {
		writeSkillEvalFixture(t, registryRoot, skill, "committed", skill+"-committed", skillEvalBaselineSkills[skill], true)
		writeSkillEvalFixture(t, registryRoot, skill, "local", skill+"-local", skillEvalBaselineSkills[skill], true)
	}
	registry, err := loadSkillEvalRegistry(registryRoot, shippableSkills)
	if err != nil {
		t.Fatalf("loadSkillEvalRegistry: %v", err)
	}
	fixturesSHA256, err := skillEvalTreeDigest("taskrail-skill-eval-fixtures-v1", registryRoot)
	if err != nil {
		t.Fatalf("derive fixtures digest: %v", err)
	}
	candidateSkills, baselineSkills := map[string]string{}, map[string]string{}
	for _, item := range registry {
		candidateSkills[item.Skill] = testSkillEvalDigest("candidate-" + item.Skill)
		if item.BaselineRequired {
			baselineSkills[item.Skill] = testSkillEvalDigest("baseline-" + item.Skill)
		}
	}
	// The maintainer selects the two bound executables and records their exact
	// digests; the fixture materializes the binaries so the driver's own
	// re-digestion of the current files can reproduce them.
	candidateExecutable, baselineExecutable := filepath.Join(root, "candidate-taskrail"), filepath.Join(root, "baseline-taskrail")
	if err := os.WriteFile(candidateExecutable, []byte("candidate binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(baselineExecutable, []byte("baseline binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	input := SkillEvalRunInput{
		SessionID:                 "publish-session",
		GeneratedAt:               time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC),
		TestedHead:                strings.Repeat("a", 40),
		ProductSHA256:             testSkillEvalDigest("product"),
		CandidateSkillsSHA256:     testSkillEvalDigest("candidate-skills"),
		BaselineSkillsSHA256:      testSkillEvalDigest("baseline-skills"),
		CandidateSkillSHA256:      candidateSkills,
		BaselineSkillSHA256:       baselineSkills,
		FixturesSHA256:            fixturesSHA256,
		CandidateExecutableSHA256: skillEvalBytesDigest([]byte("candidate binary")),
		BaselineExecutableSHA256:  skillEvalBytesDigest([]byte("baseline binary")),
		ArtifactRoot:              filepath.Join(root, "artifacts"),
		Registry:                  registry,
		Adapter:                   adapter,
		AdapterIdentity:           SkillEvalIdentity{Name: "adapter", Version: "1", Observed: true},
		ModelIdentity:             SkillEvalIdentity{Name: "model", Version: "1", Observed: true},
		DeterministicChecks:       SkillEvalDeterministicChecks{Outcome: "pass", Evidence: []string{"go-test-all"}},
	}
	stage, err := (SkillEvalRunner{}).Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	staged, err := RenderSkillEvalStage(stage)
	if err != nil {
		t.Fatalf("RenderSkillEvalStage: %v", err)
	}
	fixture := &skillEvalPublishFixture{
		root:                root,
		stagePath:           filepath.Join(root, "stage.json"),
		answersPath:         filepath.Join(root, "answers.json"),
		registryRoot:        registryRoot,
		artifactRoot:        input.ArtifactRoot,
		planningDir:         filepath.Join(root, "planning"),
		candidateExecutable: candidateExecutable,
		baselineExecutable:  baselineExecutable,
		input:               input,
	}
	fixture.bindings = SkillEvalPublishBindings{
		TestedHead:              input.TestedHead,
		ProductSHA256:           input.ProductSHA256,
		CandidateExecutablePath: candidateExecutable,
		BaselineExecutablePath:  baselineExecutable,
		CandidateSkillsSHA256:   input.CandidateSkillsSHA256,
		BaselineSkillsSHA256:    input.BaselineSkillsSHA256,
		CandidateSkillSHA256:    candidateSkills,
		BaselineSkillSHA256:     baselineSkills,
	}
	if err := os.WriteFile(fixture.stagePath, staged, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.answersPath, []byte(defaultPublishAnswers(t, registry, answersMutate)), 0o600); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (f *skillEvalPublishFixture) reportPath() string {
	return filepath.Join(f.planningDir, "reviews", "skill-evals", "v0.5.0", f.input.SessionID, "report.json")
}

func (f *skillEvalPublishFixture) request() SkillEvalPublishRequest {
	return SkillEvalPublishRequest{
		StagePath:    f.stagePath,
		AnswersPath:  f.answersPath,
		RegistryRoot: f.registryRoot,
		ArtifactRoot: f.artifactRoot,
		PlanningDir:  f.planningDir,
		Bindings:     f.bindings,
	}
}

func TestPublishSkillEvalReportWritesCanonicalCommittedReport(t *testing.T) {
	fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
	report, path, err := PublishSkillEvalReport(fixture.request())
	if err != nil {
		t.Fatalf("PublishSkillEvalReport: %v", err)
	}
	if path != fixture.reportPath() {
		t.Fatalf("report path = %q, want %q", path, fixture.reportPath())
	}
	if report.Outcome != "pass" || report.HumanReview != "Maintainer compared every frozen arm pair across sittings." {
		t.Fatalf("report = %#v", report)
	}
	if len(report.Cases) != len(fixture.input.Registry) {
		t.Fatalf("report cases = %d, want %d", len(report.Cases), len(fixture.input.Registry))
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read published report: %v", err)
	}
	if written[len(written)-1] != '\n' {
		t.Fatal("published report does not end with one final LF")
	}
	rendered, err := RenderSkillEvalReport(report)
	if err != nil {
		t.Fatalf("RenderSkillEvalReport: %v", err)
	}
	if string(written) != string(rendered) {
		t.Fatal("published bytes are not the canonical two-space-indented rendering")
	}
	if _, err := DecodeSkillEvalReport(written, report); err != nil {
		t.Fatalf("DecodeSkillEvalReport on published bytes: %v", err)
	}
	if strings.Contains(string(written), fixture.root) || strings.Contains(string(written), "raw/") {
		t.Fatal("published report leaked a producer-local path")
	}
	if _, _, err = PublishSkillEvalReport(fixture.request()); err != nil {
		t.Fatalf("republish: %v", err)
	}
	again, err := os.ReadFile(path)
	if err != nil || string(again) != string(written) {
		t.Fatalf("republished bytes differ: err=%v", err)
	}
}

func TestPublishSkillEvalReportPublishesIncompleteStage(t *testing.T) {
	fixture := stageSkillEvalPublishFixture(t,
		skillEvalUnavailableCandidateAdapter{caseID: "autonomous-task-local"},
		func(file *publishAnswerFile) {
			publishAnswerFor(file, "autonomous-task-local").Comparison = "inconclusive"
			publishAnswerFor(file, "autonomous-task-local").HumanReview = "The candidate arm was unavailable this session."
		})
	report, path, err := PublishSkillEvalReport(fixture.request())
	if err != nil {
		t.Fatalf("PublishSkillEvalReport: %v", err)
	}
	if report.Outcome != "incomplete" {
		t.Fatalf("outcome = %q, want incomplete", report.Outcome)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("published report missing: %v", err)
	}
}

func TestPublishSkillEvalReportRefusesStaleBindingsWithoutWriting(t *testing.T) {
	t.Run("raw evidence changed", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
		var facts string
		if err := filepath.WalkDir(fixture.artifactRoot, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && d.Name() == "facts.json" {
				facts = path
			}
			return nil
		}); err != nil {
			t.Fatalf("walk staged raw evidence: %v", err)
		}
		if facts == "" {
			t.Fatal("staged run produced no raw facts receipt")
		}
		if err := os.WriteFile(facts, []byte("drift\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		assertSkillEvalPublishRefused(t, fixture)
	})
	t.Run("registry worksheet drifted", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
		caseJSON := filepath.Join(fixture.registryRoot, "autonomous-task", "autonomous-task-committed", "case.json")
		data, err := os.ReadFile(caseJSON)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(caseJSON, []byte(strings.Replace(string(data), "review?", "changed?", 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		assertSkillEvalPublishRefused(t, fixture)
	})
	t.Run("registry case removed", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
		if err := os.RemoveAll(filepath.Join(fixture.registryRoot, "autonomous-task", "autonomous-task-committed")); err != nil {
			t.Fatal(err)
		}
		assertSkillEvalPublishRefused(t, fixture)
	})
	t.Run("stage seal tampered", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
		data, err := os.ReadFile(fixture.stagePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fixture.stagePath, []byte(strings.Replace(string(data), "inconclusive", "same", 1)), 0o600); err != nil {
			t.Fatal(err)
		}
		assertSkillEvalPublishRefused(t, fixture)
	})
	t.Run("missing artifact raw tree", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
		if err := os.RemoveAll(fixture.artifactRoot); err != nil {
			t.Fatal(err)
		}
		assertSkillEvalPublishRefused(t, fixture)
	})
}

// TestPublishSkillEvalReportRefusesDriftedCurrentBindings holds the sealed
// stage, the raw trees, and the answers fixed while exactly one current input
// drifts. Every variant must refuse before any write: the bindings the driver
// feeds Resume are derived from the current inputs, never echoed back from the
// stage being validated.
func TestPublishSkillEvalReportRefusesDriftedCurrentBindings(t *testing.T) {
	t.Run("tested head moved", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
		request := fixture.request()
		request.Bindings.TestedHead = strings.Repeat("b", 40)
		assertSkillEvalPublishRequestRefused(t, fixture, request)
	})
	t.Run("product snapshot changed", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
		request := fixture.request()
		request.Bindings.ProductSHA256 = testSkillEvalDigest("drifted-product")
		assertSkillEvalPublishRequestRefused(t, fixture, request)
	})
	t.Run("candidate executable rebuilt", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
		if err := os.WriteFile(fixture.candidateExecutable, []byte("rebuilt candidate binary"), 0o755); err != nil {
			t.Fatal(err)
		}
		assertSkillEvalPublishRefused(t, fixture)
	})
	t.Run("baseline executable rebuilt", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
		if err := os.WriteFile(fixture.baselineExecutable, []byte("rebuilt baseline binary"), 0o755); err != nil {
			t.Fatal(err)
		}
		assertSkillEvalPublishRefused(t, fixture)
	})
	t.Run("candidate skill set changed", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
		request := fixture.request()
		request.Bindings.CandidateSkillsSHA256 = testSkillEvalDigest("drifted-candidate-skills")
		assertSkillEvalPublishRequestRefused(t, fixture, request)
	})
	t.Run("baseline skill set changed", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
		request := fixture.request()
		request.Bindings.BaselineSkillsSHA256 = testSkillEvalDigest("drifted-baseline-skills")
		assertSkillEvalPublishRequestRefused(t, fixture, request)
	})
	t.Run("candidate skill subtree changed", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
		request := fixture.request()
		drifted := maps.Clone(request.Bindings.CandidateSkillSHA256)
		drifted["autonomous-task"] = testSkillEvalDigest("drifted-autonomous-task")
		request.Bindings.CandidateSkillSHA256 = drifted
		assertSkillEvalPublishRequestRefused(t, fixture, request)
	})
	t.Run("baseline skill subtree changed", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
		request := fixture.request()
		drifted := maps.Clone(request.Bindings.BaselineSkillSHA256)
		drifted["autonomous-task"] = testSkillEvalDigest("drifted-autonomous-task")
		request.Bindings.BaselineSkillSHA256 = drifted
		assertSkillEvalPublishRequestRefused(t, fixture, request)
	})
	t.Run("fixture tree changed", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
		drifted := filepath.Join(fixture.registryRoot, "autonomous-task", "autonomous-task-committed", "fixture", "drift.txt")
		if err := os.WriteFile(drifted, []byte("drifted fixture bytes"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertSkillEvalPublishRefused(t, fixture)
	})
}

func assertSkillEvalPublishRequestRefused(t *testing.T, fixture *skillEvalPublishFixture, request SkillEvalPublishRequest) {
	t.Helper()
	if _, _, err := PublishSkillEvalReport(request); err == nil {
		t.Fatal("PublishSkillEvalReport succeeded over drifted current bindings")
	}
	if _, statErr := os.Stat(fixture.reportPath()); !os.IsNotExist(statErr) {
		t.Fatal("refused publish still wrote a report")
	}
	if _, statErr := os.Stat(filepath.Join(fixture.planningDir, "reviews")); !os.IsNotExist(statErr) {
		t.Fatal("refused publish still created the review directory")
	}
}

func TestPublishSkillEvalReportRejectsDefectiveAnswers(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mutate     func(*publishAnswerFile)
		mutateJSON func(string) string
	}{
		{"omitted case", func(file *publishAnswerFile) {
			file.Cases = file.Cases[1:]
		}, nil},
		{"unregistered case", func(file *publishAnswerFile) {
			publishAnswerFor(file, "autonomous-task-committed").CaseID = "ghost-case"
		}, nil},
		{"repeated case", func(file *publishAnswerFile) {
			file.Cases = append(file.Cases, file.Cases[0])
		}, nil},
		{"wrong comparison family", func(file *publishAnswerFile) {
			publishAnswerFor(file, "autonomous-task-committed").Comparison = "candidate-only"
		}, nil},
		{"unsupported comparison", func(file *publishAnswerFile) {
			publishAnswerFor(file, "autonomous-task-committed").Comparison = "improved"
		}, nil},
		{"unsafe review text", func(file *publishAnswerFile) {
			publishAnswerFor(file, "autonomous-task-committed").HumanReview = "Reviewed /tmp/session/raw/candidate notes."
		}, nil},
		{"empty case review", func(file *publishAnswerFile) {
			publishAnswerFor(file, "autonomous-task-committed").HumanReview = ""
		}, nil},
		{"wrong schema version", func(file *publishAnswerFile) {
			file.SchemaVersion = 2
		}, nil},
		{"unknown member", nil, func(data string) string {
			return strings.Replace(data, `"schema_version": 1,`, `"schema_version": 1, "note": "side channel",`, 1)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, tc.mutate)
			if tc.mutateJSON != nil {
				if err := os.WriteFile(fixture.answersPath, []byte(tc.mutateJSON(string(mustReadFile(t, fixture.answersPath)))), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			assertSkillEvalPublishRefused(t, fixture)
		})
	}
}

// A baseline-required case whose baseline arm was never run stages incomplete
// exactly like a missing candidate arm, but through the baseline completeness
// branches. The driver must publish it only under an inconclusive answer and
// refuse any comparison its staged completeness does not permit.
func TestPublishSkillEvalReportPublishesBaselineIncompleteStage(t *testing.T) {
	t.Run("permitted inconclusive answer publishes incomplete", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t,
			skillEvalUnavailableBaselineAdapter{caseID: "autonomous-task-committed"},
			func(file *publishAnswerFile) {
				publishAnswerFor(file, "autonomous-task-committed").Comparison = "inconclusive"
				publishAnswerFor(file, "autonomous-task-committed").HumanReview = "The baseline arm was unavailable this session."
			})
		report, path, err := PublishSkillEvalReport(fixture.request())
		if err != nil {
			t.Fatalf("PublishSkillEvalReport: %v", err)
		}
		if report.Outcome != "incomplete" {
			t.Fatalf("outcome = %q, want incomplete", report.Outcome)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("published report missing: %v", err)
		}
		for _, item := range report.Cases {
			if item.CaseID == "autonomous-task-committed" && (item.Baseline != nil || item.Comparison != "inconclusive") {
				t.Fatalf("baseline-incomplete case = %+v", item)
			}
		}
	})
	t.Run("impermitted comparison refused", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t,
			skillEvalUnavailableBaselineAdapter{caseID: "autonomous-task-committed"},
			func(file *publishAnswerFile) {
				publishAnswerFor(file, "autonomous-task-committed").Comparison = "same"
			})
		_, _, err := PublishSkillEvalReport(fixture.request())
		if err == nil || !strings.Contains(err.Error(), "completeness") {
			t.Fatalf("error = %v, want a completeness refusal", err)
		}
		if _, statErr := os.Stat(fixture.reportPath()); !os.IsNotExist(statErr) {
			t.Fatal("refused publish still wrote a report")
		}
	})
}

type skillEvalUnavailableBaselineAdapter struct{ caseID string }

func (adapter skillEvalUnavailableBaselineAdapter) Run(ctx context.Context, request SkillEvalAdapterRequest) (SkillEvalAdapterResult, error) {
	if request.Arm == skillEvalBaselineArm && request.Case.CaseID == adapter.caseID {
		return SkillEvalAdapterResult{}, errSkillEvalArmUnavailable
	}
	return skillEvalTestAdapter{}.Run(ctx, request)
}

func TestPublishSkillEvalReportRejectsComparisonCompletenessPermits(t *testing.T) {
	// The candidate arm of autonomous-task-local was never run, so its staged
	// case is incomplete and the only comparison its completeness permits is
	// inconclusive; Resume would silently override any supplied answer, so the
	// driver must refuse it before writing anything.
	fixture := stageSkillEvalPublishFixture(t, skillEvalUnavailableCandidateAdapter{caseID: "autonomous-task-local"}, nil)
	_, _, err := PublishSkillEvalReport(fixture.request())
	if err == nil || !strings.Contains(err.Error(), "completeness") {
		t.Fatalf("error = %v, want a completeness refusal", err)
	}
	if _, statErr := os.Stat(fixture.reportPath()); !os.IsNotExist(statErr) {
		t.Fatal("refused publish still wrote a report")
	}
}

// The per-skill digests are current caller bindings, never values recovered
// from the stage under validation, so a binding set that does not cover every
// registry skill is refused before any write.
func TestPublishSkillEvalReportRequiresCompleteCurrentSkillDigests(t *testing.T) {
	t.Run("missing candidate skill digest", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
		request := fixture.request()
		incomplete := maps.Clone(request.Bindings.CandidateSkillSHA256)
		delete(incomplete, "autonomous-task")
		request.Bindings.CandidateSkillSHA256 = incomplete
		_, _, err := PublishSkillEvalReport(request)
		if err == nil || !strings.Contains(err.Error(), "no candidate skill digest") {
			t.Fatalf("error = %v, want a missing candidate digest refusal", err)
		}
		if _, statErr := os.Stat(fixture.reportPath()); !os.IsNotExist(statErr) {
			t.Fatal("refused publish still wrote a report")
		}
	})
	t.Run("missing baseline skill digest", func(t *testing.T) {
		fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
		request := fixture.request()
		incomplete := maps.Clone(request.Bindings.BaselineSkillSHA256)
		delete(incomplete, "autonomous-task")
		request.Bindings.BaselineSkillSHA256 = incomplete
		_, _, err := PublishSkillEvalReport(request)
		if err == nil || !strings.Contains(err.Error(), "no baseline skill digest") {
			t.Fatalf("error = %v, want a missing baseline digest refusal", err)
		}
		if _, statErr := os.Stat(fixture.reportPath()); !os.IsNotExist(statErr) {
			t.Fatal("refused publish still wrote a report")
		}
	})
}

func TestPublishSkillEvalReportRejectsIncompleteRequests(t *testing.T) {
	fixture := stageSkillEvalPublishFixture(t, skillEvalTestAdapter{}, nil)
	request := fixture.request()
	request.ArtifactRoot = ""
	if _, _, err := PublishSkillEvalReport(request); err == nil {
		t.Fatal("PublishSkillEvalReport accepted an empty artifact root")
	}
	for name, mutate := range map[string]func(*SkillEvalPublishRequest){
		"missing tested head":          func(request *SkillEvalPublishRequest) { request.Bindings.TestedHead = "" },
		"missing product digest":       func(request *SkillEvalPublishRequest) { request.Bindings.ProductSHA256 = "" },
		"missing candidate executable": func(request *SkillEvalPublishRequest) { request.Bindings.CandidateExecutablePath = "" },
		"missing baseline executable":  func(request *SkillEvalPublishRequest) { request.Bindings.BaselineExecutablePath = "" },
		"missing skill digest maps":    func(request *SkillEvalPublishRequest) { request.Bindings.CandidateSkillSHA256 = nil },
	} {
		t.Run(name, func(t *testing.T) {
			drifting := fixture.request()
			mutate(&drifting)
			if _, _, err := PublishSkillEvalReport(drifting); err == nil {
				t.Fatalf("PublishSkillEvalReport accepted a request with a %s", name)
			}
			if _, statErr := os.Stat(fixture.reportPath()); !os.IsNotExist(statErr) {
				t.Fatal("refused publish still wrote a report")
			}
		})
	}
	if _, _, err := PublishSkillEvalReport(fixture.request()); err != nil {
		t.Fatalf("baseline fixture must publish: %v", err)
	}
	if _, statErr := os.Stat(fixture.reportPath()); os.IsNotExist(statErr) {
		t.Fatalf("baseline fixture report missing: %v", statErr)
	}
}

func assertSkillEvalPublishRefused(t *testing.T, fixture *skillEvalPublishFixture) {
	t.Helper()
	_, _, err := PublishSkillEvalReport(fixture.request())
	if err == nil {
		t.Fatal("PublishSkillEvalReport succeeded over refused evidence")
	}
	if _, statErr := os.Stat(fixture.reportPath()); !os.IsNotExist(statErr) {
		t.Fatal("refused publish still wrote a report")
	}
	if _, statErr := os.Stat(filepath.Join(fixture.planningDir, "reviews")); !os.IsNotExist(statErr) {
		t.Fatal("refused publish still created the review directory")
	}
}

type skillEvalUnavailableCandidateAdapter struct{ caseID string }

func (adapter skillEvalUnavailableCandidateAdapter) Run(ctx context.Context, request SkillEvalAdapterRequest) (SkillEvalAdapterResult, error) {
	if request.Arm == skillEvalCandidateArm && request.Case.CaseID == adapter.caseID {
		return SkillEvalAdapterResult{}, errSkillEvalArmUnavailable
	}
	return skillEvalTestAdapter{}.Run(ctx, request)
}
