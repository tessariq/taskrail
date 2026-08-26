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
   `--emit-prompt` is an exact-text exception because its prompt bytes are the
   workflow input, not a structured result.
2. Follow that prompt and produce one compatibility ImportDraft v1 JSON object.
   Do the semantic work: give each task one independently meaningful outcome,
   split independently useful results, merge file/layer/phase fragments that only
   establish one result, name the integrated-behavior owner, use real headings,
   and wire real dependencies and operator gates. The optional legacy `body`
   member is accepted but ignored by v1 apply.
3. Save your JSON to a file, e.g. `draft.json`.
4. Apply it: `${TASKRAIL:-taskrail} import --apply draft.json --json`. The binary validates the draft
   and writes spec/task files. Each task receives the standard outcome-focused
   Description, Acceptance, Verification Notes, and Implementation Notes
   scaffold. The scaffold is structural and does not certify semantic sizing.
5. Review the created files. Run `${TASKRAIL:-taskrail} validate --json`.

Apply publishes the validated draft as one repository-locked transaction. A
source or destination race refuses with `write_conflict`; handled publication
failures roll back unchanged candidates and preserve external edits with common
transaction evidence. Retry only after resolving the reported conflict or
rollback evidence.

## Rules

- never hand-edit `planning/STATE.md` frontmatter
- treat `planning/STATE.md` as current state, never as a task/session log; put
  durable context in task implementation notes or follow-up tasks
- never hand-edit task status fields
- return only the JSON draft in step 2; no prose, no code fence
- every `spec_ref` must point at a heading that already exists
- omit `loop_policy` and `loop_reason`; imported tasks remain implicitly held
- split independently useful outcomes, but never split one result by file, layer,
  discipline, phase, or estimate
- require durable evidence and one owner for required integrated behavior
- the thin `--llm` adapter (binary calling a model directly) is not available; it
  is deferred to a later version by design
