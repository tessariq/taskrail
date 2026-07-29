---
name: taskrail-spec
description: Inspect and author Taskrail specs, and anchor tracked work to real spec_ref headings, through the taskrail spec command family
---

# taskrail-spec

Work with Taskrail specs through the `spec` command family: inspect the versioned
specs, discover the `spec_ref` heading anchors that `validate` accepts, advance
the active spec, inspect version-to-version area changes, and scaffold a new one.
Use it before authoring or migrating tracked work so every task points at a
heading that already exists rather than a guessed anchor a later `validate`
would reject.

Requires the installed `taskrail` binary on `PATH`. Run it from the managed
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

1. **Inspect the specs.** Run `${TASKRAIL:-taskrail} spec list` to see the
   versioned specs and which one is active. Read a spec's body with
   `${TASKRAIL:-taskrail} spec show <version>`.
2. **Discover anchors before authoring.** Run
   `${TASKRAIL:-taskrail} spec show <version> --anchors --json` to list the
   spec's `spec_ref` heading anchors exactly as `validate` accepts them. Pick the
   anchor the new task belongs under; never hand-craft a `path#anchor` string.
3. **Inspect a version migration.** Before activation, run
   `${TASKRAIL:-taskrail} spec diff <current-version> <target-version>`. Its
   read-only area-set delta shows added areas to decompose and removed areas
   whose open tasks may need re-pointing; rename candidates are best-effort only.
4. **Advance the active spec (when moving versions).** Run
   `${TASKRAIL:-taskrail} spec activate <version>` to repoint `STATE.md`'s active
   spec. It re-renders `STATE.md` and re-validates; it is the CLI-only writer of
   the active spec and never touches task files or status fields. Check
   `git status` and stage the regenerated `STATE.md`.
5. **Author against an active-spec area.** Create a task through the CLI with
   the discovered anchor: `${TASKRAIL:-taskrail} task new --title "..." --area
   <anchor>`. `--area` resolves against the active spec; use `--spec-ref
   <path#anchor>` only for an intentional cross-spec task.
6. **Re-point migrated open work.** Preview each move with
   `${TASKRAIL:-taskrail} task repoint <id> --area <anchor> --dry-run`, then omit
   `--dry-run` to apply the reviewed move. Re-pointing changes only `spec_ref` and
   rejects completed or cancelled history.
7. **Scaffold a new spec (when starting one).** Run
   `${TASKRAIL:-taskrail} spec add <version>` to create `specs/<version>.md` with
   the standard section skeleton and add it to the `specs/README.md` reading
   order. `spec add` does not activate the new spec; run `spec activate
   <version>` separately once you are ready to work against it.
8. **Validate.** Run `${TASKRAIL:-taskrail} validate` and confirm the state is
   valid.

## Rules

- discover anchors with `spec show <version> --anchors --json` before
  `task new` or `task repoint`; never hand-craft a `spec_ref` string
- create tasks through `${TASKRAIL:-taskrail} task new`, never by hand-authoring
  task markdown
- never hand-edit `planning/STATE.md` frontmatter or task status fields
- `spec list`, `spec show`, and `spec diff` are read-only; after any writer check
  `git status` and stage the files the CLI rewrote
- `spec add` scaffolds but does not activate; activation is a separate,
  deliberate `spec activate` step
