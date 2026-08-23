---
id: T-357-make-late-result-parent-swap-test-capability-aware
title: Make late result parent swap test capability-aware
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-356-stabilize-dead-process-fixtures-against-pid-reuse
updated_at: "2026-08-23T20:11:57Z"
completion_id: "69d5b49b3fef3ce86a796504969fb3f3"
last_verification_id: "254216d48145c6cd9c2efcf6e5aeff07"
last_verification_result: pass
last_verified_at: "2026-08-23T20:11:57Z"
last_verified_completion_id: "69d5b49b3fef3ce86a796504969fb3f3"
---

# T-357-make-late-result-parent-swap-test-capability-aware Make late result parent swap test capability-aware

## Description

Follow-up derived from T-356-stabilize-dead-process-fixtures-against-pid-reuse's verification or discovery. This task owns the independently meaningful deferred outcome and any required integrated delivery.

Keep the loop result-file parent substitution test portable when a native filesystem refuses to rename an open directory. The unsupported late-swap simulation should skip visibly without suppressing the portable no-clobber and pre-publication substitution coverage required by the cross-platform autonomous loop.

## Acceptance

- The late parent-swap case passes where renaming an open directory is supported and skips visibly where it is refused.
- The no-clobber and pre-publication parent-swap cases still execute on every platform.
- Full and native filesystem test suites pass.

## Verification Notes

- Run the focused result-file test, the dedicated native filesystem suite, full tests, vet, and planning validation.

## Implementation Notes

- 2026-08-23T20:11:53Z: Scoped the late parent substitution race to a subtest that skips when the native filesystem refuses renaming the open parent, while preserving universal no-clobber and pre-publication swap coverage.
- 2026-08-23T20:11:57Z: verification pass id 254216d48145c6cd9c2efcf6e5aeff07 previous none completion 69d5b49b3fef3ce86a796504969fb3f3
