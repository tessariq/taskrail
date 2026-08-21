---
id: T-236-resolve-local-prompt-replacements-through-the
title: Resolve local prompt replacements through the overlay
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#workflow-prompt-catalog-and-overrides
dependencies:
    - T-159-add-a-versioned-workflow-prompt-catalog
    - T-222-initialize-and-discover-ignored-local-taskrail
updated_at: "2026-08-21T13:34:32Z"
completion_id: "6e77f928649d64055b64a31fc8a90d72"
last_verification_id: "1c050c74df98ed32a882bc9580c71e14"
last_verification_result: pass
last_verified_at: "2026-08-21T13:34:32Z"
last_verified_completion_id: "6e77f928649d64055b64a31fc8a90d72"
---

# T-236-resolve-local-prompt-replacements-through-the Resolve local prompt replacements through the overlay

## Description

Map logical prompt replacements into ignored local storage without changing the
catalog's public path, validation, authorization, or whole-file replacement model.

## Acceptance

- A1. Local mode resolves logical `.taskrail/prompts/v1/...` replacements from the
  managed overlay and reports only the logical path.
- A2. A simultaneous committed replacement, alias, tracked/staged entry, invalid
  UTF-8, oversize file, or physical-path substitution fails without fallback.
- A3. Local replacement bytes determine the ordinary exact template hash while
  source class remains separate metadata; both participate in drift snapshots and
  expose exact logical inputs for later promotion while runtime/artifact paths do not.

## Verification Notes

- A1: catalog show/render fixtures compare committed and local logical results.
- A2: collision and no-follow matrices observe exact refusals and unchanged Git
  status.
- A3: hash fixtures prove equal-byte built-in/replacement resolutions retain equal
  hashes but distinct source classes and expose no physical prefix. T-224 owns the
  later byte-preserving promotion; T-255 owns publication-time replacement races.

## Implementation Notes

- 2026-08-21T13:34:18Z: Resolved local prompt replacements through the ignored overlay with logical-path reporting and fail-closed source checks.
- 2026-08-21T13:34:32Z: verification pass id 1c050c74df98ed32a882bc9580c71e14 previous none completion 6e77f928649d64055b64a31fc8a90d72
