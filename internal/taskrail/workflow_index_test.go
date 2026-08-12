package taskrail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestDecodeWorkflowIndexRoundTripsCanonicalGolden(t *testing.T) {
	raw := workflowIndexGolden(t)
	index, err := DecodeWorkflowIndex(raw)
	if err != nil {
		t.Fatalf("DecodeWorkflowIndex: %v", err)
	}
	if string(index.Raw) != string(raw) {
		t.Fatal("accepted bytes were not preserved")
	}
	if index.NextFindingNumber != 2 || len(index.Surfaces) != 1 || len(index.Findings) != 1 {
		t.Fatalf("unexpected index: %#v", index)
	}
	if !bytes.HasSuffix(raw, []byte("}\n")) || bytes.Contains(raw, []byte("\n\t")) {
		t.Fatalf("index is not two-space indented with one final LF: %q", string(raw))
	}
}

func TestDecodeWorkflowIndexRejectsNoncanonicalEncoding(t *testing.T) {
	raw := workflowIndexGolden(t)
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		t.Fatalf("json.Compact: %v", err)
	}
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"compact", compact.Bytes(), "not canonically encoded"},
		{"member order", []byte(strings.Replace(string(raw),
			"\"schema_version\": 1,\n  \"next_finding_number\": 2",
			"\"next_finding_number\": 2,\n  \"schema_version\": 1", 1)), "not canonically encoded"},
		{"nonstandard escaping", []byte(strings.Replace(string(raw), `"severity": "high"`, `"severity": "\u0068igh"`, 1)), "not canonically encoded"},
		{"missing final newline", bytes.TrimRight(raw, "\n"), "not canonically encoded"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeWorkflowIndex(tc.data); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestDecodeWorkflowIndexRejectsIdentityAndCapMutations(t *testing.T) {
	base := string(workflowIndexGolden(t))
	tests := []struct {
		name string
		data string
		want string
	}{
		{"unknown", strings.Replace(base, `"schema_version": 1,`, `"schema_version": 1,`+"\n  "+`"unknown": true,`, 1), "unknown member"},
		{"missing", strings.Replace(base, `"next_finding_number": 2,`, "", 1), "missing member"},
		{"version", strings.Replace(base, `"schema_version": 1`, `"schema_version": 2`, 1), "unsupported schema version"},
		{"reused counter", strings.Replace(base, `"next_finding_number": 2`, `"next_finding_number": 1`, 1), "at or above next_finding_number"},
		{"resolved status", strings.Replace(base, `"status": "open"`, `"status": "resolved"`, 1), "allowed value"},
		{"tracked without task", strings.Replace(base, `"status": "open"`, `"status": "tracked"`, 1), "without task_id"},
		{"dangling finding", strings.Replace(base, `"WF-000001"`, `"WF-000009"`, 1), "unknown finding"},
		{"finding id form", strings.ReplaceAll(base, `"WF-000001"`, `"WF-0000001"`), "canonical form"},
		{"surface key", strings.Replace(base, `"surface_key": "lifecycle/transitions"`, `"surface_key": "Lifecycle"`, 1), "normalized surface key"},
		{"freshness", strings.Replace(base, `"freshness": "fresh"`, `"freshness": "unknown"`, 1), "allowed value"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeWorkflowIndex([]byte(tc.data)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
	oversize := append(workflowIndexGolden(t), make([]byte, workflowIndexByteLimit)...)
	if _, err := DecodeWorkflowIndex(oversize); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversize error = %v", err)
	}
	if _, err := DecodeWorkflowIndex(mustEncodeIndex(t, workflowIndexWithSurfaces(workflowIndexSurfaceLimit+1))); err == nil ||
		!strings.Contains(err.Error(), "more than 256 surface rows") {
		t.Fatalf("surface cap error = %v", err)
	}
}

func TestDecodeWorkflowIndexRejectsUnorderedRows(t *testing.T) {
	index := workflowIndexWithSurfaces(2)
	index.Surfaces[0], index.Surfaces[1] = index.Surfaces[1], index.Surfaces[0]
	if _, err := DecodeWorkflowIndex(mustEncodeIndex(t, index)); err == nil || !strings.Contains(err.Error(), "surfaces are not sorted") {
		t.Fatalf("surface order error = %v", err)
	}
	unordered := workflowIndexWithSurfaces(0)
	unordered.NextFindingNumber = 1000001
	unordered.Findings = []WorkflowIndexFinding{workflowIndexFindingRow("WF-1000000"), workflowIndexFindingRow("WF-999999")}
	if _, err := DecodeWorkflowIndex(mustEncodeIndex(t, unordered)); err == nil || !strings.Contains(err.Error(), "findings are not sorted") {
		t.Fatalf("finding order error = %v", err)
	}
}

func TestDeriveWorkflowIndexFromAbsentMemory(t *testing.T) {
	raw, candidate := resolveParts(t, nil, firstRunParts(""))
	report, err := DecodeWorkflowReport(raw, workflowSubjects())
	if err != nil {
		t.Fatalf("DecodeWorkflowReport: %v", err)
	}
	derived, err := DeriveWorkflowIndex(nil, report)
	if err != nil {
		t.Fatalf("DeriveWorkflowIndex: %v", err)
	}
	if derived.SHA256 != candidate.SHA256 || !bytes.Equal(derived.Index.Raw, candidate.Index.Raw) {
		t.Fatal("derivation is not deterministic")
	}
	if derived.Index.NextFindingNumber != 2 {
		t.Fatalf("counter = %d, want 2", derived.Index.NextFindingNumber)
	}
	finding := derived.Index.Findings[0]
	if finding.FirstSeen != finding.LastChecked || finding.FirstSeen.ReviewID != "wf-1" {
		t.Fatalf("unexpected snapshots: %#v", finding)
	}
	if derived.Index.Surfaces[0].Freshness != "fresh" || derived.Index.Surfaces[0].CheckedAt != "2026-08-12T10:00:00Z" {
		t.Fatalf("unexpected surface: %#v", derived.Index.Surfaces[0])
	}
	report.IndexSHA256After = reviewDigestB
	if _, err := DeriveWorkflowIndex(nil, report); err == nil || !strings.Contains(err.Error(), "index_sha256_after does not match") {
		t.Fatalf("after-digest error = %v", err)
	}
	report.IndexSHA256Before = reviewDigestA
	if _, err := DeriveWorkflowIndex(nil, report); err == nil || !strings.Contains(err.Error(), "but none exists") {
		t.Fatalf("absent-memory error = %v", err)
	}
}

func TestDeriveWorkflowIndexPreservesUnexplainedRowsAndHistory(t *testing.T) {
	prior := workflowIndexGolden(t)
	parts := workflowReportParts{
		reviewID: "wf-2", productSHA: reviewDigestA, before: digestRaw(prior),
		scopeSurfaces: wfScopeSurface("wf-2", "probe-2", "obs-2", "publication/fence", "finding", `"WF-000002"`),
		probes:        wfProbe("wf-2", "probe-2", "obs-2", "publication/fence", "fail", true),
		observations:  wfObservation("probe-2", "obs-2", "supports-finding"),
		findings:      wfFinding("wf-2", "probe-2", "obs-2", "WF-000002", "open", ""),
	}
	raw, _ := resolveParts(t, prior, parts)
	report, err := DecodeWorkflowReport(raw, workflowSubjects())
	if err != nil {
		t.Fatalf("DecodeWorkflowReport: %v", err)
	}
	derived, err := DeriveWorkflowIndex(prior, report)
	if err != nil {
		t.Fatalf("DeriveWorkflowIndex: %v", err)
	}
	if len(derived.Index.Surfaces) != 2 || len(derived.Index.Findings) != 2 || derived.Index.NextFindingNumber != 3 {
		t.Fatalf("unexpected candidate: %#v", derived.Index)
	}
	priorIndex, err := DecodeWorkflowIndex(prior)
	if err != nil {
		t.Fatalf("DecodeWorkflowIndex: %v", err)
	}
	untouched := findWorkflowSurface(t, derived.Index, "lifecycle/transitions")
	if fmt.Sprintf("%#v", untouched) != fmt.Sprintf("%#v", priorIndex.Surfaces[0]) {
		t.Fatalf("unexplained row changed: %#v", untouched)
	}
	kept := derived.Index.Findings[0]
	if kept.FindingID != "WF-000001" || kept.LastChecked.ReviewID != "wf-1" {
		t.Fatalf("unexplained finding advanced: %#v", kept)
	}
	// A report naming the finding again advances last-checked but never first-seen.
	parts.findings = wfFinding("wf-2", "probe-2", "obs-2", "WF-000001", "open", "")
	parts.scopeSurfaces = wfScopeSurface("wf-2", "probe-2", "obs-2", "publication/fence", "finding", `"WF-000001"`)
	raw, _ = resolveParts(t, prior, parts)
	if report, err = DecodeWorkflowReport(raw, workflowSubjects()); err != nil {
		t.Fatalf("DecodeWorkflowReport: %v", err)
	}
	if derived, err = DeriveWorkflowIndex(prior, report); err != nil {
		t.Fatalf("DeriveWorkflowIndex: %v", err)
	}
	updated := derived.Index.Findings[0]
	if updated.FirstSeen.ReviewID != "wf-1" || updated.LastChecked.ReviewID != "wf-2" {
		t.Fatalf("unexpected snapshot transition: %#v", updated)
	}
}

func TestDeriveWorkflowIndexRollsFreshnessOnlyWithAssessments(t *testing.T) {
	prior := workflowIndexGolden(t)
	subjects := workflowSubjects()
	subjects.ProductSHA256 = reviewDigestB
	parts := workflowReportParts{
		reviewID: "wf-2", productSHA: reviewDigestB, before: digestRaw(prior),
		scopeSurfaces: wfScopeSurface("wf-2", "probe-2", "obs-2", "publication/fence", "inconclusive", ""),
		probes:        wfProbe("wf-2", "probe-2", "obs-2", "publication/fence", "inconclusive", true),
		observations:  wfObservation("probe-2", "obs-2", "inconclusive"),
	}
	staleness := deriveWithSubjects(t, prior, parts, subjects)
	if got := findWorkflowSurface(t, staleness.Index, "lifecycle/transitions").Freshness; got != "stale" {
		t.Fatalf("freshness = %q, want stale after a product digest change", got)
	}
	parts.freshness = `{"surface_key":"lifecycle/transitions","decision":"retain","changed_paths":["docs/workflow/releasing.md"],` +
		`"evidence":"git diff --name-only over the tested commits","rationale":"no lifecycle path changed"}`
	retained := deriveWithSubjects(t, prior, parts, subjects)
	retainedRow := findWorkflowSurface(t, retained.Index, "lifecycle/transitions")
	if retainedRow.Freshness != "fresh" {
		t.Fatalf("freshness = %q, want fresh after an evidenced retain", retainedRow.Freshness)
	}
	// Retaining freshness must not advance the row's snapshot tuple: the surface
	// was never probed at the new commit, so claiming it was would be a false
	// record. Only the freshness verdict moves.
	staleRow := findWorkflowSurface(t, staleness.Index, "lifecycle/transitions")
	if retainedRow.ProductSHA256 != staleRow.ProductSHA256 || retainedRow.SpecSHA256 != staleRow.SpecSHA256 ||
		retainedRow.TestedHead != staleRow.TestedHead || retainedRow.CheckedAt != staleRow.CheckedAt {
		t.Fatalf("retain advanced the snapshot tuple: %#v", retainedRow)
	}
	if retainedRow.ProductSHA256 == subjects.ProductSHA256 {
		t.Fatal("retained row claims the untested product snapshot")
	}
	parts.freshness = `{"surface_key":"unknown/surface","decision":"retain","changed_paths":[],` +
		`"evidence":"git diff --name-only over the tested commits","rationale":"no lifecycle path changed"}`
	if _, err := deriveWithSubjectsErr(t, prior, parts, subjects); err == nil || !strings.Contains(err.Error(), "names unknown surface") {
		t.Fatalf("dangling assessment error = %v", err)
	}
}

func TestDeriveWorkflowIndexClosesOnlyEvidencedFindings(t *testing.T) {
	prior := workflowIndexGolden(t)
	parts := workflowReportParts{
		reviewID: "wf-2", productSHA: reviewDigestA, before: digestRaw(prior),
		scopeSurfaces: wfScopeSurface("wf-2", "probe-2", "obs-2", "lifecycle/transitions", "clean", ""),
		probes:        wfProbe("wf-2", "probe-2", "obs-2", "lifecycle/transitions", "pass", true),
		observations:  wfObservation("probe-2", "obs-2", "supports-clean"),
		findings:      wfFinding("wf-2", "probe-2", "obs-2", "WF-000001", "resolved", ""),
	}
	raw, _ := resolveParts(t, prior, parts)
	report, err := DecodeWorkflowReport(raw, workflowSubjects())
	if err != nil {
		t.Fatalf("DecodeWorkflowReport: %v", err)
	}
	derived, err := DeriveWorkflowIndex(prior, report)
	if err != nil {
		t.Fatalf("DeriveWorkflowIndex: %v", err)
	}
	if len(derived.Index.Findings) != 0 || derived.Index.NextFindingNumber != 2 {
		t.Fatalf("closure did not remove the finding or reused its number: %#v", derived.Index)
	}
	if len(findWorkflowSurface(t, derived.Index, "lifecycle/transitions").FindingIDs) != 0 {
		t.Fatal("closed finding is still referenced by a surface row")
	}
}

func TestDeriveWorkflowIndexRejectsHistoryRewrites(t *testing.T) {
	prior := workflowIndexGolden(t)
	tests := []struct {
		name  string
		parts workflowReportParts
		want  string
	}{
		{
			name: "closes an unknown finding",
			parts: workflowReportParts{
				reviewID: "wf-2", productSHA: reviewDigestA, before: "absent",
				scopeSurfaces: wfScopeSurface("wf-2", "probe-2", "obs-2", "lifecycle/transitions", "finding", `"WF-000001"`),
				probes:        wfProbe("wf-2", "probe-2", "obs-2", "lifecycle/transitions", "pass", true),
				observations:  wfObservation("probe-2", "obs-2", "supports-clean"),
				findings:      wfFinding("wf-2", "probe-2", "obs-2", "WF-000001", "resolved", ""),
			},
			want: "closes a finding absent from prior memory",
		},
		{
			name: "invents a task_id",
			parts: workflowReportParts{
				reviewID: "wf-2", productSHA: reviewDigestA, before: digestRaw(prior),
				scopeSurfaces: wfScopeSurface("wf-2", "probe-2", "obs-2", "publication/fence", "finding", `"WF-000002"`),
				probes:        wfProbe("wf-2", "probe-2", "obs-2", "publication/fence", "fail", true),
				observations:  wfObservation("probe-2", "obs-2", "supports-finding"),
				findings:      wfFinding("wf-2", "probe-2", "obs-2", "WF-000002", "deferred", "T-300-example"),
			},
			want: "invents a task_id without tracking it",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			priorArg := prior
			if tc.parts.before == "absent" {
				priorArg = nil
			}
			tc.parts.after = reviewDigestB
			report, err := DecodeWorkflowReport(tc.parts.json(), workflowSubjects())
			if err != nil {
				t.Fatalf("DecodeWorkflowReport: %v", err)
			}
			if _, err := DeriveWorkflowIndex(priorArg, report); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
	t.Run("reuses a retired finding number", func(t *testing.T) {
		advanced := workflowIndexWithSurfaces(0)
		advanced.NextFindingNumber = 5
		advancedRaw := mustEncodeIndex(t, advanced)
		parts := workflowReportParts{
			reviewID: "wf-2", productSHA: reviewDigestA, before: digestRaw(advancedRaw), after: reviewDigestB,
			scopeSurfaces: wfScopeSurface("wf-2", "probe-2", "obs-2", "publication/fence", "finding", `"WF-000002"`),
			probes:        wfProbe("wf-2", "probe-2", "obs-2", "publication/fence", "fail", true),
			observations:  wfObservation("probe-2", "obs-2", "supports-finding"),
			findings:      wfFinding("wf-2", "probe-2", "obs-2", "WF-000002", "open", ""),
		}
		report, err := DecodeWorkflowReport(parts.json(), workflowSubjects())
		if err != nil {
			t.Fatalf("DecodeWorkflowReport: %v", err)
		}
		if _, err := DeriveWorkflowIndex(advancedRaw, report); err == nil || !strings.Contains(err.Error(), "reuses a finding number") {
			t.Fatalf("error = %v, want a rejected finding-number reuse", err)
		}
	})
	t.Run("rewrites a stored task_id", func(t *testing.T) {
		tracked := workflowIndexWithSurfaces(0)
		tracked.NextFindingNumber = 2
		row := workflowIndexFindingRow("WF-000001")
		id := "T-100-original"
		row.Status, row.TaskID = "tracked", &id
		tracked.Findings = []WorkflowIndexFinding{row}
		trackedRaw := mustEncodeIndex(t, tracked)
		parts := workflowReportParts{
			reviewID: "wf-2", productSHA: reviewDigestA, before: digestRaw(trackedRaw), after: reviewDigestB,
			scopeSurfaces: wfScopeSurface("wf-2", "probe-2", "obs-2", "publication/fence", "finding", `"WF-000001"`),
			probes:        wfProbe("wf-2", "probe-2", "obs-2", "publication/fence", "fail", true),
			observations:  wfObservation("probe-2", "obs-2", "supports-finding"),
			findings:      wfFinding("wf-2", "probe-2", "obs-2", "WF-000001", "tracked", "T-200-renamed"),
		}
		report, err := DecodeWorkflowReport(parts.json(), workflowSubjects())
		if err != nil {
			t.Fatalf("DecodeWorkflowReport: %v", err)
		}
		if _, err := DeriveWorkflowIndex(trackedRaw, report); err == nil || !strings.Contains(err.Error(), "rewrites the historical task_id") {
			t.Fatalf("error = %v, want a rejected task_id rewrite", err)
		}
	})
	t.Run("prior memory digest drift", func(t *testing.T) {
		raw, _ := resolveParts(t, prior, firstRunParts(""))
		report, err := DecodeWorkflowReport(raw, workflowSubjects())
		if err != nil {
			t.Fatalf("DecodeWorkflowReport: %v", err)
		}
		if _, err := DeriveWorkflowIndex(prior, report); err == nil || !strings.Contains(err.Error(), "index_sha256_before does not match") {
			t.Fatalf("error = %v, want a rejected prior-memory digest", err)
		}
	})
}

func TestDeriveWorkflowIndexRefusesOverflowWithoutDroppingFindings(t *testing.T) {
	prior := workflowIndexWithSurfaces(workflowIndexSurfaceLimit)
	prior.NextFindingNumber = 2
	prior.Findings = []WorkflowIndexFinding{workflowIndexFindingRow("WF-000001")}
	priorRaw := mustEncodeIndex(t, prior)
	if _, err := DecodeWorkflowIndex(priorRaw); err != nil {
		t.Fatalf("prior index must itself be valid: %v", err)
	}
	parts := workflowReportParts{
		reviewID: "wf-2", productSHA: reviewDigestA, before: digestRaw(priorRaw), after: reviewDigestB,
		scopeSurfaces: wfScopeSurface("wf-2", "probe-2", "obs-2", "zzz/new-surface", "finding", `"WF-000002"`),
		probes:        wfProbe("wf-2", "probe-2", "obs-2", "zzz/new-surface", "fail", true),
		observations:  wfObservation("probe-2", "obs-2", "supports-finding"),
		findings:      wfFinding("wf-2", "probe-2", "obs-2", "WF-000002", "open", ""),
	}
	report, err := DecodeWorkflowReport(parts.json(), workflowSubjects())
	if err != nil {
		t.Fatalf("DecodeWorkflowReport: %v", err)
	}
	_, err = DeriveWorkflowIndex(priorRaw, report)
	if err == nil || !strings.Contains(err.Error(), "over the 256 row cap") {
		t.Fatalf("error = %v, want a refused row overflow", err)
	}
	// Refusal must leave prior memory intact rather than trimming rows to fit.
	reread, err := DecodeWorkflowIndex(priorRaw)
	if err != nil || len(reread.Findings) != 1 || len(reread.Surfaces) != workflowIndexSurfaceLimit {
		t.Fatalf("prior memory was not preserved: %#v %v", reread, err)
	}
	small := workflowIndexGolden(t)
	parts.before = digestRaw(small)
	parts.scopeSurfaces = strings.Replace(wfScopeSurface("wf-2", "probe-2", "obs-2", "zzz/new-surface", "inconclusive", ""),
		`"next_angle":"probe the rework path"`, `"next_angle":"`+strings.Repeat("b", workflowIndexByteLimit)+`"`, 1)
	parts.findings = ""
	if report, err = DecodeWorkflowReport(parts.json(), workflowSubjects()); err != nil {
		t.Fatalf("DecodeWorkflowReport: %v", err)
	}
	if _, err := DeriveWorkflowIndex(small, report); err == nil || !strings.Contains(err.Error(), "over the 262144 byte cap") {
		t.Fatalf("error = %v, want a refused byte overflow", err)
	}
	if reread, err := DecodeWorkflowIndex(small); err != nil || len(reread.Findings) != 1 {
		t.Fatalf("prior memory was not preserved: %#v %v", reread, err)
	}
}

func deriveWithSubjects(t *testing.T, prior []byte, parts workflowReportParts, subjects WorkflowSubjects) WorkflowIndexCandidate {
	t.Helper()
	candidate, err := deriveWithSubjectsErr(t, prior, parts, subjects)
	if err != nil {
		t.Fatalf("DeriveWorkflowIndex: %v", err)
	}
	return candidate
}

func deriveWithSubjectsErr(t *testing.T, prior []byte, parts workflowReportParts, subjects WorkflowSubjects) (WorkflowIndexCandidate, error) {
	t.Helper()
	raw, _, err := resolvePartsWith(t, prior, parts, subjects)
	if err != nil {
		return WorkflowIndexCandidate{}, err
	}
	report, err := DecodeWorkflowReport(raw, subjects)
	if err != nil {
		t.Fatalf("DecodeWorkflowReport: %v", err)
	}
	return DeriveWorkflowIndex(prior, report)
}

func findWorkflowSurface(t *testing.T, index WorkflowIndex, key string) WorkflowSurface {
	t.Helper()
	for _, surface := range index.Surfaces {
		if surface.SurfaceKey == key {
			return surface
		}
	}
	t.Fatalf("surface %q is missing from %#v", key, index.Surfaces)
	return WorkflowSurface{}
}

func mustEncodeIndex(t *testing.T, index WorkflowIndex) []byte {
	t.Helper()
	raw, err := EncodeWorkflowIndex(index)
	if err != nil {
		t.Fatalf("EncodeWorkflowIndex: %v", err)
	}
	return raw
}

func workflowIndexWithSurfaces(count int) WorkflowIndex {
	index := emptyWorkflowIndex()
	for i := 0; i < count; i++ {
		index.Surfaces = append(index.Surfaces, WorkflowSurface{
			SurfaceKey:    fmt.Sprintf("surface/%04d", i),
			EvidenceRefs:  []WorkflowEvidenceRef{{ReviewID: "wf-0", ProbeID: "probe-0", ObservationID: "obs-0"}},
			Outcome:       "inconclusive",
			Freshness:     "fresh",
			SpecPath:      "specs/v0.5.0.md",
			SpecSHA256:    digestRaw([]byte(workflowSpecText)),
			ProductSHA256: reviewDigestA,
			TestedHead:    workflowHead,
			CheckedAt:     "2026-08-12T09:00:00Z",
			FindingIDs:    []string{},
			NextAngle:     "probe deeper",
		})
	}
	return index
}

func workflowIndexFindingRow(id string) WorkflowIndexFinding {
	snapshot := WorkflowSnapshot{
		ReviewID:      "wf-0",
		SpecPath:      "specs/v0.5.0.md",
		SpecSHA256:    digestRaw([]byte(workflowSpecText)),
		ProductSHA256: reviewDigestA,
		TestedHead:    workflowHead,
		CheckedAt:     "2026-08-12T09:00:00Z",
	}
	return WorkflowIndexFinding{
		FindingID:    id,
		Severity:     "high",
		EvidenceRefs: []WorkflowEvidenceRef{{ReviewID: "wf-0", ProbeID: "probe-0", ObservationID: "obs-0"}},
		Impact:       "a completed task can be completed again",
		FirstSeen:    snapshot,
		LastChecked:  snapshot,
		Status:       "open",
		Rationale:    "observed directly",
	}
}
