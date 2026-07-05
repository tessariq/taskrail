---
name: autonomous-verify
description: Run deterministic verification against Taskrail tracked-work acceptance criteria and spec alignment
argument-hint: "[task-id]"
---

# autonomous-verify

Run deterministic verification against Taskrail tracked-work acceptance criteria and spec alignment.

Requires the installed `taskrail` binary on `PATH`.

## Required Flow

1. Run `taskrail validate`.
2. Choose the task to verify.
3. Run `taskrail verify <task-id> --result pass|fail --summary "..."`.
4. Confirm plan and report artifacts were written under
   `planning/artifacts/verify/`.
5. Review unresolved findings.
6. Create a follow-up task with `taskrail task new` (or
   `taskrail verify <task-id> --create-followup`) when unresolved work should
   enter the backlog.

## Rules

- verification-only runs should not mutate unrelated product code
- keep artifact paths in notes and reports
- keep verification grounded in the active spec and the task acceptance criteria
- create follow-up tasks with `taskrail task new`, never by hand-authoring markdown
