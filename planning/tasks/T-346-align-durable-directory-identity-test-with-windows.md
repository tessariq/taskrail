---
id: T-346-align-durable-directory-identity-test-with-windows
title: Align durable directory identity test with Windows
status: completed
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-292-publish-spec-review-bundles-safely
updated_at: "2026-08-21T16:02:09Z"
completion_id: "9dc45eb0ecc060c9ef25a222de3de1fd"
last_verification_id: "a04fa2aa9a8673f843af3520a7189676"
last_verification_result: pass
last_verified_at: "2026-08-21T16:02:09Z"
last_verified_completion_id: "9dc45eb0ecc060c9ef25a222de3de1fd"
---

# T-346-align-durable-directory-identity-test-with-windows Align durable directory identity test with Windows

## Description

Align the durable-directory replacement regression fixture added by T-292 with
the native Windows durability contract. Windows cannot durably sync directories,
so the test cannot create the directory whose replacement identity it intends to
exercise and must skip that unsupported path explicitly.

Follow-up derived from T-292-publish-spec-review-bundles-safely's verification or discovery.

## Acceptance

- `TestRemoveDirExpectedRefusesReplacement` skips native Windows with the same
  explicit unsupported-directory-durability reason as adjacent durable
  directory mutation fixtures.
- Native Windows continues to exercise the mapping from an access-denied
  directory barrier to `ErrUnsupported`; production durability behavior remains
  fail-closed.
- Formatting, vet, the full test suite, planning validation, task-body hygiene,
  and native filesystem portability checks pass.

## Verification Notes

- GitHub Actions run 32500274834 demonstrates the missing fixture guard:
  `TestRemoveDirExpectedRefusesReplacement` fails at its setup `Mkdir` with the
  expected unsupported parent-directory durability barrier.

## Implementation Notes

- 2026-08-21T16:01:47Z: verification pass id 12e6d33c44b204b80c4b0a3f5fa7946b previous none completion none
- 2026-08-21T16:02:02Z: Guarded the durable directory replacement fixture on native Windows without changing fail-closed durability behavior.
- 2026-08-21T16:02:09Z: verification pass id a04fa2aa9a8673f843af3520a7189676 previous none completion 9dc45eb0ecc060c9ef25a222de3de1fd
