---
id: T-424-accept-lifecycle-only-selected-task-writes
title: Accept lifecycle-only selected-task writes in loop postflight
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies: []
updated_at: "2026-10-31T14:34:19Z"
completion_id: "dbd15d70aadef1720687e2f1b6f6f7e5"
last_verification_id: "37390f58d4b4a64b2b120b51f2dcc9fb"
last_verification_result: pass
last_verified_at: "2026-10-31T14:34:19Z"
last_verified_completion_id: "dbd15d70aadef1720687e2f1b6f6f7e5"
---

# T-424-accept-lifecycle-only-selected-task-writes Accept lifecycle-only selected-task writes in loop postflight

## Description

Bug. The loop integrity oracle rejects a selected task that a child only
started, or started and completed without verifying, as
`selected_task_mutation` plus `integrity_violation`, so the run ends
`invalid_postflight`. The spec allows canonical lifecycle fields, timestamps,
blocker and Implementation Notes to change, and defines `completed_unverified`
and `no_progress` for exactly these cases; they are currently unreachable.

Cause: `validSelectedTaskMutation` (internal/taskrail/loop_integrity.go) builds
its expected bytes through `patchVerificationMetadata`, which always writes
`last_verification_id`, `last_verification_result` and `last_verified_at`
(internal/taskrail/lifecycle_tx.go, `verificationFieldPointer`) even when the
task has no verification. `start` and `complete` never write those empty fields.

Repro: child `sh -c 'cat >/dev/null; "$TASKRAIL" start <id>; exit 1'` gives
`mutation_violations: [selected_task_mutation, integrity_violation]`. Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- `validSelectedTaskMutation` accepts the bytes produced by `start`, `start`+`complete`, and `start`+`block` on an unverified task.
- A real change to title, priority, spec_ref, dependencies, loop policy, Description, Acceptance, or Verification Notes is still rejected.
- A loop child that starts, completes, and commits without verifying yields `completed_unverified`.

## Verification Notes

- Unit: compare bytes before and after `Service.Start` (and Complete/Block) through `validSelectedTaskMutation`.
- Integration: a helper child that completes without verify and commits; assert `completed_unverified`.

## Implementation Notes

- 2026-10-31T14:34:11Z: Accept canonical unverified lifecycle writes while preserving protected task contracts; public regression, full deterministic tests, sandbox CLI evidence, simplifier and independent integrity review passed.
- 2026-10-31T14:34:19Z: verification pass id 37390f58d4b4a64b2b120b51f2dcc9fb previous none completion dbd15d70aadef1720687e2f1b6f6f7e5
