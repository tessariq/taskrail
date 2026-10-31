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

Spec gap. The spec calls review delivery timeout-bounded, but `--timeout` is
optional for `--delivery review` (`validateParallelInvocation`,
internal/taskrail/loop_preflight.go), so adapter calls run without a deadline
when it is omitted, and adapter stdout is buffered in memory without a limit
(internal/taskrail/loop_review_adapter.go). Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- Either executing `--delivery review` requires `--timeout` (`invalid_arguments` otherwise) or the spec drops the timeout-bounded wording; the choice is recorded in the spec.
- Adapter stdout is capped at a documented size, and exceeding it yields `adapter_failed`.

## Verification Notes

- Smoke test for the missing-timeout refusal; unit test with an adapter that writes past the cap.

## Implementation Notes
