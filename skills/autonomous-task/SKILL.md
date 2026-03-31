---
name: autonomous-task
description: Execute one specified Taskrail tracked task with deterministic workflow transitions
argument-hint: "[task-id]"
---

# autonomous-task

Execute one specified Taskrail tracked task with deterministic workflow transitions.

## Required Flow

1. Run `go run ./cmd/taskrail validate`.
2. Read the target task file.
3. Run `go run ./cmd/taskrail start <task-id>`.
4. Implement only the requested scope in a TDD loop.
5. Run the appropriate tests.
6. Run manual testing using the `autonomous-manual-test` skill when the task changes visible Taskrail behavior.
7. Run `go run ./cmd/taskrail verify <task-id> --result pass|fail --summary "..."`.
8. Create a follow-up task when unresolved work deserves backlog treatment.
9. Finish as `completed` or `blocked`.

## Rules

- do not auto-select another task
- do not hand-edit machine-managed state once the CLI exists
- do not hand-edit task status fields once the CLI exists
