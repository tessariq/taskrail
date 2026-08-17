---
id: T-326-unblock-the-operator-lock-surface-under-a-retained
title: Unblock the operator lock surface under a retained recovery fence
status: todo
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-232-recover-v0-5-transactions-through-one-command
updated_at: "2026-08-17T12:18:28Z"
---

# T-326-unblock-the-operator-lock-surface-under-a-retained Unblock the operator lock surface under a retained recovery fence

## Description

An abruptly killed durable writer leaves both the mutation lock and a retained recovery fence: recover refuses lock_held (any holder) while lock status and lock clear refuse recovery_pending, so the crash scenario recover exists for currently has no CLI path to clear the abandoned lock. Decide and implement the sanctioned operator sequence - most plausibly admitting the lock family past the admission fence, or a guarded takeover of a provably dead owner's lock - and update the T-231 fence tests and docs to match the chosen contract. Spec anchor: specs/v0.5.0.md#repository-discovery-locking-and-recovery.

## Acceptance

- The follow-up issue described by verification is resolved.
- Verification evidence is updated.

## Verification Notes

- Re-run task-scoped verification after implementing the fix.
