---
name: taskrail-spec
description: Inspect and author Taskrail specs, and anchor tracked work to real spec_ref headings, through the taskrail spec command family
---

# taskrail-spec

Work with Taskrail specs through the `spec` command family: inspect the versioned
specs, discover the `spec_ref` heading anchors that `validate` accepts, advance
the active spec, and scaffold a new one. Use it before authoring tracked work so
every `task new --spec-ref` points at a heading that already exists rather than a
guessed anchor a later `validate` would reject.

Requires the installed `taskrail` binary on `PATH`. Run it from the managed
repository's root. `spec list` and `spec show` are read-only; `spec activate` and
`spec add` write `STATE.md` and `specs/`, so check `git status` after them.

## Flow

1. **Inspect the specs.** Run `${TASKRAIL:-taskrail} spec list` to see the
   versioned specs and which one is active. Read a spec's body with
   `${TASKRAIL:-taskrail} spec show <version>`.
2. **Discover anchors before authoring.** Run
   `${TASKRAIL:-taskrail} spec show <version> --anchors --json` to list the
   spec's `spec_ref` heading anchors exactly as `validate` accepts them. Pick the
   anchor the new task belongs under; never hand-craft a `path#anchor` string.
3. **Author against a real anchor.** Create the task through the CLI with the
   discovered anchor: `${TASKRAIL:-taskrail} task new --title "..." --spec-ref
   <path#anchor>`. Because the anchor came from step 2, the follow-up
   `${TASKRAIL:-taskrail} validate` passes rather than rejecting an unknown
   `spec_ref`.
4. **Advance the active spec (when moving versions).** Run
   `${TASKRAIL:-taskrail} spec activate <version>` to repoint `STATE.md`'s active
   spec. It re-renders `STATE.md` and re-validates; it is the CLI-only writer of
   the active spec and never touches task files or status fields. Check
   `git status` and stage the regenerated `STATE.md`.
5. **Scaffold a new spec (when starting one).** Run
   `${TASKRAIL:-taskrail} spec add <version>` to create `specs/<version>.md` with
   the standard section skeleton and add it to the `specs/README.md` reading
   order. `spec add` does not activate the new spec; run `spec activate
   <version>` separately once you are ready to work against it.
6. **Validate.** Run `${TASKRAIL:-taskrail} validate` and confirm the state is
   valid.

## Rules

- discover anchors with `spec show <version> --anchors --json` before
  `task new`; never hand-craft a `spec_ref` string
- create tasks through `${TASKRAIL:-taskrail} task new`, never by hand-authoring
  task markdown
- never hand-edit `planning/STATE.md` frontmatter or task status fields
- `spec list` and `spec show` are read-only; after `spec activate` or `spec add`
  check `git status` and stage the files the CLI rewrote
- `spec add` scaffolds but does not activate; activation is a separate,
  deliberate `spec activate` step
