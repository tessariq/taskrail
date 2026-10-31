package taskrail

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillEvalStableSymlinkedAnchor(t *testing.T) {
	in := skillEvalTestInput(t, skillEvalTestAdapter{})
	link := filepath.Join(t.TempDir(), "artifacts")
	if err := os.Symlink(in.ArtifactRoot, link); err != nil {
		t.Fatal(err)
	}
	in.ArtifactRoot = link
	stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	data, err := RenderSkillEvalStage(stage)
	if err != nil || bytes.Contains(data, []byte(link)) {
		t.Fatalf("rendered stage disclosed path or failed: %v", err)
	}
	decoded, err := DecodeSkillEvalStage(data, in)
	if err != nil {
		t.Fatal(err)
	}
	in.AdoptedBaselineCases = []string{"existing"}
	in.AdoptedBaselineStage = &decoded
	adopted, err := (SkillEvalRunner{}).Execute(context.Background(), in)
	if err != nil || *adopted.Report.Cases[0].Baseline != *stage.Report.Cases[0].Baseline {
		t.Fatalf("stable linked adoption = %+v, %v", adopted, err)
	}
	report, err := (SkillEvalRunner{}).Resume(adopted, in, in.HumanReview, in.CaseReviews)
	if err != nil || report.Outcome != "pass" {
		t.Fatalf("stable linked resume = %+v, %v", report, err)
	}
}

func TestSkillEvalLegacyStageResumeWithoutRepinning(t *testing.T) {
	in := skillEvalTestInput(t, skillEvalTestAdapter{})
	stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	// Construct the pre-extension schema and seal independently of the current
	// renderer, so omitempty changes cannot silently invalidate old stages.
	payload := struct {
		SchemaVersion int                      `json:"schema_version"`
		Report        SkillEvalReport          `json:"report"`
		Worksheet     []SkillEvalWorksheetCase `json:"worksheet"`
	}{1, stage.Report, stage.Worksheet}
	compact, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(append([]byte("taskrail-skill-eval-stage-v1\x00"), compact...))
	legacy := struct {
		SchemaVersion int                      `json:"schema_version"`
		Report        SkillEvalReport          `json:"report"`
		Worksheet     []SkillEvalWorksheetCase `json:"worksheet"`
		Seal          string                   `json:"seal"`
	}{1, stage.Report, stage.Worksheet, hex.EncodeToString(digest[:])}
	data, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	decoded, err := DecodeSkillEvalStage(data, in)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := RenderSkillEvalStage(decoded)
	if err != nil || !bytes.Equal(data, rendered) {
		t.Fatalf("legacy stage was re-pinned or re-encoded: %v", err)
	}
	for _, extra := range []string{`"artifact_anchor": null,`, `"unknown": 1,`} {
		noncanonical := strings.Replace(string(data), `"seal":`, extra+`"seal":`, 1)
		if _, err := DecodeSkillEvalStage([]byte(noncanonical), in); err == nil || !strings.Contains(err.Error(), "canonical") {
			t.Fatalf("legacy decoder accepted noncanonical member %s: %v", extra, err)
		}
	}
	report, err := (SkillEvalRunner{}).Resume(decoded, in, in.HumanReview, in.CaseReviews)
	if err != nil || report.Outcome != "pass" || decoded.ArtifactAnchor != nil {
		t.Fatalf("legacy Resume = %+v, %v; anchor = %+v", report, err, decoded.ArtifactAnchor)
	}
	// Stripping a new stage's anchor is not a legacy-compatible downgrade.
	stage.ArtifactAnchor = nil
	if _, err := (SkillEvalRunner{}).Resume(stage, in, in.HumanReview, in.CaseReviews); err == nil || !strings.Contains(err.Error(), "modified") {
		t.Fatalf("stripped new stage error = %v", err)
	}
}

func TestSkillEvalAnchoredStageCanonicalPreimages(t *testing.T) {
	in := skillEvalTestInput(t, skillEvalTestAdapter{})
	stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	absolute, err := filepath.Abs(in.ArtifactRoot)
	if err != nil {
		t.Fatal(err)
	}
	pathDigest := sha256.Sum256([]byte("taskrail-skill-eval-anchor-v1\x00" + filepath.Clean(absolute)))
	expectedPath := hex.EncodeToString(pathDigest[:])
	if stage.ArtifactAnchor == nil || stage.ArtifactAnchor.PathSHA256 != expectedPath {
		t.Fatalf("unexpected path digest: %+v", stage.ArtifactAnchor)
	}
	report, _ := json.Marshal(stage.Report)
	worksheet, _ := json.Marshal(stage.Worksheet)
	var facts []string
	for _, fact := range stage.ArtifactAnchor.Ancestors {
		facts = append(facts, fmt.Sprintf(`{"type":%d,"device":%d,"inode":%d}`, fact.Type, fact.Device, fact.Inode))
	}
	anchor := fmt.Sprintf(`{"path_sha256":%q,"ancestors":[%s]}`, expectedPath, strings.Join(facts, ","))
	preimage := fmt.Sprintf(`{"schema_version":1,"report":%s,"worksheet":%s,"artifact_anchor":%s}`, report, worksheet, anchor)
	seal := sha256.Sum256([]byte("taskrail-skill-eval-stage-v1\x00" + preimage))
	expectedSeal := hex.EncodeToString(seal[:])
	compact := strings.TrimSuffix(preimage, "}") + fmt.Sprintf(`,"seal":%q}`, expectedSeal)
	var expected bytes.Buffer
	if err := json.Indent(&expected, []byte(compact), "", "  "); err != nil {
		t.Fatal(err)
	}
	expected.WriteByte('\n')
	data, err := RenderSkillEvalStage(stage)
	if err != nil || stage.Seal != expectedSeal || !bytes.Equal(data, expected.Bytes()) {
		t.Fatalf("anchored stage violated canonical bytes or seal preimage: %v", err)
	}
}
