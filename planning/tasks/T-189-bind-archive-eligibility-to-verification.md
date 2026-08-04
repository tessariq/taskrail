---
id: T-189-bind-archive-eligibility-to-verification
title: Bind archive eligibility to verification generations
status: todo
priority: high
spec_ref: specs/v0.6.0.md#archive-eligibility-and-verification-metadata
dependencies:
    - T-185-upgrade-repositories-transactionally-to-layout-3
    - T-183-validate-cancellation-generation-and-archive
    - T-184-recover-retained-semantic-transactions-explicitly
    - T-182-define-exact-v0-6-machine-result-schemas
updated_at: "2026-08-04T23:06:23Z"
---

# T-189-bind-archive-eligibility-to-verification Bind archive eligibility to verification generations

## Description

Add arbitrary-width completion generations and verification writers over the
pure metadata/eligibility matrix while preserving v0.5 IDs.

## Acceptance

- Complete/verify acquire transaction ownership before target/current evidence
  reads and hold through artifact/state/task commit or recovery.
- Complete increments from absent zero, preserves/creates completion ID, and
  clears stale verification.
- Verify writes exact completed-pass and non-completed/fail matrices; failure
  never initializes missing completion metadata.
- Every valid v0.5 debt shape follows specified pass/fail/complete repair;
  unlisted partials fail through shared validation.
- Artifact/follow-up/policy/state/task publication uses final eligibility
  marker/recovery and failed/partial execution never qualifies archive.

## Verification Notes

- Map criteria to pre-read races, full state matrix, huge generations,
  repeat/audit/recovery/stale preview and every publication fault.
- Persist manual adoption/eligibility evidence.

## Implementation Notes
