---
id: T-299-bind-spec-review-publication-to-resolved-prompts
title: Bind spec review publication to resolved prompts
status: todo
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-255-bind-review-artifacts-to-resolved-prompt-templates
    - T-292-publish-spec-review-bundles-safely
updated_at: "2026-08-08T14:23:09Z"
---

# T-299-bind-spec-review-publication-to-resolved-prompts Bind spec review publication to resolved prompts

## Description

Bind each spec-review lens observation to its exact role-mandated prompt resolution
at publication while leaving the aggregate manifest free of duplicate bindings.

## Acceptance

- A1. Consistency, gaps, additions, and adversarial files require their matching
  prompt IDs, contract `v1`, exact template digests, and effective sources; the
  manifest carries only transitive file-digest bindings.
- A2. Every lens passes strict role/binding validation before active committed or
  local resolution, with specified malformed/invalid-replacement/drift precedence.
- A3. Preview/apply snapshot and final-recheck all four prompt/config resolutions
  with the spec bundle; any one stale lens prevents the complete directory commit.
- A4. Equal-byte source transitions remain detectable, later prompt changes leave
  published sessions stable, and no execution or reviewer-independence claim is made.

## Verification Notes

- A1-A3: a four-lens role matrix covers mixed built-in/replacement sources, wrong
  IDs/contracts, malformed fields, invalid replacements, one-byte/source/config
  races, and all-or-none refusal.
- A4: historical reads and schema/help assertions prove immutable bindings without
  duplicate manifest metadata or certification language.

## Implementation Notes
