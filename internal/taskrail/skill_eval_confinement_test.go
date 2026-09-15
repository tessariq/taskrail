package taskrail

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSkillEvalRunnerExecuteRefusesUnconfinedExecutedRawAncestor pins the
// executed-arm half of raw-root confinement: adoption and stage resume already
// refuse a raw root whose path from the artifact root traverses a symlinked or
// non-directory ancestor, and an executed arm must refuse the same relocation
// before the adapter is invoked, so no provider evidence is ever written
// through a root that left the artifact tree.
func TestSkillEvalRunnerExecuteRefusesUnconfinedExecutedRawAncestor(t *testing.T) {
	for _, tc := range []struct {
		name  string
		plant func(t *testing.T, skillDir string)
	}{
		{"symlinked ancestor", func(t *testing.T, skillDir string) {
			if err := os.MkdirAll(filepath.Dir(skillDir), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(t.TempDir(), skillDir); err != nil {
				t.Fatal(err)
			}
		}},
		{"non-directory ancestor", func(t *testing.T, skillDir string) {
			if err := os.MkdirAll(filepath.Dir(skillDir), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(skillDir, []byte("not a directory"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := &skillEvalRecordingAdapter{}
			in := skillEvalTestInput(t, adapter)
			skillDir := filepath.Dir(filepath.Dir(skillEvalRawRoot(in, in.Registry[0], skillEvalCandidateArm)))
			tc.plant(t, skillDir)
			_, err := (SkillEvalRunner{}).Execute(context.Background(), in)
			// Pin the arm-naming prefix runSkillEvalArm itself adds, not the
			// "candidate <case>" context Execute wraps every candidate error in.
			if err == nil || !strings.Contains(err.Error(), "candidate arm: raw root") || !strings.Contains(err.Error(), "artifact root") {
				t.Fatalf("Execute error = %v, want a confinement refusal naming the candidate arm", err)
			}
			if adapter.calls != 0 {
				t.Fatalf("adapter invoked %d times, want 0 before any evidence is written", adapter.calls)
			}
		})
	}
}

// TestSkillEvalRunnerExecuteCreatesMissingRawAncestors pins the legitimate
// executed-arm shape the confinement check must keep admitting: an artifact
// root whose raw-root ancestors do not exist yet is created by MkdirAll and
// both arms execute without false refusals.
func TestSkillEvalRunnerExecuteCreatesMissingRawAncestors(t *testing.T) {
	adapter := &skillEvalRecordingAdapter{}
	in := skillEvalTestInput(t, adapter)
	if _, err := os.Stat(filepath.Join(in.ArtifactRoot, "skill-evals")); !os.IsNotExist(err) {
		t.Fatalf("artifact root already holds a skill-evals tree: %v", err)
	}
	if _, err := (SkillEvalRunner{}).Execute(context.Background(), in); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if adapter.calls != 2 {
		t.Fatalf("adapter invoked %d times, want both executed arms", adapter.calls)
	}
}
