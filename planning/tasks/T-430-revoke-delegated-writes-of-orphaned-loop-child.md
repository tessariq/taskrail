---
id: T-430-revoke-delegated-writes-of-orphaned-loop-child
title: Revoke delegated writes of an orphaned loop child
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies: []
updated_at: "2026-10-31T11:39:10Z"
---

# T-430-revoke-delegated-writes-of-orphaned-loop-child Revoke delegated writes of an orphaned loop child

## Description

Design risk. On Unix, killing the coordinator with SIGKILL leaves the child
process group running (only `Setpgid` is set; there is no parent-death tie), and
the lock record with its delegation token stays valid until `lock clear`. The
orphaned child can still join the lock and record lifecycle transitions with no
coordinator to run postflight: in the review, `$TASKRAIL complete <id>` from the
orphan exited 0 and wrote `status: completed`. Windows kill-on-close jobs do not
have this gap. The spec already accepts that abrupt termination cannot guarantee
a result file; it does not say orphans keep write authority. Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- On Linux the child process group receives a parent-death signal or equivalent containment when the coordinator dies.
- A delegated writer refuses to join when the lock owner PID is provably dead on the same host.
- docs/loop.md documents the remaining platform limits (for example macOS and deliberate setsid escapes).

## Verification Notes

- Manual: kill -9 the coordinator and attempt a delegated `complete` from the child environment; expect a refusal.

## Implementation Notes
