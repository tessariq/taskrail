---
id: T-366-bind-parallel-integration-prompts-and-result
title: Bind parallel integration prompts and result evidence
status: completed
priority: high
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies:
    - T-334-deliver-parallel-clone-batches-locally
    - T-335-deliver-parallel-batches-through-review-adapters
updated_at: "2026-08-26T10:56:38Z"
completion_id: "6e4e818f852fe133da32c85e178b708e"
last_verification_id: "7f863131a5932ce7d36ba8619d1551bb"
last_verification_result: pass
last_verified_at: "2026-08-26T10:56:38Z"
last_verified_completion_id: "6e4e818f852fe133da32c85e178b708e"
---

# T-366-bind-parallel-integration-prompts-and-result Bind parallel integration prompts and result evidence

## Description

Make every parallel conflict-resolution and aggregate integration child consume
one versioned coordinator-owned prompt and produce auditable machine evidence
bound to the exact worker candidate and integration head it assessed.

## Acceptance

- The built-in-only `loop-integration` v1 prompt is listed and inspectable, resolves
  replacements with exact digest authorization, and rejects direct public render.
- Conflict and aggregate children receive only their role-appropriate validated
  coordinator context and freeze template/rendered digests before launch.
- Parallel result files emit ordered `ParallelIntegrationChild` records with the
  exact role, nullable task/candidate/evidence fields, bound head, prompt binding,
  child outcome, and affected checks; stale or mismatched bindings fail closed.
- Local and review-adapter delivery share the same integration evidence contract.

## Verification Notes

- Cover built-in/replacement resolution, forbidden direct render, conflict and
  aggregate success/failure, stale heads/evidence, deterministic ordering, strict
  decoding, and both delivery modes.
- Run prompt goldens, loop integration tests, full tests, race tests, parity, vet,
  and cross-platform CI.

## Implementation Notes

- 2026-08-26T10:56:17Z: Bound coordinator-owned parallel integration prompts and ordered child evidence across local and review delivery.
- 2026-08-26T10:56:38Z: verification pass id 7f863131a5932ce7d36ba8619d1551bb previous none completion 6e4e818f852fe133da32c85e178b708e
