package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowDocsKeepArtifactsEphemeral(t *testing.T) {
	for _, name := range []string{"development-workflow.md", "human-workflow.md"} {
		path := filepath.Join("..", "..", "docs", "workflow", name)
		body := readWorkflowDoc(t, path)
		for _, phrase := range []string{"ephemeral", "gitignored", "never commit"} {
			if !strings.Contains(body, phrase) {
				t.Errorf("%s must describe manual-test and verify artifacts as %q", path, phrase)
			}
		}
		if strings.Contains(body, "Commit the artifacts") || strings.Contains(body, "Commit only the artifacts") {
			t.Errorf("%s must not tell contributors to commit ephemeral artifacts", path)
		}
	}
}

func TestAutonomousContractNamesCurrentSkillAndBinarySources(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "workflow", "autonomous-contract.md")
	body := readWorkflowDoc(t, path)
	for _, phrase := range []string{"`internal/taskrail/skills/`", "`.agents/skills/`", "`.claude/skills/`", "`${TASKRAIL:-taskrail}`"} {
		if !strings.Contains(body, phrase) {
			t.Errorf("%s missing current workflow source %s", path, phrase)
		}
	}
	if strings.Contains(body, "`skills/` is the canonical") || strings.Contains(body, "`go run ./cmd/taskrail ...` is the intended transition path") {
		t.Errorf("%s retains a retired workflow source or transition path", path)
	}
}

func TestV04ShippedPrerequisitesAreDocumented(t *testing.T) {
	agents := readWorkflowDoc(t, filepath.Join("..", "..", "AGENTS.md"))
	if !strings.Contains(agents, "`specs/v0.4.0.md`") {
		t.Error("AGENTS.md source-of-truth specs must include v0.4.0")
	}

	backlog := readWorkflowDoc(t, filepath.Join("..", "..", "planning", "BACKLOG.md"))
	migration := markdownBullet(backlog, "- **`taskrail spec migrate`")
	for _, phrase := range []string{"shipped `spec diff`", "shipped `task repoint`"} {
		if !strings.Contains(migration, phrase) {
			t.Errorf("spec migrate backlog entry must treat %s as a shipped prerequisite", phrase)
		}
	}
	if strings.Contains(migration, "deferred `spec diff`") {
		t.Error("spec migrate backlog entry must not call shipped spec diff deferred")
	}
}

func readWorkflowDoc(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}
