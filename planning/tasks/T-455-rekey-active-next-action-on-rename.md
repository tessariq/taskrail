---
id: T-455-rekey-active-next-action-on-rename
title: Re-key the active task's next_action on task rename
status: todo
priority: low
spec_ref: specs/v0.5.0.md#canonical-transition-order
dependencies:
    - T-454-rekey-blockers-on-rename
updated_at: "2026-11-25T23:12:45Z"
---

# T-455-rekey-active-next-action-on-rename Re-key the active task's next_action on task rename

## Description

Follow-up derived from T-454-rekey-blockers-on-rename's review.

`task rename` re-keys `current_task` (and, since T-454, the blockers entry and
a `Resolve blocker on <id>` pointer) but leaves the other `next_action` values
that name the active task on the old id. `start` writes
`Implement <id> and run targeted tests`, and `next` with an active task writes
`Continue task <id>`. After renaming the in-progress task, `STATE.md` points
`current_task` at the new id while `next_action` names an id that no longer
exists. `repair` does not notice it. Found by the T-454 review and reproduced on
the base build, so it predates T-454.

Outcome: renaming the task that `current_task` names re-keys an exact
`Implement <old-id> and run targeted tests` or `Continue task <old-id>` pointer to
the new id in the same transaction as the rest of the rename.

## Acceptance

- Renaming the in-progress task rewrites `next_action` from
  `Implement <old-id> and run targeted tests` to the same text with the new id.
- The same holds for `Continue task <old-id>`.
- Only those exact strings for the renamed id change; any other `next_action`,
  including one naming a different task or an id that merely starts with the
  old id, is unchanged.
- `task rename --dry-run` writes nothing, and its validation previews the
  re-keyed state.
- After the rename, `repair` (dry run) proposes no change.

## Verification Notes

- Failing-first unit test: start a task, rename it, assert the `Implement`
  pointer uses the new id; run `next`, rename again, assert the `Continue`
  pointer; add a prefix case (old id a prefix of another task's id) that stays
  unchanged.
- Full gate: `gofmt -l .`, `go vet ./...`, `go test ./...`, `taskrail validate`.

## Implementation Notes
