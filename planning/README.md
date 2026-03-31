# Planning Scope

`planning/` contains the tracked execution layer for this repository.

## Current Default Scope

- active spec: `specs/v0.1.0.md`
- active version focus: `v0.1.0`

## Rules

- `planning/STATE.md` is the authoritative current state.
- `planning/tasks/` contains one file per tracked work item.
- Every task must reference at least one live heading in a spec file.
- Dependency references must point to existing task IDs.
- Verification artifacts are written under `planning/artifacts/verify/`.
- Temporary dogfooding workflow docs live under `docs/workflow/` until Taskrail itself replaces more of the manual scaffolding.
