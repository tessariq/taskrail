---
id: T-145-reconcile-coverage-gap-gating-with-the-advisory
title: Reconcile coverage gap gating with the advisory spec contract
status: completed
priority: high
spec_ref: specs/v0.4.0.md#gap-analysis
dependencies:
    - T-100
updated_at: "2026-07-29T16:24:31Z"
---

# T-145-reconcile-coverage-gap-gating-with-the-advisory Reconcile coverage gap gating with the advisory spec contract

## Description

The normative v0.4.0 Gap Analysis area calls structural signals advisory and
report-only, while `coverage --gaps --fail-on` can gate CI. Resolve this scope drift
explicitly rather than shipping behavior that silently outgrows the spec.

## Acceptance

- Decide whether opt-in exit-code gating is compatible with the intended v0.4.0
  advisory contract; record the decision in the normative spec before changing
  behavior.
- If retained, make the spec, README, packaged `taskrail-gap` guidance, help, and
  changelog consistently distinguish default advisory output from explicit gating.
- If deferred/removed, remove the flag and its user-facing documentation without
  changing default read-only gap reporting.
- Tests enforce the selected contract and `validate` remains unaffected.

## Verification Notes

- T-140 semantic review found the implementation and changelog expose a gate not
  named by the source-of-truth spec.

## Implementation Notes

- This task requires maintainer approval before changing the spec contract.
- 2026-07-29T16:24:07Z: verification pass
