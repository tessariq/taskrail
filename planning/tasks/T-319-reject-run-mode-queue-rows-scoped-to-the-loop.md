---
id: T-319-reject-run-mode-queue-rows-scoped-to-the-loop
title: Reject run-mode queue rows scoped to the loop directory
status: todo
priority: high
spec_ref: specs/v0.5.0.md#source-checkout-bootstrap-loop-retirement
dependencies:
    - T-318-accept-inline-loop-follow-up-recommendations
updated_at: "2026-08-11T18:41:05Z"
---

# T-319-reject-run-mode-queue-rows-scoped-to-the-loop Reject run-mode queue rows scoped to the loop directory

## Description

Queue validation admits a run-mode row whose acceptance surface lies entirely under scripts/autonomous-loop/, which the delegated child may not edit. T-318 burnt a full agent attempt, a blocked status, and a pushed failing verification before that was discovered. Add a queue-time guard so such rows must be hold-operator, and cover it in the harness.

## Acceptance

- The follow-up issue described by verification is resolved.
- Verification evidence is updated.

## Verification Notes

- Re-run task-scoped verification after implementing the fix.
