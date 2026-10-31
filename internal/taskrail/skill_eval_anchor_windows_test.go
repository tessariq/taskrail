//go:build windows

package taskrail

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillEvalLongWindowsArtifactRoot(t *testing.T) {
	in := skillEvalTestInput(t, skillEvalTestAdapter{})
	for len(in.ArtifactRoot) <= 300 {
		in.ArtifactRoot = filepath.Join(in.ArtifactRoot, strings.Repeat("a", 40))
	}
	if err := os.MkdirAll(in.ArtifactRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	data, err := RenderSkillEvalStage(stage)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeSkillEvalStage(data, in)
	if err != nil {
		t.Fatal(err)
	}
	if report, err := (SkillEvalRunner{}).Resume(decoded, in, in.HumanReview, in.CaseReviews); err != nil || report.Outcome != "pass" {
		t.Fatalf("long root resume = %+v, %v", report, err)
	}
}
