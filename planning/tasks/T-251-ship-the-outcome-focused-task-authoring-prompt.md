---
id: T-251-ship-the-outcome-focused-task-authoring-prompt
title: Ship the outcome-focused task-authoring prompt
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#outcome-focused-task-authoring
dependencies:
    - T-250-render-prompts-from-storage-neutral-context
updated_at: "2026-08-22T10:20:26Z"
completion_id: "e60670d3d75104000add18be8fc95783"
last_verification_id: "26a7ec1f186888f6f2ec86f96622f715"
last_verification_result: pass
last_verified_at: "2026-08-22T10:20:26Z"
last_verified_completion_id: "e60670d3d75104000add18be8fc95783"
---

# T-251-ship-the-outcome-focused-task-authoring-prompt Ship the outcome-focused task-authoring prompt

## Description

Ship the read-only authoring guidance and reusable body-quality contract separately
from the later CAS writer that applies a reviewed proposal.

## Acceptance

- A1. The prompt owns the semantic sizing rubric: require one independently
  meaningful outcome and invariant, observable actor/precondition/success/failure
  boundaries, and explicit integration ownership; split bundled outcomes and
  merge fragments that are not independently valuable, without using file count,
  criterion count, or implementation layers as size proxies.
- A2. Every criterion maps setup/action/expected observation to the cheapest
  sufficient evidence and a public or durable oracle, with regression proof where relevant.
- A3. It rejects non-todo authoring, vague suite-pass evidence, unnecessary internal
  prescription, speculative checklist scope, and direct task mutation.

## Verification Notes

- A1/A2: aligned, oversized, fragmented, integration-owner, shallow-oracle,
  boundary, and regression fixtures receive expected rubric decisions in prompt
  contract tests, including rejection of mechanical size proxies.
- A3: non-todo and over-prescribed proposals demonstrate refusal while prompt
  rendering remains read-only.

## Implementation Notes

- 2026-08-22T10:20:16Z: Ship the read-only outcome-focused task-authoring prompt and contract fixtures.
- 2026-08-22T10:20:26Z: verification pass id 26a7ec1f186888f6f2ec86f96622f715 previous none completion e60670d3d75104000add18be8fc95783
