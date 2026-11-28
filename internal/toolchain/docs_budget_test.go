package toolchain_test

import (
	"io/fs"
	"path/filepath"
	"testing"
)

// Agent guidance loads into every agent session, so its size is a recurring cost
// and a long file lowers adherence. Codex also stops reading after 32 KiB of
// combined instructions, counting the user's own global file, so the repository's
// share must leave headroom. Context files grow through small additions that no
// single review rejects; this budget is the check that does.
const (
	rootAgentsBudget  = 12 * 1024
	totalAgentsBudget = 16 * 1024
)

func TestAgentsGuidanceStaysWithinBudget(t *testing.T) {
	root := repoRoot(t)
	rootSize := len(readFile(t, root, "AGENTS.md"))
	if rootSize > rootAgentsBudget {
		t.Errorf("AGENTS.md is %d bytes, over the %d-byte budget: move procedures to docs/ and point to them", rootSize, rootAgentsBudget)
	}

	total := 0
	for _, rel := range agentsFiles(t, root) {
		total += len(readFile(t, root, rel))
	}
	if total > totalAgentsBudget {
		t.Errorf("AGENTS.md files total %d bytes, over the %d-byte budget", total, totalAgentsBudget)
	}
}

// agentsFiles lists every committed-tree AGENTS.md, skipping directories that
// hold no repository guidance (VCS data and gitignored build or evidence output).
func agentsFiles(t *testing.T, root string) []string {
	t.Helper()
	skip := map[string]bool{".git": true, ".amp": true, "bin": true, "artifacts": true, "node_modules": true}
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && skip[d.Name()] {
			return filepath.SkipDir
		}
		if !d.IsDir() && d.Name() == "AGENTS.md" {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return files
}
