---
id: T-217-release-interrupted-active-work-safely
title: Release interrupted active work safely
status: completed
priority: high
spec_ref: specs/v0.5.0.md#canonical-transition-order
dependencies:
    - T-158-bind-completion-and-verification-with-stable
    - T-223-run-every-v0-5-command-against-local-storage
updated_at: "2026-08-22T09:59:26Z"
completion_id: "a6d7f3e00319beff7a0ca8954daadf7b"
last_verification_id: "00bdfe78ba26cff4964a549cbf615f58"
last_verification_result: pass
last_verified_at: "2026-08-22T09:59:26Z"
last_verified_completion_id: "a6d7f3e00319beff7a0ca8954daadf7b"
---

# T-217-release-interrupted-active-work-safely Release interrupted active work safely

## Description

Add a direct-operator recovery transition that relinquishes interrupted or
deliberate-rework active work without inventing blocker history or cancelling it.

## Acceptance

- `task release` accepts exactly one full-ID `in_progress` task plus a bounded portable reason; dry-run and common JSON report the exact candidate and refusal class.
- Apply changes status to `todo`, clears both matching current-task fields,
  recomputes summary/blockers/next action from the candidate ledger, appends a
  timestamped Implementation Note, validates, and publishes transactionally.
- Reason grammar, exact success/error JSON, candidate digests, contradictory
  pointer refusal, and dry-run/apply parity follow the canonical contract.
- Completion/verification history, identity, body, dependencies, spec reference, and paired loop policy remain unchanged. No blocker or cancellation provenance is created.
- Other statuses, unknown IDs, delegated loop ownership, and retained recovery transactions refuse without writes.
- Human/agent recovery docs distinguish release from unblock, cancel, reopen, and automatic continuation.

## Verification Notes

- Exercise every status/storage/pointer combination, reason grammar/YAML quoting, preview/apply parity, task/state rollback, lock delegation, and exact byte preservation.
- Run an interrupted-loop sandbox that releases work, proves ordinary next-task eligibility, and records no fabricated blocker history.

## Implementation Notes

- 2026-08-22T09:59:10Z: Implemented direct release recovery with transactional preimage guards.
- 2026-08-22T09:59:26Z: verification pass id 00bdfe78ba26cff4964a549cbf615f58 previous none completion a6d7f3e00319beff7a0ca8954daadf7b
