---
id: T-233-protect-lifecycle-and-task-writers-transactionally
title: Protect lifecycle and task writers transactionally
status: todo
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-156-protect-existing-semantic-writers-with-snapshot
    - T-157-upgrade-repositories-transactionally-to-layout-2
updated_at: "2026-08-06T13:46:30Z"
---

# T-233-protect-lifecycle-and-task-writers-transactionally Protect lifecycle and task writers transactionally

## Description

Retrofit existing lifecycle, verification, task, dependency, and state writers to
the shared normal transaction substrate with explicit write sets and capabilities.

## Acceptance

- A1. Each writer locks the correct repository, snapshots the complete consumed
  set, validates candidates before publication, and rewrites only declared files.
- A2. Every handled write/post-validation failure compare-and-swap rolls back to
  the original coherent task/state result or reports retained recovery evidence.
- A3. Non-selected tasks remain byte-identical and delegated writers cannot exceed
  the selected command/task/field capability.

## Verification Notes

- A1: writer-matrix integration tests assert lock root, read/write sets, and
  candidate validation for committed, local, and non-Git applicable cases.
- A2: inject failure at each publication/rollback boundary and compare all bytes.
- A3: sentinel tasks and delegated capability negatives prove no broad save-all or
  unrelated mutation remains.

## Implementation Notes
