---
id: T-310-contain-loop-process-trees-on-windows
title: Contain loop process trees on Windows
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-309-launch-loop-children-with-exact-prompt-transport
updated_at: "2026-08-22T14:48:30Z"
completion_id: "95c12097e4c97511bb9e6b0cfb6c8df7"
last_verification_id: "3fbf610164921350f7ecc332e816951d"
last_verification_result: pass
last_verified_at: "2026-08-22T14:48:30Z"
last_verified_completion_id: "95c12097e4c97511bb9e6b0cfb6c8df7"
last_verification_previous_id: "3c6b13fc83d65a1d3e67f5972498a28a"
---

# T-310-contain-loop-process-trees-on-windows Contain loop process trees on Windows

## Description

Contain each loop child and its assignable descendants in a Windows
kill-on-close Job Object established before untrusted code runs. Provide bounded
termination and survivor evidence for portable integration without claiming
control over permitted or undetectable breakaway processes.

## Acceptance

- Native Windows launch creates a kill-on-close Job Object and assigns the child
  before its untrusted entry point can run; unsupported nesting, assignment, or
  setup refuses launch rather than running uncontained.
- Leader exit, launch/stream failure, signal-equivalent termination, timeout
  request, and operator interruption all drain assigned processes, request job
  termination, wait up to ten seconds, force remaining members, and close handles
  without losing survivor evidence.
- Cleanup deterministically reports normal drain, requested termination, forced
  termination, unassignable/breakaway detection where observable, handle/setup
  failures, and survivors. It never reports containment success while an assigned
  process is known alive.
- The Windows result states the honest boundary: allowed, privileged, or
  undetectable breakaway escape may evade control and is not certified as
  contained.

## Verification Notes

- Native Windows helpers prove assignment to the kill-on-close job occurs before
  first child action and refusal occurs for unsupported nested-job or assignment
  conditions.
- Normal exit, leader-first exit, nested descendants, hanging streams,
  interruption, ten-second escalation, forced cleanup, and supported breakaway/
  survivor probes inspect process and handle state after return.
- Platform-specific tests assert stable evidence consumed by T-311 and run in the
  native Windows CI lane, without treating Wine or Unix emulation as sufficient.

## Implementation Notes

- 2026-08-22T14:00:32Z: Added Windows Job Object containment with bounded cleanup evidence.
- 2026-08-22T14:00:44Z: verification pass id 61be6852244ff4475fe304bddde7f5bf previous none completion 95c12097e4c97511bb9e6b0cfb6c8df7
- 2026-08-22T14:28:00Z: Integrated the verified Windows implementation with the Unix containment evidence and lifecycle contract from T-243.
- 2026-08-22T14:36:07Z: verification pass id 3c6b13fc83d65a1d3e67f5972498a28a previous 61be6852244ff4475fe304bddde7f5bf completion 95c12097e4c97511bb9e6b0cfb6c8df7
- 2026-08-22T14:48:30Z: verification pass id 3fbf610164921350f7ecc332e816951d previous 3c6b13fc83d65a1d3e67f5972498a28a completion 95c12097e4c97511bb9e6b0cfb6c8df7
