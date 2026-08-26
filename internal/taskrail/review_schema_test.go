package taskrail

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

const (
	reviewDigestA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	reviewDigestB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestDecodeTaskReviewPreservesAcceptedBytes(t *testing.T) {
	raw := taskReviewGolden()
	review, err := DecodeTaskReview(raw)
	if err != nil {
		t.Fatalf("DecodeTaskReview: %v", err)
	}
	if string(review.Raw) != string(raw) {
		t.Fatal("accepted bytes were not preserved")
	}
	if review.TaskID != "T-240-example" || len(review.Findings) != 1 {
		t.Fatalf("unexpected review: %#v", review)
	}
}

func TestDecodeTaskReviewRejectsStrictMutations(t *testing.T) {
	base := string(taskReviewGolden())
	tests := []struct {
		name string
		data string
		want string
	}{
		{"unknown", strings.Replace(base, `"session_id"`, `"unknown":true,"session_id"`, 1), "unknown member"},
		{"missing", strings.Replace(base, `"task_path":"planning/tasks/T-240-example.md",`, "", 1), "missing member"},
		{"duplicate", strings.Replace(base, `"session_id":"review-1"`, `"session_id":"review-1","session_id":"review-2"`, 1), "repeats member"},
		{"null", strings.Replace(base, `"context_mode":"fresh"`, `"context_mode":null`, 1), "is null"},
		{"trailing", base + `{}`, "trailing value"},
		{"version", strings.Replace(base, `"schema_version":1`, `"schema_version":2`, 1), "unsupported schema version"},
		{"role", strings.Replace(base, `"prompt_id":"task-review"`, `"prompt_id":"spec-gaps"`, 1), `must be "task-review"`},
		{"contract", strings.Replace(base, `"prompt_contract_version":"v1"`, `"prompt_contract_version":"v2"`, 1), `must be "v1"`},
		{"digest", strings.Replace(base, reviewDigestA, strings.ToUpper(reviewDigestA), 1), "lower-case 64-hex"},
		{"path", strings.Replace(base, `planning/tasks/T-240-example.md`, `planning//tasks/T-240-example.md`, 1), "canonical repository-relative path"},
		{"windows absolute path", strings.Replace(base, `specs/v0.5.0.md`, `C:/repo/specs/v0.5.0.md`, 1), "canonical repository-relative path"},
		{"time", strings.Replace(base, `2026-08-12T10:00:00Z`, `2026-08-12T10:00:00+00:00`, 1), "canonical RFC3339 UTC"},
		{"session", strings.Replace(base, `"session_id":"review-1"`, `"session_id":"Review_1"`, 1), "portable review key"},
		{"duplicate finding", strings.Replace(base, `]}`, `,{"finding_id":"finding-1","severity":"low","evidence":"e","impact":"i","recommendation":"r","disposition":"open","rationale":"why"}]}`, 1), "duplicate finding_id"},
		{"enum", strings.Replace(base, `"severity":"high"`, `"severity":"urgent"`, 1), "allowed value"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeTaskReview([]byte(tc.data))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
	oversize := append(taskReviewGolden(), make([]byte, reviewFileLimit+1)...)
	if _, err := DecodeTaskReview(oversize); err == nil || !strings.Contains(err.Error(), "exceeds 1 MiB") {
		t.Fatalf("oversize error = %v", err)
	}
}

func TestDecodeSpecReviewBundleValidatesLensesAndManifest(t *testing.T) {
	files := specReviewGolden()
	bundle, err := DecodeSpecReviewBundle(files)
	if err != nil {
		t.Fatalf("DecodeSpecReviewBundle: %v", err)
	}
	if len(bundle.Lenses) != 4 || len(bundle.Manifest.Dispositions) != 4 {
		t.Fatalf("unexpected bundle: %#v", bundle)
	}
	if string(bundle.Raw["gaps.json"]) != string(files["gaps.json"]) {
		t.Fatal("accepted lens bytes were not preserved")
	}
	if string(bundle.Manifest.Raw) != string(files["manifest.json"]) || string(bundle.Raw["manifest.json"]) != string(files["manifest.json"]) {
		t.Fatal("accepted manifest bytes were not preserved")
	}
}

func TestDecodeSpecReviewBundleRejectsBindingAndDispositionMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string][]byte)
		want   string
	}{
		{"missing file", func(f map[string][]byte) { delete(f, "gaps.json") }, "missing file"},
		{"unknown file", func(f map[string][]byte) { f["notes.json"] = []byte(`{}`) }, "unknown file"},
		{"wrong role", func(f map[string][]byte) {
			f["gaps.json"] = []byte(strings.Replace(string(f["gaps.json"]), `"prompt_id":"spec-gaps"`, `"prompt_id":"spec-consistency"`, 1))
		}, `must be "spec-gaps"`},
		{"lens trailing", func(f map[string][]byte) { f["gaps.json"] = append(f["gaps.json"], []byte(`{}`)...) }, "trailing value"},
		{"lens duplicate", func(f map[string][]byte) {
			f["gaps.json"] = []byte(strings.Replace(string(f["gaps.json"]), `"lens":"gaps"`, `"lens":"gaps","lens":"gaps"`, 1))
		}, "repeats member"},
		{"lens version", func(f map[string][]byte) {
			f["gaps.json"] = []byte(strings.Replace(string(f["gaps.json"]), `"schema_version":1`, `"schema_version":2`, 1))
		}, "unsupported schema version"},
		{"lens disposition", func(f map[string][]byte) {
			f["gaps.json"] = []byte(strings.Replace(string(f["gaps.json"]), `"disposition":"open"`, `"disposition":"accepted"`, 1))
		}, `must be "open"`},
		{"future target missing", func(f map[string][]byte) {
			f["gaps.json"] = []byte(strings.Replace(string(f["gaps.json"]), `"scope":"current"`, `"scope":"future"`, 1))
		}, "target_version is required"},
		{"current target forbidden", func(f map[string][]byte) {
			f["gaps.json"] = []byte(strings.Replace(string(f["gaps.json"]), `"disposition":"open"`, `"disposition":"open","target_version":"v0.6.0"`, 1))
		}, "target_version is required only"},
		{"lens nested unknown", func(f map[string][]byte) {
			f["gaps.json"] = []byte(strings.Replace(string(f["gaps.json"]), `"evidence":"evidence"`, `"extra":true,"evidence":"evidence"`, 1))
		}, "unknown member"},
		{"lens oversize", func(f map[string][]byte) { f["gaps.json"] = append(f["gaps.json"], make([]byte, reviewFileLimit+1)...) }, "exceeds 1 MiB"},
		{"snapshot conflict", func(f map[string][]byte) {
			f["gaps.json"] = []byte(strings.Replace(string(f["gaps.json"]), reviewDigestA, reviewDigestB, 1))
			refreshManifestDigest(f, "gaps.json")
		}, "spec_sha256 does not match"},
		{"session conflict", func(f map[string][]byte) {
			f["gaps.json"] = []byte(strings.Replace(string(f["gaps.json"]), `"session_id":"spec-review-1"`, `"session_id":"spec-review-2"`, 1))
			refreshManifestDigest(f, "gaps.json")
		}, "session_id does not match"},
		{"cross-lens finding", func(f map[string][]byte) {
			f["gaps.json"] = []byte(strings.Replace(string(f["gaps.json"]), `"finding_id":"gaps-1"`, `"finding_id":"consistency-1"`, 1))
			refreshManifestDigest(f, "gaps.json")
		}, "duplicate finding_id"},
		{"lens order", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `"lens":"consistency"`, `"lens":"gaps"`, 1))
		}, "fixed lens order"},
		{"manifest unknown", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `"approved_at"`, `"unknown":true,"approved_at"`, 1))
		}, "unknown member"},
		{"manifest path", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `"path":"gaps.json"`, `"path":"other.json"`, 1))
		}, `must be "gaps.json"`},
		{"manifest snapshot conflict", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `"spec_sha256":"`+reviewDigestA+`"`, `"spec_sha256":"`+reviewDigestB+`"`, 1))
		}, "snapshot does not match"},
		{"duplicate disposition", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `"finding_id":"gaps-1"`, `"finding_id":"consistency-1"`, 1))
		}, "duplicate disposition"},
		{"severity conflict", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `"finding_id":"gaps-1","lens":"gaps","severity":"medium"`, `"finding_id":"gaps-1","lens":"gaps","severity":"low"`, 1))
		}, "severity conflicts"},
		{"lens conflict", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `"finding_id":"gaps-1","lens":"gaps"`, `"finding_id":"gaps-1","lens":"additions"`, 1))
		}, "lens conflicts"},
		{"digest mismatch", func(f map[string][]byte) { f["gaps.json"] = append(f["gaps.json"], ' ') }, "digest does not match"},
		{"missing disposition", func(f map[string][]byte) {
			s := string(f["manifest.json"])
			start := strings.Index(s, `,{"finding_id":"gaps-1"`)
			end := start + strings.Index(s[start+1:], `}`) + 2
			f["manifest.json"] = []byte(s[:start] + s[end:])
		}, "missing disposition"},
		{"unknown disposition", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `"finding_id":"gaps-1"`, `"finding_id":"unknown-1"`, 1))
		}, "unknown finding_id"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			files := specReviewGolden()
			tc.mutate(files)
			_, err := DecodeSpecReviewBundle(files)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestDecodeSpecReviewBundleAllowsDeferredHighOrMediumFinding(t *testing.T) {
	files := specReviewGolden()
	files["manifest.json"] = []byte(strings.Replace(string(files["manifest.json"]),
		`"disposition":"accepted","rationale":"fixed","resulting_spec_ref":"specs/v0.5.0.md#safe-review-artifact-publication"`,
		`"disposition":"deferred","rationale":"later","target_version":"v0.6.0"`, 1))
	if _, err := DecodeSpecReviewBundle(files); err != nil {
		t.Fatalf("deferred high finding: %v", err)
	}
}

func taskReviewGolden() []byte {
	return []byte(`{"schema_version":1,"prompt_id":"task-review","prompt_contract_version":"v1","prompt_template_sha256":"` + reviewDigestB + `","prompt_source":"builtin","session_id":"review-1","task_id":"T-240-example","task_path":"planning/tasks/T-240-example.md","task_sha256":"` + reviewDigestA + `","spec_path":"specs/v0.5.0.md","spec_sha256":"` + reviewDigestB + `","context_mode":"fresh","generated_at":"2026-08-12T10:00:00Z","findings":[{"finding_id":"finding-1","severity":"high","evidence":"evidence","impact":"impact","recommendation":"recommendation","disposition":"open","rationale":"rationale"}]}`)
}

func specReviewGolden() map[string][]byte {
	files := map[string][]byte{}
	for _, lens := range specReviewLensOrder {
		files[lens+".json"] = []byte(`{"schema_version":1,"prompt_id":"spec-` + lens + `","prompt_contract_version":"v1","prompt_template_sha256":"` + reviewDigestB + `","prompt_source":"builtin","session_id":"spec-review-1","lens":"` + lens + `","spec_path":"specs/v0.5.0.md","spec_sha256":"` + reviewDigestA + `","context_mode":"fresh","generated_at":"2026-08-12T10:00:00Z","findings":[{"finding_id":"` + lens + `-1","severity":"` + map[string]string{"consistency": "high", "gaps": "medium", "additions": "low", "adversarial": "low"}[lens] + `","evidence":"evidence","impact":"impact","recommendation":"recommendation","scope":"current","disposition":"open","rationale":"rationale"}]}`)
	}
	entries := make([]string, 0, 4)
	dispositions := make([]string, 0, 4)
	for _, lens := range specReviewLensOrder {
		sum := sha256.Sum256(files[lens+".json"])
		entries = append(entries, `{"lens":"`+lens+`","path":"`+lens+`.json","sha256":"`+hex.EncodeToString(sum[:])+`","spec_sha256":"`+reviewDigestA+`"}`)
		severity := map[string]string{"consistency": "high", "gaps": "medium", "additions": "low", "adversarial": "low"}[lens]
		dispositions = append(dispositions, `{"finding_id":"`+lens+`-1","lens":"`+lens+`","severity":"`+severity+`","disposition":"rejected","rationale":"not applicable"}`)
	}
	// Make one accepted decision exercise the required resulting spec reference.
	dispositions[0] = `{"finding_id":"consistency-1","lens":"consistency","severity":"high","disposition":"accepted","rationale":"fixed","resulting_spec_ref":"specs/v0.5.0.md#safe-review-artifact-publication"}`
	files["manifest.json"] = []byte(`{"schema_version":1,"session_id":"spec-review-1","spec_path":"specs/v0.5.0.md","spec_sha256":"` + reviewDigestA + `","generated_at":"2026-08-12T10:00:00Z","approved_at":"2026-08-12T11:00:00Z","lenses":[` + strings.Join(entries, ",") + `],"dispositions":[` + strings.Join(dispositions, ",") + `]}`)
	return files
}

func refreshManifestDigest(files map[string][]byte, path string) {
	sum := sha256.Sum256(files[path])
	manifest := string(files["manifest.json"])
	lens := strings.TrimSuffix(path, ".json")
	marker := `"lens":"` + lens + `","path":"` + path + `","sha256":"`
	start := strings.Index(manifest, marker) + len(marker)
	files["manifest.json"] = []byte(manifest[:start] + hex.EncodeToString(sum[:]) + manifest[start+64:])
}
