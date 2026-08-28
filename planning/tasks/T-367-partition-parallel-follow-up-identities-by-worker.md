---
id: T-367-partition-parallel-follow-up-identities-by-worker
title: Partition parallel follow-up identities by worker rank
status: completed
priority: high
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies:
    - T-318-accept-inline-loop-follow-up-recommendations
    - T-334-deliver-parallel-clone-batches-locally
updated_at: "2026-08-28T08:18:01Z"
completion_id: "6580ae07732f1470a9b0e60d13cc15d5"
last_verification_id: "306d06ac265820b9cd72aaa462dbbd68"
last_verification_result: pass
last_verified_at: "2026-08-28T08:18:01Z"
last_verified_completion_id: "6580ae07732f1470a9b0e60d13cc15d5"
---

# T-367-partition-parallel-follow-up-identities-by-worker Partition parallel follow-up identities by worker rank

## Description

Prevent independently valid parallel workers from allocating the same v0.5
follow-up task identity. Each worker receives a disjoint deterministic numeric
sequence while retaining the existing no-cap semantic follow-up policy.

## Acceptance

- The coordinator freezes the ledger maximum `M`, frontier width `W`, and
  one-based worker rank `R` into each delegated grant.
- A worker's zero-based `K`th follow-up receives `M + R + K*W`; slug derivation and
  all existing verification provenance rules remain unchanged.
- Delegated creation outside the granted sequence refuses without a write, and
  concurrent workers can each create multiple follow-ups without ID collision.
- Sequential and direct-operator task allocation remains byte-compatible.

## Verification Notes

- Exercise two- and four-worker batches with zero, one, and multiple follow-ups,
  invalid sequence attempts, integration ordering, and unchanged sequential IDs.
- Run focused delegation and parallel integration tests, full tests, race tests,
  vet, and cross-platform CI.

## Implementation Notes

- 2026-08-28T08:17:42Z: Authenticated worker-rank follow-up allocation from the frozen task ledger.
- 2026-08-28T08:18:01Z: verification pass id 306d06ac265820b9cd72aaa462dbbd68 previous none completion 6580ae07732f1470a9b0e60d13cc15d5
