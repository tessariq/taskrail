package taskrail

import (
	"bytes"
	"path"
	"sort"
	"strings"
)

// LoopDelivery is the Git portion of the postflight diagnostic. It describes
// only local repository evidence; remote delivery is deliberately not probed.
type LoopDelivery struct {
	Ref        string   `json:"ref"`
	HeadBefore string   `json:"head_before"`
	HeadAfter  string   `json:"head_after"`
	Clean      bool     `json:"clean"`
	Descendant bool     `json:"descendant"`
	Commits    []string `json:"commits"`
	Remote     string   `json:"remote"`
}

// loopDeliveryEvidence combines T-312's integrity result with the Git
// observations needed to decide whether a lifecycle result was delivered.
type loopDeliveryEvidence struct {
	Root                string
	Preflight           LoopPreflightSnapshot
	Postflight          LoopGitSnapshot
	PostflightInputs    map[string][]byte
	PlanningDir         string
	VerifyDir           string
	SelectedTask        string
	LifecycleCandidate  string
	IntegrityViolations []MachineViolation
	AllowedProductPaths map[string]bool
	ChildFailed         bool
}

// validateLoopDelivery rejects visible delivery shapes that do not match the
// active storage contract. It is observation-only and never repairs Git.
func validateLoopDelivery(evidence loopDeliveryEvidence) (LoopDelivery, []MachineViolation) {
	before := evidence.Preflight.Git()
	after := evidence.Postflight
	delivery := LoopDelivery{
		Ref: before.Ref, HeadBefore: before.Head, HeadAfter: after.Head,
		Clean: after.Clean, Remote: "not_checked",
	}
	delivery.Descendant, delivery.Commits = loopDeliveryHistory(evidence.Root, before.Head, after.Head)

	violations := make([]MachineViolation, 0)
	if !loopKnownLifecycleCandidate(evidence.LifecycleCandidate) {
		violations = append(violations, loopDeliveryViolation("delivery_candidate_unknown", "lifecycle candidate is not recognized"))
	}
	if len(evidence.IntegrityViolations) != 0 {
		violations = append(violations, loopDeliveryViolation("integrity_violation", "T-312 integrity violations prevent delivery"))
	}
	if !after.Clean && !evidence.ChildFailed {
		violations = append(violations, loopDeliveryViolation("delivery_dirty", "delivered lifecycle requires a clean visible worktree"))
	}
	if after.Detached || after.Ref != before.Ref {
		violations = append(violations, loopDeliveryViolation("delivery_ref_changed", "delivered lifecycle changed the attached ref"))
	}
	if !delivery.Descendant {
		violations = append(violations, loopDeliveryViolation("delivery_not_descendant", "postflight HEAD is not descended from preflight HEAD"))
	}
	if evidence.ChildFailed && !after.Clean && before.Head == after.Head {
		sortLoopDeliveryViolations(violations)
		return delivery, violations
	}

	mode := evidence.Preflight.Storage().Mode
	switch mode {
	case string(StorageCommitted):
		violations = append(violations, validateCommittedLoopDelivery(evidence, delivery)...)
	case string(StorageLocal):
		violations = append(violations, validateLocalLoopDelivery(evidence, delivery)...)
	default:
		violations = append(violations, loopDeliveryViolation("delivery_storage_unknown", "storage mode is not recognized"))
	}
	sortLoopDeliveryViolations(violations)
	return delivery, violations
}

func validateCommittedLoopDelivery(evidence loopDeliveryEvidence, delivery LoopDelivery) []MachineViolation {
	if !loopDirectChild(evidence.Root, delivery.HeadBefore, delivery.HeadAfter, delivery.Commits) {
		return []MachineViolation{loopDeliveryViolation("delivery_commit_shape", "committed delivery requires exactly one direct-child commit")}
	}
	paths, ok := loopDeliveryCommitPaths(evidence.Root, delivery.HeadAfter)
	if !ok {
		return []MachineViolation{loopDeliveryViolation("delivery_evidence_missing", "cannot inspect committed delivery tree")}
	}
	changed := loopChangedDeliveryInputs(evidence.Preflight.Inputs(), evidence.PostflightInputs)
	violations := make([]MachineViolation, 0)
	for inputPath := range changed {
		if loopDeliveryVerificationPath(evidence, inputPath) {
			continue
		}
		if !paths[inputPath] {
			violations = append(violations, loopDeliveryViolation("delivery_metadata_missing", "committed delivery omitted generated Taskrail metadata"))
			break
		}
	}
	if loopDeliveryHasUnexpectedProductPath(paths, evidence) {
		violations = append(violations, loopDeliveryViolation("delivery_product_path_unexpected", "committed delivery changed a product path outside frozen policy"))
	}
	return violations
}

func validateLocalLoopDelivery(evidence loopDeliveryEvidence, delivery LoopDelivery) []MachineViolation {
	violations := make([]MachineViolation, 0)
	if paths, ok := loopDeliveryTrackedPaths(evidence.Root); ok && loopDeliveryHasManagedPath(paths, evidence) {
		violations = append(violations, loopDeliveryViolation("delivery_metadata_exposed", "local Taskrail metadata is tracked"))
	}
	if !delivery.Clean {
		if paths, ok := loopDeliveryIndexPaths(evidence.Root); ok && loopDeliveryHasManagedPath(paths, evidence) {
			violations = append(violations, loopDeliveryViolation("delivery_metadata_exposed", "local Taskrail metadata is staged"))
		}
		return violations
	}
	if delivery.HeadBefore == delivery.HeadAfter {
		return violations
	}
	if !loopDirectChild(evidence.Root, delivery.HeadBefore, delivery.HeadAfter, delivery.Commits) {
		return append(violations, loopDeliveryViolation("delivery_commit_shape", "local delivery requires an unchanged HEAD or one direct-child product commit"))
	}
	paths, ok := loopDeliveryCommitPaths(evidence.Root, delivery.HeadAfter)
	if !ok {
		return append(violations, loopDeliveryViolation("delivery_evidence_missing", "cannot inspect local delivery tree"))
	}
	if loopDeliveryHasManagedPath(paths, evidence) {
		return append(violations, loopDeliveryViolation("delivery_metadata_exposed", "local delivery commit contains Taskrail metadata"))
	}
	if loopDeliveryMetadataLeaks(evidence.Root, delivery.HeadAfter, evidence) {
		return append(violations, loopDeliveryViolation("delivery_metadata_exposed", "local delivery commit metadata exposes Taskrail provenance"))
	}
	if loopDeliveryHasUnexpectedProductPath(paths, evidence) {
		return append(violations, loopDeliveryViolation("delivery_product_path_unexpected", "local delivery changed a product path outside frozen policy"))
	}
	return violations
}

func loopDeliveryHistory(root, before, after string) (bool, []string) {
	if root == "" || before == "" || after == "" {
		return false, nil
	}
	if _, err := gitCommand(root, "merge-base", "--is-ancestor", before, after); err != nil {
		return false, nil
	}
	output, err := gitCommand(root, "rev-list", "--reverse", before+".."+after)
	if err != nil {
		return false, nil
	}
	return true, strings.Fields(output)
}

func loopDirectChild(root, before, after string, commits []string) bool {
	if len(commits) != 1 || root == "" || before == "" || after == "" {
		return false
	}
	output, err := gitCommand(root, "rev-list", "--parents", "-n", "1", after)
	if err != nil {
		return false
	}
	fields := strings.Fields(output)
	return len(fields) == 2 && fields[0] == after && fields[1] == before
}

func loopDeliveryCommitPaths(root, commit string) (map[string]bool, bool) {
	output, err := gitCommand(root, "diff-tree", "--no-commit-id", "--name-only", "-z", "-r", commit)
	if err != nil {
		return nil, false
	}
	return loopDeliveryPathSet(output), true
}

func loopDeliveryIndexPaths(root string) (map[string]bool, bool) {
	output, err := gitCommand(root, "diff", "--cached", "--name-only", "-z")
	if err != nil {
		return nil, false
	}
	return loopDeliveryPathSet(output), true
}

func loopDeliveryTrackedPaths(root string) (map[string]bool, bool) {
	output, err := gitCommand(root, "ls-files", "-z")
	if err != nil {
		return nil, false
	}
	return loopDeliveryPathSet(output), true
}

func loopDeliveryMetadataLeaks(root, commit string, evidence loopDeliveryEvidence) bool {
	output, err := gitCommand(root, "show", "-s", "--format=%B%x1e%an <%ae>%x1e%cn <%ce>", commit)
	if err != nil {
		return true
	}
	metadata := strings.ToLower(output)
	for _, value := range loopDeliveryMetadataTerms(evidence) {
		if strings.Contains(metadata, value) {
			return true
		}
	}
	return false
}

func loopDeliveryMetadataTerms(evidence loopDeliveryEvidence) []string {
	terms := map[string]bool{"taskrail": true, "agent": true, "delegation": true}
	for _, value := range []string{evidence.SelectedTask, evidence.PlanningDir, evidence.VerifyDir} {
		if value != "" {
			terms[strings.ToLower(value)] = true
		}
	}
	if storageRoot := evidence.Preflight.Storage().Root; storageRoot != "" && storageRoot != "." {
		terms[strings.ToLower(storageRoot)] = true
	}
	for inputPath := range evidence.Preflight.Inputs() {
		terms[strings.ToLower(inputPath)] = true
	}
	for inputPath := range evidence.PostflightInputs {
		terms[strings.ToLower(inputPath)] = true
	}
	values := make([]string, 0, len(terms))
	for term := range terms {
		values = append(values, term)
	}
	sort.Strings(values)
	return values
}

func loopDeliveryPathSet(output string) map[string]bool {
	paths := make(map[string]bool)
	for _, name := range strings.Split(strings.TrimSuffix(output, "\x00"), "\x00") {
		if name == "" {
			continue
		}
		paths[path.Clean(name)] = true
	}
	return paths
}

func loopChangedDeliveryInputs(before, after map[string][]byte) map[string]bool {
	changed := make(map[string]bool)
	for inputPath, data := range before {
		if afterData, ok := after[inputPath]; !ok || !bytes.Equal(data, afterData) {
			changed[inputPath] = true
		}
	}
	for inputPath := range after {
		if _, ok := before[inputPath]; !ok {
			changed[inputPath] = true
		}
	}
	return changed
}

func loopDeliveryHasManagedPath(paths map[string]bool, evidence loopDeliveryEvidence) bool {
	planningDir := evidence.PlanningDir
	if planningDir == "" {
		planningDir = "planning"
	}
	for inputPath := range paths {
		if strings.HasPrefix(inputPath, planningDir+"/") {
			return true
		}
		if _, ok := evidence.Preflight.Inputs()[inputPath]; ok {
			return true
		}
		if _, ok := evidence.PostflightInputs[inputPath]; ok {
			return true
		}
	}
	return false
}

func loopDeliveryVerificationPath(evidence loopDeliveryEvidence, inputPath string) bool {
	verifyDir := evidence.VerifyDir
	if verifyDir == "" {
		planningDir := evidence.PlanningDir
		if planningDir == "" {
			planningDir = "planning"
		}
		verifyDir = planningDir + "/artifacts/verify"
	}
	return strings.HasPrefix(inputPath, verifyDir+"/")
}

func loopDeliveryHasUnexpectedProductPath(paths map[string]bool, evidence loopDeliveryEvidence) bool {
	if evidence.AllowedProductPaths == nil {
		return false
	}
	for inputPath := range paths {
		if !loopDeliveryHasManagedPath(map[string]bool{inputPath: true}, evidence) && !evidence.AllowedProductPaths[inputPath] {
			return true
		}
	}
	return false
}

func loopKnownLifecycleCandidate(candidate string) bool {
	switch candidate {
	case "completed_pass", "blocked_fail", "rework_fail", "completed_unverified", "completed_audit_fail", "no_progress":
		return true
	default:
		return false
	}
}

func loopDeliveryViolation(code, message string) MachineViolation {
	return MachineViolation{Code: code, Message: message}
}

func sortLoopDeliveryViolations(violations []MachineViolation) {
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Code != violations[j].Code {
			return violations[i].Code < violations[j].Code
		}
		return violations[i].Message < violations[j].Message
	})
}
