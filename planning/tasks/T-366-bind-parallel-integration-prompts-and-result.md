---
id: T-366-bind-parallel-integration-prompts-and-result
title: Bind parallel integration prompts and result evidence
status: todo
priority: high
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies:
    - T-334-deliver-parallel-clone-batches-locally
    - T-335-deliver-parallel-batches-through-review-adapters
updated_at: "2026-08-26T09:03:46Z"
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
