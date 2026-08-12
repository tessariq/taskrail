package taskrail

import (
	"strings"
	"testing"
)

const (
	workflowHead     = "cafecafecafecafecafecafecafecafecafecafe"
	workflowSpecText = "# Spec\n"
)

func workflowSubjects() WorkflowSubjects {
	return WorkflowSubjects{
		SpecPath:      "specs/v0.5.0.md",
		Spec:          []byte(workflowSpecText),
		TestedHead:    workflowHead,
		ProductSHA256: reviewDigestA,
		ArtifactsDir:  "planning/artifacts",
	}
}

func wfRef(review, probe, observation string) string {
	return `{"review_id":"` + review + `","probe_id":"` + probe + `","observation_id":"` + observation + `"}`
}

func wfProbe(review, probe, observation, surface, outcome string, executed bool) string {
	return `{"probe_id":"` + probe + `","surface_keys":["` + surface + `"],"action":"run the transition twice","executed":` +
		map[bool]string{true: "true", false: "false"}[executed] + `,"outcome":"` + outcome +
		`","observation_ids":["` + observation + `"],"evidence_refs":[` + wfRef(review, probe, observation) + `]}`
}

func wfObservation(probe, observation, outcome string) string {
	return `{"observation_id":"` + observation + `","probe_id":"` + probe + `","expected":"one write","observed":"two writes","outcome":"` +
		outcome + `","evidence":[{"kind":"command","summary":"second complete succeeded","path":null,"sha256":null,"command":"taskrail complete T-1","exit_code":0}]}`
}

func wfScopeSurface(review, probe, observation, surface, outcome, findingIDs string) string {
	return `{"surface_key":"` + surface + `","angle":"repeat the terminal transition","rationale":"never probed before","outcome":"` +
		outcome + `","evidence_refs":[` + wfRef(review, probe, observation) + `],"finding_ids":[` + findingIDs +
		`],"next_angle":"probe the rework path"}`
}

func wfFinding(review, probe, observation, id, status, taskID string) string {
	optional := ""
	if taskID != "" {
		optional = `,"task_id":"` + taskID + `"`
	}
	return `{"finding_id":"` + id + `","severity":"high","evidence_refs":[` + wfRef(review, probe, observation) +
		`],"impact":"a completed task can be completed again","status":"` + status + `","rationale":"observed directly"` + optional + `}`
}

type workflowReportParts struct {
	reviewID, productSHA, before, after string
	scopeSurfaces, freshness            string
	probes, observations, findings      string
}

func (p workflowReportParts) json() []byte {
	return []byte(`{"schema_version":1,"prompt_id":"workflow-adversarial","prompt_contract_version":"v1","prompt_template_sha256":"` +
		reviewDigestB + `","prompt_source":"builtin","review_id":"` + p.reviewID + `","spec_path":"specs/v0.5.0.md","spec_sha256":"` +
		digestRaw([]byte(workflowSpecText)) + `","tested_head":"` + workflowHead + `","product_sha256":"` + p.productSHA +
		`","context_mode":"fresh","generated_at":"2026-08-12T10:00:00Z","scope":{"summary":"serial probe of lifecycle transitions","surfaces":[` +
		p.scopeSurfaces + `],"freshness_assessments":[` + p.freshness + `]},"probes":[` + p.probes + `],"observations":[` +
		p.observations + `],"findings":[` + p.findings + `],"index_sha256_before":"` + p.before + `","index_sha256_after":"` + p.after + `"}`)
}

// firstRunParts is one report against absent memory: one tested surface, one
// executed probe, and one new open finding.
func firstRunParts(after string) workflowReportParts {
	return workflowReportParts{
		reviewID:      "wf-1",
		productSHA:    reviewDigestA,
		before:        "absent",
		after:         after,
		scopeSurfaces: wfScopeSurface("wf-1", "probe-1", "obs-1", "lifecycle/transitions", "finding", `"WF-000001"`),
		probes:        wfProbe("wf-1", "probe-1", "obs-1", "lifecycle/transitions", "fail", true),
		observations:  wfObservation("probe-1", "obs-1", "supports-finding"),
		findings:      wfFinding("wf-1", "probe-1", "obs-1", "WF-000001", "open", ""),
	}
}

// resolveParts computes the candidate a set of report parts derives, then
// rewrites index_sha256_after so the report binds the bytes Taskrail derives.
func resolveParts(t *testing.T, priorRaw []byte, parts workflowReportParts) ([]byte, WorkflowIndexCandidate) {
	t.Helper()
	raw, candidate, err := resolvePartsWith(t, priorRaw, parts, workflowSubjects())
	if err != nil {
		t.Fatalf("buildWorkflowIndexCandidate: %v", err)
	}
	return raw, candidate
}

func resolvePartsWith(t *testing.T, priorRaw []byte, parts workflowReportParts, subjects WorkflowSubjects) ([]byte, WorkflowIndexCandidate, error) {
	t.Helper()
	parts.after = reviewDigestB
	report, err := DecodeWorkflowReport(parts.json(), subjects)
	if err != nil {
		t.Fatalf("DecodeWorkflowReport: %v", err)
	}
	prior := emptyWorkflowIndex()
	if priorRaw != nil {
		if prior, err = DecodeWorkflowIndex(priorRaw); err != nil {
			t.Fatalf("DecodeWorkflowIndex: %v", err)
		}
	}
	candidate, err := buildWorkflowIndexCandidate(prior, report)
	if err != nil {
		return nil, WorkflowIndexCandidate{}, err
	}
	parts.after = candidate.SHA256
	return parts.json(), candidate, nil
}

func workflowReportGolden(t *testing.T) []byte {
	t.Helper()
	raw, _ := resolveParts(t, nil, firstRunParts(""))
	return raw
}

func workflowIndexGolden(t *testing.T) []byte {
	t.Helper()
	_, candidate := resolveParts(t, nil, firstRunParts(""))
	return candidate.Index.Raw
}

func TestDecodeWorkflowReportPreservesAcceptedBytes(t *testing.T) {
	raw := workflowReportGolden(t)
	report, err := DecodeWorkflowReport(raw, workflowSubjects())
	if err != nil {
		t.Fatalf("DecodeWorkflowReport: %v", err)
	}
	if string(report.Raw) != string(raw) {
		t.Fatal("accepted bytes were not preserved")
	}
	if report.ReviewID != "wf-1" || report.IndexSHA256Before != "absent" || len(report.Scope.Surfaces) != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
	if report.Binding.PromptID != "workflow-adversarial" || report.Binding.PromptContractVersion != "v1" {
		t.Fatalf("unexpected prompt binding: %#v", report.Binding)
	}
}

func TestDecodeWorkflowReportRejectsStrictMutations(t *testing.T) {
	base := string(workflowReportGolden(t))
	tests := []struct {
		name string
		data string
		want string
	}{
		{"unknown", strings.Replace(base, `"review_id"`, `"unknown":true,"review_id"`, 1), "unknown member"},
		{"missing", strings.Replace(base, `"context_mode":"fresh",`, "", 1), "missing member"},
		{"duplicate", strings.Replace(base, `"review_id":"wf-1"`, `"review_id":"wf-1","review_id":"wf-2"`, 1), "repeats member"},
		{"trailing", base + `{}`, "trailing value"},
		{"version", strings.Replace(base, `"schema_version":1`, `"schema_version":2`, 1), "unsupported schema version"},
		{"role", strings.Replace(base, `"prompt_id":"workflow-adversarial"`, `"prompt_id":"spec-gaps"`, 1), `must be "workflow-adversarial"`},
		{"contract", strings.Replace(base, `"prompt_contract_version":"v1"`, `"prompt_contract_version":"v2"`, 1), `must be "v1"`},
		{"spec snapshot", strings.Replace(base, `"spec_sha256":"`+digestRaw([]byte(workflowSpecText)), `"spec_sha256":"`+reviewDigestB, 1), "does not match selected spec"},
		{"spec path", strings.Replace(base, `"spec_path":"specs/v0.5.0.md"`, `"spec_path":"specs/v0.6.0.md"`, 1), `must be "specs/v0.5.0.md"`},
		{"head", strings.Replace(base, `"tested_head":"`+workflowHead, `"tested_head":"cafecafe`, 1), "full Git object ID"},
		{"head snapshot", strings.Replace(base, `"tested_head":"`+workflowHead, `"tested_head":"`+strings.Repeat("d", 40), 1), "does not match the tested commit"},
		{"product snapshot", strings.Replace(base, `"product_sha256":"`+reviewDigestA, `"product_sha256":"`+reviewDigestB, 1), "does not match the tested product digest"},
		{"context mode", strings.Replace(base, `"context_mode":"fresh"`, `"context_mode":"cached"`, 1), "allowed value"},
		{"time", strings.Replace(base, `2026-08-12T10:00:00Z`, `2026-08-12T10:00:00+00:00`, 1), "canonical RFC3339 UTC"},
		{"review key", strings.Replace(base, `"review_id":"wf-1"`, `"review_id":"WF_1"`, 1), "portable review key"},
		{"index before", strings.Replace(base, `"index_sha256_before":"absent"`, `"index_sha256_before":"none"`, 1), `or "absent"`},
		{"surface key", strings.Replace(base, `"surface_key":"lifecycle/transitions"`, `"surface_key":"Lifecycle Transitions"`, 1), "normalized surface key"},
		{"scope cap", strings.Replace(base, `"surfaces":[`+wfScopeSurface("wf-1", "probe-1", "obs-1", "lifecycle/transitions", "finding", `"WF-000001"`),
			`"surfaces":[`+wfScopeSurface("wf-1", "probe-1", "obs-1", "a", "inconclusive", "")+`,`+
				wfScopeSurface("wf-1", "probe-1", "obs-1", "b", "inconclusive", "")+`,`+
				wfScopeSurface("wf-1", "probe-1", "obs-1", "c", "inconclusive", "")+`,`+
				wfScopeSurface("wf-1", "probe-1", "obs-1", "d", "inconclusive", ""), 1), "more than 3 surface keys"},
		{"finding id", strings.Replace(base, `"finding_id":"WF-000001"`, `"finding_id":"WF-1"`, 2), "zero-padded number"},
		{"finding status", strings.Replace(base, `"status":"open"`, `"status":"unknown"`, 1), "allowed value"},
		{"tracked without task", strings.Replace(base, `"status":"open"`, `"status":"tracked"`, 1), "without task_id"},
		{"task id form", strings.Replace(base, `"status":"open"`, `"status":"tracked","task_id":"not-a-task"`, 1), "full task ID"},
		{"dangling observation", strings.Replace(base, `"observation_id":"obs-1"}]`, `"observation_id":"obs-9"}]`, 1), "unknown observation"},
		{"dangling finding", strings.Replace(base, `"finding_ids":["WF-000001"]`, `"finding_ids":["WF-000002"]`, 1), "unknown finding"},
		{"probe surface", strings.Replace(base, `"surface_keys":["lifecycle/transitions"]`, `"surface_keys":["other/surface"]`, 1), "untested surface"},
		{"duplicate refs", strings.Replace(base, `"evidence_refs":[`+wfRef("wf-1", "probe-1", "obs-1")+`],"impact"`,
			`"evidence_refs":[`+wfRef("wf-1", "probe-1", "obs-1")+`,`+wfRef("wf-1", "probe-1", "obs-1")+`],"impact"`, 1), "not sorted and unique"},
		{"unsorted refs", strings.Replace(base, `"evidence_refs":[`+wfRef("wf-1", "probe-1", "obs-1")+`],"impact"`,
			`"evidence_refs":[`+wfRef("wf-2", "probe-1", "obs-1")+`,`+wfRef("wf-1", "probe-1", "obs-1")+`],"impact"`, 1), "not sorted and unique"},
		{"observation reference inside evidence", strings.Replace(base, `"evidence":[{"kind":"command"`, `"evidence":[{"observation_id":"obs-1","kind":"command"`, 1), "unknown member"},
		{"evidence combination", strings.Replace(base, `"command":"taskrail complete T-1","exit_code":0`, `"command":null,"exit_code":null`, 1), "requires command and exit_code"},
		{"file evidence combination", strings.Replace(base, `"kind":"command","summary":"second complete succeeded","path":null,"sha256":null,"command":"taskrail complete T-1","exit_code":0`,
			`"kind":"file","summary":"captured output","path":"internal/taskrail/service.go","sha256":null,"command":null,"exit_code":null`, 1), "lower-case 64-hex sha256"},
		{"manual evidence combination", strings.Replace(base, `"kind":"command","summary":"second complete succeeded","path":null,"sha256":null,"command":"taskrail complete T-1","exit_code":0`,
			`"kind":"manual","summary":"read the code","path":null,"sha256":null,"command":"ls","exit_code":null`, 1), `manual evidence must leave "command" null`},
		{"transient artifact evidence", strings.Replace(base, `"kind":"command","summary":"second complete succeeded","path":null,"sha256":null,"command":"taskrail complete T-1","exit_code":0`,
			`"kind":"file","summary":"proposal bytes","path":"planning/artifacts/review-proposals/workflow/p1/report.json","sha256":"`+reviewDigestA+`","command":null,"exit_code":null`, 1), "is transient"},
		{"local overlay evidence", strings.Replace(base, `"kind":"command","summary":"second complete succeeded","path":null,"sha256":null,"command":"taskrail complete T-1","exit_code":0`,
			`"kind":"file","summary":"overlay bytes","path":".taskrail/local/planning/STATE.md","sha256":"`+reviewDigestA+`","command":null,"exit_code":null`, 1), "is transient"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeWorkflowReport([]byte(tc.data), workflowSubjects())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
	oversize := append(workflowReportGolden(t), make([]byte, reviewFileLimit+1)...)
	if _, err := DecodeWorkflowReport(oversize, workflowSubjects()); err == nil || !strings.Contains(err.Error(), "exceeds 1 MiB") {
		t.Fatalf("oversize error = %v", err)
	}
}

func TestDecodeWorkflowReportRejectsUnsupportedCleanAndClosureEvidence(t *testing.T) {
	tests := []struct {
		name  string
		parts workflowReportParts
		want  string
	}{
		{
			name: "clean without executed probe",
			parts: workflowReportParts{
				reviewID: "wf-1", productSHA: reviewDigestA, before: "absent", after: reviewDigestB,
				scopeSurfaces: wfScopeSurface("wf-1", "probe-1", "obs-1", "lifecycle/transitions", "clean", ""),
				probes:        wfProbe("wf-1", "probe-1", "obs-1", "lifecycle/transitions", "inconclusive", false),
				observations:  wfObservation("probe-1", "obs-1", "supports-clean"),
			},
			want: "claims clean without an executed probe",
		},
		{
			name: "clean without observable evidence",
			parts: workflowReportParts{
				reviewID: "wf-1", productSHA: reviewDigestA, before: "absent", after: reviewDigestB,
				scopeSurfaces: wfScopeSurface("wf-1", "probe-1", "obs-1", "lifecycle/transitions", "clean", ""),
				probes:        wfProbe("wf-1", "probe-1", "obs-1", "lifecycle/transitions", "pass", true),
				observations: `{"observation_id":"obs-1","probe_id":"probe-1","expected":"no write","observed":"no write","outcome":"supports-clean",` +
					`"evidence":[{"kind":"manual","summary":"read the transition code","path":null,"sha256":null,"command":null,"exit_code":null}]}`,
			},
			want: "claims clean without an executed probe",
		},
		{
			name: "closure without executed attempt",
			parts: workflowReportParts{
				reviewID: "wf-1", productSHA: reviewDigestA, before: "absent", after: reviewDigestB,
				scopeSurfaces: wfScopeSurface("wf-1", "probe-1", "obs-1", "lifecycle/transitions", "finding", `"WF-000001"`),
				probes:        wfProbe("wf-1", "probe-1", "obs-1", "lifecycle/transitions", "inconclusive", false),
				observations:  wfObservation("probe-1", "obs-1", "supports-finding"),
				findings:      wfFinding("wf-1", "probe-1", "obs-1", "WF-000001", "resolved", ""),
			},
			want: "without a fresh executed attempt supporting absence",
		},
		{
			name: "clean on evidence that supports a finding",
			parts: workflowReportParts{
				reviewID: "wf-1", productSHA: reviewDigestA, before: "absent", after: reviewDigestB,
				scopeSurfaces: wfScopeSurface("wf-1", "probe-1", "obs-1", "lifecycle/transitions", "clean", ""),
				probes:        wfProbe("wf-1", "probe-1", "obs-1", "lifecycle/transitions", "fail", true),
				observations:  wfObservation("probe-1", "obs-1", "supports-finding"),
			},
			want: "claims clean without an executed probe",
		},
		{
			name: "closure on evidence that still supports the finding",
			parts: workflowReportParts{
				reviewID: "wf-1", productSHA: reviewDigestA, before: "absent", after: reviewDigestB,
				scopeSurfaces: wfScopeSurface("wf-1", "probe-1", "obs-1", "lifecycle/transitions", "finding", `"WF-000001"`),
				probes:        wfProbe("wf-1", "probe-1", "obs-1", "lifecycle/transitions", "fail", true),
				observations:  wfObservation("probe-1", "obs-1", "supports-finding"),
				findings:      wfFinding("wf-1", "probe-1", "obs-1", "WF-000001", "resolved", ""),
			},
			want: "without a fresh executed attempt supporting absence",
		},
		{
			name: "closure referencing only prior evidence",
			parts: workflowReportParts{
				reviewID: "wf-2", productSHA: reviewDigestA, before: "absent", after: reviewDigestB,
				scopeSurfaces: wfScopeSurface("wf-2", "probe-1", "obs-1", "lifecycle/transitions", "finding", `"WF-000001"`),
				probes:        wfProbe("wf-2", "probe-1", "obs-1", "lifecycle/transitions", "pass", true),
				observations:  wfObservation("probe-1", "obs-1", "supports-clean"),
				findings:      wfFinding("wf-1", "probe-1", "obs-1", "WF-000001", "not-reproducible", ""),
			},
			want: "without a fresh executed attempt supporting absence",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeWorkflowReport(tc.parts.json(), workflowSubjects())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestDecodeWorkflowReportRejectsUnexplainedFreshnessRetention(t *testing.T) {
	parts := firstRunParts(reviewDigestB)
	parts.freshness = `{"surface_key":"lifecycle/transitions","decision":"retain","changed_paths":["docs/workflow/releasing.md"],"evidence":"","rationale":"unrelated docs only"}`
	if _, err := DecodeWorkflowReport(parts.json(), workflowSubjects()); err == nil || !strings.Contains(err.Error(), `member "evidence" is empty`) {
		t.Fatalf("error = %v, want empty retain evidence rejected", err)
	}
	parts.freshness = `{"surface_key":"lifecycle/transitions","decision":"retain","changed_paths":["a.md","a.md"],"evidence":"diff of changed paths","rationale":"unrelated docs only"}`
	if _, err := DecodeWorkflowReport(parts.json(), workflowSubjects()); err == nil || !strings.Contains(err.Error(), "repeats changed path") {
		t.Fatalf("error = %v, want repeated changed path rejected", err)
	}
	parts.freshness = `{"surface_key":"lifecycle/transitions","decision":"retain","changed_paths":["../outside.md"],"evidence":"diff of changed paths","rationale":"unrelated docs only"}`
	if _, err := DecodeWorkflowReport(parts.json(), workflowSubjects()); err == nil || !strings.Contains(err.Error(), "not a canonical repository-relative path") {
		t.Fatalf("error = %v, want escaping changed path rejected", err)
	}
	// The spec calls changed_paths ordered, not byte-sorted, so producer order stands.
	parts.freshness = `{"surface_key":"lifecycle/transitions","decision":"retain","changed_paths":["b.md","a.md"],"evidence":"diff of changed paths","rationale":"unrelated docs only"}`
	if _, err := DecodeWorkflowReport(parts.json(), workflowSubjects()); err != nil {
		t.Fatalf("unsorted changed_paths were rejected: %v", err)
	}
}
