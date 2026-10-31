package taskrail

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSkillEvalResumeRefusesArtifactNamespaceSwap(t *testing.T) {
	for _, ancestor := range []bool{false, true} {
		for _, rename := range []bool{false, true} {
			name := "anchor-copy"
			if ancestor {
				name = "ancestor-copy"
			}
			if rename {
				name += "-rename"
			}
			t.Run(name, func(t *testing.T) {
				in := skillEvalTestInput(t, skillEvalTestAdapter{})
				parent := filepath.Join(t.TempDir(), "parent")
				in.ArtifactRoot = filepath.Join(parent, "artifacts")
				stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
				if err != nil {
					t.Fatal(err)
				}
				data, err := RenderSkillEvalStage(stage)
				if err != nil {
					t.Fatal(err)
				}
				swap := in.ArtifactRoot
				if ancestor {
					swap = parent
				}
				saved := swap + "-saved"
				if err := os.Rename(swap, saved); err != nil {
					t.Fatal(err)
				}
				if rename {
					if err := os.Symlink(saved, swap); err != nil {
						t.Fatal(err)
					}
				} else {
					copySkillEvalTree(t, saved, swap)
				}
				// Byte-identical evidence must fail on confinement, not a digest.
				if _, err := DecodeSkillEvalStage(data, in); err == nil || !strings.Contains(err.Error(), "artifact root") {
					t.Fatalf("Decode error = %v, want artifact root confinement refusal", err)
				}
				if report, err := (SkillEvalRunner{}).Resume(stage, in, in.HumanReview, in.CaseReviews); err == nil || report.SessionID != "" || !strings.Contains(err.Error(), "artifact root") {
					t.Fatalf("Resume report = %+v, error = %v, want confinement refusal without report", report, err)
				}
				adapter := &skillEvalRecordingAdapter{}
				in.Adapter = adapter
				in.AdoptedBaselineCases = []string{"existing"}
				in.AdoptedBaselineStage = &stage
				if adopted, err := (SkillEvalRunner{}).Execute(context.Background(), in); err == nil || adopted.Seal != "" || !strings.Contains(err.Error(), "artifact root") {
					t.Fatalf("pinned adoption = %+v, %v, want confinement refusal", adopted, err)
				}
				if adapter.calls != 0 {
					t.Fatalf("swapped pinned adoption invoked adapter %d times", adapter.calls)
				}
				if err := os.RemoveAll(swap); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(saved, swap); err != nil {
					t.Fatal(err)
				}
				// Restoring the original namespace restores its type and identity.
				decoded, err := DecodeSkillEvalStage(data, in)
				if err != nil {
					t.Fatal(err)
				}
				report, err := (SkillEvalRunner{}).Resume(decoded, in, in.HumanReview, in.CaseReviews)
				if err != nil || report.Outcome != "pass" || *report.Cases[0].Baseline != *stage.Report.Cases[0].Baseline {
					t.Fatalf("restored report = %+v, error = %v", report, err)
				}
				adopted, err := (SkillEvalRunner{}).Execute(context.Background(), in)
				if err != nil || *adopted.Report.Cases[0].Baseline != *stage.Report.Cases[0].Baseline {
					t.Fatalf("restored adoption = %+v, %v", adopted, err)
				}
			})
		}
	}
}

type skillEvalChdirAdapter struct {
	directory string
}

func (adapter skillEvalChdirAdapter) Run(_ context.Context, request SkillEvalAdapterRequest) (SkillEvalAdapterResult, error) {
	result, err := skillEvalSuccessfulResult(request)
	if err != nil {
		return result, err
	}
	if err := os.Chdir(adapter.directory); err != nil {
		return result, err
	}
	// A relative request would now create a second valid-looking evidence tree.
	if err := os.MkdirAll(request.RawRoot, 0o700); err != nil {
		return result, err
	}
	return skillEvalSuccessfulResult(request)
}

func TestSkillEvalExecutePinsRelativeArtifactRootAgainstChdir(t *testing.T) {
	for _, relative := range []bool{true, false} {
		name := "absolute"
		if relative {
			name = "relative"
		}
		t.Run(name, func(t *testing.T) {
			start, moved := t.TempDir(), t.TempDir()
			t.Chdir(start)
			in := skillEvalTestInput(t, skillEvalChdirAdapter{directory: moved})
			in.ArtifactRoot = filepath.Join(start, "artifacts")
			if relative {
				in.ArtifactRoot = "artifacts"
			}
			stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
			if relative {
				if err == nil || !strings.Contains(err.Error(), "artifact root") || stage.Seal != "" {
					t.Fatalf("Execute = %+v, %v, want loud confinement refusal", stage, err)
				}
				original := filepath.Join(start, skillEvalRawRoot(in, in.Registry[0], skillEvalCandidateArm))
				if _, err := os.Stat(filepath.Join(original, "facts.json")); err != nil {
					t.Fatalf("adapter did not write evidence at original root: %v", err)
				}
				if _, err := os.Stat(filepath.Join(original, "outcome.json")); !os.IsNotExist(err) {
					t.Fatalf("runner wrote a receipt before refusing rebasing: %v", err)
				}
			} else if err != nil || stage.Seal == "" {
				t.Fatalf("absolute Execute = %+v, %v", stage, err)
			}
			if _, err := os.Stat(filepath.Join(moved, "artifacts")); !os.IsNotExist(err) {
				t.Fatalf("runner wrote outside the original root: %v", err)
			}
		})
	}
}

func TestSkillEvalSiblingRenameRefusesOriginalSealedNamespace(t *testing.T) {
	in := skillEvalTestInput(t, skillEvalTestAdapter{})
	stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	data, err := RenderSkillEvalStage(stage)
	if err != nil {
		t.Fatal(err)
	}
	original := in.ArtifactRoot
	in.ArtifactRoot += "-renamed"
	if err := os.Rename(original, in.ArtifactRoot); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(in.ArtifactRoot, original) })
	current, err := skillEvalCaptureArtifactAnchor(in.ArtifactRoot)
	if err != nil || current.PathSHA256 == stage.ArtifactAnchor.PathSHA256 || !slices.Equal(current.Ancestors, stage.ArtifactAnchor.Ancestors) {
		t.Fatalf("sibling rename did not isolate path identity: %+v, %v", current, err)
	}
	if _, err := DecodeSkillEvalStage(data, in); err == nil || !strings.Contains(err.Error(), "artifact root") {
		t.Fatalf("Decode accepted renamed namespace: %v", err)
	}
	if report, err := (SkillEvalRunner{}).Resume(stage, in, in.HumanReview, in.CaseReviews); err == nil || report.SessionID != "" {
		t.Fatalf("Resume accepted renamed namespace: %+v, %v", report, err)
	}
	adapter := &skillEvalRecordingAdapter{}
	in.Adapter = adapter
	in.AdoptedBaselineCases, in.AdoptedBaselineStage = []string{"existing"}, &stage
	if adopted, err := (SkillEvalRunner{}).Execute(context.Background(), in); err == nil || adopted.Seal != "" || adapter.calls != 0 {
		t.Fatalf("pinned adoption accepted renamed namespace: %+v, %v, calls=%d", adopted, err, adapter.calls)
	}
}
