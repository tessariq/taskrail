package taskrail

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"time"
)

const reviewFileLimit = 1 << 20

var (
	portableReviewKey   = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	reviewDigest        = regexp.MustCompile(`^[0-9a-f]{64}$`)
	specReviewLensOrder = []string{"consistency", "gaps", "additions", "adversarial"}
)

type ReviewPromptBinding struct {
	PromptID              string
	PromptContractVersion string
	PromptTemplateSHA256  string
	PromptSource          string
}

type TaskReview struct {
	Raw         []byte
	Binding     ReviewPromptBinding
	SessionID   string
	TaskID      string
	TaskPath    string
	TaskSHA256  string
	SpecPath    string
	SpecSHA256  string
	ContextMode string
	GeneratedAt string
	Findings    []TaskReviewFinding
}

type TaskReviewFinding struct {
	FindingID, Severity, Evidence, Impact, Recommendation, Disposition, Rationale string
}

type SpecReviewLens struct {
	Raw         []byte
	Binding     ReviewPromptBinding
	SessionID   string
	Lens        string
	SpecPath    string
	SpecSHA256  string
	ContextMode string
	GeneratedAt string
	Findings    []SpecReviewFinding
}

type SpecReviewFinding struct {
	FindingID, Severity, Evidence, Impact, Recommendation, Scope, Disposition, Rationale string
	TargetVersion                                                                        *string
}

type SpecReviewManifest struct {
	Raw          []byte
	SessionID    string
	SpecPath     string
	SpecSHA256   string
	GeneratedAt  string
	ApprovedAt   string
	Lenses       []SpecReviewManifestLens
	Dispositions []SpecReviewDisposition
}

type SpecReviewManifestLens struct{ Lens, Path, SHA256, SpecSHA256 string }

type SpecReviewDisposition struct {
	FindingID, Lens, Severity, Disposition, Rationale string
	ResultingSpecRef, TargetVersion                   *string
}

type SpecReviewBundle struct {
	Raw      map[string][]byte
	Lenses   []SpecReviewLens
	Manifest SpecReviewManifest
}

type specFindingBinding struct {
	Finding SpecReviewFinding
	Lens    string
}

func DecodeTaskReview(data []byte) (TaskReview, error) {
	var out TaskReview
	obj, err := decodeReviewObject(data, "task review")
	if err != nil {
		return out, err
	}
	names := []string{"schema_version", "prompt_id", "prompt_contract_version", "prompt_template_sha256", "prompt_source", "session_id", "task_id", "task_path", "task_sha256", "spec_path", "spec_sha256", "context_mode", "generated_at", "findings"}
	if err := exactMembers(obj, "task review", names); err != nil {
		return out, err
	}
	if err := schemaVersionOne(obj, "task review"); err != nil {
		return out, err
	}
	if out.Binding, err = decodePromptBinding(obj, "task review", "task-review"); err != nil {
		return out, err
	}
	if out.SessionID, err = reviewKeyMember(obj, "task review", "session_id"); err != nil {
		return out, err
	}
	if out.TaskID, err = stringMember(obj, "task review", "task_id"); err != nil {
		return out, err
	}
	if _, ok := taskIDPrefix(out.TaskID); !ok {
		return out, fmt.Errorf("task review member %q is not a full task ID", "task_id")
	}
	if out.TaskPath, err = reviewPathMember(obj, "task review", "task_path"); err != nil {
		return out, err
	}
	if out.TaskSHA256, err = reviewDigestMember(obj, "task review", "task_sha256"); err != nil {
		return out, err
	}
	if out.SpecPath, err = reviewPathMember(obj, "task review", "spec_path"); err != nil {
		return out, err
	}
	if out.SpecSHA256, err = reviewDigestMember(obj, "task review", "spec_sha256"); err != nil {
		return out, err
	}
	if out.ContextMode, err = enumMember(obj, "task review", "context_mode", []string{"fresh", "same-context"}); err != nil {
		return out, err
	}
	if out.GeneratedAt, err = reviewTimeMember(obj, "task review", "generated_at"); err != nil {
		return out, err
	}
	if out.Findings, err = decodeTaskFindings(obj["findings"]); err != nil {
		return out, err
	}
	out.Raw = append([]byte(nil), data...)
	return out, nil
}

func DecodeSpecReviewBundle(files map[string][]byte) (SpecReviewBundle, error) {
	var bundle SpecReviewBundle
	want := append([]string{}, specReviewLensOrder...)
	for i := range want {
		want[i] += ".json"
	}
	want = append(want, "manifest.json")
	for _, name := range want {
		if _, ok := files[name]; !ok {
			return bundle, fmt.Errorf("spec review bundle is missing file %q", name)
		}
	}
	for name := range files {
		if !slices.Contains(want, name) {
			return bundle, fmt.Errorf("spec review bundle has unknown file %q", name)
		}
	}
	bundle.Raw = make(map[string][]byte, len(files))
	findings := map[string]specFindingBinding{}
	for _, lensName := range specReviewLensOrder {
		path := lensName + ".json"
		lens, err := decodeSpecReviewLens(files[path], lensName)
		if err != nil {
			return bundle, fmt.Errorf("%s: %w", path, err)
		}
		if len(bundle.Lenses) > 0 {
			first := bundle.Lenses[0]
			if lens.SessionID != first.SessionID {
				return bundle, fmt.Errorf("%s session_id does not match other lenses", path)
			}
			if lens.SpecPath != first.SpecPath || lens.SpecSHA256 != first.SpecSHA256 {
				return bundle, fmt.Errorf("%s spec_path or spec_sha256 does not match other lenses", path)
			}
		}
		for _, finding := range lens.Findings {
			if _, exists := findings[finding.FindingID]; exists {
				return bundle, fmt.Errorf("duplicate finding_id %q across lenses", finding.FindingID)
			}
			findings[finding.FindingID] = specFindingBinding{Finding: finding, Lens: lensName}
		}
		bundle.Lenses = append(bundle.Lenses, lens)
		bundle.Raw[path] = append([]byte(nil), files[path]...)
	}
	manifest, err := decodeSpecReviewManifest(files["manifest.json"])
	if err != nil {
		return bundle, fmt.Errorf("manifest.json: %w", err)
	}
	if err := validateSpecManifest(manifest, bundle.Lenses, findings); err != nil {
		return bundle, err
	}
	bundle.Manifest = manifest
	bundle.Raw["manifest.json"] = append([]byte(nil), files["manifest.json"]...)
	return bundle, nil
}

func decodeReviewObject(data []byte, what string) (map[string]json.RawMessage, error) {
	if len(data) > reviewFileLimit {
		return nil, fmt.Errorf("%s exceeds 1 MiB", what)
	}
	if err := checkDocumentFraming(data); err != nil {
		return nil, err
	}
	return strictObject(data, what)
}

func schemaVersionOne(obj map[string]json.RawMessage, what string) error {
	raw, ok := obj["schema_version"]
	if !ok {
		return fmt.Errorf("%s is missing member %q", what, "schema_version")
	}
	version, ok := decodeJSONInteger(raw)
	if !ok {
		return fmt.Errorf("%s member %q is not an integer", what, "schema_version")
	}
	if version != 1 {
		return fmt.Errorf("%s has unsupported schema version %d", what, version)
	}
	return nil
}

func decodePromptBinding(obj map[string]json.RawMessage, what, promptID string) (ReviewPromptBinding, error) {
	var b ReviewPromptBinding
	var err error
	if b.PromptID, err = fixedMember(obj, what, "prompt_id", promptID); err != nil {
		return b, err
	}
	if b.PromptContractVersion, err = fixedMember(obj, what, "prompt_contract_version", "v1"); err != nil {
		return b, err
	}
	if b.PromptTemplateSHA256, err = reviewDigestMember(obj, what, "prompt_template_sha256"); err != nil {
		return b, err
	}
	if b.PromptSource, err = enumMember(obj, what, "prompt_source", []string{"builtin", "replacement"}); err != nil {
		return b, err
	}
	return b, nil
}

func decodeTaskFindings(raw json.RawMessage) ([]TaskReviewFinding, error) {
	elements, err := arrayMember(raw, "task review", "findings")
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	result := make([]TaskReviewFinding, 0, len(elements))
	for i, element := range elements {
		what := fmt.Sprintf("task review finding at index %d", i)
		obj, err := strictObject(element, what)
		if err != nil {
			return nil, err
		}
		if err := exactMembers(obj, what, []string{"finding_id", "severity", "evidence", "impact", "recommendation", "disposition", "rationale"}); err != nil {
			return nil, err
		}
		var f TaskReviewFinding
		if f.FindingID, err = stringMember(obj, what, "finding_id"); err != nil {
			return nil, err
		}
		if _, ok := seen[f.FindingID]; ok {
			return nil, fmt.Errorf("duplicate finding_id %q", f.FindingID)
		}
		seen[f.FindingID] = struct{}{}
		if f.Severity, err = enumMember(obj, what, "severity", []string{"high", "medium", "low"}); err != nil {
			return nil, err
		}
		if f.Evidence, err = stringMember(obj, what, "evidence"); err != nil {
			return nil, err
		}
		if f.Impact, err = stringMember(obj, what, "impact"); err != nil {
			return nil, err
		}
		if f.Recommendation, err = stringMember(obj, what, "recommendation"); err != nil {
			return nil, err
		}
		if f.Disposition, err = enumMember(obj, what, "disposition", []string{"open", "accepted", "rejected", "deferred"}); err != nil {
			return nil, err
		}
		if f.Rationale, err = stringMember(obj, what, "rationale"); err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, nil
}

func decodeSpecReviewLens(data []byte, lensName string) (SpecReviewLens, error) {
	var out SpecReviewLens
	obj, err := decodeReviewObject(data, "spec review lens")
	if err != nil {
		return out, err
	}
	names := []string{"schema_version", "prompt_id", "prompt_contract_version", "prompt_template_sha256", "prompt_source", "session_id", "lens", "spec_path", "spec_sha256", "context_mode", "generated_at", "findings"}
	if err := exactMembers(obj, "spec review lens", names); err != nil {
		return out, err
	}
	if err := schemaVersionOne(obj, "spec review lens"); err != nil {
		return out, err
	}
	if out.Binding, err = decodePromptBinding(obj, "spec review lens", "spec-"+lensName); err != nil {
		return out, err
	}
	if out.SessionID, err = reviewKeyMember(obj, "spec review lens", "session_id"); err != nil {
		return out, err
	}
	if out.Lens, err = fixedMember(obj, "spec review lens", "lens", lensName); err != nil {
		return out, err
	}
	if out.SpecPath, err = reviewPathMember(obj, "spec review lens", "spec_path"); err != nil {
		return out, err
	}
	if out.SpecSHA256, err = reviewDigestMember(obj, "spec review lens", "spec_sha256"); err != nil {
		return out, err
	}
	if out.ContextMode, err = enumMember(obj, "spec review lens", "context_mode", []string{"fresh", "same-context"}); err != nil {
		return out, err
	}
	if out.GeneratedAt, err = reviewTimeMember(obj, "spec review lens", "generated_at"); err != nil {
		return out, err
	}
	if out.Findings, err = decodeSpecFindings(obj["findings"]); err != nil {
		return out, err
	}
	out.Raw = append([]byte(nil), data...)
	return out, nil
}

func decodeSpecFindings(raw json.RawMessage) ([]SpecReviewFinding, error) {
	elements, err := arrayMember(raw, "spec review lens", "findings")
	if err != nil {
		return nil, err
	}
	result := make([]SpecReviewFinding, 0, len(elements))
	seen := map[string]struct{}{}
	for i, element := range elements {
		what := fmt.Sprintf("spec review finding at index %d", i)
		obj, err := strictObject(element, what)
		if err != nil {
			return nil, err
		}
		required := []string{"finding_id", "severity", "evidence", "impact", "recommendation", "scope", "disposition", "rationale"}
		if err := exactOptionalMembers(obj, what, required, []string{"target_version"}); err != nil {
			return nil, err
		}
		var f SpecReviewFinding
		if f.FindingID, err = stringMember(obj, what, "finding_id"); err != nil {
			return nil, err
		}
		if _, ok := seen[f.FindingID]; ok {
			return nil, fmt.Errorf("duplicate finding_id %q", f.FindingID)
		}
		seen[f.FindingID] = struct{}{}
		if f.Severity, err = enumMember(obj, what, "severity", []string{"high", "medium", "low"}); err != nil {
			return nil, err
		}
		if f.Evidence, err = stringMember(obj, what, "evidence"); err != nil {
			return nil, err
		}
		if f.Impact, err = stringMember(obj, what, "impact"); err != nil {
			return nil, err
		}
		if f.Recommendation, err = stringMember(obj, what, "recommendation"); err != nil {
			return nil, err
		}
		if f.Scope, err = enumMember(obj, what, "scope", []string{"current", "future", "reject"}); err != nil {
			return nil, err
		}
		if f.Disposition, err = fixedMember(obj, what, "disposition", "open"); err != nil {
			return nil, err
		}
		if f.Rationale, err = stringMember(obj, what, "rationale"); err != nil {
			return nil, err
		}
		if _, ok := obj["target_version"]; ok {
			value, err := stringMember(obj, what, "target_version")
			if err != nil {
				return nil, err
			}
			f.TargetVersion = &value
		}
		if (f.Scope == "future") != (f.TargetVersion != nil) {
			return nil, fmt.Errorf("%s target_version is required only for future scope", what)
		}
		result = append(result, f)
	}
	return result, nil
}

func decodeSpecReviewManifest(data []byte) (SpecReviewManifest, error) {
	var out SpecReviewManifest
	obj, err := decodeReviewObject(data, "spec review manifest")
	if err != nil {
		return out, err
	}
	if err := exactMembers(obj, "spec review manifest", []string{"schema_version", "session_id", "spec_path", "spec_sha256", "generated_at", "approved_at", "lenses", "dispositions"}); err != nil {
		return out, err
	}
	if err := schemaVersionOne(obj, "spec review manifest"); err != nil {
		return out, err
	}
	if out.SessionID, err = reviewKeyMember(obj, "spec review manifest", "session_id"); err != nil {
		return out, err
	}
	if out.SpecPath, err = reviewPathMember(obj, "spec review manifest", "spec_path"); err != nil {
		return out, err
	}
	if out.SpecSHA256, err = reviewDigestMember(obj, "spec review manifest", "spec_sha256"); err != nil {
		return out, err
	}
	if out.GeneratedAt, err = reviewTimeMember(obj, "spec review manifest", "generated_at"); err != nil {
		return out, err
	}
	if out.ApprovedAt, err = reviewTimeMember(obj, "spec review manifest", "approved_at"); err != nil {
		return out, err
	}
	if out.Lenses, err = decodeManifestLenses(obj["lenses"]); err != nil {
		return out, err
	}
	if out.Dispositions, err = decodeManifestDispositions(obj["dispositions"]); err != nil {
		return out, err
	}
	out.Raw = append([]byte(nil), data...)
	return out, nil
}

func decodeManifestLenses(raw json.RawMessage) ([]SpecReviewManifestLens, error) {
	elements, err := arrayMember(raw, "spec review manifest", "lenses")
	if err != nil {
		return nil, err
	}
	if len(elements) != len(specReviewLensOrder) {
		return nil, fmt.Errorf("manifest lenses must contain the four fixed lenses")
	}
	result := make([]SpecReviewManifestLens, 0, len(elements))
	for i, element := range elements {
		what := fmt.Sprintf("manifest lens at index %d", i)
		obj, err := strictObject(element, what)
		if err != nil {
			return nil, err
		}
		if err := exactMembers(obj, what, []string{"lens", "path", "sha256", "spec_sha256"}); err != nil {
			return nil, err
		}
		var lens SpecReviewManifestLens
		if lens.Lens, err = fixedMember(obj, what, "lens", specReviewLensOrder[i]); err != nil {
			return nil, fmt.Errorf("manifest lenses are not in fixed lens order: %w", err)
		}
		if lens.Path, err = fixedMember(obj, what, "path", lens.Lens+".json"); err != nil {
			return nil, err
		}
		if lens.SHA256, err = reviewDigestMember(obj, what, "sha256"); err != nil {
			return nil, err
		}
		if lens.SpecSHA256, err = reviewDigestMember(obj, what, "spec_sha256"); err != nil {
			return nil, err
		}
		result = append(result, lens)
	}
	return result, nil
}

func decodeManifestDispositions(raw json.RawMessage) ([]SpecReviewDisposition, error) {
	elements, err := arrayMember(raw, "spec review manifest", "dispositions")
	if err != nil {
		return nil, err
	}
	result := make([]SpecReviewDisposition, 0, len(elements))
	seen := map[string]struct{}{}
	for i, element := range elements {
		what := fmt.Sprintf("manifest disposition at index %d", i)
		obj, err := strictObject(element, what)
		if err != nil {
			return nil, err
		}
		required := []string{"finding_id", "lens", "severity", "disposition", "rationale"}
		if err := exactOptionalMembers(obj, what, required, []string{"resulting_spec_ref", "target_version"}); err != nil {
			return nil, err
		}
		var d SpecReviewDisposition
		if d.FindingID, err = stringMember(obj, what, "finding_id"); err != nil {
			return nil, err
		}
		if _, ok := seen[d.FindingID]; ok {
			return nil, fmt.Errorf("duplicate disposition finding_id %q", d.FindingID)
		}
		seen[d.FindingID] = struct{}{}
		if d.Lens, err = enumMember(obj, what, "lens", specReviewLensOrder); err != nil {
			return nil, err
		}
		if d.Severity, err = enumMember(obj, what, "severity", []string{"high", "medium", "low"}); err != nil {
			return nil, err
		}
		if d.Disposition, err = enumMember(obj, what, "disposition", []string{"accepted", "rejected", "deferred"}); err != nil {
			return nil, err
		}
		if d.Rationale, err = stringMember(obj, what, "rationale"); err != nil {
			return nil, err
		}
		if _, ok := obj["resulting_spec_ref"]; ok {
			value, err := stringMember(obj, what, "resulting_spec_ref")
			if err != nil {
				return nil, err
			}
			normalized, err := normalizeSpecRef(value)
			if err != nil || normalized != value {
				return nil, fmt.Errorf("%s resulting_spec_ref is not canonical", what)
			}
			d.ResultingSpecRef = &value
		}
		if _, ok := obj["target_version"]; ok {
			value, err := stringMember(obj, what, "target_version")
			if err != nil {
				return nil, err
			}
			d.TargetVersion = &value
		}
		if (d.Disposition == "accepted") != (d.ResultingSpecRef != nil) || (d.Disposition == "deferred") != (d.TargetVersion != nil) {
			return nil, fmt.Errorf("%s optional fields do not match disposition", what)
		}
		result = append(result, d)
	}
	return result, nil
}

func validateSpecManifest(manifest SpecReviewManifest, lenses []SpecReviewLens, findings map[string]specFindingBinding) error {
	first := lenses[0]
	if manifest.SessionID != first.SessionID {
		return fmt.Errorf("manifest session_id does not match lenses")
	}
	if manifest.SpecPath != first.SpecPath || manifest.SpecSHA256 != first.SpecSHA256 {
		return fmt.Errorf("manifest spec snapshot does not match lenses")
	}
	for i, entry := range manifest.Lenses {
		lens := lenses[i]
		if entry.SpecSHA256 != manifest.SpecSHA256 {
			return fmt.Errorf("manifest lens %q spec_sha256 does not match", entry.Lens)
		}
		sum := sha256.Sum256(lens.Raw)
		if entry.SHA256 != hex.EncodeToString(sum[:]) {
			return fmt.Errorf("manifest lens %q digest does not match exact bytes", entry.Lens)
		}
	}
	for _, disposition := range manifest.Dispositions {
		binding, ok := findings[disposition.FindingID]
		if !ok {
			return fmt.Errorf("manifest disposition has unknown finding_id %q", disposition.FindingID)
		}
		if binding.Finding.Severity != disposition.Severity {
			return fmt.Errorf("disposition %q severity conflicts with finding", disposition.FindingID)
		}
		if binding.Lens != disposition.Lens {
			return fmt.Errorf("disposition %q lens conflicts with finding", disposition.FindingID)
		}
		if disposition.Disposition == "deferred" && (binding.Finding.Severity == "high" || binding.Finding.Severity == "medium") {
			return fmt.Errorf("high or medium finding cannot be deferred: %q", disposition.FindingID)
		}
		delete(findings, disposition.FindingID)
	}
	for id := range findings {
		return fmt.Errorf("manifest is missing disposition for finding_id %q", id)
	}
	return nil
}

func reviewDigestMember(obj map[string]json.RawMessage, what, name string) (string, error) {
	value, err := stringMember(obj, what, name)
	if err != nil {
		return "", err
	}
	if !reviewDigest.MatchString(value) {
		return "", fmt.Errorf("%s member %q is not a lower-case 64-hex digest", what, name)
	}
	return value, nil
}

func reviewKeyMember(obj map[string]json.RawMessage, what, name string) (string, error) {
	value, err := stringMember(obj, what, name)
	if err != nil {
		return "", err
	}
	if len(value) > 128 || !portableReviewKey.MatchString(value) {
		return "", fmt.Errorf("%s member %q is not a portable review key", what, name)
	}
	return value, nil
}

func reviewPathMember(obj map[string]json.RawMessage, what, name string) (string, error) {
	value, err := stringMember(obj, what, name)
	if err != nil {
		return "", err
	}
	if absolutePathStart.MatchString(value) || !canonicalPathSegments(value) {
		return "", fmt.Errorf("%s member %q is not a canonical repository-relative path", what, name)
	}
	return value, nil
}

func reviewTimeMember(obj map[string]json.RawMessage, what, name string) (string, error) {
	value, err := stringMember(obj, what, name)
	if err != nil {
		return "", err
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil || parsed.UTC().Format(time.RFC3339) != value {
		return "", fmt.Errorf("%s member %q is not canonical RFC3339 UTC", what, name)
	}
	return value, nil
}

func exactOptionalMembers(obj map[string]json.RawMessage, what string, required, optional []string) error {
	for _, name := range required {
		if _, ok := obj[name]; !ok {
			return fmt.Errorf("%s is missing member %q", what, name)
		}
	}
	for name := range obj {
		if !slices.Contains(required, name) && !slices.Contains(optional, name) {
			return fmt.Errorf("%s has unknown member %q", what, name)
		}
	}
	return nil
}
