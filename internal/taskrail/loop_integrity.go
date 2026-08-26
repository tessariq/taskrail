package taskrail

import (
	"bytes"
	"encoding/json"
	"path"
	"reflect"
	"sort"
	"strings"
)

// loopIntegrityEvidence is the frozen preflight material and the post-child
// observations needed to decide whether a child stayed within its ledger grant.
// Delivery shape remains T-313's responsibility; this checker only preserves
// inputs that delivery does not authorize changing.
type loopIntegrityEvidence struct {
	Preflight          LoopPreflightSnapshot
	SelectedTask       string
	PlanningDir        string
	VerifyDir          string
	Inputs             map[string][]byte
	Git                LoopGitSnapshot
	RootRefs           map[string][]byte
	GitConfig          map[string]loopGitConfigFile
	Storage            *LoopStorageSnapshot
	Review             *LoopReviewSnapshot
	Prompt             *LoopPromptExecution
	ExpectedPrompt     *LoopPromptExecution
	Executable         *loopChildIdentity
	ExpectedExecutable *loopChildIdentity
}

// checkLoopIntegrity reports all post-child input and ledger violations. Its
// stable ordering lets the eventual loop diagnostic remain deterministic even
// when several boundaries were changed by the same child.
func checkLoopIntegrity(evidence loopIntegrityEvidence) []MachineViolation {
	planningDir := evidence.PlanningDir
	if planningDir == "" {
		planningDir = "planning"
	}
	verifyDir := evidence.VerifyDir
	if verifyDir == "" {
		verifyDir = planningDir + "/artifacts/verify"
	}
	inputs := evidence.Preflight.Inputs()
	violations := make([]MachineViolation, 0)

	if evidence.Storage == nil {
		violations = append(violations, loopIntegrityViolation("postflight_evidence_missing", "storage observation is required", nil))
	} else if *evidence.Storage != evidence.Preflight.Storage() {
		violations = append(violations, loopIntegrityViolation("storage_changed", "storage mode or root changed", nil))
	}
	if evidence.Review == nil {
		violations = append(violations, loopIntegrityViolation("postflight_evidence_missing", "review policy observation is required", nil))
	} else if *evidence.Review != evidence.Preflight.Review() {
		violations = append(violations, loopIntegrityViolation("review_policy_changed", "review policy changed", nil))
	}
	if evidence.ExpectedPrompt == nil || evidence.Prompt == nil {
		violations = append(violations, loopIntegrityViolation("postflight_evidence_missing", "prompt observation is required", nil))
	} else if !sameLoopPrompt(*evidence.ExpectedPrompt, evidence.Prompt) {
		violations = append(violations, loopIntegrityViolation("prompt_changed", "frozen prompt changed", nil))
	}
	if evidence.ExpectedExecutable == nil || evidence.Executable == nil {
		violations = append(violations, loopIntegrityViolation("postflight_evidence_missing", "staged executable observation is required", nil))
	} else if *evidence.ExpectedExecutable != *evidence.Executable {
		violations = append(violations, loopIntegrityViolation("executable_changed", "staged executable changed", nil))
	}

	preTasks := loopTaskInputs(inputs, planningDir)
	postTasks := loopTaskInputs(evidence.Inputs, planningDir)
	postReports := loopVerificationReports(evidence.Inputs, verifyDir)
	allowedArtifacts, provenFollowups := loopFreshVerificationAllowances(loopFrozenVerificationIDs(inputs, planningDir, verifyDir), postReports, postTasks[loopSelectedTaskPath(planningDir, evidence.SelectedTask)], evidence.SelectedTask, verifyDir, evidence.Inputs)
	if !sameLoopFrozenState(inputs[planningDir+"/STATE.md"], evidence.Inputs[planningDir+"/STATE.md"]) {
		statePath := planningDir + "/STATE.md"
		violations = append(violations, loopIntegrityViolation("state_mutation", "frozen state fields changed during child execution", &statePath))
	}

	for inputPath, before := range inputs {
		if inputPath == planningDir+"/STATE.md" || isLoopTaskPath(inputPath, planningDir) || isLoopVerificationPath(inputPath, verifyDir) {
			continue
		}
		after, present := evidence.Inputs[inputPath]
		if !present {
			violations = append(violations, loopIntegrityViolation("frozen_input_deleted", "frozen input was deleted", &inputPath))
		} else if !bytes.Equal(before, after) {
			violations = append(violations, loopIntegrityViolation("frozen_input_changed", "frozen input changed", &inputPath))
		}
	}
	for inputPath := range evidence.Inputs {
		if _, present := inputs[inputPath]; present || inputPath == planningDir+"/STATE.md" || isLoopTaskPath(inputPath, planningDir) || isLoopVerificationPath(inputPath, verifyDir) {
			continue
		}
		violations = append(violations, loopIntegrityViolation("frozen_input_created", "new frozen input was created", &inputPath))
	}

	for inputPath, before := range preTasks {
		after, present := postTasks[inputPath]
		if !present {
			violations = append(violations, loopIntegrityViolation("ledger_task_deleted", "pre-existing task was deleted", &inputPath))
			continue
		}
		if inputPath == loopSelectedTaskPath(planningDir, evidence.SelectedTask) {
			if !bytes.Equal(before, after) && !validSelectedTaskMutation(before, after) {
				violations = append(violations, loopIntegrityViolation("selected_task_mutation", "selected task changed outside canonical lifecycle fields", &inputPath))
			}
			continue
		}
		if !bytes.Equal(before, after) {
			violations = append(violations, loopIntegrityViolation("ledger_task_changed", "non-selected task changed", &inputPath))
		}
	}
	for inputPath, after := range postTasks {
		if _, present := preTasks[inputPath]; present {
			continue
		}
		if !validLoopFollowup(after, evidence.SelectedTask, provenFollowups) {
			violations = append(violations, loopIntegrityViolation("ledger_task_created", "new task is not a proven implicit follow-up", &inputPath))
		}
	}

	for inputPath, before := range inputs {
		if !isLoopVerificationPath(inputPath, verifyDir) {
			continue
		}
		after, present := evidence.Inputs[inputPath]
		if !present {
			violations = append(violations, loopIntegrityViolation("verification_artifact_deleted", "pre-existing verification artifact was deleted", &inputPath))
		} else if !bytes.Equal(before, after) {
			violations = append(violations, loopIntegrityViolation("verification_artifact_changed", "pre-existing verification artifact changed", &inputPath))
		}
	}
	for inputPath := range evidence.Inputs {
		if _, present := inputs[inputPath]; present || !isLoopVerificationPath(inputPath, verifyDir) {
			continue
		}
		if !allowedArtifacts[inputPath] {
			violations = append(violations, loopIntegrityViolation("verification_artifact_created", "new verification artifact is not part of a fresh selected-task report", &inputPath))
		}
	}

	preGit := evidence.Preflight.Git()
	for ref, object := range preGit.Refs {
		if ref == preGit.Ref {
			continue
		}
		if evidence.Git.Refs[ref] != object {
			violations = append(violations, loopIntegrityViolation("git_ref_changed", "non-attached Git ref changed", &ref))
		}
	}
	for ref := range evidence.Git.Refs {
		if ref == preGit.Ref {
			continue
		}
		if _, present := preGit.Refs[ref]; !present {
			violations = append(violations, loopIntegrityViolation("git_ref_created", "new non-attached Git ref was created", &ref))
		}
	}
	for inputPath, before := range evidence.Preflight.RootRefs() {
		after, present := evidence.RootRefs[inputPath]
		if !present {
			violations = append(violations, loopIntegrityViolation("root_ref_deleted", "captured Git root candidate was deleted", &inputPath))
		} else if !bytes.Equal(before, after) {
			violations = append(violations, loopIntegrityViolation("root_ref_changed", "captured Git root candidate changed", &inputPath))
		}
	}
	for inputPath := range evidence.RootRefs {
		if _, present := evidence.Preflight.RootRefs()[inputPath]; !present {
			violations = append(violations, loopIntegrityViolation("root_ref_created", "new Git root candidate was created", &inputPath))
		}
	}
	if evidence.GitConfig == nil {
		violations = append(violations, loopIntegrityViolation("postflight_evidence_missing", "Git configuration observation is required", nil))
	} else {
		for configPath, before := range evidence.Preflight.GitConfig() {
			after, present := evidence.GitConfig[configPath]
			if !present || before.Present != after.Present || before.Snapshot != after.Snapshot || !bytes.Equal(before.Bytes, after.Bytes) {
				violations = append(violations, loopIntegrityViolation("git_config_changed", "repository-owned Git configuration changed", &configPath))
			}
		}
		for configPath := range evidence.GitConfig {
			if _, present := evidence.Preflight.GitConfig()[configPath]; !present {
				violations = append(violations, loopIntegrityViolation("git_config_changed", "repository-owned Git configuration changed", &configPath))
			}
		}
	}

	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Code != violations[j].Code {
			return violations[i].Code < violations[j].Code
		}
		if violations[i].Path == nil || violations[j].Path == nil {
			if violations[i].Path == nil && violations[j].Path == nil {
				return violations[i].Message < violations[j].Message
			}
			return violations[i].Path != nil
		}
		if *violations[i].Path != *violations[j].Path {
			return *violations[i].Path < *violations[j].Path
		}
		return violations[i].Message < violations[j].Message
	})
	return violations
}

func sameLoopFrozenState(before, after []byte) bool {
	left, _, leftErr := parseFrontmatter[StateFrontmatter](before)
	right, _, rightErr := parseFrontmatter[StateFrontmatter](after)
	return leftErr == nil && rightErr == nil && left.SchemaVersion == right.SchemaVersion && left.ActiveSpecVersion == right.ActiveSpecVersion && left.ActiveSpecPath == right.ActiveSpecPath && reflect.DeepEqual(left.ContinuationNotes, right.ContinuationNotes)
}

func loopIntegrityViolation(code, message string, inputPath *string) MachineViolation {
	if inputPath == nil {
		return MachineViolation{Code: code, Message: message}
	}
	value := *inputPath
	return MachineViolation{Code: code, Message: message, Path: &value}
}

func loopTaskInputs(inputs map[string][]byte, planningDir string) map[string][]byte {
	tasks := make(map[string][]byte)
	for inputPath, data := range inputs {
		if isLoopTaskPath(inputPath, planningDir) {
			tasks[inputPath] = data
		}
	}
	return tasks
}

func isLoopTaskPath(inputPath, planningDir string) bool {
	return strings.HasPrefix(inputPath, planningDir+"/tasks/") && path.Ext(inputPath) == ".md" && path.Dir(strings.TrimPrefix(inputPath, planningDir+"/tasks/")) == "."
}

func loopSelectedTaskPath(planningDir, taskID string) string {
	return planningDir + "/tasks/" + taskID + ".md"
}

func validSelectedTaskMutation(before, after []byte) bool {
	final, body, err := parseFrontmatter[TaskFrontmatter](after)
	if err != nil {
		return false
	}
	task := &Task{Frontmatter: final, Body: body}
	expected, err := patchLifecycleTaskBytes(before, task, map[string]string{
		"status":     final.Status,
		"updated_at": "\"" + final.UpdatedAt + "\"",
	})
	if err != nil {
		return false
	}
	expected, err = patchVerificationMetadata(expected, task)
	return err == nil && bytes.Equal(expected, after)
}

func validLoopFollowup(data []byte, selected string, proven map[string]bool) bool {
	frontmatter, _, err := parseFrontmatter[TaskFrontmatter](data)
	policyPresent, reasonPresent := loopPolicyPresence(frontmatter.LoopPolicyMetadata)
	if err != nil || frontmatter.Status != "todo" || !proven[frontmatter.ID] || policyPresent || reasonPresent {
		return false
	}
	for _, dependency := range frontmatter.Dependencies {
		if dependency == selected {
			return true
		}
	}
	return false
}

func isLoopVerificationPath(inputPath, verifyDir string) bool {
	return strings.HasPrefix(inputPath, verifyDir+"/")
}

func loopVerificationReports(inputs map[string][]byte, verifyDir string) map[string]VerificationArtifact {
	reports := make(map[string]VerificationArtifact)
	for inputPath, data := range inputs {
		if !strings.HasPrefix(inputPath, verifyDir+"/") || path.Base(inputPath) != "report.json" {
			continue
		}
		var report VerificationArtifact
		if json.Unmarshal(data, &report) == nil && report.VerificationID != "" {
			reports[inputPath] = report
		}
	}
	return reports
}

func loopFrozenVerificationIDs(inputs map[string][]byte, planningDir, verifyDir string) map[string]bool {
	ids := make(map[string]bool)
	for _, report := range loopVerificationReports(inputs, verifyDir) {
		ids[report.VerificationID] = true
	}
	for inputPath, data := range inputs {
		if isLoopTaskPath(inputPath, planningDir) {
			frontmatter, _, err := parseFrontmatter[TaskFrontmatter](data)
			if err == nil {
				ids[frontmatter.LastVerificationID] = frontmatter.LastVerificationID != ""
				ids[frontmatter.LastVerificationPreviousID] = frontmatter.LastVerificationPreviousID != ""
			}
		}
	}
	if state, _, err := parseFrontmatter[StateFrontmatter](inputs[planningDir+"/STATE.md"]); err == nil {
		ids[state.LastVerificationID] = state.LastVerificationID != ""
		ids[state.LastVerificationPreviousID] = state.LastVerificationPreviousID != ""
	}
	for inputPath := range inputs {
		if !isLoopVerificationPath(inputPath, verifyDir) {
			continue
		}
		name := path.Base(path.Dir(inputPath))
		if dash := strings.LastIndex(name, "-"); dash >= 0 && lowerHex32.MatchString(name[dash+1:]) {
			ids[name[dash+1:]] = true
		}
	}
	return ids
}

func loopFreshVerificationAllowances(preIDs map[string]bool, post map[string]VerificationArtifact, selectedTask []byte, selected, verifyDir string, inputs map[string][]byte) (map[string]bool, map[string]bool) {
	artifacts := make(map[string]bool)
	followups := make(map[string]bool)
	fresh := make(map[string]VerificationArtifact)
	paths := make(map[string]string)
	duplicates := make(map[string]bool)
	for reportPath, report := range post {
		if preIDs[report.VerificationID] || report.TaskID != selected || !lowerHex32.MatchString(report.VerificationID) {
			continue
		}
		dir := path.Dir(reportPath)
		if path.Base(dir) == "" || !strings.HasSuffix(path.Base(dir), "-"+report.VerificationID) || !strings.HasPrefix(dir, verifyDir+"/"+selected+"/") {
			continue
		}
		if _, present := fresh[report.VerificationID]; present {
			duplicates[report.VerificationID] = true
			continue
		}
		fresh[report.VerificationID] = report
		paths[report.VerificationID] = dir
	}
	frontmatter, _, err := parseFrontmatter[TaskFrontmatter](selectedTask)
	if err != nil {
		return artifacts, followups
	}
	seen := make(map[string]bool)
	for id := frontmatter.LastVerificationID; id != ""; {
		if seen[id] {
			return make(map[string]bool), make(map[string]bool)
		}
		seen[id] = true
		report, present := fresh[id]
		if !present || duplicates[id] {
			break
		}
		complete := true
		for _, name := range []string{"plan.md", "report.json", "report.md"} {
			if _, present := inputs[path.Join(paths[id], name)]; !present {
				complete = false
			}
		}
		if !complete {
			break
		}
		for _, name := range []string{"plan.md", "report.json", "report.md"} {
			artifacts[path.Join(paths[id], name)] = true
		}
		if report.FollowupTaskID != "" {
			followups[report.FollowupTaskID] = true
		}
		if report.PreviousVerificationID == nil {
			break
		}
		id = *report.PreviousVerificationID
	}
	return artifacts, followups
}

func sameLoopPrompt(expected LoopPromptExecution, actual *LoopPromptExecution) bool {
	return actual != nil && reflect.DeepEqual(expected, *actual)
}
