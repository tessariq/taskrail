---
id: T-293-publish-decomposition-review-bundles-safely
title: Publish decomposition review bundles safely
status: completed
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-215-add-the-generic-review-artifact-publisher
    - T-274-decode-reviewed-decomposition-bundles-strictly
updated_at: "2026-08-21T15:28:04Z"
completion_id: "03bfe2de51fe91e4709642daf9dc4b90"
last_verification_id: "98ac4c533254abab2b1af173c0909156"
last_verification_result: pass
last_verified_at: "2026-08-21T15:28:04Z"
last_verified_completion_id: "03bfe2de51fe91e4709642daf9dc4b90"
---

# T-293-publish-decomposition-review-bundles-safely Publish decomposition review bundles safely

## Description

Add the decomposition-review adapter to the shared directory publisher so one
final reviewed draft/trace session is published intact for later import.

## Acceptance

- A1. `review publish --type decomposition` accepts only its declared flags and
  exact manifest-selected inventory: draft, trace, review 1, optional review 2,
  and manifest beneath the transient proposal root.
- A2. The adapter enforces session/version identity, selected spec and final
  spec-review bindings, exact bundle digests, consecutive review passes, final-byte
  review, complete dispositions, and draft/trace referential integrity.
- A3. Preview/apply recheck subject, config, proposal, path, and destination
  snapshots before one absent-directory commit that preserves every selected file
  byte-for-byte.
- A4. Invalid, stale, aliased, tracked/staged/non-ignored, or changed inputs expose
  no final directory and never apply tasks or edit spec/task/state. Prompt binding
  is T-300.

## Verification Notes

- A1/A2: one-pass/two-pass goldens plus inventory, pass-order, disposition,
  trace/draft, spec-review, digest, session, and final-byte mutations prove strict
  adapter behavior.
- A3/A4: preview/apply snapshots, path/Git races, and publication fault injection
  prove exact-byte all-or-none output with semantic-write sentinels.

## Implementation Notes

- 2026-08-21T15:27:51Z: Published strict decomposition review bundles through the shared atomic directory boundary.
- 2026-08-21T15:28:04Z: verification pass id 98ac4c533254abab2b1af173c0909156 previous none completion 03bfe2de51fe91e4709642daf9dc4b90
