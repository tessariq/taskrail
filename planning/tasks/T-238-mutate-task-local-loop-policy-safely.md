---
id: T-238-mutate-task-local-loop-policy-safely
title: Mutate task-local loop policy safely
status: completed
priority: high
spec_ref: specs/v0.5.0.md#task-local-loop-policy
dependencies:
    - T-168-parse-and-validate-an-optional-autonomous-run
    - T-282-protect-inherited-task-mutation-writers
updated_at: "2026-08-25T18:08:12Z"
completion_id: "31932a1fbcd3649c683c8caa8158c4b3"
last_verification_id: "7857aa6cebfdd4a52be5324fe8b6b31c"
last_verification_result: pass
last_verified_at: "2026-08-25T18:08:12Z"
last_verified_completion_id: "31932a1fbcd3649c683c8caa8158c4b3"
---

# T-238-mutate-task-local-loop-policy-safely Mutate task-local loop policy safely

## Description

Add direct-operator allow, hold, and clear commands as one selected-task
transaction without coupling policy to lifecycle selection or generated follow-ups.

## Acceptance

- A1. Mutators enforce exact ID, reason grammar, lifecycle restrictions, dry-run
  parity, timestamp/state reprojection, and exact prior/candidate results.
- A2. Clear removes both persisted fields and restores deterministic implicit hold;
  all unrelated task/body/frontmatter bytes remain identical.
- A3. Delegated ownership always refuses policy mutation before writing.

## Verification Notes

- A1: status/reason boundary tables exercise all operations in committed and local
  storage and compare preview/apply candidates.
- A2: raw-byte sentinels prove paired removal and unrelated preservation.
- A3: delegated-token integration observes `delegated_write_refused` and zero diff.

## Implementation Notes

- 2026-08-25T18:08:00Z: Implemented transactional direct task-loop allow, hold, and clear policy mutation with dry-run parity, delegated refusal, and stale-preimage protection.
- 2026-08-25T18:08:12Z: verification pass id 7857aa6cebfdd4a52be5324fe8b6b31c previous none completion 31932a1fbcd3649c683c8caa8158c4b3
