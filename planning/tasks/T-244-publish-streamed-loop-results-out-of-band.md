---
id: T-244-publish-streamed-loop-results-out-of-band
title: Publish streamed loop results out of band
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-314-integrate-loop-continuation-and-terminal
updated_at: "2026-08-23T19:38:18Z"
completion_id: "cfd1fa2bb3bad4094729c6e1bd0c69aa"
last_verification_id: "898b8ae80438c190684995bc7a77ea3a"
last_verification_result: pass
last_verified_at: "2026-08-23T19:38:18Z"
last_verified_completion_id: "cfd1fa2bb3bad4094729c6e1bd0c69aa"
---

# T-244-publish-streamed-loop-results-out-of-band Publish streamed loop results out of band

## Description

Publish one exact terminal loop envelope outside the repository while child output
continues streaming and cannot be mixed with JSON stdout.

## Acceptance

- A1. Result destinations are preflighted outside worktree/Git/managed inputs with
  retained parent identity, absent target, and no-follow no-clobber publication.
- A2. Every handled terminal outcome attempts one schema-1 result/error document
  with complete execution, executable, Git, lifecycle, policy, and next-action data.
- A3. Publication failure never overwrites, never claims a result file, preserves
  repository cleanliness, and exits as `result_file_publish_failed`.

## Verification Notes

- A1: external-path, alias, parent-swap, existing, and forbidden-destination tests
  observe exact preflight classifications.
- A2: golden files cover no-work, iteration limit, every postflight stop, and interrupt.
- A3: final-create failure injection proves absent target and clean repository.

## Implementation Notes

- 2026-08-23T19:38:17Z: Published safe out-of-band loop result envelopes with no-clobber destination checks.
- 2026-08-23T19:38:18Z: verification pass id 898b8ae80438c190684995bc7a77ea3a previous none completion cfd1fa2bb3bad4094729c6e1bd0c69aa
