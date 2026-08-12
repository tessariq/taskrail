package taskrail

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

type DecompositionSubjects struct {
	SpecPath, SpecReviewManifestPath string
	Spec, SpecReviewManifest         []byte
}

type DecompositionBundle struct {
	Raw      map[string][]byte
	Draft    ReviewedImportDraft
	Trace    DecompositionTrace
	Reviews  []DecompositionReview
	Manifest DecompositionManifest
}

type ReviewedImportDraft struct {
	Raw, SpecSectionsRaw []byte
	ReviewSessionID      string
	Tasks                []TaskDraft
}

type DecompositionTrace struct {
	Raw                             []byte
	SessionID, SpecPath, SpecSHA256 string
	Requirements                    []DecompositionRequirement
}

type DecompositionRequirement struct {
	RequirementID, SpecRef, Disposition, Rationale string
	Source                                         DecompositionSource
	TaskKeys                                       []string
}

type DecompositionSource struct {
	Kind, Text string
	Start, End int
}

type DecompositionReview struct {
	Raw                                                                                          []byte
	Binding                                                                                      ReviewPromptBinding
	SessionID, SpecPath, SpecSHA256, DraftPath, DraftSHA256, TracePath, TraceSHA256, ContextMode string
	PassNumber                                                                                   int
	Findings                                                                                     []DecompositionFinding
}

type DecompositionFinding struct{ FindingID, Severity, Evidence, Impact, Recommendation string }

type DecompositionManifest struct {
	Raw                                                                                                                  []byte
	SessionID, SpecReviewManifestPath, SpecReviewManifestSHA256, SpecPath, SpecSHA256, DraftPath, DraftSHA256, TracePath string
	TraceSHA256, GeneratedAt, ApprovedAt                                                                                 string
	Reviews                                                                                                              []DecompositionManifestReview
	Dispositions                                                                                                         []DecompositionDisposition
}

type DecompositionManifestReview struct {
	PassNumber                                                      int
	Path, SHA256, ContextMode, SpecSHA256, DraftSHA256, TraceSHA256 string
}

type DecompositionDisposition struct{ FindingID, Severity, Disposition, Rationale string }

func DecodeDecompositionBundle(files map[string][]byte, subjects DecompositionSubjects) (DecompositionBundle, error) {
	var bundle DecompositionBundle
	want := []string{"draft.json", "trace.json", "review-1.json", "manifest.json"}
	for _, name := range want {
		if _, ok := files[name]; !ok {
			return bundle, fmt.Errorf("decomposition bundle is missing file %q", name)
		}
	}
	for name := range files {
		if !slices.Contains(want, name) && name != "review-2.json" {
			return bundle, fmt.Errorf("decomposition bundle has unknown file %q", name)
		}
	}
	if subjects.SpecPath == "" || subjects.SpecReviewManifestPath == "" || !utf8.Valid(subjects.Spec) {
		return bundle, fmt.Errorf("decomposition subjects are incomplete or selected spec is not UTF-8")
	}
	specReview, err := decodeSpecReviewManifest(subjects.SpecReviewManifest)
	if err != nil {
		return bundle, fmt.Errorf("post-spec review manifest: %w", err)
	}
	if specReview.SpecPath != subjects.SpecPath || specReview.SpecSHA256 != digestRaw(subjects.Spec) {
		return bundle, fmt.Errorf("post-spec review manifest does not bind selected spec exact bytes")
	}

	if bundle.Draft, err = decodeReviewedDraft(files["draft.json"]); err != nil {
		return bundle, fmt.Errorf("draft.json: %w", err)
	}
	if bundle.Trace, err = decodeDecompositionTrace(files["trace.json"], subjects); err != nil {
		return bundle, fmt.Errorf("trace.json: %w", err)
	}
	if err := validateDraftTrace(bundle.Draft, bundle.Trace, subjects); err != nil {
		return bundle, err
	}

	reviewCount := 1
	if _, ok := files["review-2.json"]; ok {
		reviewCount = 2
	}
	findings := map[string]DecompositionFinding{}
	for pass := 1; pass <= reviewCount; pass++ {
		path := fmt.Sprintf("review-%d.json", pass)
		review, err := decodeDecompositionReview(files[path], pass)
		if err != nil {
			return bundle, fmt.Errorf("%s: %w", path, err)
		}
		for _, finding := range review.Findings {
			if prior, ok := findings[finding.FindingID]; ok && prior.Severity != finding.Severity {
				return bundle, fmt.Errorf("%s finding %q changes severity", path, finding.FindingID)
			}
			findings[finding.FindingID] = finding
		}
		bundle.Reviews = append(bundle.Reviews, review)
	}
	if bundle.Manifest, err = decodeDecompositionManifest(files["manifest.json"]); err != nil {
		return bundle, fmt.Errorf("manifest.json: %w", err)
	}
	if err := validateDecompositionManifest(bundle, subjects, findings); err != nil {
		return bundle, err
	}
	bundle.Raw = make(map[string][]byte, len(files))
	for name, raw := range files {
		bundle.Raw[name] = append([]byte(nil), raw...)
	}
	return bundle, nil
}

func decodeReviewedDraft(data []byte) (ReviewedImportDraft, error) {
	var out ReviewedImportDraft
	obj, err := decodeReviewObject(data, "reviewed import draft")
	if err != nil {
		return out, err
	}
	if err := exactOptionalMembers(obj, "reviewed import draft", []string{"schema_version", "review_session_id", "target", "tasks", "spec_sections"}, []string{"source"}); err != nil {
		return out, err
	}
	if v, ok := decodeJSONInteger(obj["schema_version"]); !ok || v != 2 {
		return out, fmt.Errorf("reviewed import draft schema_version must be 2")
	}
	if _, ok := obj["source"]; ok {
		return out, fmt.Errorf("decomposition draft forbids source")
	}
	if out.ReviewSessionID, err = reviewKeyMember(obj, "reviewed import draft", "review_session_id"); err != nil {
		return out, err
	}
	if _, err = fixedMember(obj, "reviewed import draft", "target", "tasks"); err != nil {
		return out, err
	}
	sections, err := arrayMember(obj["spec_sections"], "reviewed import draft", "spec_sections")
	if err != nil || len(sections) != 0 {
		return out, fmt.Errorf("reviewed import draft spec_sections must be an empty array")
	}
	elements, err := arrayMember(obj["tasks"], "reviewed import draft", "tasks")
	if err != nil {
		return out, err
	}
	if len(elements) == 0 {
		return out, fmt.Errorf("reviewed import draft tasks must not be empty")
	}
	seen := map[string]struct{}{}
	for i, raw := range elements {
		what := fmt.Sprintf("reviewed task at index %d", i)
		obj, err := strictObject(raw, what)
		if err != nil {
			return out, err
		}
		if err := exactOptionalMembers(obj, what, []string{"key", "title", "dependencies", "body", "spec_ref"}, []string{"priority"}); err != nil {
			return out, err
		}
		var task TaskDraft
		if task.Key, err = reviewKeyMember(obj, what, "key"); err != nil {
			return out, err
		}
		if _, exists := seen[task.Key]; exists {
			return out, fmt.Errorf("duplicate task key %q", task.Key)
		}
		seen[task.Key] = struct{}{}
		if task.Title, err = nonEmptyMember(obj, what, "title"); err != nil {
			return out, err
		}
		if task.Body, err = nonEmptyMember(obj, what, "body"); err != nil {
			return out, err
		}
		if err := validateReviewedBody(task.Body, what); err != nil {
			return out, err
		}
		if task.SpecRef, err = stringMember(obj, what, "spec_ref"); err != nil {
			return out, err
		}
		if normalized, e := normalizeSpecRef(task.SpecRef); e != nil || normalized != task.SpecRef {
			return out, fmt.Errorf("%s spec_ref is not canonical", what)
		}
		deps, err := arrayMember(obj["dependencies"], what, "dependencies")
		if err != nil {
			return out, err
		}
		depSeen := map[string]struct{}{}
		for j, depRaw := range deps {
			dep, err := rawString(depRaw, fmt.Sprintf("%s dependency at index %d", what, j))
			if err != nil {
				return out, err
			}
			if dep == "" {
				return out, fmt.Errorf("%s has empty dependency", what)
			}
			if _, exists := depSeen[dep]; exists {
				return out, fmt.Errorf("%s repeats dependency %q", what, dep)
			}
			depSeen[dep] = struct{}{}
			task.Dependencies = append(task.Dependencies, dep)
		}
		if raw, ok := obj["priority"]; ok {
			if task.Priority, err = rawString(raw, what+" priority"); err != nil {
				return out, err
			}
			if _, ok := validPriorites[task.Priority]; !ok {
				return out, fmt.Errorf("%s has invalid priority %q", what, task.Priority)
			}
		}
		out.Tasks = append(out.Tasks, task)
	}
	for _, task := range out.Tasks {
		for _, dep := range task.Dependencies {
			if _, ok := seen[dep]; !ok {
				if _, full := taskIDPrefix(dep); !full {
					return out, fmt.Errorf("task %q has unresolved dependency %q", task.Key, dep)
				}
			}
		}
	}
	if _, err := orderTaskDraftsByDeps(out.Tasks); err != nil {
		return out, err
	}
	out.Raw = append([]byte(nil), data...)
	return out, nil
}

func decodeDecompositionTrace(data []byte, subjects DecompositionSubjects) (DecompositionTrace, error) {
	var out DecompositionTrace
	obj, err := decodeReviewObject(data, "decomposition trace")
	if err != nil {
		return out, err
	}
	if err := exactMembers(obj, "decomposition trace", []string{"schema_version", "session_id", "spec_path", "spec_sha256", "requirements"}); err != nil {
		return out, err
	}
	if err := schemaVersionOne(obj, "decomposition trace"); err != nil {
		return out, err
	}
	if out.SessionID, err = reviewKeyMember(obj, "decomposition trace", "session_id"); err != nil {
		return out, err
	}
	if out.SpecPath, err = fixedMember(obj, "decomposition trace", "spec_path", subjects.SpecPath); err != nil {
		return out, err
	}
	if out.SpecSHA256, err = reviewDigestMember(obj, "decomposition trace", "spec_sha256"); err != nil {
		return out, err
	}
	if out.SpecSHA256 != digestRaw(subjects.Spec) {
		return out, fmt.Errorf("decomposition trace spec_sha256 does not match selected spec exact bytes")
	}
	elements, err := arrayMember(obj["requirements"], "decomposition trace", "requirements")
	if err != nil {
		return out, err
	}
	seen := map[string]struct{}{}
	anchors := collectHeadingAnchors(string(subjects.Spec))
	for i, raw := range elements {
		what := fmt.Sprintf("trace requirement at index %d", i)
		obj, err := strictObject(raw, what)
		if err != nil {
			return out, err
		}
		if err := exactMembers(obj, what, []string{"requirement_id", "spec_ref", "source", "task_keys", "disposition", "rationale"}); err != nil {
			return out, err
		}
		var req DecompositionRequirement
		if req.RequirementID, err = stringMember(obj, what, "requirement_id"); err != nil {
			return out, err
		}
		if _, ok := seen[req.RequirementID]; ok {
			return out, fmt.Errorf("duplicate requirement_id %q", req.RequirementID)
		}
		seen[req.RequirementID] = struct{}{}
		if req.SpecRef, err = stringMember(obj, what, "spec_ref"); err != nil {
			return out, err
		}
		path, anchor, e := parseSpecRef(req.SpecRef)
		normalized, ne := normalizeSpecRef(req.SpecRef)
		if e != nil || ne != nil || normalized != req.SpecRef || path != subjects.SpecPath {
			return out, fmt.Errorf("%s spec_ref is not a normalized selected-spec reference", what)
		}
		if _, ok := anchors[anchor]; !ok {
			return out, fmt.Errorf("%s spec_ref anchor does not exist", what)
		}
		if req.Source, err = decodeDecompositionSource(obj["source"], what, subjects.Spec); err != nil {
			return out, err
		}
		keys, err := arrayMember(obj["task_keys"], what, "task_keys")
		if err != nil {
			return out, err
		}
		keySeen := map[string]struct{}{}
		for j, rawKey := range keys {
			key, err := rawString(rawKey, fmt.Sprintf("%s task key at index %d", what, j))
			if err != nil {
				return out, err
			}
			if !portableReviewKey.MatchString(key) {
				return out, fmt.Errorf("%s has non-portable task key", what)
			}
			if _, ok := keySeen[key]; ok {
				return out, fmt.Errorf("%s repeats task key %q", what, key)
			}
			keySeen[key] = struct{}{}
			req.TaskKeys = append(req.TaskKeys, key)
		}
		if req.Disposition, err = enumMember(obj, what, "disposition", []string{"task", "no-task"}); err != nil {
			return out, err
		}
		if (req.Disposition == "task") != (len(req.TaskKeys) > 0) {
			return out, fmt.Errorf("%s task_keys do not match disposition", what)
		}
		if req.Rationale, err = nonEmptyMember(obj, what, "rationale"); err != nil {
			return out, err
		}
		out.Requirements = append(out.Requirements, req)
	}
	out.Raw = append([]byte(nil), data...)
	return out, nil
}

func decodeDecompositionSource(raw json.RawMessage, what string, spec []byte) (DecompositionSource, error) {
	var out DecompositionSource
	obj, err := strictObject(raw, what+" source")
	if err != nil {
		return out, err
	}
	if out.Kind, err = enumMember(obj, what+" source", "kind", []string{"quote", "lines"}); err != nil {
		return out, err
	}
	if out.Kind == "quote" {
		if err := exactMembers(obj, what+" source", []string{"kind", "text"}); err != nil {
			return out, err
		}
		if out.Text, err = nonEmptyMember(obj, what+" source", "text"); err != nil {
			return out, err
		}
		if bytes.Count(spec, []byte(out.Text)) != 1 {
			return out, fmt.Errorf("%s quote must occur exactly once in selected spec", what)
		}
		return out, nil
	}
	if err := exactMembers(obj, what+" source", []string{"kind", "start", "end"}); err != nil {
		return out, err
	}
	start, ok := decodeJSONInteger(obj["start"])
	if !ok {
		return out, fmt.Errorf("%s source start is not an integer", what)
	}
	end, ok := decodeJSONInteger(obj["end"])
	if !ok {
		return out, fmt.Errorf("%s source end is not an integer", what)
	}
	lines := 1 + bytes.Count(spec, []byte{'\n'})
	if len(spec) > 0 && spec[len(spec)-1] == '\n' {
		lines--
	}
	if start < 1 || end < start || end > int64(lines) {
		return out, fmt.Errorf("%s source line range is out of bounds", what)
	}
	out.Start, out.End = int(start), int(end)
	return out, nil
}

func validateDraftTrace(draft ReviewedImportDraft, trace DecompositionTrace, subjects DecompositionSubjects) error {
	if draft.ReviewSessionID != trace.SessionID {
		return fmt.Errorf("draft and trace session identities do not match")
	}
	keys, traced := map[string]struct{}{}, map[string]struct{}{}
	anchors := collectHeadingAnchors(string(subjects.Spec))
	for _, task := range draft.Tasks {
		keys[task.Key] = struct{}{}
		path, anchor, _ := parseSpecRef(task.SpecRef)
		if path != subjects.SpecPath {
			return fmt.Errorf("task %q spec_ref does not target selected spec", task.Key)
		}
		if _, ok := anchors[anchor]; !ok {
			return fmt.Errorf("task %q spec_ref anchor does not exist", task.Key)
		}
	}
	for _, req := range trace.Requirements {
		for _, key := range req.TaskKeys {
			if _, ok := keys[key]; !ok {
				return fmt.Errorf("trace requirement %q references unknown task key %q", req.RequirementID, key)
			}
			traced[key] = struct{}{}
		}
	}
	for key := range keys {
		if _, ok := traced[key]; !ok {
			return fmt.Errorf("draft task key %q is not covered by trace", key)
		}
	}
	return nil
}

func decodeDecompositionReview(data []byte, pass int) (DecompositionReview, error) {
	var out DecompositionReview
	obj, err := decodeReviewObject(data, "decomposition review")
	if err != nil {
		return out, err
	}
	if err := exactMembers(obj, "decomposition review", []string{"schema_version", "prompt_id", "prompt_contract_version", "prompt_template_sha256", "prompt_source", "session_id", "pass_number", "spec_path", "spec_sha256", "draft_path", "draft_sha256", "trace_path", "trace_sha256", "context_mode", "generated_at", "findings"}); err != nil {
		return out, err
	}
	if err := schemaVersionOne(obj, "decomposition review"); err != nil {
		return out, err
	}
	if out.Binding, err = decodePromptBinding(obj, "decomposition review", "task-decomposition-adversarial"); err != nil {
		return out, err
	}
	if out.SessionID, err = reviewKeyMember(obj, "decomposition review", "session_id"); err != nil {
		return out, err
	}
	n, ok := decodeJSONInteger(obj["pass_number"])
	if !ok || int(n) != pass {
		return out, fmt.Errorf("decomposition review pass_number must be %d", pass)
	}
	out.PassNumber = pass
	if out.SpecPath, err = reviewPathMember(obj, "decomposition review", "spec_path"); err != nil {
		return out, err
	}
	if out.SpecSHA256, err = reviewDigestMember(obj, "decomposition review", "spec_sha256"); err != nil {
		return out, err
	}
	if out.DraftPath, err = fixedMember(obj, "decomposition review", "draft_path", "draft.json"); err != nil {
		return out, err
	}
	if out.DraftSHA256, err = reviewDigestMember(obj, "decomposition review", "draft_sha256"); err != nil {
		return out, err
	}
	if out.TracePath, err = fixedMember(obj, "decomposition review", "trace_path", "trace.json"); err != nil {
		return out, err
	}
	if out.TraceSHA256, err = reviewDigestMember(obj, "decomposition review", "trace_sha256"); err != nil {
		return out, err
	}
	if out.ContextMode, err = enumMember(obj, "decomposition review", "context_mode", []string{"fresh", "same-context"}); err != nil {
		return out, err
	}
	if _, err = reviewTimeMember(obj, "decomposition review", "generated_at"); err != nil {
		return out, err
	}
	elements, err := arrayMember(obj["findings"], "decomposition review", "findings")
	if err != nil {
		return out, err
	}
	seen := map[string]struct{}{}
	for i, raw := range elements {
		what := fmt.Sprintf("decomposition finding at index %d", i)
		o, err := strictObject(raw, what)
		if err != nil {
			return out, err
		}
		if err := exactMembers(o, what, []string{"finding_id", "severity", "evidence", "impact", "recommendation"}); err != nil {
			return out, err
		}
		var f DecompositionFinding
		if f.FindingID, err = stringMember(o, what, "finding_id"); err != nil {
			return out, err
		}
		if _, ok := seen[f.FindingID]; ok {
			return out, fmt.Errorf("duplicate finding_id %q", f.FindingID)
		}
		seen[f.FindingID] = struct{}{}
		if f.Severity, err = enumMember(o, what, "severity", []string{"high", "medium", "low"}); err != nil {
			return out, err
		}
		if f.Evidence, err = decompositionTextMember(o, what, "evidence"); err != nil {
			return out, err
		}
		if f.Impact, err = decompositionTextMember(o, what, "impact"); err != nil {
			return out, err
		}
		if f.Recommendation, err = decompositionTextMember(o, what, "recommendation"); err != nil {
			return out, err
		}
		out.Findings = append(out.Findings, f)
	}
	out.Raw = append([]byte(nil), data...)
	return out, nil
}

func decodeDecompositionManifest(data []byte) (DecompositionManifest, error) {
	var out DecompositionManifest
	obj, err := decodeReviewObject(data, "decomposition manifest")
	if err != nil {
		return out, err
	}
	names := []string{"schema_version", "session_id", "spec_review_manifest_path", "spec_review_manifest_sha256", "spec_path", "spec_sha256", "draft_path", "draft_sha256", "trace_path", "trace_sha256", "generated_at", "approved_at", "reviews", "dispositions"}
	if err := exactMembers(obj, "decomposition manifest", names); err != nil {
		return out, err
	}
	if err := schemaVersionOne(obj, "decomposition manifest"); err != nil {
		return out, err
	}
	if out.SessionID, err = reviewKeyMember(obj, "decomposition manifest", "session_id"); err != nil {
		return out, err
	}
	if out.SpecReviewManifestPath, err = reviewPathMember(obj, "decomposition manifest", "spec_review_manifest_path"); err != nil {
		return out, err
	}
	if out.SpecReviewManifestSHA256, err = reviewDigestMember(obj, "decomposition manifest", "spec_review_manifest_sha256"); err != nil {
		return out, err
	}
	if out.SpecPath, err = reviewPathMember(obj, "decomposition manifest", "spec_path"); err != nil {
		return out, err
	}
	if out.SpecSHA256, err = reviewDigestMember(obj, "decomposition manifest", "spec_sha256"); err != nil {
		return out, err
	}
	if out.DraftPath, err = fixedMember(obj, "decomposition manifest", "draft_path", "draft.json"); err != nil {
		return out, err
	}
	if out.DraftSHA256, err = reviewDigestMember(obj, "decomposition manifest", "draft_sha256"); err != nil {
		return out, err
	}
	if out.TracePath, err = fixedMember(obj, "decomposition manifest", "trace_path", "trace.json"); err != nil {
		return out, err
	}
	if out.TraceSHA256, err = reviewDigestMember(obj, "decomposition manifest", "trace_sha256"); err != nil {
		return out, err
	}
	if out.GeneratedAt, err = reviewTimeMember(obj, "decomposition manifest", "generated_at"); err != nil {
		return out, err
	}
	if out.ApprovedAt, err = reviewTimeMember(obj, "decomposition manifest", "approved_at"); err != nil {
		return out, err
	}
	reviews, err := arrayMember(obj["reviews"], "decomposition manifest", "reviews")
	if err != nil {
		return out, err
	}
	if len(reviews) < 1 || len(reviews) > 2 {
		return out, fmt.Errorf("decomposition manifest reviews must contain one or two passes")
	}
	for i, raw := range reviews {
		what := fmt.Sprintf("manifest review at index %d", i)
		o, err := strictObject(raw, what)
		if err != nil {
			return out, err
		}
		if err := exactMembers(o, what, []string{"pass_number", "path", "sha256", "context_mode", "spec_sha256", "draft_sha256", "trace_sha256"}); err != nil {
			return out, err
		}
		var r DecompositionManifestReview
		n, ok := decodeJSONInteger(o["pass_number"])
		if !ok || int(n) != i+1 {
			return out, fmt.Errorf("manifest reviews must be consecutive")
		}
		r.PassNumber = i + 1
		if r.Path, err = fixedMember(o, what, "path", fmt.Sprintf("review-%d.json", i+1)); err != nil {
			return out, err
		}
		if r.SHA256, err = reviewDigestMember(o, what, "sha256"); err != nil {
			return out, err
		}
		if r.ContextMode, err = enumMember(o, what, "context_mode", []string{"fresh", "same-context"}); err != nil {
			return out, err
		}
		if r.SpecSHA256, err = reviewDigestMember(o, what, "spec_sha256"); err != nil {
			return out, err
		}
		if r.DraftSHA256, err = reviewDigestMember(o, what, "draft_sha256"); err != nil {
			return out, err
		}
		if r.TraceSHA256, err = reviewDigestMember(o, what, "trace_sha256"); err != nil {
			return out, err
		}
		out.Reviews = append(out.Reviews, r)
	}
	dispositions, err := arrayMember(obj["dispositions"], "decomposition manifest", "dispositions")
	if err != nil {
		return out, err
	}
	seen := map[string]struct{}{}
	for i, raw := range dispositions {
		what := fmt.Sprintf("decomposition disposition at index %d", i)
		o, err := strictObject(raw, what)
		if err != nil {
			return out, err
		}
		if err := exactMembers(o, what, []string{"finding_id", "severity", "disposition", "rationale"}); err != nil {
			return out, err
		}
		var d DecompositionDisposition
		if d.FindingID, err = stringMember(o, what, "finding_id"); err != nil {
			return out, err
		}
		if _, ok := seen[d.FindingID]; ok {
			return out, fmt.Errorf("duplicate disposition finding_id %q", d.FindingID)
		}
		seen[d.FindingID] = struct{}{}
		if d.Severity, err = enumMember(o, what, "severity", []string{"high", "medium", "low"}); err != nil {
			return out, err
		}
		if d.Disposition, err = enumMember(o, what, "disposition", []string{"resolved", "rejected", "deferred"}); err != nil {
			return out, err
		}
		if d.Rationale, err = decompositionTextMember(o, what, "rationale"); err != nil {
			return out, err
		}
		out.Dispositions = append(out.Dispositions, d)
	}
	out.Raw = append([]byte(nil), data...)
	return out, nil
}

func validateDecompositionManifest(bundle DecompositionBundle, subjects DecompositionSubjects, findings map[string]DecompositionFinding) error {
	m := bundle.Manifest
	if len(m.Reviews) != len(bundle.Reviews) {
		return fmt.Errorf("manifest review membership does not match bundle")
	}
	if m.SessionID != bundle.Draft.ReviewSessionID || m.SessionID != bundle.Trace.SessionID {
		return fmt.Errorf("manifest session_id does not match draft and trace")
	}
	if m.SpecPath != subjects.SpecPath || m.SpecSHA256 != digestRaw(subjects.Spec) {
		return fmt.Errorf("manifest spec binding does not match selected spec exact bytes")
	}
	if m.SpecReviewManifestPath != subjects.SpecReviewManifestPath || m.SpecReviewManifestSHA256 != digestRaw(subjects.SpecReviewManifest) {
		return fmt.Errorf("manifest post-spec review binding does not match exact subject")
	}
	if m.DraftSHA256 != digestRaw(bundle.Draft.Raw) || m.TraceSHA256 != digestRaw(bundle.Trace.Raw) {
		return fmt.Errorf("manifest draft or trace digest does not match exact bytes")
	}
	for i, entry := range m.Reviews {
		review := bundle.Reviews[i]
		if entry.SHA256 != digestRaw(review.Raw) {
			return fmt.Errorf("manifest review %d digest does not match exact bytes", i+1)
		}
		if entry.ContextMode != review.ContextMode || entry.SpecSHA256 != review.SpecSHA256 || entry.DraftSHA256 != review.DraftSHA256 || entry.TraceSHA256 != review.TraceSHA256 {
			return fmt.Errorf("manifest review %d snapshot conflicts with review", i+1)
		}
		if review.SessionID != m.SessionID || review.SpecPath != m.SpecPath {
			return fmt.Errorf("review %d identity does not match manifest", i+1)
		}
		if review.SpecSHA256 != m.SpecSHA256 {
			return fmt.Errorf("review %d spec snapshot does not match selected spec", i+1)
		}
	}
	last := bundle.Reviews[len(bundle.Reviews)-1]
	if last.SpecSHA256 != m.SpecSHA256 || last.DraftSHA256 != m.DraftSHA256 || last.TraceSHA256 != m.TraceSHA256 {
		return fmt.Errorf("last review does not bind final exact spec, draft, and trace bytes")
	}
	for _, d := range m.Dispositions {
		finding, ok := findings[d.FindingID]
		if !ok {
			return fmt.Errorf("manifest disposition has unknown finding_id %q", d.FindingID)
		}
		if finding.Severity != d.Severity {
			return fmt.Errorf("disposition %q severity conflicts with finding", d.FindingID)
		}
		if d.Disposition == "deferred" && (d.Severity == "high" || d.Severity == "medium") {
			return fmt.Errorf("high or medium finding cannot be deferred: %q", d.FindingID)
		}
		delete(findings, d.FindingID)
	}
	missing := make([]string, 0, len(findings))
	for id := range findings {
		missing = append(missing, id)
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return fmt.Errorf("manifest is missing disposition for finding_id %q", missing[0])
	}
	return nil
}

func validateReviewedBody(body, what string) error {
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	lines := markdownLinesWithoutFencedContent(normalized)
	if len(lines) > 0 && lines[0] == "---" {
		return fmt.Errorf("%s body contains frontmatter", what)
	}
	for _, line := range lines {
		level, _ := markdownATXHeading(line)
		if level == 1 {
			return fmt.Errorf("%s body contains top-level heading", what)
		}
	}
	for _, heading := range []string{"## Description", "## Acceptance", "## Verification Notes"} {
		if countMarkdownHeading(lines, heading) != 1 {
			return fmt.Errorf("%s body must contain exactly one %s", what, heading)
		}
		start := indexMarkdownHeading(lines, heading) + 1
		end := len(lines)
		for i := start; i < len(lines); i++ {
			if level, _ := markdownATXHeading(lines[i]); level == 2 {
				end = i
				break
			}
		}
		if strings.TrimSpace(strings.Join(lines[start:end], "\n")) == "" {
			return fmt.Errorf("%s body section %s is empty", what, heading)
		}
	}
	if countMarkdownHeading(lines, "## Implementation Notes") > 1 {
		return fmt.Errorf("%s body repeats ## Implementation Notes", what)
	}
	return nil
}

func nonEmptyMember(obj map[string]json.RawMessage, what, name string) (string, error) {
	value, err := stringMember(obj, what, name)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s member %q must not be empty", what, name)
	}
	return value, nil
}
func rawString(raw json.RawMessage, what string) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s is not a string", what)
	}
	if value == "" {
		return "", fmt.Errorf("%s must not be empty", what)
	}
	return value, nil
}

func decompositionTextMember(obj map[string]json.RawMessage, what, name string) (string, error) {
	raw, ok := obj[name]
	if !ok {
		return "", fmt.Errorf("%s is missing member %q", what, name)
	}
	if isJSONNull(raw) {
		return "", fmt.Errorf("%s member %q is null", what, name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s member %q is not a string", what, name)
	}
	return value, nil
}

func digestRaw(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func countMarkdownHeading(lines []string, want string) int {
	count := 0
	_, wantText := markdownATXHeading(want)
	for _, line := range lines {
		level, text := markdownATXHeading(line)
		if level == 2 && text == wantText {
			count++
		}
	}
	return count
}

func indexMarkdownHeading(lines []string, want string) int {
	_, wantText := markdownATXHeading(want)
	for i, line := range lines {
		level, text := markdownATXHeading(line)
		if level == 2 && text == wantText {
			return i
		}
	}
	return -1
}

func markdownATXHeading(line string) (int, string) {
	indent := len(line) - len(strings.TrimLeft(line, " "))
	if indent > 3 {
		return 0, ""
	}
	line = line[indent:]
	level := 0
	for level < len(line) && level < 6 && line[level] == '#' {
		level++
	}
	if level == 0 || level == len(line) || (line[level] != ' ' && line[level] != '\t') {
		return 0, ""
	}
	text := strings.TrimSpace(line[level:])
	text = strings.TrimSpace(strings.TrimRight(text, "#"))
	return level, text
}
