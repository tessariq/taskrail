---
id: T-233-protect-lifecycle-and-task-writers-transactionally
title: Protect lifecycle and state-selection writers transactionally
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-156-protect-existing-semantic-writers-with-snapshot
    - T-157-upgrade-repositories-transactionally-to-layout-2
updated_at: "2026-08-17T20:29:56Z"
---

# T-233-protect-lifecycle-and-task-writers-transactionally Protect lifecycle and state-selection writers transactionally

## Description

Route `next`, `start`, `complete`, `block`, and `unblock` through the shared normal
transaction substrate. This task owns lifecycle and state-selection publication;
verification/follow-up and inherited task mutation writers are separate slices.

## Acceptance

- `next`, `start`, `complete`, `block`, and `unblock` acquire the discovered
  repository lock, snapshot their complete task/state read and write sets, and
  validate the full candidate ledger before publication.
- Each command publishes only its declared task and/or state files; non-selected
  task bytes and unrelated state history remain unchanged.
- Handled publication or post-validation failure compare-and-swap restores the
  original coherent bytes, while a concurrent-byte conflict returns the common
  partial-write/rollback evidence without overwriting the external edit.
- Direct and delegated execution enforce the selected command, task, and field
  capability before mutation; verification, follow-up creation, and other task
  mutations are not implemented by this slice.

## Verification Notes

- Run a table-driven command matrix in committed and applicable non-Git fixtures,
  asserting lock identity, consumed paths, candidate validation, and exact writes.
- Inject faults at every task/state publication and rollback boundary and compare
  original, candidate, and retained bytes plus schema-1 error snapshots.
- Use sentinel tasks and delegated-capability negatives to prove no save-all or
  cross-task mutation remains.

## Implementation Notes

- 2026-08-17T20:29:03Z: Routed next, start, complete, block, and unblock through the repository mutation lock and normal transaction substrate with exact task/state publication, full candidate validation, semantic corpus rechecks, selected-task metadata preservation, delegated capability enforcement, and rollback/conflict evidence.
- 2026-08-17T20:29:15Z: verification pass
- 2026-08-17T20:29:56Z: verification pass
