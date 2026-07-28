---
id: T-128-task-new-title-portability
title: Guard task new title against gitignored artifact paths
status: completed
priority: medium
spec_ref: specs/v0.2.0.md#portable-committed-state
dependencies: []
updated_at: "2026-07-28T12:29:56Z"
---

# T-128-task-new-title-portability Guard task new title against gitignored artifact paths

## Description

Discovered reviewing T-127. `CreateTask` (`taskrail task new`) writes `--title`
straight into the committed task heading via `renderNewTaskBody` with no
portability check, even though `taskArtifactRefs` treats a task `Title` exactly
like its body for dangling-path detection (specs/v0.2.0.md#portable-committed-state).
So `task new --title "... planning/artifacts/.../report.md"` scaffolds a task the
next `validate` rejects — the same "write succeeds, repo left invalid" failure class
T-127 closed for transition notes, but via the task-creation writer (not a
transition, hence out of T-127's scope).

Extend the same `ensurePortableNote`-style guard (reusing `danglingArtifactPaths`)
to `CreateTask`'s effective title before it writes the task file.

## Acceptance

- `task new` fails before writing when `--title` (or a `--slug`-independent title
  source) embeds a concrete gitignored `planning/artifacts/` file path, naming the
  offending path.
- On rejection no task file is written and `STATE.md` is unchanged.
- Directory-prefix / placeholder prose that `validate` allows is still accepted.
- Reuses the shared `danglingArtifactPaths` detector — no second path rule.
- Automated coverage: service reject + accept tests and a CLI smoke test.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes

- 2026-07-28T12:29:49Z: verification pass
- 2026-07-28T12:29:56Z: Guarded task new title via shared ensurePortableNote and mirrored the check in import pre-flight; follow-up T-132 filed for partial-apply reporting
