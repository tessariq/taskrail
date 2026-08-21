package taskrail

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validateVerificationEvidence keeps predecessor validation independent from
// wall-clock ordering. A task's persisted latest tuple must match its report,
// and each predecessor named by that report must be a real prior report for the
// same task.
func (s *Service) validateVerificationEvidence(state *State, tasks []*Task) []string {
	violations := make([]string, 0)
	byID := make(map[string]*Task, len(tasks))
	for _, task := range tasks {
		byID[task.Frontmatter.ID] = task
		if task.Frontmatter.LastVerificationID == "" {
			continue
		}
		reports, available, err := s.verificationReports(task.Frontmatter.ID)
		if err != nil {
			violations = append(violations, fmt.Sprintf("task %s verification evidence: %v", task.Frontmatter.ID, err))
			continue
		}
		if !available {
			continue // Verification artifacts are producer-local and absent in clones.
		}
		if err := validateTaskVerificationReports(task, reports); err != nil {
			violations = append(violations, fmt.Sprintf("task %s verification evidence: %v", task.Frontmatter.ID, err))
		}
	}

	if verificationID := state.Frontmatter.LastVerificationID; verificationID != "" {
		match := canonicalStateVerification.FindStringSubmatch(state.Frontmatter.LastVerificationResult)
		if match != nil {
			task := byID[match[2]]
			if task == nil || task.Frontmatter.LastVerificationID != verificationID || task.Frontmatter.LastVerificationPreviousID != state.Frontmatter.LastVerificationPreviousID {
				violations = append(violations, "state verification tuple must match the task latest verification tuple")
			}
		}
	}
	return violations
}

func (s *Service) verificationReports(taskID string) (map[string]VerificationArtifact, bool, error) {
	dir := filepath.Join(s.paths.VerifyDir, taskID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read verification report directory: %w", fsCause(err))
	}
	reports := make(map[string]VerificationArtifact)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name(), "report.json"))
		if err != nil {
			return nil, true, fmt.Errorf("read verification report %s: %w", entry.Name(), fsCause(err))
		}
		var report VerificationArtifact
		if err := json.Unmarshal(data, &report); err != nil {
			return nil, true, fmt.Errorf("parse verification report %s: %w", entry.Name(), err)
		}
		if report.VerificationID == "" {
			continue // legacy reports do not participate in identity chains.
		}
		if reports[report.VerificationID].VerificationID != "" {
			return nil, true, fmt.Errorf("repeated verification report id %s", report.VerificationID)
		}
		if filepath.Base(entry.Name()) == "" || filepath.Ext(entry.Name()) != "" || len(entry.Name()) < len(report.VerificationID)+1 || entry.Name()[len(entry.Name())-len(report.VerificationID)-1:] != "-"+report.VerificationID {
			return nil, true, fmt.Errorf("report %s is not stored in its identity-named artifact directory", report.VerificationID)
		}
		reports[report.VerificationID] = report
	}
	return reports, true, nil
}

func validateTaskVerificationReports(task *Task, reports map[string]VerificationArtifact) error {
	meta := task.Frontmatter.CompletionVerificationMetadata
	report, ok := reports[meta.LastVerificationID]
	if !ok {
		return fmt.Errorf("missing latest verification report %s", meta.LastVerificationID)
	}
	if report.TaskID != task.Frontmatter.ID || report.Result != meta.LastVerificationResult || report.GeneratedAt != meta.LastVerifiedAt {
		return fmt.Errorf("latest verification report does not match the task tuple")
	}
	if note := verificationNoteLine(meta.LastVerifiedAt, meta.LastVerificationResult, meta.LastVerificationID, meta.LastVerificationPreviousID); !containsLine(task.Body, note) {
		return fmt.Errorf("latest verification task note does not match the task tuple")
	}
	previous := meta.LastVerificationPreviousID
	if (report.PreviousVerificationID == nil) != (previous == "") || report.PreviousVerificationID != nil && *report.PreviousVerificationID != previous {
		return fmt.Errorf("latest verification report predecessor does not match the task tuple")
	}
	seen := map[string]bool{report.VerificationID: true}
	for previous != "" {
		prior, ok := reports[previous]
		if !ok || prior.TaskID != task.Frontmatter.ID {
			return fmt.Errorf("predecessor %s does not identify a prior task-level verification report", previous)
		}
		if seen[prior.VerificationID] {
			return fmt.Errorf("verification predecessor chain repeats id %s", prior.VerificationID)
		}
		seen[prior.VerificationID] = true
		if prior.PreviousVerificationID == nil {
			break
		}
		previous = *prior.PreviousVerificationID
	}
	return nil
}

func containsLine(body, want string) bool {
	for _, line := range strings.Split(body, "\n") {
		if line == want {
			return true
		}
	}
	return false
}
