---
id: T-378-close-implementation-review-disposition-vocabulary
title: Close implementation review disposition vocabulary
status: completed
priority: high
spec_ref: specs/v0.5.0.md#task-implementation-prompt
dependencies: []
updated_at: "2026-08-29T08:09:56Z"
completion_id: "b7491af40f22046cc38475141bc7f492"
last_verification_id: "76eb969ce68c30035395d5735515b7cc"
last_verification_result: pass
last_verified_at: "2026-08-29T08:09:56Z"
last_verified_completion_id: "b7491af40f22046cc38475141bc7f492"
---

# T-378-close-implementation-review-disposition-vocabulary Close implementation review disposition vocabulary

## Description

Keep implementation review on one closed four-state finding vocabulary by defining
non-actionable low-value observations as rejected with rationale rather than an
undocumented report-only disposition. This resolves final v0.5 finding CONS-002.

## Acceptance

- The spec, task-implementation prompt, and full-task skills use only `fix-now`,
  `separate-followup`, `blocked`, and `rejected` for review findings.
- Low-value non-actionable observations map to `rejected` with rationale; current
  acceptance or invariant findings cannot evade repair through that mapping.
- Behavioral contract tests reject invented disposition states and stale guidance.

## Verification Notes

- Run focused prompt/skill contract tests, package parity, the workflow contract,
  and the full Go suite.

## Implementation Notes

- 2026-08-29T08:09:39Z: Closed the implementation-review vocabulary across the v0.5 spec, prompt, and full-task skills; added stale/invented-state contract coverage.
- 2026-08-29T08:09:56Z: verification pass id 76eb969ce68c30035395d5735515b7cc previous none completion b7491af40f22046cc38475141bc7f492
