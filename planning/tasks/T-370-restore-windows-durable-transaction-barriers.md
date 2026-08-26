---
id: T-370-restore-windows-durable-transaction-barriers
title: Restore Windows durable transaction barriers
status: completed
priority: high
spec_ref: specs/v0.5.0.md#adversarial-spec-to-task-decomposition
dependencies:
    - T-163-validate-and-apply-importdraft-v2-transactionally
updated_at: "2026-08-26T11:27:00Z"
completion_id: "981c3e4a290bc552caeeccced0baff78"
last_verification_id: "c6b7e3667864a2844ecb95c7695c3d62"
last_verification_result: pass
last_verified_at: "2026-08-26T11:27:00Z"
last_verified_completion_id: "981c3e4a290bc552caeeccced0baff78"
---

# T-370-restore-windows-durable-transaction-barriers Restore Windows durable transaction barriers

## Description

Follow-up derived from T-163-validate-and-apply-importdraft-v2-transactionally's verification or discovery. This task owns the independently meaningful deferred outcome and any required integrated delivery.

Keep reviewed-import and workflow-publication transaction tests portable without
weakening their fail-closed durability contract. Success-path fixtures must run
only where the filesystem supports the parent-directory barriers that production
requires.

## Acceptance

- Reviewed ImportDraft v2 publication and workflow publication/recovery tests
  probe actual directory durability before requiring a successful transaction.
- Unsupported filesystems, including native Windows runners, skip only the
  durability-dependent success assertions; validation-only coverage remains.
- Production transaction and native durability behavior is unchanged, and all
  supported-platform assertions continue to execute and pass.

## Verification Notes

- Run the focused reviewed-import and workflow publication/recovery tests on a
  supported filesystem, the native durability classifier test, full tests, vet,
  and exact-head cross-platform CI.

## Implementation Notes

- Reused the established runtime capability probe immediately before each
  durability-dependent apply or recovery call. This preserves fixture, schema,
  preview, and no-write assertions on unsupported filesystems while leaving the
  production parent-directory barrier unchanged.
- A portability review found one overly broad workflow gate; the narrow
  final-diff review found the same issue in import and recovery setup. All three
  probes now guard only their transaction success assertions.
- 2026-08-26T11:27:00Z: Capability-gated only durability-dependent transaction assertions while retaining cross-platform validation coverage.
- 2026-08-26T11:27:00Z: verification pass id c6b7e3667864a2844ecb95c7695c3d62 previous none completion 981c3e4a290bc552caeeccced0baff78
