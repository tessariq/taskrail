---
id: T-243-contain-autonomous-loop-process-trees
title: Contain autonomous loop process trees on Unix
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-309-launch-loop-children-with-exact-prompt-transport
updated_at: "2026-08-22T14:10:40Z"
completion_id: "18f027d6c3240b7141c29816228bd3d1"
last_verification_id: "885e75ae436e9dfe0a87d1d869a51b5b"
last_verification_result: pass
last_verified_at: "2026-08-22T14:10:40Z"
last_verified_completion_id: "18f027d6c3240b7141c29816228bd3d1"
---

# T-243-contain-autonomous-loop-process-trees Contain autonomous loop process trees on Unix

## Description

Contain each loop child and its detectable descendants in an isolated Unix
process group established before untrusted code runs. Provide bounded termination
and survivor evidence for the later portable integration without claiming control
over privileged or undetectable `setsid` escape.

## Acceptance

- On supported Unix platforms, launch establishes a new process group before the
  child can execute untrusted code; inability to establish or verify that group
  refuses launch.
- Leader exit, launch/stream failure, signal, timeout request, and operator
  interruption all enter cleanup: drain assigned descendants, request group
  termination, wait up to ten seconds, force remaining members, and reap what the
  platform exposes.
- Cleanup returns deterministic evidence for normal drain, requested termination,
  forced termination, detected escape/unassignable members, and survivors. It
  never reports containment success while an assigned process is known alive.
- The Unix result states the honest boundary: deliberate privileged or
  undetectable session escape may evade observation and is not certified as
  contained.

## Verification Notes

- Native Linux and macOS helpers prove the process group exists before the first
  child action and launch refusal occurs when setup cannot be guaranteed.
- Normal exit, leader-first exit, signal, hanging streams, nested descendants,
  ten-second escalation, interruption, and supported escape/survivor probes
  record PIDs/groups and demonstrate no known assigned process survives return.
- Platform-specific tests assert stable evidence consumed by T-311 without
  asserting control over undetectable or privileged escape.

## Implementation Notes

- 2026-08-22T14:10:25Z: Contain Unix loop process groups with bounded cleanup and process evidence.
- 2026-08-22T14:10:40Z: verification pass id 885e75ae436e9dfe0a87d1d869a51b5b previous none completion 18f027d6c3240b7141c29816228bd3d1
