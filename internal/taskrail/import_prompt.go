package taskrail

import (
	"fmt"
	"strings"
)

// Agent-driven import (T-034): `taskrail import <src> --emit-prompt` renders a
// deterministic, ready-to-paste prompt that hands the semantic lift to a coding
// agent while the binary stays LLM-free. The agent returns an ImportDraft, which
// `taskrail import --apply <draft.json>` validates and writes. The thin `--llm`
// adapter (the binary calling a model directly) is deferred to v0.3 per the spec
// ordering and is intentionally not implemented here.

// EmitPromptInput selects the source and the draft target the prompt asks for.
type EmitPromptInput struct {
	SourcePath string
	Target     string
}

// EmitPromptResult carries the rendered prompt plus the portable source label and
// target it was built for.
type EmitPromptResult struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Prompt string `json:"prompt"`
}

// EmitImportPrompt renders the import prompt for a source. It only reads the
// source and formats text; it makes no model call and writes nothing.
func (s *Service) EmitImportPrompt(input EmitPromptInput) (EmitPromptResult, error) {
	target := strings.TrimSpace(input.Target)
	if _, ok := validImportTargets[target]; !ok {
		return EmitPromptResult{}, fmt.Errorf("import target must be one of tasks, spec, planning; got %q", target)
	}
	markdown, label, err := s.readImportSource(input.SourcePath)
	if err != nil {
		return EmitPromptResult{}, err
	}
	return EmitPromptResult{Source: label, Target: target, Prompt: renderImportPrompt(label, target, markdown)}, nil
}

// renderImportPrompt builds the deterministic prompt: instructions, the T-032
// draft schema, Taskrail conventions, and the verbatim source. It uses no clock
// or randomness, so identical inputs yield byte-identical output.
func renderImportPrompt(source, target, markdown string) string {
	var b strings.Builder
	b.WriteString("# Taskrail import prompt\n\n")
	fmt.Fprintf(&b, "Convert the source below into a Taskrail import draft for target %q.\n", target)
	b.WriteString("Do the semantic lift yourself: split the material into coherent tasks and/or\n")
	b.WriteString("spec sections. Return ONLY a single JSON object that conforms to the schema\n")
	b.WriteString("below. Do not wrap it in prose or a code fence.\n\n")

	b.WriteString("## Draft schema\n\n")
	fmt.Fprintf(&b, "- `schema_version` (integer, required): must be %d.\n", importDraftSchemaVersion)
	fmt.Fprintf(&b, "- `target` (string, required): must be %q.\n", target)
	fmt.Fprintf(&b, "- `source` (string, optional): set to %q.\n", source)
	b.WriteString("- `tasks` (array, optional): each task has\n")
	b.WriteString("  - `key` (string): a unique kebab-case handle for in-draft dependencies.\n")
	b.WriteString("  - `title` (string, required): imperative, one outcome.\n")
	b.WriteString("  - `spec_ref` (string): `specs/<file>.md#<heading-anchor>` pointing at a real heading.\n")
	b.WriteString("  - `priority` (string): one of high, medium, low.\n")
	b.WriteString("  - `dependencies` (array of strings): each an in-draft `key` or an existing task id like `T-012`.\n")
	b.WriteString("  - `body` (string): markdown detail for the task file.\n")
	b.WriteString("- `spec_sections` (array, optional): each has `heading` (required) and `body`.\n\n")

	b.WriteString("## Conventions\n\n")
	b.WriteString("- Provide at least one task or one spec section; empty drafts are rejected.\n")
	b.WriteString("- Keep every `key` unique; a task must not depend on itself.\n")
	b.WriteString("- Prefer small, focused tasks with a clear single outcome.\n")
	b.WriteString("- Only reference a `spec_ref` heading that exists; apply verifies it.\n\n")

	b.WriteString("## Apply\n\n")
	b.WriteString("Save your JSON to a file and run `taskrail import --apply <draft.json>`.\n\n")

	fmt.Fprintf(&b, "## Source: %s\n\n", source)
	// Size the fence to exceed any backtick run in the source so an embedded
	// code fence can never close the block early (CommonMark fencing rule).
	fence := codeFence(markdown)
	b.WriteString(fence + "\n")
	b.WriteString(markdown)
	if !strings.HasSuffix(markdown, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(fence + "\n")
	return b.String()
}

// codeFence returns a run of backticks at least three long and always longer
// than the longest backtick run in content, so content fenced with it cannot
// terminate the fence prematurely.
func codeFence(content string) string {
	longest, run := 0, 0
	for _, r := range content {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
			continue
		}
		run = 0
	}
	n := 3
	if longest+1 > n {
		n = longest + 1
	}
	return strings.Repeat("`", n)
}
