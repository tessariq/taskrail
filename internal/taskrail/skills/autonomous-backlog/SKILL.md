---
name: autonomous-backlog
description: Execute one deterministic autonomous backlog cycle for Taskrail tracked work
---

# autonomous-backlog

Execute one deterministic autonomous backlog cycle for Taskrail tracked work.

Requires the installed `taskrail` binary on `PATH`.

## Required Flow

1. Run `taskrail validate`.
2. Run `taskrail next --json`.
3. If no task is eligible, report that and stop.
4. Read the selected task file under `planning/tasks/`.
5. Run `taskrail start <task-id>`.
6. Implement in a TDD loop.
7. Run the appropriate test tiers.
8. Run manual testing when the task changes visible behavior.
9. Run `taskrail verify <task-id> --result pass|fail --summary "..."`.
10. If additional work is discovered, create a follow-up task with
    `taskrail task new` (or `taskrail verify <task-id> --create-followup`).
11. Finish as `completed` or `blocked`.

## Rules

- never hand-edit `planning/STATE.md` frontmatter
- never hand-edit task status fields
- create follow-up tasks with `taskrail task new`, never by hand-authoring markdown
- always keep evidence paths in notes and reports
- stop after one task
