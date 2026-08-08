---
id: T-255-bind-review-artifacts-to-resolved-prompt-templates
title: Add the prompt-binding publication hook
status: todo
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-215-add-the-generic-review-artifact-publisher
    - T-276-integrate-contextual-review-schema-validation
    - T-297-ship-complete-storage-neutral-prompt-rendering
updated_at: "2026-08-07T20:40:30Z"
---

# T-255-bind-review-artifacts-to-resolved-prompt-templates Add the prompt-binding publication hook

## Description

Add a reusable publisher hook that validates and snapshots the role-mandated
prompt resolution for one prompt-produced observation. Type adapters remain
separate outcomes.

## Acceptance

- A1. A publication adapter registers an artifact role and required prompt
  ID/contract; the hook validates the four binding fields and resolves that exact
  template through active storage.
- A2. Preview snapshots source class, template bytes, configuration bytes, and
  identities; apply rechecks them under lock immediately before publication and
  classifies malformed binding, invalid replacement, and drift with the specified
  error precedence.
- A3. Built-ins and equal-byte replacements remain distinct by source, no physical
  replacement path becomes durable, and manifests/indexes that are not prompt
  observations cannot register the hook.
- A4. The reusable result claims only publication-time resolution, never prompt
  delivery, reviewer identity/independence, provider use, or semantic quality.

## Verification Notes

- A1-A3: hook contract fixtures cover registered/unregistered roles, built-in and
  committed/local replacements, wrong fields, invalid replacements, equal bytes,
  and source/config/template races with no-publication snapshots.
- A4: API/help wording assertions reject certification claims; adapter tests in
  T-298 through T-300 and T-305 provide end-to-end publication evidence.

## Implementation Notes
