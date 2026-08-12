package taskrail

import (
	"fmt"
	"strings"
	"testing"
)

func TestDecodeDecompositionBundlePreservesCompleteValidBundle(t *testing.T) {
	files, subjects := decompositionGolden()
	bundle, err := DecodeDecompositionBundle(files, subjects)
	if err != nil {
		t.Fatalf("DecodeDecompositionBundle: %v", err)
	}
	if len(bundle.Reviews) != 1 || len(bundle.Draft.Tasks) != 2 || len(bundle.Trace.Requirements) != 2 {
		t.Fatalf("unexpected bundle: %#v", bundle)
	}
	for name, raw := range files {
		if string(bundle.Raw[name]) != string(raw) {
			t.Fatalf("%s bytes were not preserved", name)
		}
	}
}

func TestDecodeDecompositionBundleRejectsStrictMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string][]byte, *DecompositionSubjects)
		want   string
	}{
		{"missing file", func(f map[string][]byte, _ *DecompositionSubjects) { delete(f, "trace.json") }, "missing file"},
		{"extra file", func(f map[string][]byte, _ *DecompositionSubjects) { f["notes.json"] = []byte(`{}`) }, "unknown file"},
		{"v1 draft", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "draft.json", `"schema_version":2`, `"schema_version":1`)
		}, "must be 2"},
		{"draft source", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "draft.json", `"target":"tasks"`, `"source":"notes.md","target":"tasks"`)
		}, "forbids source"},
		{"draft key", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "draft.json", `"key":"first"`, `"key":"First_Key"`)
		}, "portable review key"},
		{"duplicate draft key", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "draft.json", `"key":"second"`, `"key":"first"`)
		}, "duplicate task key"},
		{"draft anchor", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "draft.json", `#second-area`, `#missing-area`)
		}, "anchor does not exist"},
		{"duplicate dependency", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "draft.json", `"dependencies":["first",`, `"dependencies":["first","first",`)
		}, "repeats dependency"},
		{"cycle", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "draft.json", `"dependencies":[]`, `"dependencies":["second"]`)
		}, "dependency cycle"},
		{"body missing section", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "draft.json", `## Acceptance\n\n- Works.\n\n`, ``)
		}, "exactly one ## Acceptance"},
		{"trace session", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "trace.json", `"session_id":"decomposition-1"`, `"session_id":"other-session"`)
		}, "session identities do not match"},
		{"trace anchor", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "trace.json", `#second-area`, `#missing-area`)
		}, "anchor does not exist"},
		{"repeated quote", func(f map[string][]byte, s *DecompositionSubjects) {
			oldDigest := digestRaw(s.Spec)
			s.Spec = append(s.Spec, []byte("\nUnique requirement text.\n")...)
			newDigest := digestRaw(s.Spec)
			replaceDecomposition(f, "trace.json", oldDigest, newDigest)
			s.SpecReviewManifest = []byte(strings.ReplaceAll(string(s.SpecReviewManifest), oldDigest, newDigest))
		}, "occur exactly once"},
		{"line range", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "trace.json", `"start":7,"end":9`, `"start":7,"end":99`)
		}, "out of bounds"},
		{"unknown trace key", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "trace.json", `"task_keys":["second"]`, `"task_keys":["missing"]`)
		}, "unknown task key"},
		{"uncovered draft key", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "trace.json", `"task_keys":["second"]`, `"task_keys":["first"]`)
		}, "not covered by trace"},
		{"wrong review role", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "review-1.json", `"prompt_id":"task-decomposition-adversarial"`, `"prompt_id":"task-review"`)
		}, `must be "task-decomposition-adversarial"`},
		{"wrong pass", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "review-1.json", `"pass_number":1`, `"pass_number":2`)
		}, "pass_number must be 1"},
		{"duplicate finding", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "review-1.json", `]}`, `,{"finding_id":"finding-1","severity":"medium","evidence":"e","impact":"i","recommendation":"r"}]}`)
		}, "duplicate finding_id"},
		{"stale manifest draft", func(f map[string][]byte, _ *DecompositionSubjects) { f["draft.json"] = append(f["draft.json"], ' ') }, "draft or trace digest"},
		{"post-final draft change", func(f map[string][]byte, _ *DecompositionSubjects) {
			f["draft.json"] = append(f["draft.json"], ' ')
			replaceManifestDigest(f, "draft_sha256", digestRaw(f["draft.json"]), 1)
		}, "last review does not bind final"},
		{"post-spec review digest", func(_ map[string][]byte, s *DecompositionSubjects) {
			s.SpecReviewManifest = append(s.SpecReviewManifest, ' ')
		}, "post-spec review binding"},
		{"manifest review digest", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "manifest.json", digestRaw(f["review-1.json"]), reviewDigestA)
		}, "review 1 digest"},
		{"unknown disposition", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "manifest.json", `"finding_id":"finding-1"`, `"finding_id":"unknown"`)
		}, "unknown finding_id"},
		{"missing disposition", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "manifest.json", `{"finding_id":"finding-1","severity":"medium","disposition":"resolved","rationale":"fixed"}`, ``)
		}, "missing disposition"},
		{"deferred medium", func(f map[string][]byte, _ *DecompositionSubjects) {
			replaceDecomposition(f, "manifest.json", `"disposition":"resolved"`, `"disposition":"deferred"`)
		}, "cannot be deferred"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			files, subjects := decompositionGolden()
			tc.mutate(files, &subjects)
			_, err := DecodeDecompositionBundle(files, subjects)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestDecodeDecompositionBundleAcceptsTwoConsecutivePasses(t *testing.T) {
	files, subjects := decompositionGolden()
	addSecondDecompositionPass(files, subjects)
	if _, err := DecodeDecompositionBundle(files, subjects); err != nil {
		t.Fatalf("DecodeDecompositionBundle two passes: %v", err)
	}
}

func TestDecodeDecompositionBundleAcceptsEmptyReviewText(t *testing.T) {
	files, subjects := decompositionGolden()
	for _, field := range []string{"evidence", "impact", "recommendation"} {
		replaceDecomposition(files, "review-1.json", `"`+field+`":"`+field+`"`, `"`+field+`":""`)
	}
	replaceDecomposition(files, "manifest.json", `"rationale":"fixed"`, `"rationale":""`)
	replaceManifestDigest(files, "sha256", digestRaw(files["review-1.json"]), 1)
	if _, err := DecodeDecompositionBundle(files, subjects); err != nil {
		t.Fatalf("DecodeDecompositionBundle empty review text: %v", err)
	}
}

func TestDecodeDecompositionBundleValidatesReviewedBodyMarkdownStructure(t *testing.T) {
	t.Run("headings in fence do not count", func(t *testing.T) {
		files, subjects := decompositionGolden()
		replaceDecomposition(files, "draft.json", `## Description\n\nDeliver one outcome.\n\n## Acceptance\n\n- Works.\n\n## Verification Notes\n\n- Test it.`, "```markdown\\n## Description\\n\\nExample.\\n\\n## Acceptance\\n\\nExample.\\n\\n## Verification Notes\\n\\nExample.\\n```")
		refreshDecompositionFinalDigests(files)
		_, err := DecodeDecompositionBundle(files, subjects)
		if err == nil || !strings.Contains(err.Error(), "exactly one ## Description") {
			t.Fatalf("error = %v, want missing structural Description", err)
		}
	})
	t.Run("fenced h1 and thematic break are content", func(t *testing.T) {
		files, subjects := decompositionGolden()
		replaceDecomposition(files, "draft.json", `Deliver one outcome.`, "---\\n\\nDeliver one outcome.\\n\\n```markdown\\n# Example\\n```")
		refreshDecompositionFinalDigests(files)
		if _, err := DecodeDecompositionBundle(files, subjects); err != nil {
			t.Fatalf("DecodeDecompositionBundle fenced content: %v", err)
		}
	})
}

func TestDecodeDecompositionBundleRejectsFirstPassSpecDrift(t *testing.T) {
	files, subjects := decompositionGolden()
	addSecondDecompositionPass(files, subjects)
	replaceDecomposition(files, "review-1.json", digestRaw(subjects.Spec), reviewDigestA)
	replaceManifestDigest(files, "sha256", digestRaw(files["review-1.json"]), 1)
	replaceManifestDigest(files, "spec_sha256", reviewDigestA, 2)
	_, err := DecodeDecompositionBundle(files, subjects)
	if err == nil || !strings.Contains(err.Error(), "review 1 spec snapshot") {
		t.Fatalf("error = %v, want first-pass spec snapshot rejection", err)
	}
}

func TestDecodeDecompositionBundleReportsMissingDispositionsDeterministically(t *testing.T) {
	files, subjects := decompositionGolden()
	addSecondDecompositionPass(files, subjects)
	replaceDecomposition(files, "manifest.json", `{"finding_id":"finding-1","severity":"medium","disposition":"resolved","rationale":"fixed"},{"finding_id":"finding-2","severity":"medium","disposition":"rejected","rationale":"not applicable"}`, ``)
	for range 20 {
		_, err := DecodeDecompositionBundle(files, subjects)
		if err == nil || !strings.Contains(err.Error(), `finding_id "finding-1"`) {
			t.Fatalf("error = %v, want deterministic first missing finding", err)
		}
	}
}

func TestDecodeDecompositionBundleAcceptsNonPortableRequirementAndFindingIDs(t *testing.T) {
	files, subjects := decompositionGolden()
	replaceDecomposition(files, "trace.json", `"requirement_id":"requirement-1"`, `"requirement_id":"Requirement_1"`)
	replaceDecomposition(files, "review-1.json", `"finding_id":"finding-1"`, `"finding_id":"Finding_1"`)
	replaceDecomposition(files, "manifest.json", `"finding_id":"finding-1"`, `"finding_id":"Finding_1"`)
	refreshDecompositionFinalDigests(files)
	if _, err := DecodeDecompositionBundle(files, subjects); err != nil {
		t.Fatalf("DecodeDecompositionBundle non-portable IDs: %v", err)
	}
}

func TestDecodeDecompositionBundleRejectsMarkdownEquivalentBodyHeadings(t *testing.T) {
	tests := []struct{ name, suffix, want string }{
		{"indented h1", `\n\n   # Extra title`, "top-level heading"},
		{"tab h1", `\n\n#\tExtra title`, "top-level heading"},
		{"trailing-space duplicate", `\n\n## Description   \n\nDuplicate.`, "exactly one ## Description"},
		{"closing-hash duplicate", `\n\n## Description ##\n\nDuplicate.`, "exactly one ## Description"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			files, subjects := decompositionGolden()
			replaceDecomposition(files, "draft.json", `- Test it.`, `- Test it.`+tc.suffix)
			refreshDecompositionFinalDigests(files)
			_, err := DecodeDecompositionBundle(files, subjects)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func decompositionGolden() (map[string][]byte, DecompositionSubjects) {
	spec := []byte("# Spec\n\n### First Area\n\nUnique requirement text.\n\n### Second Area\n\nAnother requirement.\n")
	specSum := digestRaw(spec)
	specReviewFiles := specReviewGolden()
	specReview := []byte(strings.ReplaceAll(string(specReviewFiles["manifest.json"]), reviewDigestA, specSum))
	specReviewSum := digestRaw(specReview)
	body := "## Description\\n\\nDeliver one outcome.\\n\\n## Acceptance\\n\\n- Works.\\n\\n## Verification Notes\\n\\n- Test it."
	draft := []byte(`{"schema_version":2,"review_session_id":"decomposition-1","target":"tasks","tasks":[{"key":"first","title":"First task","dependencies":[],"body":"` + body + `","spec_ref":"specs/v0.5.0.md#first-area","priority":"high"},{"key":"second","title":"Second task","dependencies":["first","T-240-implement-the-normative-review-schema-decoders"],"body":"` + body + `","spec_ref":"specs/v0.5.0.md#second-area"}],"spec_sections":[]}`)
	trace := []byte(`{"schema_version":1,"session_id":"decomposition-1","spec_path":"specs/v0.5.0.md","spec_sha256":"` + specSum + `","requirements":[{"requirement_id":"requirement-1","spec_ref":"specs/v0.5.0.md#first-area","source":{"kind":"quote","text":"Unique requirement text."},"task_keys":["first"],"disposition":"task","rationale":"implemented"},{"requirement_id":"requirement-2","spec_ref":"specs/v0.5.0.md#second-area","source":{"kind":"lines","start":7,"end":9},"task_keys":["second"],"disposition":"task","rationale":"implemented"}]}`)
	review := []byte(`{"schema_version":1,"prompt_id":"task-decomposition-adversarial","prompt_contract_version":"v1","prompt_template_sha256":"` + reviewDigestB + `","prompt_source":"builtin","session_id":"decomposition-1","pass_number":1,"spec_path":"specs/v0.5.0.md","spec_sha256":"` + specSum + `","draft_path":"draft.json","draft_sha256":"` + digestRaw(draft) + `","trace_path":"trace.json","trace_sha256":"` + digestRaw(trace) + `","context_mode":"fresh","generated_at":"2026-08-12T10:00:00Z","findings":[{"finding_id":"finding-1","severity":"medium","evidence":"evidence","impact":"impact","recommendation":"recommendation"}]}`)
	manifest := []byte(fmt.Sprintf(`{"schema_version":1,"session_id":"decomposition-1","spec_review_manifest_path":"planning/reviews/spec/v0.5.0/spec-review-1/manifest.json","spec_review_manifest_sha256":"%s","spec_path":"specs/v0.5.0.md","spec_sha256":"%s","draft_path":"draft.json","draft_sha256":"%s","trace_path":"trace.json","trace_sha256":"%s","generated_at":"2026-08-12T10:30:00Z","approved_at":"2026-08-12T11:00:00Z","reviews":[{"pass_number":1,"path":"review-1.json","sha256":"%s","context_mode":"fresh","spec_sha256":"%s","draft_sha256":"%s","trace_sha256":"%s"}],"dispositions":[{"finding_id":"finding-1","severity":"medium","disposition":"resolved","rationale":"fixed"}]}`,
		specReviewSum, specSum, digestRaw(draft), digestRaw(trace), digestRaw(review), specSum, digestRaw(draft), digestRaw(trace)))
	return map[string][]byte{"draft.json": draft, "trace.json": trace, "review-1.json": review, "manifest.json": manifest}, DecompositionSubjects{
		SpecPath: "specs/v0.5.0.md", Spec: spec,
		SpecReviewManifestPath: "planning/reviews/spec/v0.5.0/spec-review-1/manifest.json", SpecReviewManifest: specReview,
	}
}

func replaceManifestDigest(files map[string][]byte, field, digest string, occurrence int) {
	manifest := string(files["manifest.json"])
	marker := `"` + field + `":"`
	start := 0
	for range occurrence {
		i := strings.Index(manifest[start:], marker)
		start += i + len(marker)
	}
	files["manifest.json"] = []byte(manifest[:start] + digest + manifest[start+64:])
}

func addSecondDecompositionPass(files map[string][]byte, subjects DecompositionSubjects) {
	review2 := []byte(strings.Replace(string(files["review-1.json"]), `"pass_number":1`, `"pass_number":2`, 1))
	review2 = []byte(strings.Replace(string(review2), `"finding_id":"finding-1"`, `"finding_id":"finding-2"`, 1))
	files["review-2.json"] = review2
	entry := fmt.Sprintf(`,{"pass_number":2,"path":"review-2.json","sha256":"%s","context_mode":"fresh","spec_sha256":"%s","draft_sha256":"%s","trace_sha256":"%s"}`, digestRaw(review2), digestRaw(subjects.Spec), digestRaw(files["draft.json"]), digestRaw(files["trace.json"]))
	replaceDecomposition(files, "manifest.json", `}],"dispositions"`, `}`+entry+`],"dispositions"`)
	replaceDecomposition(files, "manifest.json", `]}`, `,{"finding_id":"finding-2","severity":"medium","disposition":"rejected","rationale":"not applicable"}]}`)
}

func refreshDecompositionFinalDigests(files map[string][]byte) {
	draftDigest := digestRaw(files["draft.json"])
	traceDigest := digestRaw(files["trace.json"])
	replaceManifestDigest(files, "draft_sha256", draftDigest, 1)
	replaceManifestDigest(files, "draft_sha256", draftDigest, 2)
	review := string(files["review-1.json"])
	for _, binding := range []struct{ field, digest string }{{"draft_sha256", draftDigest}, {"trace_sha256", traceDigest}} {
		marker := `"` + binding.field + `":"`
		start := strings.Index(review, marker) + len(marker)
		review = review[:start] + binding.digest + review[start+64:]
	}
	files["review-1.json"] = []byte(review)
	replaceManifestDigest(files, "trace_sha256", traceDigest, 1)
	replaceManifestDigest(files, "trace_sha256", traceDigest, 2)
	replaceManifestDigest(files, "sha256", digestRaw(files["review-1.json"]), 1)
}

func replaceDecomposition(files map[string][]byte, name, old, new string) {
	files[name] = []byte(strings.Replace(string(files[name]), old, new, 1))
}
