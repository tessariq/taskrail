---
id: T-350-make-loop-launch-fixtures-portable
title: Make loop launch fixtures portable
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-309-launch-loop-children-with-exact-prompt-transport
updated_at: "2026-08-22T13:33:22Z"
completion_id: "36f3e20c593625f1e605dd66a71adf49"
last_verification_id: "5b8e14d3dbc2e70b41c3d50f356d5f98"
last_verification_result: pass
last_verified_at: "2026-08-22T13:33:22Z"
last_verified_completion_id: "36f3e20c593625f1e605dd66a71adf49"
---

# T-350-make-loop-launch-fixtures-portable Make loop launch fixtures portable

## Description

Keep loop-child cwd and separator-path launch fixtures portable across native
filesystem aliases and executable naming without weakening exact transport
assertions.

Follow-up derived from T-309-launch-loop-children-with-exact-prompt-transport's verification or discovery. This task owns the independently meaningful deferred outcome and any required integrated delivery.

## Acceptance

- Working-directory assertions compare filesystem identity so macOS's `/var` and
  `/private/var` aliases are accepted only when they identify the same directory.
- The separator-path helper preserves the native `.exe` suffix on Windows while
  still proving resolution against the caller's cwd before the repository cwd.
- Prompt, argv, environment, stream, process, and no-shell assertions remain
  unchanged and the full cross-platform suite passes.

## Verification Notes

- Run focused launch tests, vet, the full Go suite, validation, task-body checks,
  and exact-head native macOS and Windows CI.

## Implementation Notes

- 2026-08-22T13:33:22Z: Made loop launch cwd and executable-path fixtures portable across macOS and Windows.
- 2026-08-22T13:33:22Z: verification pass id 5b8e14d3dbc2e70b41c3d50f356d5f98 previous none completion 36f3e20c593625f1e605dd66a71adf49
