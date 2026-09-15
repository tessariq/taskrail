package taskrail

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// reviewProposalSkills are the three review skills that direct an agent to one
// transient proposal directory before `prompt render` renders its review prompt.
var reviewProposalSkills = []string{
	"taskrail-spec-review",
	"taskrail-task-review",
	"taskrail-workflow-adversarial",
}

// T-401: each review skill must document exactly the proposal-directory
// precondition the renderer enforces. `prompt render` refuses a selected review
// path whose proposal directory does not exist, so the skills must tell the
// agent to create the ignored proposal directory first and must not tell it to
// choose an absent one.
func TestReviewSkillsMatchRendererProposalDirectoryPrecondition(t *testing.T) {
	for _, name := range reviewProposalSkills {
		body := strings.Join(strings.Fields(readShippableSkill(t, name)), " ")
		for _, forbidden := range []string{
			"absent proposal directory",
			"absent effectively ignored",
		} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s instructs choosing an %q; the renderer refuses a missing proposal directory", name, forbidden)
			}
		}
		for _, want := range []string{
			"rendering refuses a missing proposal directory",
			"must already exist",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s must state that %q", name, want)
			}
		}
	}
}

// The documented precondition is the renderer's actual behavior: authorizing a
// review proposal path fails with the exact missing-directory refusal until the
// ignored proposal directory exists, then succeeds. The authorization checked
// here is the same boundary `prompt render` applies to every --review path.
func TestRendererEnforcesDocumentedProposalDirectoryPrecondition(t *testing.T) {
	for _, skill := range []struct {
		name         string
		proposalType string
		file         string
	}{
		{"taskrail-spec-review", "spec", "round-1-consistency.json"},
		{"taskrail-task-review", "task", "review.json"},
		{"taskrail-workflow-adversarial", "workflow-adversarial", "report.json"},
	} {
		t.Run(skill.name, func(t *testing.T) {
			repo := t.TempDir()
			initLocalGitRepo(t, repo)
			writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), "planning/artifacts/\n")
			svc := transientPromptService(repo)
			reviewPath := "planning/artifacts/review-proposals/" + skill.proposalType + "/session-1/" + skill.file
			candidate := []TransientPromptPath{{Role: PromptContextReviewPath, ProposalType: skill.proposalType, Path: reviewPath}}

			_, err := svc.AuthorizeTransientPromptPaths(candidate)
			if err == nil || !strings.Contains(err.Error(), "transient prompt proposal directory") || !strings.Contains(err.Error(), "is missing") {
				t.Fatalf("missing-directory error = %v, want the renderer's missing proposal-directory refusal", err)
			}
			if MachineFailureFor(err).Code != MachineCodePathBlocked {
				t.Fatalf("missing-directory error code = %q, want %q", MachineFailureFor(err).Code, MachineCodePathBlocked)
			}

			if err := os.MkdirAll(filepath.Join(repo, filepath.FromSlash(path.Dir(reviewPath))), 0o755); err != nil {
				t.Fatalf("create proposal directory: %v", err)
			}
			if _, err := svc.AuthorizeTransientPromptPaths(candidate); err != nil {
				t.Fatalf("existing ignored proposal directory must authorize: %v", err)
			}
		})
	}
}
