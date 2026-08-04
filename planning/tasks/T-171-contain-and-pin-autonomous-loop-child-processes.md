---
id: T-171-contain-and-pin-autonomous-loop-child-processes
title: Contain and pin autonomous loop child processes
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-170-add-deterministic-autonomous-loop-preflight-and
updated_at: "2026-08-04T21:32:13Z"
---

# T-171-contain-and-pin-autonomous-loop-child-processes Contain and pin autonomous loop child processes

## Description

Add direct generic child execution with one invocation-pinned executable,
continuous loop lock ownership, delegated writer participation, exact prompt
delivery, and bounded process-tree cleanup.

## Acceptance

- Loop acquires the repository lock before semantic preflight snapshots and holds
  it continuously through every child, cleanup, postflight, between-iteration
  selection, and final release; only verified delegated child writers join.
- Before first child, stage/hash the executable no-replace once; every iteration
  uses it, conflicting inherited `TASKRAIL` is reported/refused, and child writers
  verify bytes plus token.
- Bare commands resolve through PATH; separator paths resolve against original
  cwd; argv goes directly to the OS with no shell, repository-root cwd, exact
  finite stdin/EOF, inherited environment except pinned identity, and faithful
  streams.
- Unix groups or preassigned Windows jobs exist before child code; inability
  refuses, there is no wall-clock timeout, and setsid/breakaway limits never
  become clean claims.
- Every exit/failure/signal drains, requests termination, waits exactly 10
  seconds, then force-terminates; escape/survivors fail, and staged bytes clean
  only after contained writers exit.

## Verification Notes

- Map criteria to portable helpers covering lock refusal after child exit/before
  postflight, path/argv/no-shell/stdin/streams, `TASKRAIL` conflict, delegation,
  multi-iteration mutation, no-timeout work, descendants, exact grace, escape,
  and cleanup.
- Run packaged native Linux, macOS, and Windows smoke checks rather than treating
  cross-build success as execution evidence.

## Implementation Notes
