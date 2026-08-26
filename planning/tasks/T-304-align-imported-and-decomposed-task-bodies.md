---
id: T-304-align-imported-and-decomposed-task-bodies
title: Align imported and decomposed task bodies
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#outcome-focused-task-authoring
dependencies:
    - T-251-ship-the-outcome-focused-task-authoring-prompt
    - T-252-validate-reviewed-decomposition-bundles
updated_at: "2026-08-26T12:19:51Z"
completion_id: "c52e15085fadf8ea7cf5d228c029c72e"
last_verification_id: "6bc0649b72aa2dfdba4aa3a04d9d56a8"
last_verification_result: pass
last_verified_at: "2026-08-26T12:19:51Z"
last_verified_completion_id: "c52e15085fadf8ea7cf5d228c029c72e"
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

- Kept ImportDraft v1 schema-compatible: supplied legacy body text remains
  accepted but apply emits the standard outcome-focused scaffold and leaves the
  task implicitly held. ImportDraft v2 now validates canonical ordered body
  sections while preserving exact reviewed bytes for publication.
- Expanded both decomposition prompts and the packaged decomposition/import
  skills with the shared split, do-not-split, integration-owner, boundary, and
  durable-oracle rubric plus the digest-bound two-pass review flow.
- Review repairs made Setext detection paragraph-aware at common Markdown block
  boundaries and added a plain spec-content read before digest comparison.
- A disposable committed-storage sandbox confirmed v1 scaffold creation,
  implicit hold, prompt guidance, and repository validation end to end.
- 2026-08-26T12:19:50Z: Aligned v1 scaffold compatibility and exact reviewed v2 decomposition bodies with the shared outcome-focused contract.
- 2026-08-26T12:19:51Z: verification pass id 6bc0649b72aa2dfdba4aa3a04d9d56a8 previous none completion c52e15085fadf8ea7cf5d228c029c72e
