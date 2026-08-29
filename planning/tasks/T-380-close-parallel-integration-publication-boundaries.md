---
id: T-380-close-parallel-integration-publication-boundaries
title: Close parallel integration publication boundaries
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies: []
updated_at: "2026-08-29T08:29:34Z"
completion_id: "b761f8d974df38e28c3787e8073775b5"
last_verification_id: "73e988170a9fcc4b0c138381703d6279"
last_verification_result: pass
last_verified_at: "2026-08-29T08:29:34Z"
last_verified_completion_id: "b761f8d974df38e28c3787e8073775b5"
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

- 2026-08-29T08:29:23Z: Bound aggregate publication to exact clean integration evidence; added drift/interruption recovery safeguards and tests.
- 2026-08-29T08:29:34Z: verification pass id 73e988170a9fcc4b0c138381703d6279 previous none completion b761f8d974df38e28c3787e8073775b5
