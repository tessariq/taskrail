package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateLoopDeliveryCommittedRequiresOneDirectChild(t *testing.T) {
	for _, test := range []struct {
		name    string
		commits int
		want    string
	}{
		{name: "one direct child", commits: 1},
		{name: "unchanged head", want: "delivery_commit_shape"},
		{name: "multiple commits", commits: 2, want: "delivery_commit_shape"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, evidence := loopDeliveryFixture(t, "committed")
			for i := 0; i < test.commits; i++ {
				loopDeliveryWrite(t, root, "product.txt", "product "+strings.Repeat("x", i+1)+"\n")
				if i == 0 {
					evidence.PostflightInputs["planning/STATE.md"] = []byte("after\n")
					loopDeliveryWrite(t, root, "planning/STATE.md", "after\n")
				}
				loopDeliveryCommit(t, root, "delivery", true)
			}
			evidence.Postflight = loopDeliverySnapshot(t, root)

			_, violations := validateLoopDelivery(evidence)
			if test.want == "" && len(violations) != 0 {
				t.Fatalf("violations = %+v", violations)
			}
			if test.want != "" && !hasLoopIntegrityCode(violations, test.want) {
				t.Fatalf("violations = %+v, want %s", violations, test.want)
			}
		})
	}
}

func TestValidateLoopDeliveryRejectsCommittedMergeAndDirtyShapes(t *testing.T) {
	for _, test := range []struct {
		name string
		want string
		run  func(*testing.T, string, *loopDeliveryEvidence)
	}{
		{
			name: "merge", want: "delivery_commit_shape",
			run: func(t *testing.T, root string, evidence *loopDeliveryEvidence) {
				loopDeliveryRun(t, root, "checkout", "-qb", "feature")
				loopDeliveryWrite(t, root, "feature.txt", "feature\n")
				loopDeliveryCommit(t, root, "feature", true)
				loopDeliveryRun(t, root, "checkout", "main")
				evidence.PostflightInputs["planning/STATE.md"] = []byte("after\n")
				loopDeliveryWrite(t, root, "planning/STATE.md", "after\n")
				loopDeliveryCommit(t, root, "delivery", true)
				loopDeliveryRun(t, root, "merge", "--no-ff", "-m", "merge", "feature")
			},
		},
		{
			name: "dirty", want: "delivery_dirty",
			run: func(t *testing.T, root string, _ *loopDeliveryEvidence) {
				loopDeliveryWrite(t, root, "product.txt", "dirty\n")
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, evidence := loopDeliveryFixture(t, "committed")
			test.run(t, root, &evidence)
			evidence.Postflight = loopDeliverySnapshot(t, root)

			_, violations := validateLoopDelivery(evidence)
			if !hasLoopIntegrityCode(violations, test.want) {
				t.Fatalf("violations = %+v, want %s", violations, test.want)
			}
		})
	}
}

func TestValidateLoopDeliveryRejectsCommittedPartialMetadata(t *testing.T) {
	root, evidence := loopDeliveryFixture(t, "committed")
	evidence.PostflightInputs["planning/STATE.md"] = []byte("after\n")
	loopDeliveryWrite(t, root, "product.txt", "product changed\n")
	loopDeliveryCommit(t, root, "omits state", true)
	evidence.Postflight = loopDeliverySnapshot(t, root)

	_, violations := validateLoopDelivery(evidence)
	if !hasLoopIntegrityCode(violations, "delivery_metadata_missing") {
		t.Fatalf("violations = %+v, want omitted metadata refusal", violations)
	}
}

func TestValidateLoopDeliveryAllowsIgnoredVerificationArtifacts(t *testing.T) {
	root, evidence := loopDeliveryFixture(t, "committed")
	evidence.PostflightInputs["planning/STATE.md"] = []byte("after\n")
	evidence.PostflightInputs["planning/artifacts/verify/T-001-selected/new/report.json"] = []byte("report\n")
	loopDeliveryWrite(t, root, "product.txt", "product changed\n")
	loopDeliveryWrite(t, root, "planning/STATE.md", "after\n")
	loopDeliveryCommit(t, root, "delivery", true)
	evidence.Postflight = loopDeliverySnapshot(t, root)

	_, violations := validateLoopDelivery(evidence)
	if len(violations) != 0 {
		t.Fatalf("violations = %+v", violations)
	}
}

func TestValidateLoopDeliveryReportsOrderedFullCommitFacts(t *testing.T) {
	root, evidence := loopDeliveryFixture(t, "committed")
	evidence.PostflightInputs["planning/STATE.md"] = []byte("after\n")
	loopDeliveryWrite(t, root, "product.txt", "product changed\n")
	loopDeliveryWrite(t, root, "planning/STATE.md", "after\n")
	loopDeliveryCommit(t, root, "delivery", true)
	evidence.Postflight = loopDeliverySnapshot(t, root)

	delivery, violations := validateLoopDelivery(evidence)
	wantHead := strings.TrimSpace(loopDeliveryRun(t, root, "rev-parse", "HEAD"))
	if len(violations) != 0 || delivery.Ref != "refs/heads/main" || !delivery.Clean || !delivery.Descendant || delivery.Remote != "not_checked" || delivery.HeadAfter != wantHead || len(delivery.Commits) != 1 || delivery.Commits[0] != wantHead {
		t.Fatalf("delivery = %+v, violations = %+v", delivery, violations)
	}
}

func TestValidateLoopDeliveryRejectsIntegrityAndStagedLocalMetadata(t *testing.T) {
	root, evidence := loopDeliveryFixture(t, "local")
	loopDeliveryWrite(t, root, "planning/STATE.md", "after\n")
	loopDeliveryRun(t, root, "add", "-f", "planning/STATE.md")
	evidence.Postflight = loopDeliverySnapshot(t, root)
	evidence.IntegrityViolations = []MachineViolation{{Code: "selected_task_mutation"}}

	_, violations := validateLoopDelivery(evidence)
	for _, want := range []string{"delivery_dirty", "delivery_metadata_exposed", "integrity_violation"} {
		if !hasLoopIntegrityCode(violations, want) {
			t.Fatalf("violations = %+v, want %s", violations, want)
		}
	}
}

func TestValidateLoopDeliveryUsesFrozenProductPathPolicy(t *testing.T) {
	root, evidence := loopDeliveryFixture(t, "committed")
	evidence.PostflightInputs["planning/STATE.md"] = []byte("after\n")
	evidence.AllowedProductPaths = map[string]bool{"product.txt": true}
	loopDeliveryWrite(t, root, "product.txt", "product changed\n")
	loopDeliveryWrite(t, root, "unrelated.txt", "unrelated\n")
	loopDeliveryWrite(t, root, "planning/STATE.md", "after\n")
	loopDeliveryCommit(t, root, "delivery", true)
	evidence.Postflight = loopDeliverySnapshot(t, root)

	_, violations := validateLoopDelivery(evidence)
	if !hasLoopIntegrityCode(violations, "delivery_product_path_unexpected") {
		t.Fatalf("violations = %+v, want frozen policy refusal", violations)
	}
}

func TestValidateLoopDeliveryRejectsLocalProvenanceMetadata(t *testing.T) {
	for _, message := range []string{"Taskrail implementation", "agent implementation", "uses .taskrail/local"} {
		t.Run(message, func(t *testing.T) {
			root, evidence := loopDeliveryFixture(t, "local")
			loopDeliveryWrite(t, root, "product.txt", "product changed\n")
			loopDeliveryRun(t, root, "add", "product.txt")
			loopDeliveryRun(t, root, "commit", "-qm", message)
			evidence.Postflight = loopDeliverySnapshot(t, root)

			_, violations := validateLoopDelivery(evidence)
			if !hasLoopIntegrityCode(violations, "delivery_metadata_exposed") {
				t.Fatalf("violations = %+v, want provenance refusal", violations)
			}
		})
	}
}

func TestValidateLoopDeliveryChecksChildFailureIntegrityAndRef(t *testing.T) {
	root, evidence := loopDeliveryFixture(t, "committed")
	loopDeliveryRun(t, root, "checkout", "-qb", "other")
	evidence.Postflight = loopDeliverySnapshot(t, root)
	evidence.ChildFailed = true
	evidence.IntegrityViolations = []MachineViolation{{Code: "selected_task_mutation"}}

	_, violations := validateLoopDelivery(evidence)
	for _, want := range []string{"delivery_ref_changed", "integrity_violation"} {
		if !hasLoopIntegrityCode(violations, want) {
			t.Fatalf("violations = %+v, want %s", violations, want)
		}
	}
}

func TestValidateLoopDeliveryChecksCleanChildFailureCommitShape(t *testing.T) {
	root, evidence := loopDeliveryFixture(t, "committed")
	evidence.ChildFailed = true
	evidence.Postflight = loopDeliverySnapshot(t, root)

	_, violations := validateLoopDelivery(evidence)
	if !hasLoopIntegrityCode(violations, "delivery_commit_shape") {
		t.Fatalf("violations = %+v, want clean child-failure delivery refusal", violations)
	}
}

func TestValidateLoopDeliveryLocalRequiresProductOnlyCommit(t *testing.T) {
	for _, test := range []struct {
		name          string
		changeProduct bool
		commitState   bool
		want          string
	}{
		{name: "product commit", changeProduct: true},
		{name: "unchanged product"},
		{name: "metadata only commit", commitState: true, want: "delivery_metadata_exposed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, evidence := loopDeliveryFixture(t, "local")
			if test.changeProduct {
				loopDeliveryWrite(t, root, "product.txt", "product changed\n")
			}
			if test.commitState {
				loopDeliveryWrite(t, root, "planning/STATE.md", "after\n")
				loopDeliveryRun(t, root, "add", "-f", "planning/STATE.md")
			}
			if test.changeProduct || test.commitState {
				loopDeliveryCommit(t, root, "delivery", !test.commitState)
			}
			evidence.Postflight = loopDeliverySnapshot(t, root)

			_, violations := validateLoopDelivery(evidence)
			if test.want == "" && len(violations) != 0 {
				t.Fatalf("violations = %+v", violations)
			}
			if test.want != "" && !hasLoopIntegrityCode(violations, test.want) {
				t.Fatalf("violations = %+v, want %s", violations, test.want)
			}
		})
	}
}

func TestValidateLoopDeliveryAllowsLocalBlockedAndReworkWithoutProductCommit(t *testing.T) {
	for _, candidate := range []string{"blocked_fail", "rework_fail"} {
		t.Run(candidate, func(t *testing.T) {
			_, evidence := loopDeliveryFixture(t, "local")
			evidence.LifecycleCandidate = candidate

			_, violations := validateLoopDelivery(evidence)
			if len(violations) != 0 {
				t.Fatalf("violations = %+v", violations)
			}
		})
	}
}

func TestValidateLoopDeliveryReportsFactsAndDefersChildFailureDirt(t *testing.T) {
	root, evidence := loopDeliveryFixture(t, "committed")
	loopDeliveryWrite(t, root, "uncommitted.txt", "evidence\n")
	evidence.Postflight = loopDeliverySnapshot(t, root)
	evidence.ChildFailed = true

	delivery, violations := validateLoopDelivery(evidence)
	if len(violations) != 0 || delivery.Remote != "not_checked" || delivery.Clean || !delivery.Descendant || len(delivery.Commits) != 0 {
		t.Fatalf("delivery = %+v, violations = %+v", delivery, violations)
	}
}

func loopDeliveryFixture(t *testing.T, mode string) (string, loopDeliveryEvidence) {
	t.Helper()
	root := t.TempDir()
	loopDeliveryRun(t, root, "init", "-q", "-b", "main")
	loopDeliveryRun(t, root, "config", "user.email", "operator@example.test")
	loopDeliveryRun(t, root, "config", "user.name", "Loop Operator")
	loopDeliveryWrite(t, root, "product.txt", "product\n")
	if mode == "local" {
		loopDeliveryCommit(t, root, "initial", true)
		loopDeliveryWrite(t, root, ".git/info/exclude", ".taskrail/\nplanning/\n")
	}
	loopDeliveryWrite(t, root, ".taskrail/config.yml", "layout_version: 2\n")
	loopDeliveryWrite(t, root, "planning/STATE.md", "before\n")
	loopDeliveryWrite(t, root, "planning/tasks/T-001-selected.md", "selected\n")
	if mode != "local" {
		loopDeliveryCommit(t, root, "initial", true)
	}

	preflight := loopDeliverySnapshot(t, root)
	inputs := map[string][]byte{
		".taskrail/config.yml":             []byte("layout_version: 2\n"),
		"planning/STATE.md":                []byte("before\n"),
		"planning/tasks/T-001-selected.md": []byte("selected\n"),
	}
	return root, loopDeliveryEvidence{
		Root: root, Preflight: LoopPreflightSnapshot{git: preflight, inputs: cloneLoopBytes(inputs), storage: LoopStorageSnapshot{Mode: mode, Root: "."}},
		Postflight: preflight, PostflightInputs: cloneLoopBytes(inputs), PlanningDir: "planning", SelectedTask: "T-001-selected",
		LifecycleCandidate: "completed_pass",
	}
}

func loopDeliverySnapshot(t *testing.T, root string) LoopGitSnapshot {
	t.Helper()
	gitDir := strings.TrimSpace(loopDeliveryRun(t, root, "rev-parse", "--git-dir"))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(root, gitDir)
	}
	snapshot, err := loopGitSnapshot(root, gitDir)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func loopDeliveryWrite(t *testing.T, root, name, content string) {
	t.Helper()
	filename := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func loopDeliveryCommit(t *testing.T, root, message string, stage bool) {
	t.Helper()
	if stage {
		loopDeliveryRun(t, root, "add", "-A")
	}
	loopDeliveryRun(t, root, "commit", "-qm", message)
}

func loopDeliveryRun(t *testing.T, root string, args ...string) string {
	t.Helper()
	output, err := gitCommand(root, args...)
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return output
}
