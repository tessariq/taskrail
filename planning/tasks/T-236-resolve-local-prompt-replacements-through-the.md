---
id: T-236-resolve-local-prompt-replacements-through-the
title: Resolve local prompt replacements through the overlay
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#workflow-prompt-catalog-and-overrides
dependencies:
    - T-159-add-a-versioned-workflow-prompt-catalog
    - T-222-initialize-and-discover-ignored-local-taskrail
updated_at: "2026-08-06T13:46:30Z"
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
- A3. Local replacement bytes participate in prompt hashes, drift snapshots, and
  promotion while runtime/artifact paths do not.

## Verification Notes

- A1: catalog show/render fixtures compare committed and local logical results.
- A2: collision and no-follow matrices observe exact refusals and unchanged Git
  status.
- A3: hash/promotion integration proves exact bytes move to the committed logical
  destination and no physical prefix becomes durable.

## Implementation Notes
