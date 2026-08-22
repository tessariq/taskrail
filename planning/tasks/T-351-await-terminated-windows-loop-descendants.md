---
id: T-351-await-terminated-windows-loop-descendants
title: Await terminated Windows loop descendants
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-310-contain-loop-process-trees-on-windows
updated_at: "2026-08-22T14:58:23Z"
completion_id: "e7e8f8dcc5e513ceb1e0fbb7f01fa940"
last_verification_id: "43179086b757e08bbdf879002ec87605"
last_verification_result: pass
last_verified_at: "2026-08-22T14:58:23Z"
last_verified_completion_id: "e7e8f8dcc5e513ceb1e0fbb7f01fa940"
---

# T-351-await-terminated-windows-loop-descendants Await terminated Windows loop descendants

## Description

Do not return from Windows Job Object cleanup merely because membership reaches
zero; await the captured member process handles so all known descendants have
actually exited.

Follow-up derived from T-310-contain-loop-process-trees-on-windows's verification or discovery. This task owns the independently meaningful deferred outcome and any required integrated delivery.

## Acceptance

- Cleanup captures waitable handles for observable job members before requesting
  termination and closes every captured handle before return.
- Requested and forced termination each wait within the existing ten-second
  bound for both zero job membership and signaled member process handles.
- Cleanup reports every still-unsignaled or still-assigned process as a survivor.
- Native Windows descendant and cancellation tests pass without weakening Unix
  containment behavior.

## Verification Notes

- Run Windows cross-compilation, focused Unix containment tests, the full local
  suite, vet, validation, task-body checks, and exact-head native Windows CI.

## Implementation Notes

- 2026-08-22T14:58:23Z: Captured Windows job member handles before termination and awaited actual process exit before returning containment evidence.
- 2026-08-22T14:58:23Z: verification pass id 43179086b757e08bbdf879002ec87605 previous none completion e7e8f8dcc5e513ceb1e0fbb7f01fa940
