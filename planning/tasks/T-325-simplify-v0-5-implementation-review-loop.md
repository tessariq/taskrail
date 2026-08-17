---
id: T-325-simplify-v0-5-implementation-review-loop
title: Simplify the v0.5 implementation review loop
status: completed
priority: high
spec_ref: specs/v0.5.0.md#task-implementation-prompt
dependencies: []
updated_at: "2026-08-17T10:32:45Z"
---

# T-325-simplify-v0-5-implementation-review-loop Simplify the v0.5 implementation review loop

## Description

Make deterministic verification the primary convergence mechanism for v0.5 task
implementation while retaining one focused independent review and every existing
mechanical lifecycle, policy, and fail-closed boundary.

## Acceptance

- The v0.5 contract defaults implementation review to one broad round with one
  fresh reviewer, while retaining the configured `1..2` range, three-reviewer
  ceiling, and conditional non-recursive final-diff capability.
- Simplification remains an explicit quality consideration without mandatory
  independent delegation. Additional reviewers and a second broad round require
  distinct task risks rather than finding repair alone.
- Review findings receive the existing dispositions and current-scope repairs are
  deterministically re-verified. Objective evidence may close a final-diff repair;
  judgment-heavy unresolved repairs remain `in_progress` with failing verification.
- Layout-2 migration candidates write the new default value `1`, while strict
  decoding continues accepting explicit values from `1` through `2`.
- Open tasks coupled to implementation review, migration, prompt rendering, and
  loop reporting encode the amended contract without rewriting completed history.
- The temporary source-checkout loop prompt, local guidance, and executable
  assertions dogfood the canonical amended workflow.

## Verification Notes

- Targeted Go tests prove generated migration candidates use `1` and explicit `2`
  remains valid.
- The autonomous-loop shell suite proves the prompt requires verify-first focused
  review, risk-justified expansion, deterministic repair closure, and fail-closed
  unresolved judgment.
- Full Go checks, Taskrail validation, task-body hygiene, and a sandboxed manual
  report cover repository-wide consistency and visible prompt behavior.

## Implementation Notes

- 2026-08-17T10:32:37Z: Aligned v0.5, future task contracts, migration defaults, and the temporary loop around one focused review with deterministic convergence.
- 2026-08-17T10:32:45Z: verification pass
