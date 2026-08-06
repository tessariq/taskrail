---
id: T-243-contain-autonomous-loop-process-trees
title: Contain autonomous loop process trees
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-171-contain-and-pin-autonomous-loop-child-processes
updated_at: "2026-08-06T13:46:30Z"
---

# T-243-contain-autonomous-loop-process-trees Contain autonomous loop process trees

## Description

Establish honest cross-platform containment and optional per-child timeout behavior
after direct child launch is available, without claiming control over undetectable
privileged escape.

## Acceptance

- A1. Unix process groups and Windows kill-on-close jobs are established before
  untrusted child code runs or launch refuses.
- A2. Leader exit, stream failure, signal, timeout, and interruption drain assigned
  descendants, request termination, wait ten seconds, then force survivors.
- A3. Detected escape/unassignable/surviving processes and timeout report exact
  process violations and `child_failed`; omission of timeout remains unlimited.

## Verification Notes

- A1: native Linux/macOS/Windows helpers prove containment exists before child action.
- A2: normal/signal/hang/descendant trees provide observable cleanup and timing evidence.
- A3: supported escape/survivor probes and dry-run/result goldens prove honest bounds.

## Implementation Notes
