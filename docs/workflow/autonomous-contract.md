# Autonomous Contract

Deterministic tracked-work contract for Taskrail repository planning and implementation.

## Source Of Truth

- `planning/STATE.md` frontmatter is machine-managed run state.
- `planning/tasks/` contains tracked work item metadata, dependencies, and acceptance criteria.
- `docs/workflow/` contains the human-readable workflow contract.
- `skills/` is the canonical skill set.
- `.agents/skills/` and `.claude/skills/` mirror the canonical skill files.
- `go run ./cmd/taskrail ...` is the intended transition path once the CLI exists.

## Lifecycle

Recommended task status lifecycle:

- `todo`
- `in_progress`
- terminal: `completed`, `blocked`, `cancelled`

Rules:

- At most one tracked item may be `in_progress`.
- `planning/STATE.md` must point at the same active task.
- Human or agent users should not hand-edit machine-managed state or task statuses once Taskrail commands are available.

## Deterministic Selection

`taskrail next` selects work in this order:

1. Consider only `todo` tasks.
2. Filter to tasks whose dependencies are resolved.
3. Sort by priority.
4. Break ties by stable task identifier.

## Verification Contract

- Run verification through `taskrail verify`.
- Verification writes plan and report artifacts under `planning/artifacts/verify/`.
- Verification should update `planning/STATE.md` with the last verification result and artifact paths.
- Follow-up work discovered during verification should become new task files when it deserves backlog treatment.

## Safety Rules

- Never hand-edit machine-managed state to force progress.
- Never hand-edit task status fields once the Taskrail CLI is available.
- Keep workflow commands non-interactive and scriptable.
- Completion and blocking notes should reference concrete evidence when relevant.
