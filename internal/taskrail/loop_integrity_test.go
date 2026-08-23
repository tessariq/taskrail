package taskrail

import (
	"encoding/json"
	"path"
	"reflect"
	"strings"
	"testing"
)

func TestLoopIntegrityAllowsCanonicalLifecycleAndProvenFollowup(t *testing.T) {
	before := loopIntegrityFixtureInputs(t)
	after := cloneLoopBytes(before)
	after["planning/tasks/T-001-selected.md"] = loopIntegritySelectedLifecycleBytes(t, before["planning/tasks/T-001-selected.md"])
	after["planning/STATE.md"] = loopIntegrityState("after lifecycle")
	after["planning/tasks/T-002-followup.md"] = loopIntegrityTask("T-002-followup", "todo", []string{"T-001-selected"})
	report := VerificationArtifact{TaskID: "T-001-selected", VerificationID: strings.Repeat("a", 32), Result: "pass", FollowupTaskID: "T-002-followup"}
	reportBytes, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	after["planning/artifacts/verify/T-001-selected/20260823T120000Z-"+report.VerificationID+"/report.json"] = reportBytes
	after["planning/artifacts/verify/T-001-selected/20260823T120000Z-"+report.VerificationID+"/plan.md"] = []byte("plan\n")
	after["planning/artifacts/verify/T-001-selected/20260823T120000Z-"+report.VerificationID+"/report.md"] = []byte("report\n")

	violations := checkLoopIntegrity(loopIntegrityEvidenceFor(loopIntegrityPreflight(before), "T-001-selected", after))
	if len(violations) != 0 {
		t.Fatalf("violations = %+v", violations)
	}
}

func TestLoopIntegrityReportsOrderedFrozenAndLedgerMutations(t *testing.T) {
	before := loopIntegrityFixtureInputs(t)
	after := cloneLoopBytes(before)
	after[".taskrail/config.yml"] = []byte("changed\n")
	after["planning/tasks/T-009-sentinel.md"] = []byte(strings.Replace(string(after["planning/tasks/T-009-sentinel.md"]), "priority: high", "priority: low", 1))
	after["planning/tasks/T-001-selected.md"] = []byte(strings.Replace(string(after["planning/tasks/T-001-selected.md"]), "priority: high", "priority: low", 1))
	after["planning/tasks/T-002-unproven.md"] = loopIntegrityTask("T-002-unproven", "todo", []string{"T-001-selected"})

	evidence := loopIntegrityEvidenceFor(loopIntegrityPreflight(before), "T-001-selected", after)
	evidence.RootRefs = map[string][]byte{".git/EVIL_REV": []byte("two\n")}
	violations := checkLoopIntegrity(evidence)
	got := make([]string, len(violations))
	for i, violation := range violations {
		path := ""
		if violation.Path != nil {
			path = *violation.Path
		}
		got[i] = violation.Code + "|" + path
	}
	want := []string{
		"frozen_input_changed|.taskrail/config.yml",
		"ledger_task_changed|planning/tasks/T-009-sentinel.md",
		"ledger_task_created|planning/tasks/T-002-unproven.md",
		"root_ref_changed|.git/EVIL_REV",
		"selected_task_mutation|planning/tasks/T-001-selected.md",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("violations = %v, want %v", got, want)
	}
}

func TestLoopIntegrityRefusesUnprovenFollowupBoundaries(t *testing.T) {
	for _, test := range []struct {
		name      string
		task      []byte
		addReport bool
	}{
		{name: "missing report", task: loopIntegrityTask("T-002-followup", "todo", []string{"T-001-selected"})},
		{name: "wrong dependency", task: loopIntegrityTask("T-002-followup", "todo", []string{"T-009-sentinel"}), addReport: true},
		{name: "explicit policy", task: []byte(strings.Replace(string(loopIntegrityTask("T-002-followup", "todo", []string{"T-001-selected"})), "updated_at:", "loop_policy: allow\nloop_reason: not implicit\nupdated_at:", 1)), addReport: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := loopIntegrityFixtureInputs(t)
			after := cloneLoopBytes(before)
			after["planning/tasks/T-001-selected.md"] = loopIntegritySelectedLifecycleBytes(t, before["planning/tasks/T-001-selected.md"])
			after["planning/STATE.md"] = loopIntegrityState("after lifecycle")
			after["planning/tasks/T-002-followup.md"] = test.task
			if test.addReport {
				loopIntegrityAddFollowupReport(t, after, "T-002-followup")
			}
			violations := checkLoopIntegrity(loopIntegrityEvidenceFor(loopIntegrityPreflight(before), "T-001-selected", after))
			if !hasLoopIntegrityCode(violations, "ledger_task_created") {
				t.Fatalf("violations = %+v, want unproven follow-up refusal", violations)
			}
		})
	}
}

func TestLoopIntegrityFreezesActiveSpecAndReportsNullPathsLast(t *testing.T) {
	before := loopIntegrityFixtureInputs(t)
	after := cloneLoopBytes(before)
	after["planning/STATE.md"] = []byte("---\nactive_spec_version: v0.6.0\nactive_spec_path: specs/v0.6.0.md\n---\nafter\n")
	changedReview := loopIntegrityPreflight(before).Review()
	changedReview.EffectiveMaxRounds = 2
	evidence := loopIntegrityEvidenceFor(loopIntegrityPreflight(before), "", after)
	evidence.Review = &changedReview
	violations := checkLoopIntegrity(evidence)
	if len(violations) != 2 || violations[0].Code != "review_policy_changed" || violations[0].Path != nil || violations[1].Code != "state_mutation" || violations[1].Path == nil {
		t.Fatalf("violations = %+v, want ordered null-path review and state violations", violations)
	}
}

func TestLoopIntegrityUsesConfiguredPlanningRoot(t *testing.T) {
	before := loopIntegrityFixtureInputs(t)
	after := make(map[string][]byte, len(before))
	for inputPath, data := range before {
		after[strings.Replace(inputPath, "planning/", "work/", 1)] = data
	}
	preflight := loopIntegrityPreflight(after)
	after["work/tasks/T-001-selected.md"] = loopIntegritySelectedLifecycleBytes(t, before["planning/tasks/T-001-selected.md"])
	after["work/STATE.md"] = loopIntegrityState("after lifecycle")
	evidence := loopIntegrityEvidenceFor(preflight, "T-001-selected", after)
	evidence.PlanningDir = "work"
	evidence.VerifyDir = "work/artifacts/verify"
	violations := checkLoopIntegrity(evidence)
	if len(violations) != 0 {
		t.Fatalf("violations = %+v", violations)
	}
}

func TestLoopIntegrityRejectsReplayedReportAndQuotedFollowupPolicy(t *testing.T) {
	before := loopIntegrityFixtureInputs(t)
	previous := VerificationArtifact{TaskID: "T-001-selected", VerificationID: strings.Repeat("a", 32), Result: "pass"}
	previousBytes, err := json.Marshal(previous)
	if err != nil {
		t.Fatal(err)
	}
	before["planning/artifacts/verify/T-001-selected/20260822T120000Z-"+previous.VerificationID+"/report.json"] = previousBytes
	after := cloneLoopBytes(before)
	after["planning/tasks/T-001-selected.md"] = loopIntegritySelectedLifecycleBytes(t, before["planning/tasks/T-001-selected.md"])
	after["planning/STATE.md"] = loopIntegrityState("after lifecycle")
	after["planning/tasks/T-002-followup.md"] = []byte(strings.Replace(string(loopIntegrityTask("T-002-followup", "todo", []string{"T-001-selected"})), "updated_at:", "\"loop_policy\": allow\nupdated_at:", 1))
	loopIntegrityAddFollowupReport(t, after, "T-002-followup")
	violations := checkLoopIntegrity(loopIntegrityEvidenceFor(loopIntegrityPreflight(before), "T-001-selected", after))
	if !hasLoopIntegrityCode(violations, "ledger_task_created") || !hasLoopIntegrityCode(violations, "verification_artifact_created") {
		t.Fatalf("violations = %+v, want quoted-policy and replay refusals", violations)
	}
}

func TestLoopIntegrityFreezesStateNotesAndRequiresPostflightControls(t *testing.T) {
	before := loopIntegrityFixtureInputs(t)
	after := cloneLoopBytes(before)
	after["planning/STATE.md"] = []byte("---\nactive_spec_version: v0.5.0\nactive_spec_path: specs/v0.5.0.md\ncontinuation_notes: [rewritten]\n---\nafter\n")
	violations := checkLoopIntegrity(loopIntegrityEvidence{Preflight: loopIntegrityPreflight(before), Inputs: after, Git: loopIntegrityPreflight(before).Git(), RootRefs: map[string][]byte{".git/EVIL_REV": []byte("one\n")}})
	for _, code := range []string{"state_mutation", "postflight_evidence_missing"} {
		if !hasLoopIntegrityCode(violations, code) {
			t.Fatalf("violations = %+v, want %s", violations, code)
		}
	}
}

func TestLoopIntegrityRejectsFrozenTaskVerificationIDReplay(t *testing.T) {
	before := loopIntegrityFixtureInputs(t)
	selected := "planning/tasks/T-001-selected.md"
	before[selected] = []byte(strings.Replace(string(before[selected]), "updated_at:", "last_verification_id: \""+strings.Repeat("a", 32)+"\"\nupdated_at:", 1))
	after := cloneLoopBytes(before)
	after[selected] = loopIntegritySelectedLifecycleBytes(t, before[selected])
	after["planning/STATE.md"] = loopIntegrityState("after lifecycle")
	after["planning/tasks/T-002-followup.md"] = loopIntegrityTask("T-002-followup", "todo", []string{"T-001-selected"})
	loopIntegrityAddFollowupReport(t, after, "T-002-followup")
	violations := checkLoopIntegrity(loopIntegrityEvidenceFor(loopIntegrityPreflight(before), "T-001-selected", after))
	if !hasLoopIntegrityCode(violations, "ledger_task_created") || !hasLoopIntegrityCode(violations, "verification_artifact_created") {
		t.Fatalf("violations = %+v, want frozen-ID replay refusal", violations)
	}
}

func TestLoopFreshVerificationAllowancesRejectsCyclicChain(t *testing.T) {
	first, second := strings.Repeat("a", 32), strings.Repeat("b", 32)
	firstPrevious, secondPrevious := second, first
	post := map[string]VerificationArtifact{
		"planning/artifacts/verify/T-001-selected/20260823T120000Z-" + first + "/report.json":  {TaskID: "T-001-selected", VerificationID: first, PreviousVerificationID: &firstPrevious, FollowupTaskID: "T-002-first"},
		"planning/artifacts/verify/T-001-selected/20260823T120001Z-" + second + "/report.json": {TaskID: "T-001-selected", VerificationID: second, PreviousVerificationID: &secondPrevious, FollowupTaskID: "T-003-second"},
	}
	selected := []byte(strings.Replace(string(loopIntegrityTask("T-001-selected", "completed", nil)), "updated_at:", "last_verification_id: \""+first+"\"\nupdated_at:", 1))
	inputs := make(map[string][]byte)
	for reportPath := range post {
		dir := path.Dir(reportPath)
		inputs[path.Join(dir, "plan.md")] = []byte("plan")
		inputs[path.Join(dir, "report.json")] = []byte("report")
		inputs[path.Join(dir, "report.md")] = []byte("report")
	}
	artifacts, followups := loopFreshVerificationAllowances(map[string]bool{}, post, selected, "T-001-selected", "planning/artifacts/verify", inputs)
	if len(artifacts) != 0 || len(followups) != 0 {
		t.Fatalf("cycle allowances = %v, %v, want none", artifacts, followups)
	}
}

func TestLoopIntegrityOrdersNullPathEvidenceMessages(t *testing.T) {
	before := loopIntegrityFixtureInputs(t)
	violations := checkLoopIntegrity(loopIntegrityEvidence{Preflight: loopIntegrityPreflight(before), Inputs: before, Git: loopIntegrityPreflight(before).Git(), RootRefs: map[string][]byte{".git/EVIL_REV": []byte("one\n")}})
	got := make([]string, 0)
	for _, violation := range violations {
		if violation.Code == "postflight_evidence_missing" {
			got = append(got, violation.Message)
		}
	}
	want := []string{"prompt observation is required", "review policy observation is required", "staged executable observation is required", "storage observation is required"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("missing evidence messages = %v, want %v", got, want)
	}
}

func loopIntegrityEvidenceFor(preflight LoopPreflightSnapshot, selected string, inputs map[string][]byte) loopIntegrityEvidence {
	storage := preflight.Storage()
	review := preflight.Review()
	prompt := LoopPromptExecution{ID: "task-implementation", Source: "builtin", TemplateSHA256: "template", RenderedSHA256: "rendered", OverrideAuthorized: true}
	executable := loopChildIdentity{Executable: "/git/taskrail-staged", SHA256: strings.Repeat("a", 64), InvocationID: strings.Repeat("b", 32), Token: "secret"}
	return loopIntegrityEvidence{
		Preflight: preflight, SelectedTask: selected, Inputs: inputs, Git: preflight.Git(),
		RootRefs: map[string][]byte{".git/EVIL_REV": []byte("one\n")}, Storage: &storage,
		Review: &review, ExpectedPrompt: &prompt, Prompt: &prompt,
		ExpectedExecutable: &executable, Executable: &executable,
	}
}

func loopIntegrityFixtureInputs(t *testing.T) map[string][]byte {
	t.Helper()
	return map[string][]byte{
		".taskrail/config.yml":                    []byte("layout_version: 2\n"),
		"specs/v0.5.0.md":                         []byte("# Active spec\n"),
		"planning/STATE.md":                       loopIntegrityState("before lifecycle"),
		"planning/tasks/T-001-selected.md":        loopIntegrityTask("T-001-selected", "todo", nil),
		"planning/tasks/T-009-sentinel.md":        loopIntegrityTask("T-009-sentinel", "todo", nil),
		"planning/artifacts/verify/old/report.md": []byte("old report\n"),
	}
}

func loopIntegrityState(note string) []byte {
	return []byte("---\nactive_spec_version: v0.5.0\nactive_spec_path: specs/v0.5.0.md\n---\n" + note + "\n")
}

func loopIntegrityPreflight(inputs map[string][]byte) LoopPreflightSnapshot {
	return LoopPreflightSnapshot{
		inputs:   cloneLoopBytes(inputs),
		git:      LoopGitSnapshot{Ref: "refs/heads/main", Refs: map[string]string{"refs/heads/main": "before", "refs/tags/v1": "tag"}},
		storage:  LoopStorageSnapshot{Mode: "committed", Root: "."},
		review:   LoopReviewSnapshot{ConfiguredMaxRounds: 1, EffectiveMaxRounds: 1, Source: "config", MaxReviewersPerRound: 3, FinalDiffReviewRequiredOnChange: true},
		rootRefs: map[string][]byte{".git/EVIL_REV": []byte("one\n")},
	}
}

func loopIntegrityTask(id, status string, dependencies []string) []byte {
	dependenciesJSON, _ := json.Marshal(dependencies)
	return []byte("---\n" +
		"id: " + id + "\n" +
		"title: " + id + " title\n" +
		"status: " + status + "\n" +
		"priority: high\n" +
		"spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop\n" +
		"dependencies: " + string(dependenciesJSON) + "\n" +
		"updated_at: \"2026-08-23T12:00:00Z\"\n" +
		"sentinel: preserve\n" +
		"---\n\n# " + id + "\n\n## Description\n\nfixed\n\n## Acceptance\n\nfixed\n\n## Verification Notes\n\nfixed\n\n## Implementation Notes\n")
}

func loopIntegritySelectedLifecycleBytes(t *testing.T, before []byte) []byte {
	t.Helper()
	frontmatter, body, err := parseFrontmatter[TaskFrontmatter](before)
	if err != nil {
		t.Fatal(err)
	}
	frontmatter.Status = "completed"
	frontmatter.UpdatedAt = "2026-08-23T12:05:00Z"
	frontmatter.CompletionID = strings.Repeat("b", 32)
	frontmatter.LastVerificationID = strings.Repeat("a", 32)
	frontmatter.LastVerificationResult = "pass"
	frontmatter.LastVerifiedAt = "2026-08-23T12:05:00Z"
	frontmatter.LastVerifiedCompletionID = frontmatter.CompletionID
	task := &Task{Frontmatter: frontmatter, Body: body + "\n- lifecycle note\n"}
	patched, err := patchLifecycleTaskBytes(before, task, map[string]string{
		"status":     frontmatter.Status,
		"updated_at": "\"" + frontmatter.UpdatedAt + "\"",
	})
	if err != nil {
		t.Fatal(err)
	}
	patched, err = patchVerificationMetadata(patched, task)
	if err != nil {
		t.Fatal(err)
	}
	return patched
}

func loopIntegrityAddFollowupReport(t *testing.T, inputs map[string][]byte, followup string) {
	t.Helper()
	report := VerificationArtifact{TaskID: "T-001-selected", VerificationID: strings.Repeat("a", 32), Result: "pass", FollowupTaskID: followup}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	dir := "planning/artifacts/verify/T-001-selected/20260823T120000Z-" + report.VerificationID
	inputs[dir+"/plan.md"] = []byte("plan\n")
	inputs[dir+"/report.json"] = data
	inputs[dir+"/report.md"] = []byte("report\n")
}

func hasLoopIntegrityCode(violations []MachineViolation, code string) bool {
	for _, violation := range violations {
		if violation.Code == code {
			return true
		}
	}
	return false
}
