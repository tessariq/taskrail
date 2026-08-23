---
id: T-356-stabilize-dead-process-fixtures-against-pid-reuse
title: Stabilize dead process fixtures against PID reuse
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-355-ignore-transient-git-locks-in-cli-snapshots
updated_at: "2026-08-23T20:00:45Z"
completion_id: "6889722e2677359d944e0a4f98435eae"
last_verification_id: "fb57720fda0a2cc74907f9bc0d99e0f9"
last_verification_result: pass
last_verified_at: "2026-08-23T20:00:45Z"
last_verified_completion_id: "6889722e2677359d944e0a4f98435eae"
---

# T-356-stabilize-dead-process-fixtures-against-pid-reuse Stabilize dead process fixtures against PID reuse

## Description

Follow-up derived from T-355-ignore-transient-git-locks-in-cli-snapshots's verification or discovery. This task owns the independently meaningful deferred outcome and any required integrated delivery.

Make repository-lock tests use a reliably absent process ID rather than a just-exited child PID that Windows may recycle before the liveness probe. This keeps the native filesystem portability gate deterministic while preserving the real same-host live-owner behavior required by the cross-platform autonomous loop.

## Acceptance

- Dead-owner fixtures cannot become live through normal PID reuse between setup and `Clear`.
- The live same-host owner test still exercises the native process-liveness probe.
- The repository-lock tests and dedicated native filesystem portability suite pass.

## Verification Notes

- Run the focused repository-lock package tests repeatedly, then run the dedicated native filesystem portability command and standard repository gates.

## Implementation Notes

- 2026-08-23T20:00:37Z: verification pass id d2006eb9d85ce368bba7f7f3cd2d7953 previous none completion none
- 2026-08-23T20:00:41Z: Replaced the just-exited child PID fixture with a high sentinel checked by the native liveness probe, eliminating Windows PID reuse races while retaining real live-owner coverage.
- 2026-08-23T20:00:45Z: verification pass id fb57720fda0a2cc74907f9bc0d99e0f9 previous none completion 6889722e2677359d944e0a4f98435eae
