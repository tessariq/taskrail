---
id: T-229-canonicalize-v0-5-lifecycle-and-task-identities
title: Canonicalize v0.5 lifecycle and task identities
status: completed
priority: high
spec_ref: specs/v0.5.0.md#canonical-transition-order
dependencies: []
updated_at: "2026-08-08T15:56:49Z"
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
- A3. The reusable contract, current documentation, and contract-lint fixtures
  reject complete-on-failure, verify-as-transition, fuzzy task identity, and any
  lifecycle branch not named by the v0.5 spec. T-173 owns final proof that every
  later prompt, skill, and command consumes the contract.

## Verification Notes

- A1: table-driven contract tests feed every branch and exact/bare/sluggified ID;
  expected observations are the named branch or a stable identity refusal.
- A2: validation fixtures exercise every legal table row and each partial/broken
  combination; evidence is unit-level validation output.
- A3: golden docs/registry checks demonstrate the reusable citation/lint mechanism;
  T-173 applies it to the complete shipped cross-surface inventory.

## Implementation Notes

- 2026-08-08T15:56:28Z: Implemented the canonical lifecycle, exact-ID, metadata-table, and predecessor-chain contract; go test ./..., go vet ./..., validation, parity/body checks, and sandbox manual evidence passed.
- 2026-08-08T15:56:49Z: verification pass
