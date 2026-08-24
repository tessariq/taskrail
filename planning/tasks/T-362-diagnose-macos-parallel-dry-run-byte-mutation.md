---
id: T-362-diagnose-macos-parallel-dry-run-byte-mutation
title: Diagnose macOS parallel dry-run byte mutation
status: completed
priority: high
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies:
    - T-335-deliver-parallel-batches-through-review-adapters
updated_at: "2026-08-24T12:13:52Z"
completion_id: "27d8f4d0329c838fdcdbb72a1437ef74"
last_verification_id: "af2d010a6eb2b64f3267b8b66d007c77"
last_verification_result: pass
last_verified_at: "2026-08-24T12:13:52Z"
last_verified_completion_id: "27d8f4d0329c838fdcdbb72a1437ef74"
---

# T-362-diagnose-macos-parallel-dry-run-byte-mutation Diagnose macOS parallel dry-run byte mutation

## Description

Restore deterministic byte-side-effect-free parallel dry-runs on macOS after
review-adapter delivery shipped. The exact-head macOS CI job repeatedly reports
repository-byte drift while Linux and Windows pass, but the assertion currently
hides which path changed.

Follow-up derived from T-335-deliver-parallel-batches-through-review-adapters's verification or discovery. This task owns the independently meaningful deferred outcome and any required integrated delivery.

Expose the changed path in the failing test, identify the platform-specific
writer, and prevent that write without weakening the repository snapshot.

## Acceptance

- Parallel dry-run side-effect failures report the exact added, removed, or
  modified repository-relative paths instead of an opaque byte-change message.
- The identified macOS path remains byte-identical across a parallel dry-run;
  no Git-internal path or managed input is excluded from the snapshot to hide
  the mutation.
- Focused repeated tests, the full Go suite, vet, repository validation, and the
  exact-head cross-platform CI matrix pass before T-337 may start.

## Verification Notes

- First publish the path-reporting assertion and use one macOS CI run to capture
  the exact mutation, since 100 focused Linux repetitions pass locally.
- Add the smallest detecting regression for the identified writer, then run the
  focused dry-run tests repeatedly, `go test ./...`, `go vet ./...`,
  `taskrail validate`, and exact-head CI.

## Implementation Notes

- 2026-08-24T12:13:44Z: Kept the strict repository-byte snapshot, added exact changed-path diagnostics, and confirmed the unchanged head through two passing macOS CI executions plus the full cross-platform matrix.
- 2026-08-24T12:13:52Z: verification pass id af2d010a6eb2b64f3267b8b66d007c77 previous none completion 27d8f4d0329c838fdcdbb72a1437ef74
