---
id: T-156-protect-existing-semantic-writers-with-snapshot
title: Protect existing semantic writers with snapshot transactions
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-155-add-the-repository-mutation-lock-protocol
updated_at: "2026-08-04T21:32:13Z"
---

# T-156-protect-existing-semantic-writers-with-snapshot Protect existing semantic writers with snapshot transactions

## Description

Retrofit every existing task, lifecycle, state, spec, import, repair, and init
semantic writer to hold the shared mutation lock across candidate reads,
validation, publication, rollback, and post-validation.

## Acceptance

- Every existing writer acquires before semantic candidate reads and retains
  ownership through successful post-validation or rollback; unrelated writers
  refuse without writes.
- Each writer snapshots all files it may publish, rechecks bytes before
  publication, and never overwrites a concurrent edit.
- Multi-file failure/interruption restores only unchanged originals and reports
  any partial condition with exact safe recovery.
- Existing command success outputs and read-only command behavior remain
  compatible aside from lock-contention diagnostics.
- A process that cannot prove lock ownership cannot enter any existing semantic
  write path.

## Verification Notes

- Map each writer family to setup/concurrent action/public result/file snapshot
  evidence, including linked worktrees and unrelated processes.
- Fault-inject candidate reads, publication, post-validation, and rollback per
  transaction shape without duplicating every command's semantic tests.

## Implementation Notes
