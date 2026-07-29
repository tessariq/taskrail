---
name: autonomous-task
description: Execute one specified Taskrail tracked task with deterministic workflow transitions
argument-hint: "[task-id]"
---

# autonomous-task

Execute one specified Taskrail tracked task with deterministic workflow transitions.

Requires the installed `taskrail` binary on `PATH`.

## Source Checkout Guard

Before every command that can write tracked state, check whether the repository
is the Taskrail source checkout (it contains both `Taskfile.yml` and
`internal/toolchain/cmd/freshcheck`). If so, run `task taskrail:check`
immediately before the writer. This checks the exact `${TASKRAIL:-taskrail}`
binary the workflow will invoke. If it fails, stop, apply the remedy it names,
and rerun the guard; do not run the writer first. Installed adopter repositories
do not contain the source helper and skip this source-only guard.

## Required Flow

1. Run `${TASKRAIL:-taskrail} validate`.
2. Read the target task file.
3. Run `${TASKRAIL:-taskrail} start <task-id>`.
4. Implement only the requested scope in a TDD loop.
5. Run the appropriate tests.
6. Run manual testing when the task changes visible behavior.
7. Run `${TASKRAIL:-taskrail} verify <task-id> --result pass|fail --summary "..."`.
8. Create a follow-up task with `${TASKRAIL:-taskrail} task new` when unresolved work
   deserves backlog treatment.
9. Finish as `completed` or `blocked`.

## Rules

- do not auto-select another task
- do not hand-edit machine-managed state
- do not hand-edit task status fields
- create follow-up tasks with `${TASKRAIL:-taskrail} task new`, never by hand-authoring markdown
