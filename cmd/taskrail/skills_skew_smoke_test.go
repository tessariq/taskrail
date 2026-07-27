package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stageSkillSkew installs the embedded skills into a fresh repo and rewrites one
// copy's marker so the running binary sees a version it did not write.
func stageSkillSkew(t *testing.T, root string) {
	t.Helper()
	if out, err := runRoot(t, "init", "--with-skills"); err != nil {
		t.Fatalf("init --with-skills: %v (output %q)", err, out)
	}
	path := filepath.Join(root, ".claude", "skills", "taskrail-repair", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	staged := strings.Replace(string(data), "taskrail_version: \""+version+"\"", "taskrail_version: \"v0.0.1-stale\"", 1)
	if staged == string(data) {
		t.Fatalf("installed skill carries no %q marker to restage:\n%s", version, staged)
	}
	if err := os.WriteFile(path, []byte(staged), 0o644); err != nil {
		t.Fatalf("write staged skill: %v", err)
	}
}

// The skew warning is advisory: it lands on stderr, names both versions and the
// resolving command, and leaves --json stdout parseable by an agent.
func TestSkillSkewWarningGoesToStderrAndKeepsJSONParseable(t *testing.T) {
	root := setupRepo(t)
	stageSkillSkew(t, root)

	stdout, stderr, err := runRootSplit(t, "status", "--json")
	if err != nil {
		t.Fatalf("status --json: %v (stderr %q)", err, stderr)
	}
	var parsed map[string]any
	if jsonErr := json.Unmarshal([]byte(stdout), &parsed); jsonErr != nil {
		t.Fatalf("stdout is not parseable json: %v\n%s", jsonErr, stdout)
	}
	for _, want := range []string{"v0.0.1-stale", version, "taskrail-repair", "taskrail init --with-skills --force"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr %q missing %q", stderr, want)
		}
	}
}

// A stale skill is a missed improvement, not invalid state: validate still
// succeeds and its own stdout is untouched.
func TestSkillSkewWarningDoesNotFailValidate(t *testing.T) {
	root := setupRepo(t)
	stageSkillSkew(t, root)

	stdout, stderr, err := runRootSplit(t, "validate")
	if err != nil {
		t.Fatalf("validate with skewed skills: %v (stderr %q)", err, stderr)
	}
	if !strings.Contains(stdout, "state valid") {
		t.Errorf("validate stdout = %q, want it to report valid state", stdout)
	}
	if !strings.Contains(stderr, "v0.0.1-stale") {
		t.Errorf("stderr %q omits the skew warning", stderr)
	}
}

// A transition is never blocked by skew, and the warning still reaches stderr.
func TestSkillSkewWarningDoesNotBlockTransition(t *testing.T) {
	root := setupRepo(t)
	stageSkillSkew(t, root)
	writeTask(t, root, "T-900", "todo", "")

	stdout, stderr, err := runRootSplit(t, "start", "T-900")
	if err != nil {
		t.Fatalf("start with skewed skills: %v (stderr %q)", err, stderr)
	}
	if !strings.Contains(stdout, "T-900") {
		t.Errorf("start stdout = %q, want the transitioned task id", stdout)
	}
	if !strings.Contains(stderr, "taskrail init --with-skills --force") {
		t.Errorf("stderr %q omits the resolving command", stderr)
	}
}

// Skills the running binary wrote are silent, so the check does not become noise
// in the common case.
func TestNoSkillSkewWarningWhenVersionsMatch(t *testing.T) {
	setupRepo(t)
	if out, err := runRoot(t, "init", "--with-skills"); err != nil {
		t.Fatalf("init --with-skills: %v (output %q)", err, out)
	}

	_, stderr, err := runRootSplit(t, "status")
	if err != nil {
		t.Fatalf("status: %v (stderr %q)", err, stderr)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Errorf("matching skills produced stderr output: %q", stderr)
	}
}

// A repository with no materialized skills is silent.
func TestNoSkillSkewWarningWithoutInstalledSkills(t *testing.T) {
	setupRepo(t)

	_, stderr, err := runRootSplit(t, "status")
	if err != nil {
		t.Fatalf("status: %v (stderr %q)", err, stderr)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Errorf("repo without skills produced stderr output: %q", stderr)
	}
}
