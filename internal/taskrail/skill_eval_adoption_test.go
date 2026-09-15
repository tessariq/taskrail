package taskrail

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// skillEvalAdoptionInput returns a single baseline-required case input whose
// baseline raw root already holds the canonical receipts a prior session
// wrote: the adapter's facts receipt and the runner's declared-outcome
// receipt.
func skillEvalAdoptionInput(t *testing.T, adapter SkillEvalAdapter) (SkillEvalRunInput, string, []SkillEvalObservedFact) {
	t.Helper()
	in := skillEvalTestInput(t, adapter)
	root := skillEvalRawRoot(in, in.Registry[0], skillEvalBaselineArm)
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	result, err := skillEvalSuccessfulResult(SkillEvalAdapterRequest{Case: in.Registry[0], RawRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeSkillEvalOutcomeReceipt(root, result.Outcome); err != nil {
		t.Fatal(err)
	}
	return in, root, result.Facts
}

func TestSkillEvalRunnerAdoptsEnumeratedBaselineWithoutInvokingAdapter(t *testing.T) {
	adapter := &skillEvalRecordingAdapter{}
	in, root, _ := skillEvalAdoptionInput(t, adapter)
	in.Registry = append(in.Registry, testSkillEvalCase("second", "autonomous-task", "committed", true, "second"))
	in.AdoptedBaselineCases = []string{"existing"}
	in.CaseReviews["second"] = SkillEvalCaseReview{Comparison: "same", HumanReview: "Equivalent behavior."}
	stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// Only the two candidate arms and the unadopted baseline arm may invoke the
	// adapter; the adopted baseline arm must not.
	if adapter.calls != 3 {
		t.Fatalf("adapter invoked %d times, want 3 (two candidates plus the unadopted baseline)", adapter.calls)
	}
	adopted := stage.Report.Cases[0].Baseline
	if adopted == nil {
		t.Fatal("adopted baseline arm is missing from the stage")
	}
	if adopted.Outcome != "pass" || adopted.DeterministicGrade != "pass" {
		t.Fatalf("adopted baseline = %+v", *adopted)
	}
	if adopted.SkillSHA256 != testSkillEvalDigest("baseline-task") || adopted.ExecutableSHA256 != testSkillEvalDigest("baseline-exe") {
		t.Fatalf("adopted baseline bindings = %+v", *adopted)
	}
	digest, err := nonEmptySkillEvalRawDigest(root)
	if err != nil {
		t.Fatalf("recompute adopted raw digest: %v", err)
	}
	if adopted.RawSHA256 != digest {
		t.Fatalf("adopted raw digest = %s, want %s", adopted.RawSHA256, digest)
	}
	if stage.Report.Cases[1].Baseline == nil {
		t.Fatal("unadopted baseline arm is missing from the stage")
	}
	report, err := (SkillEvalRunner{}).Resume(stage, in, "Maintainer reviewed outcomes; the baseline arm of case existing was adopted from pre-staged evidence.", in.CaseReviews)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if report.Outcome != "pass" {
		t.Fatalf("outcome = %q, want pass with an adopted baseline arm", report.Outcome)
	}
	encoded, err := RenderSkillEvalReport(report)
	if err != nil {
		t.Fatalf("RenderSkillEvalReport: %v", err)
	}
	if _, err := DecodeSkillEvalReport(encoded, report); err != nil {
		t.Fatalf("DecodeSkillEvalReport: %v", err)
	}
}

// copySkillEvalTree copies one staged raw tree's regular files so a test can
// relocate evidence between raw roots exactly like a staging maintainer.
func copySkillEvalTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular staged file %q", rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSkillEvalRunnerAdoptionRefusesSymlinkedRawAncestor(t *testing.T) {
	adapter := &skillEvalRecordingAdapter{}
	in, root, _ := skillEvalAdoptionInput(t, adapter)
	in.AdoptedBaselineCases = []string{"existing"}
	// Relocate the staged tree outside the artifact root and re-expose it
	// through a symlinked skill ancestor. The walk rejects symlinked entries
	// beneath the root but cannot see a symlinked ancestor, which resolves to
	// the external directory; adoption must refuse the unconfined raw root
	// before reading any evidence from it.
	skillDir := filepath.Dir(filepath.Dir(root))
	external := filepath.Join(t.TempDir(), "outside", "autonomous-task")
	copySkillEvalTree(t, skillDir, external)
	if err := os.RemoveAll(skillDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, skillDir); err != nil {
		t.Fatal(err)
	}
	_, err := (SkillEvalRunner{}).Execute(context.Background(), in)
	if err == nil || !strings.Contains(err.Error(), "artifact root") {
		t.Fatalf("Execute error = %v, want a confinement refusal for a symlinked raw-root ancestor", err)
	}
}

// TestSkillEvalRunnerAdoptionOutcomeParity pins Behavior A: adopting a
// pre-staged baseline arm preserves the outcome the original execution
// recorded in its raw tree, whatever the re-derived grade says. Reuse must
// not upgrade a result just because the saved checks pass — an originally
// incomplete arm stays incomplete and an originally failed arm stays failed —
// so the adopted run record stays indistinguishable from the executed one.
func TestSkillEvalRunnerAdoptionOutcomeParity(t *testing.T) {
	for _, tc := range []struct {
		name       string
		original   string
		wantReport string
	}{
		{"originally incomplete stays incomplete", "incomplete", "incomplete"},
		{"originally failed stays failed", "fail", "pass"},
		{"completed pass stays pass", "pass", "pass"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executed := skillEvalTestInput(t, skillEvalResultAdapter{candidate: "pass", baseline: tc.original})
			stage, err := (SkillEvalRunner{}).Execute(context.Background(), executed)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			executedBaseline := stage.Report.Cases[0].Baseline
			if executedBaseline == nil || executedBaseline.Outcome != tc.original || executedBaseline.DeterministicGrade != "pass" {
				t.Fatalf("executed baseline = %+v, want outcome %q over facts that grade pass", executedBaseline, tc.original)
			}
			adopted := skillEvalTestInput(t, &skillEvalRecordingAdapter{})
			adopted.AdoptedBaselineCases = []string{"existing"}
			copySkillEvalTree(t,
				skillEvalRawRoot(executed, executed.Registry[0], skillEvalBaselineArm),
				skillEvalRawRoot(adopted, adopted.Registry[0], skillEvalBaselineArm))
			adoptedStage, err := (SkillEvalRunner{}).Execute(context.Background(), adopted)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			adoptedBaseline := adoptedStage.Report.Cases[0].Baseline
			if adoptedBaseline == nil || *adoptedBaseline != *executedBaseline {
				t.Fatalf("adopted baseline = %+v, want the executed record preserved verbatim: %+v", adoptedBaseline, executedBaseline)
			}
			report, err := (SkillEvalRunner{}).Resume(adoptedStage, adopted, "Prior baseline evidence adopted after maintainer review; the original outcome is preserved.", adopted.CaseReviews)
			if err != nil {
				t.Fatalf("Resume: %v", err)
			}
			if report.Outcome != tc.wantReport {
				t.Fatalf("outcome = %q, want %q with the adopted baseline arm", report.Outcome, tc.wantReport)
			}
		})
	}
}

func TestSkillEvalStageResumeRefusesSymlinkedRawAncestor(t *testing.T) {
	in, root, _ := skillEvalAdoptionInput(t, &skillEvalRecordingAdapter{})
	in.AdoptedBaselineCases = []string{"existing"}
	stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// Swap the skill ancestor for a symlink to a byte-identical external copy
	// after the stage was sealed: the recomputed digest still matches, so only
	// an ancestor-confinement re-check can refuse the relocated evidence.
	skillDir := filepath.Dir(filepath.Dir(root))
	external := t.TempDir()
	copySkillEvalTree(t, filepath.Dir(root), filepath.Join(external, "existing"))
	if err := os.RemoveAll(skillDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, skillDir); err != nil {
		t.Fatal(err)
	}
	_, err = (SkillEvalRunner{}).Resume(stage, in, "Maintainer review.", in.CaseReviews)
	if err == nil || !strings.Contains(err.Error(), "artifact root") {
		t.Fatalf("Resume error = %v, want a confinement refusal for a symlinked raw-root ancestor", err)
	}
}

func TestSkillEvalRunnerAdoptionRefusesLoudly(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, in *SkillEvalRunInput)
	}{
		{"unregistered case", func(t *testing.T, in *SkillEvalRunInput) {
			in.AdoptedBaselineCases = []string{"ghost"}
		}},
		{"duplicate case", func(t *testing.T, in *SkillEvalRunInput) {
			in.AdoptedBaselineCases = []string{"existing", "existing"}
		}},
		{"candidate-only case", func(t *testing.T, in *SkillEvalRunInput) {
			in.AdoptedBaselineCases = []string{"existing"}
			in.Registry[0].BaselineRequired = false
			in.BaselineSkillSHA256 = nil
			in.CaseReviews["existing"] = SkillEvalCaseReview{Comparison: "candidate-only", HumanReview: "No baseline arm exists."}
		}},
		{"missing raw root", func(t *testing.T, in *SkillEvalRunInput) {
			in.AdoptedBaselineCases = []string{"existing"}
		}},
		{"empty raw tree", func(t *testing.T, in *SkillEvalRunInput) {
			in.AdoptedBaselineCases = []string{"existing"}
			if err := os.MkdirAll(skillEvalRawRoot(*in, in.Registry[0], skillEvalBaselineArm), 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{"noncanonical receipt", func(t *testing.T, in *SkillEvalRunInput) {
			in.AdoptedBaselineCases = []string{"existing"}
			root := skillEvalRawRoot(*in, in.Registry[0], skillEvalBaselineArm)
			if err := os.MkdirAll(root, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "facts.json"), []byte("[]"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing outcome receipt", func(t *testing.T, in *SkillEvalRunInput) {
			in.AdoptedBaselineCases = []string{"existing"}
			root := skillEvalRawRoot(*in, in.Registry[0], skillEvalBaselineArm)
			if err := os.MkdirAll(root, 0o700); err != nil {
				t.Fatal(err)
			}
			if _, err := skillEvalSuccessfulResult(SkillEvalAdapterRequest{Case: in.Registry[0], RawRoot: root}); err != nil {
				t.Fatal(err)
			}
		}},
		{"unsupported outcome receipt", func(t *testing.T, in *SkillEvalRunInput) {
			in.AdoptedBaselineCases = []string{"existing"}
			root := skillEvalRawRoot(*in, in.Registry[0], skillEvalBaselineArm)
			if err := os.MkdirAll(root, 0o700); err != nil {
				t.Fatal(err)
			}
			if _, err := skillEvalSuccessfulResult(SkillEvalAdapterRequest{Case: in.Registry[0], RawRoot: root}); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "outcome.json"), skillEvalOutcomeReceiptBytes("escalated"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"noncanonical outcome receipt", func(t *testing.T, in *SkillEvalRunInput) {
			in.AdoptedBaselineCases = []string{"existing"}
			root := skillEvalRawRoot(*in, in.Registry[0], skillEvalBaselineArm)
			if err := os.MkdirAll(root, 0o700); err != nil {
				t.Fatal(err)
			}
			if _, err := skillEvalSuccessfulResult(SkillEvalAdapterRequest{Case: in.Registry[0], RawRoot: root}); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "outcome.json"), []byte("{\"outcome\":\"pass\"}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := &skillEvalRecordingAdapter{}
			in := skillEvalTestInput(t, adapter)
			tc.mutate(t, &in)
			if _, err := (SkillEvalRunner{}).Execute(context.Background(), in); err == nil {
				t.Fatal("Execute accepted an inadmissible baseline adoption")
			}
		})
	}
}

func TestSkillEvalRunnerAdoptedArmGradesThroughCasePredicates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(facts []SkillEvalObservedFact) []SkillEvalObservedFact
	}{
		{"missing declared facts", func(facts []SkillEvalObservedFact) []SkillEvalObservedFact {
			return facts[:len(facts)-2]
		}},
		{"fabricated extra fact", func(facts []SkillEvalObservedFact) []SkillEvalObservedFact {
			return append(facts, SkillEvalObservedFact{Action: "not-declared", Operation: "git-command", Command: []string{"git", "status"}})
		}},
		{"failed declared action", func(facts []SkillEvalObservedFact) []SkillEvalObservedFact {
			facts[len(facts)-1].ExitCode = 1
			return facts
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := &skillEvalRecordingAdapter{}
			in, root, facts := skillEvalAdoptionInput(t, adapter)
			in.AdoptedBaselineCases = []string{"existing"}
			if err := writeSkillEvalFacts(root, tc.mutate(facts)); err != nil {
				t.Fatal(err)
			}
			stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			baseline := stage.Report.Cases[0].Baseline
			// The grade is re-derived through the case predicates over the
			// mutated receipt while the preserved outcome stays what the
			// original execution recorded, exactly like an executed arm whose
			// adapter declared pass over failing facts.
			if baseline == nil || baseline.DeterministicGrade != "fail" || baseline.Outcome != "pass" {
				t.Fatalf("adopted baseline = %+v, want preserved outcome pass over a grade derived through the case predicates", baseline)
			}
		})
	}
}
