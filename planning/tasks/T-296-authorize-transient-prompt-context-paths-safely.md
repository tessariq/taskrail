---
id: T-296-authorize-transient-prompt-context-paths-safely
title: Authorize transient prompt context paths safely
status: todo
priority: high
spec_ref: specs/v0.5.0.md#workflow-prompt-catalog-and-overrides
dependencies:
    - T-250-render-prompts-from-storage-neutral-context
    - T-222-initialize-and-discover-ignored-local-taskrail
updated_at: "2026-08-08T14:23:08Z"
---

# T-296-authorize-transient-prompt-context-paths-safely Authorize transient prompt context paths safely

## Description

Validate the narrow physical-path exception for prompt proposal inputs and outputs
against the exact active transient artifacts directory.

## Acceptance

- A1. Only declared `REVIEW_PATH`, `DRAFT_PATH`, and `TRACE_PATH` context may use a
  physical path, and each canonical path must remain beneath the status-reported
  `artifacts_dir` and its role-specific proposal subtree.
- A2. In Git, authorization proves the selected path is effectively ignored,
  untracked, and unstaged and crosses no symlink/reparse, alias, collision, or
  special entry; non-Git authorization enforces containment without an ignore claim.
- A3. Managed state/spec/task/prompt/review roots, unrelated artifacts, outside
  paths, missing/changed ancestors, and undeclared transient flags are rejected.
- A4. Authorization is read-only and returns a snapshot suitable for final
  publication recheck; it never creates a proposal, directory, exclusion, or local
  storage context.

## Verification Notes

- A1-A3: committed/local/non-Git matrices cover every transient role plus ignored,
  tracked, staged, untracked, outside-root, alias, special-entry, and ancestor-swap
  cases.
- A4: before/after filesystem, index, and exclusion snapshots prove no mutation and
  identity goldens prove deterministic recheck inputs.

## Implementation Notes
