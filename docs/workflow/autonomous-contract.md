# Autonomous Contract

Deterministic tracked-work contract for Taskrail repository planning and implementation.

## Source Of Truth

- `planning/STATE.md` frontmatter is machine-managed run state.
- `planning/STATE.md` is a bounded current snapshot, not a task/session log;
  durable context belongs with the task, blocker, verification report, or
  follow-up work.
- Optional `planning/NOTES.md` is human-owned repository context, outside
  machine-managed state. Agents read it when relevant and edit it only on an
  explicit human request.
- `planning/tasks/` contains tracked work item metadata, dependencies, and acceptance criteria.
- `docs/workflow/` contains the human-readable workflow contract.
- `internal/taskrail/skills/` is the embedded package source for the canonical
  skill set.
- `.agents/skills/` and `.claude/skills/` are committed parity copies of the
  package source.
- `${TASKRAIL:-taskrail}` is the transition path used by packaged skills.

## Lifecycle

The executable registry and metadata validator in
`internal/taskrail/lifecycle_contract.go` are the reusable v0.5 contract. Commands,
prompts, skills, and tests cite that contract rather than defining another order.

Task statuses are:

- `todo`
- `in_progress`
- `completed`
- `blocked`
- `cancelled`

The only canonical tracked-work run branches are:

```text
completed-pass: validate -> start -> implement -> checks -> complete -> verify pass
blocked-fail: validate -> start -> block --reason -> verify fail -> stop
rework-fail: validate -> start -> verify fail -> stop (remains in_progress)
```

Rules:

- At most one tracked item may be `in_progress`.
- `planning/STATE.md` must point at the same active task.
- Human or agent users should not hand-edit machine-managed state or task statuses once Taskrail commands are available.
- Human or agent users should never append continuation narratives to `STATE.md`.
- Success transitions to `completed` before recording a passing verification.
- Cannot-proceed transitions to `blocked` with a reason before recording a failing verification. Blocked work remains reversible through `unblock`.
- Deliberate rework may record a failing verification while remaining `in_progress`.
- Each canonical branch terminates the current autonomous run. `blocked` is still
  reversible through `unblock`; terminal-run does not mean immutable history.
- A direct operator may create a follow-up before completion with `task new`, use
  `verify --create-followup`, or release interrupted active work. A delegated child
  may create follow-ups only through `verify --create-followup` and cannot release.
- `task release <id> --reason <text>` is direct-operator recovery, never automatic
  continuation: it returns one consistently pointed-to active task to `todo`
  without creating blocker or cancellation history. `block` records an impediment,
  `unblock` resumes blocked work, and cancellation remains deliberate terminal
  history.

## Task Identity

In the v0.5 contract, every task operand, dependency, blocker identity, and
task-valued result is the exact full persisted task ID, including a slug suffix.
For example, `T-229-canonicalize-v0-5-lifecycle-and-task-identities` cannot be
addressed as `T-229`. A bare ID such as `T-230` is valid only when that exact bare
ID is persisted. v0.5 has no `task_ref` field, fuzzy resolver, or stable-reference
semantics; those begin in v0.6.

## Completion And Verification Metadata

Legacy tasks with no v0.5 lifecycle metadata remain readable until a later writer
adopts them. Once metadata is present, these are the only valid shapes:

| Lifecycle shape | `completion_id` | Latest verification tuple | Completion binding |
|---|---|---|---|
| Non-completed before verification | absent | absent | absent |
| Non-completed after verification | absent | complete | absent |
| Newly completed | required | absent | absent |
| Legacy completed before adoption | absent | absent or legacy-only | absent |
| Completed after fail or unbound pass | required, except legacy pre-adoption fail | complete tuple | absent |
| Completed with a current pass | required | complete pass tuple | equal to `completion_id` |

A complete verification tuple has an ID, `pass` or `fail` result, and timestamp;
its predecessor is absent for the first link and otherwise names the exact
immediately preceding verification ID. Partial tuples, repeated IDs, broken
predecessors, fail bindings, non-completed bindings, and bindings unequal to the
current completion ID are invalid. Verification records lifecycle state but never
performs a lifecycle transition.

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
