---
id: T-380-close-parallel-integration-publication-boundaries
title: Close parallel integration publication boundaries
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies: []
updated_at: "2026-08-28T16:13:47Z"
---

# T-380-close-parallel-integration-publication-boundaries Close parallel integration publication boundaries

## Description

Close the final parallel-local-delivery boundary by making aggregate validation
explicitly read-only over the exact integration commit and defining the guarded
fast-forward's interruption and recovery contract. This resolves final v0.5
findings ADV-001 and ADV-003 as one integrated publication-safety outcome.

## Acceptance

- Aggregate validation must leave the integration tree, index, refs, configuration,
  and worktree unchanged and must attest the exact commit later published.
- The spec and operator result define what interruption can leave during the final
  branch/index/worktree fast-forward and name a conservative recovery procedure;
  no all-or-none durable-transaction claim is implied unless mechanically enforced.
- Tests cover aggregate mutation refusal, exact-head binding, publication
  interruption evidence, and retry/refusal without overwriting source drift.

## Verification Notes

- Run focused parallel integration/publication tests with interruption injection,
  full tests, race, workflow contracts, and native cross-platform CI.

## Implementation Notes
