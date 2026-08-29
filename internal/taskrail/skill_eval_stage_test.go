package taskrail

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillEvalRunnerStagesAllArmsBeforeHumanComparison(t *testing.T) {
	in := skillEvalTestInput(t, skillEvalTestAdapter{})
	stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if stage.Report.Cases[0].Candidate == nil || stage.Report.Cases[0].Baseline == nil {
		t.Fatalf("staged arms = %#v", stage.Report.Cases[0])
	}
	if stage.Report.Cases[0].Comparison != "inconclusive" || len(stage.Worksheet) != 1 || len(stage.Worksheet[0].HumanReviewQuestions) != 1 {
		t.Fatalf("staged worksheet = %#v", stage)
	}

	report, err := (SkillEvalRunner{}).Resume(stage, in, "Maintainer reviewed the frozen evidence.", in.CaseReviews)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if report.Outcome != "pass" || report.Cases[0].Comparison != "same" {
		t.Fatalf("resumed report = %#v", report)
	}
}

func TestSkillEvalRunnerResumeRejectsStagedTampering(t *testing.T) {
	for _, mutate := range []struct {
		name   string
		mutate func(*SkillEvalStage)
	}{
		{"snapshot", func(stage *SkillEvalStage) { stage.Report.TestedHead = testSkillEvalDigest("forged-head") }},
		{"executable", func(stage *SkillEvalStage) {
			stage.Report.Cases[0].Candidate.ExecutableSHA256 = testSkillEvalDigest("forged-executable")
		}},
		{"skill", func(stage *SkillEvalStage) {
			stage.Report.Cases[0].Candidate.SkillSHA256 = testSkillEvalDigest("forged-skill")
		}},
		{"fixture", func(stage *SkillEvalStage) {
			stage.Report.Cases[0].FixtureSHA256 = testSkillEvalDigest("forged-fixture")
		}},
		{"case", func(stage *SkillEvalStage) { stage.Report.Cases[0].CaseID = "forged-case" }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			in := skillEvalTestInput(t, skillEvalTestAdapter{})
			stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			mutate.mutate(&stage)
			if _, err := (SkillEvalRunner{}).Resume(stage, in, "Maintainer reviewed the frozen evidence.", in.CaseReviews); err == nil {
				t.Fatal("Resume accepted tampered staged evidence")
			}
		})
	}

	t.Run("raw evidence", func(t *testing.T) {
		in := skillEvalTestInput(t, skillEvalTestAdapter{})
		stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		raw := skillEvalRawRoot(in, in.Registry[0], skillEvalCandidateArm)
		if err := os.WriteFile(filepath.Join(raw, "result.txt"), []byte("tampered"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := (SkillEvalRunner{}).Resume(stage, in, "Maintainer reviewed the frozen evidence.", in.CaseReviews); err == nil {
			t.Fatal("Resume accepted changed raw evidence")
		}
	})

	t.Run("current bindings", func(t *testing.T) {
		in := skillEvalTestInput(t, skillEvalTestAdapter{})
		stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		in.TestedHead = testSkillEvalDigest("new-head")
		if _, err := (SkillEvalRunner{}).Resume(stage, in, "Maintainer reviewed the frozen evidence.", in.CaseReviews); err == nil {
			t.Fatal("Resume accepted stale current bindings")
		}
	})

	t.Run("review binding", func(t *testing.T) {
		in := skillEvalTestInput(t, skillEvalTestAdapter{})
		stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		reviews := map[string]SkillEvalCaseReview{"other": {Comparison: "same", HumanReview: "Wrong case."}}
		if _, err := (SkillEvalRunner{}).Resume(stage, in, "Maintainer reviewed the frozen evidence.", reviews); err == nil {
			t.Fatal("Resume accepted unbound human review")
		}
	})
}

func TestSkillEvalStageRoundTripRejectsByteTampering(t *testing.T) {
	in := skillEvalTestInput(t, skillEvalTestAdapter{})
	stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	data, err := RenderSkillEvalStage(stage)
	if err != nil {
		t.Fatalf("RenderSkillEvalStage: %v", err)
	}
	if strings.Contains(string(data), in.ArtifactRoot) {
		t.Fatalf("stage leaked producer-local raw root: %s", data)
	}
	decoded, err := DecodeSkillEvalStage(data, in)
	if err != nil {
		t.Fatalf("DecodeSkillEvalStage: %v", err)
	}
	if _, err := (SkillEvalRunner{}).Resume(decoded, in, "Maintainer reviewed serialized evidence.", in.CaseReviews); err != nil {
		t.Fatalf("Resume decoded stage: %v", err)
	}
	for _, tampered := range [][]byte{
		[]byte(strings.Replace(string(data), `"schema_version": 1`, `"schema_version": 2`, 1)),
		[]byte(strings.Replace(string(data), `"seal": "`, `"seal": "x`, 1)),
	} {
		if _, err := DecodeSkillEvalStage(tampered, in); err == nil {
			t.Fatal("DecodeSkillEvalStage accepted staged byte tampering")
		}
	}
}

func TestSkillEvalRunnerRejectsProducerLocalStageFields(t *testing.T) {
	in := skillEvalTestInput(t, skillEvalTestAdapter{})
	in.AdapterIdentity.Name = "/tmp/adapter"
	if _, err := (SkillEvalRunner{}).Execute(context.Background(), in); err == nil {
		t.Fatal("Execute accepted a producer-local stage identity")
	}
}
