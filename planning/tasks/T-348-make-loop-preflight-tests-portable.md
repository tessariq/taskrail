---
id: T-348-make-loop-preflight-tests-portable
title: Make loop preflight tests portable
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-170-add-deterministic-autonomous-loop-preflight-and
updated_at: "2026-08-22T11:50:56Z"
completion_id: "edaf7b24b3ec4c01156400d9587ba411"
last_verification_id: "e2e2b6c0735da3584343962e363ff0ab"
last_verification_result: pass
last_verified_at: "2026-08-22T11:50:56Z"
last_verified_completion_id: "edaf7b24b3ec4c01156400d9587ba411"
---

# T-348-make-loop-preflight-tests-portable Make loop preflight tests portable

## Description

Keep the loop-preflight alias and local-storage fixtures portable across native
filesystems without weakening the product's alias rejection or local ignore
proof.

Follow-up derived from T-170-add-deterministic-autonomous-loop-preflight-and's verification or discovery. This task owns the independently meaningful deferred outcome and any required integrated delivery.

## Acceptance

- The Git-root alias fixture exercises distinct case-variant entries where the
  filesystem supports them and skips when both spellings identify one file.
- The local-storage fixture uses the established directory-durability capability
  gate before requiring successful local initialization.
- Focused tests and the full suite pass on Linux, macOS, and native Windows.

## Verification Notes

- Run the focused loop-preflight tests, repository validation, vet, and the full
  Go suite; confirm exact-head native Windows and macOS CI.

## Implementation Notes

- 2026-08-22T11:50:55Z: Made loop preflight alias and local-storage fixtures capability-aware across native filesystems.
- 2026-08-22T11:50:56Z: verification pass id e2e2b6c0735da3584343962e363ff0ab previous none completion edaf7b24b3ec4c01156400d9587ba411
