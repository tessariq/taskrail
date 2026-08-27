---
id: T-374-canonicalize-local-transaction-containment-roots
title: Canonicalize local transaction containment roots
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-373-make-local-loop-delivery-fixture-portable-on-macos
updated_at: "2026-08-27T15:09:36Z"
completion_id: "7cb4c415f714933ed3ef2ee4b78cfa3b"
last_verification_id: "cb006e106f3026d64eb3a6296b44a143"
last_verification_result: pass
last_verified_at: "2026-08-27T15:09:36Z"
last_verified_completion_id: "7cb4c415f714933ed3ef2ee4b78cfa3b"
---

# T-374-canonicalize-local-transaction-containment-roots Canonicalize local transaction containment roots

## Description

Make repository-local transaction containment robust to platform path aliases
such as macOS `/var` to `/private/var` by comparing consistently canonicalized
roots and managed paths without weakening outside-repository rejection.

This task owns integrated delivery of the deferred outcome and its invariant after T-373-make-local-loop-delivery-fixture-portable-on-macos's verification.

## Acceptance

- Transaction authorization accepts a managed physical path beneath the canonical
  repository when the lock records a symlink-aliased spelling of that root.
- Existing lexical and symlink-based outside-repository paths remain rejected.
- The local loop delivery fixture and full test suite pass on native macOS CI.

## Verification Notes

- Exercise authorization through a portable temporary-directory symlink alias and
  rerun the existing transaction escape tests.
- Run the local delivery regression, full Go suite, vet, and exact-head CI matrix.

## Implementation Notes

- 2026-08-27T15:09:35Z: Canonicalized transaction roots and managed paths consistently, pinned the root for authorization and publication, and added publication-time containment proof for removals; regression, full suite, vet, validation, and independent security review passed.
- 2026-08-27T15:09:36Z: verification pass id cb006e106f3026d64eb3a6296b44a143 previous none completion 7cb4c415f714933ed3ef2ee4b78cfa3b
