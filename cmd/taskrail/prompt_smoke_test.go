package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
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

func TestPromptRenderPublishesExactReadOnlyContent(t *testing.T) {
	root := seedTodo(t)
	before := snapshotTree(t, root)

	text, err := runRoot(t, "prompt", "render", "task-implementation", "--task", "T-100")
	if err != nil {
		t.Fatalf("render task implementation: %v (output %q)", err, text)
	}
	if strings.Contains(text, "{{") || !strings.Contains(text, "T-100") || !strings.Contains(text, "maximum is 1") {
		t.Fatalf("rendered text = %q", text)
	}
	if out, err := runRoot(t, "prompt", "render", "task-implementation", "--task", "T-100", "--max-review-rounds", "2"); err != nil || !strings.Contains(out, "maximum is 2") {
		t.Fatalf("render override = %q, error = %v", out, err)
	}
	stdout, stderr, err := runRootSplit(t, "prompt", "render", "task-authoring", "--task", "T-100", "--max-review-rounds", "2", "--json")
	if err == nil {
		t.Fatal("expected task-authoring to reject implementation-only override")
	}
	invalid := decodeEnvelope(t, stdout)
	if invalid.Error == nil || invalid.Error.Code != "prompt_invalid" {
		t.Fatalf("invalid render envelope = %+v, stderr = %q", invalid, stderr)
	}

	stdout, stderr, err = runRootSplit(t, "prompt", "render", "task-implementation", "--task", "T-100", "--json")
	if err != nil {
		t.Fatalf("render JSON: %v (stderr %q)", err, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Command != "prompt render" || envelope.Error != nil {
		t.Fatalf("render envelope = %+v", envelope)
	}
	var result struct {
		ID             string `json:"id"`
		Content        string `json:"content"`
		SHA256         string `json:"sha256"`
		TemplateSHA256 string `json:"template_sha256"`
	}
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatalf("decode render result: %v", err)
	}
	digest := sha256.Sum256([]byte(result.Content))
	if result.ID != "task-implementation" || result.SHA256 != fmt.Sprintf("%x", digest) || result.SHA256 == result.TemplateSHA256 {
		t.Fatalf("render result = %+v", result)
	}
	if after := snapshotTree(t, root); !maps.Equal(before, after) {
		t.Fatal("prompt render changed repository bytes")
	}
}
