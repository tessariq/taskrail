package taskrail

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// SpecReviewRound is one declared round of a schema-2 spec-review history. The
// round number is its identity: rounds are consecutive from 1, the last
// declared round is final, and RepeatsEarlierSpec is the visible label an
// unchanged-byte repeat must carry.
type SpecReviewRound struct {
	Round              int
	SpecSHA256         string
	RepeatsEarlierSpec bool
	Lenses             []SpecReviewManifestLens
}

// specReviewProvenanceLabel is the only value a schema-2 manifest's
// disposition_provenance member may carry, so every published history bundle
// states on its face that dispositions are unverified caller/agent-recorded
// claims rather than authenticated human approval.
const specReviewProvenanceLabel = "unverified-caller-recorded-claims"

// specReviewOccurrence names one finding occurrence: the same finding_id in
// two rounds is two occurrences, and a disposition must say which one it
// decides.
type specReviewOccurrence struct {
	Round     int
	FindingID string
}

// specReview2LensPath is the fixed bundle basename of one lens observation.
func specReview2LensPath(round int, lens string) string {
	return fmt.Sprintf("round-%d-%s.json", round, lens)
}

// decodeSpecReview2Bundle decodes and cross-validates one schema-2 history
// bundle: every declared round keeps its four immutable lens observations with
// exact file and spec-digest bindings, every repeat of an earlier spec digest
// is visibly labeled, dispositions carry the unverified caller-recorded-claims
// provenance and cover every retained finding occurrence exactly once, and the
// final round binds the manifest's final spec digest.
func decodeSpecReview2Bundle(files map[string][]byte) (SpecReviewBundle, error) {
	var bundle SpecReviewBundle
	manifest, err := decodeSpecReview2Manifest(files["manifest.json"])
	if err != nil {
		return bundle, fmt.Errorf("manifest.json: %w", err)
	}
	want := []string{"manifest.json"}
	for _, round := range manifest.Rounds {
		for _, entry := range round.Lenses {
			want = append(want, entry.Path)
		}
	}
	if err := requireSpecReviewFiles(files, want); err != nil {
		return bundle, err
	}
	bundle.Raw = make(map[string][]byte, len(files))
	findings := map[specReviewOccurrence]specFindingBinding{}
	for _, round := range manifest.Rounds {
		for i, entry := range round.Lenses {
			lensName := specReviewLensOrder[i]
			lens, err := decodeSpecReviewLens(files[entry.Path], lensName, true)
			if err != nil {
				return bundle, fmt.Errorf("%s: %w", entry.Path, err)
			}
			if lens.SessionID != manifest.SessionID {
				return bundle, fmt.Errorf("%s session_id does not match manifest", entry.Path)
			}
			if lens.SpecPath != manifest.SpecPath {
				return bundle, fmt.Errorf("%s spec_path does not match manifest", entry.Path)
			}
			if lens.SpecSHA256 != round.SpecSHA256 || entry.SpecSHA256 != round.SpecSHA256 {
				return bundle, fmt.Errorf("%s spec_sha256 does not match its declared round", entry.Path)
			}
			sum := sha256.Sum256(lens.Raw)
			if entry.SHA256 != hex.EncodeToString(sum[:]) {
				return bundle, fmt.Errorf("%s digest does not match exact bytes", entry.Path)
			}
			for _, finding := range lens.Findings {
				occurrence := specReviewOccurrence{Round: round.Round, FindingID: finding.FindingID}
				if _, exists := findings[occurrence]; exists {
					return bundle, fmt.Errorf("duplicate finding_id %q in round %d", finding.FindingID, round.Round)
				}
				findings[occurrence] = specFindingBinding{Finding: finding, Lens: lensName}
			}
			bundle.Lenses = append(bundle.Lenses, lens)
			bundle.Raw[entry.Path] = append([]byte(nil), files[entry.Path]...)
		}
	}
	if err := validateSpecReview2Rounds(manifest); err != nil {
		return bundle, err
	}
	if err := validateSpecReview2Dispositions(manifest, findings); err != nil {
		return bundle, err
	}
	bundle.Manifest = manifest
	bundle.Raw["manifest.json"] = append([]byte(nil), files["manifest.json"]...)
	return bundle, nil
}

func decodeSpecReview2Manifest(data []byte) (SpecReviewManifest, error) {
	var out SpecReviewManifest
	obj, err := decodeReviewObject(data, "spec review manifest")
	if err != nil {
		return out, err
	}
	if err := exactMembers(obj, "spec review manifest", []string{
		"schema_version", "session_id", "spec_path", "spec_sha256", "generated_at",
		"disposition_provenance", "rounds", "dispositions",
	}); err != nil {
		return out, err
	}
	if err := schemaVersion(obj, "spec review manifest", 2); err != nil {
		return out, err
	}
	out.SchemaVersion = 2
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
	if out.DispositionProvenance, err = fixedMember(obj, "spec review manifest", "disposition_provenance", specReviewProvenanceLabel); err != nil {
		return out, err
	}
	if out.Rounds, err = decodeSpecReview2Rounds(obj["rounds"]); err != nil {
		return out, err
	}
	if out.Dispositions, err = decodeManifestDispositions(obj["dispositions"], true); err != nil {
		return out, err
	}
	out.Raw = append([]byte(nil), data...)
	return out, nil
}

func decodeSpecReview2Rounds(raw json.RawMessage) ([]SpecReviewRound, error) {
	elements, err := arrayMember(raw, "spec review manifest", "rounds")
	if err != nil {
		return nil, err
	}
	if len(elements) == 0 {
		return nil, fmt.Errorf("spec review manifest rounds must declare at least one round")
	}
	rounds := make([]SpecReviewRound, 0, len(elements))
	for i, element := range elements {
		what := fmt.Sprintf("manifest round at index %d", i)
		obj, err := strictObject(element, what)
		if err != nil {
			return nil, err
		}
		if err := exactMembers(obj, what, []string{"round", "spec_sha256", "repeats_earlier_spec", "lenses"}); err != nil {
			return nil, err
		}
		var round SpecReviewRound
		number, ok := decodeJSONInteger(obj["round"])
		if !ok || number != int64(i+1) {
			return nil, fmt.Errorf("manifest rounds must be consecutive from 1")
		}
		round.Round = int(number)
		if round.SpecSHA256, err = reviewDigestMember(obj, what, "spec_sha256"); err != nil {
			return nil, err
		}
		if round.RepeatsEarlierSpec, err = boolMember(obj, what, "repeats_earlier_spec"); err != nil {
			return nil, err
		}
		if round.Lenses, err = decodeSpecReview2RoundLenses(obj["lenses"], round.Round); err != nil {
			return nil, err
		}
		rounds = append(rounds, round)
	}
	return rounds, nil
}

func decodeSpecReview2RoundLenses(raw json.RawMessage, round int) ([]SpecReviewManifestLens, error) {
	elements, err := arrayMember(raw, "spec review manifest", "round lenses")
	if err != nil {
		return nil, err
	}
	if len(elements) != len(specReviewLensOrder) {
		return nil, fmt.Errorf("manifest round lenses must contain the four fixed lenses")
	}
	result := make([]SpecReviewManifestLens, 0, len(elements))
	for i, element := range elements {
		what := fmt.Sprintf("manifest round lens at index %d", i)
		obj, err := strictObject(element, what)
		if err != nil {
			return nil, err
		}
		if err := exactMembers(obj, what, []string{"lens", "path", "sha256", "spec_sha256"}); err != nil {
			return nil, err
		}
		var lens SpecReviewManifestLens
		if lens.Lens, err = fixedMember(obj, what, "lens", specReviewLensOrder[i]); err != nil {
			return nil, fmt.Errorf("manifest round lenses are not in fixed lens order: %w", err)
		}
		if lens.Path, err = fixedMember(obj, what, "path", specReview2LensPath(round, lens.Lens)); err != nil {
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

// validateSpecReview2Rounds refuses a repeat of an earlier spec digest that is
// not visibly labeled and an invented repeat label over fresh bytes, then binds
// the manifest's final digest to the last declared round.
func validateSpecReview2Rounds(manifest SpecReviewManifest) error {
	seen := map[string]struct{}{}
	for _, round := range manifest.Rounds {
		_, repeats := seen[round.SpecSHA256]
		seen[round.SpecSHA256] = struct{}{}
		if repeats && !round.RepeatsEarlierSpec {
			return fmt.Errorf("round %d repeats an earlier spec digest but is not labeled as a repeat", round.Round)
		}
		if !repeats && round.RepeatsEarlierSpec {
			return fmt.Errorf("round %d declares an unchanged-byte repeat without repeating an earlier spec digest", round.Round)
		}
	}
	last := manifest.Rounds[len(manifest.Rounds)-1]
	if last.SpecSHA256 != manifest.SpecSHA256 {
		return fmt.Errorf("final round spec digest does not match the manifest")
	}
	return nil
}

func validateSpecReview2Dispositions(manifest SpecReviewManifest, findings map[specReviewOccurrence]specFindingBinding) error {
	declared := make(map[int]struct{}, len(manifest.Rounds))
	for _, round := range manifest.Rounds {
		declared[round.Round] = struct{}{}
	}
	seen := map[specReviewOccurrence]struct{}{}
	for i := range manifest.Dispositions {
		disposition := manifest.Dispositions[i]
		if _, ok := declared[disposition.Round]; !ok {
			return fmt.Errorf("manifest disposition round %d is not a declared round", disposition.Round)
		}
		occurrence := specReviewOccurrence{Round: disposition.Round, FindingID: disposition.FindingID}
		if _, ok := seen[occurrence]; ok {
			return fmt.Errorf("duplicate disposition for finding_id %q in round %d", disposition.FindingID, disposition.Round)
		}
		seen[occurrence] = struct{}{}
		binding, ok := findings[occurrence]
		if !ok {
			return fmt.Errorf("manifest disposition has unknown finding_id %q in round %d", disposition.FindingID, disposition.Round)
		}
		if binding.Finding.Severity != disposition.Severity {
			return fmt.Errorf("disposition %q severity conflicts with finding", disposition.FindingID)
		}
		if binding.Lens != disposition.Lens {
			return fmt.Errorf("disposition %q lens conflicts with finding", disposition.FindingID)
		}
		if disposition.Disposition == "deferred" && (disposition.Severity == "high" || disposition.Severity == "medium") {
			return fmt.Errorf("manifest defers high or medium finding %q in round %d", disposition.FindingID, disposition.Round)
		}
		delete(findings, occurrence)
	}
	if len(findings) == 0 {
		return nil
	}
	missing := firstSpecReviewOccurrence(findings)
	return fmt.Errorf("manifest is missing disposition for finding_id %q in round %d", missing.FindingID, missing.Round)
}

func firstSpecReviewOccurrence(findings map[specReviewOccurrence]specFindingBinding) specReviewOccurrence {
	var first *specReviewOccurrence
	for occurrence := range findings {
		if first == nil || occurrence.Round < first.Round || occurrence.Round == first.Round && occurrence.FindingID < first.FindingID {
			candidate := occurrence
			first = &candidate
		}
	}
	return *first
}

func specReviewManifestSchemaVersion(files map[string][]byte) (int64, error) {
	data, ok := files["manifest.json"]
	if !ok {
		return 0, fmt.Errorf("spec review bundle is missing file %q", "manifest.json")
	}
	obj, err := decodeReviewObject(data, "spec review manifest")
	if err != nil {
		return 0, err
	}
	return schemaVersionValue(obj, "spec review manifest")
}
