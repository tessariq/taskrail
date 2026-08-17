---
id: T-280-publish-layout-2-through-the-durable-migration
title: Publish layout 2 through the durable migration fence
status: completed
priority: high
spec_ref: specs/v0.5.0.md#layout-compatibility-and-upgrade
dependencies:
    - T-279-preview-complete-layout-2-migration-decisions
    - T-232-recover-v0-5-transactions-through-one-command
    - T-277-add-durable-transaction-journals-and-recovery
    - T-272-implement-init-status-and-warning-machine
updated_at: "2026-08-17T17:36:29Z"
---

# T-280-publish-layout-2-through-the-durable-migration Publish layout 2 through the durable migration fence

## Description

Publish the exact approved layout-2 candidate through the durable migration fence,
keeping marker, state, notes, tasks, and eligible skills on one recoverable
compatibility boundary and proving supported old/new binary behavior.

## Acceptance

- A1. Apply requires the writer lock, matching complete preview decisions, and
  explicit quiescence; it durably records originals before publishing the exact
  fenced layout-2 marker ahead of any semantic candidate byte.
- A2. Apply rechecks all source snapshots, publishes the complete candidate, post-
  validates it, and atomically replaces the fence with the strict final marker as
  its last successful operation. Both fence and final marker preserve the previewed
  default broad review-round maximum `1`.
- A3. Handled failure restores candidate-written bytes before the original marker,
  only when unchanged; interruption leaves an exact shared recovery action and
  blocks incompatible reads/writes without overwriting concurrent edits.
- A4. The complete compatibility matrix covers direct and multi-hop schema-1
  upgrades, already-current/direct-schema-2 flows, note choices, skill refresh,
  preserved task policy, and old-binary refusal of layout 2.
- A5. A successful migration reports the same candidate paths and decisions as
  preview, leaves no recovery fence, and directs downgrade through complete Git
  reversion rather than marker editing.

## Verification Notes

- A1: inspect lock/journal/fence ordering during an applied sandbox migration and
  verify no semantic byte precedes the durable fence.
- A2: compare preview bytes with final marker/state/note/task/skill bytes and
  capture successful post-validation plus fence removal order.
- A3: interrupt and fail every publication phase, race external edits, and use the
  shared recovery preview/apply to observe safe restore, accept, or clear outcomes.
- A4: execute the full source/decision/skill/task-policy matrix and record both old-
  binary refusal and current-binary inspection behavior.
- A5: compare preview/apply machine results, confirm no retained transaction after
  success, and exercise documented Git-reversion downgrade guidance.

## Implementation Notes

- 2026-08-17T17:40:00Z: Published the gated layout-2 apply through one durable transaction: the durabletx engine gained a fence member (intermediate fence bytes after durable originals and before any semantic byte, final candidate bytes as the last semantic operation after post-publication validation, retained finals so recovery completes publication mechanically, restore-last ordering enforced structurally); init --apply acquires the writer lock naming the transaction, rebuilds and re-gates the candidate under it, and publishes marker/state/notes/skills with consumed parity rechecks; a fenced marker refuses ordinary commands as migration_in_progress (recovery_pending while the transaction is retained) and admits only the recovery boundary, which now registers the init-migration validator; the applied result reports the previewed paths/decisions plus the recorded note choice and Git-reversion downgrade guidance.
- 2026-08-17T17:36:15Z: Published the gated layout-2 apply through one durable fenced transaction with recovery admission, the init recovery validator, and preview/apply parity
- 2026-08-17T17:36:29Z: verification pass
