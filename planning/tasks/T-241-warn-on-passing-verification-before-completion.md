---
id: T-241-warn-on-passing-verification-before-completion
title: Warn on passing verification before completion
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#verify-lifecycle-advisory
dependencies:
    - T-286-bind-verification-to-completion-and-adopt-legacy
updated_at: "2026-08-22T09:20:46Z"
completion_id: "b900b4d516a69cffdd085536c0bf82ff"
last_verification_id: "a3ceaaa8c2b338ff102285d265067043"
last_verification_result: pass
last_verified_at: "2026-08-22T09:20:46Z"
last_verified_completion_id: "b900b4d516a69cffdd085536c0bf82ff"
---

# T-241-warn-on-passing-verification-before-completion Warn on passing verification before completion

## Description

Add the exact advisory for passing verification before completion without turning
verification into a lifecycle transition or rejecting useful evidence.

## Acceptance

- A1. Pass on every non-completed status emits the exact human and schema-1 warning
  while writing the normal chained verification evidence and exiting zero.
- A2. Completed pass and every fail emit no order warning; status never changes.
- A3. Warning order composes deterministically with other envelope warnings.

## Verification Notes

- A1: status matrix compares artifacts, task/state fields, stderr, JSON, and exit.
- A2: completed/fail controls prove absence and lifecycle preservation.
- A3: combined warning fixtures verify canonical ordering and no command-local list.

## Implementation Notes

- 2026-08-22T09:20:33Z: Added pass-before-completion verification advisories across human and schema-1 outputs.
- 2026-08-22T09:20:46Z: verification pass id a3ceaaa8c2b338ff102285d265067043 previous none completion b900b4d516a69cffdd085536c0bf82ff
