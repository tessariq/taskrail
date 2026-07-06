---
name: taskrail-import
description: Import markdown notes or draft docs into Taskrail spec and task files, agent-assisted and LLM-free in the binary
---

# taskrail-import

Turn a markdown source (notes, a rough feature doc, a draft spec) into real
Taskrail spec sections and task files. The `taskrail` binary never calls a model;
you, the agent, do the semantic lift between two deterministic binary steps.

Requires the installed `taskrail` binary on `PATH`.

## Flow

1. Emit the prompt for the source and target:
   `taskrail import <source.md> --to <tasks|spec|planning> --emit-prompt`
2. Follow that prompt: read the embedded source and produce a single JSON draft
   that conforms to the schema the prompt describes (`schema_version`, `target`,
   `tasks`, `spec_sections`). Do the real work — split coherent tasks, write
   clear titles, set `spec_ref` to real spec headings, wire `dependencies`.
3. Save your JSON to a file, e.g. `draft.json`.
4. Apply it: `taskrail import --apply draft.json`. The binary validates the draft
   and writes spec/task files, scaffolding each task through the same path as
   `taskrail task new`.
5. Review the created files. Run `taskrail validate`.

## Rules

- never hand-edit `planning/STATE.md` frontmatter
- never hand-edit task status fields
- return only the JSON draft in step 2; no prose, no code fence
- every `spec_ref` must point at a heading that already exists
- keep drafts small and focused; prefer several tasks over one broad task
- the thin `--llm` adapter (binary calling a model directly) is not available; it
  is deferred to a later version by design
