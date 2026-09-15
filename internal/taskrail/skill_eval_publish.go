package taskrail

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SkillEvalPublishRequest names the producer-local inputs of the resume
// publication driver. The staging run chose every binding; this driver only
// re-supplies them, so every path here stays out of the committed report.
type SkillEvalPublishRequest struct {
	StagePath    string
	AnswersPath  string
	RegistryRoot string
	ArtifactRoot string
	PlanningDir  string
	Bindings     SkillEvalPublishBindings
}

// SkillEvalPublishBindings carries the freshly recomputed caller bindings the
// stage must still match: the current product snapshot, the two bound
// executables, and the current candidate and baseline skill digests. They are
// derived from the current trees and binaries — the same facilities the
// staging run used — never echoed back from the stage under validation, so
// Resume can refuse a stage whose inputs drifted after it was sealed.
type SkillEvalPublishBindings struct {
	TestedHead              string
	ProductSHA256           string
	CandidateExecutablePath string
	BaselineExecutablePath  string
	CandidateSkillsSHA256   string
	BaselineSkillsSHA256    string
	CandidateSkillSHA256    map[string]string
	BaselineSkillSHA256     map[string]string
}

// skillEvalResumeAdapter occupies the run input's non-nil adapter slot. Resume
// never invokes an adapter, so any future call path that reaches it fails
// loudly instead of silently rerunning a provider arm.
type skillEvalResumeAdapter struct{}

func (skillEvalResumeAdapter) Run(context.Context, SkillEvalAdapterRequest) (SkillEvalAdapterResult, error) {
	return SkillEvalAdapterResult{}, fmt.Errorf("skill evaluation resume must not invoke an adapter arm")
}

// PublishSkillEvalReport turns a sealed skill-evaluation stage plus the
// maintainer's adopted worksheet answers into the committed release report at
// <planning-dir>/reviews/skill-evals/v0.5.0/<session-id>/report.json. No
// adapter arm runs: the stage is decoded against the current registry and raw
// trees plus the freshly recomputed caller bindings (current snapshot,
// executables, skills, and fixtures digest), the answers are checked against
// staged completeness, and Resume renders the report. Nothing is written
// unless every step passes, and republishing the same stage and answers
// writes byte-identical output.
func PublishSkillEvalReport(request SkillEvalPublishRequest) (SkillEvalReport, string, error) {
	if err := validateSkillEvalPublishRequest(request); err != nil {
		return SkillEvalReport{}, "", err
	}
	stageBytes, err := os.ReadFile(request.StagePath)
	if err != nil {
		return SkillEvalReport{}, "", fmt.Errorf("read skill evaluation stage: %w", err)
	}
	answersBytes, err := os.ReadFile(request.AnswersPath)
	if err != nil {
		return SkillEvalReport{}, "", fmt.Errorf("read skill evaluation answers: %w", err)
	}
	stage, err := parseSkillEvalStageDocument(stageBytes)
	if err != nil {
		return SkillEvalReport{}, "", err
	}
	registry, err := loadSkillEvalRegistry(request.RegistryRoot, shippableSkills)
	if err != nil {
		return SkillEvalReport{}, "", err
	}
	fixturesSHA256, err := skillEvalTreeDigest("taskrail-skill-eval-fixtures-v1", request.RegistryRoot)
	if err != nil {
		return SkillEvalReport{}, "", fmt.Errorf("digest skill evaluation fixtures: %w", err)
	}
	candidateExecutableSHA256, baselineExecutableSHA256, err := skillEvalExecutableDigests(request.Bindings)
	if err != nil {
		return SkillEvalReport{}, "", err
	}
	derived := skillEvalCurrentDigests{
		FixturesSHA256:            fixturesSHA256,
		CandidateExecutableSHA256: candidateExecutableSHA256,
		BaselineExecutableSHA256:  baselineExecutableSHA256,
	}
	input, err := skillEvalResumeRunInput(stage, registry, request.ArtifactRoot, request.Bindings, derived)
	if err != nil {
		return SkillEvalReport{}, "", err
	}
	decoded, err := DecodeSkillEvalStage(stageBytes, input)
	if err != nil {
		return SkillEvalReport{}, "", err
	}
	humanReview, reviews, err := decodeSkillEvalAnswers(answersBytes, decoded)
	if err != nil {
		return SkillEvalReport{}, "", err
	}
	report, err := (SkillEvalRunner{}).Resume(decoded, input, humanReview, reviews)
	if err != nil {
		return SkillEvalReport{}, "", err
	}
	rendered, err := RenderSkillEvalReport(report)
	if err != nil {
		return SkillEvalReport{}, "", err
	}
	directory := filepath.Join(request.PlanningDir, "reviews", "skill-evals", "v0.5.0", report.SessionID)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return SkillEvalReport{}, "", fmt.Errorf("create skill evaluation review directory: %w", err)
	}
	path := filepath.Join(directory, "report.json")
	if err := os.WriteFile(path, rendered, 0o644); err != nil {
		return SkillEvalReport{}, "", fmt.Errorf("write skill evaluation report: %w", err)
	}
	return report, path, nil
}

func validateSkillEvalPublishRequest(request SkillEvalPublishRequest) error {
	for _, field := range []struct{ name, value string }{
		{"stage path", request.StagePath},
		{"answers path", request.AnswersPath},
		{"registry root", request.RegistryRoot},
		{"artifact root", request.ArtifactRoot},
		{"planning dir", request.PlanningDir},
		{"tested head", request.Bindings.TestedHead},
		{"product digest", request.Bindings.ProductSHA256},
		{"candidate executable path", request.Bindings.CandidateExecutablePath},
		{"baseline executable path", request.Bindings.BaselineExecutablePath},
		{"candidate skills digest", request.Bindings.CandidateSkillsSHA256},
		{"baseline skills digest", request.Bindings.BaselineSkillsSHA256},
	} {
		if field.value == "" {
			return fmt.Errorf("skill evaluation publication requires a %s", field.name)
		}
	}
	if len(request.Bindings.CandidateSkillSHA256) == 0 || len(request.Bindings.BaselineSkillSHA256) == 0 {
		return fmt.Errorf("skill evaluation publication requires current per-skill digests")
	}
	return nil
}

// skillEvalExecutableDigests records the SHA-256 of the two bound executables
// exactly as they stand now, so a binary rebuilt or replaced after staging
// refuses the stage instead of silently republishing under stale bindings.
func skillEvalExecutableDigests(bindings SkillEvalPublishBindings) (string, string, error) {
	candidateBytes, err := os.ReadFile(bindings.CandidateExecutablePath)
	if err != nil {
		return "", "", fmt.Errorf("read skill evaluation candidate executable: %w", err)
	}
	baselineBytes, err := os.ReadFile(bindings.BaselineExecutablePath)
	if err != nil {
		return "", "", fmt.Errorf("read skill evaluation baseline executable: %w", err)
	}
	return skillEvalBytesDigest(candidateBytes), skillEvalBytesDigest(baselineBytes), nil
}

// parseSkillEvalStageDocument pre-reads the staged record far enough to
// reconstruct the run input. DecodeSkillEvalStage re-validates the same bytes
// canonically, so nothing here trusts an unsealed field.
func parseSkillEvalStageDocument(data []byte) (SkillEvalStage, error) {
	if err := checkDocumentFraming(data); err != nil {
		return SkillEvalStage{}, fmt.Errorf("read skill evaluation stage: %w", err)
	}
	var stage SkillEvalStage
	if err := json.Unmarshal(data, &stage); err != nil {
		return SkillEvalStage{}, fmt.Errorf("read skill evaluation stage: %w", err)
	}
	return stage, nil
}

// skillEvalCurrentDigests collects the current digests the driver derives
// itself from the request's current inputs, complementary to the caller's
// recomputed bindings.
type skillEvalCurrentDigests struct {
	FixturesSHA256            string
	CandidateExecutableSHA256 string
	BaselineExecutableSHA256  string
}

// skillEvalResumeRunInput assembles the run input Resume validates the stage
// against. Every digest-bearing field is genuinely current — the caller's
// freshly recomputed snapshot and skill bindings, the driver-derived fixtures
// and executable digests, and the current registry — never a value echoed
// back from the stage under validation. Only staging-run records (session,
// timestamp, identities, deterministic checks), which no current tree can
// recompute, still come from the sealed report.
func skillEvalResumeRunInput(stage SkillEvalStage, registry []SkillEvalCase, artifactRoot string, bindings SkillEvalPublishBindings, current skillEvalCurrentDigests) (SkillEvalRunInput, error) {
	generatedAt, err := time.Parse(time.RFC3339, stage.Report.GeneratedAt)
	if err != nil {
		return SkillEvalRunInput{}, fmt.Errorf("skill evaluation stage has an invalid timestamp")
	}
	return SkillEvalRunInput{
		SessionID:                 stage.Report.SessionID,
		GeneratedAt:               generatedAt,
		TestedHead:                bindings.TestedHead,
		CandidateExecutableSHA256: current.CandidateExecutableSHA256,
		BaselineExecutableSHA256:  current.BaselineExecutableSHA256,
		ProductSHA256:             bindings.ProductSHA256,
		CandidateSkillsSHA256:     bindings.CandidateSkillsSHA256,
		BaselineSkillsSHA256:      bindings.BaselineSkillsSHA256,
		CandidateSkillSHA256:      bindings.CandidateSkillSHA256,
		BaselineSkillSHA256:       bindings.BaselineSkillSHA256,
		FixturesSHA256:            current.FixturesSHA256,
		ArtifactRoot:              artifactRoot,
		Registry:                  registry,
		Adapter:                   skillEvalResumeAdapter{},
		AdapterIdentity:           stage.Report.Adapter,
		ModelIdentity:             stage.Report.Model,
		DeterministicChecks:       stage.Report.DeterministicChecks,
	}, nil
}

type skillEvalAnswerCase struct {
	CaseID      string
	Comparison  string
	HumanReview string
}

// decodeSkillEvalAnswers reads the maintainer's worksheet answers and checks
// them against the decoded stage before anything is written. Resume tolerates
// a supplied comparison for an incomplete case by silently recording
// inconclusive; the driver refuses it instead, because a maintainer answer the
// report silently overrides is not an answer at all.
func decodeSkillEvalAnswers(data []byte, stage SkillEvalStage) (string, map[string]SkillEvalCaseReview, error) {
	if err := checkDocumentFraming(data); err != nil {
		return "", nil, fmt.Errorf("read skill evaluation answers: %w", err)
	}
	object, err := strictObject(data, "skill evaluation answers")
	if err != nil {
		return "", nil, err
	}
	if err := exactMembers(object, "skill evaluation answers", []string{"schema_version", "human_review", "cases"}); err != nil {
		return "", nil, err
	}
	version, ok := decodeJSONInteger(object["schema_version"])
	if !ok || version != 1 {
		return "", nil, fmt.Errorf("skill evaluation answers member %q must be integer 1", "schema_version")
	}
	humanReview, err := stringMember(object, "skill evaluation answers", "human_review")
	if err != nil {
		return "", nil, err
	}
	if err := validateSkillEvalSummary(humanReview, ""); err != nil {
		return "", nil, fmt.Errorf("skill evaluation answers human review: %w", err)
	}
	elements, err := arrayMember(object["cases"], "skill evaluation answers", "cases")
	if err != nil || len(elements) == 0 {
		return "", nil, fmt.Errorf("skill evaluation answers member %q must be a non-empty array", "cases")
	}
	reviews := map[string]SkillEvalCaseReview{}
	for _, element := range elements {
		answer, err := decodeSkillEvalAnswerCase(element)
		if err != nil {
			return "", nil, err
		}
		if _, exists := reviews[answer.CaseID]; exists {
			return "", nil, fmt.Errorf("skill evaluation answers repeat case %q", answer.CaseID)
		}
		reviews[answer.CaseID] = SkillEvalCaseReview{Comparison: answer.Comparison, HumanReview: answer.HumanReview}
	}
	staged := map[string]bool{}
	for _, item := range stage.Report.Cases {
		staged[item.CaseID] = true
	}
	for caseID := range reviews {
		if !staged[caseID] {
			return "", nil, fmt.Errorf("skill evaluation answers name unregistered case %q", caseID)
		}
	}
	if err := validateSkillEvalAnswerCoverage(reviews, stage.Report.Cases); err != nil {
		return "", nil, err
	}
	return humanReview, reviews, nil
}

// validateSkillEvalAnswerCoverage requires one answer per staged case, with a
// comparison its staged completeness permits.
func validateSkillEvalAnswerCoverage(reviews map[string]SkillEvalCaseReview, cases []SkillEvalCaseReport) error {
	for _, item := range cases {
		answer, exists := reviews[item.CaseID]
		if !exists {
			return fmt.Errorf("skill evaluation answers omit staged case %q", item.CaseID)
		}
		if !skillEvalAnswerComparisonPermitted(item, answer.Comparison) {
			return fmt.Errorf("skill evaluation answers comparison %q for case %q is not permitted by its staged completeness", answer.Comparison, item.CaseID)
		}
	}
	return nil
}

func decodeSkillEvalAnswerCase(element []byte) (skillEvalAnswerCase, error) {
	item, err := strictObject(element, "skill evaluation answer")
	if err != nil {
		return skillEvalAnswerCase{}, err
	}
	if err := exactMembers(item, "skill evaluation answer", []string{"case_id", "comparison", "human_review"}); err != nil {
		return skillEvalAnswerCase{}, err
	}
	caseID, err := stringMember(item, "skill evaluation answer", "case_id")
	if err != nil {
		return skillEvalAnswerCase{}, err
	}
	comparison, err := enumMember(item, "skill evaluation answer", "comparison", []string{"better", "same", "worse", "candidate-only", "inconclusive"})
	if err != nil {
		return skillEvalAnswerCase{}, err
	}
	review, err := stringMember(item, "skill evaluation answer", "human_review")
	if err != nil {
		return skillEvalAnswerCase{}, err
	}
	if err := validateSkillEvalSummary(review, ""); err != nil {
		return skillEvalAnswerCase{}, fmt.Errorf("skill evaluation answers case %q human review: %w", caseID, err)
	}
	return skillEvalAnswerCase{CaseID: caseID, Comparison: comparison, HumanReview: review}, nil
}

// skillEvalAnswerComparisonPermitted mirrors the report contract: an
// incomplete case can only be inconclusive, a complete paired case records a
// human better/same/worse disposition, and a complete no-baseline case is
// exactly candidate-only.
func skillEvalAnswerComparisonPermitted(item SkillEvalCaseReport, comparison string) bool {
	if !skillEvalCaseComplete(item) {
		return comparison == "inconclusive"
	}
	if item.BaselineRequired {
		return validSkillEvalPairedComparison(comparison)
	}
	return comparison == "candidate-only"
}
