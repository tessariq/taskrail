---
id: T-393-report-lock-takeover-operands-when-recovery-is
title: Report lock takeover operands when recovery is pending
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies: []
updated_at: "2026-09-09T11:02:30Z"
completion_id: "0d5ed744a08c02c570a7a0c87f01cf68"
last_verification_id: "aae5f1bd06a3aa44c643d8656a0a3c63"
last_verification_result: pass
last_verified_at: "2026-09-09T11:02:30Z"
last_verified_completion_id: "0d5ed744a08c02c570a7a0c87f01cf68"
---

# T-393-report-lock-takeover-operands-when-recovery-is Report lock takeover operands when recovery is pending

## Description

Recovering an interrupted transaction is circular when the interrupted
process died holding the mutation lock, which is the ordinary case a
recovery exists for. `taskrail recover <id> --apply` refuses with
`lock_held` and names its own remedy:

> inspect it with taskrail lock status and use its exact
> --take-over-lock and --expect-sha256 operands before recovering

but `taskrail lock status` on that same repository refuses with
`recovery_pending` and reports no operands at all. Each command sends the
operator to the other, so the documented path out is closed.

Observed while verifying T-392: `init --apply --confirm-quiescent` was
killed mid-flight (SIGKILL, leaving a held lock and a retained
transaction), after which `recover --apply` demanded takeover operands
that `lock status` would not produce. Recovery only proceeded because the
lock id appeared incidentally in the `lock_held` message text and the
digest was computed by hand over `.git/taskrail/mutation.lock` — neither
is an operator-facing contract, and the second is exactly the raw-file
handling the CLI exists to prevent.

The takeover operands are a deliberate safety interlock, so the outcome
is to make them obtainable, not to weaken them. A pending recovery is a
state `lock status` should be able to describe: it is read-only evidence
about a lock, and withholding it protects nothing.

## Acceptance

- On a repository holding both a held mutation lock and a retained
  transaction, `taskrail lock status` reports the lock and its
  `--take-over-lock` and `--expect-sha256` operands instead of refusing
  with `recovery_pending`.
- The reported operands are accepted verbatim by `recover <id> --apply`
  on that repository, so the remedy the `lock_held` message names is
  followable end to end with no hand-computed digest and no reading of
  a storage file.
- The interlock is unchanged in strictness: a wrong or stale
  `--take-over-lock` or `--expect-sha256` is still refused, and a lock
  whose owner is alive is still not takeable.
- `lock status` stays read-only: it publishes no bytes and leaves the
  working tree clean whether or not a recovery is pending.

## Verification Notes

- Reproduce mechanically rather than by killing a process: seed a
  repository that holds a mutation lock and a retained transaction, then
  assert `lock status` reports operands and `recover --apply` accepts
  them.
- Cover the strictness cases in the same suite: a mismatched digest, an
  unknown lock id, and a live owner must each still refuse.
- Exercise the real interrupted flow once end to end over a corpus of a
  few hundred task files, since that is the size at which an interrupted
  upgrade is most likely to be met.

## Implementation Notes

- 2026-09-09T11:02:22Z: lock status now reports the held lock and its takeover operands for every retained transaction state; only an unreadable or moving transactions tree refuses. Closes the circular refusal between recover --apply (lock_held, naming the operands) and lock status (recovery_pending, withholding them), which the sidecar preparing/clearing markers an interrupted durable writer normally leaves triggered. Interlock and read-only scope unchanged.
- 2026-09-09T11:02:30Z: verification pass id aae5f1bd06a3aa44c643d8656a0a3c63 previous none completion 0d5ed744a08c02c570a7a0c87f01cf68
