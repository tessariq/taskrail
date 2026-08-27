package taskrail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	skillEvalCandidateArm = "candidate"
	skillEvalBaselineArm  = "baseline"
)

var skillEvalRequiredWaiverChecks = []string{
	"command", "cross-platform", "lifecycle", "machine-api", "parity", "security",
}

var (
	skillEvalDigestPattern     = regexp.MustCompile(`^[a-f0-9]{64}$`)
	skillEvalSessionID         = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	skillEvalWindowsPath       = regexp.MustCompile(`(?i)^[a-z]:[\\/]`)
	skillEvalLocalPath         = regexp.MustCompile(`(?i)(^|[\s(\[=:'"])(/|~/|~\\|[a-z]:[\\/]|\\\\)`)
	errSkillEvalArmUnavailable = errors.New("skill evaluation arm unavailable")
)

// SkillEvalAdapter is supplied by a maintainer. It owns provider selection and
// must write raw output only to the RawRoot provided for the requested arm.
type SkillEvalAdapter interface {
	Run(context.Context, SkillEvalAdapterRequest) (SkillEvalAdapterResult, error)
}

type SkillEvalAdapterRequest struct {
	Case    SkillEvalCase
	Arm     string
	RawRoot string
}

type SkillEvalAdapterResult struct {
	Outcome            string
	DeterministicGrade string
}

type SkillEvalIdentity struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Observed bool   `json:"observed"`
}

type SkillEvalDeterministicChecks struct {
	Outcome  string   `json:"outcome"`
	Evidence []string `json:"evidence"`
}

// SkillEvalWaiver records a maintainer's explicit acceptance of unavailable
// behavioral evidence. It cannot turn failed deterministic evidence into a pass.
type SkillEvalWaiver struct {
	Approver              string   `json:"approver"`
	Reason                string   `json:"reason"`
	UnavailableCapability string   `json:"unavailable_capability"`
	AffectedSkills        []string `json:"affected_skills"`
	AffectedCases         []string `json:"affected_cases"`
	ResidualRisk          string   `json:"residual_risk"`
	CompensatingEvidence  []string `json:"compensating_evidence"`
	Followup              string   `json:"followup"`
}

type SkillEvalCaseReview struct {
	Comparison  string
	HumanReview string
}

type SkillEvalRunInput struct {
	SessionID                 string
	GeneratedAt               time.Time
	TestedHead                string
	CandidateExecutableSHA256 string
	BaselineExecutableSHA256  string
	ProductSHA256             string
	CandidateSkillsSHA256     string
	BaselineSkillsSHA256      string
	CandidateSkillSHA256      map[string]string
	BaselineSkillSHA256       map[string]string
	FixturesSHA256            string
	ArtifactRoot              string
	Registry                  []SkillEvalCase
	Adapter                   SkillEvalAdapter
	AdapterIdentity           SkillEvalIdentity
	ModelIdentity             SkillEvalIdentity
	DeterministicChecks       SkillEvalDeterministicChecks
	HumanReview               string
	CaseReviews               map[string]SkillEvalCaseReview
}

type SkillEvalRun struct {
	Outcome            string `json:"outcome"`
	SkillSHA256        string `json:"skill_sha256"`
	ExecutableSHA256   string `json:"executable_sha256"`
	DeterministicGrade string `json:"deterministic_grade"`
	RawSHA256          string `json:"raw_sha256"`
}

type SkillEvalCaseReport struct {
	CaseID           string        `json:"case_id"`
	Skill            string        `json:"skill"`
	StorageMode      string        `json:"storage_mode"`
	FixtureSHA256    string        `json:"fixture_sha256"`
	BaselineRequired bool          `json:"baseline_required"`
	Candidate        *SkillEvalRun `json:"candidate"`
	Baseline         *SkillEvalRun `json:"baseline"`
	Comparison       string        `json:"comparison"`
	HumanReview      string        `json:"human_review"`
}

type SkillEvalSkillSet struct {
	TaskrailVersion string `json:"taskrail_version"`
	SkillsSHA256    string `json:"skills_sha256"`
}

// SkillEvalReport is a safe summary only. Raw provider output remains in the
// caller-selected artifacts root and is represented here solely by a digest.
type SkillEvalReport struct {
	SchemaVersion             int                          `json:"schema_version"`
	SessionID                 string                       `json:"session_id"`
	GeneratedAt               string                       `json:"generated_at"`
	TestedHead                string                       `json:"tested_head"`
	CandidateExecutableSHA256 string                       `json:"candidate_executable_sha256"`
	BaselineExecutableSHA256  string                       `json:"baseline_executable_sha256"`
	ProductSHA256             string                       `json:"product_sha256"`
	Outcome                   string                       `json:"outcome"`
	Candidate                 SkillEvalSkillSet            `json:"candidate"`
	Baseline                  SkillEvalSkillSet            `json:"baseline"`
	FixturesSHA256            string                       `json:"fixtures_sha256"`
	Adapter                   SkillEvalIdentity            `json:"adapter"`
	Model                     SkillEvalIdentity            `json:"model"`
	Cases                     []SkillEvalCaseReport        `json:"cases"`
	DeterministicChecks       SkillEvalDeterministicChecks `json:"deterministic_checks"`
	HumanReview               string                       `json:"human_review"`
	Waiver                    *SkillEvalWaiver             `json:"waiver"`
	manifest                  skillEvalReportManifest
}

type skillEvalReportManifest struct {
	Cases map[string]skillEvalExpectedCase
}

type skillEvalExpectedCase struct {
	Skill            string
	StorageMode      string
	FixtureSHA256    string
	BaselineRequired bool
	CandidateSkill   string
	BaselineSkill    string
}

type SkillEvalRunner struct{}

// Run invokes each required arm once. Adapter failures are reportable missing
// evidence rather than runner failures, so unavailable credentials can never be
// mistaken for a passing release evaluation.
func (SkillEvalRunner) Run(ctx context.Context, input SkillEvalRunInput) (SkillEvalReport, error) {
	if err := validateSkillEvalRunInput(input); err != nil {
		return SkillEvalReport{}, err
	}
	cases := slices.Clone(input.Registry)
	slices.SortFunc(cases, compareSkillEvalCases)
	manifest := skillEvalReportManifest{Cases: make(map[string]skillEvalExpectedCase, len(cases))}
	for _, item := range cases {
		manifest.Cases[item.CaseID] = skillEvalExpectedCase{
			Skill: item.Skill, StorageMode: item.StorageMode, FixtureSHA256: item.FixtureSHA256,
			BaselineRequired: item.BaselineRequired, CandidateSkill: input.CandidateSkillSHA256[item.Skill], BaselineSkill: input.BaselineSkillSHA256[item.Skill],
		}
	}
	report := SkillEvalReport{
		SchemaVersion:             1,
		SessionID:                 input.SessionID,
		GeneratedAt:               input.GeneratedAt.UTC().Format(time.RFC3339),
		TestedHead:                input.TestedHead,
		CandidateExecutableSHA256: input.CandidateExecutableSHA256,
		BaselineExecutableSHA256:  input.BaselineExecutableSHA256,
		ProductSHA256:             input.ProductSHA256,
		Candidate:                 SkillEvalSkillSet{TaskrailVersion: "v0.5.0", SkillsSHA256: input.CandidateSkillsSHA256},
		Baseline:                  SkillEvalSkillSet{TaskrailVersion: "v0.4.0", SkillsSHA256: input.BaselineSkillsSHA256},
		FixturesSHA256:            input.FixturesSHA256,
		Adapter:                   input.AdapterIdentity,
		Model:                     input.ModelIdentity,
		DeterministicChecks:       input.DeterministicChecks,
		HumanReview:               input.HumanReview,
		Waiver:                    nil,
		manifest:                  manifest,
	}
	for _, evaluation := range cases {
		review := input.CaseReviews[evaluation.CaseID]
		item := SkillEvalCaseReport{
			CaseID: evaluation.CaseID, Skill: evaluation.Skill, StorageMode: evaluation.StorageMode,
			FixtureSHA256: evaluation.FixtureSHA256, BaselineRequired: evaluation.BaselineRequired,
			HumanReview: review.HumanReview,
		}
		var err error
		item.Candidate, err = runSkillEvalArm(ctx, input, evaluation, skillEvalCandidateArm)
		if err != nil {
			return SkillEvalReport{}, fmt.Errorf("candidate %s: %w", evaluation.CaseID, err)
		}
		if evaluation.BaselineRequired {
			item.Baseline, err = runSkillEvalArm(ctx, input, evaluation, skillEvalBaselineArm)
			if err != nil {
				return SkillEvalReport{}, fmt.Errorf("baseline %s: %w", evaluation.CaseID, err)
			}
		}
		item.Comparison = comparisonForSkillEvalCase(item, review.Comparison)
		report.Cases = append(report.Cases, item)
	}
	observed := false
	for _, item := range report.Cases {
		observed = observed || item.Candidate != nil || item.Baseline != nil
	}
	report.Adapter.Observed = observed
	report.Model.Observed = observed
	report.Outcome = skillEvalOutcome(report)
	return report, nil
}

func validateSkillEvalRunInput(input SkillEvalRunInput) error {
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
	if err := validateSkillEvalSummary(input.HumanReview, input.ArtifactRoot); err != nil {
		return fmt.Errorf("human review: %w", err)
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
		if !skillEvalDigestPattern.MatchString(input.CandidateSkillSHA256[item.Skill]) {
			return fmt.Errorf("skill evaluation case %q has no candidate skill digest", item.CaseID)
		}
		if item.BaselineRequired && !skillEvalDigestPattern.MatchString(input.BaselineSkillSHA256[item.Skill]) {
			return fmt.Errorf("skill evaluation case %q has no baseline skill digest", item.CaseID)
		}
		review, ok := input.CaseReviews[item.CaseID]
		if !ok || !validSkillEvalComparison(review.Comparison) {
			return fmt.Errorf("skill evaluation case %q has no valid human comparison", item.CaseID)
		}
		if err := validateSkillEvalSummary(review.HumanReview, input.ArtifactRoot); err != nil {
			return fmt.Errorf("skill evaluation case %q review: %w", item.CaseID, err)
		}
	}
	return nil
}

func runSkillEvalArm(ctx context.Context, input SkillEvalRunInput, evaluation SkillEvalCase, arm string) (*SkillEvalRun, error) {
	rawRoot := filepath.Join(input.ArtifactRoot, "skill-evals", "v0.5.0", input.SessionID, "raw", evaluation.Skill, evaluation.CaseID, arm)
	if err := os.MkdirAll(rawRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create raw root: %w", err)
	}
	result, err := input.Adapter.Run(ctx, SkillEvalAdapterRequest{Case: evaluation, Arm: arm, RawRoot: rawRoot})
	if err != nil {
		return nil, nil
	}
	if !validSkillEvalRunOutcome(result.Outcome) || !validSkillEvalGrade(result.DeterministicGrade) {
		return nil, nil
	}
	digest, err := nonEmptySkillEvalRawDigest(rawRoot)
	if err != nil {
		return nil, fmt.Errorf("validate raw evidence: %w", err)
	}
	skills, executable := input.CandidateSkillSHA256[evaluation.Skill], input.CandidateExecutableSHA256
	if arm == skillEvalBaselineArm {
		skills, executable = input.BaselineSkillSHA256[evaluation.Skill], input.BaselineExecutableSHA256
	}
	return &SkillEvalRun{Outcome: result.Outcome, SkillSHA256: skills, ExecutableSHA256: executable, DeterministicGrade: result.DeterministicGrade, RawSHA256: digest}, nil
}

func nonEmptySkillEvalRawDigest(root string) (string, error) {
	files := 0
	err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("raw evidence contains a symlink")
		}
		if entry.Type().IsRegular() {
			files++
		}
		return nil
	})
	if err != nil || files == 0 {
		return "", fmt.Errorf("raw evidence is empty or unsafe")
	}
	return skillEvalTreeDigest("taskrail-skill-eval-raw-v1", root)
}

func comparisonForSkillEvalCase(item SkillEvalCaseReport, comparison string) string {
	if item.Candidate == nil || item.Candidate.Outcome == "incomplete" || (item.BaselineRequired && (item.Baseline == nil || item.Baseline.Outcome == "incomplete")) {
		return "inconclusive"
	}
	return comparison
}

func skillEvalOutcome(report SkillEvalReport) string {
	if report.DeterministicChecks.Outcome == "fail" {
		return "fail"
	}
	allPass := true
	for _, item := range report.Cases {
		if item.Candidate != nil && (item.Candidate.Outcome == "fail" || item.Candidate.DeterministicGrade == "fail") || item.Comparison == "worse" {
			return "fail"
		}
		if item.Candidate == nil || item.Candidate.Outcome != "pass" || item.Candidate.DeterministicGrade != "pass" || (item.BaselineRequired && (item.Baseline == nil || item.Baseline.Outcome == "incomplete")) || (item.Comparison != "same" && item.Comparison != "better") {
			allPass = false
		}
	}
	if allPass {
		return "pass"
	}
	return "incomplete"
}

func RenderSkillEvalReport(report SkillEvalReport) ([]byte, error) {
	if err := validateSkillEvalReport(report); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode skill evaluation report: %w", err)
	}
	return append(data, '\n'), nil
}

// DecodeSkillEvalReport accepts only the canonical schema-v1 representation
// bound to a runner-created report. Comparing the rendered form preserves strict
// field ordering and rejects fields that encoding/json would otherwise ignore.
func DecodeSkillEvalReport(data []byte, expected SkillEvalReport) (SkillEvalReport, error) {
	if err := checkDocumentFraming(data); err != nil {
		return SkillEvalReport{}, fmt.Errorf("decode skill evaluation report: %w", err)
	}
	if len(expected.manifest.Cases) == 0 {
		return SkillEvalReport{}, fmt.Errorf("decode skill evaluation report: expected runner manifest is empty")
	}
	var report SkillEvalReport
	if err := json.Unmarshal(data, &report); err != nil {
		return SkillEvalReport{}, fmt.Errorf("decode skill evaluation report: %w", err)
	}
	report.manifest = expected.manifest
	canonical, err := RenderSkillEvalReport(report)
	if err != nil {
		return SkillEvalReport{}, err
	}
	if !bytes.Equal(data, canonical) {
		return SkillEvalReport{}, fmt.Errorf("skill evaluation report is not canonical schema-v1 JSON")
	}
	return report, nil
}

func validateSkillEvalReport(report SkillEvalReport) error {
	if report.SchemaVersion != 1 || !validSkillEvalReportOutcome(report.Outcome) {
		return fmt.Errorf("skill evaluation report has an unsupported schema or outcome")
	}
	if !skillEvalSessionID.MatchString(report.SessionID) || len(report.SessionID) > 64 {
		return fmt.Errorf("skill evaluation report session ID is not portable")
	}
	generatedAt, err := time.Parse(time.RFC3339, report.GeneratedAt)
	if err != nil || generatedAt.Location() != time.UTC || generatedAt.Format(time.RFC3339) != report.GeneratedAt {
		return fmt.Errorf("skill evaluation report timestamp is not canonical UTC")
	}
	for _, digest := range []string{report.TestedHead, report.CandidateExecutableSHA256, report.BaselineExecutableSHA256, report.ProductSHA256, report.Candidate.SkillsSHA256, report.Baseline.SkillsSHA256, report.FixturesSHA256} {
		if !skillEvalDigestPattern.MatchString(digest) {
			return fmt.Errorf("skill evaluation report contains an invalid digest")
		}
	}
	if report.Candidate.TaskrailVersion != "v0.5.0" || report.Baseline.TaskrailVersion != "v0.4.0" {
		return fmt.Errorf("skill evaluation report has invalid arm versions")
	}
	if err := validateSkillEvalIdentity(report.Adapter); err != nil {
		return fmt.Errorf("report adapter identity: %w", err)
	}
	if err := validateSkillEvalIdentity(report.Model); err != nil {
		return fmt.Errorf("report model identity: %w", err)
	}
	if err := validateSkillEvalChecks(report.DeterministicChecks); err != nil {
		return err
	}
	if err := validateSkillEvalSummary(report.HumanReview, ""); err != nil {
		return fmt.Errorf("report human review: %w", err)
	}
	if len(report.Cases) == 0 || len(report.manifest.Cases) == 0 || len(report.Cases) != len(report.manifest.Cases) {
		return fmt.Errorf("skill evaluation report has no cases")
	}
	seen := map[string]bool{}
	for index, item := range report.Cases {
		expected, found := report.manifest.Cases[item.CaseID]
		if item.CaseID == "" || len(item.CaseID) > 64 || !skillEvalCaseID.MatchString(item.CaseID) || !skillEvalCaseID.MatchString(item.Skill) || seen[item.CaseID] || !found || item.Skill != expected.Skill || item.StorageMode != expected.StorageMode || item.FixtureSHA256 != expected.FixtureSHA256 || item.BaselineRequired != expected.BaselineRequired || !validSkillEvalStorageMode(item.StorageMode) || !skillEvalDigestPattern.MatchString(item.FixtureSHA256) || !validSkillEvalComparison(item.Comparison) || validateSkillEvalSummary(item.HumanReview, "") != nil {
			return fmt.Errorf("skill evaluation report case %q is invalid", item.CaseID)
		}
		seen[item.CaseID] = true
		if index > 0 && compareSkillEvalReports(report.Cases[index-1], item) >= 0 {
			return fmt.Errorf("skill evaluation report cases are not strictly ordered")
		}
		if item.Candidate != nil && (!validSkillEvalReportRun(*item.Candidate, report.CandidateExecutableSHA256) || item.Candidate.SkillSHA256 != expected.CandidateSkill) {
			return fmt.Errorf("skill evaluation report candidate %q is invalid", item.CaseID)
		}
		if !item.BaselineRequired && item.Baseline != nil {
			return fmt.Errorf("skill evaluation report new skill %q has a baseline arm", item.CaseID)
		}
		if item.Baseline != nil && (!validSkillEvalReportRun(*item.Baseline, report.BaselineExecutableSHA256) || item.Baseline.SkillSHA256 != expected.BaselineSkill) {
			return fmt.Errorf("skill evaluation report baseline %q is invalid", item.CaseID)
		}
		complete := item.Candidate != nil && item.Candidate.Outcome != "incomplete" && (!item.BaselineRequired || item.Baseline != nil && item.Baseline.Outcome != "incomplete")
		if complete && item.Comparison == "inconclusive" || !complete && item.Comparison != "inconclusive" {
			return fmt.Errorf("skill evaluation report case %q comparison is inconsistent", item.CaseID)
		}
	}
	observed := false
	for _, item := range report.Cases {
		observed = observed || item.Candidate != nil || item.Baseline != nil
	}
	if report.Adapter.Observed != observed || report.Model.Observed != observed {
		return fmt.Errorf("skill evaluation report identity observation is inconsistent")
	}
	return validateSkillEvalReportOutcome(report)
}

func validateSkillEvalReportOutcome(report SkillEvalReport) error {
	baseOutcome := skillEvalOutcome(report)
	if report.Waiver == nil {
		if report.Outcome != baseOutcome {
			return fmt.Errorf("skill evaluation report outcome does not match its evidence")
		}
		return nil
	}
	if report.Outcome != "waived" || baseOutcome != "incomplete" {
		return fmt.Errorf("skill evaluation report waiver is inconsistent with its outcome")
	}
	if !hasRequiredSkillEvalWaiverChecks(report.DeterministicChecks) {
		return fmt.Errorf("skill evaluation waiver requires every credential-free deterministic gate")
	}
	return validateSkillEvalWaiver(*report.Waiver, report.Cases)
}

func validateSkillEvalWaiver(waiver SkillEvalWaiver, cases []SkillEvalCaseReport) error {
	for _, value := range []string{waiver.Approver, waiver.Reason, waiver.UnavailableCapability, waiver.ResidualRisk, waiver.Followup} {
		if err := validateSkillEvalSummary(value, ""); err != nil {
			return fmt.Errorf("skill evaluation waiver has an unsafe required summary: %w", err)
		}
	}
	for _, values := range [][]string{waiver.AffectedSkills, waiver.AffectedCases, waiver.CompensatingEvidence} {
		if err := validateSkillEvalWaiverStrings(values); err != nil {
			return err
		}
	}
	for _, value := range waiver.CompensatingEvidence {
		if err := validateSkillEvalSummary(value, ""); err != nil {
			return fmt.Errorf("skill evaluation waiver compensating evidence is unsafe: %w", err)
		}
	}
	missingCases, missingSkills := skillEvalIncompleteCoverage(cases)
	if !slices.Equal(waiver.AffectedCases, missingCases) || !slices.Equal(waiver.AffectedSkills, missingSkills) {
		return fmt.Errorf("skill evaluation waiver does not exactly cover incomplete cases and skills")
	}
	return nil
}

func hasRequiredSkillEvalWaiverChecks(checks SkillEvalDeterministicChecks) bool {
	if checks.Outcome != "pass" {
		return false
	}
	for _, required := range skillEvalRequiredWaiverChecks {
		if !slices.Contains(checks.Evidence, required) {
			return false
		}
	}
	return true
}

func validateSkillEvalWaiverStrings(values []string) error {
	if len(values) == 0 || !slices.IsSorted(values) || slices.Contains(values, "") {
		return fmt.Errorf("skill evaluation waiver arrays must be non-empty, sorted, and non-empty")
	}
	for index, value := range values {
		if !utf8.ValidString(value) || index > 0 && values[index-1] == value {
			return fmt.Errorf("skill evaluation waiver arrays must be unique UTF-8 strings")
		}
	}
	return nil
}

func skillEvalIncompleteCoverage(cases []SkillEvalCaseReport) ([]string, []string) {
	missingSkills := map[string]struct{}{}
	var missingCases []string
	for _, item := range cases {
		complete := item.Candidate != nil && item.Candidate.Outcome != "incomplete" && (!item.BaselineRequired || item.Baseline != nil && item.Baseline.Outcome != "incomplete")
		if complete {
			continue
		}
		missingCases = append(missingCases, item.CaseID)
		missingSkills[item.Skill] = struct{}{}
	}
	var skills []string
	for skill := range missingSkills {
		skills = append(skills, skill)
	}
	slices.Sort(missingCases)
	slices.Sort(skills)
	return missingCases, skills
}

func validSkillEvalReportRun(run SkillEvalRun, executable string) bool {
	return validSkillEvalRunOutcome(run.Outcome) && validSkillEvalGrade(run.DeterministicGrade) && skillEvalDigestPattern.MatchString(run.SkillSHA256) && run.ExecutableSHA256 == executable && skillEvalDigestPattern.MatchString(run.RawSHA256)
}

func validateSkillEvalIdentity(identity SkillEvalIdentity) error {
	if identity.Name == "" || identity.Version == "" || !utf8.ValidString(identity.Name) || !utf8.ValidString(identity.Version) {
		return fmt.Errorf("name and version must be non-empty UTF-8")
	}
	return nil
}

func validateSkillEvalChecks(checks SkillEvalDeterministicChecks) error {
	if checks.Outcome != "pass" && checks.Outcome != "fail" || len(checks.Evidence) == 0 || !slices.IsSorted(checks.Evidence) || slices.Contains(checks.Evidence, "") {
		return fmt.Errorf("deterministic checks must have a pass/fail outcome and sorted non-empty evidence")
	}
	for i, evidence := range checks.Evidence {
		if !utf8.ValidString(evidence) || i > 0 && checks.Evidence[i-1] == evidence {
			return fmt.Errorf("deterministic check evidence is not unique UTF-8")
		}
	}
	return nil
}

func validateSkillEvalSummary(summary, artifactRoot string) error {
	if summary == "" || !utf8.ValidString(summary) {
		return fmt.Errorf("must be a non-empty UTF-8 safe summary")
	}
	if artifactRoot != "" && strings.Contains(summary, artifactRoot) || strings.Contains(summary, "\x00") || strings.Contains(summary, "raw/") || skillEvalSummaryContainsLocalPath(summary) {
		return fmt.Errorf("contains a producer-local raw path")
	}
	return nil
}

func skillEvalSummaryContainsLocalPath(summary string) bool {
	if skillEvalLocalPath.MatchString(summary) {
		return true
	}
	for _, word := range strings.Fields(summary) {
		word = strings.Trim(word, "()[]{}<>,.;:'\"")
		if filepath.IsAbs(word) || strings.HasPrefix(word, "~/") || strings.HasPrefix(word, "~\\") || skillEvalWindowsPath.MatchString(word) {
			return true
		}
	}
	return false
}

func compareSkillEvalCases(a, b SkillEvalCase) int {
	if a.Skill == b.Skill {
		return strings.Compare(a.CaseID, b.CaseID)
	}
	return strings.Compare(a.Skill, b.Skill)
}

func compareSkillEvalReports(a, b SkillEvalCaseReport) int {
	if a.Skill == b.Skill {
		return strings.Compare(a.CaseID, b.CaseID)
	}
	return strings.Compare(a.Skill, b.Skill)
}

func validSkillEvalRunOutcome(value string) bool {
	return value == "pass" || value == "fail" || value == "incomplete"
}
func validSkillEvalGrade(value string) bool       { return value == "pass" || value == "fail" }
func validSkillEvalStorageMode(value string) bool { return value == "committed" || value == "local" }
func validSkillEvalComparison(value string) bool {
	return value == "better" || value == "same" || value == "worse" || value == "inconclusive"
}
func validSkillEvalReportOutcome(value string) bool {
	return value == "pass" || value == "fail" || value == "incomplete" || value == "waived"
}
