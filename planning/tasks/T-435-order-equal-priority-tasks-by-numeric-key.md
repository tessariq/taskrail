---
id: T-435-order-equal-priority-tasks-by-numeric-key
title: Order equal-priority tasks by numeric task key
status: todo
priority: low
spec_ref: specs/v0.5.0.md#task-local-loop-policy
dependencies: []
updated_at: "2026-10-31T11:39:12Z"
---

# T-435-order-equal-priority-tasks-by-numeric-key Order equal-priority tasks by numeric task key

## Description

Bug. Selection breaks priority ties by comparing full task IDs as strings
(internal/taskrail/tasks.go `eligibleTasks`, repeated in loop_dry_run.go and
loop_list.go). Once a repository passes 999 tasks, `task new` creates T-1000,
which then sorts before T-999. Selection stays deterministic but no longer
follows creation order, and the four copies of the comparator can drift. Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- One shared comparator orders by priority, then numeric task key, then full ID, and is used by `next`, `status`, loop dry-run, loop execution and `task loop list`.
- T-999 sorts before T-1000 at equal priority.
- A shuffle property test asserts the same order for any input permutation.

## Verification Notes

- Unit tests over `eligibleTasks` and the loop ranking.

## Implementation Notes
