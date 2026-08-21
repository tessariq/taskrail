---
id: T-340-accept-verification-identity-reports-in-temporary
title: Accept verification identity reports in temporary loop
status: completed
priority: high
spec_ref: specs/v0.5.0.md#source-checkout-bootstrap-loop-retirement
dependencies: []
updated_at: "2026-08-21T09:17:18Z"
completion_id: "080eeea105d0f74898cb23ea1877cb59"
---

# T-340-accept-verification-identity-reports-in-temporary Accept verification identity reports in temporary loop

## Description

Keep the temporary source-checkout loop able to validate and deliver T-285 when
that task adds stable verification identity fields to schema-1 `report.json`.
Recognize only the planned fields, validate non-null identities, and preserve the
strict rejection of unrelated report extensions.

## Acceptance

- The temporary report checker accepts both the current schema-1 report and the
  T-285 shape carrying `verification_id`, `previous_verification_id`, and
  `observed_completion_id`.
- Every present non-null identity is lower-case 32-hex, and predecessor or
  completion identity fields cannot appear without a verification identity.
- Unknown fields, malformed identities, task/result mismatches, and existing
  follow-up recommendation violations remain fail-closed before integration.
- The autonomous-loop harness proves identity-bearing delivery succeeds while
  malformed and unrelated extensions cannot commit or push.

## Verification Notes

- Run `bash -n` over the loop shell scripts, the complete autonomous-loop
  harness, queue validation, repository validation, vet, and the full Go suite.

## Implementation Notes

- 2026-08-21T09:17:18Z: Taught the temporary report checker the stable verification identity fields while preserving strict legacy and unknown-field handling.
- 2026-08-21T09:17:18Z: verification pass
