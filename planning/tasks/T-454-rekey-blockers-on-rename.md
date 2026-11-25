---
id: T-454-rekey-blockers-on-rename
title: Re-key blockers ledger on task rename
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#canonical-transition-order
dependencies:
    - T-453-recompute-idle-status-on-input-change
updated_at: "2026-11-25T13:07:17Z"
completion_id: "fd089cfa2a7f787b4a37f42f2ce3df9b"
last_verification_id: "91724f7ae878b4c993c541469d92a501"
last_verification_result: pass
last_verified_at: "2026-11-25T13:07:17Z"
last_verified_completion_id: "fd089cfa2a7f787b4a37f42f2ce3df9b"
---

# T-454-rekey-blockers-on-rename Re-key blockers ledger on task rename

## Description

Follow-up derived from T-453-recompute-idle-status-on-input-change's review.

`task rename` re-keys `current_task` but not the `STATE.md` blockers ledger or a
`Resolve blocker on <old-id>` next action. Renaming a blocked task therefore
leaves a ledger entry under the old id. `unblock` on the new id cannot remove
it, and the idle projection (T-452, T-453) treats an entry whose task is absent
as blocking. The state stays `blocked` on a phantom id, and `repair --apply`
proposes `blocked` on that phantom id. Reproduced on 5a54b0e and on the T-453
candidate by the T-453 review: on 5a54b0e, rename then unblock leaves
`blocked`/`Resolve blocker on <old-id>`. On T-453, after `spec activate` makes
the state idle, rename followed by `repair` proposes `blocked` on the old id.

Outcome: `task rename` re-keys the renamed task's blockers entry, keeping its
reason, and any `Resolve blocker on <old-id>` next action to the new id, in the
same transaction as the rest of the rename.

## Acceptance

- Renaming a blocked task rewrites its `STATE.md` blockers entry from
  `<old-id>: <reason>` to `<new-id>: <reason>`; other entries are unchanged.
- A `next_action` of `Resolve blocker on <old-id>` becomes `Resolve blocker on
  <new-id>`; any other `next_action` is unchanged.
- After the rename, `unblock <new-id>` clears the entry and the idle projection
  reads `idle` when no other active-spec blocker remains.
- After renaming a blocked task, `repair` (dry run) proposes no change.
- `task rename --dry-run` still writes nothing.

## Verification Notes

- Failing-first unit test: block, rename, then assert the ledger entry and
  pointer; unblock the new id and assert `idle`; `repair` dry run reports no
  changes.
- Full gate: `gofmt -l .`, `go vet ./...`, `go test ./...`, `taskrail validate`.

## Implementation Notes

- 2026-11-25T13:07:16Z: task rename now re-keys the renamed task's blockers entry (reason and position kept, matched with blockerID like unblock) and an exact Resolve blocker pointer to the new id in renamePreview, so dry run and apply share it. Review found nothing blocking; an Implement <old-id> pointer surviving a rename of the active task predates this and is out of scope.
- 2026-11-25T13:07:17Z: verification pass id 91724f7ae878b4c993c541469d92a501 previous none completion fd089cfa2a7f787b4a37f42f2ce3df9b
