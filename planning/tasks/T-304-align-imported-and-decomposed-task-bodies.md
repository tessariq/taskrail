---
id: T-304-align-imported-and-decomposed-task-bodies
title: Align imported and decomposed task bodies
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#outcome-focused-task-authoring
dependencies:
    - T-251-ship-the-outcome-focused-task-authoring-prompt
    - T-252-validate-reviewed-decomposition-bundles
updated_at: "2026-08-08T14:23:09Z"
---

# T-304-align-imported-and-decomposed-task-bodies Align imported and decomposed task bodies

## Description

Align imported task producers and reviewed decomposition outputs with the shared
outcome-focused body contract while preserving each schema version's adoption boundary.

## Acceptance

- A1. Legacy ImportDraft v1 task production emits the required scaffold/body shape
  and authoring guidance without changing v1 schema or pretending unreviewed
  scaffolds are semantically right-sized.
- A2. ImportDraft v2 requires and preserves exact reviewed body bytes containing
  one non-empty ordered Description, Acceptance, and Verification Notes section;
  optional Implementation Notes retain their defined meaning.
- A3. Decomposition author/adversarial guidance applies the shared split,
  do-not-split, anti-fragmentation, observable-oracle, boundary, and integration-
  ownership rubric before publication/import.
- A4. Every imported/decomposed task omits loop-policy fields and remains implicitly
  held; schema, review, digest, dependency, anchor, and transactional behavior stay
  owned by their existing validators/writers.

## Verification Notes

- A1/A2: v1 scaffold and v2 exact-body goldens cover empty/duplicate/extra headings,
  byte preservation, and compatibility.
- A3/A4: oversized, fragmented, coupled-integration, shallow-evidence, and
  loop-policy mutation bundles receive the expected review/refusal while existing
  schema/digest/transaction fixtures remain green.

## Implementation Notes
