---
id: T-440-bound-review-delivery-adapter-calls
title: Bound review-delivery adapter calls
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies: []
updated_at: "2026-10-31T11:39:14Z"
---

# T-440-bound-review-delivery-adapter-calls Bound review-delivery adapter calls

## Description

The spec now requires `--timeout` when executing `--delivery review` and caps
adapter stdout at 1 MiB, matching its "timeout-bounded" wording. Today
`--timeout` is optional for review delivery (`validateParallelInvocation`,
internal/taskrail/loop_preflight.go), so adapter calls can run without a
deadline, and adapter stdout is buffered in memory without a limit
(internal/taskrail/loop_review_adapter.go). Found in the 2026-10-31 critical
review of the loop family at 5672a13.

## Acceptance

- Executing `--delivery review` without `--timeout` is `invalid_arguments` before any repository, workspace, or adapter activity.
- Adapter stdout beyond 1 MiB fails that operation as `adapter_failed` without unbounded buffering.
- docs/loop.md states both limits.

## Verification Notes

- Smoke test for the missing-timeout refusal; unit test with an adapter that writes past the cap.

## Implementation Notes
