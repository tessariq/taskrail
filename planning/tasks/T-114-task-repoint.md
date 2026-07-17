---
id: T-114-task-repoint
title: 'task repoint: re-point an open task spec_ref onto the active spec'
status: todo
priority: medium
spec_ref: specs/v0.4.0.md#task-spec-ref-re-pointing
dependencies: []
updated_at: "2026-07-17T12:04:24Z"
---

# T-114-task-repoint task repoint: re-point an open task spec_ref onto the active spec

## Description

After `spec activate`, `next` skips off-spec open tasks (T-108), `status` lists them
(T-106), and `next --include-off-spec` (T-110) recovers one to run — but moving an
open task *onto* the active spec still means hand-editing its `spec_ref` frontmatter,
exactly the error-prone manual edit v0.4.0 removes elsewhere (slugged ids, atomic
rename, `--area` shorthand). Add `taskrail task repoint <id>` per the v0.4.0 Task
Spec-Ref Re-pointing amendment: the small sibling of `task new --area` that rewrites
one reference field and closes the drift-recovery loop.

It re-encodes one field only — never the id, slug, filename, title, status, or
dependencies, and never other task files. Not a status mutator, not a bulk migrator.

## Acceptance

- `taskrail task repoint <id> --area <anchor>` resolves `spec_ref` to
  `<active_spec_path>#<anchor>` from `STATE.md`, matched against the active spec
  exactly as `task new --area` and `validate` compute anchors, then rewrites only the
  `spec_ref` field, re-projects `planning/STATE.md`, and re-runs `validate`.
- `--spec-ref <path#anchor>` sets an explicit reference for the cross-spec case;
  `--area` and `--spec-ref` are mutually exclusive.
- An unknown anchor fails before any write and points the operator at
  `spec show <active-version> --anchors`, mirroring `task new --area`.
- Re-pointing a completed or cancelled task is rejected (terminal tasks are delivered
  history, excluded from drift by the coverage orphan rule).
- The command changes nothing but `spec_ref` and the regenerated `STATE.md`
  projection: id, slug, filename, title, status, and dependencies are untouched, and
  no other task file is modified.
- `--dry-run` reports the old and new `spec_ref` and writes nothing; `--json` emits
  the planned or applied change.
- Automated coverage: service-level repoint tests (valid `--area`, explicit
  `--spec-ref`, unknown anchor, terminal-task rejection, `--area`/`--spec-ref`
  mutual exclusion) and CLI smoke tests for human, `--dry-run`, and `--json` output.
- README, workflow docs, and the Unreleased changelog document `task repoint` and its
  post-run `git status`/staging follow-up (it rewrites `STATE.md`).

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes

- Mirrors the `task new --area` anchor-resolution path; reuse it rather than
  re-implementing anchor matching.
