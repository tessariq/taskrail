---
id: T-381-restore-portable-verification-predecessor-evidence
title: Restore portable verification predecessor evidence
status: completed
priority: high
spec_ref: specs/v0.5.0.md#canonical-transition-order
dependencies: []
updated_at: "2026-08-29T10:14:37Z"
completion_id: "e98e5bdc258f4d301df9f06a9d0de64d"
last_verification_id: "bdad1a640a5440f3f09ce866a5923736"
last_verification_result: pass
last_verified_at: "2026-08-29T10:14:37Z"
last_verified_completion_id: "e98e5bdc258f4d301df9f06a9d0de64d"
---

# T-381-restore-portable-verification-predecessor-evidence Restore portable verification predecessor evidence

## Description

Make verification predecessor validation portable across fresh checkouts. A
newly written report may name a predecessor report that remains producer-local
and gitignored, so its absence must not invalidate otherwise canonical committed
task and state metadata. Available report evidence must continue to enforce the
same-task, predecessor, and cycle consistency checks.

## Acceptance

- Validation accepts a task whose latest locally available verification report
  names an absent predecessor artifact when its persisted tuple and latest
  report agree.
- Validation still rejects a missing latest report, a latest tuple/report
  mismatch, and any available malformed, cross-task, contradictory, or cyclic
  predecessor evidence.
- Regression coverage proves a fresh-clone-style partial artifact set remains
  valid without weakening available-evidence checks.

## Verification Notes

- Use focused verification-identity tests with deterministic report IDs for
  partial artifact sets and malformed predecessor chains.
- Run formatting, vet, targeted and full/race tests, repository validation, and
  task-body and skill parity checks.

## Implementation Notes

- 2026-08-29T10:14:26Z: Allow absent producer-local predecessor artifacts while retaining validation of available malformed, cross-task, and cyclic evidence.
- 2026-08-29T10:14:37Z: verification pass id bdad1a640a5440f3f09ce866a5923736 previous none completion e98e5bdc258f4d301df9f06a9d0de64d
