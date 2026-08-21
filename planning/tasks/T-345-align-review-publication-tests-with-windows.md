---
id: T-345-align-review-publication-tests-with-windows
title: Align review publication tests with Windows durability
status: completed
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-215-add-the-generic-review-artifact-publisher
updated_at: "2026-08-21T14:57:46Z"
completion_id: "8b9e0803cba4aaa8a0d7f9260984c0e1"
last_verification_id: "7329f6e5f3b7d561f372afdebb00b760"
last_verification_result: pass
last_verified_at: "2026-08-21T14:57:46Z"
last_verified_completion_id: "8b9e0803cba4aaa8a0d7f9260984c0e1"
---

# T-345-align-review-publication-tests-with-windows Align review publication tests with Windows durability

## Description

Align the task-review publisher's positive apply fixture with the durable
filesystem contract on native Windows, where directory durability barriers are
explicitly unsupported and successful durable publication cannot be claimed.

## Acceptance

- The positive review-publication apply fixture skips native Windows with the
  same explicit unsupported-directory-durability reason as the shared typed
  directory publication fixtures.
- Native Windows still exercises the durable filesystem classification that
  maps an access-denied directory barrier to `ErrUnsupported`; production code
  does not downgrade or ignore that failure.
- The full Linux test suite, formatting, vet, planning validation, and task-body
  hygiene checks pass.

## Verification Notes

- GitHub Actions run 32494183619 demonstrated the omitted fixture guard:
  `TestReviewPublishTaskPreviewAndApplyBindExactBytes` reached the expected
  Windows parent-directory barrier and failed with `ErrUnsupported`.

## Implementation Notes

- 2026-08-21T14:57:41Z: Aligned the positive review publication fixture with the established Windows directory-durability contract.
- 2026-08-21T14:57:46Z: verification pass id 7329f6e5f3b7d561f372afdebb00b760 previous none completion 8b9e0803cba4aaa8a0d7f9260984c0e1
