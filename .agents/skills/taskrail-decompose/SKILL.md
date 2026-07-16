---
name: taskrail-decompose
description: Draft spec-anchored Taskrail tasks for uncovered active-spec areas, agent-assisted and LLM-free in the binary
---

# taskrail-decompose

Turn uncovered areas of the active spec into reviewable draft tasks, each anchored
to a real `spec_ref` heading. The `taskrail` binary never calls a model; you, the
agent, do the semantic lift between deterministic binary steps. This composes
shipped primitives — `coverage --json`, `spec show <version> --anchors`, and
`import --apply` — and adds no new binary surface.

Requires the installed `taskrail` binary on `PATH`. Run it from the managed
repository's root.

## When to use this vs the sibling skills

- **taskrail-decompose** (this skill) is **spec-driven**: it starts from the active
  spec's own headings and fills coverage gaps against anchors that already exist.
- **taskrail-import** is **arbitrary-source-driven**: it turns external notes or a
  draft doc into tasks and may propose new spec sections.
- **taskrail-retrofit** is **whole-repo bootstrap**: it brings an unmanaged
  repository under Taskrail before any tracked work exists.
- **taskrail-spec** inspects and authors specs and anchors single tasks; reach for
  it to add or discover the headings this skill then decomposes against.

## Flow

1. **Find uncovered areas.** Run `${TASKRAIL:-taskrail} coverage --json`. Each entry
   in `areas` with `"covered": false` is a coverable active-spec area with no linked
   task — those are the decomposition targets. `coverage` is read-only and writes
   nothing.
2. **Confirm the live anchors.** Run
   `${TASKRAIL:-taskrail} spec show <version> --anchors --json` for the active spec
   (derive `<version>` from step 1's `active_spec_path`, e.g. `specs/v0.3.0.md` →
   `v0.3.0`). Match each uncovered area to its real heading anchor; never hand-craft
   a `path#anchor` string.
3. **Author a draft.** Produce a single JSON `ImportDraft` (`schema_version` 1,
   `target` `"tasks"`, `tasks`). Do the real work — split each uncovered area into
   coherent tasks, write clear imperative titles, wire `dependencies`, and set every
   `spec_ref` to an anchor discovered in step 2. Save it to `draft.json`. To scaffold
   the exact schema, emit the prompt over the active spec file itself (the
   decomposition source, always present):
   `${TASKRAIL:-taskrail} import <active-spec-path> --to tasks --emit-prompt`.
4. **Apply (single writer).** Run `${TASKRAIL:-taskrail} import --apply draft.json`.
   The binary validates the draft and writes real task files through the same path
   as `${TASKRAIL:-taskrail} task new`, rejecting any `spec_ref` whose anchor does
   not exist. Steps 1–3 write no committed state; this reviewed, human-invoked apply
   is the only writer.
5. **Validate.** Review the created task files and run
   `${TASKRAIL:-taskrail} validate`. Confirm the state is valid and re-run
   `${TASKRAIL:-taskrail} coverage --json` to see the gaps now closed.

## Rules

- never hand-edit `planning/STATE.md` frontmatter or task status fields
- create tasks with `import --apply`, never by hand-authoring task markdown
- every `spec_ref` must point at an anchor from `spec show --anchors`; apply verifies it
- decompose only uncovered areas (`"covered": false`); do not duplicate covered ones
- discovery and authoring stay draft-only; `import --apply` is the single reviewed writer
- return only the JSON draft in step 3; no prose, no code fence
- keep drafts small and focused; prefer several tasks over one broad task
- the thin `--llm` adapter (binary calling a model directly) is not available; it
  is deferred to a later version by design
