---
id: T-297-ship-complete-storage-neutral-prompt-rendering
title: Ship complete storage-neutral prompt rendering
status: todo
priority: high
spec_ref: specs/v0.5.0.md#workflow-prompt-catalog-and-overrides
dependencies:
    - T-250-render-prompts-from-storage-neutral-context
    - T-295-resolve-managed-prompt-subjects-through-active
    - T-296-authorize-transient-prompt-context-paths-safely
    - T-236-resolve-local-prompt-replacements-through-the
    - T-213-define-the-uniform-agent-machine-api
updated_at: "2026-08-08T14:23:08Z"
---

# T-297-ship-complete-storage-neutral-prompt-rendering Ship complete storage-neutral prompt rendering

## Description

Ship the complete public `prompt render` command by composing catalog resolution,
managed subject context, transient-path authorization, and strict substitution.

## Acceptance

- A1. Every v1 prompt ID accepts exactly its declared context flags, derives all
  required values, rejects undeclared or missing context, and applies the exact
  max-review-round rule for `task-implementation` only: repository default `1`
  with an explicit render override restricted to `1..2`.
- A2. Built-in and active committed/local replacements render through the pure
  strict renderer; invalid replacements never fall back and resolution is atomic
  with managed/transient context snapshots.
- A3. Text output is exact rendered content. JSON returns the common envelope with
  exact ID, contract, source, nullable replacement path, content, rendered digest,
  and pre-substitution template digest.
- A4. The command is read-only, provider-neutral, and storage-neutral: managed
  paths stay logical, only authorized proposal paths may expose a local physical
  prefix, and no proposal or durable artifact is created.

## Verification Notes

- A1-A3: a complete prompt/flag matrix, built-in/replacement fixtures, token/context
  mutations, and exact text/JSON hash goldens prove public behavior.
- A4: committed/local/non-Git fixtures with decoys and repository snapshots prove
  storage-neutral reads, the sole transient exception, and zero writes.

## Implementation Notes
