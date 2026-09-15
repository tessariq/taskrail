package taskrail

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"
)

const reviewDigestC = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

func TestDecodeSpecReviewRejectsOverflowingIdentities(t *testing.T) {
	for _, tc := range []struct{ name, file, old, replacement string }{
		{"manifest schema", "manifest.json", `"schema_version":2`, `"schema_version":4294967298`},
		{"lens schema", "round-1-consistency.json", `"schema_version":1`, `"schema_version":4294967297`},
		{"round", "manifest.json", `"round":1,"spec_sha256"`, `"round":4294967297,"spec_sha256"`},
		{"disposition round", "manifest.json", `"round":1,"finding_id"`, `"round":4294967297,"finding_id"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := specReview2Golden()
			files[tc.file] = []byte(strings.Replace(string(files[tc.file]), tc.old, tc.replacement, 1))
			if tc.file != "manifest.json" {
				refreshRoundLensDigest(files, tc.file)
			}
			if _, err := decodeSpecReviewProposalBundle(files); err == nil {
				t.Fatal("accepted overflowing identity")
			}
		})
	}
}

// TestDecodeSpecReviewSchema2BundleDeclaresRoundsAndUnverifiedDispositions
// covers the schema-2 history bundle: an ordered declared round history, the
// fixed provenance label marking dispositions as unverified caller-recorded
// claims, and disposition finding references that distinguish occurrences of
// the same finding_id across rounds.
func TestDecodeSpecReviewSchema2BundleDeclaresRoundsAndUnverifiedDispositions(t *testing.T) {
	files := specReview2Golden()
	for _, decode := range []struct {
		name string
		call func(map[string][]byte) (SpecReviewBundle, error)
	}{
		{"historical read", DecodeSpecReviewBundle},
		{"proposal", decodeSpecReviewProposalBundle},
	} {
		t.Run(decode.name, func(t *testing.T) {
			bundle, err := decode.call(files)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if bundle.Manifest.SchemaVersion != 2 || bundle.Manifest.DispositionProvenance != specReviewProvenanceLabel {
				t.Fatalf("manifest schema/provenance = %d/%q", bundle.Manifest.SchemaVersion, bundle.Manifest.DispositionProvenance)
			}
			if bundle.Manifest.ApprovedAt != "" || len(bundle.Manifest.Lenses) != 0 {
				t.Fatalf("schema 2 acquired schema-1 approval members: %+v", bundle.Manifest)
			}
			if len(bundle.Manifest.Rounds) != 2 {
				t.Fatalf("declared rounds = %d, want 2", len(bundle.Manifest.Rounds))
			}
			first, second := bundle.Manifest.Rounds[0], bundle.Manifest.Rounds[1]
			if first.Round != 1 || first.SpecSHA256 != reviewDigestA || first.RepeatsEarlierSpec {
				t.Fatalf("round 1 = %+v", first)
			}
			if second.Round != 2 || second.SpecSHA256 != reviewDigestC || second.RepeatsEarlierSpec {
				t.Fatalf("round 2 = %+v", second)
			}
			if second.Lenses[0].Path != "round-2-consistency.json" {
				t.Fatalf("round 2 lens path = %q", second.Lenses[0].Path)
			}
			if len(bundle.Lenses) != 8 {
				t.Fatalf("flattened lens observations = %d, want 8", len(bundle.Lenses))
			}
			if len(bundle.Manifest.Dispositions) != 6 {
				t.Fatalf("dispositions = %d, want 6", len(bundle.Manifest.Dispositions))
			}
			if bundle.Manifest.Dispositions[0].Round != 1 || bundle.Manifest.Dispositions[4].Round != 2 {
				t.Fatalf("occurrence rounds = %d and %d, want 1 and 2", bundle.Manifest.Dispositions[0].Round, bundle.Manifest.Dispositions[4].Round)
			}
			if string(bundle.Raw["round-1-gaps.json"]) != string(files["round-1-gaps.json"]) ||
				string(bundle.Raw["manifest.json"]) != string(files["manifest.json"]) || string(bundle.Manifest.Raw) != string(files["manifest.json"]) {
				t.Fatal("accepted bytes were not preserved")
			}
		})
	}
}

// TestDecodeSpecReviewSchema2BundleLabelsUnchangedByteRepeats asserts the two
// refusal directions of repeat visibility: a round that repeats an earlier spec
// digest cannot hide it, and a round over fresh bytes cannot claim a repeat.
func TestDecodeSpecReviewSchema2BundleLabelsUnchangedByteRepeats(t *testing.T) {
	repeat := specReview2RepeatGolden()
	bundle, err := DecodeSpecReviewBundle(repeat)
	if err != nil {
		t.Fatalf("disclosed repeat bundle: %v", err)
	}
	if !bundle.Manifest.Rounds[1].RepeatsEarlierSpec || bundle.Manifest.Rounds[1].SpecSHA256 != reviewDigestA {
		t.Fatalf("repeat round = %+v", bundle.Manifest.Rounds[1])
	}

	hidden := []byte(strings.Replace(string(specReview2RepeatGolden()["manifest.json"]),
		`"round":2,"spec_sha256":"`+reviewDigestA+`","repeats_earlier_spec":true`,
		`"round":2,"spec_sha256":"`+reviewDigestA+`","repeats_earlier_spec":false`, 1))
	files := specReview2RepeatGolden()
	files["manifest.json"] = hidden
	if _, err := decodeSpecReviewProposalBundle(files); err == nil || !strings.Contains(err.Error(), "round 2 repeats an earlier spec digest") {
		t.Fatalf("hidden repeat error = %v, want round 2 repeats an earlier spec digest", err)
	}

	invented := []byte(strings.Replace(string(specReview2Golden()["manifest.json"]),
		`"round":2,"spec_sha256":"`+reviewDigestC+`","repeats_earlier_spec":false`,
		`"round":2,"spec_sha256":"`+reviewDigestC+`","repeats_earlier_spec":true`, 1))
	files = specReview2Golden()
	files["manifest.json"] = invented
	if _, err := decodeSpecReviewProposalBundle(files); err == nil || !strings.Contains(err.Error(), "round 2 declares an unchanged-byte repeat") {
		t.Fatalf("invented repeat error = %v, want round 2 declares an unchanged-byte repeat", err)
	}
}

func TestDecodeSpecReviewSchema2BundleRejectsMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string][]byte)
		want   string
	}{
		{"manifest unknown member", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `"disposition_provenance"`, `"approved_at":"2026-08-12T11:00:00Z","disposition_provenance"`, 1))
		}, "unknown member"},
		{"wrong provenance", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `"`+specReviewProvenanceLabel+`"`, `"human-approved"`, 1))
		}, specReviewProvenanceLabel},
		{"unsupported version", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `"schema_version":2`, `"schema_version":3`, 1))
		}, "unsupported schema version"},
		{"empty rounds", func(f map[string][]byte) {
			s := string(f["manifest.json"])
			start := strings.Index(s, `"rounds":[`) + len(`"rounds":[`)
			end := strings.Index(s, `],"dispositions"`)
			f["manifest.json"] = []byte(s[:start] + s[end:])
		}, "at least one round"},
		{"nonconsecutive rounds", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `"round":2,"spec_sha256"`, `"round":3,"spec_sha256"`, 1))
		}, "consecutive"},
		{"wrong round path", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `"path":"round-2-consistency.json"`, `"path":"consistency.json"`, 1))
		}, `must be "round-2-consistency.json"`},
		{"renamed path member", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `{"lens":"consistency","path":"round-1-consistency.json"`, `{"lens":"consistency","filename":"round-1-consistency.json"`, 1))
		}, `missing member "path"`},
		{"round lens order", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `{"lens":"consistency","path":"round-1-consistency.json"`, `{"lens":"gaps","path":"round-1-consistency.json"`, 1))
		}, "fixed lens order"},
		{"unlisted file", func(f map[string][]byte) { f["round-3-gaps.json"] = []byte(`{}`) }, "unknown file"},
		{"missing declared file", func(f map[string][]byte) { delete(f, "round-2-gaps.json") }, "missing file"},
		{"round digest conflict", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `"round":1,"spec_sha256":"`+reviewDigestA+`"`, `"round":1,"spec_sha256":"`+reviewDigestB+`"`, 1))
		}, "spec_sha256 does not match"},
		{"entry digest mismatch", func(f map[string][]byte) { f["round-2-additions.json"] = append(f["round-2-additions.json"], ' ') }, "digest does not match"},
		{"final digest mismatch", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `"spec_sha256":"`+reviewDigestC+`","generated_at"`, `"spec_sha256":"`+reviewDigestB+`","generated_at"`, 1))
		}, "final round spec digest does not match the manifest"},
		{"session conflict", func(f map[string][]byte) {
			f["round-2-gaps.json"] = []byte(strings.Replace(string(f["round-2-gaps.json"]), `"session_id":"spec-review-1"`, `"session_id":"spec-review-2"`, 1))
			refreshRoundLensDigest(f, "round-2-gaps.json")
		}, "session_id does not match"},
		{"lens spec path conflict", func(f map[string][]byte) {
			f["round-2-gaps.json"] = []byte(strings.Replace(string(f["round-2-gaps.json"]), `"spec_path":"specs/v0.5.0.md"`, `"spec_path":"specs/v0.6.0.md"`, 1))
			refreshRoundLensDigest(f, "round-2-gaps.json")
		}, "spec_path does not match"},
		{"disposition missing round", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `{"round":1,"finding_id":"GAPS-001"`, `{"finding_id":"GAPS-001"`, 1))
		}, "missing member"},
		{"disposition undeclared round", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `{"round":1,"finding_id":"GAPS-001"`, `{"round":3,"finding_id":"GAPS-001"`, 1))
		}, "is not a declared round"},
		{"disposition lens conflict", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `{"round":1,"finding_id":"GAPS-001","lens":"gaps"`, `{"round":1,"finding_id":"GAPS-001","lens":"additions"`, 1))
		}, "lens conflicts"},
		{"disposition severity conflict", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `{"round":1,"finding_id":"GAPS-001","lens":"gaps","severity":"medium"`, `{"round":1,"finding_id":"GAPS-001","lens":"gaps","severity":"low"`, 1))
		}, "severity conflicts"},
		{"duplicate occurrence disposition", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]),
				`{"round":2,"finding_id":"GAPS-002","lens":"gaps","severity":"low","disposition":"rejected","rationale":"covered elsewhere"}`,
				`{"round":1,"finding_id":"GAPS-001","lens":"gaps","severity":"medium","disposition":"rejected","rationale":"duplicate"},{"round":2,"finding_id":"GAPS-002","lens":"gaps","severity":"low","disposition":"rejected","rationale":"covered elsewhere"}`, 1))
		}, "duplicate disposition"},
		{"missing retained disposition", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `,{"round":2,"finding_id":"GAPS-002","lens":"gaps","severity":"low","disposition":"rejected","rationale":"covered elsewhere"}`, "", 1))
		}, "missing disposition"},
		{"unknown finding occurrence", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `{"round":2,"finding_id":"GAPS-002"`, `{"round":2,"finding_id":"GAPS-003"`, 1))
		}, "unknown finding"},
		{"deferred high finding", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `{"round":1,"finding_id":"CONS-001","lens":"consistency","severity":"high","disposition":"accepted","rationale":"fixed","resulting_spec_ref":"specs/v0.5.0.md#safe-review-artifact-publication"}`, `{"round":1,"finding_id":"CONS-001","lens":"consistency","severity":"high","disposition":"deferred","rationale":"later","target_version":"v0.6.0"}`, 1))
		}, "defers high or medium finding"},
		{"accepted without resulting reference", func(f map[string][]byte) {
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `{"round":2,"finding_id":"GAPS-002","lens":"gaps","severity":"low","disposition":"rejected","rationale":"covered elsewhere"}`, `{"round":2,"finding_id":"GAPS-002","lens":"gaps","severity":"low","disposition":"accepted","rationale":"fixed"}`, 1))
		}, "optional fields do not match disposition"},
		{"cross-lens namespace", func(f map[string][]byte) {
			f["round-2-gaps.json"] = []byte(strings.Replace(string(f["round-2-gaps.json"]), `"finding_id":"GAPS-002"`, `"finding_id":"CONS-002"`, 1))
			f["manifest.json"] = []byte(strings.Replace(string(f["manifest.json"]), `{"round":2,"finding_id":"GAPS-002"`, `{"round":2,"finding_id":"CONS-002"`, 1))
			refreshRoundLensDigest(f, "round-2-gaps.json")
		}, "namespace"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			files := specReview2Golden()
			tc.mutate(files)
			_, err := decodeSpecReviewProposalBundle(files)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

// TestDecodeSpecReviewSchema2ReadStillAcceptsLegacySchema1 asserts the split
// between publication and historical reads: a legacy five-file bundle no
// longer publishes, but it remains readable exactly as before.
func TestDecodeSpecReviewSchema2ReadStillAcceptsLegacySchema1(t *testing.T) {
	files := specReviewGolden()
	if _, err := decodeSpecReviewProposalBundle(files); err == nil || !strings.Contains(err.Error(), "legacy schema-1 spec review bundles no longer publish") {
		t.Fatalf("legacy proposal error = %v, want legacy schema-1 refusal", err)
	}
	if _, err := DecodeSpecReviewBundle(files); err != nil {
		t.Fatalf("legacy historical read: %v", err)
	}
}

func buildSpecReview2Golden(finalDigest string, repeat bool) map[string][]byte {
	files := map[string][]byte{}
	severities := map[string]string{"consistency": "high", "gaps": "medium", "additions": "low", "adversarial": "low"}
	roundDigests := map[int]string{1: reviewDigestA, 2: finalDigest}
	roundFindings := map[int]map[string]string{
		1: {"consistency": "CONS-001", "gaps": "GAPS-001", "additions": "ADDS-001", "adversarial": "ADV-001"},
		2: {"consistency": "CONS-001", "gaps": "GAPS-002"},
	}
	roundSeverities := map[string]string{"consistency": "high", "gaps": "low"}
	rounds := make([]string, 0, 2)
	for round := 1; round <= 2; round++ {
		digest := roundDigests[round]
		entries := make([]string, 0, 4)
		for _, lens := range specReviewLensOrder {
			name := specReview2LensPath(round, lens)
			findingID, has := roundFindings[round][lens]
			severity := severities[lens]
			if round == 2 {
				severity = roundSeverities[lens]
			}
			findings := "[]"
			if has {
				findings = `[{"finding_id":"` + findingID + `","severity":"` + severity + `","evidence":"evidence","impact":"impact","recommendation":"recommendation","scope":"current","disposition":"open","rationale":"rationale"}]`
			}
			files[name] = []byte(`{"schema_version":1,"prompt_id":"spec-` + lens + `","prompt_contract_version":"v1","prompt_template_sha256":"` + reviewDigestB + `","prompt_source":"builtin","session_id":"spec-review-1","lens":"` + lens + `","spec_path":"specs/v0.5.0.md","spec_sha256":"` + digest + `","context_mode":"fresh","generated_at":"2026-08-12T10:00:00Z","findings":` + findings + `}`)
			sum := sha256.Sum256(files[name])
			entries = append(entries, `{"lens":"`+lens+`","path":"`+name+`","sha256":"`+hex.EncodeToString(sum[:])+`","spec_sha256":"`+digest+`"}`)
		}
		repeatLabel := repeat && round == 2
		rounds = append(rounds, `{"round":`+strconv.Itoa(round)+`,"spec_sha256":"`+digest+`","repeats_earlier_spec":`+strconv.FormatBool(repeatLabel)+`,"lenses":[`+strings.Join(entries, ",")+`]}`)
	}
	dispositions := []string{
		`{"round":1,"finding_id":"CONS-001","lens":"consistency","severity":"high","disposition":"accepted","rationale":"fixed","resulting_spec_ref":"specs/v0.5.0.md#safe-review-artifact-publication"}`,
		`{"round":1,"finding_id":"GAPS-001","lens":"gaps","severity":"medium","disposition":"rejected","rationale":"not applicable"}`,
		`{"round":1,"finding_id":"ADDS-001","lens":"additions","severity":"low","disposition":"deferred","rationale":"later","target_version":"v0.6.0"}`,
		`{"round":1,"finding_id":"ADV-001","lens":"adversarial","severity":"low","disposition":"rejected","rationale":"not applicable"}`,
		`{"round":2,"finding_id":"CONS-001","lens":"consistency","severity":"high","disposition":"rejected","rationale":"still open but out of scope"}`,
		`{"round":2,"finding_id":"GAPS-002","lens":"gaps","severity":"low","disposition":"rejected","rationale":"covered elsewhere"}`,
	}
	files["manifest.json"] = []byte(`{"schema_version":2,"session_id":"spec-review-1","spec_path":"specs/v0.5.0.md","spec_sha256":"` + finalDigest + `","generated_at":"2026-08-12T10:00:00Z","disposition_provenance":"` + specReviewProvenanceLabel + `","rounds":[` + strings.Join(rounds, ",") + `],"dispositions":[` + strings.Join(dispositions, ",") + `]}`)
	return files
}

func specReview2Golden() map[string][]byte       { return buildSpecReview2Golden(reviewDigestC, false) }
func specReview2RepeatGolden() map[string][]byte { return buildSpecReview2Golden(reviewDigestA, true) }

func refreshRoundLensDigest(files map[string][]byte, path string) {
	sum := sha256.Sum256(files[path])
	manifest := string(files["manifest.json"])
	marker := `"path":"` + path + `","sha256":"`
	start := strings.Index(manifest, marker) + len(marker)
	files["manifest.json"] = []byte(manifest[:start] + hex.EncodeToString(sum[:]) + manifest[start+64:])
}
