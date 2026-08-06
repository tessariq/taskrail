---
id: T-229-canonicalize-v0-5-lifecycle-and-task-identities
title: Canonicalize v0.5 lifecycle and task identities
status: todo
priority: high
spec_ref: specs/v0.5.0.md#canonical-transition-order
dependencies: []
updated_at: "2026-08-06T13:46:30Z"
---

# T-229-canonicalize-v0-5-lifecycle-and-task-identities Canonicalize v0.5 lifecycle and task identities

## Description

Establish the shared v0.5 lifecycle vocabulary and exact-full-ID contract used by
commands, prompts, skills, verification, and loop postflight before those surfaces
implement their own behavior.

## Acceptance

- A1. One executable contract represents the three canonical lifecycle branches,
  exact-full-ID operands/results, terminal-run meaning, and direct-operator versus
  delegated capabilities without introducing stable `task_ref` semantics.
- A2. The legal completion/verification metadata table and predecessor-chain
  invariants are represented once and consumed by validation and later writers.
- A3. Documentation and fixtures reject complete-on-failure, verify-as-transition,
  fuzzy task identity, and any lifecycle branch not named by the v0.5 spec.

## Verification Notes

- A1: table-driven contract tests feed every branch and exact/bare/sluggified ID;
  expected observations are the named branch or a stable identity refusal.
- A2: validation fixtures exercise every legal table row and each partial/broken
  combination; evidence is unit-level validation output.
- A3: golden docs/registry checks demonstrate that prompts and later command tests
  cite the shared contract rather than restating a divergent order.

## Implementation Notes
