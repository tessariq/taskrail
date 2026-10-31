---
id: T-428-retain-integrated-work-when-publication-refuses
title: Retain integrated work when parallel publication refuses
status: completed
priority: high
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies: []
updated_at: "2026-10-31T16:29:24Z"
completion_id: "a266206345ef2ad542a0dd70c893f178"
last_verification_id: "d37bde54e62ab7f5b7b24d0017749361"
last_verification_result: pass
last_verified_at: "2026-10-31T16:29:24Z"
last_verified_completion_id: "a266206345ef2ad542a0dd70c893f178"
---

# T-428-retain-integrated-work-when-publication-refuses Retain integrated work when parallel publication refuses

## Description

Bug. If the source drifts after the aggregate gate passes, publication is
refused (`batch_failed`, `source_drift`), but under the default
`--keep-workspaces failure` every worker and integration workspace is deleted
and `pending_tasks` is empty. Completed, integrated work is lost and the report
lists those tasks as neither published nor pending.

Cause: workers are marked `Integrated` before the aggregate step and
publication; cleanup retains only non-integrated workers and removes the
integration clone whenever `AggregatePass` is true; `PendingTasks` collects only
non-integrated workers (internal/taskrail/loop_parallel.go). Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- An aggregate-passed but unpublished integration counts as a failure for retention: under `failure` and `always` the integration workspace survives and its path is reported.
- Tasks integrated but not published appear in `parallel.delivery.pending_tasks`.
- `never` retention still removes everything, and the next action names the lost-work risk.

## Verification Notes

- Integration test using the publish-after-fetch hook to inject drift; assert retained integration path and pending tasks.

## Implementation Notes

- 2026-10-31T16:29:09Z: Retain aggregate-passed unpublished integration until publication is proven; report unpublished tasks pending and warn on never retention. Focused red/green regression, full tests, vet, simplification, independent high-risk review and fifteen disposable acceptance scenarios passed.
- 2026-10-31T16:29:24Z: verification pass id d37bde54e62ab7f5b7b24d0017749361 previous none completion a266206345ef2ad542a0dd70c893f178
