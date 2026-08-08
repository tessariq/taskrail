---
id: T-314-integrate-loop-continuation-and-terminal
title: Integrate loop continuation and terminal diagnostics
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-313-validate-committed-and-local-loop-delivery-shapes
updated_at: "2026-08-08T14:23:09Z"
---

# T-314-integrate-loop-continuation-and-terminal Integrate loop continuation and terminal diagnostics

## Description

Integrate lifecycle derivation, mutation integrity, process containment, and Git
delivery into the complete sequential loop. Continue only after a delivered
`completed_pass`, apply no-work-before-limit ordering, and emit exact terminal
diagnostics and safe recovery guidance.

## Acceptance

- Only a zero-exit, valid, contained, integrity-clean, correctly delivered
  per-child `completed_pass` performs another read-only selection. Every other
  per-child outcome stops immediately, exits non-zero, and launches no further
  child.
- After `completed_pass`, selection against the frozen invocation policy returns
  successful `no_work` when no eligible allowance remains. If work remains and
  the completed-child count reached `--max-iterations`, it returns successful
  `iteration_limit` with that task; `no_work` takes precedence when the last child
  exhausted eligible work.
- The terminal `LoopDiagnostic` has the exact v0.5 fields, enums, nullability,
  ordering, iteration counting, remaining-task row, lifecycle and verification
  identities, Git/storage/review/execution/executable facts, mutation/process
  violations, `remote:not_checked`, and non-empty safe `next_action`.
- Initial no-work uses null iteration and zero completed iterations; launch failure
  has a last iteration but does not increment completed count; every launched
  child reaching termination/containment postflight increments it exactly once.
  Preflight refusal uses common error details and fabricates no iteration evidence.
- Execution success envelopes use only invocation outcomes `no_work` or
  `iteration_limit`; terminal post-child failures use their registered outcome and
  same complete diagnostic object. T-244 publication receives this object without
  schema drift, while streamed execution never claims JSON mode.
- Diagnostics prescribe manual inspection and existing lifecycle/verify/Git
  recovery only. The loop never retries, resets, rewrites failed work, commits,
  pushes, merges, or attests hooks, semantic review convergence, provider state,
  or remote delivery.

## Verification Notes

- End-to-end matrices cover every per-child outcome, initial and post-pass no-work,
  remaining work below/at the limit, no-work precedence, launch failure, timeout,
  interrupt, and multi-iteration success with an exact launch count.
- Golden human and envelope diagnostics cover every nested field, enum,
  nullability, ordering, counter, safe next action, and common-error versus
  post-child boundary, including the T-244 handoff.
- Sandbox workflows exercise committed/local pass, block, rework, partial
  complete, audit fail, child failure, invalid integrity/delivery, held-task
  bypass, and held-dependency isolation while proving no hidden recovery action.

## Implementation Notes
