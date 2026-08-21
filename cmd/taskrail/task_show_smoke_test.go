package main

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/taskrail"
)

func TestTaskShowPublishesExactReadOnlyTask(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-001", "todo", "")
	before := snapshotTree(t, root)
	want, err := os.ReadFile(filepath.Join(root, "planning", "tasks", "T-001.md"))
	if err != nil {
		t.Fatalf("read task fixture: %v", err)
	}

	text, stderr, err := runRootSplit(t, "task", "show", "T-001")
	if err != nil {
		t.Fatalf("task show text: %v (stderr %q)", err, stderr)
	}
	if text != string(want) {
		t.Fatalf("task show text = %q, want exact bytes %q", text, want)
	}

	stdout, stderr, err := runRootSplit(t, "task", "show", "T-001", "--json")
	if err != nil {
		t.Fatalf("task show: %v (stderr %q)", err, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Command != "task show" || envelope.Error != nil {
		t.Fatalf("task show envelope = %+v", envelope)
	}
	var result struct {
		TaskID   string `json:"task_id"`
		TaskPath string `json:"task_path"`
		Content  string `json:"content"`
		SHA256   string `json:"sha256"`
	}
	decodeMachineResult(t, stdout, &result)
	if result.TaskID != "T-001" || result.TaskPath != "planning/tasks/T-001.md" {
		t.Fatalf("task show identity = %+v", result)
	}
	if result.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(result.Content))) {
		t.Fatalf("task show digest does not match content: %+v", result)
	}
	if got := snapshotTree(t, root); !maps.Equal(got, before) {
		t.Fatal("task show changed repository bytes")
	}
}

func TestTaskShowRejectsNonExactIDWithMachineError(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-001", "todo", "")
	stdout, _, err := runRootSplit(t, "task", "show", "T-001-", "--json")
	if err == nil {
		t.Fatal("task show accepted a fuzzy task ID")
	}
	if failure := decodeMachineError(t, stdout); failure.Code != taskrail.MachineCodeTaskNotFound {
		t.Fatalf("task show error = %+v, want task_not_found", failure)
	}
	if strings.Contains(stdout, ".taskrail/local/") {
		t.Fatalf("task show error leaked local storage path: %q", stdout)
	}
}
