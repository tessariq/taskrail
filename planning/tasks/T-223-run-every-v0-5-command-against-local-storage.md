---
id: T-223-run-every-v0-5-command-against-local-storage
title: Route existing commands through local storage context
status: todo
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-222-initialize-and-discover-ignored-local-taskrail
    - T-233-protect-lifecycle-and-task-writers-transactionally
    - T-234-protect-repository-and-planning-writers
updated_at: "2026-08-05T22:04:22Z"
---

# T-223-run-every-v0-5-command-against-local-storage Route existing commands through local storage context

## Description

Provide the shared path-neutral repository context and retrofit the already
shipped command families that downstream v0.5 prompt, review, and loop tasks
consume. Local mode must preserve task/spec/lifecycle meaning while writing only
beneath its ignored storage context; each later command task owns its own local
integration acceptance.

## Acceptance

- Shared readers/writers resolve configured logical and physical roots; no
  template, artifact guard, rename, validation, or state renderer requires
  literal repository-root `specs/` or `planning/` paths.
- Existing lifecycle, task/spec/import, repair, and local status/path commands
  produce equivalent semantic results in committed and local fixtures; new prompt,
  review, and loop tasks consume the same context and prove their own behavior.
- Local writers remain untracked/unstaged and ordinary Git status clean; rename
  uses filesystem publication rather than staging local files.
- Local prompt and skill feature tasks consume this context but own their separate
  content, collision, exclusion, delivery, and promotion behavior.
- Status exposes exact storage mode/root from the same context so agents never
  inspect config or probe physical semantic paths to determine delivery mode.
- Unsupported mode-specific behavior returns a classified capability refusal and
  never falls back to committed paths or creates mixed state.

## Verification Notes

- Build a table-driven committed/local command matrix with exact state/task/spec
  snapshots, Git index/status assertions, custom invocation cwd, and fault points.
- Run skill parity plus sandbox CLI smoke workflows through create, start, verify,
  complete/block, repair, import, and rename paths; expose reusable fixtures for
  downstream prompt/review/loop tasks.

## Implementation Notes
