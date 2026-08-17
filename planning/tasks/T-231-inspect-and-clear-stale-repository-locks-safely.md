---
id: T-231-inspect-and-clear-stale-repository-locks-safely
title: Inspect and clear stale repository locks safely
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-155-add-the-repository-mutation-lock-protocol
    - T-213-define-the-uniform-agent-machine-api
updated_at: "2026-08-17T11:22:48Z"
---

# T-231-inspect-and-clear-stale-repository-locks-safely Inspect and clear stale repository locks safely

## Description

Give operators a safe, explicit way to inspect and compare-and-delete an abandoned
repository lock without treating PID, host, or age as an automatic lease.

## Acceptance

- A1. `lock status` reports absence or the exact owner metadata and lock digest
  read-only in Git-common and non-Git root-local repositories.
- A2. `lock clear` requires the observed lock ID and digest, refuses a changed or
  provably live owner, and removes only the unchanged lock.
- A3. Text and schema-1 JSON expose no delegation token and never remove retained
  transaction data as a side effect of clearing ownership.

## Verification Notes

- A1: temporary ordinary, linked-worktree, and non-Git repositories prove root
  selection and zero-write inspection.
- A2: owner-live, digest-race, replacement, absent, and stale fixtures observe one
  compare-and-delete success or exact refusal.
- A3: golden output and filesystem sentinels prove token secrecy and transaction
  preservation.

## Implementation Notes

- 2026-08-17T11:22:32Z: Added internal/repolock Clear (compare-and-delete with ErrChanged/ErrLiveOwner, cross-platform signal-0/OpenProcess liveness probes), Service LockStatus/LockClear with companion-exact LockStatusResult/LockClearResult and registered machine-code mapping, and the lock status/lock clear CLI subcommands migrated onto the schema-1 envelope; README/docs/CHANGELOG updated.
- 2026-08-17T11:22:48Z: verification pass
