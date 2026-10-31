---
id: T-431-bound-loop-child-timeout-when-descendant-holds-output
title: Bound loop child timeout when a descendant holds output
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies: []
updated_at: "2026-10-31T11:39:11Z"
---

# T-431-bound-loop-child-timeout-when-descendant-holds-output Bound loop child timeout when a descendant holds output

## Description

Bug. `--timeout` does not bound a child whose detached descendant keeps the
stdout pipe open, and the coordinator busy-spins while waiting. With
`--timeout 2s` and child `sh -c 'setsid sleep 8 & exit 0'` the launch returned
after 8.0 s using 5.6 s of CPU; with a long-lived daemon instead of `sleep` it
never returns. The spec disclaims detecting a deliberate setsid escape, not
hanging on one.

Cause: in `launchLoopChild` (internal/taskrail/loop_launch.go) the select keeps
picking the closed context Done channel while the stream copy blocks, and
nothing closes the parent read ends or sets `cmd.WaitDelay` after the deadline. Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- After the deadline plus the specified 10 s termination grace, the coordinator closes its pipe read ends and returns even if a descendant still holds them.
- Waiting does not busy-loop.
- The outcome is `child_failed` with the timeout process violation.

## Verification Notes

- Unit test with the setsid grandchild, asserting elapsed time below timeout+grace+epsilon and bounded CPU.

## Implementation Notes
