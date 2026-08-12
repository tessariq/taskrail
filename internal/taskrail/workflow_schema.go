package taskrail

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
)

// Strict decoding for the workflow-adversarial report and its canonical review
// memory index. Serial testing keeps its history in these two files alone, so a
// lenient reading would let one run silently rewrite evidence that no later run
// can reconstruct.

const (
	workflowIndexByteLimit     = 256 << 10
	workflowIndexSurfaceLimit  = 256
	workflowReportSurfaceLimit = 3
)

var (
	workflowSurfaceKey = regexp.MustCompile(`^[a-z0-9]+(?:[._/-][a-z0-9]+)*$`)
	workflowFindingID  = regexp.MustCompile(`^WF-[0-9]{6,}$`)
	// Git object IDs are recorded in full, so both SHA-1 and SHA-256 repository
	// formats are accepted but abbreviations never are.
	workflowObjectID = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
)

// WorkflowSubjects are the exact bytes and identities a report must bind. They
// come from the caller's preflight, never from the untrusted proposal.
type WorkflowSubjects struct {
	SpecPath      string
	Spec          []byte
	TestedHead    string
	ProductSHA256 string
	// ArtifactsDir is the repository-relative artifacts root. Its subtree is
	// transient, so durable file evidence may never point inside it.
	ArtifactsDir string
}

type WorkflowEvidenceRef struct {
	ReviewID      string `json:"review_id"`
	ProbeID       string `json:"probe_id"`
	ObservationID string `json:"observation_id"`
}

func (r WorkflowEvidenceRef) key() string {
	return r.ReviewID + "\x00" + r.ProbeID + "\x00" + r.ObservationID
}

type WorkflowEvidence struct {
	Kind     string
	Summary  string
	Path     *string
	SHA256   *string
	Command  *string
	ExitCode *int64
}

type WorkflowReport struct {
	Raw               []byte
	Binding           ReviewPromptBinding
	ReviewID          string
	SpecPath          string
	SpecSHA256        string
	TestedHead        string
	ProductSHA256     string
	ContextMode       string
	GeneratedAt       string
	Scope             WorkflowScope
	Probes            []WorkflowProbe
	Observations      []WorkflowObservation
	Findings          []WorkflowReportFinding
	IndexSHA256Before string
	IndexSHA256After  string
}

type WorkflowScope struct {
	Summary              string
	Surfaces             []WorkflowScopeSurface
	FreshnessAssessments []WorkflowFreshnessAssessment
}

type WorkflowScopeSurface struct {
	SurfaceKey, Angle, Rationale, Outcome, NextAngle string
	EvidenceRefs                                     []WorkflowEvidenceRef
	FindingIDs                                       []string
}

type WorkflowFreshnessAssessment struct {
	SurfaceKey, Decision, Evidence, Rationale string
	ChangedPaths                              []string
}

type WorkflowProbe struct {
	ProbeID, Action, Outcome    string
	Executed                    bool
	SurfaceKeys, ObservationIDs []string
	EvidenceRefs                []WorkflowEvidenceRef
}

type WorkflowObservation struct {
	ObservationID, ProbeID, Expected, Observed, Outcome string
	Evidence                                            []WorkflowEvidence
}

type WorkflowReportFinding struct {
	FindingID, Severity, Impact, Status, Rationale string
	EvidenceRefs                                   []WorkflowEvidenceRef
	TaskID                                         *string
}

// DecodeWorkflowReport decodes one staged workflow-adversarial report against
// the exact subjects the caller proved, preserving the accepted bytes.
func DecodeWorkflowReport(data []byte, subjects WorkflowSubjects) (WorkflowReport, error) {
	var out WorkflowReport
	if subjects.SpecPath == "" || subjects.ArtifactsDir == "" {
		return out, fmt.Errorf("workflow subjects are incomplete")
	}
	obj, err := decodeReviewObject(data, "workflow report")
	if err != nil {
		return out, err
	}
	names := []string{"schema_version", "prompt_id", "prompt_contract_version", "prompt_template_sha256", "prompt_source", "review_id", "spec_path", "spec_sha256", "tested_head", "product_sha256", "context_mode", "generated_at", "scope", "probes", "observations", "findings", "index_sha256_before", "index_sha256_after"}
	if err := exactMembers(obj, "workflow report", names); err != nil {
		return out, err
	}
	if err := schemaVersionOne(obj, "workflow report"); err != nil {
		return out, err
	}
	if out.Binding, err = decodePromptBinding(obj, "workflow report", "workflow-adversarial"); err != nil {
		return out, err
	}
	if err := decodeWorkflowReportSubjects(&out, obj, subjects); err != nil {
		return out, err
	}
	if out.Probes, err = decodeWorkflowProbes(obj["probes"]); err != nil {
		return out, err
	}
	if out.Observations, err = decodeWorkflowObservations(obj["observations"], subjects); err != nil {
		return out, err
	}
	if out.Findings, err = decodeWorkflowReportFindings(obj["findings"]); err != nil {
		return out, err
	}
	if out.Scope, err = decodeWorkflowScope(obj["scope"]); err != nil {
		return out, err
	}
	if err := validateWorkflowReportGraph(out); err != nil {
		return out, err
	}
	out.Raw = append([]byte(nil), data...)
	return out, nil
}

func decodeWorkflowReportSubjects(out *WorkflowReport, obj map[string]json.RawMessage, subjects WorkflowSubjects) error {
	var err error
	if out.ReviewID, err = reviewKeyMember(obj, "workflow report", "review_id"); err != nil {
		return err
	}
	if out.SpecPath, err = fixedMember(obj, "workflow report", "spec_path", subjects.SpecPath); err != nil {
		return err
	}
	if out.SpecSHA256, err = reviewDigestMember(obj, "workflow report", "spec_sha256"); err != nil {
		return err
	}
	if out.SpecSHA256 != digestRaw(subjects.Spec) {
		return fmt.Errorf("workflow report spec_sha256 does not match selected spec exact bytes")
	}
	if out.TestedHead, err = workflowObjectIDMember(obj, "workflow report", "tested_head"); err != nil {
		return err
	}
	if out.TestedHead != subjects.TestedHead {
		return fmt.Errorf("workflow report tested_head does not match the tested commit")
	}
	if out.ProductSHA256, err = reviewDigestMember(obj, "workflow report", "product_sha256"); err != nil {
		return err
	}
	if out.ProductSHA256 != subjects.ProductSHA256 {
		return fmt.Errorf("workflow report product_sha256 does not match the tested product digest")
	}
	if out.ContextMode, err = enumMember(obj, "workflow report", "context_mode", []string{"fresh", "same-context"}); err != nil {
		return err
	}
	if out.GeneratedAt, err = reviewTimeMember(obj, "workflow report", "generated_at"); err != nil {
		return err
	}
	if out.IndexSHA256Before, err = workflowPriorMemberDigest(obj, "workflow report", "index_sha256_before"); err != nil {
		return err
	}
	out.IndexSHA256After, err = reviewDigestMember(obj, "workflow report", "index_sha256_after")
	return err
}

func decodeWorkflowScope(raw json.RawMessage) (WorkflowScope, error) {
	var out WorkflowScope
	obj, err := strictObject(raw, "workflow report scope")
	if err != nil {
		return out, err
	}
	if err := exactMembers(obj, "workflow report scope", []string{"summary", "surfaces", "freshness_assessments"}); err != nil {
		return out, err
	}
	if out.Summary, err = nonEmptyMember(obj, "workflow report scope", "summary"); err != nil {
		return out, err
	}
	elements, err := arrayMember(obj["surfaces"], "workflow report scope", "surfaces")
	if err != nil {
		return out, err
	}
	if len(elements) > workflowReportSurfaceLimit {
		return out, fmt.Errorf("workflow report tests more than %d surface keys", workflowReportSurfaceLimit)
	}
	keys := make([]string, 0, len(elements))
	for i, element := range elements {
		surface, err := decodeWorkflowScopeSurface(element, fmt.Sprintf("workflow scope surface at index %d", i))
		if err != nil {
			return out, err
		}
		keys = append(keys, surface.SurfaceKey)
		out.Surfaces = append(out.Surfaces, surface)
	}
	if !sortedUniqueStrings(keys) {
		return out, fmt.Errorf("workflow scope surfaces are not sorted and unique by surface_key")
	}
	if out.FreshnessAssessments, err = decodeWorkflowFreshnessAssessments(obj["freshness_assessments"]); err != nil {
		return out, err
	}
	return out, nil
}

func decodeWorkflowScopeSurface(raw json.RawMessage, what string) (WorkflowScopeSurface, error) {
	var out WorkflowScopeSurface
	obj, err := strictObject(raw, what)
	if err != nil {
		return out, err
	}
	if err := exactMembers(obj, what, []string{"surface_key", "angle", "rationale", "outcome", "evidence_refs", "finding_ids", "next_angle"}); err != nil {
		return out, err
	}
	if out.SurfaceKey, err = workflowSurfaceKeyMember(obj, what, "surface_key"); err != nil {
		return out, err
	}
	if out.Angle, err = nonEmptyMember(obj, what, "angle"); err != nil {
		return out, err
	}
	if out.Rationale, err = nonEmptyMember(obj, what, "rationale"); err != nil {
		return out, err
	}
	if out.Outcome, err = enumMember(obj, what, "outcome", []string{"clean", "finding", "inconclusive"}); err != nil {
		return out, err
	}
	if out.EvidenceRefs, err = decodeWorkflowEvidenceRefs(obj["evidence_refs"], what); err != nil {
		return out, err
	}
	if len(out.EvidenceRefs) == 0 {
		return out, fmt.Errorf("%s has no evidence reference", what)
	}
	if out.FindingIDs, err = decodeWorkflowFindingIDs(obj["finding_ids"], what); err != nil {
		return out, err
	}
	out.NextAngle, err = nonEmptyMember(obj, what, "next_angle")
	return out, err
}

func decodeWorkflowFreshnessAssessments(raw json.RawMessage) ([]WorkflowFreshnessAssessment, error) {
	elements, err := arrayMember(raw, "workflow report scope", "freshness_assessments")
	if err != nil {
		return nil, err
	}
	result := make([]WorkflowFreshnessAssessment, 0, len(elements))
	keys := make([]string, 0, len(elements))
	for i, element := range elements {
		what := fmt.Sprintf("workflow freshness assessment at index %d", i)
		obj, err := strictObject(element, what)
		if err != nil {
			return nil, err
		}
		if err := exactMembers(obj, what, []string{"surface_key", "decision", "changed_paths", "evidence", "rationale"}); err != nil {
			return nil, err
		}
		var a WorkflowFreshnessAssessment
		if a.SurfaceKey, err = workflowSurfaceKeyMember(obj, what, "surface_key"); err != nil {
			return nil, err
		}
		if a.Decision, err = enumMember(obj, what, "decision", []string{"stale", "retain"}); err != nil {
			return nil, err
		}
		if a.ChangedPaths, err = decodeWorkflowChangedPaths(obj["changed_paths"], what); err != nil {
			return nil, err
		}
		// A retain decision keeps a prior clean surface trusted across a snapshot
		// change, so its explanation is what stands in for a rerun.
		if a.Evidence, err = nonEmptyMember(obj, what, "evidence"); err != nil {
			return nil, err
		}
		if a.Rationale, err = nonEmptyMember(obj, what, "rationale"); err != nil {
			return nil, err
		}
		keys = append(keys, a.SurfaceKey)
		result = append(result, a)
	}
	if !sortedUniqueStrings(keys) {
		return nil, fmt.Errorf("workflow freshness assessments are not sorted and unique by surface_key")
	}
	return result, nil
}

// decodeWorkflowChangedPaths reads the assessment's affected-path list. The
// spec calls it ordered but never byte-sorted (unlike the ID collections), so
// the producer's own order stands; only a repeat is refused as ambiguous.
func decodeWorkflowChangedPaths(raw json.RawMessage, what string) ([]string, error) {
	elements, err := arrayMember(raw, what, "changed_paths")
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(elements))
	for i, element := range elements {
		path, err := rawString(element, fmt.Sprintf("%s changed path at index %d", what, i))
		if err != nil {
			return nil, err
		}
		if absolutePathStart.MatchString(path) || !canonicalPathSegments(path) {
			return nil, fmt.Errorf("%s changed path %q is not a canonical repository-relative path", what, path)
		}
		if slices.Contains(paths, path) {
			return nil, fmt.Errorf("%s repeats changed path %q", what, path)
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func decodeWorkflowReportFindings(raw json.RawMessage) ([]WorkflowReportFinding, error) {
	elements, err := arrayMember(raw, "workflow report", "findings")
	if err != nil {
		return nil, err
	}
	result := make([]WorkflowReportFinding, 0, len(elements))
	ids := make([]string, 0, len(elements))
	for i, element := range elements {
		what := fmt.Sprintf("workflow report finding at index %d", i)
		obj, err := strictObject(element, what)
		if err != nil {
			return nil, err
		}
		required := []string{"finding_id", "severity", "evidence_refs", "impact", "status", "rationale"}
		if err := exactOptionalMembers(obj, what, required, []string{"task_id"}); err != nil {
			return nil, err
		}
		var f WorkflowReportFinding
		if f.FindingID, err = workflowFindingIDMember(obj, what, "finding_id"); err != nil {
			return nil, err
		}
		if f.Severity, err = enumMember(obj, what, "severity", []string{"high", "medium", "low"}); err != nil {
			return nil, err
		}
		if f.EvidenceRefs, err = decodeWorkflowEvidenceRefs(obj["evidence_refs"], what); err != nil {
			return nil, err
		}
		if f.Impact, err = nonEmptyMember(obj, what, "impact"); err != nil {
			return nil, err
		}
		if f.Status, err = enumMember(obj, what, "status", []string{"open", "tracked", "resolved", "not-reproducible", "deferred", "obsolete"}); err != nil {
			return nil, err
		}
		if f.Rationale, err = nonEmptyMember(obj, what, "rationale"); err != nil {
			return nil, err
		}
		if f.TaskID, err = decodeWorkflowTaskID(obj, what, f.Status); err != nil {
			return nil, err
		}
		ids = append(ids, f.FindingID)
		result = append(result, f)
	}
	if !sortedUniqueFindingIDs(ids) {
		return nil, fmt.Errorf("workflow report findings are not sorted and unique by finding number")
	}
	return result, nil
}

func decodeWorkflowTaskID(obj map[string]json.RawMessage, what, status string) (*string, error) {
	raw, ok := obj["task_id"]
	if !ok || isJSONNull(raw) {
		if status == "tracked" {
			return nil, fmt.Errorf("%s has status %q without task_id", what, status)
		}
		if ok {
			return nil, fmt.Errorf("%s member %q is null", what, "task_id")
		}
		return nil, nil
	}
	value, err := stringMember(obj, what, "task_id")
	if err != nil {
		return nil, err
	}
	if _, full := taskIDPrefix(value); !full {
		return nil, fmt.Errorf("%s member %q is not a full task ID", what, "task_id")
	}
	return &value, nil
}

// workflowPriorMemberDigest reads the prior-memory binding, whose exact string
// "absent" is how a first publication states that no index existed.
func workflowPriorMemberDigest(obj map[string]json.RawMessage, what, name string) (string, error) {
	value, err := stringMember(obj, what, name)
	if err != nil {
		return "", err
	}
	if value != "absent" && !reviewDigest.MatchString(value) {
		return "", fmt.Errorf("%s member %q is not a lower-case 64-hex digest or %q", what, name, "absent")
	}
	return value, nil
}
