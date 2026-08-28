---
id: T-378-close-implementation-review-disposition-vocabulary
title: Close implementation review disposition vocabulary
status: todo
priority: high
spec_ref: specs/v0.5.0.md#task-implementation-prompt
dependencies: []
updated_at: "2026-08-28T16:13:36Z"
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
