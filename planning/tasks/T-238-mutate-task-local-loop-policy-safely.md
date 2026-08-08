---
id: T-238-mutate-task-local-loop-policy-safely
title: Mutate task-local loop policy safely
status: todo
priority: high
spec_ref: specs/v0.5.0.md#task-local-loop-policy
dependencies:
    - T-168-parse-and-validate-an-optional-autonomous-run
    - T-282-protect-inherited-task-mutation-writers
updated_at: "2026-08-06T13:46:30Z"
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
