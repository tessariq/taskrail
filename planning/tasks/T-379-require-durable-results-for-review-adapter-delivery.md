---
id: T-379-require-durable-results-for-review-adapter-delivery
title: Require durable results for review-adapter delivery
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies: []
updated_at: "2026-08-29T08:18:16Z"
completion_id: "9bec9442243143c92b3065b3166ea736"
last_verification_id: "fcf70c3f67c2e817975cf0cbf06d99f3"
last_verification_result: pass
last_verified_at: "2026-08-29T08:18:16Z"
last_verified_completion_id: "9bec9442243143c92b3065b3166ea736"
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

- 2026-08-29T08:18:02Z: Review delivery now requires a prepared external result file before repository, workspace, or adapter activity; focused, workflow-contract, full-suite, vet, formatting, validation, and cross-build checks passed.
- 2026-08-29T08:18:16Z: verification pass id fcf70c3f67c2e817975cf0cbf06d99f3 previous none completion 9bec9442243143c92b3065b3166ea736
