---
id: T-321-align-temporary-loop-with-the-maximum-review
title: Align temporary loop with the maximum review budget
status: completed
priority: high
spec_ref: specs/v0.5.0.md#source-checkout-bootstrap-loop-retirement
dependencies: []
updated_at: "2026-08-12T17:30:01Z"
---

# T-321-align-temporary-loop-with-the-maximum-review Align temporary loop with the maximum review budget

## Description

Align the temporary source-checkout loop's hard-coded correctness-review ceiling
with v0.5's supported maximum so late current-scope findings can be fixed and
reviewed autonomously more often without adding retries or weakening fail-closed
delivery.

## Acceptance

- A1. The temporary loop permits at most five correctness-review invocations,
  stops early on a clean review, and requires every material fix to be reviewed
  on frozen changed bytes while budget remains.
- A2. Required findings from reviews one through four are fixed in the current
  task; review five must be clean, otherwise the child records failing lifecycle
  evidence and stops without converting unfinished scope into a follow-up.
- A3. Temporary operator guidance and prompt-contract assertions describe the
  same bounded policy while preserving fresh delegation, timeout, Git ownership,
  and no-retry rules.
- A4. The autonomous-loop harness, shell syntax checks, repository tests, vet,
  formatting, Taskrail validation, skill parity, and task-body hygiene pass.

## Verification Notes

- Assert the rendered shared prompt names the five-review ceiling, reviews one
  through four as repairable, review five as terminal-clean, and budget
  exhaustion as failing rework rather than follow-up scope.
- Run `bash -n` over the temporary loop scripts and run
  `scripts/autonomous-loop/test.sh` plus the standard repository checks.

## Implementation Notes

- 2026-08-12T17:24:38Z: Aligned the temporary source-checkout prompt and operator contract with v0.5's bounded maximum of five correctness reviews; clean reviews stop early and exhausted current scope still fails closed.
- 2026-08-12T17:24:44Z: verification pass
- 2026-08-12T17:30:01Z: verification pass
