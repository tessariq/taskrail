package taskrail

import (
	"strings"
	"testing"
)

func TestEmitImportPromptIsDeterministic(t *testing.T) {
	t.Parallel()
	svc := importFixture(t)

	first, err := svc.EmitImportPrompt(EmitPromptInput{SourcePath: "notes.md", Target: "tasks"})
	if err != nil {
		t.Fatalf("emit prompt first: %v", err)
	}
	second, err := svc.EmitImportPrompt(EmitPromptInput{SourcePath: "notes.md", Target: "tasks"})
	if err != nil {
		t.Fatalf("emit prompt second: %v", err)
	}
	if first.Prompt != second.Prompt {
		t.Fatal("emit-prompt must be byte-identical for identical input")
	}
}

func TestEmitImportPromptEmbedsSourceSchemaAndConventions(t *testing.T) {
	t.Parallel()
	svc := importFixture(t)

	result, err := svc.EmitImportPrompt(EmitPromptInput{SourcePath: "notes.md", Target: "planning"})
	if err != nil {
		t.Fatalf("emit prompt: %v", err)
	}
	// The source content must be embedded verbatim so the agent does the lift.
	if !strings.Contains(result.Prompt, "Add checkout endpoint") {
		t.Fatalf("prompt must embed the source content, got:\n%s", result.Prompt)
	}
	// The T-032 schema surface must be described.
	for _, want := range []string{"schema_version", "spec_ref", "dependencies", "planning"} {
		if !strings.Contains(result.Prompt, want) {
			t.Fatalf("prompt must mention %q, got:\n%s", want, result.Prompt)
		}
	}
	if result.Target != "planning" || result.Source != "notes.md" {
		t.Fatalf("unexpected result metadata: %+v", result)
	}
}

func TestEmitImportPromptFencesSourceContainingBackticks(t *testing.T) {
	t.Parallel()
	svc := importFixture(t)
	// A source with its own triple-backtick fence must not break the prompt's
	// outer fence: everything after the inner fence must stay inside the block.
	writeSource(t, svc.paths.RepoRoot, "fenced.md", "# Doc\n\n## A task\n\n```\ncode\n```\n\n- trailing bullet\n")

	result, err := svc.EmitImportPrompt(EmitPromptInput{SourcePath: "fenced.md", Target: "tasks"})
	if err != nil {
		t.Fatalf("emit prompt: %v", err)
	}
	// The outer fence must be longer than the longest backtick run in the source.
	if !strings.Contains(result.Prompt, "````\n") {
		t.Fatalf("outer fence must exceed the inner ``` run, got:\n%s", result.Prompt)
	}
	// The bullet after the inner fence must still be embedded as source content.
	if !strings.Contains(result.Prompt, "- trailing bullet") {
		t.Fatalf("source content after an inner fence must stay embedded, got:\n%s", result.Prompt)
	}
}

func TestEmitImportPromptMakesNoLLMCall(t *testing.T) {
	t.Parallel()
	svc := importFixture(t)
	// The binary is provider-agnostic: emit-prompt only reads the source and
	// renders text. It must never modify the source or write files.
	before := snapshotTree(t, svc.paths.RepoRoot)
	if _, err := svc.EmitImportPrompt(EmitPromptInput{SourcePath: "notes.md", Target: "tasks"}); err != nil {
		t.Fatalf("emit prompt: %v", err)
	}
	after := snapshotTree(t, svc.paths.RepoRoot)
	if len(before) != len(after) {
		t.Fatalf("emit-prompt must not create or remove files: before=%d after=%d", len(before), len(after))
	}
}

func TestEmitImportPromptRejectsUnknownTarget(t *testing.T) {
	t.Parallel()
	svc := importFixture(t)
	if _, err := svc.EmitImportPrompt(EmitPromptInput{SourcePath: "notes.md", Target: "everything"}); err == nil {
		t.Fatal("expected error for unknown target")
	}
}

func TestEmitImportPromptRejectsMissingSource(t *testing.T) {
	t.Parallel()
	svc := importFixture(t)
	if _, err := svc.EmitImportPrompt(EmitPromptInput{SourcePath: "does-not-exist.md", Target: "tasks"}); err == nil {
		t.Fatal("expected error for missing source file")
	}
}
