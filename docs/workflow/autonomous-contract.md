# Autonomous Contract

Deterministic tracked-work contract for Taskrail repository planning and implementation.

## Source Of Truth

- `planning/STATE.md` frontmatter is machine-managed run state.
- `planning/STATE.md` is a bounded current snapshot, not a task/session log;
  durable context belongs with the task, blocker, verification report, or
  follow-up work.
- `planning/tasks/` contains tracked work item metadata, dependencies, and acceptance criteria.
- `docs/workflow/` contains the human-readable workflow contract.
- `internal/taskrail/skills/` is the embedded package source for the canonical
  skill set.
- `.agents/skills/` and `.claude/skills/` are committed parity copies of the
  package source.
- `${TASKRAIL:-taskrail}` is the transition path used by packaged skills.

## Lifecycle

Recommended task status lifecycle:

- `todo`
- `in_progress`
- terminal: `completed`, `blocked`, `cancelled`

Rules:

- At most one tracked item may be `in_progress`.
- `planning/STATE.md` must point at the same active task.
- Human or agent users should not hand-edit machine-managed state or task statuses once Taskrail commands are available.
- Human or agent users should never append continuation narratives to `STATE.md`.

## Deterministic Selection

`taskrail next` selects work in this order:

1. Consider only `todo` tasks.
2. Filter to tasks whose dependencies are resolved.
3. Filter to tasks whose `spec_ref` points at the active spec.
4. Sort by priority.
5. Break ties by stable task identifier.

Steps 3–5 apply to idle selection. When only older/other-spec tasks are runnable,
`next` reports no eligible task and lists the skipped work under `warnings`
(`skipped_non_active_spec`) rather than selecting it — recover such work
explicitly with `start <id>`. An already-active `in_progress` task is always
returned so it can be continued; if it points outside the active spec, `next`
adds a `selected_non_active_spec` warning. Read-only `status` computes the same
selection without writing state.

## Verification Contract

- Run verification through `taskrail verify`.
- Verification writes ephemeral, gitignored plan and report artifacts under
  `planning/artifacts/verify/`; never commit them.
- Verification updates `planning/STATE.md` with a portable, path-free summary of
  the last verification result.
- Follow-up work discovered during verification should become new task files when it deserves backlog treatment.

## Safety Rules

- Never hand-edit machine-managed state to force progress.
- Never hand-edit task status fields once the Taskrail CLI is available.
- Keep workflow commands non-interactive and scriptable.
- Completion and blocking notes should reference concrete evidence when relevant.
