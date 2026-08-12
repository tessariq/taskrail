package taskrail

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// The workflow-adversarial index is the only cross-run memory serial testing
// keeps, and Taskrail derives it rather than accepting agent bytes. Everything
// here therefore has one canonical encoding: the reviewing agent must be able to
// predict the exact bytes it declares in index_sha256_after.

type WorkflowIndex struct {
	Raw               []byte                 `json:"-"`
	SchemaVersion     int64                  `json:"schema_version"`
	NextFindingNumber int64                  `json:"next_finding_number"`
	Surfaces          []WorkflowSurface      `json:"surfaces"`
	Findings          []WorkflowIndexFinding `json:"findings"`
}

type WorkflowSurface struct {
	SurfaceKey    string                `json:"surface_key"`
	EvidenceRefs  []WorkflowEvidenceRef `json:"evidence_refs"`
	Outcome       string                `json:"outcome"`
	Freshness     string                `json:"freshness"`
	SpecPath      string                `json:"spec_path"`
	SpecSHA256    string                `json:"spec_sha256"`
	ProductSHA256 string                `json:"product_sha256"`
	TestedHead    string                `json:"tested_head"`
	CheckedAt     string                `json:"checked_at"`
	FindingIDs    []string              `json:"finding_ids"`
	NextAngle     string                `json:"next_angle"`
}

type WorkflowIndexFinding struct {
	FindingID    string                `json:"finding_id"`
	Severity     string                `json:"severity"`
	EvidenceRefs []WorkflowEvidenceRef `json:"evidence_refs"`
	Impact       string                `json:"impact"`
	FirstSeen    WorkflowSnapshot      `json:"first_seen"`
	LastChecked  WorkflowSnapshot      `json:"last_checked"`
	Status       string                `json:"status"`
	Rationale    string                `json:"rationale"`
	TaskID       *string               `json:"task_id,omitempty"`
}

type WorkflowSnapshot struct {
	ReviewID      string `json:"review_id"`
	SpecPath      string `json:"spec_path"`
	SpecSHA256    string `json:"spec_sha256"`
	ProductSHA256 string `json:"product_sha256"`
	TestedHead    string `json:"tested_head"`
	CheckedAt     string `json:"checked_at"`
}

// WorkflowIndexCandidate is the complete candidate INDEX.json a report derives.
// Its canonical bytes are Index.Raw; SHA256 is what the report must have
// declared in index_sha256_after.
type WorkflowIndexCandidate struct {
	Index  WorkflowIndex
	SHA256 string
}

// EncodeWorkflowIndex renders the one canonical form: two-space indent, schema
// member order, standard escaping without HTML escaping, and one final LF.
func EncodeWorkflowIndex(index WorkflowIndex) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(index); err != nil {
		return nil, fmt.Errorf("workflow index cannot be encoded: %w", err)
	}
	return buf.Bytes(), nil
}

// DecodeWorkflowIndex reads prior review memory strictly. Accepted bytes must
// already be canonical, so a hand-edited or re-serialized index is rejected
// instead of being silently rewritten by the next derivation.
func DecodeWorkflowIndex(data []byte) (WorkflowIndex, error) {
	var out WorkflowIndex
	if len(data) > workflowIndexByteLimit {
		return out, fmt.Errorf("workflow index exceeds %d bytes", workflowIndexByteLimit)
	}
	if err := checkDocumentFraming(data); err != nil {
		return out, err
	}
	obj, err := strictObject(data, "workflow index")
	if err != nil {
		return out, err
	}
	if err := exactMembers(obj, "workflow index", []string{"schema_version", "next_finding_number", "surfaces", "findings"}); err != nil {
		return out, err
	}
	if err := schemaVersionOne(obj, "workflow index"); err != nil {
		return out, err
	}
	out.SchemaVersion = 1
	if out.NextFindingNumber, err = workflowCounterMember(obj); err != nil {
		return out, err
	}
	if out.Surfaces, err = decodeWorkflowSurfaces(obj["surfaces"]); err != nil {
		return out, err
	}
	if out.Findings, err = decodeWorkflowIndexFindings(obj["findings"], out.NextFindingNumber); err != nil {
		return out, err
	}
	if err := validateWorkflowIndexGraph(out); err != nil {
		return out, err
	}
	canonical, err := EncodeWorkflowIndex(out)
	if err != nil {
		return out, err
	}
	if !bytes.Equal(canonical, data) {
		return out, fmt.Errorf("workflow index is not canonically encoded")
	}
	out.Raw = append([]byte(nil), data...)
	return out, nil
}

func workflowCounterMember(obj map[string]json.RawMessage) (int64, error) {
	value, ok := decodeJSONInteger(obj["next_finding_number"])
	if !ok || value < 1 {
		return 0, fmt.Errorf("workflow index member %q is not a positive integer", "next_finding_number")
	}
	return value, nil
}

func decodeWorkflowSurfaces(raw json.RawMessage) ([]WorkflowSurface, error) {
	elements, err := arrayMember(raw, "workflow index", "surfaces")
	if err != nil {
		return nil, err
	}
	if len(elements) > workflowIndexSurfaceLimit {
		return nil, fmt.Errorf("workflow index holds more than %d surface rows", workflowIndexSurfaceLimit)
	}
	result := make([]WorkflowSurface, 0, len(elements))
	keys := make([]string, 0, len(elements))
	for i, element := range elements {
		what := fmt.Sprintf("workflow index surface at index %d", i)
		obj, err := strictObject(element, what)
		if err != nil {
			return nil, err
		}
		names := []string{"surface_key", "evidence_refs", "outcome", "freshness", "spec_path", "spec_sha256", "product_sha256", "tested_head", "checked_at", "finding_ids", "next_angle"}
		if err := exactMembers(obj, what, names); err != nil {
			return nil, err
		}
		var s WorkflowSurface
		if s.SurfaceKey, err = workflowSurfaceKeyMember(obj, what, "surface_key"); err != nil {
			return nil, err
		}
		if s.EvidenceRefs, err = decodeWorkflowEvidenceRefs(obj["evidence_refs"], what); err != nil {
			return nil, err
		}
		if s.Outcome, err = enumMember(obj, what, "outcome", []string{"clean", "finding", "inconclusive"}); err != nil {
			return nil, err
		}
		if s.Freshness, err = enumMember(obj, what, "freshness", []string{"fresh", "stale"}); err != nil {
			return nil, err
		}
		snapshot, err := decodeWorkflowSnapshotFields(obj, what)
		if err != nil {
			return nil, err
		}
		s.SpecPath, s.SpecSHA256 = snapshot.SpecPath, snapshot.SpecSHA256
		s.ProductSHA256, s.TestedHead, s.CheckedAt = snapshot.ProductSHA256, snapshot.TestedHead, snapshot.CheckedAt
		if s.FindingIDs, err = decodeWorkflowFindingIDs(obj["finding_ids"], what); err != nil {
			return nil, err
		}
		if s.NextAngle, err = nonEmptyMember(obj, what, "next_angle"); err != nil {
			return nil, err
		}
		keys = append(keys, s.SurfaceKey)
		result = append(result, s)
	}
	if !sortedUniqueStrings(keys) {
		return nil, fmt.Errorf("workflow index surfaces are not sorted and unique by surface_key")
	}
	return result, nil
}

// decodeWorkflowSnapshotFields reads the five snapshot members a surface row and
// a finding snapshot share. ReviewID is left to the caller: a surface binds its
// review through evidence references instead of naming one.
func decodeWorkflowSnapshotFields(obj map[string]json.RawMessage, what string) (WorkflowSnapshot, error) {
	var out WorkflowSnapshot
	var err error
	if out.SpecPath, err = reviewPathMember(obj, what, "spec_path"); err != nil {
		return out, err
	}
	if out.SpecSHA256, err = reviewDigestMember(obj, what, "spec_sha256"); err != nil {
		return out, err
	}
	if out.ProductSHA256, err = reviewDigestMember(obj, what, "product_sha256"); err != nil {
		return out, err
	}
	if out.TestedHead, err = workflowObjectIDMember(obj, what, "tested_head"); err != nil {
		return out, err
	}
	out.CheckedAt, err = reviewTimeMember(obj, what, "checked_at")
	return out, err
}

func decodeWorkflowIndexFindings(raw json.RawMessage, counter int64) ([]WorkflowIndexFinding, error) {
	elements, err := arrayMember(raw, "workflow index", "findings")
	if err != nil {
		return nil, err
	}
	result := make([]WorkflowIndexFinding, 0, len(elements))
	ids := make([]string, 0, len(elements))
	for i, element := range elements {
		what := fmt.Sprintf("workflow index finding at index %d", i)
		obj, err := strictObject(element, what)
		if err != nil {
			return nil, err
		}
		required := []string{"finding_id", "severity", "evidence_refs", "impact", "first_seen", "last_checked", "status", "rationale"}
		if err := exactOptionalMembers(obj, what, required, []string{"task_id"}); err != nil {
			return nil, err
		}
		var f WorkflowIndexFinding
		if f.FindingID, err = workflowFindingIDMember(obj, what, "finding_id"); err != nil {
			return nil, err
		}
		number, err := workflowFindingNumber(f.FindingID)
		if err != nil {
			return nil, err
		}
		if number >= counter {
			return nil, fmt.Errorf("%s allocates %q at or above next_finding_number %d", what, f.FindingID, counter)
		}
		if f.Severity, err = enumMember(obj, what, "severity", []string{"high", "medium", "low"}); err != nil {
			return nil, err
		}
		if f.EvidenceRefs, err = decodeWorkflowEvidenceRefs(obj["evidence_refs"], what); err != nil {
			return nil, err
		}
		if len(f.EvidenceRefs) == 0 {
			return nil, fmt.Errorf("%s has no evidence reference", what)
		}
		if f.Impact, err = nonEmptyMember(obj, what, "impact"); err != nil {
			return nil, err
		}
		if f.FirstSeen, err = decodeWorkflowSnapshot(obj["first_seen"], what+" first_seen"); err != nil {
			return nil, err
		}
		if f.LastChecked, err = decodeWorkflowSnapshot(obj["last_checked"], what+" last_checked"); err != nil {
			return nil, err
		}
		if f.Status, err = enumMember(obj, what, "status", []string{"open", "tracked", "deferred"}); err != nil {
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
		return nil, fmt.Errorf("workflow index findings are not sorted and unique by finding number")
	}
	return result, nil
}

func decodeWorkflowSnapshot(raw json.RawMessage, what string) (WorkflowSnapshot, error) {
	obj, err := strictObject(raw, what)
	if err != nil {
		return WorkflowSnapshot{}, err
	}
	names := []string{"review_id", "spec_path", "spec_sha256", "product_sha256", "tested_head", "checked_at"}
	if err := exactMembers(obj, what, names); err != nil {
		return WorkflowSnapshot{}, err
	}
	reviewID, err := reviewKeyMember(obj, what, "review_id")
	if err != nil {
		return WorkflowSnapshot{}, err
	}
	out, err := decodeWorkflowSnapshotFields(obj, what)
	out.ReviewID = reviewID
	return out, err
}

func validateWorkflowIndexGraph(index WorkflowIndex) error {
	findings := map[string]struct{}{}
	for _, finding := range index.Findings {
		findings[finding.FindingID] = struct{}{}
	}
	for _, surface := range index.Surfaces {
		for _, id := range surface.FindingIDs {
			if _, ok := findings[id]; !ok {
				return fmt.Errorf("workflow index surface %q references unknown finding %q", surface.SurfaceKey, id)
			}
		}
	}
	return nil
}

// DeriveWorkflowIndex applies one immutable report to prior memory and returns
// the candidate index. priorRaw is nil only when INDEX.json does not exist; an
// existing but empty file is present memory and is decoded (and rejected) as
// such. The report declares the digests it expects on both sides, so a report
// written against memory that has since moved is refused rather than merged.
//
// Seeing only one prior index and one report, this cannot prove the report's
// review ID is unique across the whole workflow-adversarial subtree; the
// publisher owes that check, or stored snapshots would point at the wrong run.
func DeriveWorkflowIndex(priorRaw []byte, report WorkflowReport) (WorkflowIndexCandidate, error) {
	var candidate WorkflowIndexCandidate
	prior := emptyWorkflowIndex()
	if priorRaw == nil {
		if report.IndexSHA256Before != "absent" {
			return candidate, fmt.Errorf("workflow report expects prior memory %q but none exists", report.IndexSHA256Before)
		}
	} else {
		if report.IndexSHA256Before != digestRaw(priorRaw) {
			return candidate, fmt.Errorf("workflow report index_sha256_before does not match prior memory exact bytes")
		}
		decoded, err := DecodeWorkflowIndex(priorRaw)
		if err != nil {
			return candidate, err
		}
		prior = decoded
	}
	candidate, err := buildWorkflowIndexCandidate(prior, report)
	if err != nil {
		return WorkflowIndexCandidate{}, err
	}
	if candidate.SHA256 != report.IndexSHA256After {
		return WorkflowIndexCandidate{}, fmt.Errorf("workflow report index_sha256_after does not match the derived candidate index")
	}
	return candidate, nil
}

// emptyWorkflowIndex is the memory a first publication derives from.
func emptyWorkflowIndex() WorkflowIndex {
	return WorkflowIndex{SchemaVersion: 1, NextFindingNumber: 1, Surfaces: []WorkflowSurface{}, Findings: []WorkflowIndexFinding{}}
}

func buildWorkflowIndexCandidate(prior WorkflowIndex, report WorkflowReport) (WorkflowIndexCandidate, error) {
	var candidate WorkflowIndexCandidate
	findings, closed, err := deriveWorkflowFindings(prior, report)
	if err != nil {
		return candidate, err
	}
	surfaces, err := deriveWorkflowSurfaces(prior, report, closed)
	if err != nil {
		return candidate, err
	}
	index := WorkflowIndex{
		SchemaVersion:     1,
		NextFindingNumber: deriveWorkflowCounter(prior, report),
		Surfaces:          surfaces,
		Findings:          findings,
	}
	raw, err := EncodeWorkflowIndex(index)
	if err != nil {
		return candidate, err
	}
	// Refusing here is the point: an overflowing candidate must never be trimmed,
	// because the only rows worth dropping are the unresolved findings.
	if len(index.Surfaces) > workflowIndexSurfaceLimit {
		return candidate, fmt.Errorf("candidate workflow index holds %d surface rows, over the %d row cap", len(index.Surfaces), workflowIndexSurfaceLimit)
	}
	if len(raw) > workflowIndexByteLimit {
		return candidate, fmt.Errorf("candidate workflow index is %d bytes, over the %d byte cap", len(raw), workflowIndexByteLimit)
	}
	if _, err := DecodeWorkflowIndex(raw); err != nil {
		return candidate, fmt.Errorf("candidate workflow index is invalid: %w", err)
	}
	index.Raw = raw
	return WorkflowIndexCandidate{Index: index, SHA256: digestRaw(raw)}, nil
}

func deriveWorkflowFindings(prior WorkflowIndex, report WorkflowReport) ([]WorkflowIndexFinding, map[string]struct{}, error) {
	priorFindings := map[string]WorkflowIndexFinding{}
	for _, finding := range prior.Findings {
		priorFindings[finding.FindingID] = finding
	}
	closed := map[string]struct{}{}
	updated := map[string]WorkflowIndexFinding{}
	snapshot := workflowReportSnapshot(report)
	for _, finding := range report.Findings {
		existing, known := priorFindings[finding.FindingID]
		if err := validateWorkflowFindingTransition(finding, existing, known, prior.NextFindingNumber); err != nil {
			return nil, nil, err
		}
		if slices.Contains([]string{"resolved", "not-reproducible", "obsolete"}, finding.Status) {
			closed[finding.FindingID] = struct{}{}
			continue
		}
		updated[finding.FindingID] = workflowIndexFinding(finding, existing, known, snapshot, report.ReviewID)
	}
	result := make([]WorkflowIndexFinding, 0, len(prior.Findings)+len(updated))
	for _, finding := range prior.Findings {
		if _, ok := closed[finding.FindingID]; ok {
			continue
		}
		if _, ok := updated[finding.FindingID]; ok {
			continue // the report's row replaces it below
		}
		result = append(result, finding)
	}
	for _, finding := range report.Findings {
		if replacement, ok := updated[finding.FindingID]; ok {
			result = append(result, replacement)
		}
	}
	slices.SortFunc(result, func(a, b WorkflowIndexFinding) int {
		left, _ := workflowFindingNumber(a.FindingID)
		right, _ := workflowFindingNumber(b.FindingID)
		return cmp.Compare(left, right)
	})
	return result, closed, nil
}

func validateWorkflowFindingTransition(finding WorkflowReportFinding, existing WorkflowIndexFinding, known bool, counter int64) error {
	if !known {
		if finding.Status != "open" && finding.Status != "tracked" && finding.Status != "deferred" {
			return fmt.Errorf("workflow report finding %q closes a finding absent from prior memory", finding.FindingID)
		}
		number, err := workflowFindingNumber(finding.FindingID)
		if err != nil {
			return err
		}
		if number < counter {
			return fmt.Errorf("workflow report finding %q reuses a finding number below next_finding_number %d", finding.FindingID, counter)
		}
		if finding.TaskID != nil && finding.Status != "tracked" {
			return fmt.Errorf("workflow report finding %q invents a task_id without tracking it", finding.FindingID)
		}
		return nil
	}
	// A stored task ID is snapshot evidence of when the finding became tracked, so
	// a later report may keep it but never relabel or invent one.
	if existing.TaskID != nil && finding.TaskID != nil && *existing.TaskID != *finding.TaskID {
		return fmt.Errorf("workflow report finding %q rewrites the historical task_id", finding.FindingID)
	}
	if existing.TaskID == nil && finding.TaskID != nil && finding.Status != "tracked" {
		return fmt.Errorf("workflow report finding %q invents a task_id without tracking it", finding.FindingID)
	}
	return nil
}

func workflowIndexFinding(finding WorkflowReportFinding, existing WorkflowIndexFinding, known bool, snapshot WorkflowSnapshot, reviewID string) WorkflowIndexFinding {
	derived := WorkflowIndexFinding{
		FindingID:    finding.FindingID,
		Severity:     finding.Severity,
		EvidenceRefs: finding.EvidenceRefs,
		Impact:       finding.Impact,
		FirstSeen:    snapshot,
		LastChecked:  snapshot,
		Status:       finding.Status,
		Rationale:    finding.Rationale,
		TaskID:       finding.TaskID,
	}
	if known {
		derived.FirstSeen = existing.FirstSeen
		// Last-checked is an assertion that someone looked again, so it advances
		// only when this run produced the supporting observation.
		derived.LastChecked = existing.LastChecked
		if workflowHasLocalRef(finding.EvidenceRefs, reviewID) {
			derived.LastChecked = snapshot
		}
		if derived.TaskID == nil {
			derived.TaskID = existing.TaskID
		}
	}
	return derived
}

func deriveWorkflowSurfaces(prior WorkflowIndex, report WorkflowReport, closed map[string]struct{}) ([]WorkflowSurface, error) {
	tested := map[string]struct{}{}
	result := make([]WorkflowSurface, 0, len(prior.Surfaces)+len(report.Scope.Surfaces))
	for _, surface := range report.Scope.Surfaces {
		tested[surface.SurfaceKey] = struct{}{}
		result = append(result, WorkflowSurface{
			SurfaceKey:    surface.SurfaceKey,
			EvidenceRefs:  surface.EvidenceRefs,
			Outcome:       surface.Outcome,
			Freshness:     "fresh",
			SpecPath:      report.SpecPath,
			SpecSHA256:    report.SpecSHA256,
			ProductSHA256: report.ProductSHA256,
			TestedHead:    report.TestedHead,
			CheckedAt:     report.GeneratedAt,
			FindingIDs:    workflowOpenFindingIDs(surface.FindingIDs, closed),
			NextAngle:     surface.NextAngle,
		})
	}
	carried := map[string]struct{}{}
	for _, surface := range prior.Surfaces {
		carried[surface.SurfaceKey] = struct{}{}
		if _, ok := tested[surface.SurfaceKey]; ok {
			continue
		}
		surface.Freshness = workflowRolledFreshness(surface, report)
		surface.FindingIDs = workflowOpenFindingIDs(surface.FindingIDs, closed)
		result = append(result, surface)
	}
	for _, assessment := range report.Scope.FreshnessAssessments {
		_, isPrior := carried[assessment.SurfaceKey]
		if _, isTested := tested[assessment.SurfaceKey]; !isPrior && !isTested {
			return nil, fmt.Errorf("workflow freshness assessment names unknown surface %q", assessment.SurfaceKey)
		}
	}
	slices.SortFunc(result, func(a, b WorkflowSurface) int { return strings.Compare(a.SurfaceKey, b.SurfaceKey) })
	return result, nil
}

// workflowRolledFreshness marks an untested row stale as soon as its bound spec
// or product snapshot moves, unless this report explicitly retained it.
func workflowRolledFreshness(surface WorkflowSurface, report WorkflowReport) string {
	if surface.Freshness != "fresh" {
		return surface.Freshness
	}
	if surface.SpecSHA256 == report.SpecSHA256 && surface.ProductSHA256 == report.ProductSHA256 {
		return "fresh"
	}
	for _, assessment := range report.Scope.FreshnessAssessments {
		if assessment.SurfaceKey == surface.SurfaceKey && assessment.Decision == "retain" {
			return "fresh"
		}
	}
	return "stale"
}

func workflowOpenFindingIDs(ids []string, closed map[string]struct{}) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := closed[id]; !ok {
			result = append(result, id)
		}
	}
	return result
}

func deriveWorkflowCounter(prior WorkflowIndex, report WorkflowReport) int64 {
	next := prior.NextFindingNumber
	for _, finding := range report.Findings {
		number, err := workflowFindingNumber(finding.FindingID)
		if err == nil && number >= next {
			next = number + 1
		}
	}
	return next
}

func workflowReportSnapshot(report WorkflowReport) WorkflowSnapshot {
	return WorkflowSnapshot{
		ReviewID:      report.ReviewID,
		SpecPath:      report.SpecPath,
		SpecSHA256:    report.SpecSHA256,
		ProductSHA256: report.ProductSHA256,
		TestedHead:    report.TestedHead,
		CheckedAt:     report.GeneratedAt,
	}
}

func workflowHasLocalRef(refs []WorkflowEvidenceRef, reviewID string) bool {
	for _, ref := range refs {
		if ref.ReviewID == reviewID {
			return true
		}
	}
	return false
}
