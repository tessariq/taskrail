---
id: T-332-gate-task-mutation-permission-fault-tests-on
title: Gate task mutation permission fault tests on Windows
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-282-protect-inherited-task-mutation-writers
updated_at: "2026-08-17T22:24:25Z"
---

# T-332-gate-task-mutation-permission-fault-tests-on Gate task mutation permission fault tests on Windows

## Description

Restore native Windows CI after T-282 added Unix permission-based publication
fault tests. Windows does not enforce the directory mode used by those fixtures;
portable transaction rollback and conflict tests remain active.

Follow-up derived from T-282-protect-inherited-task-mutation-writers's verification or discovery.

## Acceptance

- Permission-mode fault injections skip explicitly on Windows and root hosts.
- Portable mutation, rollback, conflict, and exact-write-set coverage remains.
- Focused, full, and native Windows CI tests pass.

## Verification Notes

- Run focused task-mutation tests, full tests, vet, Windows test compilation, and
  post-push native Windows CI.

## Implementation Notes

- 2026-08-17T22:24:17Z: Centralized the taskrail-package permission-fault capability skip and gated both task mutation and CLI smoke fault fixtures on Windows or root hosts, preserving portable rollback coverage.
- 2026-08-17T22:24:25Z: verification pass
