package taskrail

import (
	"fmt"
	"os"
	"path/filepath"
)

func DiscoverPaths(start string) (Paths, error) {
	root, err := findRepoRoot(start)
	if err != nil {
		return Paths{}, err
	}

	planningDir := filepath.Join(root, "planning")
	artifactsDir := filepath.Join(planningDir, "artifacts")

	return Paths{
		RepoRoot:     root,
		SpecsDir:     filepath.Join(root, "specs"),
		PlanningDir:  planningDir,
		TasksDir:     filepath.Join(planningDir, "tasks"),
		ArtifactsDir: artifactsDir,
		VerifyDir:    filepath.Join(artifactsDir, "verify"),
		StateFile:    filepath.Join(planningDir, "STATE.md"),
	}, nil
}

func findRepoRoot(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	current := abs
	for {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("repository root not found from %s", start)
		}
		current = parent
	}
}
