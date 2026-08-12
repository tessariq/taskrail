---
name: taskrail-retrofit
description: Retrofit an existing repository into a Taskrail-managed layout, agent-assisted and LLM-free in the binary
---

# taskrail-retrofit

Take an existing repository plus optional human notes and bring it under
Taskrail: detect the current layout, scaffold `specs/`, `planning/tasks/`, and an
initial `STATE.md`, then adopt the reviewed notes as real tracked work. The
`taskrail` binary never calls a model; you, the agent, do the semantic lift
between deterministic binary steps.

Requires the installed `taskrail` binary on `PATH`. Run it from the target
repository's root.

## Source Checkout Guard

Before every command that can write tracked state, check whether the repository
is the Taskrail source checkout (it contains both `Taskfile.yml` and
`internal/toolchain/cmd/freshcheck`). If so, run `task taskrail:check`
immediately before the writer. This checks the exact `${TASKRAIL:-taskrail}`
binary the workflow will invoke. If it fails, stop, apply the remedy it names,
and rerun the guard; do not run the writer first. Installed adopter repositories
do not contain the source helper and skip this source-only guard.

## Flow

1. **Detect + dry-run.** Run `${TASKRAIL:-taskrail} retrofit <notes.md> --json` (or
   `${TASKRAIL:-taskrail} retrofit --json` with no notes). This defaults to a dry run: it detects the existing
   layout, proposes a mapping, and prints the scaffold diff. Nothing is written
   and the notes file is only read.
2. **Confirm.** Review the proposed mapping and diff. Retrofit never overwrites
   existing files; confirm the scaffold is what the adopter wants before
   applying.
3. **Apply.** Run `${TASKRAIL:-taskrail} retrofit <notes.md> --apply --json` to write the scaffold
   and layout marker and re-run validation. The imported bootstrap is a proposal
   to review, not tracked work retrofit creates.
4. **Adopt (persist reviewed tasks).** Emit the import prompt for the notes:
    `${TASKRAIL:-taskrail} retrofit <notes.md> --emit-prompt`.
    `--emit-prompt` is an exact-text exception because its prompt bytes are the workflow input. Follow
    that prompt and produce a single JSON draft conforming to the schema it describes (`schema_version`,
   `target`, `tasks`, `spec_sections`). Do the real work — split coherent tasks,
   write clear titles, set `spec_ref` to real spec headings, wire
   `dependencies`. Save it to `draft.json`.
5. **Land the tasks.** Run `${TASKRAIL:-taskrail} import --apply draft.json --json`. The binary
   validates the draft and writes real spec/task files through the same path as
   `${TASKRAIL:-taskrail} task new`.
6. **Validate.** Review the created files and run `${TASKRAIL:-taskrail} validate --json`.

## Rules

- never hand-edit `planning/STATE.md` frontmatter
- treat `planning/STATE.md` as current state, never as a task/session log; put
  durable context in task implementation notes or follow-up tasks
- never hand-edit task status fields
- dry-run and confirm before `--apply`; retrofit never clobbers existing files
- return only the JSON draft in step 4; no prose, no code fence
- every `spec_ref` must point at a heading that already exists after step 3
- keep drafts small and focused; prefer several tasks over one broad task
- the thin `--llm` adapter (binary calling a model directly) is not available; it
  is deferred to a later version by design
