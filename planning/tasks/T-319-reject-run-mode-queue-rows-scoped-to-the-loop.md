---
id: T-319-reject-run-mode-queue-rows-scoped-to-the-loop
title: Reject run-mode queue rows scoped to the loop directory
status: completed
priority: high
spec_ref: specs/v0.5.0.md#source-checkout-bootstrap-loop-retirement
dependencies:
    - T-318-accept-inline-loop-follow-up-recommendations
updated_at: "2026-08-11T19:01:23Z"
---

# T-319-reject-run-mode-queue-rows-scoped-to-the-loop Reject run-mode queue rows scoped to the loop directory

## Description

Queue validation admits a run-mode row whose acceptance surface lies entirely under scripts/autonomous-loop/, which the delegated child may not edit. T-318 burnt a full agent attempt, a blocked status, and a pushed failing verification before that was discovered. Add a queue-time guard so such rows must be hold-operator, and cover it in the harness.

## Acceptance

- A1. Queue validation rejects a `run` row whose open task file references
  `scripts/autonomous-loop`, exits before any agent launches, and names
  `hold-operator` as the remedy.
- A2. A `hold-operator` row referencing the same path stays valid, and completed
  or cancelled tasks are exempt so historical `run` rows keep validating.
- A3. The guard is mechanical: it matches the literal loop path in the task file
  and infers nothing about the task's meaning.
- A4. The harness covers rejection and both exemptions, and the existing suite
  stays green.

## Verification Notes

- Run `scripts/autonomous-loop/test.sh` and `bash -n`; the repository's own queue
  is evidence for A2, since its only loop-referencing `run` rows (T-257, T-318)
  are completed.

## Implementation Notes

- 2026-08-11T19:01:22Z: Queue validation now rejects run rows scoped to the loop directory before any agent launches; completed and cancelled tasks stay exempt.
- 2026-08-11T19:01:23Z: verification pass
