---
id: T-349-serialize-global-transaction-hook-tests
title: Serialize global transaction hook tests
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-171-contain-and-pin-autonomous-loop-child-processes
updated_at: "2026-08-22T13:00:56Z"
completion_id: "6f6b394117a397c21de48a62282ac5ed"
last_verification_id: "ef2470ce321031ec168ad6cd409665e3"
last_verification_result: pass
last_verified_at: "2026-08-22T13:00:56Z"
last_verified_completion_id: "6f6b394117a397c21de48a62282ac5ed"
---

# T-349-serialize-global-transaction-hook-tests Serialize global transaction hook tests

## Description

Prevent package-global transaction fault hooks from contaminating parallel writer
tests while preserving their rollback and Git-cleanliness assertions.

Follow-up derived from T-171-contain-and-pin-autonomous-loop-child-processes's verification or discovery. This task owns the independently meaningful deferred outcome and any required integrated delivery.

## Acceptance

- The two rename tests that install `testHookWriterValidated` execute in the
  package's serial test phase, matching every other installer of that global hook.
- Permission-fault rollback and Git-index cleanliness assertions remain unchanged.
- Repeated focused runs, vet, and the full Go suite pass without cross-test hook
  contamination or read-only temporary-directory cleanup failures.

## Verification Notes

- Run the two affected tests repeatedly with the package's transaction tests,
  then run vet, the full Go suite, validation, and exact-head CI.

## Implementation Notes

- 2026-08-22T13:00:56Z: Serialized the two rename tests that install the package-global transaction fault hook.
- 2026-08-22T13:00:56Z: verification pass id ef2470ce321031ec168ad6cd409665e3 previous none completion 6f6b394117a397c21de48a62282ac5ed
