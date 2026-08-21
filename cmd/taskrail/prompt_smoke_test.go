package main

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptListAndShowPublishExactReadOnlyResults(t *testing.T) {
	root := setupRepo(t)
	before := snapshotTree(t, root)

	stdout, _, err := runRootSplit(t, "prompt", "list", "--json")
	if err != nil {
		t.Fatalf("prompt list: %v", err)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Command != "prompt list" || envelope.Error != nil {
		t.Fatalf("list envelope = %+v", envelope)
	}
	var list struct {
		Prompts []struct {
			ID              string  `json:"id"`
			ContractVersion string  `json:"contract_version"`
			Source          string  `json:"source"`
			ReplacementPath *string `json:"replacement_path"`
		} `json:"prompts"`
	}
	if err := json.Unmarshal(envelope.Result, &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Prompts) != 10 || list.Prompts[2].ID != "task-review" || list.Prompts[0].ReplacementPath != nil {
		t.Fatalf("list = %+v, want ordered v1 catalog", list)
	}

	text, err := runRoot(t, "prompt", "show", "task-implementation")
	if err != nil {
		t.Fatalf("prompt show text: %v", err)
	}
	if !strings.Contains(text, "{{TASK_ID}}") {
		t.Fatalf("show output did not contain declared token: %q", text)
	}
	if after := snapshotTree(t, root); !maps.Equal(before, after) {
		t.Fatal("prompt list/show changed repository bytes")
	}
}

func TestPromptShowTextPreservesExactReplacementBytes(t *testing.T) {
	root := setupRepo(t)
	path := filepath.Join(root, ".taskrail", "prompts", "v1", "task-review.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create replacement directory: %v", err)
	}
	const replacement = "Review exactly these bytes."
	if err := os.WriteFile(path, []byte(replacement), 0o644); err != nil {
		t.Fatalf("write replacement: %v", err)
	}

	stdout, stderr, err := runRootSplit(t, "prompt", "show", "task-review")
	if err != nil {
		t.Fatalf("prompt show: %v (stderr %q)", err, stderr)
	}
	if stdout != replacement {
		t.Fatalf("prompt show bytes = %q, want %q", stdout, replacement)
	}
}
