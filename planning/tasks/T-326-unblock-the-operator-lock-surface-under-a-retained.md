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

An abruptly killed durable writer leaves both the mutation lock and a retained
recovery fence. Make that state recoverable through one explicit, exact-observation
takeover: admit read-only lock inspection through the fence, preview the retained
transaction against the operator-supplied lock identity, and let confirmed
recovery apply compare-and-delete only that unchanged lock before acquiring
recovery ownership. Preserve the rule that Taskrail never infers a remote owner
dead or clears a lock from PID, host, or age alone.

## Acceptance

- `lock status` remains read-only and reports exact owner metadata and raw-file
  digest while a valid recovery fence exists; malformed or substituted recovery
  state still fails closed, and `lock clear` does not bypass the fence.
- `recover <transaction-id> --take-over-lock <lock-id> --expect-sha256 <digest>`
  requires both takeover operands, requires the lock metadata's transaction ID to
  equal the requested transaction, and previews the same mechanically derived
  recovery action without changing the lock, journal, fence, or managed bytes.
- Confirmed `--apply` compare-and-deletes only the exact unchanged observed lock,
  acquires recovery ownership, rechecks the complete transaction snapshot, and
  performs only the previewed restore-original, accept-candidate, or clear-fence
  action. A changed lock, replacement owner, or lost acquisition race refuses
  without changing transaction state.
- A provably live same-host owner always returns `lock_held`. A different-host
  owner is not inferred dead; the exact ID/digest operands are explicit operator
  authorization, with no automatic takeover, timeout, lease, or age policy.
- Text, strict JSON, machine schema/error registration, command help, recovery
  docs, and T-337-consumed diagnostics distinguish ordinary lock refusal,
  takeover-required preview, successful takeover, and recovery outcome without
  exposing delegation secrets or weakening ordinary writer admission.

## Verification Notes

- Unit and CLI fixtures cover fenced lock status, missing/paired/malformed
  takeover operands, transaction mismatch, same-host live refusal, same-host
  exited and explicitly authorized cross-host owners, changed bytes, takeover
  races, preview purity, apply interruption, and every recovery action.
- Re-run recovery admission, durable transaction, lock, machine-contract, race,
  full test, vet, build, cross-platform, Taskrail validation, and manual sandbox
  checks with portable evidence.
