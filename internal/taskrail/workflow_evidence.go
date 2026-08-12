package taskrail

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Probe, observation, and terminal-evidence decoding for one workflow-adversarial
// report, plus the referential-integrity pass over the graph they form.

func decodeWorkflowProbes(raw json.RawMessage) ([]WorkflowProbe, error) {
	elements, err := arrayMember(raw, "workflow report", "probes")
	if err != nil {
		return nil, err
	}
	result := make([]WorkflowProbe, 0, len(elements))
	ids := make([]string, 0, len(elements))
	for i, element := range elements {
		what := fmt.Sprintf("workflow probe at index %d", i)
		obj, err := strictObject(element, what)
		if err != nil {
			return nil, err
		}
		if err := exactMembers(obj, what, []string{"probe_id", "surface_keys", "action", "executed", "outcome", "observation_ids", "evidence_refs"}); err != nil {
			return nil, err
		}
		var p WorkflowProbe
		if p.ProbeID, err = reviewKeyMember(obj, what, "probe_id"); err != nil {
			return nil, err
		}
		if p.SurfaceKeys, err = decodeWorkflowSurfaceKeys(obj["surface_keys"], what); err != nil {
			return nil, err
		}
		if p.Action, err = nonEmptyMember(obj, what, "action"); err != nil {
			return nil, err
		}
		if p.Executed, err = boolMember(obj, what, "executed"); err != nil {
			return nil, err
		}
		if p.Outcome, err = enumMember(obj, what, "outcome", []string{"pass", "fail", "inconclusive"}); err != nil {
			return nil, err
		}
		if p.ObservationIDs, err = decodeWorkflowKeyList(obj["observation_ids"], what, "observation_ids"); err != nil {
			return nil, err
		}
		for _, id := range p.ObservationIDs {
			if !isPortableReviewKey(id) {
				return nil, fmt.Errorf("%s observation_id %q is not a portable review key", what, id)
			}
		}
		if p.EvidenceRefs, err = decodeWorkflowEvidenceRefs(obj["evidence_refs"], what); err != nil {
			return nil, err
		}
		ids = append(ids, p.ProbeID)
		result = append(result, p)
	}
	if !sortedUniqueStrings(ids) {
		return nil, fmt.Errorf("workflow probes are not sorted and unique by probe_id")
	}
	return result, nil
}

func decodeWorkflowObservations(raw json.RawMessage, subjects WorkflowSubjects) ([]WorkflowObservation, error) {
	elements, err := arrayMember(raw, "workflow report", "observations")
	if err != nil {
		return nil, err
	}
	result := make([]WorkflowObservation, 0, len(elements))
	ids := make([]string, 0, len(elements))
	for i, element := range elements {
		what := fmt.Sprintf("workflow observation at index %d", i)
		obj, err := strictObject(element, what)
		if err != nil {
			return nil, err
		}
		if err := exactMembers(obj, what, []string{"observation_id", "probe_id", "expected", "observed", "outcome", "evidence"}); err != nil {
			return nil, err
		}
		var o WorkflowObservation
		if o.ObservationID, err = reviewKeyMember(obj, what, "observation_id"); err != nil {
			return nil, err
		}
		if o.ProbeID, err = reviewKeyMember(obj, what, "probe_id"); err != nil {
			return nil, err
		}
		if o.Expected, err = nonEmptyMember(obj, what, "expected"); err != nil {
			return nil, err
		}
		if o.Observed, err = nonEmptyMember(obj, what, "observed"); err != nil {
			return nil, err
		}
		if o.Outcome, err = enumMember(obj, what, "outcome", []string{"supports-clean", "supports-finding", "inconclusive"}); err != nil {
			return nil, err
		}
		if o.Evidence, err = decodeWorkflowEvidenceList(obj["evidence"], what, subjects); err != nil {
			return nil, err
		}
		ids = append(ids, o.ObservationID)
		result = append(result, o)
	}
	if !sortedUniqueStrings(ids) {
		return nil, fmt.Errorf("workflow observations are not sorted and unique by observation_id")
	}
	return result, nil
}

// decodeWorkflowEvidenceList reads the closed terminal-evidence union. Keeping
// it terminal is what forbids an observation from citing itself or another
// observation, so supporting evidence can never close a cycle.
func decodeWorkflowEvidenceList(raw json.RawMessage, owner string, subjects WorkflowSubjects) ([]WorkflowEvidence, error) {
	elements, err := arrayMember(raw, owner, "evidence")
	if err != nil {
		return nil, err
	}
	result := make([]WorkflowEvidence, 0, len(elements))
	for i, element := range elements {
		what := fmt.Sprintf("%s evidence at index %d", owner, i)
		obj, err := strictObject(element, what)
		if err != nil {
			return nil, err
		}
		if err := exactMembers(obj, what, []string{"kind", "summary", "path", "sha256", "command", "exit_code"}); err != nil {
			return nil, err
		}
		var e WorkflowEvidence
		if e.Kind, err = enumMember(obj, what, "kind", []string{"command", "file", "manual"}); err != nil {
			return nil, err
		}
		if e.Summary, err = nonEmptyMember(obj, what, "summary"); err != nil {
			return nil, err
		}
		if e.Path, err = nullableStringMember(obj, what, "path"); err != nil {
			return nil, err
		}
		if e.SHA256, err = nullableStringMember(obj, what, "sha256"); err != nil {
			return nil, err
		}
		if e.Command, err = nullableStringMember(obj, what, "command"); err != nil {
			return nil, err
		}
		if e.ExitCode, err = nullableIntegerMember(obj, what, "exit_code"); err != nil {
			return nil, err
		}
		if err := validateWorkflowEvidenceKind(e, what, subjects); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, nil
}

// workflowEvidenceForbidden lists, per kind, the members the closed union in the
// machine companion requires to be null.
var workflowEvidenceForbidden = map[string][]string{
	"command": {"path", "sha256"},
	"file":    {"command", "exit_code"},
	"manual":  {"path", "sha256", "command", "exit_code"},
}

func validateWorkflowEvidenceKind(e WorkflowEvidence, what string, subjects WorkflowSubjects) error {
	set := map[string]bool{"path": e.Path != nil, "sha256": e.SHA256 != nil, "command": e.Command != nil, "exit_code": e.ExitCode != nil}
	for _, name := range workflowEvidenceForbidden[e.Kind] {
		if set[name] {
			return fmt.Errorf("%s %s evidence must leave %q null", what, e.Kind, name)
		}
	}
	switch e.Kind {
	case "command":
		if e.Command == nil || strings.TrimSpace(*e.Command) == "" || e.ExitCode == nil {
			return fmt.Errorf("%s command evidence requires command and exit_code", what)
		}
	case "file":
		if e.Path == nil || e.SHA256 == nil || !reviewDigest.MatchString(*e.SHA256) {
			return fmt.Errorf("%s file evidence requires path and a lower-case 64-hex sha256", what)
		}
		return validateWorkflowEvidencePath(*e.Path, what, subjects)
	}
	return nil
}

// validateWorkflowEvidencePath keeps file evidence durable: a proposal, artifact,
// or local-overlay path is deleted or rewritten long before a later run reads
// this memory, so it can never stand as history.
func validateWorkflowEvidencePath(path, what string, subjects WorkflowSubjects) error {
	if absolutePathStart.MatchString(path) || !canonicalPathSegments(path) {
		return fmt.Errorf("%s file evidence path is not a canonical repository-relative path", what)
	}
	for _, root := range []string{subjects.ArtifactsDir, localStorageRoot} {
		if path == root || strings.HasPrefix(path, root+"/") {
			return fmt.Errorf("%s file evidence path %q is transient, not durable review evidence", what, path)
		}
	}
	return nil
}

// validateWorkflowReportGraph resolves every intra-report reference. References
// carrying another review's ID are historical: this report cannot resolve them,
// and publication checks them against the published subtree instead.
func validateWorkflowReportGraph(report WorkflowReport) error {
	probes := map[string]WorkflowProbe{}
	for _, probe := range report.Probes {
		probes[probe.ProbeID] = probe
	}
	observations := map[string]WorkflowObservation{}
	for _, observation := range report.Observations {
		probe, ok := probes[observation.ProbeID]
		if !ok {
			return fmt.Errorf("workflow observation %q references unknown probe %q", observation.ObservationID, observation.ProbeID)
		}
		if !slices.Contains(probe.ObservationIDs, observation.ObservationID) {
			return fmt.Errorf("workflow probe %q does not list observation %q", probe.ProbeID, observation.ObservationID)
		}
		observations[observation.ObservationID] = observation
	}
	surfaces := map[string]struct{}{}
	for _, surface := range report.Scope.Surfaces {
		surfaces[surface.SurfaceKey] = struct{}{}
	}
	for _, probe := range report.Probes {
		for _, id := range probe.ObservationIDs {
			if _, ok := observations[id]; !ok {
				return fmt.Errorf("workflow probe %q references unknown observation %q", probe.ProbeID, id)
			}
		}
		for _, key := range probe.SurfaceKeys {
			if _, ok := surfaces[key]; !ok {
				return fmt.Errorf("workflow probe %q references untested surface %q", probe.ProbeID, key)
			}
		}
	}
	if err := validateWorkflowLocalRefs(report, probes, observations); err != nil {
		return err
	}
	return validateWorkflowReportOutcomes(report, probes, observations)
}

func validateWorkflowLocalRefs(report WorkflowReport, probes map[string]WorkflowProbe, observations map[string]WorkflowObservation) error {
	check := func(owner string, refs []WorkflowEvidenceRef) error {
		for _, ref := range refs {
			if ref.ReviewID != report.ReviewID {
				continue
			}
			observation, ok := observations[ref.ObservationID]
			if !ok {
				return fmt.Errorf("%s references unknown observation %q", owner, ref.ObservationID)
			}
			if observation.ProbeID != ref.ProbeID {
				return fmt.Errorf("%s references observation %q under the wrong probe", owner, ref.ObservationID)
			}
			if _, ok := probes[ref.ProbeID]; !ok {
				return fmt.Errorf("%s references unknown probe %q", owner, ref.ProbeID)
			}
		}
		return nil
	}
	for _, probe := range report.Probes {
		if err := check("workflow probe "+probe.ProbeID, probe.EvidenceRefs); err != nil {
			return err
		}
	}
	for _, surface := range report.Scope.Surfaces {
		if err := check("workflow scope surface "+surface.SurfaceKey, surface.EvidenceRefs); err != nil {
			return err
		}
	}
	for _, finding := range report.Findings {
		if err := check("workflow report finding "+finding.FindingID, finding.EvidenceRefs); err != nil {
			return err
		}
	}
	return nil
}

func validateWorkflowReportOutcomes(report WorkflowReport, probes map[string]WorkflowProbe, observations map[string]WorkflowObservation) error {
	findings := map[string]WorkflowReportFinding{}
	for _, finding := range report.Findings {
		findings[finding.FindingID] = finding
	}
	for _, surface := range report.Scope.Surfaces {
		for _, id := range surface.FindingIDs {
			if _, ok := findings[id]; !ok {
				return fmt.Errorf("workflow scope surface %q references unknown finding %q", surface.SurfaceKey, id)
			}
		}
		// A clean claim is the one outcome that asserts absence, so it must rest on
		// an executed probe and an observation someone else can re-run.
		if surface.Outcome == "clean" && !hasExecutedObservableRef(report.ReviewID, surface.EvidenceRefs, probes, observations, "supports-clean") {
			return fmt.Errorf("workflow scope surface %q claims clean without an executed probe and observable evidence", surface.SurfaceKey)
		}
	}
	for _, finding := range report.Findings {
		if len(finding.EvidenceRefs) == 0 {
			return fmt.Errorf("workflow report finding %q has no evidence reference", finding.FindingID)
		}
		if finding.Status != "resolved" && finding.Status != "not-reproducible" {
			continue
		}
		// Closing a finding drops it from review memory, so the cited attempt must
		// both be fresh and actually support absence: an executed probe whose own
		// observation still supports the finding cannot retire it.
		if !hasExecutedObservableRef(report.ReviewID, finding.EvidenceRefs, probes, observations, "supports-clean") {
			return fmt.Errorf("workflow report finding %q claims %q without a fresh executed attempt supporting absence in this report", finding.FindingID, finding.Status)
		}
	}
	return nil
}

// hasExecutedObservableRef reports whether refs reach an observation of the
// wanted outcome, produced by this run's executed probe, that carries evidence
// someone else can re-run. Refs to another review are historical, not an attempt.
func hasExecutedObservableRef(reviewID string, refs []WorkflowEvidenceRef, probes map[string]WorkflowProbe, observations map[string]WorkflowObservation, outcome string) bool {
	for _, ref := range refs {
		if ref.ReviewID != reviewID {
			continue
		}
		observation, ok := observations[ref.ObservationID]
		if !ok || observation.Outcome != outcome || !probes[observation.ProbeID].Executed {
			continue
		}
		for _, evidence := range observation.Evidence {
			if evidence.Kind == "command" || evidence.Kind == "file" {
				return true
			}
		}
	}
	return false
}

func decodeWorkflowEvidenceRefs(raw json.RawMessage, owner string) ([]WorkflowEvidenceRef, error) {
	elements, err := arrayMember(raw, owner, "evidence_refs")
	if err != nil {
		return nil, err
	}
	result := make([]WorkflowEvidenceRef, 0, len(elements))
	keys := make([]string, 0, len(elements))
	for i, element := range elements {
		what := fmt.Sprintf("%s evidence reference at index %d", owner, i)
		obj, err := strictObject(element, what)
		if err != nil {
			return nil, err
		}
		if err := exactMembers(obj, what, []string{"review_id", "probe_id", "observation_id"}); err != nil {
			return nil, err
		}
		var ref WorkflowEvidenceRef
		if ref.ReviewID, err = reviewKeyMember(obj, what, "review_id"); err != nil {
			return nil, err
		}
		if ref.ProbeID, err = reviewKeyMember(obj, what, "probe_id"); err != nil {
			return nil, err
		}
		if ref.ObservationID, err = reviewKeyMember(obj, what, "observation_id"); err != nil {
			return nil, err
		}
		keys = append(keys, ref.key())
		result = append(result, ref)
	}
	if !sortedUniqueStrings(keys) {
		return nil, fmt.Errorf("%s evidence_refs are not sorted and unique", owner)
	}
	return result, nil
}

func decodeWorkflowFindingIDs(raw json.RawMessage, owner string) ([]string, error) {
	elements, err := arrayMember(raw, owner, "finding_ids")
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(elements))
	for i, element := range elements {
		id, err := rawString(element, fmt.Sprintf("%s finding_id at index %d", owner, i))
		if err != nil {
			return nil, err
		}
		if _, err := workflowFindingNumber(id); err != nil {
			return nil, fmt.Errorf("%s: %w", owner, err)
		}
		ids = append(ids, id)
	}
	if !sortedUniqueStrings(ids) {
		return nil, fmt.Errorf("%s finding_ids are not sorted and unique", owner)
	}
	return ids, nil
}

func decodeWorkflowSurfaceKeys(raw json.RawMessage, owner string) ([]string, error) {
	keys, err := decodeWorkflowKeyList(raw, owner, "surface_keys")
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("%s has no surface key", owner)
	}
	for _, key := range keys {
		if !isWorkflowSurfaceKey(key) {
			return nil, fmt.Errorf("%s surface key %q is not normalized", owner, key)
		}
	}
	return keys, nil
}

func decodeWorkflowKeyList(raw json.RawMessage, owner, name string) ([]string, error) {
	elements, err := arrayMember(raw, owner, name)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(elements))
	for i, element := range elements {
		value, err := rawString(element, fmt.Sprintf("%s %s at index %d", owner, name, i))
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if !sortedUniqueStrings(values) {
		return nil, fmt.Errorf("%s %s are not sorted and unique", owner, name)
	}
	return values, nil
}

func workflowSurfaceKeyMember(obj map[string]json.RawMessage, what, name string) (string, error) {
	value, err := stringMember(obj, what, name)
	if err != nil {
		return "", err
	}
	if !isWorkflowSurfaceKey(value) {
		return "", fmt.Errorf("%s member %q is not a normalized surface key", what, name)
	}
	return value, nil
}

func isWorkflowSurfaceKey(value string) bool {
	return len(value) <= 128 && workflowSurfaceKey.MatchString(value)
}

func workflowObjectIDMember(obj map[string]json.RawMessage, what, name string) (string, error) {
	value, err := stringMember(obj, what, name)
	if err != nil {
		return "", err
	}
	if !workflowObjectID.MatchString(value) {
		return "", fmt.Errorf("%s member %q is not a lower-case full Git object ID", what, name)
	}
	return value, nil
}

func workflowFindingIDMember(obj map[string]json.RawMessage, what, name string) (string, error) {
	value, err := stringMember(obj, what, name)
	if err != nil {
		return "", err
	}
	if _, err := workflowFindingNumber(value); err != nil {
		return "", fmt.Errorf("%s member %q: %w", what, name, err)
	}
	return value, nil
}

// workflowFindingNumber requires the exact canonical rendering of the number, so
// one number denotes one ID and index ordering by number is total.
func workflowFindingNumber(id string) (int64, error) {
	if !workflowFindingID.MatchString(id) {
		return 0, fmt.Errorf("finding ID %q is not WF- plus a zero-padded number", id)
	}
	number, err := strconv.ParseInt(strings.TrimPrefix(id, "WF-"), 10, 64)
	if err != nil || number < 1 || fmt.Sprintf("WF-%06d", number) != id {
		return 0, fmt.Errorf("finding ID %q is not the canonical form of a positive number", id)
	}
	return number, nil
}

func nullableIntegerMember(obj map[string]json.RawMessage, what, name string) (*int64, error) {
	raw, ok := obj[name]
	if !ok {
		return nil, fmt.Errorf("%s is missing member %q", what, name)
	}
	if isJSONNull(raw) {
		return nil, nil
	}
	value, ok := decodeJSONInteger(raw)
	if !ok {
		return nil, fmt.Errorf("%s member %q is not an integer", what, name)
	}
	return &value, nil
}

func sortedUniqueStrings(values []string) bool {
	for i := 1; i < len(values); i++ {
		if values[i] <= values[i-1] {
			return false
		}
	}
	return true
}

// sortedUniqueFindingIDs orders by number rather than bytes, because padding
// widens past six digits and byte order would then put WF-1000000 before
// WF-999999.
func sortedUniqueFindingIDs(ids []string) bool {
	for i := 1; i < len(ids); i++ {
		previous, err := workflowFindingNumber(ids[i-1])
		if err != nil {
			return false
		}
		current, err := workflowFindingNumber(ids[i])
		if err != nil || current <= previous {
			return false
		}
	}
	return true
}

func findingIDsOf(findings []WorkflowReportFinding) []string {
	ids := make([]string, 0, len(findings))
	for _, finding := range findings {
		ids = append(ids, finding.FindingID)
	}
	return ids
}
