---
name: autonomous-backlog
description: Execute one deterministic autonomous backlog cycle for Taskrail tracked work
---

# autonomous-backlog

Execute one deterministic autonomous backlog cycle for Taskrail tracked work.

## Required Flow

1. Run `go run ./cmd/taskrail validate`.
2. Run `go run ./cmd/taskrail next --json`.
3. If no task is eligible, report that and stop.
4. Read the selected task file under `planning/tasks/`.
5. Run `go run ./cmd/taskrail start <task-id>`.
6. Implement in a TDD loop.
7. Run the appropriate test tiers.
8. Run manual testing using the `autonomous-manual-test` skill when the task changes visible Taskrail behavior.
9. Run `go run ./cmd/taskrail verify <task-id> --result pass|fail --summary "..."`.
10. If additional work is discovered, create a follow-up task.
11. Finish as `completed` or `blocked`.

## Rules

- never hand-edit `planning/STATE.md` frontmatter once the CLI exists
- never hand-edit task status fields once the CLI exists
- always keep evidence paths in notes and reports
- stop after one task
