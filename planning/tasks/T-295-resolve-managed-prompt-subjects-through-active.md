---
id: T-295-resolve-managed-prompt-subjects-through-active
title: Resolve managed prompt subjects through active storage
status: todo
priority: high
spec_ref: specs/v0.5.0.md#workflow-prompt-catalog-and-overrides
dependencies:
    - T-250-render-prompts-from-storage-neutral-context
    - T-235-show-a-task-by-exact-id-through-active-storage
    - T-294-show-durable-review-artifacts-through-active
updated_at: "2026-08-08T14:23:08Z"
---

# T-295-resolve-managed-prompt-subjects-through-active Resolve managed prompt subjects through active storage

## Description

Resolve task, spec, and durable-review prompt context through active-storage
commands while rendering only canonical logical managed paths.

## Acceptance

- A1. Task-derived prompt context resolves one exact task through `task show`,
  derives and validates its referenced spec, and supplies exact task/spec IDs,
  versions, logical paths, and storage mode required by the prompt declaration.
- A2. Explicit spec context accepts only a discoverable version or its matching
  configured versioned path; durable spec-review and memory inputs are read only
  through `review show`.
- A3. Committed and local modes produce equivalent logical managed placeholders;
  decoy logical files cannot substitute for active-storage bytes and no physical
  overlay prefix is rendered for managed subjects.
- A4. Missing, ambiguous, malformed, off-contract, or incompatible managed context
  fails before template rendering and leaves the repository unchanged.

## Verification Notes

- A1/A2: task-derived and explicit-spec matrices cover valid and invalid IDs,
  references, versions, review inputs, and per-prompt required context.
- A3/A4: committed/local custom-directory fixtures with decoy logical files prove
  command-mediated reads, logical-path parity, no overlay disclosure, and no writes.

## Implementation Notes
