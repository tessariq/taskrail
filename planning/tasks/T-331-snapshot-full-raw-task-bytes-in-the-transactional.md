---
id: T-331-snapshot-full-raw-task-bytes-in-the-transactional
title: Snapshot full raw task bytes in the transactional writer corpus recheck
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-282-protect-inherited-task-mutation-writers
updated_at: "2026-08-17T22:14:39Z"
---

# T-331-snapshot-full-raw-task-bytes-in-the-transactional Snapshot full raw task bytes in the transactional writer corpus recheck

## Description

The shared corpus recheck (snapshotTaskCorpus/sameTaskCorpus) compares parsed frontmatter plus body, so a lock-ignoring external editor that changes only parser-invisible bytes (for example an unmodeled frontmatter field) between corpus capture and the transaction snapshot can have its edit silently reverted by any transactional writer (lifecycle, verify, and the task mutation family). Snapshot and compare full raw file bytes (or derive candidates from the transaction snapshot) so any byte change refuses as a stale candidate, with tests injecting an unmodeled-byte edit in that window.

## Acceptance

- The follow-up issue described by verification is resolved.
- Verification evidence is updated.

## Verification Notes

- Re-run task-scoped verification after implementing the fix.
