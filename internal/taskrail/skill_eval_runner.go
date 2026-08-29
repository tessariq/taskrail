package taskrail

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

var skillEvalGenericScenarioLabels = map[string]bool{
	"create-isolated-sandbox": true,
	"positive":                true,
	"negative":                true,
	"recovery":                true,
	"boundary":                true,
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
	Case        SkillEvalCase
	Arm         string
	FixtureRoot string
	RawRoot     string
}

type SkillEvalAdapterResult struct {
	Outcome string
	Facts   []SkillEvalObservedFact
}

// SkillEvalObservedFact records one command the adapter ran in the sandbox.
// The same canonical facts must be present in raw facts.json before the runner
// will evaluate an oracle predicate.
type SkillEvalObservedFact struct {
	Action           string   `json:"action"`
	Operation        string   `json:"operation"`
	Command          []string `json:"command"`
	ExitCode         int      `json:"exit_code"`
	StdoutSHA256     string   `json:"stdout_sha256"`
	StderrSHA256     string   `json:"stderr_sha256"`
	BeforeSHA256     string   `json:"before_sha256"`
	AfterSHA256      string   `json:"after_sha256"`
	GitBeforeSHA256  string   `json:"git_before_sha256"`
	GitAfterSHA256   string   `json:"git_after_sha256"`
	ValidationPassed bool     `json:"validation_passed"`
	StoragePaths     []string `json:"storage_paths"`
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

// Run is the single-call convenience path. Execute and Resume let a maintainer
// stop at the required human comparison boundary without rerunning any arm.
func (SkillEvalRunner) Run(ctx context.Context, input SkillEvalRunInput) (SkillEvalReport, error) {
	stage, err := (SkillEvalRunner{}).Execute(ctx, input)
	if err != nil {
		return SkillEvalReport{}, err
	}
	return (SkillEvalRunner{}).Resume(stage, input, input.HumanReview, input.CaseReviews)
}

// Execute invokes every candidate and required baseline arm exactly once, then
// returns an unrenderable staged record with only inconclusive comparisons.
func (SkillEvalRunner) Execute(ctx context.Context, input SkillEvalRunInput) (SkillEvalStage, error) {
	if err := validateSkillEvalRunInput(input, false); err != nil {
		return SkillEvalStage{}, err
	}
	cases := slices.Clone(input.Registry)
	slices.SortFunc(cases, compareSkillEvalCases)
	manifest := skillEvalManifest(cases, input)
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
		HumanReview:               "",
		Waiver:                    nil,
		manifest:                  manifest,
	}
	for _, evaluation := range cases {
		item := SkillEvalCaseReport{
			CaseID: evaluation.CaseID, Skill: evaluation.Skill, StorageMode: evaluation.StorageMode,
			FixtureSHA256: evaluation.FixtureSHA256, BaselineRequired: evaluation.BaselineRequired,
			Comparison: "inconclusive",
		}
		var err error
		item.Candidate, err = runSkillEvalArm(ctx, input, evaluation, skillEvalCandidateArm)
		if err != nil {
			return SkillEvalStage{}, fmt.Errorf("candidate %s: %w", evaluation.CaseID, err)
		}
		if evaluation.BaselineRequired {
			item.Baseline, err = runSkillEvalArm(ctx, input, evaluation, skillEvalBaselineArm)
			if err != nil {
				return SkillEvalStage{}, fmt.Errorf("baseline %s: %w", evaluation.CaseID, err)
			}
		}
		report.Cases = append(report.Cases, item)
	}
	observed := false
	for _, item := range report.Cases {
		observed = observed || item.Candidate != nil || item.Baseline != nil
	}
	report.Adapter.Observed = observed
	report.Model.Observed = observed
	stage := SkillEvalStage{SchemaVersion: 1, Report: report}
	for _, item := range cases {
		reportCase := report.Cases[len(stage.Worksheet)]
		stage.Worksheet = append(stage.Worksheet, SkillEvalWorksheetCase{
			CaseID: item.CaseID, Skill: item.Skill, HumanReviewQuestions: slices.Clone(item.HumanReviewQuestions),
			Candidate: reportCase.Candidate, Baseline: reportCase.Baseline,
		})
	}
	stage.Seal = skillEvalStageSeal(stage)
	return stage, nil
}

func runSkillEvalArm(ctx context.Context, input SkillEvalRunInput, evaluation SkillEvalCase, arm string) (*SkillEvalRun, error) {
	rawRoot := skillEvalRawRoot(input, evaluation, arm)
	if err := os.MkdirAll(rawRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create raw root: %w", err)
	}
	fixtureRoot := ""
	if evaluation.fixtureRoot != "" {
		fixtureRoot = filepath.Join(evaluation.fixtureRoot, evaluation.Scenario.Fixture)
	}
	result, err := input.Adapter.Run(ctx, SkillEvalAdapterRequest{Case: evaluation, Arm: arm, FixtureRoot: fixtureRoot, RawRoot: rawRoot})
	if err != nil {
		return nil, nil
	}
	if !validSkillEvalRunOutcome(result.Outcome) {
		return nil, nil
	}
	grade, err := skillEvalDeterministicGrade(evaluation, rawRoot, result.Facts)
	if err != nil {
		return nil, fmt.Errorf("evaluate deterministic oracle: %w", err)
	}
	digest, err := nonEmptySkillEvalRawDigest(rawRoot)
	if err != nil {
		return nil, fmt.Errorf("validate raw evidence: %w", err)
	}
	skills, executable := input.CandidateSkillSHA256[evaluation.Skill], input.CandidateExecutableSHA256
	if arm == skillEvalBaselineArm {
		skills, executable = input.BaselineSkillSHA256[evaluation.Skill], input.BaselineExecutableSHA256
	}
	return &SkillEvalRun{Outcome: result.Outcome, SkillSHA256: skills, ExecutableSHA256: executable, DeterministicGrade: grade, RawSHA256: digest}, nil
}

func skillEvalRawRoot(input SkillEvalRunInput, evaluation SkillEvalCase, arm string) string {
	return filepath.Join(input.ArtifactRoot, "skill-evals", "v0.5.0", input.SessionID, "raw", evaluation.Skill, evaluation.CaseID, arm)
}

func skillEvalDeterministicGrade(evaluation SkillEvalCase, rawRoot string, facts []SkillEvalObservedFact) (string, error) {
	recorded, err := decodeSkillEvalFacts(rawRoot)
	if err != nil {
		return "", err
	}
	if !slices.EqualFunc(facts, recorded, skillEvalSameFact) {
		return "", fmt.Errorf("adapter facts do not match raw facts receipt")
	}
	byAction := make(map[string]SkillEvalObservedFact, len(facts))
	for _, fact := range facts {
		if _, exists := byAction[fact.Action]; exists {
			return "", fmt.Errorf("duplicate observed action %q", fact.Action)
		}
		byAction[fact.Action] = fact
	}
	declared := append(slices.Clone(evaluation.Scenario.Setup), evaluation.Scenario.Actions...)
	if len(byAction) != len(declared) {
		return "fail", nil
	}
	actions := make(map[string]SkillEvalScenarioAction, len(declared))
	for _, action := range declared {
		actions[action.ID] = action
	}
	for _, oracle := range evaluation.Oracle.Assertions {
		action, found := actions[oracle.Action]
		fact, observed := byAction[oracle.Action]
		if !found || !observed || action.Operation != fact.Operation || !slices.Equal(action.Command, fact.Command) {
			return "fail", nil
		}
		if !skillEvalPredicatePasses(oracle.Predicate, fact) {
			return "fail", nil
		}
	}
	return "pass", nil
}

func skillEvalSameFact(a, b SkillEvalObservedFact) bool {
	return a.Action == b.Action && a.Operation == b.Operation && slices.Equal(a.Command, b.Command) && a.ExitCode == b.ExitCode && a.StdoutSHA256 == b.StdoutSHA256 && a.StderrSHA256 == b.StderrSHA256 && a.BeforeSHA256 == b.BeforeSHA256 && a.AfterSHA256 == b.AfterSHA256 && a.GitBeforeSHA256 == b.GitBeforeSHA256 && a.GitAfterSHA256 == b.GitAfterSHA256 && a.ValidationPassed == b.ValidationPassed && slices.Equal(a.StoragePaths, b.StoragePaths)
}

func skillEvalPredicatePasses(predicate string, fact SkillEvalObservedFact) bool {
	switch predicate {
	case "command-exit-zero":
		return fact.ExitCode == 0
	case "taskrail-validation-pass":
		return fact.Operation == "taskrail-command" && fact.ExitCode == 0 && fact.ValidationPassed
	case "git-worktree-clean":
		return fact.Operation == "git-command" && fact.ExitCode == 0 && fact.GitBeforeSHA256 == fact.GitAfterSHA256
	default:
		return false
	}
}

func validateSkillEvalCaseDefinition(item SkillEvalCase) error {
	if item.Prompt == "" || item.ExpectedObservation == "" || !utf8.ValidString(item.Prompt) || !utf8.ValidString(item.ExpectedObservation) {
		return fmt.Errorf("has an invalid prompt or expected observation")
	}
	if !validSkillEvalStorageMode(item.StorageMode) || !validSkillEvalStrings(item.Assertions) || !validSkillEvalStrings(item.HumanReviewQuestions) {
		return fmt.Errorf("has invalid assertions, questions, or storage mode")
	}
	if item.Scenario.Fixture != "fixture" || item.Scenario.Sandbox != item.CaseID || len(item.Scenario.Setup) == 0 || len(item.Scenario.Actions) != len(item.Assertions) || len(item.Oracle.Assertions) != len(item.Assertions) {
		return fmt.Errorf("has an incomplete executable scenario or oracle")
	}
	assertions, actions, declared := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, assertion := range item.Assertions {
		assertions[assertion] = true
	}
	for _, action := range append(slices.Clone(item.Scenario.Setup), item.Scenario.Actions...) {
		if action.ID == "" || skillEvalGenericScenarioLabels[action.ID] || !skillEvalCaseID.MatchString(action.ID) || (action.Operation != "git-command" && action.Operation != "taskrail-command") || len(action.Command) < 2 || declared[action.ID] {
			return fmt.Errorf("has ambiguous executable actions")
		}
		if action.Operation == "git-command" && action.Command[0] != "git" || action.Operation == "taskrail-command" && action.Command[0] != "taskrail" {
			return fmt.Errorf("does not use a documented %s executable", action.Operation)
		}
		for _, part := range action.Command {
			if part == "" || !utf8.ValidString(part) {
				return fmt.Errorf("has invalid executable action command")
			}
		}
		declared[action.ID] = true
	}
	for _, action := range item.Scenario.Actions {
		actions[action.ID] = true
	}
	for _, oracle := range item.Oracle.Assertions {
		if !assertions[oracle.Assertion] || !actions[oracle.Action] || !validSkillEvalPredicate(oracle.Predicate) || assertions[oracle.Assertion] == false {
			return fmt.Errorf("has an unsupported or ambiguous oracle predicate")
		}
		delete(assertions, oracle.Assertion)
		delete(actions, oracle.Action)
	}
	if len(assertions) != 0 || len(actions) != 0 {
		return fmt.Errorf("does not map every assertion to one executable oracle")
	}
	return nil
}

func validSkillEvalPredicate(value string) bool {
	return value == "command-exit-zero" || value == "taskrail-validation-pass" || value == "git-worktree-clean"
}

func skillEvalBytesDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func validSkillEvalStrings(values []string) bool {
	if len(values) == 0 {
		return false
	}
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if value == "" || !utf8.ValidString(value) || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}

func skillEvalSafeStrings(values []string) bool {
	if !validSkillEvalStrings(values) {
		return false
	}
	for _, value := range values {
		if validateSkillEvalSummary(value, "") != nil {
			return false
		}
	}
	return true
}

func decodeSkillEvalFacts(rawRoot string) ([]SkillEvalObservedFact, error) {
	data, err := os.ReadFile(filepath.Join(rawRoot, "facts.json"))
	if err != nil {
		return nil, fmt.Errorf("read raw facts receipt: %w", err)
	}
	if err := checkDocumentFraming(data); err != nil {
		return nil, fmt.Errorf("decode raw facts receipt: %w", err)
	}
	var facts []SkillEvalObservedFact
	if err := json.Unmarshal(data, &facts); err != nil || len(facts) == 0 {
		return nil, fmt.Errorf("decode raw facts receipt: invalid facts")
	}
	canonical, err := json.MarshalIndent(facts, "", "  ")
	if err != nil || !bytes.Equal(data, append(canonical, '\n')) {
		return nil, fmt.Errorf("raw facts receipt is not canonical")
	}
	return facts, nil
}

func writeSkillEvalFacts(root string, facts []SkillEvalObservedFact) error {
	data, err := json.MarshalIndent(facts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "facts.json"), append(data, '\n'), 0o600)
}

func skillEvalManifest(cases []SkillEvalCase, input SkillEvalRunInput) skillEvalReportManifest {
	manifest := skillEvalReportManifest{Cases: make(map[string]skillEvalExpectedCase, len(cases))}
	for _, item := range cases {
		manifest.Cases[item.CaseID] = skillEvalExpectedCase{Skill: item.Skill, StorageMode: item.StorageMode, FixtureSHA256: item.FixtureSHA256, BaselineRequired: item.BaselineRequired, CandidateSkill: input.CandidateSkillSHA256[item.Skill], BaselineSkill: input.BaselineSkillSHA256[item.Skill]}
	}
	return manifest
}

func skillEvalCaseComplete(item SkillEvalCaseReport) bool {
	return item.Candidate != nil && item.Candidate.Outcome != "incomplete" && (!item.BaselineRequired || item.Baseline != nil && item.Baseline.Outcome != "incomplete")
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

func skillEvalOutcome(report SkillEvalReport) string {
	if report.DeterministicChecks.Outcome == "fail" {
		return "fail"
	}
	allPass := true
	for _, item := range report.Cases {
		if item.Candidate != nil && (item.Candidate.Outcome == "fail" || item.Candidate.DeterministicGrade == "fail") || item.Comparison == "worse" {
			return "fail"
		}
		comparisonPasses := item.Comparison == "candidate-only"
		if item.BaselineRequired {
			comparisonPasses = item.Comparison == "same" || item.Comparison == "better"
		}
		if item.Candidate == nil || item.Candidate.Outcome != "pass" || item.Candidate.DeterministicGrade != "pass" || (item.BaselineRequired && (item.Baseline == nil || item.Baseline.Outcome == "incomplete")) || !comparisonPasses {
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
		complete := skillEvalCaseComplete(item)
		if complete && item.Comparison == "inconclusive" || !complete && item.Comparison != "inconclusive" {
			return fmt.Errorf("skill evaluation report case %q comparison is inconsistent", item.CaseID)
		}
		if complete && !item.BaselineRequired && item.Comparison != "candidate-only" || complete && item.BaselineRequired && !validSkillEvalPairedComparison(item.Comparison) {
			return fmt.Errorf("skill evaluation report case %q comparison does not match its baseline", item.CaseID)
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
		complete := skillEvalCaseComplete(item)
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
	if skillEvalSummaryContainsLocalPath(identity.Name) || skillEvalSummaryContainsLocalPath(identity.Version) {
		return fmt.Errorf("name and version must not contain producer-local paths")
	}
	return nil
}

func validateSkillEvalChecks(checks SkillEvalDeterministicChecks) error {
	if checks.Outcome != "pass" && checks.Outcome != "fail" || len(checks.Evidence) == 0 || !slices.IsSorted(checks.Evidence) || slices.Contains(checks.Evidence, "") {
		return fmt.Errorf("deterministic checks must have a pass/fail outcome and sorted non-empty evidence")
	}
	for i, evidence := range checks.Evidence {
		if !utf8.ValidString(evidence) || skillEvalSummaryContainsLocalPath(evidence) || i > 0 && checks.Evidence[i-1] == evidence {
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
	return validSkillEvalPairedComparison(value) || value == "candidate-only" || value == "inconclusive"
}
func validSkillEvalPairedComparison(value string) bool {
	return value == "better" || value == "same" || value == "worse"
}
func validSkillEvalReportOutcome(value string) bool {
	return value == "pass" || value == "fail" || value == "incomplete" || value == "waived"
}
