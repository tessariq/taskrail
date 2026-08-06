---
id: T-244-publish-streamed-loop-results-out-of-band
title: Publish streamed loop results out of band
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-172-enforce-autonomous-loop-lifecycle-and-delivery
updated_at: "2026-08-06T13:46:30Z"
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
