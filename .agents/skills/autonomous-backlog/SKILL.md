---
name: autonomous-backlog
description: Execute one deterministic autonomous backlog cycle for Taskrail tracked work
---

# autonomous-backlog

Execute one deterministic autonomous backlog cycle for Taskrail tracked work.

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
2. Run `${TASKRAIL:-taskrail} next --json`.
3. If no task is eligible, report that and stop.
4. Read the selected task file under `planning/tasks/`.
5. Run `${TASKRAIL:-taskrail} start <task-id>`.
6. Implement in a TDD loop.
7. Run the appropriate test tiers.
8. Run manual testing when the task changes visible behavior.
9. Run `${TASKRAIL:-taskrail} verify <task-id> --result pass|fail --summary "..."`.
10. If additional work is discovered, create a follow-up task with
    `${TASKRAIL:-taskrail} task new` (or `${TASKRAIL:-taskrail} verify <task-id> --create-followup`).
11. Finish as `completed` or `blocked`.

## Rules

- never hand-edit `planning/STATE.md` frontmatter
- treat `planning/STATE.md` as current state, never as a task/session log; put
  durable context in task implementation notes, blocker reasons, portable
  verification summaries/reports, or follow-up tasks
- treat optional `planning/NOTES.md` as human-owned repository context: read it
  when relevant, edit it only on explicit human request
- never hand-edit task status fields
- create follow-up tasks with `${TASKRAIL:-taskrail} task new`, never by hand-authoring markdown
- keep concrete local artifact paths in ephemeral reports; committed task notes
  use portable summaries
- stop after one task
