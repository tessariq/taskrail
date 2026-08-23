---
id: T-172-enforce-autonomous-loop-lifecycle-and-delivery
title: Derive autonomous loop lifecycle outcomes
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-217-release-interrupted-active-work-safely
    - T-311-integrate-portable-loop-process-containment
updated_at: "2026-08-23T17:14:00Z"
completion_id: "7359ebb6ac1ea9c8ba7fd4ca76688559"
last_verification_id: "3f925f99cece3af2d04bc83560b07155"
last_verification_result: pass
last_verified_at: "2026-08-23T17:14:00Z"
last_verified_completion_id: "7359ebb6ac1ea9c8ba7fd4ca76688559"
---

# T-172-enforce-autonomous-loop-lifecycle-and-delivery Derive autonomous loop lifecycle outcomes

## Description

Independently derive the selected task's lifecycle and verification candidate
after every child termination, then apply the exact invalid-postflight versus
child-failure precedence. Frozen-input/ledger integrity, Git delivery, and
invocation continuation are owned by T-312, T-313, and T-314.

## Acceptance

- After every launched child terminates, valid final lifecycle/state/report bytes
  independently derive exactly one candidate: `completed_pass`,
  `completed_audit_fail`, `completed_unverified`, `blocked_fail`, `rework_fail`,
  or `no_progress`; invalid final bytes leave the candidate null.
- Fresh verification requires a preflight-absent ID/path and exact task, result,
  timestamp, predecessor, completion ID, task/state/JSON/report agreement.
  Stale, audit-only, mismatched, or partially persisted evidence cannot satisfy a
  fresh outcome.
- `completed_audit_fail` requires a fresh bound pass for the current completion ID
  followed in the same iteration by a fresh fail whose predecessor is exactly
  that pass. Its evidence separately identifies the intermediate pass ID/binding
  and final fail; any completed fresh fail without this chain is inconsistent.
- Per-child outcome applies first-match precedence: invalid repository/lifecycle
  evidence or internally inconsistent candidate is `invalid_postflight`; launch,
  stream-copy, signal, non-zero exit, or required T-311 containment failure is
  `child_failed`; otherwise use the derived candidate. A failed child still
  reports any valid candidate and persisted lifecycle evidence.
- Every child termination yields the exact `LoopIteration` lifecycle fields and
  nullability, including before/after status, completion and verification IDs,
  observed completion binding, audit-pass fields, child exit/signal/timeout,
  policy row, and prompt identity. This task does not attest Git delivery,
  semantic review convergence, hooks, opaque provenance, or continuation.

## Verification Notes

- Table-driven lifecycle fixtures map each final status/evidence shape and each
  malformed or stale binding to the exact candidate and `LoopIteration` fields.
- Precedence fixtures combine valid lifecycle writes with non-zero exit, signal,
  stream failure, timeout/containment failure, invalid report chains, and invalid
  final repository state to prove first-match classification.
- Boundary tests cover pass-before-complete, completed-unverified recovery-only
  verify, block/rework fail, partial complete, audit pass-to-fail chaining, stale
  artifacts, and null candidate derivation without depending on Git delivery.

## Implementation Notes

- 2026-08-23T17:13:51Z: Added lifecycle candidate derivation with fresh verification and child-failure precedence.
- 2026-08-23T17:14:00Z: verification pass id 3f925f99cece3af2d04bc83560b07155 previous none completion 7359ebb6ac1ea9c8ba7fd4ca76688559
