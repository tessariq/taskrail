---
name: autonomous-verify
description: Run deterministic verification against Taskrail tracked-work acceptance criteria and spec alignment
argument-hint: "[task-id]"
---

# autonomous-verify

Run deterministic verification against Taskrail tracked-work acceptance criteria and spec alignment.

## Required Flow

1. Run `go run ./cmd/taskrail validate`.
2. Choose the task to verify.
3. Run `go run ./cmd/taskrail verify <task-id> --result pass|fail --summary "..."`.
4. Confirm plan and report artifacts were written under `planning/artifacts/verify/`.
5. Review unresolved findings.
6. Create a follow-up task when unresolved work should enter the backlog.

## Rules

- verification-only runs should not mutate unrelated product code
- keep artifact paths in notes and reports
- keep verification grounded in the active spec and the task acceptance criteria
