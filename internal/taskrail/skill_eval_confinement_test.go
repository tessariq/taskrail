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

// skillEvalAncestorSwapAdapter is an untrusted adapter that writes complete
// canonical evidence for every arm and then, for one targeted arm, replaces
// the skill ancestor of the raw root at a controlled point inside Run with
// either a symlink to a byte-identical external copy or a regular file. When
// anchorRoot is set it instead replaces the artifact root itself, the trust
// anchor every path-based confinement check resolves through. It then
// attempts one further raw write through the relocated root and records
// whether that write landed, so a test can distinguish an implementation that
// prevented the write from one that detected the swap after it.
type skillEvalAncestorSwapAdapter struct {
	target              string
	external            string
	anchorRoot          string
	nonDirectory        bool
	rename              bool
	postSwapWriteLanded bool
}

func (adapter *skillEvalAncestorSwapAdapter) Run(_ context.Context, request SkillEvalAdapterRequest) (SkillEvalAdapterResult, error) {
	if err := os.WriteFile(filepath.Join(request.RawRoot, "result.txt"), []byte(request.Arm), 0o600); err != nil {
		return SkillEvalAdapterResult{}, err
	}
	result, err := skillEvalSuccessfulResult(request)
	if err != nil {
		return SkillEvalAdapterResult{}, err
	}
	if request.Arm != adapter.target {
		return result, nil
	}
	swapDir := filepath.Dir(filepath.Dir(request.RawRoot))
	if adapter.anchorRoot != "" {
		swapDir = adapter.anchorRoot
	}
	if adapter.nonDirectory {
		if err := os.RemoveAll(swapDir); err != nil {
			return SkillEvalAdapterResult{}, err
		}
		if err := os.WriteFile(swapDir, []byte("not a directory"), 0o600); err != nil {
			return SkillEvalAdapterResult{}, err
		}
	} else if adapter.rename {
		// A rename keeps the swapped directory's own inode, so an identity
		// pin alone cannot see it; only the type change at the planted path
		// (directory replaced by a symlink) exposes the relocation.
		if err := os.Rename(swapDir, adapter.external); err != nil {
			return SkillEvalAdapterResult{}, err
		}
		if err := os.Symlink(adapter.external, swapDir); err != nil {
			return SkillEvalAdapterResult{}, err
		}
	} else {
		if err := os.CopyFS(adapter.external, os.DirFS(swapDir)); err != nil {
			return SkillEvalAdapterResult{}, err
		}
		if err := os.RemoveAll(swapDir); err != nil {
			return SkillEvalAdapterResult{}, err
		}
		if err := os.Symlink(adapter.external, swapDir); err != nil {
			return SkillEvalAdapterResult{}, err
		}
	}
	adapter.postSwapWriteLanded = os.WriteFile(filepath.Join(request.RawRoot, "post-swap-write.txt"), []byte("written after relocation\n"), 0o600) == nil
	return result, nil
}

// TestSkillEvalRunnerExecuteFailsClosedOnInRunRawRootAncestorSwap pins the
// executed-arm detection gap T-415 deferred: an untrusted adapter that swaps a
// raw-root ancestor during Run must not yield a run record, runner-written
// outcome receipt, digest, or sealed stage, and the refusal must name the arm
// it refused. The sentinel observations record prevention versus detection:
// the adapter's post-swap write already lands outside the artifact tree, so
// this refusal detects the relocation after the write; it does not and cannot
// prevent or roll back bytes the adapter already wrote.
func TestSkillEvalRunnerExecuteFailsClosedOnInRunRawRootAncestorSwap(t *testing.T) {
	for _, tc := range []struct {
		name         string
		target       string
		nonDirectory bool
	}{
		{"candidate arm symlinked ancestor", skillEvalCandidateArm, false},
		{"baseline arm symlinked ancestor", skillEvalBaselineArm, false},
		{"candidate arm non-directory ancestor", skillEvalCandidateArm, true},
		{"baseline arm non-directory ancestor", skillEvalBaselineArm, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := &skillEvalAncestorSwapAdapter{target: tc.target, nonDirectory: tc.nonDirectory, external: filepath.Join(t.TempDir(), "relocated")}
			in := skillEvalTestInput(t, adapter)
			stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
			if err == nil || !strings.Contains(err.Error(), tc.target+" arm:") || !strings.Contains(err.Error(), "artifact root") {
				t.Fatalf("Execute error = %v, want an arm-naming confinement refusal for the swapped raw root", err)
			}
			if len(stage.Report.Cases) != 0 {
				t.Fatal("Execute returned stage cases after an in-run ancestor swap")
			}
			if tc.nonDirectory {
				return
			}
			// The refusal happens before the runner records its outcome
			// receipt in, grades from, or digests the relocated namespace.
			relocated := filepath.Join(adapter.external, in.Registry[0].CaseID, tc.target)
			if _, statErr := os.Stat(filepath.Join(relocated, "outcome.json")); !os.IsNotExist(statErr) {
				t.Fatalf("runner outcome receipt reached the relocated namespace: %v", statErr)
			}
			if !adapter.postSwapWriteLanded {
				t.Fatal("post-swap write did not land; the refusal would claim prevention it does not have")
			}
			if _, statErr := os.Stat(filepath.Join(relocated, "post-swap-write.txt")); statErr != nil {
				t.Fatalf("post-swap sentinel missing from the relocated tree: %v", statErr)
			}
		})
	}
}

// TestSkillEvalRunnerExecuteFailsClosedOnInRunArtifactRootSwap pins the
// anchor half of the in-run swap refusal: the component walk in
// skillEvalConfinedRawRoot validates only components strictly beneath the
// artifact root and each of its Lstat calls resolves through a symlinked
// anchor, so relocating the anchor itself (or any ancestor above it) must be
// caught by anchor identity, never by the component walk. The sentinel
// observations again record detection, not prevention: the adapter's
// post-swap write already lands in the relocated tree before the refusal.
func TestSkillEvalRunnerExecuteFailsClosedOnInRunArtifactRootSwap(t *testing.T) {
	for _, tc := range []struct {
		name   string
		target string
		rename bool
	}{
		{"candidate arm copied artifact root", skillEvalCandidateArm, false},
		{"baseline arm copied artifact root", skillEvalBaselineArm, false},
		{"candidate arm renamed artifact root", skillEvalCandidateArm, true},
		{"baseline arm renamed artifact root", skillEvalBaselineArm, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := &skillEvalAncestorSwapAdapter{target: tc.target, rename: tc.rename, external: filepath.Join(t.TempDir(), "relocated")}
			in := skillEvalTestInput(t, adapter)
			adapter.anchorRoot = in.ArtifactRoot
			stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
			if err == nil || !strings.Contains(err.Error(), tc.target+" arm:") || !strings.Contains(err.Error(), "artifact root") {
				t.Fatalf("Execute error = %v, want an arm-naming confinement refusal for the swapped artifact root", err)
			}
			if len(stage.Report.Cases) != 0 {
				t.Fatal("Execute returned stage cases after an in-run artifact root swap")
			}
			rawRoot := skillEvalRawRoot(in, in.Registry[0], tc.target)
			relocated := filepath.Join(adapter.external, mustRel(t, in.ArtifactRoot, rawRoot))
			if _, statErr := os.Stat(filepath.Join(relocated, "outcome.json")); !os.IsNotExist(statErr) {
				t.Fatalf("runner outcome receipt reached the relocated namespace: %v", statErr)
			}
			if !adapter.postSwapWriteLanded {
				t.Fatal("post-swap write did not land; the refusal would claim prevention it does not have")
			}
			if _, statErr := os.Stat(filepath.Join(relocated, "post-swap-write.txt")); statErr != nil {
				t.Fatalf("post-swap sentinel missing from the relocated tree: %v", statErr)
			}
		})
	}
}

func mustRel(t *testing.T, base, target string) string {
	t.Helper()
	rel, err := filepath.Rel(base, target)
	if err != nil {
		t.Fatal(err)
	}
	return rel
}

// TestSkillEvalRunnerExecuteAcceptsAdapterThatNeverSwaps is the positive
// control for the in-run swap refusal: the same adapter shape without a swap
// target writes identical canonical evidence through real directory
// ancestors, and both arms must still execute, seal, and resume into the
// existing report and digest shapes.
func TestSkillEvalRunnerExecuteAcceptsAdapterThatNeverSwaps(t *testing.T) {
	adapter := &skillEvalAncestorSwapAdapter{external: filepath.Join(t.TempDir(), "relocated")}
	in := skillEvalTestInput(t, adapter)
	stage, err := (SkillEvalRunner{}).Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	report, err := (SkillEvalRunner{}).Resume(stage, in, in.HumanReview, in.CaseReviews)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if report.Outcome != "pass" {
		t.Fatalf("outcome = %q, want pass over unswapped evidence", report.Outcome)
	}
}
