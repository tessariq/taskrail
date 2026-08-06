---
id: T-171-contain-and-pin-autonomous-loop-child-processes
title: Launch and pin autonomous loop child processes
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-170-add-deterministic-autonomous-loop-preflight-and
updated_at: "2026-08-04T21:32:13Z"
---

# T-171-contain-and-pin-autonomous-loop-child-processes Launch and pin autonomous loop child processes

## Description

Add direct generic child execution with one invocation-pinned executable,
continuous loop lock ownership, exact delegation environment, and exact prompt
delivery. Process-tree containment and timeout cleanup are owned by T-243.

## Acceptance

- Loop acquires the repository lock before semantic preflight snapshots and holds
  it continuously through every child, cleanup, postflight, between-iteration
  selection, and final release; only verified delegated child writers join.
- Delegation grants only selected-task lifecycle and verification-created
  implicit-hold follow-up write sets. Task-local loop-policy commands and field mutations always
  refuse delegated ownership, including attempts concerning the selected task.
- Before first child, stage/hash the executable no-replace once; every iteration
  uses it, conflicting inherited delegation variables refuse, and child writers
  verify exact `TASKRAIL`, executable digest, invocation ID, and token.
- Every child inherits the frozen storage mode/root and effective implementation
  review maximum; delegated writers refuse mode/root mismatch, and a child cannot
  mutate repository review policy or widen its own review budget.
- Bare commands resolve through PATH; separator paths resolve against original
  cwd; argv goes directly to the OS with no shell, repository-root cwd, exact
  finite stdin/EOF, inherited environment except pinned identity, and faithful
  streams.

## Verification Notes

- Map criteria to portable helpers covering lock refusal after child exit/before
  postflight, path/argv/no-shell/stdin/streams, `TASKRAIL` conflict, delegation,
  delegated loop-policy refusal, multi-iteration mutation, and staged cleanup
  after delegated writers exit.

## Implementation Notes
