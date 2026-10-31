package taskrail

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillEvalPinnedAdoptionRefusesWithoutFallback(t *testing.T) {
	for _, name := range []string{"seal", "legacy", "relocated", "session", "case", "fixture", "skill", "executable", "outcome", "grade", "digest", "unused"} {
		t.Run(name, func(t *testing.T) {
			in := skillEvalTestInput(t, skillEvalTestAdapter{})
			stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			adapter := &skillEvalRecordingAdapter{}
			in.Adapter = adapter
			in.AdoptedBaselineCases = []string{"existing"}
			in.AdoptedBaselineStage = &stage
			switch name {
			case "seal":
				stage.Seal = strings.Repeat("0", 64)
			case "legacy":
				stage.ArtifactAnchor = nil
			case "relocated":
				next := filepath.Join(t.TempDir(), "relocated")
				if err := os.Rename(in.ArtifactRoot, next); err != nil {
					t.Fatal(err)
				}
				in.ArtifactRoot = next
			case "session":
				stage.Report.SessionID = "different-session"
			case "case":
				stage.Report.Cases[0].CaseID = "different-case"
			case "fixture":
				stage.Report.Cases[0].FixtureSHA256 = testSkillEvalDigest("different-fixture")
			case "skill":
				stage.Report.Cases[0].Baseline.SkillSHA256 = testSkillEvalDigest("different-skill")
			case "executable":
				stage.Report.Cases[0].Baseline.ExecutableSHA256 = testSkillEvalDigest("different-executable")
			case "outcome":
				stage.Report.Cases[0].Baseline.Outcome = "fail"
			case "grade":
				stage.Report.Cases[0].Baseline.DeterministicGrade = "fail"
			case "digest":
				stage.Report.Cases[0].Baseline.RawSHA256 = testSkillEvalDigest("different-raw")
			case "unused":
				in.AdoptedBaselineCases = nil
			}
			if name != "seal" {
				stage.Seal = skillEvalStageSeal(stage)
			}
			got, err := (SkillEvalRunner{}).Execute(context.Background(), in)
			if err == nil || got.Seal != "" || !strings.Contains(err.Error(), "adopt baseline") {
				t.Fatalf("Execute = %+v, %v, want actionable pinned adoption refusal", got, err)
			}
			if adapter.calls != 0 {
				t.Fatalf("invalid pinned adoption invoked adapter %d times", adapter.calls)
			}
		})
	}
}

type skillEvalBaselineMutationAdapter struct {
	baseline string
	outcome  bool
	calls    int
}

func (adapter *skillEvalBaselineMutationAdapter) Run(_ context.Context, request SkillEvalAdapterRequest) (SkillEvalAdapterResult, error) {
	adapter.calls++
	result, err := skillEvalSuccessfulResult(request)
	if err != nil {
		return result, err
	}
	if adapter.outcome {
		err = os.WriteFile(filepath.Join(adapter.baseline, "outcome.json"), skillEvalOutcomeReceiptBytes("fail"), 0o600)
	} else {
		err = os.WriteFile(filepath.Join(adapter.baseline, "payload.txt"), []byte("changed after preflight"), 0o600)
	}
	return result, err
}

func TestSkillEvalPinnedAdoptionRechecksAfterCandidate(t *testing.T) {
	for _, outcome := range []bool{true, false} {
		t.Run(fmt.Sprintf("outcome=%t", outcome), func(t *testing.T) {
			in := skillEvalTestInput(t, skillEvalTestAdapter{})
			stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			adapter := &skillEvalBaselineMutationAdapter{baseline: skillEvalRawRoot(in, in.Registry[0], skillEvalBaselineArm), outcome: outcome}
			in.Adapter = adapter
			in.AdoptedBaselineCases, in.AdoptedBaselineStage = []string{"existing"}, &stage
			got, err := (SkillEvalRunner{}).Execute(context.Background(), in)
			if err == nil || !strings.Contains(err.Error(), "sealed baseline outcome, grade, or raw digest changed") || got.Seal != "" || adapter.calls != 1 {
				t.Fatalf("post-candidate adoption = %+v, %v, calls=%d", got, err, adapter.calls)
			}
		})
	}
}

func TestSkillEvalPinnedAdoptionAllowsCandidateAndRegistryEvolution(t *testing.T) {
	adapter := &skillEvalRecordingAdapter{}
	in := skillEvalTestInput(t, adapter)
	stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	in.AdoptedBaselineCases, in.AdoptedBaselineStage = []string{"existing"}, &stage
	in.CandidateExecutableSHA256 = testSkillEvalDigest("changed-candidate-executable")
	in.CandidateSkillsSHA256 = testSkillEvalDigest("changed-candidate-skillset")
	in.CandidateSkillSHA256["autonomous-task"] = testSkillEvalDigest("changed-candidate-skill")
	in.Registry = append(in.Registry, testSkillEvalCase("unrelated", "autonomous-task", "committed", false, "unrelated-fixture"))
	adapter.calls = 0
	adopted, err := (SkillEvalRunner{}).Execute(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if adapter.calls != 2 || len(adopted.Report.Cases) != 2 || *adopted.Report.Cases[0].Baseline != *stage.Report.Cases[0].Baseline || adopted.Report.Cases[0].Candidate.ExecutableSHA256 != in.CandidateExecutableSHA256 {
		t.Fatalf("candidate evolution = %+v, calls=%d", adopted, adapter.calls)
	}
}
