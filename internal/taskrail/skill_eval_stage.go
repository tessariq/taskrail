package taskrail

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

// SkillEvalStage is portable frozen evidence between provider execution and
// maintainer comparison. Raw roots are always reconstructed from current input.
type SkillEvalStage struct {
	SchemaVersion int                      `json:"schema_version"`
	Report        SkillEvalReport          `json:"report"`
	Worksheet     []SkillEvalWorksheetCase `json:"worksheet"`
	Seal          string                   `json:"seal"`
}

// SkillEvalWorksheetCase gives the maintainer the frozen arm outcomes and the
// exact questions to answer without exposing a renderable draft report.
type SkillEvalWorksheetCase struct {
	CaseID               string        `json:"case_id"`
	Skill                string        `json:"skill"`
	HumanReviewQuestions []string      `json:"human_review_questions"`
	Candidate            *SkillEvalRun `json:"candidate"`
	Baseline             *SkillEvalRun `json:"baseline"`
}

func validateSkillEvalRunInput(input SkillEvalRunInput, requireReviews bool) error {
	if !skillEvalSessionID.MatchString(input.SessionID) || len(input.SessionID) > 64 {
		return fmt.Errorf("skill evaluation session ID is not portable")
	}
	if input.GeneratedAt.IsZero() || input.GeneratedAt.Location() != time.UTC {
		return fmt.Errorf("skill evaluation timestamp must be UTC")
	}
	for _, digest := range []string{input.TestedHead, input.CandidateExecutableSHA256, input.BaselineExecutableSHA256, input.ProductSHA256, input.CandidateSkillsSHA256, input.BaselineSkillsSHA256, input.FixturesSHA256} {
		if !skillEvalDigestPattern.MatchString(digest) {
			return fmt.Errorf("skill evaluation contains an invalid digest")
		}
	}
	if input.ArtifactRoot == "" || input.Adapter == nil {
		return fmt.Errorf("skill evaluation requires an artifact root and adapter")
	}
	if err := validateSkillEvalIdentity(input.AdapterIdentity); err != nil {
		return fmt.Errorf("adapter identity: %w", err)
	}
	if err := validateSkillEvalIdentity(input.ModelIdentity); err != nil {
		return fmt.Errorf("model identity: %w", err)
	}
	if err := validateSkillEvalChecks(input.DeterministicChecks); err != nil {
		return err
	}
	if requireReviews {
		if err := validateSkillEvalSummary(input.HumanReview, input.ArtifactRoot); err != nil {
			return fmt.Errorf("human review: %w", err)
		}
	}
	if len(input.Registry) == 0 {
		return fmt.Errorf("skill evaluation registry is empty")
	}
	seen := map[string]bool{}
	for _, item := range input.Registry {
		if item.CaseID == "" || len(item.CaseID) > 64 || seen[item.CaseID] || !skillEvalDigestPattern.MatchString(item.FixtureSHA256) {
			return fmt.Errorf("skill evaluation registry is invalid")
		}
		seen[item.CaseID] = true
		if !skillEvalCaseID.MatchString(item.CaseID) || !skillEvalCaseID.MatchString(item.Skill) {
			return fmt.Errorf("skill evaluation case %q has an unsafe raw evidence path", item.CaseID)
		}
		if err := validateSkillEvalCaseDefinition(item); err != nil {
			return fmt.Errorf("skill evaluation case %q: %w", item.CaseID, err)
		}
		if !skillEvalDigestPattern.MatchString(input.CandidateSkillSHA256[item.Skill]) {
			return fmt.Errorf("skill evaluation case %q has no candidate skill digest", item.CaseID)
		}
		if item.BaselineRequired && !skillEvalDigestPattern.MatchString(input.BaselineSkillSHA256[item.Skill]) {
			return fmt.Errorf("skill evaluation case %q has no baseline skill digest", item.CaseID)
		}
	}
	return nil
}

// Resume consumes the sealed staged arms and supplied human conclusions. It never
// invokes the adapter, so a review is always bound to the exact prior evidence.
func (SkillEvalRunner) Resume(stage SkillEvalStage, input SkillEvalRunInput, humanReview string, reviews map[string]SkillEvalCaseReview) (SkillEvalReport, error) {
	if err := validateSkillEvalStageBindings(stage, input); err != nil {
		return SkillEvalReport{}, err
	}
	if err := validateSkillEvalStage(stage, input); err != nil {
		return SkillEvalReport{}, err
	}
	if err := validateSkillEvalSummary(humanReview, ""); err != nil {
		return SkillEvalReport{}, fmt.Errorf("human review: %w", err)
	}
	if len(reviews) != len(stage.Report.Cases) {
		return SkillEvalReport{}, fmt.Errorf("human reviews do not exactly cover staged cases")
	}
	report := stage.Report
	report.HumanReview = humanReview
	for index := range report.Cases {
		item := &report.Cases[index]
		review, ok := reviews[item.CaseID]
		if !ok || validateSkillEvalSummary(review.HumanReview, "") != nil {
			return SkillEvalReport{}, fmt.Errorf("skill evaluation case %q has an invalid human review", item.CaseID)
		}
		if !skillEvalCaseComplete(*item) {
			item.Comparison = "inconclusive"
		} else if item.BaselineRequired && !validSkillEvalPairedComparison(review.Comparison) || !item.BaselineRequired && review.Comparison != "candidate-only" {
			return SkillEvalReport{}, fmt.Errorf("skill evaluation case %q has an invalid comparison", item.CaseID)
		} else {
			item.Comparison = review.Comparison
		}
		item.HumanReview = review.HumanReview
	}
	report.Outcome = skillEvalOutcome(report)
	return report, nil
}

func skillEvalStageSeal(stage SkillEvalStage) string {
	payload, _ := json.Marshal(struct {
		SchemaVersion int                      `json:"schema_version"`
		Report        SkillEvalReport          `json:"report"`
		Worksheet     []SkillEvalWorksheetCase `json:"worksheet"`
	}{SchemaVersion: stage.SchemaVersion, Report: stage.Report, Worksheet: stage.Worksheet})
	digest := sha256.Sum256(append([]byte("taskrail-skill-eval-stage-v1\x00"), payload...))
	return hex.EncodeToString(digest[:])
}

func validateSkillEvalStage(stage SkillEvalStage, input SkillEvalRunInput) error {
	if stage.SchemaVersion != 1 || !skillEvalDigestPattern.MatchString(stage.Seal) || stage.Seal != skillEvalStageSeal(stage) {
		return fmt.Errorf("skill evaluation staged evidence was modified")
	}
	if len(stage.Worksheet) != len(stage.Report.Cases) {
		return fmt.Errorf("skill evaluation staged worksheet is incomplete")
	}
	for index, item := range stage.Report.Cases {
		worksheet := stage.Worksheet[index]
		if worksheet.CaseID != item.CaseID || worksheet.Skill != item.Skill || !skillEvalSameRun(worksheet.Candidate, item.Candidate) || !skillEvalSameRun(worksheet.Baseline, item.Baseline) || !skillEvalSafeStrings(worksheet.HumanReviewQuestions) {
			return fmt.Errorf("skill evaluation staged worksheet is stale")
		}
		for _, arm := range []struct {
			name string
			run  *SkillEvalRun
		}{{skillEvalCandidateArm, item.Candidate}, {skillEvalBaselineArm, item.Baseline}} {
			if arm.run == nil {
				continue
			}
			root := skillEvalRawRoot(input, SkillEvalCase{Skill: item.Skill, CaseID: item.CaseID}, arm.name)
			digest, err := nonEmptySkillEvalRawDigest(root)
			if err != nil || digest != arm.run.RawSHA256 {
				return fmt.Errorf("skill evaluation staged %s arm %q raw evidence changed", arm.name, item.CaseID)
			}
		}
	}
	return nil
}

func skillEvalSameRun(a, b *SkillEvalRun) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func validateSkillEvalStageBindings(stage SkillEvalStage, input SkillEvalRunInput) error {
	if err := validateSkillEvalRunInput(input, false); err != nil {
		return fmt.Errorf("skill evaluation resume bindings: %w", err)
	}
	report := stage.Report
	if input.SessionID != report.SessionID || input.GeneratedAt.UTC().Format(time.RFC3339) != report.GeneratedAt || input.TestedHead != report.TestedHead || input.CandidateExecutableSHA256 != report.CandidateExecutableSHA256 || input.BaselineExecutableSHA256 != report.BaselineExecutableSHA256 || input.ProductSHA256 != report.ProductSHA256 || input.CandidateSkillsSHA256 != report.Candidate.SkillsSHA256 || input.BaselineSkillsSHA256 != report.Baseline.SkillsSHA256 || input.FixturesSHA256 != report.FixturesSHA256 || len(input.Registry) != len(report.Cases) {
		return fmt.Errorf("skill evaluation staged bindings are stale")
	}
	cases := slices.Clone(input.Registry)
	slices.SortFunc(cases, compareSkillEvalCases)
	for index, item := range report.Cases {
		caseInput := cases[index]
		if compareSkillEvalCases(caseInput, SkillEvalCase{CaseID: item.CaseID, Skill: item.Skill}) != 0 || caseInput.StorageMode != item.StorageMode || caseInput.FixtureSHA256 != item.FixtureSHA256 || caseInput.BaselineRequired != item.BaselineRequired || item.Candidate != nil && input.CandidateSkillSHA256[item.Skill] != item.Candidate.SkillSHA256 || item.Baseline != nil && input.BaselineSkillSHA256[item.Skill] != item.Baseline.SkillSHA256 {
			return fmt.Errorf("skill evaluation staged case %q bindings are stale", item.CaseID)
		}
		if !slices.Equal(stage.Worksheet[index].HumanReviewQuestions, caseInput.HumanReviewQuestions) {
			return fmt.Errorf("skill evaluation staged case %q worksheet is stale", item.CaseID)
		}
	}
	return nil
}

// RenderSkillEvalStage emits portable canonical staged evidence for asynchronous
// review. It deliberately contains no producer-local raw roots.
func RenderSkillEvalStage(stage SkillEvalStage) ([]byte, error) {
	if stage.SchemaVersion != 1 || stage.Seal != skillEvalStageSeal(stage) {
		return nil, fmt.Errorf("render skill evaluation stage: invalid seal or schema")
	}
	data, err := json.MarshalIndent(stage, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("render skill evaluation stage: %w", err)
	}
	return append(data, '\n'), nil
}

// DecodeSkillEvalStage reconstructs producer-local raw roots from current input
// and rejects noncanonical, stale, or altered evidence before review resumes.
func DecodeSkillEvalStage(data []byte, input SkillEvalRunInput) (SkillEvalStage, error) {
	if err := checkDocumentFraming(data); err != nil {
		return SkillEvalStage{}, fmt.Errorf("decode skill evaluation stage: %w", err)
	}
	var stage SkillEvalStage
	if err := json.Unmarshal(data, &stage); err != nil {
		return SkillEvalStage{}, fmt.Errorf("decode skill evaluation stage: %w", err)
	}
	stage.Report.manifest = skillEvalManifest(input.Registry, input)
	canonical, err := RenderSkillEvalStage(stage)
	if err != nil || !bytes.Equal(data, canonical) {
		return SkillEvalStage{}, fmt.Errorf("skill evaluation stage is not canonical schema-v1 JSON")
	}
	if err := validateSkillEvalStageBindings(stage, input); err != nil {
		return SkillEvalStage{}, err
	}
	if err := validateSkillEvalStage(stage, input); err != nil {
		return SkillEvalStage{}, err
	}
	return stage, nil
}
