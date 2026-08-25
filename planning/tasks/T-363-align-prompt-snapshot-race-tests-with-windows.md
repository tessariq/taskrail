---
id: T-363-align-prompt-snapshot-race-tests-with-windows
title: Align prompt snapshot race tests with Windows durability
status: completed
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-255-bind-review-artifacts-to-resolved-prompt-templates
updated_at: "2026-08-25T18:24:59Z"
completion_id: "04152576ad541ee0e8aa95a83b091b11"
last_verification_id: "b94344ab25a83feaf17049cc738925e1"
last_verification_result: pass
last_verified_at: "2026-08-25T18:24:59Z"
last_verified_completion_id: "04152576ad541ee0e8aa95a83b091b11"
---

# T-363-align-prompt-snapshot-race-tests-with-windows Align prompt snapshot race tests with Windows durability

## Description

Keep prompt-snapshot race coverage aligned with the native Windows durability
contract. The race fixture requires successful durable review-parent creation
before inducing source drift, while Windows intentionally refuses that directory
barrier rather than weakening publication guarantees.

Follow-up derived from T-255-bind-review-artifacts-to-resolved-prompt-templates's verification or discovery. This task owns the independently meaningful deferred outcome and any required integrated delivery.

## Acceptance

- `TestReviewPublishTaskRechecksPromptSnapshotsBeforeCommit` skips native Windows
  with the established unsupported-directory-durability reason.
- Linux and macOS retain both source-transition and replacement-byte race
  coverage, and production publication behavior remains unchanged.
- Native Windows retains lower-level coverage that classifies an access-denied
  directory barrier as `durablefs.ErrUnsupported`.

## Verification Notes

- Run the focused prompt-snapshot race test and the durable filesystem Windows
  classification test.
- Run formatting, vet, the full Go suite, native filesystem portability,
  planning validation, and task-body hygiene checks.

## Implementation Notes

- 2026-08-25T18:24:58Z: Guarded the durable review-parent race fixture on native Windows without changing production behavior.
- 2026-08-25T18:24:59Z: verification pass id b94344ab25a83feaf17049cc738925e1 previous none completion 04152576ad541ee0e8aa95a83b091b11
