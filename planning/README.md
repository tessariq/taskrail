# Planning Scope

`planning/` contains the tracked execution layer for this repository.

## Current Default Scope

- active spec: `specs/v0.5.0.md`
- active version focus: `v0.5.0`

## Rules

- `planning/STATE.md` is the authoritative current execution projection.
- `planning/NOTES.md` is optional human-owned repository context; agents edit it
  only on explicit human instruction.
- `planning/tasks/` contains one file per tracked work item.
- Every task must reference at least one live heading in a spec file.
- Dependency references must point to existing task IDs.
- Verification artifacts are written under `planning/artifacts/verify/` on
  demand; the gitignored artifacts tree and placeholder files are not required.
- Temporary dogfooding workflow docs live under `docs/workflow/` until Taskrail itself replaces more of the manual scaffolding.
