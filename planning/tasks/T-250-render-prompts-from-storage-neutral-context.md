---
id: T-250-render-prompts-from-storage-neutral-context
title: Render strict prompt templates
status: completed
priority: high
spec_ref: specs/v0.5.0.md#workflow-prompt-catalog-and-overrides
dependencies:
    - T-159-add-a-versioned-workflow-prompt-catalog
updated_at: "2026-08-21T22:04:09Z"
completion_id: "6c135fd12d646c4517a8a0103ae9361a"
last_verification_id: "bd568d7f8e590bc556a4f05a46850bb6"
last_verification_result: pass
last_verified_at: "2026-08-21T22:04:09Z"
last_verified_completion_id: "6c135fd12d646c4517a8a0103ae9361a"
---

# T-250-render-prompts-from-storage-neutral-context Render strict prompt templates

## Description

Implement the pure template-validation and one-pass substitution primitive used
by prompt commands, independent of repository discovery, storage, and CLI flags.

## Acceptance

- A1. The renderer accepts one UTF-8 template, its declared token set, and exact
  values; it rejects BOM, size violations, malformed, unknown, duplicate-policy,
  or unresolved token-shaped text before producing content.
- A2. Substitution is deterministic, non-recursive, and limited to exact
  `{{NAME}}` tokens with names matching the v1 grammar; values are inserted as
  text without shell, environment, include, escape, or action semantics.
- A3. Results expose exact template and rendered SHA-256 values, and failure
  returns no partial content or repository side effect.

## Verification Notes

- A1/A2: table fixtures cover every token grammar boundary, unknown/unresolved
  tokens, literal braces, recursive-looking values, UTF-8, BOM, and size limits.
- A3: exact-byte/hash goldens and immutable repository snapshots prove pure,
  deterministic rendering on all supported platforms.

## Implementation Notes

- 2026-08-21T22:02:26Z: Implemented strict pure v1 prompt rendering with deterministic hashes and one-pass substitution.
- 2026-08-21T22:04:09Z: verification pass id bd568d7f8e590bc556a4f05a46850bb6 previous none completion 6c135fd12d646c4517a8a0103ae9361a
