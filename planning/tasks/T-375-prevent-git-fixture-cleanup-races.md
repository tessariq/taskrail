---
id: T-375-prevent-git-fixture-cleanup-races
title: Prevent Git fixture cleanup races
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-359-make-containment-cleanup-timing-race-aware
updated_at: "2026-08-28T11:40:00Z"
completion_id: "f964ed643a3734d6c46bddbbd5a9b82c"
last_verification_id: "60149651c5057bb92cf2a24eb1df606e"
last_verification_result: pass
last_verified_at: "2026-08-28T11:40:00Z"
last_verified_completion_id: "f964ed643a3734d6c46bddbbd5a9b82c"
---

# T-375-prevent-git-fixture-cleanup-races Prevent Git fixture cleanup races

## Description

Keep temporary real-Git fixtures deterministic when the full suite exercises many
repositories concurrently. A completed assertion must not fail during `t.TempDir`
cleanup because Git starts detached repository maintenance after its foreground
command exits.

Follow-up derived from T-359-make-containment-cleanup-timing-race-aware's verification or discovery. This task owns the independently meaningful deferred outcome and any required integrated delivery.

## Acceptance

- Shared real-Git test fixtures disable automatic maintenance and garbage
  collection before creating commits, without changing production Git behavior.
- `TestLoopFrozenSelectionUsesPreflightTaskBytes` retains its frozen-selection
  assertions and no longer races fixture cleanup under repeated and full-suite
  execution.
- Linux, macOS, and native Windows CI remain green.

## Verification Notes

- Reproduce the observed failure from GitHub Actions run `33166602747`, job
  `98833539186`: assertions passed, then cleanup failed with `.git: directory not
  empty`.
- Run the focused test repeatedly with and without race instrumentation, then run
  formatting, vet, the full suite, and exact-head remote CI.

## Implementation Notes

- 2026-08-28T11:39:55Z: Disabled repository-local automatic Git maintenance before fixture commits and added an ordering-sensitive regression test; focused normal/race repetitions, full tests, and vet pass.
- 2026-08-28T11:40:00Z: verification pass id 60149651c5057bb92cf2a24eb1df606e previous none completion f964ed643a3734d6c46bddbbd5a9b82c
