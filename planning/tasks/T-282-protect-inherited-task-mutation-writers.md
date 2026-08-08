---
id: T-282-protect-inherited-task-mutation-writers
title: Protect inherited task mutation writers transactionally
status: todo
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-233-protect-lifecycle-and-task-writers-transactionally
updated_at: "2026-08-08T14:23:08Z"
---

# T-282-protect-inherited-task-mutation-writers Protect inherited task mutation writers transactionally

## Description

Route inherited task mutation commands through normal transactions: `task new`,
`task rename`, `task repoint`, and dependency add/remove. Lifecycle, verification,
task authoring, release, and loop-policy semantics remain with their owning tasks.

## Acceptance

- Each owned command locks once, snapshots the complete task/spec/state and
  collision read set it consumes, validates the candidate ledger, and publishes
  only its declared task/state paths.
- Rename publishes by filesystem operations rather than Git staging and preserves
  exact identity/body invariants; repoint and dependency edits alter only their
  contract fields plus the state projection where required.
- Destination collision, stale source, invalid spec reference, dependency error,
  cycle, or delegated-capability mismatch refuses before semantic publication.
- Handled multi-file failures roll back unchanged candidates and report common
  conflict/recovery snapshots without rewriting unrelated task bytes.

## Verification Notes

- Cover each command with exact before/after task and state snapshots, collision
  fixtures, dependency-cycle cases, and sentinel non-target tasks.
- Inject faults across rename/create/task/state publication and rollback boundaries.
- Assert delegated command/task/field negatives and an unchanged Git index.

## Implementation Notes
