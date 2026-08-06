---
id: T-241-warn-on-passing-verification-before-completion
title: Warn on passing verification before completion
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#verify-lifecycle-advisory
dependencies:
    - T-158-bind-completion-and-verification-with-stable
updated_at: "2026-08-06T13:46:30Z"
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
