---
name: taskrail-import
description: Import markdown notes or draft docs into Taskrail spec and task files, agent-assisted and LLM-free in the binary
---

# taskrail-import

Turn a markdown source (notes, a rough feature doc, a draft spec) into real
Taskrail spec sections and task files. The `taskrail` binary never calls a model;
you, the agent, do the semantic lift between two deterministic binary steps.

Requires the installed `taskrail` binary on `PATH`.

## Source Checkout Guard

Before every command that can write tracked state, check whether the repository
is the Taskrail source checkout (it contains both `Taskfile.yml` and
`internal/toolchain/cmd/freshcheck`). If so, run `task taskrail:check`
immediately before the writer. This checks the exact `${TASKRAIL:-taskrail}`
binary the workflow will invoke. If it fails, stop, apply the remedy it names,
and rerun the guard; do not run the writer first. Installed adopter repositories
do not contain the source helper and skip this source-only guard.

## Flow

1. Emit the prompt for the source and target:
   `${TASKRAIL:-taskrail} import <source.md> --to <tasks|spec|planning> --emit-prompt`
2. Follow that prompt: read the embedded source and produce a single JSON draft
   that conforms to the schema the prompt describes (`schema_version`, `target`,
   `tasks`, `spec_sections`). Do the real work — split coherent tasks, write
   clear titles, set `spec_ref` to real spec headings, wire `dependencies`.
3. Save your JSON to a file, e.g. `draft.json`.
4. Apply it: `${TASKRAIL:-taskrail} import --apply draft.json`. The binary validates the draft
   and writes spec/task files, scaffolding each task through the same path as
   `${TASKRAIL:-taskrail} task new`.
5. Review the created files. Run `${TASKRAIL:-taskrail} validate`.

If apply fails during writing (`partial apply already wrote ...` or `partial
apply may have written ...`, non-zero exit), it still reports what landed or may
have been touched: the spec and task paths in text mode, the same envelope marked
`"partial": true` with `--json`. Review those paths first — a failed spec write
may leave an empty or truncated file, and re-applying the same draft creates any
already-written tasks a second time under new ids.

## Rules

- never hand-edit `planning/STATE.md` frontmatter
- treat `planning/STATE.md` as current state, never as a task/session log; put
  durable context in task implementation notes or follow-up tasks
- never hand-edit task status fields
- return only the JSON draft in step 2; no prose, no code fence
- every `spec_ref` must point at a heading that already exists
- keep drafts small and focused; prefer several tasks over one broad task
- the thin `--llm` adapter (binary calling a model directly) is not available; it
  is deferred to a later version by design
