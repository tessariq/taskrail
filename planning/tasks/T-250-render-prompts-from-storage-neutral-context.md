---
id: T-250-render-prompts-from-storage-neutral-context
title: Render prompts from storage-neutral context
status: todo
priority: high
spec_ref: specs/v0.5.0.md#workflow-prompt-catalog-and-overrides
dependencies:
    - T-159-add-a-versioned-workflow-prompt-catalog
    - T-215-add-the-generic-review-artifact-publisher
    - T-235-show-a-task-by-exact-id-through-active-storage
    - T-236-resolve-local-prompt-replacements-through-the
updated_at: "2026-08-06T13:46:30Z"
---

# T-250-render-prompts-from-storage-neutral-context Render prompts from storage-neutral context

## Description

Implement strict one-pass prompt rendering through subject commands and logical
review paths after catalog, task inspection, publication, and local replacement
resolution exist.

## Acceptance

- A1. Each prompt ID accepts exactly its declared context flags/placeholders,
  derives task/spec context, and rejects unknown/unresolved/non-recursive tokens.
- A2. Managed task/spec/review bytes are read through `task show`, `spec show`, and
  `review show`; only declared transient output paths may expose the local overlay.
- A3. Built-in and replacement content/hash/source results are exact and read-only;
  invalid replacements never fall back.

## Verification Notes

- A1: per-prompt positive/negative context matrices and token mutation fixtures.
- A2: committed/local parity tests use stale decoy logical paths to prove subject
  commands, not direct filesystem opens, supply context.
- A3: exact text/JSON hash goldens plus repository snapshots prove read-only behavior.

## Implementation Notes
