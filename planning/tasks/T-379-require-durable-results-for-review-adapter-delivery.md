---
id: T-379-require-durable-results-for-review-adapter-delivery
title: Require durable results for review-adapter delivery
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies: []
updated_at: "2026-08-28T16:13:41Z"
---

# T-379-require-durable-results-for-review-adapter-delivery Require durable results for review-adapter delivery

## Description

Require review-adapter parallel delivery to select an absent durable result file
before any workspace or adapter activity. This resolves final v0.5 finding
GAPS-002 and makes remote side-effect outcomes inspectable after the foreground run.

## Acceptance

- `--delivery review` without `--result-file` fails preflight as
  `invalid_arguments` before adapter or workspace activity.
- Local delivery and sequential execution retain their existing optional
  result-file behavior.
- CLI help, README/command guidance, machine-contract prose, and tests agree on
  the review-delivery requirement and exact refusal boundary.

## Verification Notes

- Add CLI/service preflight regressions proving zero side effects, then run focused
  loop tests, workflow contracts, full tests, and native cross-compilation.

## Implementation Notes
