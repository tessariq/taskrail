---
id: T-347-align-review-parent-cleanup-test-with-windows
title: Align review parent cleanup test with Windows
status: completed
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-346-align-durable-directory-identity-test-with-windows
updated_at: "2026-08-21T16:11:19Z"
completion_id: "547f745a21885343b9aff774a1d88fac"
last_verification_id: "043d49ee6783aeb1452432d5b64a96e6"
last_verification_result: pass
last_verified_at: "2026-08-21T16:11:19Z"
last_verified_completion_id: "547f745a21885343b9aff774a1d88fac"
---

# T-347-align-review-parent-cleanup-test-with-windows Align review parent cleanup test with Windows

## Description

Align the late-conflict review-parent cleanup fixture with the native Windows
durability contract. The fixture requires successful durable parent-directory
creation before inducing its conflict, which Windows explicitly does not
support.

Follow-up derived from T-346-align-durable-directory-identity-test-with-windows's verification or discovery.

## Acceptance

- `TestReviewPublishSpecCleansNewParentsAfterLateSnapshotConflict` skips native
  Windows with the established unsupported-directory-durability reason.
- Native Windows retains coverage of fail-closed directory barrier
  classification and all review validation paths that do not require successful
  durable directory mutation.
- Formatting, vet, the full test suite, planning validation, task-body hygiene,
  and native filesystem portability checks pass.

## Verification Notes

- GitHub Actions run 32500984361 demonstrates the fixture reaching the expected
  unsupported parent-directory barrier before its synthetic snapshot conflict.

## Implementation Notes

- 2026-08-21T16:11:12Z: Guarded the durable review-parent cleanup fixture on native Windows without changing fail-closed publication behavior.
- 2026-08-21T16:11:19Z: verification pass id 043d49ee6783aeb1452432d5b64a96e6 previous none completion 547f745a21885343b9aff774a1d88fac
