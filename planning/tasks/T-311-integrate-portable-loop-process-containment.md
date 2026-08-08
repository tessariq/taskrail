---
id: T-311-integrate-portable-loop-process-containment
title: Integrate portable loop process containment
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-243-contain-autonomous-loop-process-trees
    - T-310-contain-loop-process-trees-on-windows
updated_at: "2026-08-08T14:23:09Z"
---

# T-311-integrate-portable-loop-process-containment Integrate portable loop process containment

## Description

Integrate the Unix and Windows containment implementations into one loop
execution boundary, apply the optional per-child timeout, and expose exact
portable process evidence and cleanup behavior to postflight diagnostics.

## Acceptance

- Before untrusted child code runs, the active platform establishes T-243 Unix
  process-group containment or T-310 Windows job containment; unsupported or
  failed containment refuses launch with portable process evidence.
- Omitted timeout means no Taskrail wall-clock deadline. A positive frozen timeout
  applies independently to each child; expiry initiates containment cleanup,
  records a timeout violation, and marks the execution failed without selecting
  another task.
- Leader exit, launch/stream failure, signal, timeout, and handled interruption use
  one portable drain/terminate/wait-ten-seconds/force protocol and do not return
  until all observable assigned descendants are gone or survivor evidence has
  been captured.
- Detected escape, unassignable process, cleanup failure, or survivor produces
  ordered exact process violations and required containment failure evidence.
  Normal cleanup produces no false violation, and diagnostics disclose the
  platform's privileged/undetectable escape limitation.
- Integration preserves T-309's exact argv/stdin/stream behavior and T-171's lock
  and staged executable until containment cleanup has completed.

## Verification Notes

- Shared contract tests run equivalent normal, signal, timeout, interruption,
  stream-failure, descendant, escape, assignment-failure, and survivor scenarios
  against native Unix and native Windows helpers and compare portable evidence.
- Timing evidence distinguishes unlimited omission, per-child expiry, graceful
  ten-second window, and force escalation without relying only on leader exit.
- End-to-end fixtures prove lock/staged cleanup follows descendant cleanup and
  that every containment failure prevents continuation while retaining exact
  child and process diagnostics.

## Implementation Notes
